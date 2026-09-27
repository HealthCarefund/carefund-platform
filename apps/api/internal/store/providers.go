package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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

// ProviderPage is one page of a cursor-paginated provider directory
// listing. NextCursor is empty when there is no further page.
type ProviderPage struct {
	Providers  []Provider
	NextCursor string
}

// ListProviders returns every provider mirrored off-chain, ordered by id
// for stable pagination — the public provider directory admin view.
func (s *Store) ListProviders(ctx context.Context, cursor int64, limit int) (ProviderPage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, wallet_address, provider_ref, status, created_at, updated_at
		FROM providers
		WHERE id > $1
		ORDER BY id
		LIMIT $2
	`, cursor, limit)
	if err != nil {
		return ProviderPage{}, fmt.Errorf("listing providers: %w", err)
	}
	defer rows.Close()

	var page ProviderPage
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.ID, &p.WalletAddress, &p.ProviderRef, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return ProviderPage{}, fmt.Errorf("scanning provider row: %w", err)
		}
		page.Providers = append(page.Providers, p)
	}
	if err := rows.Err(); err != nil {
		return ProviderPage{}, fmt.Errorf("iterating providers: %w", err)
	}
	if len(page.Providers) == limit {
		page.NextCursor = strconv.FormatInt(page.Providers[len(page.Providers)-1].ID, 10)
	}
	return page, nil
}
