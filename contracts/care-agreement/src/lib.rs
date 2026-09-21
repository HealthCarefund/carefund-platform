#![no_std]

//! Care Agreement Contract
//!
//! Enforces the lifecycle and financial settlement of healthcare care agreements.

use soroban_sdk::{contract, contracterror, contractimpl, contracttype, Address, BytesN};

// ---------------------------------------------------------------------------
// Constants / TTL configuration
// ---------------------------------------------------------------------------

/// Minimum ledger threshold before extending instance TTL (~7 days at 5s/ledger).
pub const INSTANCE_TTL_THRESHOLD: u32 = 120_960;
/// Amount of ledgers to extend instance TTL to (~30 days at 5s/ledger).
pub const INSTANCE_TTL_EXTEND_TO: u32 = 518_400;

/// Minimum ledger threshold before extending persistent entry TTL (~7 days).
pub const PERSISTENT_TTL_THRESHOLD: u32 = 120_960;
/// Amount of ledgers to extend persistent entry TTL to (~30 days).
pub const PERSISTENT_TTL_EXTEND_TO: u32 = 518_400;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/// Lifecycle state for care agreements.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AgreementState {
    Requested,
    Funded,
    CareConfirmed,
    Disputed,
    Cancelled,
    Expired,
    Refunded,
    Settled,
}

/// Origin state when a dispute is opened.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DisputeOrigin {
    Funded,
    CareConfirmed,
}

/// Resolution action for a disputed agreement.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DisputeResolution {
    Resume,
    Settle,
    Refund,
}

/// On-chain record for a care funding agreement.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Agreement {
    pub sponsor: Address,
    pub provider: Address,
    pub attester: Address,

    /// Opaque commitment — not raw patient data.
    pub patient_ref_commitment: BytesN<32>,
    /// Opaque commitment — not raw service details.
    pub service_commitment: BytesN<32>,

    pub funding_amount: i128,
    pub settlement_amount: i128,

    pub funding_deadline: u64,
    pub care_deadline: u64,
    pub dispute_window_secs: u64,

    pub state: AgreementState,

    pub attestation_commitment: Option<BytesN<32>>,
    pub attested_by: Option<Address>,
    pub attested_at: Option<u64>,

    pub dispute_origin: Option<DisputeOrigin>,
    pub dispute_opened_by: Option<Address>,
    pub dispute_opened_at: Option<u64>,

    pub created_at: u64,
}

// ---------------------------------------------------------------------------
// Storage keys
// ---------------------------------------------------------------------------

/// Namespaced keys for contract storage.
#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    ProviderRegistry,
    SettlementAsset,
    NextAgreementId,
    Agreement(u64),
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq, PartialOrd, Ord)]
#[repr(u32)]
pub enum AgreementError {
    AlreadyInitialized = 1,
    Unauthorized = 2,
    AgreementNotFound = 3,
    InvalidState = 4,
    InvalidAmount = 5,
    InvalidDeadline = 6,
    ProviderNotActive = 7,
    AttesterNotAuthorized = 8,
    FundingDeadlineNotReached = 9,
    FundingDeadlineNotPassed = 10,
    CareDeadlineNotPassed = 11,
    DisputeWindowActive = 12,
    DisputeWindowClosed = 13,
    InvalidResolution = 14,
    TransferFailed = 15,
    ArithmeticOverflow = 16,
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct CareAgreementContract;

#[contractimpl]
impl CareAgreementContract {}
