// Package reconcile is CareFund's background reconciliation loop: it runs
// entirely inside the API process (no separate service), correcting
// transaction_refs against the chain's own getTransaction status and
// mirroring new contract events into contract_events. It never resubmits
// an uncertain transaction — only ever looks its hash up again — and
// exposes no HTTP endpoint of its own.
package reconcile

import (
	"context"
	"log/slog"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// RPC is the subset of stellarrpc.Client this package depends on.
// *stellarrpc.Client satisfies this directly.
type RPC interface {
	GetTransaction(ctx context.Context, hash string) (protocol.GetTransactionResponse, error)
	GetEvents(ctx context.Context, request protocol.GetEventsRequest) (protocol.GetEventsResponse, error)
	Health(ctx context.Context) (protocol.GetHealthResponse, error)
}

// Runner owns the reconciliation ticker. Construct with New and start it
// with Run, which blocks until ctx is cancelled (run it in a goroutine).
type Runner struct {
	store                      *store.Store
	rpc                        RPC
	logger                     *slog.Logger
	interval                   time.Duration
	providerRegistryContractID string
	careAgreementContractID    string
}

// New builds a Runner. interval is how often each reconciliation pass
// runs; a single slow or failed pass never blocks the next tick from
// eventually happening, since each pass bounds its own work per hash/event
// batch and logs-and-continues on individual failures.
func New(s *store.Store, rpc RPC, logger *slog.Logger, interval time.Duration, providerRegistryContractID, careAgreementContractID string) *Runner {
	return &Runner{
		store:                      s,
		rpc:                        rpc,
		logger:                     logger,
		interval:                   interval,
		providerRegistryContractID: providerRegistryContractID,
		careAgreementContractID:    careAgreementContractID,
	}
}

// Run blocks, running a reconciliation pass immediately and then every
// interval, until ctx is cancelled. A panic in one pass (defensively
// guarded against, e.g. an unexpected nil somewhere in RPC decoding) is
// recovered and logged rather than taking the whole process down —
// reconciliation is important but must never be a single point of failure
// for the API itself.
func (r *Runner) Run(ctx context.Context) {
	r.tick(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("reconcile: recovered from panic", "panic", rec)
		}
	}()
	r.reconcileTransactions(ctx)
	r.reconcileEvents(ctx)
}
