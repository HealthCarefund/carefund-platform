package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateAgreementIntent inserts a new off-chain agreement intent and
// returns it with its allocated id. This never touches the chain.
func (s *Store) CreateAgreementIntent(ctx context.Context, in AgreementIntent) (*AgreementIntent, error) {
	var out AgreementIntent
	err := s.pool.QueryRow(ctx, `
		INSERT INTO agreement_intents (
			sponsor_wallet, provider_wallet, attester_wallet,
			patient_ref_commitment, service_commitment,
			funding_amount, settlement_amount, settlement_asset_contract_id,
			funding_deadline, care_deadline, dispute_window_secs, status
		) VALUES (
			$1, $2, $3, $4, $5, $6::numeric, $7::numeric, $8, $9, $10, $11, 'pending'
		)
		RETURNING id, sponsor_wallet, provider_wallet, attester_wallet,
		          patient_ref_commitment, service_commitment,
		          funding_amount::text, settlement_amount::text, settlement_asset_contract_id,
		          funding_deadline, care_deadline, dispute_window_secs, status, agreement_id,
		          created_at, updated_at
	`,
		in.SponsorWallet, in.ProviderWallet, in.AttesterWallet,
		in.PatientRefCommitment, in.ServiceCommitment,
		in.FundingAmount, in.SettlementAmount, in.SettlementAssetContractID,
		in.FundingDeadline, in.CareDeadline, in.DisputeWindowSecs,
	).Scan(
		&out.ID, &out.SponsorWallet, &out.ProviderWallet, &out.AttesterWallet,
		&out.PatientRefCommitment, &out.ServiceCommitment,
		&out.FundingAmount, &out.SettlementAmount, &out.SettlementAssetContractID,
		&out.FundingDeadline, &out.CareDeadline, &out.DisputeWindowSecs, &out.Status, &out.AgreementID,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating agreement intent: %w", err)
	}
	return &out, nil
}

// GetAgreementIntent returns ErrNotFound if no such intent exists.
func (s *Store) GetAgreementIntent(ctx context.Context, id int64) (*AgreementIntent, error) {
	var out AgreementIntent
	err := s.pool.QueryRow(ctx, `
		SELECT id, sponsor_wallet, provider_wallet, attester_wallet,
		       patient_ref_commitment, service_commitment,
		       funding_amount::text, settlement_amount::text, settlement_asset_contract_id,
		       funding_deadline, care_deadline, dispute_window_secs, status, agreement_id,
		       created_at, updated_at
		FROM agreement_intents
		WHERE id = $1
	`, id).Scan(
		&out.ID, &out.SponsorWallet, &out.ProviderWallet, &out.AttesterWallet,
		&out.PatientRefCommitment, &out.ServiceCommitment,
		&out.FundingAmount, &out.SettlementAmount, &out.SettlementAssetContractID,
		&out.FundingDeadline, &out.CareDeadline, &out.DisputeWindowSecs, &out.Status, &out.AgreementID,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting agreement intent %d: %w", id, err)
	}
	return &out, nil
}
