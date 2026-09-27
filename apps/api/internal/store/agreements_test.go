package store

import (
	"context"
	"errors"
	"testing"
)

func sampleAgreement(id int64) CareAgreement {
	return CareAgreement{
		AgreementID:               id,
		SponsorWallet:             "GSPONSOR1",
		ProviderWallet:            "GPROVIDER1",
		AttesterWallet:            "GATTESTER1",
		PatientRefCommitment:      hash32(0x01),
		ServiceCommitment:         hash32(0x02),
		FundingAmount:             "170141183460469231731687303715884105727", // i128 max, as a decimal string
		SettlementAmount:          "900000",
		SettlementAssetContractID: "CSETTLEMENTASSET1",
		FundingDeadline:           1_700_000_000,
		CareDeadline:              1_700_100_000,
		DisputeWindowSecs:         86_400,
		State:                     "Requested",
	}
}

func TestAgreements_UpsertAndGet_PreservesExactI128Amount(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(1)

	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	got, err := s.GetAgreement(ctx, 1)
	if err != nil {
		t.Fatalf("GetAgreement: %v", err)
	}
	if got.FundingAmount != agreement.FundingAmount {
		t.Errorf("FundingAmount = %q, want %q (must round-trip exactly, no float precision loss)", got.FundingAmount, agreement.FundingAmount)
	}
	if got.State != "Requested" {
		t.Errorf("State = %q, want Requested", got.State)
	}
}

func TestAgreements_UpsertUpdatesStateOnReconciliation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	agreement := sampleAgreement(2)

	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement (initial): %v", err)
	}
	agreement.State = "Funded"
	if err := s.UpsertAgreement(ctx, agreement); err != nil {
		t.Fatalf("UpsertAgreement (update): %v", err)
	}

	got, err := s.GetAgreement(ctx, 2)
	if err != nil {
		t.Fatalf("GetAgreement: %v", err)
	}
	if got.State != "Funded" {
		t.Errorf("State = %q, want Funded — stale metadata must be corrected by reconciliation", got.State)
	}
}

func TestAgreements_Get_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetAgreement(context.Background(), 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAgreements_RejectsInvalidState(t *testing.T) {
	s := newTestStore(t)
	agreement := sampleAgreement(3)
	agreement.State = "NotARealState"
	if err := s.UpsertAgreement(context.Background(), agreement); err == nil {
		t.Fatal("expected an error for an invalid state — this must never invent a new contract state")
	}
}
