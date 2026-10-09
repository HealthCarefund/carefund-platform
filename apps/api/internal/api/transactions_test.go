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
// to Testnet for Block 3B verification. Not secrets: contract
// IDs and public keys are, by design, public on-chain identifiers.
// provider-registry: CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS
// care-agreement deploy tx: dbcfa8a6ad27b78a3a0631b6041b744ba68f90320371d1a628f50949c630d7da
// Agreement id 1 on this contract completed a full real lifecycle
// (created, funded, attested, settled). Agreement id 6 was created separately
// and left in Requested state, unfunded, specifically for this test.
const (
	liveCareAgreementContractID = "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	liveProviderAddress         = "GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453"
	liveAttesterAddress         = "GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT"
	liveSponsorAddress          = "GDRS3XRDOIQ6ZOKKBWUZ3ZOXJE6XJIYWSPZCRLZSAHKRBL6JMOP3SYPH"
	liveNetworkPassphrase       = "Test SDF Network ; September 2015"
	liveRPCURL                  = "https://soroban-testnet.stellar.org"
	liveSeededAgreementID       = int64(6)
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
	// No RPC is configured in these deps at all - if the handler tried to
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

func TestBuildOperationArgs_SettleIsPermissionless(t *testing.T) {
	agreement := sampleAgreement(206)
	thirdParty := "G" + repeatChar('B', 55)
	args, issues := buildOperationArgs(206, &agreement, prepareTransactionRequest{
		Operation:       operationSettle,
		SourcePublicKey: thirdParty,
	})
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 ScVal argument for settle, got %d", len(args))
	}
}

// TestPrepareTransaction_FundLiveTestnet exercises the full
// build->simulate->assemble pipeline against the real care-agreement
// contract deployed to Testnet and a genuine on-chain agreement (id 1,
// created for this phase's verification, still in Requested state) - a
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

	rec := postJSON(t, mux, "/api/v1/agreements/6/transactions", prepareTransactionRequest{
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
