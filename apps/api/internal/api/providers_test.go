package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/migrate"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// These are genuine integration tests against the isolated CareFund
// PostgreSQL 18.6 Docker instance (carefund-pg18, port 5439) — never the
// host's PostgreSQL 16. They skip cleanly if it isn't reachable.

func testDatabaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://carefund:carefund_dev@localhost:5439/carefund"
}

func newTestDeps(t *testing.T) (Deps, *http.ServeMux) {
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
			contract_events, attestations, care_agreements, attesters, providers
			RESTART IDENTITY CASCADE`)
		pool.Close()
	})

	deps := Deps{
		Store:  store.New(pool),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	return deps, mux
}

func TestGetProvider_ValidatesWalletFormat(t *testing.T) {
	_, mux := newTestDeps(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/not-a-wallet", nil)

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	wallet := "G" + repeatChar('A', 55)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestGetProvider_Found(t *testing.T) {
	deps, mux := newTestDeps(t)
	wallet := "G" + repeatChar('B', 55)
	if err := deps.Store.UpsertProvider(context.Background(), wallet, make32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body providerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.WalletAddress != wallet || body.Status != "Active" {
		t.Errorf("body = %+v", body)
	}
	if len(body.ProviderRef) != 64 { // 32 bytes hex-encoded
		t.Errorf("ProviderRef = %q, want 64 hex chars", body.ProviderRef)
	}
}

func TestListProviderAttesters_ProviderNotFound(t *testing.T) {
	_, mux := newTestDeps(t)
	wallet := "G" + repeatChar('C', 55)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet+"/attesters", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestListProviderAttesters_ReturnsEmptyListNotNull(t *testing.T) {
	deps, mux := newTestDeps(t)
	wallet := "G" + repeatChar('D', 55)
	if err := deps.Store.UpsertProvider(context.Background(), wallet, make32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet+"/attesters", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != "[]\n" {
		t.Errorf("body = %q, want an empty JSON array, not null", rec.Body.String())
	}
}

func TestListProviderAttesters_ReturnsAttesters(t *testing.T) {
	deps, mux := newTestDeps(t)
	wallet := "G" + repeatChar('E', 55)
	attesterWallet := "G" + repeatChar('F', 55)
	if err := deps.Store.UpsertProvider(context.Background(), wallet, make32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	if err := deps.Store.UpsertAttester(context.Background(), attesterWallet, wallet, make32(0xBB), "Active"); err != nil {
		t.Fatalf("UpsertAttester: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/"+wallet+"/attesters", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body []attesterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body) != 1 || body[0].WalletAddress != attesterWallet {
		t.Errorf("body = %+v", body)
	}
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func make32(b byte) []byte {
	h := make([]byte, 32)
	for i := range h {
		h[i] = b
	}
	return h
}
