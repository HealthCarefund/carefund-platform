package store

import (
	"context"
	"errors"
	"testing"
)

func TestAttestations_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(10)
	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	if err := s.UpsertAttestation(ctx, Attestation{
		AgreementID:    10,
		AttesterWallet: agreement.AttesterWallet,
		Commitment:     hash32(0x03),
		AttestedAt:     1_700_050_000,
	}); err != nil {
		t.Fatalf("UpsertAttestation: %v", err)
	}

	got, err := s.GetAttestationByAgreement(ctx, 10)
	if err != nil {
		t.Fatalf("GetAttestationByAgreement: %v", err)
	}
	if got.AttestedAt != 1_700_050_000 {
		t.Errorf("AttestedAt = %d", got.AttestedAt)
	}
}

func TestAttestations_UniquePerAgreement(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(11)
	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	first := Attestation{AgreementID: 11, AttesterWallet: agreement.AttesterWallet, Commitment: hash32(0x03), AttestedAt: 100}
	if err := s.UpsertAttestation(ctx, first); err != nil {
		t.Fatalf("UpsertAttestation (first): %v", err)
	}

	second := Attestation{AgreementID: 11, AttesterWallet: agreement.AttesterWallet, Commitment: hash32(0x04), AttestedAt: 200}
	if err := s.UpsertAttestation(ctx, second); err != nil {
		t.Fatalf("UpsertAttestation (reconciliation correction): %v", err)
	}

	got, err := s.GetAttestationByAgreement(ctx, 11)
	if err != nil {
		t.Fatalf("GetAttestationByAgreement: %v", err)
	}
	if got.AttestedAt != 200 {
		t.Errorf("AttestedAt = %d, want 200 (the corrected value must win, not create a duplicate row)", got.AttestedAt)
	}
}

func TestAttestations_GetByAgreement_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAttestationByAgreement(context.Background(), 424242)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
