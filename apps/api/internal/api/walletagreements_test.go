package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProviderAgreements_ValidatesWallet(t *testing.T) {
	_, mux := newTestDeps(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/not-a-wallet/agreements", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestListProviderAgreements_ProviderNotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	wallet := "G" + repeatChar('Z', 55)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet+"/agreements", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestListProviderAgreements_ReturnsOnlyThatProvidersAgreements(t *testing.T) {
	deps, mux := newTestDeps(t)
	providerA := "G" + repeatChar('A', 55)
	providerB := "G" + repeatChar('B', 55)
	if err := deps.Store.UpsertProvider(context.Background(), providerA, make32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider(A): %v", err)
	}
	if err := deps.Store.UpsertProvider(context.Background(), providerB, make32(0xBB), "Active"); err != nil {
		t.Fatalf("UpsertProvider(B): %v", err)
	}

	agreementForA := sampleAgreement(401)
	agreementForA.ProviderWallet = providerA
	agreementForB := sampleAgreement(402)
	agreementForB.ProviderWallet = providerB
	if err := deps.Store.UpsertAgreement(context.Background(), agreementForA); err != nil {
		t.Fatalf("UpsertAgreement(A): %v", err)
	}
	if err := deps.Store.UpsertAgreement(context.Background(), agreementForB); err != nil {
		t.Fatalf("UpsertAgreement(B): %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+providerA+"/agreements", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body agreementsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Agreements) != 1 || body.Agreements[0].AgreementID != "401" {
		t.Errorf("Agreements = %+v, want exactly agreement 401", body.Agreements)
	}
}

func TestListSponsorAgreements_NoExistenceCheckRequired(t *testing.T) {
	_, mux := newTestDeps(t)
	// A sponsor wallet that has never sponsored anything and was never
	// separately registered anywhere — this must be 200 with an empty
	// list, not 404: any wallet may sponsor an agreement.
	wallet := "G" + repeatChar('S', 55)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsors/"+wallet+"/agreements", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body agreementsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Agreements) != 0 {
		t.Errorf("Agreements = %+v, want empty", body.Agreements)
	}
}

func TestListSponsorAgreements_ReturnsSponsoredAgreements(t *testing.T) {
	deps, mux := newTestDeps(t)
	sponsor := "G" + repeatChar('P', 55)
	agreement := sampleAgreement(403)
	agreement.SponsorWallet = sponsor
	if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sponsors/"+sponsor+"/agreements", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body agreementsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body.Agreements) != 1 || body.Agreements[0].AgreementID != "403" {
		t.Errorf("Agreements = %+v, want exactly agreement 403", body.Agreements)
	}
}

func TestListAgreements_PaginatesByCursor(t *testing.T) {
	deps, mux := newTestDeps(t)
	provider := "G" + repeatChar('Q', 55)
	if err := deps.Store.UpsertProvider(context.Background(), provider, make32(0xCC), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	for _, id := range []int64{410, 411, 412} {
		agreement := sampleAgreement(id)
		agreement.ProviderWallet = provider
		if err := deps.Store.UpsertAgreement(context.Background(), agreement); err != nil {
			t.Fatalf("UpsertAgreement(%d): %v", id, err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+provider+"/agreements?limit=2", nil)
	mux.ServeHTTP(rec, req)
	var page1 agreementsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decoding page1: %v", err)
	}
	if len(page1.Agreements) != 2 || page1.NextCursor == "" {
		t.Fatalf("page1 = %+v, want 2 agreements and a next cursor", page1)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+provider+"/agreements?limit=2&cursor="+page1.NextCursor, nil)
	mux.ServeHTTP(rec, req)
	var page2 agreementsPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decoding page2: %v", err)
	}
	if len(page2.Agreements) != 1 || page2.NextCursor != "" {
		t.Fatalf("page2 = %+v, want exactly 1 agreement and no further cursor", page2)
	}
}
