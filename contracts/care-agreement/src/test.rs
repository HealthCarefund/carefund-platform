#![cfg(test)]

use crate::{AgreementError, CareAgreementContract, CareAgreementContractClient};
use soroban_sdk::{testutils::Address as _, Address, Env};

fn create_client<'a>(env: &Env) -> (CareAgreementContractClient<'a>, Address) {
    let contract_id = env.register(CareAgreementContract, ());
    let client = CareAgreementContractClient::new(env, &contract_id);
    let admin = Address::generate(env);
    (client, admin)
}

#[test]
fn test_initialize_success() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    let provider_registry = Address::generate(&env);
    let settlement_asset = Address::generate(&env);

    let res = client.try_initialize(&admin, &provider_registry, &settlement_asset);
    assert!(res.is_ok());

    assert_eq!(client.get_admin(), admin);
    assert_eq!(client.get_provider_registry(), provider_registry);
    assert_eq!(client.get_settlement_asset(), settlement_asset);
}

#[test]
fn test_initialize_twice_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    let provider_registry = Address::generate(&env);
    let settlement_asset = Address::generate(&env);

    assert!(client
        .try_initialize(&admin, &provider_registry, &settlement_asset)
        .is_ok());

    let res = client.try_initialize(&admin, &provider_registry, &settlement_asset);
    assert_eq!(res, Err(Ok(AgreementError::AlreadyInitialized)));
}

#[test]
fn test_get_admin_uninitialized() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, _) = create_client(&env);

    let res = client.try_get_admin();
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_get_provider_registry_uninitialized() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, _) = create_client(&env);

    let res = client.try_get_provider_registry();
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_get_settlement_asset_uninitialized() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, _) = create_client(&env);

    let res = client.try_get_settlement_asset();
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_initialize_fails() {
    let env = Env::default();
    // Do not call env.mock_all_auths() -> require_auth will panic
    let (client, admin) = create_client(&env);
    let provider_registry = Address::generate(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &provider_registry, &settlement_asset);
}
