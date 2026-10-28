#![cfg(test)]
// Genuine-signature verification that the care-agreement admin can be a
// 2-of-3 multisignature classic Stellar account.
//
// Every authorization here is a real Ed25519 signature checked by
// `soroban-env-host`'s account authentication path against the account's
// medium threshold (see `multisig_testutils`). None of these tests use
// `mock_all_auths()` or `mock_auths()` for the operation under test —
// `mock_all_auths()` is used only in `setup()` to fund/create the
// agreement fixture itself, which is not part of what is being verified.
extern crate std;

use crate::multisig_testutils::{
    create_multisig_account, generate_signer, medium_threshold, sign_entry,
};
use crate::{
    AgreementState, CareAgreementContract, CareAgreementContractClient, DisputeResolution,
};
use ed25519_dalek::SigningKey;
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient};
use soroban_sdk::testutils::{Address as _, Ledger};
use soroban_sdk::token::{Client as TokenClient, StellarAssetClient};
use soroban_sdk::{Address, BytesN, Env, IntoVal};

const FUNDING: i128 = 2000_0000000;
const SETTLEMENT: i128 = 1800_0000000;
const SURPLUS: i128 = FUNDING - SETTLEMENT;
const DISPUTE_WINDOW: u64 = 3600;

struct Fixture<'a> {
    env: &'a Env,
    agreement: CareAgreementContractClient<'a>,
    s1: SigningKey,
    s2: SigningKey,
    s3: SigningKey,
    admin: Address,
    // Second, independent 2-of-3 multisig account used as the rotation
    // target in the `transfer_admin` scenarios.
    n1: SigningKey,
    n2: SigningKey,
    n3: SigningKey,
    new_admin: Address,
    token: TokenClient<'a>,
    provider: Address,
    sponsor: Address,
    agreement_id: u64,
    care_deadline: u64,
}

impl<'a> Fixture<'a> {
    fn snapshot(&self) -> (i128, i128, i128) {
        (
            self.token.balance(&self.agreement.address),
            self.token.balance(&self.sponsor),
            self.token.balance(&self.provider),
        )
    }

    fn open_dispute(&self) {
        self.env.mock_all_auths();
        self.env
            .ledger()
            .with_mut(|l| l.timestamp = self.care_deadline + 1);
        self.agreement
            .open_dispute(&self.agreement_id, &self.sponsor);
    }

    /// Signs and submits `resolve_dispute` with exactly `signers`' real
    /// Ed25519 signatures against `account` (whichever multisig account
    /// currently holds the admin role), using a fresh nonce each call.
    fn resolve_as(
        &self,
        account: &Address,
        nonce: i64,
        signers: &[&SigningKey],
        resolution: &DisputeResolution,
    ) -> Result<(), ()> {
        let args = (self.agreement_id, resolution.clone()).into_val(self.env);
        let entry = sign_entry(
            self.env,
            account,
            signers,
            nonce,
            &self.agreement.address,
            "resolve_dispute",
            args,
        );
        self.env.set_auths(&[entry]);
        self.agreement
            .try_resolve_dispute(&self.agreement_id, resolution)
            .map(|_| ())
            .map_err(|_| ())
    }

    /// Convenience wrapper for `resolve_as` against the fixture's original
    /// admin account.
    fn resolve_with(
        &self,
        nonce: i64,
        signers: &[&SigningKey],
        resolution: &DisputeResolution,
    ) -> Result<(), ()> {
        self.resolve_as(&self.admin, nonce, signers, resolution)
    }

    /// Signs and submits `transfer_admin(new_admin)`, with `old_signers`
    /// authorizing as the current (multisig) admin and `new_signers`
    /// authorizing as the incoming (also multisig) admin. Passing an empty
    /// `new_signers` omits that credential entirely, exercising the "new
    /// admin never accepted" rejection path.
    fn rotate_with(
        &self,
        nonce: i64,
        old_signers: &[&SigningKey],
        new_signers: &[&SigningKey],
    ) -> Result<(), ()> {
        let args: soroban_sdk::Vec<soroban_sdk::Val> = (self.new_admin.clone(),).into_val(self.env);
        let mut entries = std::vec![sign_entry(
            self.env,
            &self.admin,
            old_signers,
            nonce,
            &self.agreement.address,
            "transfer_admin",
            args.clone(),
        )];
        if !new_signers.is_empty() {
            entries.push(sign_entry(
                self.env,
                &self.new_admin,
                new_signers,
                nonce,
                &self.agreement.address,
                "transfer_admin",
                args,
            ));
        }
        self.env.set_auths(&entries);
        self.agreement
            .try_transfer_admin(&self.new_admin)
            .map(|_| ())
            .map_err(|_| ())
    }
}

/// Real registry, real token, one funded agreement, admin is a 2-of-3
/// multisignature classic account (three independent Ed25519 keys, weight
/// 1 each, master key weight 0, low/medium/high thresholds all set to 2).
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

    let s1 = generate_signer();
    let s2 = generate_signer();
    let s3 = generate_signer();
    let admin = create_multisig_account(env, &[&s1, &s2, &s3], (2, 2, 2));
    assert_eq!(medium_threshold(env, &admin), 2);

    let n1 = generate_signer();
    let n2 = generate_signer();
    let n3 = generate_signer();
    let new_admin = create_multisig_account(env, &[&n1, &n2, &n3], (2, 2, 2));

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
        agreement,
        s1,
        s2,
        s3,
        admin,
        n1,
        n2,
        n3,
        new_admin,
        token: TokenClient::new(env, &token_id),
        provider,
        sponsor,
        agreement_id,
        care_deadline,
    }
}

// ---------------------------------------------------------------------------
// 1-4: raw threshold behavior via `resolve_dispute`
// ---------------------------------------------------------------------------

#[test]
fn two_valid_signers_authorize_resolve_dispute() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();

    let result = f.resolve_with(1, &[&f.s1, &f.s2], &DisputeResolution::Resume);
    assert!(result.is_ok());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Funded
    );
}

#[test]
fn single_signer_is_rejected() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();

    let result = f.resolve_with(1, &[&f.s1], &DisputeResolution::Resume);
    assert!(result.is_err());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Disputed
    );
}

#[test]
fn unknown_signer_cannot_supply_missing_authorization() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();

    let outsider = generate_signer();
    let result = f.resolve_with(1, &[&f.s1, &outsider], &DisputeResolution::Resume);
    assert!(result.is_err());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Disputed
    );
}

#[test]
fn losing_one_signer_still_leaves_two_valid_signers() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();

    // s1 is "lost" (never used below). s2 + s3 still clear the medium
    // threshold of 2 on their own.
    let result = f.resolve_with(1, &[&f.s2, &f.s3], &DisputeResolution::Resume);
    assert!(result.is_ok());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Funded
    );
}

// ---------------------------------------------------------------------------
// 5: resolving an existing dispute end to end (settle branch)
// ---------------------------------------------------------------------------

#[test]
fn multisig_admin_can_resolve_existing_dispute_by_settling() {
    let env = Env::default();
    let f = setup(&env);
    f.open_dispute();

    let before = f.snapshot();
    let result = f.resolve_with(1, &[&f.s1, &f.s3], &DisputeResolution::Settle);
    assert!(result.is_ok());

    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Settled
    );
    let after = f.snapshot();
    assert_eq!(before.0 - after.0, FUNDING);
    assert_eq!(after.1 - before.1, SURPLUS);
    assert_eq!(after.2 - before.2, SETTLEMENT);
}

// ---------------------------------------------------------------------------
// 6-9: `transfer_admin` between two multisignature accounts
// ---------------------------------------------------------------------------

#[test]
fn multisig_admin_can_authorize_transfer_admin() {
    let env = Env::default();
    let f = setup(&env);

    let result = f.rotate_with(1, &[&f.s1, &f.s2], &[&f.n1, &f.n2]);
    assert!(result.is_ok());
    assert_eq!(f.agreement.get_admin(), f.new_admin);
}

#[test]
fn incoming_admin_must_also_authorize_acceptance() {
    let env = Env::default();
    let f = setup(&env);

    // Old admin clears its threshold; incoming admin supplies no
    // authorization at all.
    let result = f.rotate_with(1, &[&f.s1, &f.s2], &[]);
    assert!(result.is_err());
    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn incoming_admin_below_its_own_threshold_is_rejected() {
    let env = Env::default();
    let f = setup(&env);

    // Old admin clears its threshold; incoming admin supplies only one of
    // its own three signers, which does not clear ITS medium threshold of
    // 2 either.
    let result = f.rotate_with(1, &[&f.s1, &f.s2], &[&f.n1]);
    assert!(result.is_err());
    assert_eq!(f.agreement.get_admin(), f.admin);
}

#[test]
fn previous_multisig_admin_loses_authority_after_rotation() {
    let env = Env::default();
    let f = setup(&env);

    let rotated = f.rotate_with(1, &[&f.s2, &f.s3], &[&f.n2, &f.n3]);
    assert!(rotated.is_ok());
    assert_eq!(f.agreement.get_admin(), f.new_admin);

    // The old admin's signers, even with a full 2-of-3 that used to be
    // sufficient, can no longer resolve a dispute: the stored admin is now
    // `new_admin`, so these signatures authenticate the wrong address for
    // `resolve_dispute`'s `admin.require_auth()` check.
    f.open_dispute();
    let result = f.resolve_with(2, &[&f.s1, &f.s2], &DisputeResolution::Resume);
    assert!(result.is_err());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Disputed
    );

    // The new admin's signers can.
    let result = f.resolve_as(&f.new_admin, 3, &[&f.n1, &f.n3], &DisputeResolution::Resume);
    assert!(result.is_ok());
    assert_eq!(
        f.agreement.get_agreement(&f.agreement_id).state,
        AgreementState::Funded
    );
}

// ---------------------------------------------------------------------------
// provider-registry scenario 5: a care agreement keeps behaving correctly
// after the *registry's* multisig admin rotates. (The registry's own
// authorization scenarios 1-4 are exercised directly in
// `provider_registry::test_multisig_admin`; this test only verifies the
// cross-contract side-effect that matters to care-agreement.)
// ---------------------------------------------------------------------------

#[test]
fn care_agreement_unaffected_by_registry_multisig_admin_rotation() {
    let env = Env::default();
    env.mock_all_auths();

    // A standalone registry, independent of the main fixture, whose own
    // admin is a 2-of-3 multisig account that gets rotated with real
    // signatures mid-test.
    let registry_id = env.register(ProviderRegistryContract, ());
    let registry = ProviderRegistryContractClient::new(&env, &registry_id);
    let r1 = generate_signer();
    let r2 = generate_signer();
    let r3 = generate_signer();
    let registry_admin = create_multisig_account(&env, &[&r1, &r2, &r3], (2, 2, 2));
    registry.initialize(&registry_admin);

    let provider = Address::generate(&env);
    registry.register_provider(&provider, &BytesN::from_array(&env, &[9u8; 32]));
    let attester = Address::generate(&env);
    registry.register_attester(&attester, &provider, &BytesN::from_array(&env, &[8u8; 32]));

    // Rotate the registry's admin with real signatures from both sides.
    let o1 = generate_signer();
    let o2 = generate_signer();
    let o3 = generate_signer();
    let new_registry_admin = create_multisig_account(&env, &[&o1, &o2, &o3], (2, 2, 2));
    let rotate_args: soroban_sdk::Vec<soroban_sdk::Val> =
        (new_registry_admin.clone(),).into_val(&env);
    let old_entry = sign_entry(
        &env,
        &registry_admin,
        &[&r1, &r2],
        1,
        &registry.address,
        "transfer_admin",
        rotate_args.clone(),
    );
    let new_entry = sign_entry(
        &env,
        &new_registry_admin,
        &[&o1, &o3],
        1,
        &registry.address,
        "transfer_admin",
        rotate_args,
    );
    env.set_auths(&[old_entry, new_entry]);
    registry.transfer_admin(&new_registry_admin);
    assert_eq!(registry.get_admin(), new_registry_admin);

    // The provider/attester records, and a fresh agreement built against
    // them, are unaffected by the registry admin having rotated.
    env.mock_all_auths();
    let token_id = env.register_stellar_asset_contract(Address::generate(&env));
    let sponsor = Address::generate(&env);
    StellarAssetClient::new(&env, &token_id).mint(&sponsor, &FUNDING);

    let agreement_addr = env.register(CareAgreementContract, ());
    let agreement = CareAgreementContractClient::new(&env, &agreement_addr);
    let agreement_admin = Address::generate(&env);
    agreement.initialize(&agreement_admin, &registry.address, &token_id);

    let now = env.ledger().timestamp();
    let funding_deadline = now + 86400;
    let care_deadline = funding_deadline + 86400;
    let agreement_id = agreement.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &FUNDING,
        &SETTLEMENT,
        &funding_deadline,
        &care_deadline,
        &DISPUTE_WINDOW,
    );
    agreement.fund(&agreement_id, &sponsor);
    agreement.attest_care(
        &agreement_id,
        &attester,
        &BytesN::from_array(&env, &[5u8; 32]),
    );
    env.ledger()
        .with_mut(|l| l.timestamp = care_deadline + DISPUTE_WINDOW + 1);
    agreement.settle(&agreement_id);

    assert_eq!(
        agreement.get_agreement(&agreement_id).state,
        AgreementState::Settled
    );
}

#[test]
fn escrow_balances_are_unchanged_by_a_multisig_admin_handover() {
    let env = Env::default();
    let f = setup(&env);

    let before = f.snapshot();
    let result = f.rotate_with(1, &[&f.s1, &f.s3], &[&f.n1, &f.n3]);
    assert!(result.is_ok());
    let after = f.snapshot();

    // Rotation touches only the admin address; no token moves.
    assert_eq!(before, after);
    assert_eq!(after.0, FUNDING);

    // And the handover did not disturb the agreement's own state or
    // amounts; the new admin can still settle it correctly afterwards.
    f.open_dispute();
    let settle_result = f.resolve_as(&f.new_admin, 2, &[&f.n1, &f.n2], &DisputeResolution::Settle);
    assert!(settle_result.is_ok());
    let final_balances = f.snapshot();
    assert_eq!(before.0 - final_balances.0, FUNDING);
    assert_eq!(final_balances.1 - before.1, SURPLUS);
    assert_eq!(final_balances.2 - before.2, SETTLEMENT);
}
