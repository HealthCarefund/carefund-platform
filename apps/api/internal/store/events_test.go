package store

import (
	"context"
	"testing"
)

func TestEvents_InsertIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(20)
	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	e := ContractEvent{
		ContractID:  "Ccareagreement",
		TxHash:      "f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0",
		EventIndex:  0,
		EventType:   "agr_fun",
		AgreementID: int64Ptr(20),
		Ledger:      100,
	}

	inserted, err := s.InsertContractEvent(ctx, e)
	if err != nil {
		t.Fatalf("InsertContractEvent (first): %v", err)
	}
	if !inserted {
		t.Error("expected the first insert to report inserted=true")
	}

	inserted, err = s.InsertContractEvent(ctx, e)
	if err != nil {
		t.Fatalf("InsertContractEvent (duplicate): %v", err)
	}
	if inserted {
		t.Error("expected the duplicate insert to report inserted=false, not create a second row")
	}
}

func TestEvents_ListForAgreement_PaginatesByCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(21)
	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	for i := 0; i < 3; i++ {
		hash := make([]byte, 64)
		for j := range hash {
			hash[j] = byte('0' + i)
		}
		_, err := s.InsertContractEvent(ctx, ContractEvent{
			ContractID:  "Ccareagreement",
			TxHash:      string(hash),
			EventIndex:  0,
			EventType:   "agr_evt",
			AgreementID: int64Ptr(21),
			Ledger:      int64(100 + i),
		})
		if err != nil {
			t.Fatalf("InsertContractEvent(%d): %v", i, err)
		}
	}

	first, err := s.ListEventsForAgreement(ctx, 21, 0, 2)
	if err != nil {
		t.Fatalf("ListEventsForAgreement (page 1): %v", err)
	}
	if len(first.Events) != 2 || first.NextCursor == "" {
		t.Fatalf("page 1 = %+v, want 2 events and a next cursor", first)
	}

	lastID := first.Events[len(first.Events)-1].ID
	second, err := s.ListEventsForAgreement(ctx, 21, lastID, 2)
	if err != nil {
		t.Fatalf("ListEventsForAgreement (page 2): %v", err)
	}
	if len(second.Events) != 1 || second.NextCursor != "" {
		t.Fatalf("page 2 = %+v, want exactly 1 event and no further cursor", second)
	}
}

func int64Ptr(v int64) *int64 { return &v }
