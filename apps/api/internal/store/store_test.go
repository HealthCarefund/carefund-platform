package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/migrate"
)

// testDatabaseURL points at the isolated CareFund PostgreSQL 18.6 Docker
// instance (see docker-compose.yml / TOOLCHAIN.md) — never at the host's
// PostgreSQL 16 install. These tests are genuine integration tests against
// that real instance, not a mock: if it isn't reachable, they skip rather
// than fail, so `go test ./...` still passes in an environment without it.
//
// This package and internal/api both truncate shared tables in that same
// live instance as part of per-test cleanup. Go runs different packages'
// test binaries concurrently by default, so running the two against the
// database at once races: one package's cleanup can truncate rows the
// other package's in-flight test still depends on, surfacing as
// mysterious foreign-key violations or "0 rows" failures that vary
// between runs. Always run the full suite as `go test -p 1 ./...` (as
// this repo's CI does) to serialize package execution and avoid this.
func testDatabaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://carefund:carefund_dev@localhost:5439/carefund"
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDatabaseURL())
	if err != nil {
		t.Skipf("could not create pool for %s: %v", testDatabaseURL(), err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("isolated CareFund Postgres 18.6 instance not reachable at %s: %v", testDatabaseURL(), err)
	}

	if _, err := migrate.Run(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("running migrations: %v", err)
	}

	t.Cleanup(func() {
		truncateAll(t, pool)
		pool.Close()
	})

	return New(pool)
}

func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `TRUNCATE TABLE
		audit_records, idempotency_keys, contract_events, transaction_refs,
		attestations, care_agreements, attesters, providers
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Errorf("truncating test tables: %v", err)
	}
}

func hash32(b byte) []byte {
	h := make([]byte, 32)
	for i := range h {
		h[i] = b
	}
	return h
}
