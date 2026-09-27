package store

import (
	"context"
	"testing"
)

func TestAudit_InsertRecord(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := "GACTOR1"

	err := s.InsertAuditRecord(ctx, &actor, "attest_care", "care_agreement", "42", "success", []byte(`{"note":"no PHI here"}`))
	if err != nil {
		t.Fatalf("InsertAuditRecord: %v", err)
	}
}

func TestAudit_InsertRecord_NilActorAndMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.InsertAuditRecord(ctx, nil, "expire", "care_agreement", "7", "success", nil)
	if err != nil {
		t.Fatalf("InsertAuditRecord with nil actor/metadata: %v", err)
	}
}
