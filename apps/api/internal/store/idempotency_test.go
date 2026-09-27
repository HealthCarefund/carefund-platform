package store

import (
	"context"
	"errors"
	"testing"
)

func TestIdempotency_InsertGetAndUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rec, err := s.InsertIdempotencyRecord(ctx, "key-1", "create_agreement_intent", "hash-abc")
	if err != nil {
		t.Fatalf("InsertIdempotencyRecord: %v", err)
	}
	if rec.Status != "pending" {
		t.Errorf("Status = %q, want pending", rec.Status)
	}

	ref := "intent-123"
	if err := s.UpdateIdempotencyResult(ctx, "key-1", "create_agreement_intent", "completed", &ref); err != nil {
		t.Fatalf("UpdateIdempotencyResult: %v", err)
	}

	got, err := s.GetIdempotencyRecord(ctx, "key-1", "create_agreement_intent")
	if err != nil {
		t.Fatalf("GetIdempotencyRecord: %v", err)
	}
	if got.Status != "completed" || got.ResultReference == nil || *got.ResultReference != ref {
		t.Errorf("got %+v", got)
	}
}

func TestIdempotency_SameKeyDifferentOperationAreIndependent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.InsertIdempotencyRecord(ctx, "shared-key", "create_agreement_intent", "hash-a"); err != nil {
		t.Fatalf("InsertIdempotencyRecord (op A): %v", err)
	}
	if _, err := s.InsertIdempotencyRecord(ctx, "shared-key", "fund_transaction", "hash-b"); err != nil {
		t.Fatalf("InsertIdempotencyRecord (op B): %v", err)
	}
}

func TestIdempotency_ReusingKeyForSameOperationConflicts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.InsertIdempotencyRecord(ctx, "dup-key", "fund_transaction", "hash-a"); err != nil {
		t.Fatalf("InsertIdempotencyRecord (first): %v", err)
	}

	_, err := s.InsertIdempotencyRecord(ctx, "dup-key", "fund_transaction", "hash-b")
	if !errors.Is(err, ErrIdempotencyKeyExists) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyExists", err)
	}

	// The caller is expected to fetch the existing record and compare
	// request_hash itself to decide replay vs. 409 conflict — this layer
	// only enforces "claimed once", not the hash-comparison policy.
	existing, err := s.GetIdempotencyRecord(ctx, "dup-key", "fund_transaction")
	if err != nil {
		t.Fatalf("GetIdempotencyRecord: %v", err)
	}
	if existing.RequestHash != "hash-a" {
		t.Errorf("RequestHash = %q, want the original hash-a to have won", existing.RequestHash)
	}
}

func TestIdempotency_Get_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetIdempotencyRecord(context.Background(), "nonexistent", "op")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
