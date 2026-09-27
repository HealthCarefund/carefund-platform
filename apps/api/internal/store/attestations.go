package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertAttestation records (or corrects, on reconciliation) the off-chain
// mirror of one attest_care call. agreement_id is unique: a care agreement
// has at most one attestation on-chain.
func (s *Store) UpsertAttestation(ctx context.Context, at Attestation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO attestations (agreement_id, attester_wallet, commitment, attested_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (agreement_id) DO UPDATE
			SET attester_wallet = EXCLUDED.attester_wallet,
			    commitment = EXCLUDED.commitment,
			    attested_at = EXCLUDED.attested_at
	`, at.AgreementID, at.AttesterWallet, at.Commitment, at.AttestedAt)
	if err != nil {
		return fmt.Errorf("upserting attestation for agreement %d: %w", at.AgreementID, err)
	}
	return nil
}

// GetAttestationByAgreement returns ErrNotFound if the agreement has not
// been attested (yet, or at all).
func (s *Store) GetAttestationByAgreement(ctx context.Context, agreementID int64) (*Attestation, error) {
	var at Attestation
	err := s.pool.QueryRow(ctx, `
		SELECT id, agreement_id, attester_wallet, commitment, attested_at, recorded_at
		FROM attestations
		WHERE agreement_id = $1
	`, agreementID).Scan(&at.ID, &at.AgreementID, &at.AttesterWallet, &at.Commitment, &at.AttestedAt, &at.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting attestation for agreement %d: %w", agreementID, err)
	}
	return &at, nil
}
