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
