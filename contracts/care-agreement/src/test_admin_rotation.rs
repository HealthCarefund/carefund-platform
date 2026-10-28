#![cfg(test)]

//! Tests for `transfer_admin` on the care-agreement contract, and for how
//! rotating the provider-registry admin interacts with agreements.
//!
//! Setup uses `mock_all_auths()` for convenience. Every rotation and every
//! privileged call under test then replaces that with `mock_auths(...)`
//! naming exactly the addresses that signed, so a call only succeeds if the
//! intended signatures are present. The old-admin rejection tests double as
//! proof that the targeted mock really replaces the permissive one.

use crate::{
    AgreementError, AgreementState, CareAgreementContract, CareAgreementContractClient,
    DisputeResolution,
};
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient};
use soroban_sdk::testutils::{Address as _, Ledger, MockAuth, MockAuthInvoke};
use soroban_sdk::token::{Client as TokenClient, StellarAssetClient};
use soroban_sdk::{Address, BytesN, Env, IntoVal, Val, Vec};

const FUNDING: i128 = 2000_0000000;
const SETTLEMENT: i128 = 1800_0000000;
const SURPLUS: i128 = FUNDING - SETTLEMENT;
const DISPUTE_WINDOW: u64 = 3600;

/// Only `signer` authorizes this exact invocation.
fn mock_one(env: &Env, signer: &Address, contract: &Address, fn_name: &str, args: Vec<Val>) {
    let invoke = MockAuthInvoke {
        contract,
        fn_name,
        args,
        sub_invokes: &[],
    };
    env.mock_auths(&[MockAuth {
        address: signer,
        invoke: &invoke,
    }]);
}

/// Both `a` and `b` authorize this exact invocation.
fn mock_two(
    env: &Env,
    a: &Address,
    b: &Address,
    contract: &Address,
    fn_name: &str,
    args: Vec<Val>,
) {
    let invoke = MockAuthInvoke {
        contract,
        fn_name,
        args,
        sub_invokes: &[],
    };
    env.mock_auths(&[
        MockAuth {
            address: a,
            invoke: &invoke,
        },
        MockAuth {
            address: b,
            invoke: &invoke,
        },
    ]);
}

struct Fixture<'a> {
    env: &'a Env,
    registry: ProviderRegistryContractClient<'a>,
    registry_admin: Address,
    agreement: CareAgreementContractClient<'a>,
    admin: Address,
    token: TokenClient<'a>,
    provider: Address,
    attester: Address,
    sponsor: Address,
    agreement_id: u64,
    care_deadline: u64,
}

impl<'a> Fixture<'a> {
    fn rotate(&self, from: &Address, to: &Address) {
        mock_two(
            self.env,
            from,
            to,
            &self.agreement.address,
            "transfer_admin",
            (to.clone(),).into_val(self.env),
        );
        self.agreement.transfer_admin(to);
    }

    fn resolve_args(&self, resolution: &DisputeResolution) -> Vec<Val> {
        (self.agreement_id, resolution.clone()).into_val(self.env)
    }

    fn set_time(&self, timestamp: u64) {
        self.env.ledger().with_mut(|l| l.timestamp = timestamp);
    }

    /// Moves the funded agreement into `Disputed`.
    fn open_dispute(&self) {
        self.env.mock_all_auths();
        self.set_time(self.care_deadline + 1);
        self.agreement
            .open_dispute(&self.agreement_id, &self.sponsor);
    }

    fn snapshot(&self) -> (crate::Agreement, i128, i128, i128) {
        (
            self.agreement.get_agreement(&self.agreement_id),
            self.token.balance(&self.agreement.address),
            self.token.balance(&self.sponsor),
            self.token.balance(&self.provider),
        )
    }
}

/// Real registry, real token, one funded agreement.
fn setup(env: &Env) -> Fixture<'_> {
    env.mock_all_auths();

    let registry_id = env.register(ProviderRegistryContract, ());
    let registry = ProviderRegistryContractClient::new(env, &registry_id);
    let registry_admin = Address::generate(env);
    registry.initialize(&registry_admin);

    let provider = Address::generate(env);
    registry.register_provider(&provider, &BytesN::from_array(env, &[1u8; 32]));
    let attester = Address::generate(env);
    registry.register_attester(&attester, &provider, &BytesN::from_array(env, &[2u8; 32]));

    let token_id = env.register_stellar_asset_contract(Address::generate(env));
    let sponsor = Address::generate(env);
    StellarAssetClient::new(env, &token_id).mint(&sponsor, &FUNDING);

    let agreement_addr = env.register(CareAgreementContract, ());
    let agreement = CareAgreementContractClient::new(env, &agreement_addr);
    let admin = Address::generate(env);
    agreement.initialize(&admin, &registry.address, &token_id);

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
        registry_admin,
        agreement,
        admin,
        token: TokenClient::new(env, &token_id),
        provider,
        attester,
        sponsor,
        agreement_id,
        care_deadline,
    }
}

// ---------------------------------------------------------------------------
// Authorization of the rotation itself
// ---------------------------------------------------------------------------

#[test]
fn rotation_succeeds_when_both_admins_authorize() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    f.rotate(&f.admin, &new_admin);

    // Read the recorded authorizations first: `env.auths()` reports the
    // last top-level invocation, and a later read call would replace it.
    // Both addresses must have authorized the rotation.
    let auths = env.auths();
    assert!(auths.iter().any(|(who, _)| *who == f.admin));
    assert!(auths.iter().any(|(who, _)| *who == new_admin));

    assert_eq!(f.agreement.get_admin(), new_admin);
}

#[test]
fn rotation_fails_without_current_admin_authorization() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    mock_one(
        &env,
        &new_admin,
        &f.agreement.address,
        "transfer_admin",
        (new_admin.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&new_admin).is_err());

    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn rotation_fails_without_new_admin_authorization() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    mock_one(
        &env,
        &f.admin,
        &f.agreement.address,
        "transfer_admin",
        (new_admin.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&new_admin).is_err());

    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn rotation_fails_with_no_authorization_at_all() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    env.mock_auths(&[]);
    assert!(f.agreement.try_transfer_admin(&new_admin).is_err());

    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn outsider_cannot_install_themselves_as_admin() {
    let env = Env::default();
    let f = setup(&env);
    let attacker = Address::generate(&env);

    // The attacker signs for themselves as the incoming admin, but cannot
    // produce the current admin's signature.
    mock_one(
        &env,
        &attacker,
        &f.agreement.address,
        "transfer_admin",
        (attacker.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&attacker).is_err());

    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn failed_rotation_leaves_agreements_and_balances_untouched() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let before = f.snapshot();
    let attacker = Address::generate(&env);

    mock_one(
        &env,
        &attacker,
        &f.agreement.address,
        "transfer_admin",
        (attacker.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&attacker).is_err());

    assert_eq!(f.snapshot(), before);
    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn rotating_to_the_current_admin_is_an_authorized_no_op() {
    let env = Env::default();
    let f = setup(&env);

    mock_one(
        &env,
        &f.admin,
        &f.agreement.address,
        "transfer_admin",
        (f.admin.clone(),).into_val(&env),
    );
    f.agreement.transfer_admin(&f.admin);
    assert_eq!(f.agreement.get_admin(), f.admin);

    // Still requires the admin's signature.
    mock_one(
        &env,
        &Address::generate(&env),
        &f.agreement.address,
        "transfer_admin",
        (f.admin.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&f.admin).is_err());
}

#[test]
fn rotation_on_uninitialized_contract_is_rejected() {
    let env = Env::default();
    env.mock_all_auths();
    let id = env.register(CareAgreementContract, ());
    let client = CareAgreementContractClient::new(&env, &id);

    let res = client.try_transfer_admin(&Address::generate(&env));
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

// ---------------------------------------------------------------------------
// Effect on privileges
// ---------------------------------------------------------------------------

#[test]
fn previous_admin_loses_dispute_resolution_after_rotation() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);
    let before = f.snapshot();

    let resolution = DisputeResolution::Refund;
    mock_one(
        &env,
        &f.admin,
        &f.agreement.address,
        "resolve_dispute",
        f.resolve_args(&resolution),
    );
    assert!(f
        .agreement
        .try_resolve_dispute(&f.agreement_id, &resolution)
        .is_err());

    assert_eq!(f.snapshot(), before);
    assert_eq!(before.0.state, AgreementState::Disputed);
}

#[test]
fn previous_admin_cannot_rotate_again() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    let third = Address::generate(&env);
    mock_two(
        &env,
        &f.admin,
        &third,
        &f.agreement.address,
        "transfer_admin",
        (third.clone(),).into_val(&env),
    );
    assert!(f.agreement.try_transfer_admin(&third).is_err());
    assert_eq!(f.agreement.get_admin(), new_admin);
}

#[test]
fn new_admin_resolves_an_existing_disputed_agreement() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    let resolution = DisputeResolution::Refund;
    mock_one(
        &env,
        &new_admin,
        &f.agreement.address,
        "resolve_dispute",
        f.resolve_args(&resolution),
    );
    f.agreement.resolve_dispute(&f.agreement_id, &resolution);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Refunded
    );
    assert_eq!(f.token.balance(&f.sponsor), FUNDING);
    assert_eq!(f.token.balance(&f.provider), 0);
    assert_eq!(f.token.balance(&f.agreement.address), 0);
}

#[test]
fn new_admin_settle_resolution_pays_provider_and_returns_surplus() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    let resolution = DisputeResolution::Settle;
    mock_one(
        &env,
        &new_admin,
        &f.agreement.address,
        "resolve_dispute",
        f.resolve_args(&resolution),
    );
    f.agreement.resolve_dispute(&f.agreement_id, &resolution);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Settled
    );
    assert_eq!(f.token.balance(&f.provider), SETTLEMENT);
    assert_eq!(f.token.balance(&f.sponsor), SURPLUS);
    assert_eq!(f.token.balance(&f.agreement.address), 0);
}

#[test]
fn rotation_changes_no_agreement_state_and_moves_no_tokens() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let before = f.snapshot();

    let b = Address::generate(&env);
    f.rotate(&f.admin, &b);
    assert_eq!(f.snapshot(), before);

    let c = Address::generate(&env);
    f.rotate(&b, &c);
    assert_eq!(f.snapshot(), before);
}

#[test]
fn successive_rotations_each_hand_over_control() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();
    let b = Address::generate(&env);
    let c = Address::generate(&env);

    f.rotate(&f.admin, &b);
    f.rotate(&b, &c);
    assert_eq!(f.agreement.get_admin(), c);

    // Neither earlier admin can resolve; the latest one can.
    let resolution = DisputeResolution::Resume;
    for old in [&f.admin, &b] {
        mock_one(
            &env,
            old,
            &f.agreement.address,
            "resolve_dispute",
            f.resolve_args(&resolution),
        );
        assert!(f
            .agreement
            .try_resolve_dispute(&f.agreement_id, &resolution)
            .is_err());
    }
    mock_one(
        &env,
        &c,
        &f.agreement.address,
        "resolve_dispute",
        f.resolve_args(&resolution),
    );
    f.agreement.resolve_dispute(&f.agreement_id, &resolution);
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Funded
    );
}

#[test]
fn rotation_does_not_change_settlement_or_expiry_for_other_callers() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    // Permissionless expiry still works and still refunds the sponsor.
    env.mock_all_auths();
    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.expire(&f.agreement_id);

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Expired
    );
    assert_eq!(f.token.balance(&f.sponsor), FUNDING);
    assert_eq!(f.token.balance(&f.agreement.address), 0);
}

// ---------------------------------------------------------------------------
// The two administrators are independent
// ---------------------------------------------------------------------------

#[test]
fn rotating_the_agreement_admin_does_not_change_the_registry_admin() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    f.rotate(&f.admin, &new_admin);

    assert_eq!(f.registry.get_admin(), f.registry_admin);
    assert_eq!(f.agreement.get_admin(), new_admin);
}

#[test]
fn registry_admin_rotation_keeps_existing_agreement_flow_working() {
    let env = Env::default();
    let f = setup(&env);
    let new_registry_admin = Address::generate(&env);

    mock_two(
        &env,
        &f.registry_admin,
        &new_registry_admin,
        &f.registry.address,
        "transfer_admin",
        (new_registry_admin.clone(),).into_val(&env),
    );
    f.registry.transfer_admin(&new_registry_admin);
    assert_eq!(f.registry.get_admin(), new_registry_admin);
    // The agreement contract's own admin is a separate setting.
    assert_eq!(f.agreement.get_admin(), f.admin);

    // Existing registry records and the already-funded agreement keep working.
    assert!(f.registry.check_attester(&f.attester, &f.provider));
    env.mock_all_auths();
    f.agreement.attest_care(
        &f.agreement_id,
        &f.attester,
        &BytesN::from_array(&env, &[9u8; 32]),
    );
    f.set_time(f.care_deadline + DISPUTE_WINDOW + 1);
    f.agreement.settle(&f.agreement_id);
    assert_eq!(f.token.balance(&f.provider), SETTLEMENT);
    assert_eq!(f.token.balance(&f.sponsor), SURPLUS);
    assert_eq!(f.token.balance(&f.agreement.address), 0);
}

#[test]
fn new_registry_admin_can_revoke_an_attester_and_block_new_attestations() {
    let env = Env::default();
    let f = setup(&env);
    let new_registry_admin = Address::generate(&env);

    mock_two(
        &env,
        &f.registry_admin,
        &new_registry_admin,
        &f.registry.address,
        "transfer_admin",
        (new_registry_admin.clone(),).into_val(&env),
    );
    f.registry.transfer_admin(&new_registry_admin);

    // The previous registry admin can no longer revoke.
    mock_one(
        &env,
        &f.registry_admin,
        &f.registry.address,
        "revoke_attester",
        (f.attester.clone(),).into_val(&env),
    );
    assert!(f.registry.try_revoke_attester(&f.attester).is_err());

    // The new one can, and the care-agreement contract sees it at attestation.
    mock_one(
        &env,
        &new_registry_admin,
        &f.registry.address,
        "revoke_attester",
        (f.attester.clone(),).into_val(&env),
    );
    f.registry.revoke_attester(&f.attester);

    env.mock_all_auths();
    assert_eq!(
        f.agreement.try_attest_care(
            &f.agreement_id,
            &f.attester,
            &BytesN::from_array(&env, &[9u8; 32])
        ),
        Err(Ok(AgreementError::AttesterNotAuthorized))
    );
}
