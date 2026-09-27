package store

import "time"

// Provider mirrors the provider-registry contract's ProviderRecord, plus
// row bookkeeping. status is one of "Active", "Suspended", "Revoked".
type Provider struct {
	ID            int64
	WalletAddress string
	ProviderRef   []byte // exactly 32 bytes
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Attester mirrors the provider-registry contract's AttesterRecord.
type Attester struct {
	ID             int64
	WalletAddress  string
	ProviderWallet string
	CredentialRef  []byte // exactly 32 bytes
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CareAgreement mirrors the care-agreement contract's Agreement record.
// Amounts are decimal strings (NUMERIC(39,0) in Postgres) to avoid any
// float precision loss on i128 values.
type CareAgreement struct {
	AgreementID               int64
	SponsorWallet             string
	ProviderWallet            string
	AttesterWallet            string
	PatientRefCommitment      []byte // exactly 32 bytes
	ServiceCommitment         []byte // exactly 32 bytes
	FundingAmount             string
	SettlementAmount          string
	SettlementAssetContractID string
	FundingDeadline           int64
	CareDeadline              int64
	DisputeWindowSecs         int64
	State                     string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// Attestation is the off-chain record of an attest_care call.
type Attestation struct {
	ID             int64
	AgreementID    int64
	AttesterWallet string
	Commitment     []byte // exactly 32 bytes
	AttestedAt     int64  // on-chain ledger timestamp
	RecordedAt     time.Time
}

// TransactionRef tracks the off-chain lifecycle of one submitted
// transaction. Status is one of "pending", "submitted", "confirmed",
// "failed" — see packages/sdk's TransactionLifecycleStatus, mirrored here
// independently for the backend's own bookkeeping.
type TransactionRef struct {
	ID          int64
	AgreementID *int64
	Operation   string
	TxHash      string
	Status      string
	FirstSeen   time.Time
	LastChecked *time.Time
	ConfirmedAt *time.Time
	Ledger      *int64
	ErrorCode   *string
	ErrorDetail *string
}

// ContractEvent is one observed on-chain event, deduplicated by
// (contract_id, tx_hash, event_index).
type ContractEvent struct {
	ID          int64
	ContractID  string
	TxHash      string
	EventIndex  int32
	EventType   string
	AgreementID *int64
	Ledger      int64
	ObservedAt  time.Time
}

// IdempotencyRecord tracks one Idempotency-Key + operation pair.
type IdempotencyRecord struct {
	ID              int64
	KeyValue        string
	Operation       string
	RequestHash     string
	Status          string // "pending", "completed", "failed"
	ResultReference *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AuditRecord is one immutable audit-log entry.
type AuditRecord struct {
	ID           int64
	ActorWallet  *string
	Action       string
	ResourceType string
	ResourceID   string
	Result       string
	CreatedAt    time.Time
	MetadataJSON []byte // raw JSON, opaque to this layer
}
