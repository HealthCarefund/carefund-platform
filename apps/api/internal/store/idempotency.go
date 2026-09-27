package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrIdempotencyKeyExists means an (key_value, operation) pair was already
// inserted. Callers should fetch the existing record with
// GetIdempotencyRecord and compare request_hash: matching means "return
// the original result", differing means the caller must reject with a
// conflict.
var ErrIdempotencyKeyExists = errors.New("store: idempotency key already exists for this operation")

const uniqueViolation = "23505"

// InsertIdempotencyRecord attempts to claim a new (key, operation) pair
// with status "pending". Returns ErrIdempotencyKeyExists if it was already
// claimed — by this request racing itself, a retry, or a genuine reuse.
func (s *Store) InsertIdempotencyRecord(ctx context.Context, key, operation, requestHash string) (*IdempotencyRecord, error) {
	var r IdempotencyRecord
	err := s.pool.QueryRow(ctx, `
		INSERT INTO idempotency_keys (key_value, operation, request_hash, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, key_value, operation, request_hash, status, result_reference, created_at, updated_at
	`, key, operation, requestHash).Scan(
		&r.ID, &r.KeyValue, &r.Operation, &r.RequestHash, &r.Status, &r.ResultReference, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrIdempotencyKeyExists
		}
		return nil, fmt.Errorf("inserting idempotency record %q/%q: %w", key, operation, err)
	}
	return &r, nil
}

// GetIdempotencyRecord returns ErrNotFound if no such (key, operation) pair
// has been claimed.
func (s *Store) GetIdempotencyRecord(ctx context.Context, key, operation string) (*IdempotencyRecord, error) {
	var r IdempotencyRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, key_value, operation, request_hash, status, result_reference, created_at, updated_at
		FROM idempotency_keys
		WHERE key_value = $1 AND operation = $2
	`, key, operation).Scan(
		&r.ID, &r.KeyValue, &r.Operation, &r.RequestHash, &r.Status, &r.ResultReference, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting idempotency record %q/%q: %w", key, operation, err)
	}
	return &r, nil
}

// DeleteIdempotencyRecord removes a claimed (key, operation) pair. Used
// only to un-claim a key whose attempt never produced a workflow action
// (e.g. request validation failed before anything was created) — a
// validation failure is not a real use of the key, so a retry with a
// corrected request (even one that hashes differently) must be allowed to
// claim it fresh, not be permanently blocked by the failed attempt's hash.
func (s *Store) DeleteIdempotencyRecord(ctx context.Context, key, operation string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key_value = $1 AND operation = $2`, key, operation)
	if err != nil {
		return fmt.Errorf("deleting idempotency record %q/%q: %w", key, operation, err)
	}
	return nil
}

// UpdateIdempotencyResult records the final outcome ("completed" or
// "failed") of a previously-claimed idempotency key, so a later replay of
// the same key returns this result instead of re-running the operation.
func (s *Store) UpdateIdempotencyResult(ctx context.Context, key, operation, status string, resultReference *string) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = $3, result_reference = $4, updated_at = now()
		WHERE key_value = $1 AND operation = $2
	`, key, operation, status, resultReference)
	if err != nil {
		return fmt.Errorf("updating idempotency record %q/%q: %w", key, operation, err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
