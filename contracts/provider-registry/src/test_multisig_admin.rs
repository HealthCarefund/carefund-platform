#![cfg(test)]
// Genuine-signature verification that the provider-registry admin can be a
// 2-of-3 multisignature classic Stellar account. See `multisig_testutils`
// for how the account and its signatures are built; none of this uses
// `mock_all_auths()` or `mock_auths()` for the operation under test.
extern crate std;

use crate::multisig_testutils::{
    create_multisig_account, generate_signer, medium_threshold, sign_entry,
};
use crate::{ActorStatus, ProviderRegistryContract, ProviderRegistryContractClient};
use ed25519_dalek::SigningKey;
use soroban_sdk::testutils::Address as _;
use soroban_sdk::{Address, BytesN, Env, IntoVal};

struct Fixture<'a> {
    env: &'a Env,
    client: ProviderRegistryContractClient<'a>,
    s1: SigningKey,
    s2: SigningKey,
    s3: SigningKey,
    admin: Address,
    n1: SigningKey,
    n2: SigningKey,
    n3: SigningKey,
    new_admin: Address,
    provider: Address,
    attester: Address,
}

impl<'a> Fixture<'a> {
    fn suspend_as(&self, account: &Address, nonce: i64, signers: &[&SigningKey]) -> Result<(), ()> {
        let args = (self.provider.clone(),).into_val(self.env);
        let entry = sign_entry(
            self.env,
            account,
            signers,
            nonce,
            &self.client.address,
            "suspend_provider",
            args,
        );
        self.env.set_auths(&[entry]);
        self.client
            .try_suspend_provider(&self.provider)
            .map(|_| ())
            .map_err(|_| ())
    }

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
            &self.client.address,
            "transfer_admin",
            args.clone(),
        )];
        if !new_signers.is_empty() {
            entries.push(sign_entry(
                self.env,
                &self.new_admin,
                new_signers,
                nonce,
                &self.client.address,
                "transfer_admin",
                args,
            ));
        }
        self.env.set_auths(&entries);
        self.client
            .try_transfer_admin(&self.new_admin)
            .map(|_| ())
            .map_err(|_| ())
    }
}

fn setup(env: &Env) -> Fixture<'_> {
    env.mock_all_auths();
    let id = env.register(ProviderRegistryContract, ());
    let client = ProviderRegistryContractClient::new(env, &id);

    let s1 = generate_signer();
    let s2 = generate_signer();
    let s3 = generate_signer();
    let admin = create_multisig_account(env, &[&s1, &s2, &s3], (2, 2, 2));
    assert_eq!(medium_threshold(env, &admin), 2);

    let n1 = generate_signer();
    let n2 = generate_signer();
    let n3 = generate_signer();
    let new_admin = create_multisig_account(env, &[&n1, &n2, &n3], (2, 2, 2));

    client.initialize(&admin);

    let provider = Address::generate(env);
    client.register_provider(&provider, &BytesN::from_array(env, &[1u8; 32]));
    let attester = Address::generate(env);
    client.register_attester(&attester, &provider, &BytesN::from_array(env, &[2u8; 32]));

    Fixture {
        env,
        client,
        s1,
        s2,
        s3,
        admin,
        n1,
        n2,
        n3,
        new_admin,
        provider,
        attester,
    }
}

// ---------------------------------------------------------------------------
// 1-2: a privileged registry operation under the real threshold check
// ---------------------------------------------------------------------------

#[test]
fn two_authorized_signers_can_suspend_a_provider() {
    let env = Env::default();
    let f = setup(&env);

    let result = f.suspend_as(&f.admin, 1, &[&f.s1, &f.s2]);
    assert!(result.is_ok());
    assert_eq!(
        f.client.get_provider(&f.provider).unwrap().status,
        ActorStatus::Suspended
    );
}

#[test]
fn one_signer_cannot_suspend_a_provider() {
    let env = Env::default();
    let f = setup(&env);

    let result = f.suspend_as(&f.admin, 1, &[&f.s1]);
    assert!(result.is_err());
    assert_eq!(
        f.client.get_provider(&f.provider).unwrap().status,
        ActorStatus::Active
    );
}

// ---------------------------------------------------------------------------
// 3-5: `transfer_admin` between two multisignature accounts
// ---------------------------------------------------------------------------

#[test]
fn multisig_admin_can_rotate_to_an_authorized_replacement() {
    let env = Env::default();
    let f = setup(&env);

    let result = f.rotate_with(1, &[&f.s1, &f.s3], &[&f.n1, &f.n3]);
    assert!(result.is_ok());
    assert_eq!(f.client.get_admin(), f.new_admin);
}

#[test]
fn existing_providers_and_attesters_remain_intact_after_rotation() {
    let env = Env::default();
    let f = setup(&env);

    let before_provider = f.client.get_provider(&f.provider);
    let before_attester = f.client.get_attester(&f.attester);

    let result = f.rotate_with(1, &[&f.s2, &f.s3], &[&f.n2, &f.n3]);
    assert!(result.is_ok());

    assert_eq!(f.client.get_provider(&f.provider), before_provider);
    assert_eq!(f.client.get_attester(&f.attester), before_attester);
    assert!(f.client.is_provider_active(&f.provider));
    assert!(f.client.check_attester(&f.attester, &f.provider));
}

#[test]
fn new_multisig_admin_can_still_operate_the_registry_after_rotation() {
    let env = Env::default();
    let f = setup(&env);

    let result = f.rotate_with(1, &[&f.s1, &f.s2], &[&f.n1, &f.n2]);
    assert!(result.is_ok());

    // The new admin's own threshold now governs privileged operations.
    let suspend_result = f.suspend_as(&f.new_admin, 2, &[&f.n1, &f.n3]);
    assert!(suspend_result.is_ok());
    assert_eq!(
        f.client.get_provider(&f.provider).unwrap().status,
        ActorStatus::Suspended
    );

    // The old admin's signers can no longer do anything, even with a full
    // 2-of-3 that used to be sufficient.
    let old_admin_result = f.suspend_as(&f.admin, 3, &[&f.s1, &f.s3]);
    assert!(old_admin_result.is_err());
}
