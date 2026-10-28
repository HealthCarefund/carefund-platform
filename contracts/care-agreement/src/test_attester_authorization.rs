#![cfg(test)]

//! Tests for attester authorization at attestation time.
//!
//! `create_agreement` checks the attester against the real provider
//! registry. These tests cover the case where the registry status changes
//! after the agreement exists: `attest_care` must use the attester's
//! current registry record, while settlement, expiry and dispute
//! resolution stay independent of it.
//!
//! All tests register the real `provider-registry` contract and a real
//! Stellar Asset Contract, so the cross-contract path and token balances
//! are exercised, not stubbed.

use crate::{
    AgreementError, AgreementState, CareAgreementContract, CareAgreementContractClient,
    DisputeResolution,
};
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient, RegistryError};
use soroban_sdk::testutils::{Address as _, Ledger};
use soroban_sdk::token::{Client as TokenClient, StellarAssetClient};
use soroban_sdk::{contract, contractimpl, Address, BytesN, Env};

const FUNDING: i128 = 2000_0000000;
const SETTLEMENT: i128 = 1800_0000000;
const SURPLUS: i128 = FUNDING - SETTLEMENT;
const DISPUTE_WINDOW: u64 = 3600;

struct Fixture<'a> {
    env: &'a Env,
    registry: ProviderRegistryContractClient<'a>,
    agreement: CareAgreementContractClient<'a>,
    token: TokenClient<'a>,
    provider: Address,
    attester: Address,
    sponsor: Address,
    agreement_id: u64,
    care_deadline: u64,
}

impl<'a> Fixture<'a> {
    fn commitment(&self, byte: u8) -> BytesN<32> {
        BytesN::from_array(self.env, &[byte; 32])
    }

    fn set_time(&self, timestamp: u64) {
        self.env.ledger().with_mut(|l| l.timestamp = timestamp);
    }

    fn contract_balance(&self) -> i128 {
        self.token.balance(&self.agreement.address)
    }

    /// The agreement is still `Funded`, nothing was recorded, and the
    /// escrow is untouched.
    fn assert_still_funded_and_unattested(&self) {
        let a = self.agreement.get_agreement(&self.agreement_id);
        assert_eq!(a.state, AgreementState::Funded);
        assert_eq!(a.attestation_commitment, None);
        assert_eq!(a.attested_by, None);
        assert_eq!(a.attested_at, None);
        assert_eq!(self.contract_balance(), FUNDING);
        assert_eq!(self.token.balance(&self.sponsor), 0);
        assert_eq!(self.token.balance(&self.provider), 0);
    }
}

/// Registers the registry, a provider, an attester bound to it, a token,
/// and the agreement contract; creates and funds one agreement.
fn setup(env: &Env) -> Fixture<'_> {
    env.mock_all_auths();

    let registry_id = env.register(ProviderRegistryContract, ());
    let registry = ProviderRegistryContractClient::new(env, &registry_id);
    registry.initialize(&Address::generate(env));

    let provider = Address::generate(env);
    registry.register_provider(&provider, &BytesN::from_array(env, &[1u8; 32]));
    let attester = Address::generate(env);
    registry.register_attester(&attester, &provider, &BytesN::from_array(env, &[2u8; 32]));

    let token_id = env.register_stellar_asset_contract(Address::generate(env));
    let sponsor = Address::generate(env);
    StellarAssetClient::new(env, &token_id).mint(&sponsor, &FUNDING);

    let agreement_id_addr = env.register(CareAgreementContract, ());
    let agreement = CareAgreementContractClient::new(env, &agreement_id_addr);
    agreement.initialize(&Address::generate(env), &registry.address, &token_id);

    let now = env.ledger().timestamp();
    let funding_deadline = now + 86400;
    let care_deadline = funding_deadline + 86400;
    let agreement_id = agreement.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(env, &[3u8; 32]),
        &BytesN::from_array(env, &[4u8; 32]),
        &FUNDING,
        &SETTLEMENT,
        &funding_deadline,
        &care_deadline,
        &DISPUTE_WINDOW,
    );
    agreement.fund(&agreement_id, &sponsor);

    Fixture {
        env,
        registry,
        agreement,
        token: TokenClient::new(env, &token_id),
        provider,
        attester,
        sponsor,
        agreement_id,
        care_deadline,
    }
}

// ---------------------------------------------------------------------------
// Attestation authorization
// ---------------------------------------------------------------------------

#[test]
fn active_attester_can_attest_existing_funded_agreement() {
    let env = Env::default();
    let f = setup(&env);

    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));

    let a = f.agreement.get_agreement(&f.agreement_id);
    assert_eq!(a.state, AgreementState::CareConfirmed);
    assert_eq!(a.attestation_commitment, Some(f.commitment(9)));
    assert_eq!(a.attested_by, Some(f.attester.clone()));
    // Attestation moves no funds.
    assert_eq!(f.contract_balance(), FUNDING);
}

#[test]
fn suspended_attester_cannot_attest_existing_funded_agreement() {
    let env = Env::default();
    let f = setup(&env);

    f.registry.suspend_attester(&f.attester);

    let res = f
        .agreement
        .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    assert_eq!(res, Err(Ok(AgreementError::AttesterNotAuthorized)));
    f.assert_still_funded_and_unattested();
}

#[test]
fn revoked_attester_cannot_attest_existing_funded_agreement() {
    let env = Env::default();
    let f = setup(&env);

    f.registry.revoke_attester(&f.attester);

    let res = f
        .agreement
        .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    assert_eq!(res, Err(Ok(AgreementError::AttesterNotAuthorized)));
    f.assert_still_funded_and_unattested();
}

#[test]
fn reinstated_attester_can_attest_again_before_care_deadline() {
    let env = Env::default();
    let f = setup(&env);

    f.registry.suspend_attester(&f.attester);
    assert_eq!(
        f.agreement
            .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9)),
        Err(Ok(AgreementError::AttesterNotAuthorized))
    );
    f.assert_still_funded_and_unattested();

    f.registry.reinstate_attester(&f.attester);
    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::CareConfirmed
    );
}

#[test]
fn revoked_attester_cannot_be_reinstated_and_stays_blocked() {
    let env = Env::default();
    let f = setup(&env);

    f.registry.revoke_attester(&f.attester);

    assert_eq!(
        f.registry.try_reinstate_attester(&f.attester),
        Err(Ok(RegistryError::AttesterAlreadyRevoked))
    );
    assert_eq!(
        f.agreement
            .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9)),
        Err(Ok(AgreementError::AttesterNotAuthorized))
    );
    f.assert_still_funded_and_unattested();
}

#[test]
fn wrong_attester_is_still_rejected_even_if_active() {
    let env = Env::default();
    let f = setup(&env);

    // A second attester that is Active and bound to the same provider, but
    // is not the attester named in the agreement.
    let other = Address::generate(&env);
    f.registry
        .register_attester(&other, &f.provider, &BytesN::from_array(&env, &[7u8; 32]));

    let res = f
        .agreement
        .try_attest_care(&f.agreement_id, &other, &f.commitment(9));
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
    f.assert_still_funded_and_unattested();
}

#[test]
fn active_attester_is_still_rejected_after_care_deadline() {
    let env = Env::default();
    let f = setup(&env);

    f.set_time(f.care_deadline + 1);

    let res = f
        .agreement
        .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
    f.assert_still_funded_and_unattested();
}

#[test]
fn attestation_at_exact_care_deadline_still_allowed_for_active_attester() {
    let env = Env::default();
    let f = setup(&env);

    f.set_time(f.care_deadline);
    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::CareConfirmed
    );
}

#[test]
fn provider_suspension_alone_does_not_block_attestation() {
    let env = Env::default();
    let f = setup(&env);

    // Policy: attest_care checks the attester's own record and its binding
    // to the agreement's provider. It does not add a provider-status check,
    // so suspending the provider does not by itself stop a funded agreement
    // from being attested. (Registry `check_attester` would return false
    // here because it also requires an Active provider.)
    f.registry.suspend_provider(&f.provider);
    assert!(!f.registry.check_attester(&f.attester, &f.provider));

    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::CareConfirmed
    );
}

#[test]
fn registry_that_cannot_return_an_attester_record_fails_closed() {
    let env = Env::default();
    env.mock_all_auths();

    // A registry stand-in that approves creation and funding but whose
    // `get_attester` returns a value that is not an attester record.
    let bad_registry = env.register(MalformedRegistry, ());
    let provider = Address::generate(&env);
    let attester = Address::generate(&env);
    let sponsor = Address::generate(&env);

    let token_id = env.register_stellar_asset_contract(Address::generate(&env));
    StellarAssetClient::new(&env, &token_id).mint(&sponsor, &FUNDING);
    let agreement_addr = env.register(CareAgreementContract, ());
    let agreement = CareAgreementContractClient::new(&env, &agreement_addr);
    agreement.initialize(&Address::generate(&env), &bad_registry, &token_id);

    let now = env.ledger().timestamp();
    let id = agreement.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &FUNDING,
        &SETTLEMENT,
        &(now + 86400),
        &(now + 2 * 86400),
        &DISPUTE_WINDOW,
    );
    agreement.fund(&id, &sponsor);

    let res = agreement.try_attest_care(&id, &attester, &BytesN::from_array(&env, &[9u8; 32]));
    assert_eq!(res, Err(Ok(AgreementError::AttesterNotAuthorized)));
    let a = agreement.get_agreement(&id);
    assert_eq!(a.state, AgreementState::Funded);
    assert_eq!(a.attestation_commitment, None);
}

#[contract]
pub struct MalformedRegistry;

#[contractimpl]
impl MalformedRegistry {
    pub fn is_provider_active(_env: Env, _provider: Address) -> bool {
        true
    }

    pub fn check_attester(_env: Env, _attester: Address, _provider: Address) -> bool {
        true
    }

    pub fn get_attester(_env: Env, _attester: Address) -> u32 {
        7
    }
}

// ---------------------------------------------------------------------------
// Compatibility: paths that must not depend on the attester's current status
// ---------------------------------------------------------------------------

#[test]
fn funded_agreement_with_revoked_attester_can_still_expire_with_full_refund() {
    let env = Env::default();
    let f = setup(&env);

    f.registry.revoke_attester(&f.attester);
    assert_eq!(
        f.agreement
            .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9)),
        Err(Ok(AgreementError::AttesterNotAuthorized))
    );

    // Not yet expirable inside the dispute window.
    f.set_time(f.care_deadline + DISPUTE_WINDOW);
    assert_eq!(
        f.agreement.try_expire(&f.agreement_id),
        Err(Ok(AgreementError::DisputeWindowActive))
    );
    f.assert_still_funded_and_unattested();

    // After the window, anyone can expire and the sponsor gets everything.
    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.expire(&f.agreement_id);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Expired
    );
    assert_eq!(f.token.balance(&f.sponsor), FUNDING);
    assert_eq!(f.token.balance(&f.provider), 0);
    assert_eq!(f.contract_balance(), 0);
}

#[test]
fn accepted_attestation_survives_later_revocation_and_settles() {
    let env = Env::default();
    let f = setup(&env);

    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    f.registry.revoke_attester(&f.attester);

    // The recorded attestation is unchanged.
    let a = f.agreement.get_agreement(&f.agreement_id);
    assert_eq!(a.state, AgreementState::CareConfirmed);
    assert_eq!(a.attestation_commitment, Some(f.commitment(9)));
    assert_eq!(a.attested_by, Some(f.attester.clone()));

    // Settlement is still blocked by the dispute window, not by the registry.
    f.set_time(f.care_deadline + DISPUTE_WINDOW);
    assert_eq!(
        f.agreement.try_settle(&f.agreement_id),
        Err(Ok(AgreementError::DisputeWindowActive))
    );

    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.settle(&f.agreement_id);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Settled
    );
    assert_eq!(f.token.balance(&f.provider), SETTLEMENT);
    assert_eq!(f.token.balance(&f.sponsor), SURPLUS);
    assert_eq!(f.contract_balance(), 0);
}

#[test]
fn accepted_attestation_settles_after_attester_suspension_and_provider_revocation() {
    let env = Env::default();
    let f = setup(&env);

    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    f.registry.suspend_attester(&f.attester);
    f.registry.revoke_provider(&f.provider);

    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.settle(&f.agreement_id);

    assert_eq!(f.token.balance(&f.provider), SETTLEMENT);
    assert_eq!(f.token.balance(&f.sponsor), SURPLUS);
    assert_eq!(f.contract_balance(), 0);
}

#[test]
fn disputed_attested_agreement_still_resolved_by_admin_after_revocation() {
    let env = Env::default();
    let f = setup(&env);

    f.agreement
        .attest_care(&f.agreement_id, &f.attester, &f.commitment(9));
    f.set_time(f.care_deadline + 1);
    f.agreement.open_dispute(&f.agreement_id, &f.sponsor);
    f.registry.revoke_attester(&f.attester);

    // Not settleable or expirable while disputed.
    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    assert_eq!(
        f.agreement.try_settle(&f.agreement_id),
        Err(Ok(AgreementError::InvalidState))
    );
    assert_eq!(
        f.agreement.try_expire(&f.agreement_id),
        Err(Ok(AgreementError::InvalidState))
    );

    f.agreement
        .resolve_dispute(&f.agreement_id, &DisputeResolution::Refund);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Refunded
    );
    assert_eq!(f.token.balance(&f.sponsor), FUNDING);
    assert_eq!(f.token.balance(&f.provider), 0);
    assert_eq!(f.contract_balance(), 0);
}

#[test]
fn disputed_unattested_agreement_resumes_and_does_not_regain_attestation_with_revoked_attester() {
    let env = Env::default();
    let f = setup(&env);

    f.set_time(f.care_deadline + 1);
    f.agreement.open_dispute(&f.agreement_id, &f.provider);
    f.registry.revoke_attester(&f.attester);

    // Resume returns to Funded; resolve_dispute does not consult the registry.
    f.agreement
        .resolve_dispute(&f.agreement_id, &DisputeResolution::Resume);
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Funded
    );

    // Already past the care deadline, so attestation is closed regardless.
    assert_eq!(
        f.agreement
            .try_attest_care(&f.agreement_id, &f.attester, &f.commitment(9)),
        Err(Ok(AgreementError::CareDeadlineNotPassed))
    );

    // The funded agreement can still be expired for a full refund.
    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.expire(&f.agreement_id);
    assert_eq!(f.token.balance(&f.sponsor), FUNDING);
    assert_eq!(f.contract_balance(), 0);
}

#[test]
fn disputed_agreement_resolved_with_settle_pays_provider_after_revocation() {
    let env = Env::default();
    let f = setup(&env);

    f.set_time(f.care_deadline + 1);
    f.agreement.open_dispute(&f.agreement_id, &f.sponsor);
    f.registry.revoke_attester(&f.attester);

    f.agreement
        .resolve_dispute(&f.agreement_id, &DisputeResolution::Settle);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Settled
    );
    assert_eq!(f.token.balance(&f.provider), SETTLEMENT);
    assert_eq!(f.token.balance(&f.sponsor), SURPLUS);
    assert_eq!(f.contract_balance(), 0);
}
