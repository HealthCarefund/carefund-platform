package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertAttester inserts or updates an attester's off-chain mirror row.
func (s *Store) UpsertAttester(ctx context.Context, wallet, providerWallet string, credentialRef []byte, status string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO attesters (wallet_address, provider_wallet, credential_ref, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (wallet_address) DO UPDATE
			SET provider_wallet = EXCLUDED.provider_wallet,
			    credential_ref = EXCLUDED.credential_ref,
			    status = EXCLUDED.status,
			    updated_at = now()
	`, wallet, providerWallet, credentialRef, status)
	if err != nil {
		return fmt.Errorf("upserting attester %q: %w", wallet, err)
	}
	return nil
}

// GetAttesterByWallet returns ErrNotFound if no such attester is mirrored.
func (s *Store) GetAttesterByWallet(ctx context.Context, wallet string) (*Attester, error) {
	var a Attester
	err := s.pool.QueryRow(ctx, `
		SELECT id, wallet_address, provider_wallet, credential_ref, status, created_at, updated_at
		FROM attesters
		WHERE wallet_address = $1
	`, wallet).Scan(&a.ID, &a.WalletAddress, &a.ProviderWallet, &a.CredentialRef, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting attester %q: %w", wallet, err)
	}
	return &a, nil
}

// ListAttestersByProvider returns every attester mirrored under a
// provider, ordered by wallet address for stable pagination-free listing
// (attester counts per provider are expected to be small).
func (s *Store) ListAttestersByProvider(ctx context.Context, providerWallet string) ([]Attester, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, wallet_address, provider_wallet, credential_ref, status, created_at, updated_at
		FROM attesters
		WHERE provider_wallet = $1
		ORDER BY wallet_address
	`, providerWallet)
	if err != nil {
		return nil, fmt.Errorf("listing attesters for provider %q: %w", providerWallet, err)
	}
	defer rows.Close()

	var result []Attester
	for rows.Next() {
		var a Attester
		if err := rows.Scan(&a.ID, &a.WalletAddress, &a.ProviderWallet, &a.CredentialRef, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning attester row: %w", err)
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating attesters for provider %q: %w", providerWallet, err)
	}
	return result, nil
}
