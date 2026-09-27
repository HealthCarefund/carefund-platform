package store

import (
	"context"
	"errors"
	"testing"
)

func TestAttesters_UpsertRequiresExistingProvider(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.UpsertAttester(ctx, "GATTESTER1", "GNONEXISTENTPROVIDER", hash32(0xBB), "Active")
	if err == nil {
		t.Fatal("expected a foreign key violation for a nonexistent provider_wallet")
	}
}

func TestAttesters_UpsertGetAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	provider := "GPROVIDER1"
	attester := "GATTESTER1"

	if err := s.UpsertProvider(ctx, provider, hash32(0xAA), "Active"); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	if err := s.UpsertAttester(ctx, attester, provider, hash32(0xBB), "Active"); err != nil {
		t.Fatalf("UpsertAttester: %v", err)
	}

	got, err := s.GetAttesterByWallet(ctx, attester)
	if err != nil {
		t.Fatalf("GetAttesterByWallet: %v", err)
	}
	if got.ProviderWallet != provider || got.Status != "Active" {
		t.Errorf("got %+v", got)
	}

	list, err := s.ListAttestersByProvider(ctx, provider)
	if err != nil {
		t.Fatalf("ListAttestersByProvider: %v", err)
	}
	if len(list) != 1 || list[0].WalletAddress != attester {
		t.Errorf("list = %+v", list)
	}
}

func TestAttesters_GetByWallet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAttesterByWallet(context.Background(), "GNONEXISTENT")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
