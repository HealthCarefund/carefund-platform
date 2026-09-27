package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const readinessCheckTimeout = 2 * time.Second

func registerHealthRoutes(mux *http.ServeMux, deps Deps) {
	// Liveness: the process is up and serving. Never checks dependencies —
	// that's /readyz's job. A load balancer restarting the process on a
	// failed /healthz should only ever mean "the process itself is stuck".
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeStatusJSON(w, http.StatusOK, "ok", "")
	})

	// Readiness: can this instance actually serve traffic right now.
	// Currently checks the database; RPC health is not required for the
	// process to accept traffic (individual requests fail explicitly if
	// the chain is unreachable), so it is not part of this check.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if deps.DB == nil {
			writeStatusJSON(w, http.StatusServiceUnavailable, "not_ready", "database not configured")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
		defer cancel()
		if err := deps.DB.Ping(ctx); err != nil {
			writeStatusJSON(w, http.StatusServiceUnavailable, "not_ready", "database unreachable")
			return
		}
		writeStatusJSON(w, http.StatusOK, "ready", "")
	})
}

func writeStatusJSON(w http.ResponseWriter, code int, status string, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	body := map[string]string{"status": status}
	if reason != "" {
		body["reason"] = reason
	}
	_ = json.NewEncoder(w).Encode(body)
}
