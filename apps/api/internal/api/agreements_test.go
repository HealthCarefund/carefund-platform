package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

func sampleAgreement(id int64) store.CareAgreement {
	return store.CareAgreement{
		AgreementID:               id,
		SponsorWallet:             "G" + repeatChar('S', 55),
		ProviderWallet:            "G" + repeatChar('P', 55),
		AttesterWallet:            "G" + repeatChar('T', 55),
		PatientRefCommitment:      make32(0x01),
		ServiceCommitment:         make32(0x02),
		FundingAmount:             "1000000",
		SettlementAmount:          "900000",
		SettlementAssetContractID: "C" + repeatChar('X', 55),
		FundingDeadline:           1_700_000_000,
		CareDeadline:              1_700_100_000,
		DisputeWindowSecs:         86_400,
		State:                     "Requested",
	}
}

func TestGetAgreement_ValidatesIDFormat(t *testing.T) {
	_, mux := newTestDeps(t)
	for _, id := range []string{"not-a-number", "-1", "01", ""} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/"+id, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			// An empty id segment doesn't match the route pattern at all,
			// which net/http reports as 404; every other malformed id must
			// be rejected by our own validation as 400.
			t.Errorf("id=%q: status = %d, want 400 (or 404 for an empty segment)", id, rec.Code)
		}
		if id != "" && rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: status = %d, want %d", id, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestGetAgreement_NotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/999999", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestGetAgreement_Found(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(101)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/101", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body agreementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.AgreementID != "101" || body.State != "Requested" || body.FundingAmount != "1000000" {
		t.Errorf("body = %+v", body)
	}
}

func TestGetAgreementEvents_AgreementNotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/424242/events", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetAgreementEvents_ValidatesCursorAndLimit(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(102)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	for _, q := range []string{"cursor=not-a-number", "cursor=-1", "limit=0", "limit=abc"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/102/events?"+q, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("query=%q: status = %d, want %d", q, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestGetAgreementEvents_PaginatesAndReturnsEmptyListNotNull(t *testing.T) {
	deps, mux := newTestDeps(t)
	agreement := sampleAgreement(103)
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	// No events yet: must be [] not null.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/103/events", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var empty eventsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if empty.Events == nil || len(empty.Events) != 0 {
		t.Errorf("Events = %#v, want an empty non-nil slice", empty.Events)
	}

	for i := 0; i < 3; i++ {
		agreementID := int64(103)
		hash := make([]byte, 64)
		for j := range hash {
			hash[j] = byte('a' + i)
		}
		_, err := deps.Store.InsertContractEvent(context.Background(), store.ContractEvent{
			ContractID:  "Ccareagreement",
			TxHash:      string(hash),
			EventIndex:  0,
			EventType:   "agr_evt",
			AgreementID: &agreementID,
			Ledger:      int64(200 + i),
		})
		if err != nil {
			t.Fatalf("InsertContractEvent(%d): %v", i, err)
		}
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agreements/103/events?limit=2", nil)
	mux.ServeHTTP(rec, req)
	var page1 eventsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decoding page 1: %v", err)
	}
	if len(page1.Events) != 2 || page1.NextCursor == "" {
		t.Fatalf("page1 = %+v, want 2 events and a next cursor", page1)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agreements/103/events?limit=2&cursor="+page1.NextCursor, nil)
	mux.ServeHTTP(rec, req)
	var page2 eventsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decoding page 2: %v", err)
	}
	if len(page2.Events) != 1 || page2.NextCursor != "" {
		t.Fatalf("page2 = %+v, want exactly 1 event and no further cursor", page2)
	}
}

func TestGetAgreement_OnDemandFallback(t *testing.T) {
	const rawXDR = "AAAAEQAAAAEAAAASAAAADwAAABZhdHRlc3RhdGlvbl9jb21taXRtZW50AAAAAAABAAAADwAAAAthdHRlc3RlZF9hdAAAAAABAAAADwAAAAthdHRlc3RlZF9ieQAAAAABAAAADwAAAAhhdHRlc3RlcgAAABIAAAAAAAAAAH3/zCGtzLQUGKrxls38wdh5L06xwQJSVYDbwek4BYYnAAAADwAAAA1jYXJlX2RlYWRsaW5lAAAAAAAABQAAAABqyPdcAAAADwAAAApjcmVhdGVkX2F0AAAAAAAFAAAAAGrI8+kAAAAPAAAAEWRpc3B1dGVfb3BlbmVkX2F0AAAAAAAAAQAAAA8AAAARZGlzcHV0ZV9vcGVuZWRfYnkAAAAAAAABAAAADwAAAA5kaXNwdXRlX29yaWdpbgAAAAAAEAAAAAEAAAABAAAADwAAAAROb25lAAAADwAAABNkaXNwdXRlX3dpbmRvd19zZWNzAAAAAAUAAAAAAAAASAAAAA8AAAAOZnVuZGluZ19hbW91bnQAAAAAAAoAAAAAAAAAAAAAAAAAmJaAAAAADwAAABBmdW5kaW5nX2RlYWRsaW5lAAAABQAAAABqyPbkAAAADwAAABZwYXRpZW50X3JlZl9jb21taXRtZW50AAAAAAANAAAAIBERERERERERERERERERERERERERERERERERERERERERAAAADwAAAAhwcm92aWRlcgAAABIAAAAAAAAAAGz0qyL2+h41Qg2CM5P5+WYAJbEhf3sL3HL3fbE4gaikAAAADwAAABJzZXJ2aWNlX2NvbW1pdG1lbnQAAAAAAA0AAAAgIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIAAAAPAAAAEXNldHRsZW1lbnRfYW1vdW50AAAAAAAACgAAAAAAAAAAAAAAAAB6EgAAAAAPAAAAB3Nwb25zb3IAAAAAEgAAAAAAAAAA4y3eI3Ih7LlKDamd5ddJPXSjFpPyKK8yAdUQr8ljn7kAAAAPAAAABXN0YXRlAAAAAAAAEAAAAAEAAAABAAAADwAAAAlSZXF1ZXN0ZWQAAAA="

	// Mock JSON-RPC server simulating Soroban RPC simulateTransaction
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"results": []map[string]any{
					{"xdr": rawXDR},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer rpcServer.Close()

	deps, _ := newTestDeps(t)
	deps.Config = &config.Config{
		CareAgreementContractID:    "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O",
		ProviderRegistryContractID: "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS",
		SettlementAssetContractID:  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
		StellarNetwork:             "TESTNET",
		StellarNetworkPassphrase:   "Test SDF Network ; September 2015",
	}
	deps.RPC = stellarrpc.New(rpcServer.URL, 5*time.Second)
	defer deps.RPC.Close()

	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)

	// Agreement 7 is not yet in the DB. Requesting it triggers on-demand fallback.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/7", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first GetAgreement(7): status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp1 agreementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp1.AgreementID != "7" || resp1.State != "Requested" {
		t.Errorf("resp1 = %+v, want agreement 7 in Requested state", resp1)
	}

	// Verify agreement was persisted in PostgreSQL store
	inStore, err := deps.Store.GetAgreement(context.Background(), 7)
	if err != nil {
		t.Fatalf("Store.GetAgreement(7): %v", err)
	}
	if inStore.State != "Requested" {
		t.Errorf("inStore.State = %q, want Requested", inStore.State)
	}

	// Second request must return HTTP 200 consistently without creating duplicate records or conflicting state
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/agreements/7", nil)
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second GetAgreement(7): status = %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
}
