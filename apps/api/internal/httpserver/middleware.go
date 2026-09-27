package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
)

// withMaxBody bounds every request body, so a client can't exhaust memory
// with an oversized payload before a handler even gets to validate it.
func withMaxBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

// withCORS allows only the explicitly configured origins (or "*" if that
// was explicitly configured) — never inferred, never wildcard by default.
func withCORS(allowedOrigins []string, next http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok || allowAll {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecordingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusRecordingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// withRequestLogging logs method, path, status, and duration only.
// Request/response bodies, headers, and query strings are deliberately
// never logged here — they may carry wallet addresses or idempotency keys
// that don't need to end up in operational logs.
func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecordingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// withRecovery turns a panic in any handler into a logged 500 instead of
// killing the connection (or, without a recover, potentially the process).
func withRecovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				httpx.WriteInternal(w, logger, fmt.Errorf("panic: %v", rec), "http.recover:"+r.URL.Path)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
