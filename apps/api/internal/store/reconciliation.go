package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetReconciliationCursor returns ErrNotFound if this key has never been
// recorded (e.g. the very first run).
func (s *Store) GetReconciliationCursor(ctx context.Context, key string) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx, `SELECT value FROM reconciliation_state WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("getting reconciliation cursor %q: %w", key, err)
	}
	return value, nil
}

// SetReconciliationCursor persists progress so a process restart resumes
// from here instead of re-scanning everything (or missing a gap).
func (s *Store) SetReconciliationCursor(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO reconciliation_state (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, key, value)
	if err != nil {
		return fmt.Errorf("setting reconciliation cursor %q: %w", key, err)
	}
	return nil
}

// AcknowledgeReconciliationGap atomically records an operator's approved
// resume position and the corresponding acknowledgement. Both values are
// compared with the caller's read before either row is changed, so a stale
// operator action cannot overwrite newer reconciliation state.
func (s *Store) AcknowledgeReconciliationGap(ctx context.Context, gapKey, cursorKey, expectedGap, expectedCursor, newCursor, acknowledgedGap string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning reconciliation gap acknowledgement: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentGap string
	if err := tx.QueryRow(ctx, `
		SELECT value FROM reconciliation_state WHERE key = $1 FOR UPDATE
	`, gapKey).Scan(&currentGap); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("locking reconciliation gap: %w", err)
	}
	if currentGap != expectedGap {
		return fmt.Errorf("reconciliation gap changed while it was being acknowledged")
	}

	var currentCursor string
	if err := tx.QueryRow(ctx, `
		SELECT value FROM reconciliation_state WHERE key = $1 FOR UPDATE
	`, cursorKey).Scan(&currentCursor); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("locking reconciliation cursor: %w", err)
	}
	if currentCursor != expectedCursor {
		return fmt.Errorf("reconciliation cursor changed while the gap was being acknowledged")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reconciliation_state SET value = $2, updated_at = now() WHERE key = $1
	`, cursorKey, newCursor); err != nil {
		return fmt.Errorf("updating reconciliation cursor: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE reconciliation_state SET value = $2, updated_at = now() WHERE key = $1
	`, gapKey, acknowledgedGap); err != nil {
		return fmt.Errorf("recording reconciliation acknowledgement: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing reconciliation gap acknowledgement: %w", err)
	}
	return nil
}
