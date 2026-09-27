package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListProviders_ReturnsAllProviders(t *testing.T) {
	deps, mux := newTestDeps(t)
	providerA := "G" + repeatChar('D', 55)
	providerB := "G" + repeatChar('E', 55)
	if err := deps.Store.UpsertProvider(context.Background(), providerA, make32(0xD1), "Active"); err != nil {
		t.Fatalf("UpsertProvider(A): %v", err)
	}
	if err := deps.Store.UpsertProvider(context.Background(), providerB, make32(0xD2), "Active"); err != nil {
		t.Fatalf("UpsertProvider(B): %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body providersPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	found := map[string]bool{}
	for _, p := range body.Providers {
		found[p.WalletAddress] = true
	}
	if !found[providerA] || !found[providerB] {
		t.Errorf("Providers = %+v, want to include both seeded providers", body.Providers)
	}
}

func TestListProviders_PaginatesByCursor(t *testing.T) {
	deps, mux := newTestDeps(t)
	wallets := []string{"G" + repeatChar('K', 55), "G" + repeatChar('L', 55), "G" + repeatChar('M', 55)}
	for i, wallet := range wallets {
		if err := deps.Store.UpsertProvider(context.Background(), wallet, make32(byte(0xF1+i)), "Active"); err != nil {
			t.Fatalf("UpsertProvider: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers?limit=2", nil)
	mux.ServeHTTP(rec, req)
	var page1 providersPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decoding page1: %v", err)
	}
	if page1.NextCursor == "" {
		t.Fatalf("page1 = %+v, want a next cursor with 3+ providers seeded and limit=2", page1)
	}
}

func TestListAttesters_ReturnsAllAttesters(t *testing.T) {
	deps, mux := newTestDeps(t)
	provider := "G" + repeatChar('H', 55)
	if err := deps.Store.UpsertProvider(context.Background(), provider, make32(0xC1), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	attesterA := "G" + repeatChar('I', 55)
	attesterB := "G" + repeatChar('J', 55)
	if err := deps.Store.UpsertAttester(context.Background(), attesterA, provider, make32(0xC2), "Active"); err != nil {
		t.Fatalf("UpsertAttester(A): %v", err)
	}
	if err := deps.Store.UpsertAttester(context.Background(), attesterB, provider, make32(0xC3), "Active"); err != nil {
		t.Fatalf("UpsertAttester(B): %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/attesters", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body attestersPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	found := map[string]bool{}
	for _, a := range body.Attesters {
		found[a.WalletAddress] = true
	}
	if !found[attesterA] || !found[attesterB] {
		t.Errorf("Attesters = %+v, want to include both seeded attesters", body.Attesters)
	}
}
