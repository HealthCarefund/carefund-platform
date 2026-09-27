package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertProvider inserts or updates a provider's off-chain mirror row,
// keyed on wallet_address. Callers reconciling from chain state should
// call this with exactly what the contract reports.
func (s *Store) UpsertProvider(ctx context.Context, wallet string, providerRef []byte, status string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO providers (wallet_address, provider_ref, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (wallet_address) DO UPDATE
			SET provider_ref = EXCLUDED.provider_ref,
			    status = EXCLUDED.status,
			    updated_at = now()
	`, wallet, providerRef, status)
	if err != nil {
		return fmt.Errorf("upserting provider %q: %w", wallet, err)
	}
	return nil
}

// GetProviderByWallet returns ErrNotFound if no such provider is mirrored.
func (s *Store) GetProviderByWallet(ctx context.Context, wallet string) (*Provider, error) {
	var p Provider
	err := s.pool.QueryRow(ctx, `
		SELECT id, wallet_address, provider_ref, status, created_at, updated_at
		FROM providers
		WHERE wallet_address = $1
	`, wallet).Scan(&p.ID, &p.WalletAddress, &p.ProviderRef, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting provider %q: %w", wallet, err)
	}
	return &p, nil
}
