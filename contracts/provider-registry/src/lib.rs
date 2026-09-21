#![no_std]

//! Provider Registry Contract
//!
//! Maintains the active authorization set of providers and attesters.
//! Owns the actor lifecycle: registration, suspension, reinstatement, and revocation.

mod test;

use soroban_sdk::{contract, contracterror, contractimpl, contracttype, Address, BytesN};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/// Lifecycle status for providers and attesters.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ActorStatus {
    Active,
    Suspended,
    Revoked,
}

/// On-chain record for a registered provider.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProviderRecord {
    pub status: ActorStatus,
    /// Opaque commitment — not a raw document or license.
    pub provider_ref: BytesN<32>,
}

/// On-chain record for a registered attester.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AttesterRecord {
    pub provider: Address,
    pub status: ActorStatus,
    /// Opaque commitment — not a raw credential document.
    pub credential_ref: BytesN<32>,
}

// ---------------------------------------------------------------------------
// Storage keys
// ---------------------------------------------------------------------------

/// Namespaced keys for contract storage.
#[contracttype]
#[derive(Clone)]
pub enum DataKey {
    Admin,
    Provider(Address),
    Attester(Address),
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

#[contracterror]
#[derive(Copy, Clone, Debug, Eq, PartialEq, PartialOrd, Ord)]
#[repr(u32)]
pub enum RegistryError {
    AlreadyInitialized = 1,
    Unauthorized = 2,
    ProviderNotFound = 3,
    ProviderAlreadyExists = 4,
    ProviderAlreadySuspended = 5,
    ProviderAlreadyActive = 6,
    ProviderAlreadyRevoked = 7,
    AttesterNotFound = 8,
    AttesterAlreadyExists = 9,
    AttesterAlreadySuspended = 10,
    AttesterAlreadyActive = 11,
    AttesterAlreadyRevoked = 12,
    ProviderNotActive = 13,
    InvalidState = 14,
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct ProviderRegistryContract;

#[contractimpl]
impl ProviderRegistryContract {}
