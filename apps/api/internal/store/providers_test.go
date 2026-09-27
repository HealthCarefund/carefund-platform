package store

import (
	"context"
	"errors"
	"testing"
)

func TestProviders_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	wallet := "GPROVIDER1234567890123456789012345678901234567890AB"

	if err := s.UpsertProvider(ctx, wallet, hash32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	got, err := s.GetProviderByWallet(ctx, wallet)
	if err != nil {
		t.Fatalf("GetProviderByWallet: %v", err)
	}
	if got.WalletAddress != wallet || got.Status != "Active" {
		t.Errorf("got %+v", got)
	}
	if len(got.ProviderRef) != 32 {
		t.Errorf("ProviderRef length = %d, want 32", len(got.ProviderRef))
	}
}

func TestProviders_UpsertUpdatesExistingRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	wallet := "GPROVIDER1234567890123456789012345678901234567890AB"

	if err := s.UpsertProvider(ctx, wallet, hash32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider (initial): %v", err)
	}
	if err := s.UpsertProvider(ctx, wallet, hash32(0xAA), "Suspended"); err != nil {
		t.Fatalf("UpsertProvider (update): %v", err)
	}

	got, err := s.GetProviderByWallet(ctx, wallet)
	if err != nil {
		t.Fatalf("GetProviderByWallet: %v", err)
	}
	if got.Status != "Suspended" {
		t.Errorf("Status = %q, want Suspended (reconciliation must overwrite stale status)", got.Status)
	}
}

func TestProviders_GetByWallet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetProviderByWallet(context.Background(), "GNONEXISTENT")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProviders_RejectsWrongLengthProviderRef(t *testing.T) {
	s := newTestStore(t)
	err := s.UpsertProvider(context.Background(), "GBADREF", []byte{1, 2, 3}, "Active")
	if err == nil {
		t.Fatal("expected an error for a provider_ref that is not exactly 32 bytes")
	}
}
