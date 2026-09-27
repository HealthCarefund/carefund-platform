package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTransactions_InsertGetAndUpdateStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash := strings.Repeat("a", 64)

	if err := s.InsertTransactionRef(ctx, nil, "create_agreement", hash, "pending"); err != nil {
		t.Fatalf("InsertTransactionRef: %v", err)
	}

	got, err := s.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash: %v", err)
	}
	if got.Status != "pending" || got.LastChecked != nil {
		t.Errorf("got %+v", got)
	}

	ledger := int64(12345)
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.UpdateTransactionStatus(ctx, hash, "confirmed", &ledger, &now, nil, nil); err != nil {
		t.Fatalf("UpdateTransactionStatus: %v", err)
	}

	got, err = s.GetTransactionByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetTransactionByHash (after update): %v", err)
	}
	if got.Status != "confirmed" || got.LastChecked == nil || got.Ledger == nil || *got.Ledger != 12345 {
		t.Errorf("got %+v", got)
	}
}

func TestTransactions_UpdateStatus_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateTransactionStatus(context.Background(), "nonexistent", "confirmed", nil, nil, nil, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestTransactions_ListPendingExcludesTerminalStates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pendingHash := strings.Repeat("b", 64)
	confirmedHash := strings.Repeat("c", 64)
	failedHash := strings.Repeat("d", 64)

	for _, h := range []string{pendingHash, confirmedHash, failedHash} {
		if err := s.InsertTransactionRef(ctx, nil, "fund", h, "pending"); err != nil {
			t.Fatalf("InsertTransactionRef(%s): %v", h, err)
		}
	}
	if err := s.UpdateTransactionStatus(ctx, confirmedHash, "confirmed", nil, nil, nil, nil); err != nil {
		t.Fatalf("UpdateTransactionStatus(confirmed): %v", err)
	}
	if err := s.UpdateTransactionStatus(ctx, failedHash, "failed", nil, nil, nil, nil); err != nil {
		t.Fatalf("UpdateTransactionStatus(failed): %v", err)
	}

	pending, err := s.ListPendingTransactions(ctx)
	if err != nil {
		t.Fatalf("ListPendingTransactions: %v", err)
	}
	if len(pending) != 1 || pending[0].TxHash != pendingHash {
		t.Errorf("pending = %+v, want exactly one ref (%s)", pending, pendingHash)
	}
}

func TestTransactions_DuplicateHashRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	hash := strings.Repeat("e", 64)

	if err := s.InsertTransactionRef(ctx, nil, "settle", hash, "pending"); err != nil {
		t.Fatalf("InsertTransactionRef (first): %v", err)
	}
	if err := s.InsertTransactionRef(ctx, nil, "settle", hash, "pending"); err == nil {
		t.Fatal("expected a unique-violation error inserting the same tx_hash twice")
	}
}
