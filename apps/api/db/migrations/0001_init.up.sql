-- Off-chain workflow metadata only. No PHI: patient/service/attestation
-- fields are opaque 32-byte commitments, identical to what the contracts
-- themselves store, never raw clinical data. Chain state is authoritative;
-- these tables are a queryable, reconcilable mirror plus application-only
-- bookkeeping (idempotency, audit, transaction lifecycle) that has no
-- on-chain counterpart at all.

CREATE TABLE providers (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address TEXT NOT NULL UNIQUE,
    provider_ref   BYTEA NOT NULL CHECK (octet_length(provider_ref) = 32),
    status         TEXT NOT NULL CHECK (status IN ('Active', 'Suspended', 'Revoked')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attesters (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_address  TEXT NOT NULL UNIQUE,
    provider_wallet TEXT NOT NULL REFERENCES providers (wallet_address),
    credential_ref  BYTEA NOT NULL CHECK (octet_length(credential_ref) = 32),
    status          TEXT NOT NULL CHECK (status IN ('Active', 'Suspended', 'Revoked')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_attesters_provider_wallet ON attesters (provider_wallet);

CREATE TABLE care_agreements (
    agreement_id               BIGINT PRIMARY KEY,
    sponsor_wallet             TEXT NOT NULL,
    provider_wallet            TEXT NOT NULL,
    attester_wallet            TEXT NOT NULL,
    patient_ref_commitment     BYTEA NOT NULL CHECK (octet_length(patient_ref_commitment) = 32),
    service_commitment         BYTEA NOT NULL CHECK (octet_length(service_commitment) = 32),
    funding_amount             NUMERIC(39, 0) NOT NULL,
    settlement_amount          NUMERIC(39, 0) NOT NULL,
    settlement_asset_contract_id TEXT NOT NULL,
    funding_deadline           BIGINT NOT NULL,
    care_deadline              BIGINT NOT NULL,
    dispute_window_secs        BIGINT NOT NULL,
    state                      TEXT NOT NULL CHECK (
        state IN ('Requested', 'Funded', 'CareConfirmed', 'Disputed', 'Cancelled', 'Expired', 'Refunded', 'Settled')
    ),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_care_agreements_sponsor_wallet ON care_agreements (sponsor_wallet);
CREATE INDEX idx_care_agreements_provider_wallet ON care_agreements (provider_wallet);

CREATE TABLE attestations (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    agreement_id  BIGINT NOT NULL UNIQUE REFERENCES care_agreements (agreement_id),
    attester_wallet TEXT NOT NULL,
    commitment    BYTEA NOT NULL CHECK (octet_length(commitment) = 32),
    attested_at   BIGINT NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE transaction_refs (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    agreement_id BIGINT REFERENCES care_agreements (agreement_id),
    operation    TEXT NOT NULL,
    tx_hash      TEXT NOT NULL UNIQUE,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'submitted', 'confirmed', 'failed')),
    first_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_checked TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    ledger       BIGINT,
    error_code   TEXT,
    error_detail TEXT
);
CREATE INDEX idx_transaction_refs_agreement_id ON transaction_refs (agreement_id);
CREATE INDEX idx_transaction_refs_status ON transaction_refs (status);

CREATE TABLE contract_events (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_id  TEXT NOT NULL,
    tx_hash      TEXT NOT NULL,
    event_index  INTEGER NOT NULL,
    event_type   TEXT NOT NULL,
    agreement_id BIGINT REFERENCES care_agreements (agreement_id),
    ledger       BIGINT NOT NULL,
    observed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (contract_id, tx_hash, event_index)
);
CREATE INDEX idx_contract_events_agreement_id ON contract_events (agreement_id);

CREATE TABLE idempotency_keys (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key_value        TEXT NOT NULL,
    operation        TEXT NOT NULL,
    request_hash     TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    result_reference TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (key_value, operation)
);

CREATE TABLE audit_records (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_wallet  TEXT,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    result        TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata_json JSONB
);
CREATE INDEX idx_audit_records_resource ON audit_records (resource_type, resource_id);
