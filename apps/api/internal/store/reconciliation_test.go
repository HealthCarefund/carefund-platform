package store

import (
	"context"
	"errors"
	"testing"
)

func TestAcknowledgeReconciliationGapIsAtomicAndRejectsStaleState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	gap := `{"detected_at":"2026-10-28T00:00:00Z","requested_ledger":100,"rpc_oldest_ledger":9000,"rpc_error":"retained window"}`
	acknowledged := `{"detected_at":"2026-10-28T00:00:00Z","requested_ledger":100,"rpc_oldest_ledger":9000,"rpc_error":"retained window","acknowledged_at":"2026-10-28T01:00:00Z","resumed_at_ledger":9000}`
	if err := s.SetReconciliationCursor(ctx, "event_cursor", "100"); err != nil {
		t.Fatalf("SetReconciliationCursor cursor: %v", err)
	}
	if err := s.SetReconciliationCursor(ctx, "event_gap", gap); err != nil {
		t.Fatalf("SetReconciliationCursor gap: %v", err)
	}

	if err := s.AcknowledgeReconciliationGap(ctx, "event_gap", "event_cursor", gap, "100", "9000", acknowledged); err != nil {
		t.Fatalf("AcknowledgeReconciliationGap: %v", err)
	}
	gotCursor, err := s.GetReconciliationCursor(ctx, "event_cursor")
	if err != nil || gotCursor != "9000" {
		t.Fatalf("cursor after acknowledgement = %q, err=%v", gotCursor, err)
	}
	gotGap, err := s.GetReconciliationCursor(ctx, "event_gap")
	if err != nil || gotGap != acknowledged {
		t.Fatalf("gap after acknowledgement = %q, err=%v", gotGap, err)
	}

	if err := s.AcknowledgeReconciliationGap(ctx, "event_gap", "event_cursor", gap, "100", "9001", acknowledged); err == nil {
		t.Fatal("stale repeated acknowledgement must fail")
	}
	gotCursor, _ = s.GetReconciliationCursor(ctx, "event_cursor")
	gotGap, _ = s.GetReconciliationCursor(ctx, "event_gap")
	if gotCursor != "9000" || gotGap != acknowledged {
		t.Fatalf("stale acknowledgement changed state: cursor=%q gap=%q", gotCursor, gotGap)
	}
}

func TestAcknowledgeReconciliationGapRejectsMissingCursorWithoutChangingGap(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	gap := `{"detected_at":"2026-10-28T00:00:00Z","requested_ledger":100,"rpc_oldest_ledger":9000,"rpc_error":"retained window"}`
	if err := s.SetReconciliationCursor(ctx, "event_gap", gap); err != nil {
		t.Fatalf("SetReconciliationCursor gap: %v", err)
	}
	err := s.AcknowledgeReconciliationGap(ctx, "event_gap", "missing_cursor", gap, "", "9000", gap)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	got, err := s.GetReconciliationCursor(ctx, "event_gap")
	if err != nil || got != gap {
		t.Fatalf("gap after rejected acknowledgement = %q, err=%v", got, err)
	}
}
