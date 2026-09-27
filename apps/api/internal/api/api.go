// Package api implements CareFund's REST endpoints (/api/v1/...). Handlers
// validate every input before touching the database or RPC — never after
// — and never infer authorization from anything the frontend claims about
// itself; authorization is whatever the contract itself would enforce.
package api

import (
	"log/slog"
	"net/http"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// Deps are the dependencies every handler in this package may need.
type Deps struct {
	Store  *store.Store
	RPC    *stellarrpc.Client
	Logger *slog.Logger
}

// RegisterRoutes wires every /api/v1/... route onto mux.
func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	registerProviderRoutes(mux, deps)
	registerAgreementRoutes(mux, deps)
}
