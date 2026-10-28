#![cfg(test)]

//! Tests for `transfer_admin` on the provider-registry contract.
//!
//! Setup uses `mock_all_auths()`. Each call under test then uses
//! `mock_auths(...)` naming exactly the addresses that signed, so a call
//! only succeeds if the intended signatures are present.

use crate::{ActorStatus, ProviderRegistryContract, ProviderRegistryContractClient, RegistryError};
use soroban_sdk::testutils::{Address as _, MockAuth, MockAuthInvoke};
use soroban_sdk::{Address, BytesN, Env, IntoVal, Val, Vec};

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
    client: ProviderRegistryContractClient<'a>,
    admin: Address,
    provider: Address,
    attester: Address,
}

fn setup(env: &Env) -> Fixture<'_> {
    env.mock_all_auths();
    let id = env.register(ProviderRegistryContract, ());
    let client = ProviderRegistryContractClient::new(env, &id);
    let admin = Address::generate(env);
    client.initialize(&admin);

    let provider = Address::generate(env);
    client.register_provider(&provider, &BytesN::from_array(env, &[1u8; 32]));
    let attester = Address::generate(env);
    client.register_attester(&attester, &provider, &BytesN::from_array(env, &[2u8; 32]));

    Fixture {
        env,
        client,
        admin,
        provider,
        attester,
    }
}

impl<'a> Fixture<'a> {
    fn rotate(&self, from: &Address, to: &Address) {
        mock_two(
            self.env,
            from,
            to,
            &self.client.address,
            "transfer_admin",
            (to.clone(),).into_val(self.env),
        );
        self.client.transfer_admin(to);
    }

    fn as_admin(&self, who: &Address, fn_name: &str, arg: &Address) {
        mock_one(
            self.env,
            who,
            &self.client.address,
            fn_name,
            (arg.clone(),).into_val(self.env),
        );
    }
}

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

    assert_eq!(f.client.get_admin(), new_admin);
}

#[test]
fn rotation_fails_without_current_admin_authorization() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    f.as_admin(&new_admin, "transfer_admin", &new_admin);
    assert!(f.client.try_transfer_admin(&new_admin).is_err());
    assert_eq!(f.client.get_admin(), f.admin);
}

#[test]
fn rotation_fails_without_new_admin_authorization() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);

    f.as_admin(&f.admin, "transfer_admin", &new_admin);
    assert!(f.client.try_transfer_admin(&new_admin).is_err());
    assert_eq!(f.client.get_admin(), f.admin);
}

#[test]
fn rotation_fails_with_no_authorization_at_all() {
    let env = Env::default();
    let f = setup(&env);

    env.mock_auths(&[]);
    assert!(f
        .client
        .try_transfer_admin(&Address::generate(&env))
        .is_err());
    assert_eq!(f.client.get_admin(), f.admin);
}

#[test]
fn outsider_cannot_install_themselves_as_admin() {
    let env = Env::default();
    let f = setup(&env);
    let attacker = Address::generate(&env);

    f.as_admin(&attacker, "transfer_admin", &attacker);
    assert!(f.client.try_transfer_admin(&attacker).is_err());
    assert_eq!(f.client.get_admin(), f.admin);
}

#[test]
fn rotating_to_the_current_admin_is_an_authorized_no_op() {
    let env = Env::default();
    let f = setup(&env);

    f.as_admin(&f.admin, "transfer_admin", &f.admin);
    f.client.transfer_admin(&f.admin);
    assert_eq!(f.client.get_admin(), f.admin);

    f.as_admin(&Address::generate(&env), "transfer_admin", &f.admin);
    assert!(f.client.try_transfer_admin(&f.admin).is_err());
}

#[test]
fn rotation_on_uninitialized_contract_is_rejected() {
    let env = Env::default();
    env.mock_all_auths();
    let id = env.register(ProviderRegistryContract, ());
    let client = ProviderRegistryContractClient::new(&env, &id);

    let res = client.try_transfer_admin(&Address::generate(&env));
    assert_eq!(res, Err(Ok(RegistryError::Unauthorized)));
}

#[test]
fn previous_admin_loses_every_privileged_operation() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    let other_provider = Address::generate(&env);
    let other_attester = Address::generate(&env);

    f.as_admin(&f.admin, "suspend_provider", &f.provider);
    assert!(f.client.try_suspend_provider(&f.provider).is_err());
    f.as_admin(&f.admin, "revoke_provider", &f.provider);
    assert!(f.client.try_revoke_provider(&f.provider).is_err());
    f.as_admin(&f.admin, "suspend_attester", &f.attester);
    assert!(f.client.try_suspend_attester(&f.attester).is_err());
    f.as_admin(&f.admin, "revoke_attester", &f.attester);
    assert!(f.client.try_revoke_attester(&f.attester).is_err());

    let args: Vec<Val> =
        (other_provider.clone(), BytesN::from_array(&env, &[5u8; 32])).into_val(&env);
    mock_one(&env, &f.admin, &f.client.address, "register_provider", args);
    assert!(f
        .client
        .try_register_provider(&other_provider, &BytesN::from_array(&env, &[5u8; 32]))
        .is_err());

    let args: Vec<Val> = (
        other_attester.clone(),
        f.provider.clone(),
        BytesN::from_array(&env, &[6u8; 32]),
    )
        .into_val(&env);
    mock_one(&env, &f.admin, &f.client.address, "register_attester", args);
    assert!(f
        .client
        .try_register_attester(
            &other_attester,
            &f.provider,
            &BytesN::from_array(&env, &[6u8; 32])
        )
        .is_err());

    f.as_admin(&f.admin, "transfer_admin", &other_provider);
    assert!(f.client.try_transfer_admin(&other_provider).is_err());

    // Nothing changed.
    assert!(f.client.is_provider_active(&f.provider));
    assert!(f.client.check_attester(&f.attester, &f.provider));
    assert_eq!(f.client.get_admin(), new_admin);
}

#[test]
fn new_admin_can_use_every_privileged_operation_on_existing_records() {
    let env = Env::default();
    let f = setup(&env);
    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    // Existing records are readable and unchanged by the rotation.
    assert!(f.client.is_provider_active(&f.provider));
    assert!(f.client.check_attester(&f.attester, &f.provider));
    let attester_record = f.client.get_attester(&f.attester).unwrap();
    assert_eq!(attester_record.provider, f.provider);
    assert_eq!(attester_record.status, ActorStatus::Active);

    f.as_admin(&new_admin, "suspend_attester", &f.attester);
    f.client.suspend_attester(&f.attester);
    assert!(!f.client.check_attester(&f.attester, &f.provider));

    f.as_admin(&new_admin, "reinstate_attester", &f.attester);
    f.client.reinstate_attester(&f.attester);
    assert!(f.client.check_attester(&f.attester, &f.provider));

    f.as_admin(&new_admin, "suspend_provider", &f.provider);
    f.client.suspend_provider(&f.provider);
    assert!(!f.client.is_provider_active(&f.provider));

    f.as_admin(&new_admin, "reinstate_provider", &f.provider);
    f.client.reinstate_provider(&f.provider);
    assert!(f.client.is_provider_active(&f.provider));

    f.as_admin(&new_admin, "revoke_attester", &f.attester);
    f.client.revoke_attester(&f.attester);
    assert_eq!(
        f.client.get_attester(&f.attester).unwrap().status,
        ActorStatus::Revoked
    );
}

#[test]
fn revoked_status_stays_terminal_across_a_rotation() {
    let env = Env::default();
    let f = setup(&env);
    f.as_admin(&f.admin, "revoke_attester", &f.attester);
    f.client.revoke_attester(&f.attester);

    let new_admin = Address::generate(&env);
    f.rotate(&f.admin, &new_admin);

    f.as_admin(&new_admin, "reinstate_attester", &f.attester);
    assert_eq!(
        f.client.try_reinstate_attester(&f.attester),
        Err(Ok(RegistryError::AttesterAlreadyRevoked))
    );
}

#[test]
fn successive_rotations_each_hand_over_control() {
    let env = Env::default();
    let f = setup(&env);
    let b = Address::generate(&env);
    let c = Address::generate(&env);

    f.rotate(&f.admin, &b);
    f.rotate(&b, &c);
    assert_eq!(f.client.get_admin(), c);

    for old in [&f.admin, &b] {
        f.as_admin(old, "suspend_provider", &f.provider);
        assert!(f.client.try_suspend_provider(&f.provider).is_err());
    }
    f.as_admin(&c, "suspend_provider", &f.provider);
    f.client.suspend_provider(&f.provider);
    assert!(!f.client.is_provider_active(&f.provider));
}
