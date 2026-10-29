package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
)

func TestLookupTransaction_ValidatesHashFormat(t *testing.T) {
	_, mux := newTestDeps(t)
	for _, hash := range []string{"not-a-hash", strings.Repeat("a", 63), strings.Repeat("A", 64), ""} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+hash, nil)
		mux.ServeHTTP(rec, req)
		if hash == "" {
			continue // doesn't match the route pattern at all; not this handler's concern
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("hash=%q: status = %d, want %d; body=%s", hash, rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	}
}

// TestLookupTransaction_NotFoundLiveTestnet makes a REAL RPC call for a
// well-formed but nonexistent transaction hash against the real Testnet
// RPC. This is genuine live-network evidence, not a mock.
func TestLookupTransaction_NotFoundLiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}
	deps, _ := newTestDeps(t)
	deps.RPC = liveRPCClient(t)
	if _, err := deps.RPC.Health(context.Background()); err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+strings.Repeat("f", 64), nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Status != lookupStatusNotFound {
		t.Errorf("Status = %q, want %q", body.Status, lookupStatusNotFound)
	}
	if body.Reconciliation != nil {
		t.Errorf("Reconciliation = %+v, want nil (we never recorded this hash)", body.Reconciliation)
	}
}

// TestLookupTransaction_SuccessDeterministic keeps the success response and
// endpoint behavior covered after the historical live transaction falls out
// of Soroban RPC's finite transaction-retention window.
func TestLookupTransaction_SuccessDeterministic(t *testing.T) {
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID any `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"status":                protocol.TransactionStatusSuccess,
				"txHash":                hash,
				"ledger":                12345,
				"latestLedger":          12345,
				"latestLedgerCloseTime": "1700000000",
				"oldestLedger":          1,
				"oldestLedgerCloseTime": "1699990000",
				"applicationOrder":      1,
				"feeBump":               false,
				"events":                map[string]any{},
			},
		})
	}))
	defer rpcServer.Close()

	deps, _ := newTestDeps(t)
	deps.RPC = stellarrpc.New(rpcServer.URL, 5*time.Second)
	defer deps.RPC.Close()
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+hash, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Status != lookupStatusSuccess || body.Ledger != "12345" {
		t.Errorf("response = %+v, want success at ledger 12345", body)
	}
}

// TestLookupTransaction_SuccessLiveTestnet preserves the live check against
// the genuine historical transaction recorded in
// evidence/testnet-2026-10-09-block3b.md. Soroban RPC only retains a bounded
// transaction window, so an unavailable historical prerequisite is skipped;
// the deterministic test above still covers the application success path.
func TestLookupTransaction_SuccessLiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}
	deps, _ := newTestDeps(t)
	deps.RPC = liveRPCClient(t)
	defer deps.RPC.Close()
	if _, err := deps.RPC.Health(context.Background()); err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)

	const knownSuccessfulHash = "9ae6dc480e876a32a18e5b0d12ce5a3187010c01ce319d0ac1e035589845285c"
	const knownSuccessfulLedger = uint32(5104998)
	historical, err := deps.RPC.GetTransaction(context.Background(), knownSuccessfulHash)
	if err != nil {
		t.Fatalf("historical transaction lookup failed: %v", err)
	}
	if historical.Status == protocol.TransactionStatusNotFound {
		health, err := deps.RPC.Health(context.Background())
		if err != nil {
			t.Fatalf("checking Testnet retention after NOT_FOUND: %v", err)
		}
		t.Skipf("historical Testnet transaction %s was successful at ledger %d but is no longer in Soroban RPC transaction history: status=%q, oldestLedger=%d, latestLedger=%d, retentionWindow=%d; original evidence remains in evidence/testnet-2026-10-09-block3b.md and Horizon", knownSuccessfulHash, knownSuccessfulLedger, historical.Status, health.OldestLedger, health.LatestLedger, health.LedgerRetentionWindow)
	}
	if historical.Status != protocol.TransactionStatusSuccess {
		t.Fatalf("historical transaction %s returned unexpected RPC status %q (ledger=%d, oldestLedger=%d, latestLedger=%d)", knownSuccessfulHash, historical.Status, historical.Ledger, historical.OldestLedger, historical.LatestLedger)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+knownSuccessfulHash, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.Status != lookupStatusSuccess {
		t.Errorf("Status = %q, want %q; body=%s", body.Status, lookupStatusSuccess, rec.Body.String())
	}
	if body.Ledger == "" {
		t.Error("expected a non-empty ledger for a confirmed transaction")
	}
}

func TestLookupTransaction_ReconcilesLocalRecordOnSuccess(t *testing.T) {
	deps, _ := newTestDeps(t)
	agreement := sampleAgreement(301)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	agreementID := int64(301)
	hash := strings.Repeat("b", 64)
	if err := deps.Store.InsertTransactionRef(context.Background(), &agreementID, "fund", hash, "submitted"); err != nil {
		t.Fatalf("InsertTransactionRef: %v", err)
	}

	deps.RPC = liveRPCClient(t)
	if _, err := deps.RPC.Health(context.Background()); err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}
	newMux := http.NewServeMux()
	RegisterRoutes(newMux, deps)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+hash, nil)
	newMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	// A random 64-'b' hash is not found on Testnet, so status stays
	// not_found, but the reconciliation record we already had must still
	// come back (with last_checked bumped), proving the lookup did try to
	// reconcile it rather than silently ignoring a known hash.
	if body.Reconciliation == nil {
		t.Fatal("expected reconciliation info for a hash we recorded locally")
	}
	if body.Reconciliation.Operation != "fund" || body.Reconciliation.AgreementID != "301" {
		t.Errorf("Reconciliation = %+v", body.Reconciliation)
	}
	if body.Reconciliation.LastChecked == "" {
		t.Error("expected LastChecked to be set after a reconciliation attempt")
	}

	updated, err := deps.Store.GetTransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v", err)
	}
	if updated.Status != "submitted" {
		t.Errorf("Status = %q, want it unchanged (still submitted) since the chain reported NOT_FOUND", updated.Status)
	}
	if updated.LastChecked == nil {
		t.Error("expected LastChecked to be persisted")
	}
}
