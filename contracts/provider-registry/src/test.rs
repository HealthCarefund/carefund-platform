#![cfg(test)]

use crate::{ProviderRegistryContract, ProviderRegistryContractClient, RegistryError};
use soroban_sdk::{testutils::Address as _, Address, Env};

fn create_client<'a>(env: &Env) -> (ProviderRegistryContractClient<'a>, Address) {
    let contract_id = env.register(ProviderRegistryContract, ());
    let client = ProviderRegistryContractClient::new(env, &contract_id);
    let admin = Address::generate(env);
    (client, admin)
}

#[test]
fn test_initialize_success() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);

    let res = client.try_initialize(&admin);
    assert!(res.is_ok());

    assert_eq!(client.get_admin(), admin);
}

#[test]
fn test_initialize_twice_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    let second_admin = Address::generate(&env);

    assert!(client.try_initialize(&admin).is_ok());

    let res = client.try_initialize(&second_admin);
    assert_eq!(res, Err(Ok(RegistryError::AlreadyInitialized)));
}

#[test]
fn test_get_admin_uninitialized() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, _) = create_client(&env);

    let res = client.try_get_admin();
    assert_eq!(res, Err(Ok(RegistryError::Unauthorized)));
}

#[test]
fn test_reads_for_nonexistent_records() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let dummy_provider = Address::generate(&env);
    let dummy_attester = Address::generate(&env);

    assert_eq!(client.get_provider(&dummy_provider), None);
    assert_eq!(client.get_attester(&dummy_attester), None);
    assert!(!client.is_provider_active(&dummy_provider));
    assert!(!client.check_attester(&dummy_attester, &dummy_provider));
}
