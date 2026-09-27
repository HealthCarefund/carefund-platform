package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// These constants are the real contracts and accounts deployed/registered
// to Testnet for Block 1 verification (Phase 10). Not secrets: contract
// IDs and public keys are, by design, public on-chain identifiers.
// provider-registry: CCGF5Y7CYRJSCMXE7NRCAXY4CKYWY22BXWOYILFXZD32EXFHOOTLNMSG
// care-agreement deploy tx: ce155a193b5efdbfd32ca9fc1e5aba93a43bb1a278f3ff4d68565ab964db80f2
// Agreement id 1 on this contract completed a full real lifecycle
// (created, funded, attested, settled). Agreement id 2 (referenced
// below) was created separately and left in Requested state, unfunded,
// specifically for this test.
const (
	liveCareAgreementContractID = "CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS"
	liveProviderAddress         = "GARYN44ZRCQYNXHR6VEB6ZLJWAMSF7SAO6CDCM6NQHOXMCSKJ54UCJBP"
	liveAttesterAddress         = "GBD5ORGLBELP4MSHDHKOXSRP7BEKTO63T4JO3O2DAXBZSTJVTGMFZFGG"
	liveSponsorAddress          = "GDEI32FXSM6XIOKHUXT43MOB7GTKRRZVBNVVCW6XE6DXXKL7J3LKCI3S"
	liveNetworkPassphrase       = "Test SDF Network ; September 2015"
	liveRPCURL                  = "https://soroban-testnet.stellar.org"
	liveSeededAgreementID       = int64(2)
)

func liveTestnetAgreement() store.CareAgreement {
	return store.CareAgreement{
		AgreementID:               liveSeededAgreementID,
		SponsorWallet:             liveSponsorAddress,
		ProviderWallet:            liveProviderAddress,
		AttesterWallet:            liveAttesterAddress,
		PatientRefCommitment:      make32(0x01),
		ServiceCommitment:         make32(0x02),
		FundingAmount:             "1000000",
		SettlementAmount:          "900000",
		SettlementAssetContractID: "C" + repeatChar('X', 55),
		FundingDeadline:           time.Now().Add(time.Hour).Unix(),
		CareDeadline:              time.Now().Add(2 * time.Hour).Unix(),
		DisputeWindowSecs:         86_400,
		State:                     "Requested",
	}
}

// newLiveTestDeps builds Deps against the real Testnet RPC, for the one
// test in this file that genuinely needs it. It skips cleanly if the DB or
// Testnet aren't reachable.
func newLiveTestDeps(t *testing.T) (Deps, *http.ServeMux) {
	t.Helper()
	deps, _ := newTestDeps(t)
	deps.Config = &config.Config{
		StellarNetwork:           config.NetworkTestnet,
		StellarNetworkPassphrase: liveNetworkPassphrase,
		CareAgreementContractID:  liveCareAgreementContractID,
	}
	deps.RPC = liveRPCClient(t)

	newMux := http.NewServeMux()
	RegisterRoutes(newMux, deps)
	return deps, newMux
}

func TestPrepareTransaction_AgreementNotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	rec := postJSON(t, mux, "/api/v1/agreements/999999/transactions", prepareTransactionRequest{
		Operation:       operationFund,
		SourcePublicKey: liveSponsorAddress,
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestPrepareTransaction_UnknownOperationRejected(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(201)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	rec := postJSON(t, mux, "/api/v1/agreements/201/transactions", prepareTransactionRequest{
		Operation:       "not_a_real_operation",
		SourcePublicKey: agreement.SponsorWallet,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPrepareTransaction_WalletMismatchRejectedWithoutCallingRPC(t *testing.T) {
	// No RPC is configured in these deps at all — if the handler tried to
	// call it before validating the wallet, this would panic on a nil
	// pointer instead of returning 400.
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(202)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rec := postJSON(t, mux, "/api/v1/agreements/202/transactions", prepareTransactionRequest{
		Operation:       operationFund,
		SourcePublicKey: agreement.ProviderWallet, // fund requires the sponsor, not the provider
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPrepareTransaction_AttestCareRequiresCommitment(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(203)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	rec := postJSON(t, mux, "/api/v1/agreements/203/transactions", prepareTransactionRequest{
		Operation:       operationAttestCare,
		SourcePublicKey: agreement.AttesterWallet,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPrepareTransaction_ResolveDisputeRequiresValidResolution(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(204)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	rec := postJSON(t, mux, "/api/v1/agreements/204/transactions", prepareTransactionRequest{
		Operation:       operationResolveDispute,
		SourcePublicKey: "G" + repeatChar('A', 55),
		Resolution:      "NotAReal Resolution",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPrepareTransaction_RejectsUnknownFields(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(205)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	body := []byte(`{"operation":"fund","sourcePublicKey":"G","unexpected":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agreements/205/transactions", bytes.NewReader(body))
	req.Header.Set(idempotencyKeyHeader, uniqueIdempotencyKey(t))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestPrepareTransaction_FundLiveTestnet exercises the full
// build->simulate->assemble pipeline against the real care-agreement
// contract deployed to Testnet and a genuine on-chain agreement (id 1,
// created for this phase's verification, still in Requested state) — a
// real RPC round trip, not a mock. It only prepares the transaction; it is
// never signed or submitted here, so it makes no on-chain change.
func TestPrepareTransaction_FundLiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}
	deps, mux := newLiveTestDeps(t)
	if _, err := deps.RPC.Health(context.Background()); err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}
	agreement := liveTestnetAgreement()
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rec := postJSON(t, mux, "/api/v1/agreements/2/transactions", prepareTransactionRequest{
		Operation:       operationFund,
		SourcePublicKey: liveSponsorAddress,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body prepareTransactionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.UnsignedTransactionXDR == "" {
		t.Error("expected a non-empty unsigned transaction XDR")
	}
	if body.NetworkPassphrase != liveNetworkPassphrase {
		t.Errorf("NetworkPassphrase = %q", body.NetworkPassphrase)
	}
}

func postJSON(t *testing.T, mux http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(idempotencyKeyHeader, uniqueIdempotencyKey(t))
	mux.ServeHTTP(rec, req)
	return rec
}

func liveRPCClient(t *testing.T) *stellarrpc.Client {
	t.Helper()
	return stellarrpc.New(liveRPCURL, 20*time.Second)
}
