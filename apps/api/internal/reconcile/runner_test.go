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
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/migrate"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/sorobanenc"
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
	simulate       func(envelopeXDR string) (protocol.SimulateTransactionResponse, error)
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
func (f *fakeRPC) Simulate(_ context.Context, envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
	if f.simulate == nil {
		return protocol.SimulateTransactionResponse{}, nil
	}
	return f.simulate(envelopeXDR)
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
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55), "C"+strings.Repeat("S", 55))
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
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55), "C"+strings.Repeat("S", 55))
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
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55), "C"+strings.Repeat("S", 55))
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
	runner := New(s, rpc, testLogger(), time.Hour, "C"+strings.Repeat("A", 55), "C"+strings.Repeat("B", 55), "C"+strings.Repeat("S", 55))
	runner.reconcileTransactions(ctx)

	ok, err := s.GetTransactionByHash(ctx, okHash)
	if err != nil {
		t.Fatalf("GetTransactionByHash(ok): %v", err)
	}
	if ok.Status != "confirmed" {
		t.Errorf("okHash Status = %q, want confirmed - one RPC failure must not block reconciling other hashes", ok.Status)
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
	settlementAssetID := "C" + strings.Repeat("X", 55)

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID:               7,
		SponsorWallet:             "G" + strings.Repeat("S", 55),
		ProviderWallet:            "G" + strings.Repeat("P", 55),
		AttesterWallet:            "G" + strings.Repeat("T", 55),
		PatientRefCommitment:      make32(0x01),
		ServiceCommitment:         make32(0x02),
		FundingAmount:             "1000000",
		SettlementAmount:          "900000",
		SettlementAssetContractID: settlementAssetID,
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

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID, settlementAssetID)
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

func walletAddressValueXDR(t *testing.T, wallet string) string {
	t.Helper()
	val, err := sorobanenc.Address(wallet)
	if err != nil {
		t.Fatalf("sorobanenc.Address(%q): %v", wallet, err)
	}
	b, err := xdr.MarshalBase64(val)
	if err != nil {
		t.Fatalf("marshaling address value: %v", err)
	}
	return b
}

func makeProviderScVal(providerRef [32]byte, status string) xdr.ScVal {
	symRef := xdr.ScSymbol("provider_ref")
	symStatus := xdr.ScSymbol("status")
	refVal, _ := sorobanenc.Bytes32(providerRef[:])
	statusVal := sorobanenc.Symbol(status)
	m := xdr.ScMap{
		{Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symRef}, Val: refVal},
		{Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symStatus}, Val: statusVal},
	}
	mPtr := &m
	return xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &mPtr}
}

func makeAttesterScVal(providerWallet string, credentialRef [32]byte, status string) xdr.ScVal {
	symProvider := xdr.ScSymbol("provider")
	symCred := xdr.ScSymbol("credential_ref")
	symStatus := xdr.ScSymbol("status")
	provAddr, _ := sorobanenc.Address(providerWallet)
	credVal, _ := sorobanenc.Bytes32(credentialRef[:])
	statusVal := sorobanenc.Symbol(status)
	m := xdr.ScMap{
		{Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symProvider}, Val: provAddr},
		{Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symCred}, Val: credVal},
		{Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symStatus}, Val: statusVal},
	}
	mPtr := &m
	return xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &mPtr}
}

func TestReconcileEvents_EntityFirstReconciliation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	providerRegistryID := "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS"
	careAgreementID := "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	settlementAssetID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	// Agreement 7 is deliberately NOT pre-seeded in the database here.
	// Entity-first reconcile must fetch it on-chain and insert it before inserting the event.
	const rawXDR = "AAAAEQAAAAEAAAASAAAADwAAABZhdHRlc3RhdGlvbl9jb21taXRtZW50AAAAAAABAAAADwAAAAthdHRlc3RlZF9hdAAAAAABAAAADwAAAAthdHRlc3RlZF9ieQAAAAABAAAADwAAAAhhdHRlc3RlcgAAABIAAAAAAAAAAH3/zCGtzLQUGKrxls38wdh5L06xwQJSVYDbwek4BYYnAAAADwAAAA1jYXJlX2RlYWRsaW5lAAAAAAAABQAAAABqyPdcAAAADwAAAApjcmVhdGVkX2F0AAAAAAAFAAAAAGrI8+kAAAAPAAAAEWRpc3B1dGVfb3BlbmVkX2F0AAAAAAAAAQAAAA8AAAARZGlzcHV0ZV9vcGVuZWRfYnkAAAAAAAABAAAADwAAAA5kaXNwdXRlX29yaWdpbgAAAAAAEAAAAAEAAAABAAAADwAAAAROb25lAAAADwAAABNkaXNwdXRlX3dpbmRvd19zZWNzAAAAAAUAAAAAAAAASAAAAA8AAAAOZnVuZGluZ19hbW91bnQAAAAAAAoAAAAAAAAAAAAAAAAAmJaAAAAADwAAABBmdW5kaW5nX2RlYWRsaW5lAAAABQAAAABqyPbkAAAADwAAABZwYXRpZW50X3JlZl9jb21taXRtZW50AAAAAAANAAAAIBERERERERERERERERERERERERERERERERERERERERERAAAADwAAAAhwcm92aWRlcgAAABIAAAAAAAAAAGz0qyL2+h41Qg2CM5P5+WYAJbEhf3sL3HL3fbE4gaikAAAADwAAABJzZXJ2aWNlX2NvbW1pdG1lbnQAAAAAAA0AAAAgIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIAAAAPAAAAEXNldHRsZW1lbnRfYW1vdW50AAAAAAAACgAAAAAAAAAAAAAAAAB6EgAAAAAPAAAAB3Nwb25zb3IAAAAAEgAAAAAAAAAA4y3eI3Ih7LlKDamd5ddJPXSjFpPyKK8yAdUQr8ljn7kAAAAPAAAABXN0YXRlAAAAAAAAEAAAAAEAAAABAAAADwAAAAlSZXF1ZXN0ZWQAAAA="

	event := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          500,
		ContractID:      careAgreementID,
		ID:              "0000002000-0000000000",
		TransactionHash: strings.Repeat("f", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "agr_cre")},
		ValueXDR:        u64ValueXDR(t, 7),
	}

	retValStr := rawXDR
	rpc := &fakeRPC{
		getEvents: func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
			return protocol.GetEventsResponse{Events: []protocol.EventInfo{event}, LatestLedger: 501}, nil
		},
		simulate: func(envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
			return protocol.SimulateTransactionResponse{
				Results: []protocol.SimulateHostFunctionResult{
					{
						ReturnValueXDR: &retValStr,
					},
				},
			}, nil
		},
	}

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID, settlementAssetID)
	runner.reconcileEvents(ctx)

	// Verify agreement was automatically mirrored in care_agreements
	mirrored, err := s.GetAgreement(ctx, 7)
	if err != nil {
		t.Fatalf("GetAgreement(7): %v (agreement was not mirrored from chain)", err)
	}
	if mirrored.ProviderWallet != "GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453" {
		t.Errorf("ProviderWallet = %q", mirrored.ProviderWallet)
	}
	if mirrored.State != "Requested" {
		t.Errorf("State = %q, want Requested", mirrored.State)
	}

	// Verify event was successfully inserted without foreign key violation
	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 1 || page.Events[0].EventType != "agr_cre" {
		t.Fatalf("page = %+v", page)
	}
}

func TestReconcileEvents_ProviderRegistryIngestion(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	providerRegistryID := "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS"
	careAgreementID := "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	settlementAssetID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	providerWallet := "GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453"
	attesterWallet := "GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT"

	var providerRef [32]byte
	copy(providerRef[:], []byte("provider-ref-32-bytes-test-ok!!!"))
	var credRef [32]byte
	copy(credRef[:], []byte("attester-cred-32-bytes-test-ok!!"))

	provScVal := makeProviderScVal(providerRef, "Active")
	provXDR, err := xdr.MarshalBase64(provScVal)
	if err != nil {
		t.Fatalf("marshaling provider ScVal: %v", err)
	}

	attScVal := makeAttesterScVal(providerWallet, credRef, "Active")
	attXDR, err := xdr.MarshalBase64(attScVal)
	if err != nil {
		t.Fatalf("marshaling attester ScVal: %v", err)
	}

	event1 := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          600,
		ContractID:      providerRegistryID,
		ID:              "0000003000-0000000000",
		TransactionHash: strings.Repeat("a", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "prov_reg")},
		ValueXDR:        walletAddressValueXDR(t, providerWallet),
	}

	event2 := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          601,
		ContractID:      providerRegistryID,
		ID:              "0000003001-0000000000",
		TransactionHash: strings.Repeat("b", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "att_reg")},
		ValueXDR:        walletAddressValueXDR(t, attesterWallet),
	}

	rpc := &fakeRPC{
		getEvents: func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
			return protocol.GetEventsResponse{Events: []protocol.EventInfo{event1, event2}, LatestLedger: 602}, nil
		},
		simulate: func(envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
			var env xdr.TransactionEnvelope
			if err := xdr.SafeUnmarshalBase64(envelopeXDR, &env); err == nil && len(env.Operations()) > 0 {
				op := env.Operations()[0]
				if op.Body.Type == xdr.OperationTypeInvokeHostFunction && op.Body.InvokeHostFunctionOp != nil {
					fn := string(op.Body.InvokeHostFunctionOp.HostFunction.InvokeContract.FunctionName)
					if fn == "get_provider" {
						ret := provXDR
						return protocol.SimulateTransactionResponse{
							Results: []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &ret}},
						}, nil
					}
					if fn == "get_attester" {
						ret := attXDR
						return protocol.SimulateTransactionResponse{
							Results: []protocol.SimulateHostFunctionResult{{ReturnValueXDR: &ret}},
						}, nil
					}
				}
			}
			return protocol.SimulateTransactionResponse{}, nil
		},
	}

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID, settlementAssetID)
	runner.reconcileEvents(ctx)

	// Verify provider was ingested
	prov, err := s.GetProviderByWallet(ctx, providerWallet)
	if err != nil {
		t.Fatalf("GetProviderByWallet: %v", err)
	}
	if prov.Status != "Active" {
		t.Errorf("prov.Status = %q, want Active", prov.Status)
	}
	if string(prov.ProviderRef) != string(providerRef[:]) {
		t.Errorf("prov.ProviderRef mismatch")
	}

	// Verify attester was ingested
	att, err := s.GetAttesterByWallet(ctx, attesterWallet)
	if err != nil {
		t.Fatalf("GetAttesterByWallet: %v", err)
	}
	if att.Status != "Active" {
		t.Errorf("att.Status = %q, want Active", att.Status)
	}
	if att.ProviderWallet != providerWallet {
		t.Errorf("att.ProviderWallet = %q, want %q", att.ProviderWallet, providerWallet)
	}
}

func TestReconcileEvents_FailedEventDoesNotAdvanceCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	providerRegistryID := "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS"
	careAgreementID := "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	settlementAssetID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	// Seed an initial cursor
	initialCursor := "100"
	if err := s.SetReconciliationCursor(ctx, eventCursorKey, initialCursor); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}

	event := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          150,
		ContractID:      careAgreementID,
		ID:              "0000001500-0000000000",
		TransactionHash: strings.Repeat("e", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "agr_cre")},
		ValueXDR:        u64ValueXDR(t, 999),
	}

	// Simulation returns an error so entity fetching fails
	rpc := &fakeRPC{
		getEvents: func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
			return protocol.GetEventsResponse{Events: []protocol.EventInfo{event}, LatestLedger: 160}, nil
		},
		simulate: func(envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
			return protocol.SimulateTransactionResponse{
				Error: "Host function invocation failed",
			}, nil
		},
	}

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID, settlementAssetID)
	runner.reconcileEvents(ctx)

	// Ensure the cursor was NOT advanced to 161 (LatestLedger + 1)
	currentCursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if currentCursor != initialCursor {
		t.Errorf("cursor advanced to %q despite failed event ingestion, want %q", currentCursor, initialCursor)
	}
}

func TestReconcileEvents_ReplayRecovery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	providerRegistryID := "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS"
	careAgreementID := "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	settlementAssetID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	const rawXDR = "AAAAEQAAAAEAAAASAAAADwAAABZhdHRlc3RhdGlvbl9jb21taXRtZW50AAAAAAABAAAADwAAAAthdHRlc3RlZF9hdAAAAAABAAAADwAAAAthdHRlc3RlZF9ieQAAAAABAAAADwAAAAhhdHRlc3RlcgAAABIAAAAAAAAAAH3/zCGtzLQUGKrxls38wdh5L06xwQJSVYDbwek4BYYnAAAADwAAAA1jYXJlX2RlYWRsaW5lAAAAAAAABQAAAABqyPdcAAAADwAAAApjcmVhdGVkX2F0AAAAAAAFAAAAAGrI8+kAAAAPAAAAEWRpc3B1dGVfb3BlbmVkX2F0AAAAAAAAAQAAAA8AAAARZGlzcHV0ZV9vcGVuZWRfYnkAAAAAAAABAAAADwAAAA5kaXNwdXRlX29yaWdpbgAAAAAAEAAAAAEAAAABAAAADwAAAAROb25lAAAADwAAABNkaXNwdXRlX3dpbmRvd19zZWNzAAAAAAUAAAAAAAAASAAAAA8AAAAOZnVuZGluZ19hbW91bnQAAAAAAAoAAAAAAAAAAAAAAAAAmJaAAAAADwAAABBmdW5kaW5nX2RlYWRsaW5lAAAABQAAAABqyPbkAAAADwAAABZwYXRpZW50X3JlZl9jb21taXRtZW50AAAAAAANAAAAIBERERERERERERERERERERERERERERERERERERERERERAAAADwAAAAhwcm92aWRlcgAAABIAAAAAAAAAAGz0qyL2+h41Qg2CM5P5+WYAJbEhf3sL3HL3fbE4gaikAAAADwAAABJzZXJ2aWNlX2NvbW1pdG1lbnQAAAAAAA0AAAAgIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIAAAAPAAAAEXNldHRsZW1lbnRfYW1vdW50AAAAAAAACgAAAAAAAAAAAAAAAAB6EgAAAAAPAAAAB3Nwb25zb3IAAAAAEgAAAAAAAAAA4y3eI3Ih7LlKDamd5ddJPXSjFpPyKK8yAdUQr8ljn7kAAAAPAAAABXN0YXRlAAAAAAAAEAAAAAEAAAABAAAADwAAAAlSZXF1ZXN0ZWQAAAA="

	// Start at ledger cursor 500
	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "500"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}

	event := protocol.EventInfo{
		EventType:       "contract",
		Ledger:          500,
		ContractID:      careAgreementID,
		ID:              "0000002000-0000000000",
		TransactionHash: strings.Repeat("7", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "agr_cre")},
		ValueXDR:        u64ValueXDR(t, 7),
	}

	// Pass 1: Transient RPC simulation failure simulates network or indexer delay.
	simFail := true
	retValStr := rawXDR
	rpc := &fakeRPC{
		getEvents: func(req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
			return protocol.GetEventsResponse{Events: []protocol.EventInfo{event}, LatestLedger: 501}, nil
		},
		simulate: func(envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
			if simFail {
				return protocol.SimulateTransactionResponse{Error: "transient network timeout"}, nil
			}
			return protocol.SimulateTransactionResponse{
				Results: []protocol.SimulateHostFunctionResult{
					{ReturnValueXDR: &retValStr},
				},
			}, nil
		},
	}

	runner := New(s, rpc, testLogger(), time.Hour, providerRegistryID, careAgreementID, settlementAssetID)
	runner.reconcileEvents(ctx)

	// Ensure entity was not mirrored and cursor did NOT advance
	if _, err := s.GetAgreement(ctx, 7); err == nil {
		t.Fatal("expected error getting agreement 7 after failed pass, got nil")
	}
	cursorAfterFail, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursorAfterFail != "500" {
		t.Fatalf("cursor = %q, want 500 (must not advance on error)", cursorAfterFail)
	}

	// Pass 2: Replay and recovery without manual database intervention.
	simFail = false
	runner.reconcileEvents(ctx)

	// Agreement 7 must now be automatically mirrored in the database
	recovered, err := s.GetAgreement(ctx, 7)
	if err != nil {
		t.Fatalf("GetAgreement(7) after replay: %v (failed to recover entity)", err)
	}
	if recovered.State != "Requested" {
		t.Errorf("recovered State = %q, want Requested", recovered.State)
	}

	// Event must now be stored
	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 1 || page.Events[0].EventType != "agr_cre" {
		t.Fatalf("page = %+v, want 1 agr_cre event", page)
	}

	// Cursor must now be advanced past the replayed ledger
	cursorAfterReplay, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursorAfterReplay != "502" {
		t.Errorf("cursor after replay = %q, want 502", cursorAfterReplay)
	}
}
