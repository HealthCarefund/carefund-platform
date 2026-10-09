#![no_std]

//! Care Agreement Contract
//!
//! Enforces the lifecycle and financial settlement of healthcare care agreements.

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

/// Wrapper for optional dispute origin to work around SDK serialization.
#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum MaybeDisputeOrigin {
    None,
    Some(DisputeOrigin),
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

    pub dispute_origin: MaybeDisputeOrigin,
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
// Contract Interface
// ---------------------------------------------------------------------------

pub trait CareAgreementTrait {
    /// Initialize the contract with admin, provider registry, and settlement asset.
    /// Callable only once.
    fn initialize(
        env: Env,
        admin: Address,
        provider_registry: Address,
        settlement_asset: Address,
    ) -> Result<(), AgreementError>;

    /// Read the admin address.
    fn get_admin(env: Env) -> Result<Address, AgreementError>;

    /// Read the provider registry address.
    fn get_provider_registry(env: Env) -> Result<Address, AgreementError>;

    /// Read the settlement asset address.
    fn get_settlement_asset(env: Env) -> Result<Address, AgreementError>;
}

// ---------------------------------------------------------------------------
// Contract Implementation
// ---------------------------------------------------------------------------

#[contract]
pub struct CareAgreementContract;

#[contractimpl]
impl CareAgreementContract {
    /// Initialize the contract with admin, provider registry, and settlement asset.
    /// Callable only once.
    pub fn initialize(
        env: Env,
        admin: Address,
        provider_registry: Address,
        settlement_asset: Address,
    ) -> Result<(), AgreementError> {
        if env.storage().instance().has(&DataKey::Admin) {
            return Err(AgreementError::AlreadyInitialized);
        }

        admin.require_auth();

        env.storage().instance().set(&DataKey::Admin, &admin);
        env.storage()
            .instance()
            .set(&DataKey::ProviderRegistry, &provider_registry);
        env.storage()
            .instance()
            .set(&DataKey::SettlementAsset, &settlement_asset);
        env.storage()
            .instance()
            .set(&DataKey::NextAgreementId, &1u64);

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events().publish((symbol_short!("init"),), admin);

        Ok(())
    }

    /// Read the admin address.
    pub fn get_admin(env: Env) -> Result<Address, AgreementError> {
        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);
        env.storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(AgreementError::Unauthorized)
    }

    /// Read the provider registry address.
    pub fn get_provider_registry(env: Env) -> Result<Address, AgreementError> {
        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);
        env.storage()
            .instance()
            .get(&DataKey::ProviderRegistry)
            .ok_or(AgreementError::Unauthorized)
    }

    /// Read the settlement asset address.
    pub fn get_settlement_asset(env: Env) -> Result<Address, AgreementError> {
        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);
        env.storage()
            .instance()
            .get(&DataKey::SettlementAsset)
            .ok_or(AgreementError::Unauthorized)
    }

    /// Create a new care funding agreement.
    /// Requires provider authorization.
    pub fn create_agreement(
        env: Env,
        provider: Address,
        sponsor: Address,
        attester: Address,
        patient_ref_commitment: BytesN<32>,
        service_commitment: BytesN<32>,
        funding_amount: i128,
        settlement_amount: i128,
        funding_deadline: u64,
        care_deadline: u64,
        dispute_window_secs: u64,
    ) -> Result<u64, AgreementError> {
        provider.require_auth();

        // Load registry and asset
        let provider_registry: Address = env
            .storage()
            .instance()
            .get(&DataKey::ProviderRegistry)
            .ok_or(AgreementError::Unauthorized)?;

        let _settlement_asset: Address = env
            .storage()
            .instance()
            .get(&DataKey::SettlementAsset)
            .ok_or(AgreementError::Unauthorized)?;

        // Validate amounts
        if funding_amount <= 0 || settlement_amount <= 0 {
            return Err(AgreementError::InvalidAmount);
        }
        if settlement_amount > funding_amount {
            return Err(AgreementError::InvalidAmount);
        }

        let current_time = env.ledger().timestamp();

        // Validate deadlines
        if funding_deadline <= current_time {
            return Err(AgreementError::InvalidDeadline);
        }
        if care_deadline <= funding_deadline {
            return Err(AgreementError::InvalidDeadline);
        }
        // care_deadline + dispute_window_secs is computed later, in
        // open_dispute, as a plain u64 addition. Rejecting an overflowing
        // combination here means that computation can never trap: a
        // caller-supplied dispute_window_secs near u64::MAX would
        // otherwise pass validation here and only fail, permanently, the
        // first time anyone tried to open a dispute on this agreement.
        if care_deadline.checked_add(dispute_window_secs).is_none() {
            return Err(AgreementError::ArithmeticOverflow);
        }

        // Cross-contract call: check provider is active
        use soroban_sdk::IntoVal;
        let is_active: bool = env.invoke_contract(
            &provider_registry,
            &soroban_sdk::Symbol::new(&env, "is_provider_active"),
            soroban_sdk::Vec::from_array(&env, [provider.clone().into_val(&env)]),
        );

        if !is_active {
            return Err(AgreementError::ProviderNotActive);
        }

        // Cross-contract call: check attester is authorized for provider
        let is_authorized: bool = env.invoke_contract(
            &provider_registry,
            &soroban_sdk::Symbol::new(&env, "check_attester"),
            soroban_sdk::Vec::from_array(
                &env,
                [
                    attester.clone().into_val(&env),
                    provider.clone().into_val(&env),
                ],
            ),
        );

        if !is_authorized {
            return Err(AgreementError::AttesterNotAuthorized);
        }

        // Allocate agreement ID
        let agreement_id: u64 = env
            .storage()
            .instance()
            .get(&DataKey::NextAgreementId)
            .unwrap_or(1u64);

        env.storage()
            .instance()
            .set(&DataKey::NextAgreementId, &(agreement_id + 1));

        // Create agreement record
        let agreement = Agreement {
            sponsor,
            provider: provider.clone(),
            attester,
            patient_ref_commitment,
            service_commitment,
            funding_amount,
            settlement_amount,
            funding_deadline,
            care_deadline,
            dispute_window_secs,
            state: AgreementState::Requested,
            attestation_commitment: None,
            attested_by: None,
            attested_at: None,
            dispute_origin: MaybeDisputeOrigin::None,
            dispute_opened_by: None,
            dispute_opened_at: None,
            created_at: current_time,
        };

        let key = DataKey::Agreement(agreement_id);
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_cre"),), agreement_id);

        Ok(agreement_id)
    }

    /// Read an agreement record.
    pub fn get_agreement(env: Env, agreement_id: u64) -> Result<Agreement, AgreementError> {
        let key = DataKey::Agreement(agreement_id);
        let agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        Ok(agreement)
    }

    /// Fund an agreement by transferring from sponsor to contract.
    /// Requires sponsor authorization.
    /// Transitions state from Requested to Funded.
    pub fn fund(env: Env, agreement_id: u64, sponsor: Address) -> Result<(), AgreementError> {
        sponsor.require_auth();

        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is Requested
        if agreement.state != AgreementState::Requested {
            return Err(AgreementError::InvalidState);
        }

        // Validate sponsor matches
        if agreement.sponsor != sponsor {
            return Err(AgreementError::Unauthorized);
        }

        // Validate funding deadline not passed
        let current_time = env.ledger().timestamp();
        if current_time > agreement.funding_deadline {
            return Err(AgreementError::FundingDeadlineNotPassed);
        }

        // Load provider registry and check provider still active
        let provider_registry: Address = env
            .storage()
            .instance()
            .get(&DataKey::ProviderRegistry)
            .ok_or(AgreementError::Unauthorized)?;

        use soroban_sdk::IntoVal;
        let is_active: bool = env.invoke_contract(
            &provider_registry,
            &soroban_sdk::Symbol::new(&env, "is_provider_active"),
            soroban_sdk::Vec::from_array(&env, [agreement.provider.clone().into_val(&env)]),
        );

        if !is_active {
            return Err(AgreementError::ProviderNotActive);
        }

        // Load settlement asset (token contract address)
        let settlement_asset: Address = env
            .storage()
            .instance()
            .get(&DataKey::SettlementAsset)
            .ok_or(AgreementError::Unauthorized)?;

        // Transfer tokens from sponsor to this contract
        // Use cross-contract call to invoke token's transfer function
        env.invoke_contract::<()>(
            &settlement_asset,
            &soroban_sdk::Symbol::new(&env, "transfer"),
            soroban_sdk::Vec::from_array(
                &env,
                [
                    sponsor.clone().into_val(&env),
                    env.current_contract_address().into_val(&env),
                    agreement.funding_amount.into_val(&env),
                ],
            ),
        );

        // Update agreement state to Funded
        agreement.state = AgreementState::Funded;
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_fun"),), agreement_id);

        Ok(())
    }

    /// Cancel an agreement in Requested state.
    /// Requires provider authorization.
    /// Transitions state from Requested to Cancelled.
    pub fn cancel(env: Env, agreement_id: u64, provider: Address) -> Result<(), AgreementError> {
        provider.require_auth();

        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is Requested
        if agreement.state != AgreementState::Requested {
            return Err(AgreementError::InvalidState);
        }

        // Validate provider matches
        if agreement.provider != provider {
            return Err(AgreementError::Unauthorized);
        }

        // Update agreement state to Cancelled
        agreement.state = AgreementState::Cancelled;
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_can"),), agreement_id);

        Ok(())
    }

    /// Expire an agreement when deadlines have passed.
    /// Callable by anyone.
    /// - For Requested: allowed after funding_deadline; no token transfer.
    /// - For Funded: allowed after care_deadline + dispute_window_secs; refunds full deposit to sponsor.
    /// - For CareConfirmed: rejected (attested care cannot be expired).
    pub fn expire(env: Env, agreement_id: u64) -> Result<(), AgreementError> {
        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        let current_time = env.ledger().timestamp();

        match agreement.state {
            AgreementState::Requested => {
                if current_time <= agreement.funding_deadline {
                    return Err(AgreementError::FundingDeadlineNotReached);
                }
                agreement.state = AgreementState::Expired;
            }
            AgreementState::Funded => {
                let dispute_window_end = agreement
                    .care_deadline
                    .checked_add(agreement.dispute_window_secs)
                    .ok_or(AgreementError::ArithmeticOverflow)?;

                if current_time <= agreement.care_deadline {
                    return Err(AgreementError::CareDeadlineNotPassed);
                }
                if current_time <= dispute_window_end {
                    return Err(AgreementError::DisputeWindowActive);
                }

                // Load settlement asset (token contract address)
                let settlement_asset: Address = env
                    .storage()
                    .instance()
                    .get(&DataKey::SettlementAsset)
                    .ok_or(AgreementError::Unauthorized)?;

                // Atomically refund full funding_amount from contract to sponsor
                use soroban_sdk::IntoVal;
                env.invoke_contract::<()>(
                    &settlement_asset,
                    &soroban_sdk::Symbol::new(&env, "transfer"),
                    soroban_sdk::Vec::from_array(
                        &env,
                        [
                            env.current_contract_address().into_val(&env),
                            agreement.sponsor.clone().into_val(&env),
                            agreement.funding_amount.into_val(&env),
                        ],
                    ),
                );

                agreement.state = AgreementState::Expired;
            }
            _ => {
                return Err(AgreementError::InvalidState);
            }
        }

        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_exp"),), agreement_id);

        Ok(())
    }

    /// Attest that care has been provided.
    /// Requires attester authorization.
    /// Transitions state from Funded to CareConfirmed.
    /// Records attestation commitment, attester address, and attestation timestamp.
    pub fn attest_care(
        env: Env,
        agreement_id: u64,
        attester: Address,
        attestation_commitment: BytesN<32>,
    ) -> Result<(), AgreementError> {
        attester.require_auth();

        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is Funded
        if agreement.state != AgreementState::Funded {
            return Err(AgreementError::InvalidState);
        }

        // Validate attester matches
        if agreement.attester != attester {
            return Err(AgreementError::Unauthorized);
        }

        // Validate current time is before care deadline
        let current_time = env.ledger().timestamp();
        if current_time > agreement.care_deadline {
            return Err(AgreementError::CareDeadlineNotPassed);
        }

        // Record attestation
        agreement.attestation_commitment = Some(attestation_commitment);
        agreement.attested_by = Some(attester.clone());
        agreement.attested_at = Some(current_time);

        // Update agreement state to CareConfirmed
        agreement.state = AgreementState::CareConfirmed;
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_att"),), agreement_id);

        Ok(())
    }

    /// Open a dispute on an agreement.
    /// Requires authorization from sponsor or provider.
    /// Transitions state from Funded or CareConfirmed to Disputed.
    /// Records dispute origin state, opener address, and dispute timestamp.
    pub fn open_dispute(
        env: Env,
        agreement_id: u64,
        opener: Address,
    ) -> Result<(), AgreementError> {
        opener.require_auth();

        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is Funded or CareConfirmed
        if agreement.state != AgreementState::Funded
            && agreement.state != AgreementState::CareConfirmed
        {
            return Err(AgreementError::InvalidState);
        }

        // Validate opener is sponsor or provider
        if agreement.sponsor != opener && agreement.provider != opener {
            return Err(AgreementError::Unauthorized);
        }

        // Validate current time is within dispute window (after care deadline but before dispute window closes)
        let current_time = env.ledger().timestamp();
        let dispute_window_end = agreement
            .care_deadline
            .checked_add(agreement.dispute_window_secs)
            .ok_or(AgreementError::ArithmeticOverflow)?;

        if current_time <= agreement.care_deadline {
            return Err(AgreementError::DisputeWindowActive);
        }
        if current_time > dispute_window_end {
            return Err(AgreementError::DisputeWindowClosed);
        }

        // Record dispute origin state
        let origin = match agreement.state {
            AgreementState::Funded => DisputeOrigin::Funded,
            AgreementState::CareConfirmed => DisputeOrigin::CareConfirmed,
            _ => return Err(AgreementError::InvalidState),
        };

        // Record dispute info
        agreement.dispute_origin = MaybeDisputeOrigin::Some(origin);
        agreement.dispute_opened_by = Some(opener.clone());
        agreement.dispute_opened_at = Some(current_time);

        // Update agreement state to Disputed
        agreement.state = AgreementState::Disputed;
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_dis"),), agreement_id);

        Ok(())
    }

    /// Settle an agreement by transferring settlement amount to provider
    /// and returning any surplus (funding_amount - settlement_amount) to sponsor.
    /// Callable deterministically by anyone after the dispute window has elapsed.
    /// Transitions state from CareConfirmed to Settled.
    pub fn settle(env: Env, agreement_id: u64) -> Result<(), AgreementError> {
        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is CareConfirmed
        if agreement.state != AgreementState::CareConfirmed {
            return Err(AgreementError::InvalidState);
        }

        // Validate current time is past the dispute window
        let current_time = env.ledger().timestamp();
        let dispute_window_end = agreement
            .care_deadline
            .checked_add(agreement.dispute_window_secs)
            .ok_or(AgreementError::ArithmeticOverflow)?;

        if current_time <= agreement.care_deadline {
            return Err(AgreementError::CareDeadlineNotPassed);
        }
        if current_time <= dispute_window_end {
            return Err(AgreementError::DisputeWindowActive);
        }

        // Load settlement asset (token contract address)
        let settlement_asset: Address = env
            .storage()
            .instance()
            .get(&DataKey::SettlementAsset)
            .ok_or(AgreementError::Unauthorized)?;

        use soroban_sdk::IntoVal;

        // 1. Transfer settlement amount from contract to provider
        env.invoke_contract::<()>(
            &settlement_asset,
            &soroban_sdk::Symbol::new(&env, "transfer"),
            soroban_sdk::Vec::from_array(
                &env,
                [
                    env.current_contract_address().into_val(&env),
                    agreement.provider.clone().into_val(&env),
                    agreement.settlement_amount.into_val(&env),
                ],
            ),
        );

        // 2. Refund surplus (funding_amount - settlement_amount) to sponsor if positive
        let surplus = agreement
            .funding_amount
            .checked_sub(agreement.settlement_amount)
            .ok_or(AgreementError::ArithmeticOverflow)?;
        if surplus > 0 {
            env.invoke_contract::<()>(
                &settlement_asset,
                &soroban_sdk::Symbol::new(&env, "transfer"),
                soroban_sdk::Vec::from_array(
                    &env,
                    [
                        env.current_contract_address().into_val(&env),
                        agreement.sponsor.clone().into_val(&env),
                        surplus.into_val(&env),
                    ],
                ),
            );
        }

        // Update agreement state to Settled
        agreement.state = AgreementState::Settled;
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_set"),), agreement_id);

        Ok(())
    }

    /// Resolve a dispute on an agreement.
    /// Requires admin authorization.
    /// Transitions state from Disputed to appropriate final state based on resolution.
    /// Performs necessary token transfers based on resolution action.
    pub fn resolve_dispute(
        env: Env,
        agreement_id: u64,
        resolution: DisputeResolution,
    ) -> Result<(), AgreementError> {
        // Load admin
        let admin: Address = env
            .storage()
            .instance()
            .get(&DataKey::Admin)
            .ok_or(AgreementError::Unauthorized)?;

        admin.require_auth();

        // Load agreement
        let key = DataKey::Agreement(agreement_id);
        let mut agreement: Agreement = env
            .storage()
            .persistent()
            .get(&key)
            .ok_or(AgreementError::AgreementNotFound)?;

        // Validate state is Disputed
        if agreement.state != AgreementState::Disputed {
            return Err(AgreementError::InvalidState);
        }

        // Load settlement asset for transfers
        let settlement_asset: Address = env
            .storage()
            .instance()
            .get(&DataKey::SettlementAsset)
            .ok_or(AgreementError::Unauthorized)?;

        use soroban_sdk::IntoVal;

        // Execute resolution action
        match resolution {
            DisputeResolution::Resume => {
                // Resume from origin state: move back to CareConfirmed if originated from CareConfirmed,
                // or Funded if originated from Funded
                match agreement.dispute_origin {
                    MaybeDisputeOrigin::Some(DisputeOrigin::CareConfirmed) => {
                        agreement.state = AgreementState::CareConfirmed;
                    }
                    MaybeDisputeOrigin::Some(DisputeOrigin::Funded) => {
                        agreement.state = AgreementState::Funded;
                    }
                    MaybeDisputeOrigin::None => {
                        return Err(AgreementError::InvalidResolution);
                    }
                }
            }
            DisputeResolution::Settle => {
                // Settle: transfer settlement_amount to provider, return surplus to sponsor, mark as Settled
                env.invoke_contract::<()>(
                    &settlement_asset,
                    &soroban_sdk::Symbol::new(&env, "transfer"),
                    soroban_sdk::Vec::from_array(
                        &env,
                        [
                            env.current_contract_address().into_val(&env),
                            agreement.provider.clone().into_val(&env),
                            agreement.settlement_amount.into_val(&env),
                        ],
                    ),
                );
                let surplus = agreement
                    .funding_amount
                    .checked_sub(agreement.settlement_amount)
                    .ok_or(AgreementError::ArithmeticOverflow)?;
                if surplus > 0 {
                    env.invoke_contract::<()>(
                        &settlement_asset,
                        &soroban_sdk::Symbol::new(&env, "transfer"),
                        soroban_sdk::Vec::from_array(
                            &env,
                            [
                                env.current_contract_address().into_val(&env),
                                agreement.sponsor.clone().into_val(&env),
                                surplus.into_val(&env),
                            ],
                        ),
                    );
                }
                agreement.state = AgreementState::Settled;
            }
            DisputeResolution::Refund => {
                // Refund: transfer funding_amount back to sponsor and mark as Refunded
                env.invoke_contract::<()>(
                    &settlement_asset,
                    &soroban_sdk::Symbol::new(&env, "transfer"),
                    soroban_sdk::Vec::from_array(
                        &env,
                        [
                            env.current_contract_address().into_val(&env),
                            agreement.sponsor.clone().into_val(&env),
                            agreement.funding_amount.into_val(&env),
                        ],
                    ),
                );
                agreement.state = AgreementState::Refunded;
            }
        }

        // Update agreement with resolution and save
        env.storage().persistent().set(&key, &agreement);
        env.storage().persistent().extend_ttl(
            &key,
            PERSISTENT_TTL_THRESHOLD,
            PERSISTENT_TTL_EXTEND_TO,
        );

        env.storage()
            .instance()
            .extend_ttl(INSTANCE_TTL_THRESHOLD, INSTANCE_TTL_EXTEND_TO);

        env.events()
            .publish((symbol_short!("agr_res"),), agreement_id);

        Ok(())
    }
}
