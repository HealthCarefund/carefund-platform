// Command api is the CareFund API server entry point.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpserver"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/logging"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/migrate"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

func main() {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		// The logger depends on cfg.LogLevel, which may be exactly what
		// failed to validate, so this one line goes straight to stderr.
		os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	applied, err := migrate.Run(ctx, dbPool)
	if err != nil {
		logger.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied", "count", len(applied), "versions", applied)

	rpcClient := stellarrpc.New(cfg.StellarRPCURL, cfg.RPCTimeout)
	defer rpcClient.Close()

	dataStore := store.New(dbPool)

	server := httpserver.New(httpserver.Deps{
		Config: &cfg,
		Logger: logger,
		DB:     dbPool,
		Store:  dataStore,
		RPC:    rpcClient,
	})

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", cfg.AppAddr, "env", string(cfg.AppEnv))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
