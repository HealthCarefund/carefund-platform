// Package httpserver builds the CareFund API's HTTP server: the stdlib
// net/http mux, middleware chain, and lifecycle (start/graceful shutdown).
// It intentionally uses no router framework, per the approved Phase 7 spec.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/api"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// Deps are the dependencies the HTTP layer needs. DB may be nil in tests
// that don't exercise /readyz. Store/RPC may be nil in tests that only
// exercise health checks.
type Deps struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool
	Store  *store.Store
	RPC    *stellarrpc.Client
}

// Server wraps an *http.Server with CareFund's middleware chain and
// routes already wired up.
type Server struct {
	httpServer *http.Server
}

// New builds the server. It does not start listening.
func New(deps Deps) *Server {
	mux := http.NewServeMux()
	registerHealthRoutes(mux, deps)
	api.RegisterRoutes(mux, api.Deps{Config: deps.Config, Store: deps.Store, RPC: deps.RPC, Logger: deps.Logger})

	handler := withRecovery(deps.Logger,
		withRequestLogging(deps.Logger,
			withCORS(deps.Config.CORSAllowedOrigins,
				withMaxBody(deps.Config.MaxRequestBodyBytes, mux))))

	return &Server{
		httpServer: &http.Server{
			Addr:              deps.Config.AppAddr,
			Handler:           handler,
			ReadTimeout:       deps.Config.RequestTimeout,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      deps.Config.RequestTimeout,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Handler exposes the fully-wrapped handler, for tests that want to drive
// requests through httptest without binding a real listener.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// ListenAndServe blocks until the server stops or fails to start.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown drains in-flight requests and stops the server, bounded by ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
