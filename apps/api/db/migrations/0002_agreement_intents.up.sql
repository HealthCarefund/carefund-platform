-- An agreement intent is a purely off-chain workflow record: creating one
-- never calls create_agreement on-chain. It exists so a sponsor/provider
-- can express "we want to set up this agreement" before either side has
-- signed anything, and so the transaction-preparation step (a later unit)
-- has a stable id to prepare a transaction against. No PHI: commitments
-- are opaque 32-byte hashes, identical in shape to the on-chain fields
-- they will eventually become.

CREATE TABLE agreement_intents (
    id                          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sponsor_wallet              TEXT NOT NULL,
    provider_wallet             TEXT NOT NULL,
    attester_wallet             TEXT NOT NULL,
    patient_ref_commitment      BYTEA NOT NULL CHECK (octet_length(patient_ref_commitment) = 32),
    service_commitment          BYTEA NOT NULL CHECK (octet_length(service_commitment) = 32),
    funding_amount              NUMERIC(39, 0) NOT NULL,
    settlement_amount           NUMERIC(39, 0) NOT NULL,
    settlement_asset_contract_id TEXT NOT NULL,
    funding_deadline            BIGINT NOT NULL,
    care_deadline               BIGINT NOT NULL,
    dispute_window_secs         BIGINT NOT NULL,
    status                      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'submitted', 'expired', 'cancelled')),
    agreement_id                BIGINT REFERENCES care_agreements (agreement_id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_agreement_intents_sponsor_wallet ON agreement_intents (sponsor_wallet);
CREATE INDEX idx_agreement_intents_provider_wallet ON agreement_intents (provider_wallet);
