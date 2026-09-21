#![no_std]

//! Provider Registry Contract
//!
//! Maintains the active authorization set of providers and attesters.
//! Owns the actor lifecycle: registration, suspension, reinstatement, and revocation.

mod test;

use soroban_sdk::{
    contract, contracterror, contractimpl, contracttype, symbol_short, Address, BytesN, Env,
};

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
// Contract Interface
// ---------------------------------------------------------------------------

pub trait ProviderRegistryTrait {
    /// Initialize the contract with an admin address. Callable only once.
    fn initialize(env: Env, admin: Address) -> Result<(), RegistryError>;

    /// Read the admin address.
    fn get_admin(env: Env) -> Result<Address, RegistryError>;

    /// Check if a provider exists and is currently Active.
    fn is_provider_active(env: Env, provider: Address) -> bool;

    /// Check if an attester is Active and bound to an Active provider.
    /// Returns true only when: attester exists, is Active, provider exists,
    /// is Active, and attester.provider == provider.
    fn check_attester(env: Env, attester: Address, provider: Address) -> bool;

    /// Read a provider's full record.
    fn get_provider(env: Env, provider: Address) -> Option<ProviderRecord>;

    /// Read an attester's full record.
    fn get_attester(env: Env, attester: Address) -> Option<AttesterRecord>;
}

// ---------------------------------------------------------------------------
// Contract Implementation
// ---------------------------------------------------------------------------

#[contract]
pub struct ProviderRegistryContract;

#[contractimpl]
impl ProviderRegistryContract {
    /// Initialize the contract with an admin address. Callable only once.
    pub fn initialize(env: Env, admin: Address) -> Result<(), RegistryError> {
        if env.storage().instance().has(&DataKey::Admin) {
            return Err(RegistryError::AlreadyInitialized);
        }

        admin.require_auth();

        env.storage().instance().set(&DataKey::Admin, &admin);
        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events().publish((symbol_short!("init"),), admin);

        Ok(())
    }

    /// Read the admin address.
    pub fn get_admin(env: Env) -> Result<Address, RegistryError> {
        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)
    }

    /// Check if a provider exists and is currently Active.
    pub fn is_provider_active(env: Env, provider: Address) -> bool {
        let key = DataKey::Provider(provider);
        if let Some(record) = env
            .storage()
            .persistent()
            .get::<DataKey, ProviderRecord>(&key)
        {
            env.storage().persistent().extend_ttl(
                &key,
                PERSISTENT_TTL_THRESHOLD,
                PERSISTENT_TTL_EXTEND_TO,
            );
            record.status == ActorStatus::Active
        } else {
            false
        }
    }

    /// Check if an attester is Active and bound to an Active provider.
    /// Returns true only when: attester exists, is Active, provider exists,
    /// is Active, and attester.provider == provider.
    pub fn check_attester(env: Env, attester: Address, provider: Address) -> bool {
        let attester_key = DataKey::Attester(attester);
        let attester_record: AttesterRecord = match env
            .storage()
            .persistent()
            .get::<DataKey, AttesterRecord>(&attester_key)
        {
            Some(r) => {
                env.storage().persistent().extend_ttl(
                    &attester_key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );
                r
            }
            None => return false,
        };

        if attester_record.status != ActorStatus::Active {
            return false;
        }

        if attester_record.provider != provider {
            return false;
        }

        let provider_key = DataKey::Provider(provider);
        let provider_record: ProviderRecord = match env
            .storage()
            .persistent()
            .get::<DataKey, ProviderRecord>(&provider_key)
        {
            Some(r) => {
                env.storage().persistent().extend_ttl(
                    &provider_key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );
                r
            }
            None => return false,
        };

        provider_record.status == ActorStatus::Active
    }

    /// Read a provider's full record.
    pub fn get_provider(env: Env, provider: Address) -> Option<ProviderRecord> {
        let key = DataKey::Provider(provider);
        if let Some(record) = env
            .storage()
            .persistent()
            .get::<DataKey, ProviderRecord>(&key)
        {
            env.storage().persistent().extend_ttl(
                &key,
                PERSISTENT_TTL_THRESHOLD,
                PERSISTENT_TTL_EXTEND_TO,
            );
            Some(record)
        } else {
            None
        }
    }

    /// Read an attester's full record.
    pub fn get_attester(env: Env, attester: Address) -> Option<AttesterRecord> {
        let key = DataKey::Attester(attester);
        if let Some(record) = env
            .storage()
            .persistent()
            .get::<DataKey, AttesterRecord>(&key)
        {
            env.storage().persistent().extend_ttl(
                &key,
                PERSISTENT_TTL_THRESHOLD,
                PERSISTENT_TTL_EXTEND_TO,
            );
            Some(record)
        } else {
            None
        }
    }

    /// Register a new provider. Requires admin authorization.
    pub fn register_provider(
        env: Env,
        provider: Address,
        provider_ref: BytesN<32>,
    ) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Provider(provider.clone());

        // Check if provider already exists
        if let Some(existing) = env
            .storage()
            .persistent()
            .get::<DataKey, ProviderRecord>(&key)
        {
            return match existing.status {
                ActorStatus::Active | ActorStatus::Suspended => {
                    Err(RegistryError::ProviderAlreadyExists)
                }
                ActorStatus::Revoked => Err(RegistryError::ProviderAlreadyRevoked),
            };
        }

        let record = ProviderRecord {
            status: ActorStatus::Active,
            provider_ref,
        };

        env.storage().persistent().set(&key, &record);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.events().publish((symbol_short!("prov_reg"),), provider);

        Ok(())
    }

    /// Suspend an active provider. Requires admin authorization.
    pub fn suspend_provider(env: Env, provider: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Provider(provider.clone());
        let mut record: ProviderRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::ProviderNotFound)?;

        match record.status {
            ActorStatus::Active => {
                record.status = ActorStatus::Suspended;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("prov_sus"),), provider);

                Ok(())
            }
            ActorStatus::Suspended => Err(RegistryError::ProviderAlreadySuspended),
            ActorStatus::Revoked => Err(RegistryError::ProviderAlreadyRevoked),
        }
    }

    /// Reinstate a suspended provider. Requires admin authorization.
    pub fn reinstate_provider(env: Env, provider: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Provider(provider.clone());
        let mut record: ProviderRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::ProviderNotFound)?;

        match record.status {
            ActorStatus::Suspended => {
                record.status = ActorStatus::Active;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("prov_rei"),), provider);

                Ok(())
            }
            ActorStatus::Active => Err(RegistryError::ProviderAlreadyActive),
            ActorStatus::Revoked => Err(RegistryError::ProviderAlreadyRevoked),
        }
    }

    /// Revoke a provider (terminal operation). Requires admin authorization.
    pub fn revoke_provider(env: Env, provider: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Provider(provider.clone());
        let mut record: ProviderRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::ProviderNotFound)?;

        match record.status {
            ActorStatus::Active | ActorStatus::Suspended => {
                record.status = ActorStatus::Revoked;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("prov_rev"),), provider);

                Ok(())
            }
            ActorStatus::Revoked => Err(RegistryError::ProviderAlreadyRevoked),
        }
    }

    /// Register a new attester bound to a specific provider. Requires admin authorization.
    pub fn register_attester(
        env: Env,
        attester: Address,
        provider: Address,
        credential_ref: BytesN<32>,
    ) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        // Check provider exists and is Active
        let provider_key = DataKey::Provider(provider.clone());
        let provider_record: ProviderRecord = env
            .storage()
            .persistent()
            .get(&provider_key)
            .ok_or(RegistryError::ProviderNotFound)?;

        if provider_record.status != ActorStatus::Active {
            return Err(RegistryError::ProviderNotActive);
        }

        // Check attester doesn't already exist
        let attester_key = DataKey::Attester(attester.clone());
        if env.storage().persistent().has(&attester_key) {
            return Err(RegistryError::AttesterAlreadyExists);
        }

        let record = AttesterRecord {
            provider,
            status: ActorStatus::Active,
            credential_ref,
        };

        env.storage().persistent().set(&attester_key, &record);
        env.storage().persistent().extend_ttl(
            &attester_key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.events().publish((symbol_short!("att_reg"),), attester);

        Ok(())
    }

    /// Suspend an active attester. Requires admin authorization.
    pub fn suspend_attester(env: Env, attester: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Attester(attester.clone());
        let mut record: AttesterRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::AttesterNotFound)?;

        match record.status {
            ActorStatus::Active => {
                record.status = ActorStatus::Suspended;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("att_sus"),), attester);

                Ok(())
            }
            ActorStatus::Suspended => Err(RegistryError::AttesterAlreadySuspended),
            ActorStatus::Revoked => Err(RegistryError::AttesterAlreadyRevoked),
        }
    }

    /// Reinstate a suspended attester. Requires admin authorization.
    pub fn reinstate_attester(env: Env, attester: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Attester(attester.clone());
        let mut record: AttesterRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::AttesterNotFound)?;

        match record.status {
            ActorStatus::Suspended => {
                record.status = ActorStatus::Active;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("att_rei"),), attester);

                Ok(())
            }
            ActorStatus::Active => Err(RegistryError::AttesterAlreadyActive),
            ActorStatus::Revoked => Err(RegistryError::AttesterAlreadyRevoked),
        }
    }

    /// Revoke an attester (terminal operation). Requires admin authorization.
    pub fn revoke_attester(env: Env, attester: Address) -> Result<(), RegistryError> {
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(RegistryError::Unauthorized)?;
        admin.require_auth();

        let key = DataKey::Attester(attester.clone());
        let mut record: AttesterRecord = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(RegistryError::AttesterNotFound)?;

        match record.status {
            ActorStatus::Active | ActorStatus::Suspended => {
                record.status = ActorStatus::Revoked;
                env.storage().persistent().set(&key, &record);
                env.storage().persistent().extend_ttl(
                    &key,
                    PERSISTENT_TTL_THRESHOLD,
                    PERSISTENT_TTL_EXTEND_TO,
                );

                env.events().publish((symbol_short!("att_rev"),), attester);

                Ok(())
            }
            ActorStatus::Revoked => Err(RegistryError::AttesterAlreadyRevoked),
        }
    }
}
