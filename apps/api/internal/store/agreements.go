package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertAgreement inserts or updates a care agreement's off-chain mirror
// row, keyed on the on-chain agreement_id. Amounts are passed as decimal
// strings and cast server-side, so no float conversion ever happens to an
// i128 value.
func (s *Store) UpsertAgreement(ctx context.Context, a CareAgreement) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO care_agreements (
			agreement_id, sponsor_wallet, provider_wallet, attester_wallet,
			patient_ref_commitment, service_commitment,
			funding_amount, settlement_amount, settlement_asset_contract_id,
			funding_deadline, care_deadline, dispute_window_secs, state
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric, $9, $10, $11, $12, $13
		)
		ON CONFLICT (agreement_id) DO UPDATE
			SET sponsor_wallet = EXCLUDED.sponsor_wallet,
			    provider_wallet = EXCLUDED.provider_wallet,
			    attester_wallet = EXCLUDED.attester_wallet,
			    patient_ref_commitment = EXCLUDED.patient_ref_commitment,
			    service_commitment = EXCLUDED.service_commitment,
			    funding_amount = EXCLUDED.funding_amount,
			    settlement_amount = EXCLUDED.settlement_amount,
			    settlement_asset_contract_id = EXCLUDED.settlement_asset_contract_id,
			    funding_deadline = EXCLUDED.funding_deadline,
			    care_deadline = EXCLUDED.care_deadline,
			    dispute_window_secs = EXCLUDED.dispute_window_secs,
			    state = EXCLUDED.state,
			    updated_at = now()
	`,
		a.AgreementID, a.SponsorWallet, a.ProviderWallet, a.AttesterWallet,
		a.PatientRefCommitment, a.ServiceCommitment,
		a.FundingAmount, a.SettlementAmount, a.SettlementAssetContractID,
		a.FundingDeadline, a.CareDeadline, a.DisputeWindowSecs, a.State,
	)
	if err != nil {
		return fmt.Errorf("upserting agreement %d: %w", a.AgreementID, err)
	}
	return nil
}

// GetAgreement returns ErrNotFound if no such agreement is mirrored.
// Amounts are read back as text (never as float64), matching the
// NUMERIC(39,0) column's exact decimal representation.
func (s *Store) GetAgreement(ctx context.Context, agreementID int64) (*CareAgreement, error) {
	var a CareAgreement
	err := s.pool.QueryRow(ctx, `
		SELECT agreement_id, sponsor_wallet, provider_wallet, attester_wallet,
		       patient_ref_commitment, service_commitment,
		       funding_amount::text, settlement_amount::text, settlement_asset_contract_id,
		       funding_deadline, care_deadline, dispute_window_secs, state,
		       created_at, updated_at
		FROM care_agreements
		WHERE agreement_id = $1
	`, agreementID).Scan(
		&a.AgreementID, &a.SponsorWallet, &a.ProviderWallet, &a.AttesterWallet,
		&a.PatientRefCommitment, &a.ServiceCommitment,
		&a.FundingAmount, &a.SettlementAmount, &a.SettlementAssetContractID,
		&a.FundingDeadline, &a.CareDeadline, &a.DisputeWindowSecs, &a.State,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting agreement %d: %w", agreementID, err)
	}
	return &a, nil
}
