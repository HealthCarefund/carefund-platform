package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
