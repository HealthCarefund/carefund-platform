package reconcile

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/migrate"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// Genuine integration tests against the isolated CareFund PostgreSQL 18.6
// instance (carefund-pg18, port 5439) — never the host's PostgreSQL 16.
// Skip cleanly if it isn't reachable. Run with `go test -p 1` alongside
// internal/store and internal/api, which share the same live instance.

func testDatabaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://carefund:carefund_dev@localhost:5439/carefund"
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDatabaseURL())
	if err != nil {
		t.Skipf("could not create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("isolated CareFund Postgres 18.6 instance not reachable: %v", err)
	}
	if _, err := migrate.Run(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("running migrations: %v", err)
	}
	t.Cleanup(func() {
		truncateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(truncateCtx, `TRUNCATE TABLE
			reconciliation_state, idempotency_keys, agreement_intents, audit_records,
			contract_events, transaction_refs, attestations, care_agreements,
			attesters, providers
			RESTART IDENTITY CASCADE`)
		pool.Close()
	})

	return store.New(pool)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func make32(b byte) []byte {
	h := make([]byte, 32)
	for i := range h {
		h[i] = b
	}
	return h
}

type fakeRPC struct {
	getTransaction func(hash string) (protocol.GetTransactionResponse, error)
	getEvents      func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error)
	health         func() (protocol.GetHealthResponse, error)
}

func (f *fakeRPC) GetTransaction(_ context.Context, hash string) (protocol.GetTransactionResponse, error) {
	return f.getTransaction(hash)
}
func (f *fakeRPC) GetEvents(_ context.Context, req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
	if f.getEvents == nil {
		return protocol.GetEventsResponse{}, nil
	}
	return f.getEvents(req)
}
func (f *fakeRPC) Health(_ context.Context) (protocol.GetHealthResponse, error) {
	if f.health == nil {
		return protocol.GetHealthResponse{Status: "healthy", OldestLedger: 1}, nil
	}
	return f.health()
}

func TestReconcileTransactions_UpdatesToConfirmed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	if err := s.InsertTransactionRef(ctx, nil, "fund", hash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef: %v", err)
	}

	rpc := &fakeRPC{getTransaction: func(h string) (protocol.GetTransactionResponse, error) {
		return protocol.GetTransactionResponse{
			TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusSuccess, Ledger: 999},
		}, nil
	}}
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55))
	runner.reconcileTransactions(ctx)

	got, err := s.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v", err)
	}
	if got.Status != "confirmed" || got.Ledger == nil || *got.Ledger != 999 {
		t.Errorf("got = %+v", got)
	}
	if got.ConfirmedAt == nil {
		t.Error("expected ConfirmedAt to be set")
	}
}

func TestReconcileTransactions_UpdatesToFailed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash := strings.Repeat("b", 64)
	if err := s.InsertTransactionRef(ctx, nil, "fund", hash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef: %v", err)
	}

	rpc := &fakeRPC{getTransaction: func(h string) (protocol.GetTransactionResponse, error) {
		return protocol.GetTransactionResponse{
			TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusFailed, Ledger: 1000},
		}, nil
	}}
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55))
	runner.reconcileTransactions(ctx)

	got, err := s.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("Status = %q, want failed", got.Status)
	}
}

func TestReconcileTransactions_LeavesStatusUnchangedOnNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash := strings.Repeat("c", 64)
	if err := s.InsertTransactionRef(ctx, nil, "fund", hash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef: %v", err)
	}

	rpc := &fakeRPC{getTransaction: func(h string) (protocol.GetTransactionResponse, error) {
		return protocol.GetTransactionResponse{
			TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusNotFound},
		}, nil
	}}
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55))
	runner.reconcileTransactions(ctx)

	got, err := s.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v", err)
	}
	if got.Status != "submitted" {
		t.Errorf("Status = %q, want unchanged (submitted) — never resubmit, never fabricate a status", got.Status)
	}
	if got.LastChecked == nil {
		t.Error("expected LastChecked to be bumped even when status is unchanged")
	}
}

func TestReconcileTransactions_RPCFailureDoesNotAbortOtherHashes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	failHash := strings.Repeat("d", 64)
	okHash := strings.Repeat("e", 64)
	if err := s.InsertTransactionRef(ctx, nil, "fund", failHash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef(fail): %v", err)
	}
	if err := s.InsertTransactionRef(ctx, nil, "fund", okHash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef(ok): %v", err)
	}

	rpc := &fakeRPC{getTransaction: func(h string) (protocol.GetTransactionResponse, error) {
		if h == failHash {
			return protocol.GetTransactionResponse{}, context.DeadlineExceeded
		}
		return protocol.GetTransactionResponse{
			TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusSuccess, Ledger: 1},
		}, nil
	}}
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55))
	runner.reconcileTransactions(ctx)

	ok, err := s.GetTransactionByHash(ctx, okHash)
	if err != nil {
		t.Fatalf("GetTransactionByHash(ok): %v", err)
	}
	if ok.Status != "confirmed" {
		t.Errorf("okHash Status = %q, want confirmed — one RPC failure must not block reconciling other hashes", ok.Status)
	}

	failed, err := s.GetTransactionByHash(ctx, failHash)
	if err != nil {
		t.Fatalf("GetTransactionByHash(fail): %v", err)
	}
	if failed.Status != "submitted" {
		t.Errorf("failHash Status = %q, want unchanged (submitted)", failed.Status)
	}
}

func TestReconcileEvents_PersistsCursorAndIngestsIdempotently(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	providerRegistryID := "C" + strings.Repeat("A", 55)
	careAgreementID := "C" + strings.Repeat("B", 55)

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID:               7,
		SponsorWallet:             "G" + strings.Repeat("S", 55),
		ProviderWallet:            "G" + strings.Repeat("P", 55),
		AttesterWallet:            "G" + strings.Repeat("T", 55),
		PatientRefCommitment:      make32(0x01),
		ServiceCommitment:         make32(0x02),
		FundingAmount:             "1000000",
		SettlementAmount:          "900000",
		SettlementAssetContractID: "C" + strings.Repeat("X", 55),
		FundingDeadline:           1_700_000_000,
		CareDeadline:              1_700_100_000,
		DisputeWindowSecs:         86_400,
		State:                     "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	event := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          500,
		ContractID:      careAgreementID,
		ID:              "0000002000-0000000000",
		TransactionHash: strings.Repeat("f", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "agr_cre")},
		ValueXDR:        u64ValueXDR(t, 7),
	}
	calls := 0
	rpc := &fakeRPC{
		getEvents: func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
			calls++
			return protocol.GetEventsResponse{Events: []protocol.EventInfo{event}, LatestLedger: 501}, nil
		},
	}

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID)
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 1 || page.Events[0].EventType != "agr_cre" {
		t.Fatalf("page = %+v", page)
	}

	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != "502" {
		t.Errorf("cursor = %q, want 502 (LatestLedger + 1)", cursor)
	}

	// A second pass re-fetching the same event must not create a
	// duplicate row (idempotent ingestion).
	runner.reconcileEvents(ctx)
	page, err = s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement (second pass): %v", err)
	}
	if len(page.Events) != 1 {
		t.Errorf("page after re-ingestion = %+v, want still exactly 1 event", page)
	}
	if calls != 2 {
		t.Errorf("getEvents calls = %d, want 2", calls)
	}
}
