// Package reconcile is CareFund's background reconciliation loop: it runs
// entirely inside the API process (no separate service), correcting
// transaction_refs against the chain's own getTransaction status and
// mirroring new contract events into contract_events. It never resubmits
// an uncertain transaction - only ever looks its hash up again - and
// exposes no HTTP endpoint of its own.
package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/sorobanenc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// RPC is the subset of stellarrpc.Client this package depends on.
// *stellarrpc.Client satisfies this directly.
type RPC interface {
	GetTransaction(ctx context.Context, hash string) (protocol.GetTransactionResponse, error)
	GetEvents(ctx context.Context, request protocol.GetEventsRequest) (protocol.GetEventsResponse, error)
	Health(ctx context.Context) (protocol.GetHealthResponse, error)
	Simulate(ctx context.Context, envelopeXDR string) (protocol.SimulateTransactionResponse, error)
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
	settlementAssetContractID  string
}

// New builds a Runner. interval is how often each reconciliation pass
// runs; a single slow or failed pass never blocks the next tick from
// eventually happening, since each pass bounds its own work per hash/event
// batch and logs-and-continues on individual failures.
func New(s *store.Store, rpc RPC, logger *slog.Logger, interval time.Duration, providerRegistryContractID, careAgreementContractID, settlementAssetContractID string) *Runner {
	return &Runner{
		store:                      s,
		rpc:                        rpc,
		logger:                     logger,
		interval:                   interval,
		providerRegistryContractID: providerRegistryContractID,
		careAgreementContractID:    careAgreementContractID,
		settlementAssetContractID:  settlementAssetContractID,
	}
}

// Run blocks, running a reconciliation pass immediately and then every
// interval, until ctx is cancelled. A panic in one pass (defensively
// guarded against, e.g. an unexpected nil somewhere in RPC decoding) is
// recovered and logged rather than taking the whole process down:
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

func (r *Runner) fetchAgreement(ctx context.Context, agreementID int64) (*store.CareAgreement, error) {
	txBase64, err := sorobanenc.BuildGetAgreementTransaction(r.careAgreementContractID, agreementID)
	if err != nil {
		return nil, fmt.Errorf("building get_agreement transaction: %w", err)
	}

	sim, err := r.rpc.Simulate(ctx, txBase64)
	if err != nil {
		return nil, fmt.Errorf("simulating get_agreement: %w", err)
	}
	if sim.Error != "" {
		return nil, fmt.Errorf("simulation error: %s", sim.Error)
	}
	if len(sim.Results) == 0 {
		return nil, fmt.Errorf("simulation returned no results")
	}
	if sim.Results[0].ReturnValueXDR == nil {
		return nil, fmt.Errorf("simulation returned nil ReturnValueXDR")
	}

	var retVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(*sim.Results[0].ReturnValueXDR, &retVal); err != nil {
		return nil, fmt.Errorf("decoding simulation result ScVal: %w", err)
	}

	return sorobanenc.DecodeAgreement(retVal, agreementID, r.settlementAssetContractID)
}

func (r *Runner) reconcileAgreement(ctx context.Context, agreementID int64) error {
	agr, err := r.fetchAgreement(ctx, agreementID)
	if err != nil {
		if existing, getErr := r.store.GetAgreement(ctx, agreementID); getErr == nil && existing != nil {
			return nil
		}
		return fmt.Errorf("fetching agreement %d: %w", agreementID, err)
	}

	if err := r.store.UpsertAgreement(ctx, *agr); err != nil {
		return fmt.Errorf("upserting agreement %d: %w", agreementID, err)
	}

	r.logger.Info("reconcile: mirrored agreement from chain",
		"agreementId", agreementID,
		"state", agr.State,
		"provider", agr.ProviderWallet,
		"sponsor", agr.SponsorWallet,
	)
	return nil
}

func (r *Runner) fetchProvider(ctx context.Context, wallet string) (*store.Provider, error) {
	txBase64, err := sorobanenc.BuildGetProviderTransaction(r.providerRegistryContractID, wallet)
	if err != nil {
		return nil, fmt.Errorf("building get_provider transaction: %w", err)
	}

	sim, err := r.rpc.Simulate(ctx, txBase64)
	if err != nil {
		return nil, fmt.Errorf("simulating get_provider: %w", err)
	}
	if sim.Error != "" {
		return nil, fmt.Errorf("simulation error: %s", sim.Error)
	}
	if len(sim.Results) == 0 {
		return nil, fmt.Errorf("simulation returned no results")
	}
	if sim.Results[0].ReturnValueXDR == nil {
		return nil, fmt.Errorf("simulation returned nil ReturnValueXDR")
	}

	var retVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(*sim.Results[0].ReturnValueXDR, &retVal); err != nil {
		return nil, fmt.Errorf("decoding simulation result ScVal: %w", err)
	}

	return sorobanenc.DecodeProvider(retVal, wallet)
}

func (r *Runner) reconcileProvider(ctx context.Context, wallet string) error {
	p, err := r.fetchProvider(ctx, wallet)
	if err != nil {
		if existing, getErr := r.store.GetProviderByWallet(ctx, wallet); getErr == nil && existing != nil {
			return nil
		}
		return fmt.Errorf("fetching provider %s: %w", wallet, err)
	}

	if err := r.store.UpsertProvider(ctx, p.WalletAddress, p.ProviderRef, p.Status); err != nil {
		return fmt.Errorf("upserting provider %s: %w", wallet, err)
	}

	r.logger.Info("reconcile: mirrored provider from chain", "wallet", wallet, "status", p.Status)
	return nil
}

func (r *Runner) fetchAttester(ctx context.Context, wallet string) (*store.Attester, error) {
	txBase64, err := sorobanenc.BuildGetAttesterTransaction(r.providerRegistryContractID, wallet)
	if err != nil {
		return nil, fmt.Errorf("building get_attester transaction: %w", err)
	}

	sim, err := r.rpc.Simulate(ctx, txBase64)
	if err != nil {
		return nil, fmt.Errorf("simulating get_attester: %w", err)
	}
	if sim.Error != "" {
		return nil, fmt.Errorf("simulation error: %s", sim.Error)
	}
	if len(sim.Results) == 0 {
		return nil, fmt.Errorf("simulation returned no results")
	}
	if sim.Results[0].ReturnValueXDR == nil {
		return nil, fmt.Errorf("simulation returned nil ReturnValueXDR")
	}

	var retVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(*sim.Results[0].ReturnValueXDR, &retVal); err != nil {
		return nil, fmt.Errorf("decoding simulation result ScVal: %w", err)
	}

	return sorobanenc.DecodeAttester(retVal, wallet)
}

func (r *Runner) reconcileAttester(ctx context.Context, wallet string) error {
	a, err := r.fetchAttester(ctx, wallet)
	if err != nil {
		if existing, getErr := r.store.GetAttesterByWallet(ctx, wallet); getErr == nil && existing != nil {
			return nil
		}
		return fmt.Errorf("fetching attester %s: %w", wallet, err)
	}

	// Ensure provider row exists first for attesters.provider_wallet foreign key
	if _, err := r.store.GetProviderByWallet(ctx, a.ProviderWallet); err != nil {
		if err := r.reconcileProvider(ctx, a.ProviderWallet); err != nil {
			return fmt.Errorf("ensuring provider %s for attester: %w", a.ProviderWallet, err)
		}
	}

	if err := r.store.UpsertAttester(ctx, a.WalletAddress, a.ProviderWallet, a.CredentialRef, a.Status); err != nil {
		return fmt.Errorf("upserting attester %s: %w", wallet, err)
	}

	r.logger.Info("reconcile: mirrored attester from chain", "wallet", wallet, "provider", a.ProviderWallet, "status", a.Status)
	return nil
}
