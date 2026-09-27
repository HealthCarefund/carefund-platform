// Package store is the PostgreSQL data-access layer for CareFund's
// off-chain workflow metadata. Every field here mirrors either an on-chain
// value (kept in sync by reconciliation, never authoritative on its own)
// or purely application-local bookkeeping (idempotency, audit, transaction
// lifecycle) with no on-chain counterpart at all. Nothing here is, or may
// ever become, clinical/PHI data.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned by single-row lookups that find nothing.
var ErrNotFound = errors.New("store: not found")

// Store wraps the database pool with CareFund's repository methods.
type Store struct {
	pool *pgxpool.Pool
}

// New builds a Store over an already-connected pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
