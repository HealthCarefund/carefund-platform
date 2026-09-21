#![cfg(test)]

use crate::{ProviderRegistryContract, ProviderRegistryContractClient, RegistryError};
use soroban_sdk::{testutils::Address as _, Address, BytesN, Env};

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

// ---------------------------------------------------------------------------
// Provider Lifecycle Tests
// ---------------------------------------------------------------------------

#[test]
fn test_register_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    let res = client.try_register_provider(&provider, &provider_ref);
    assert!(res.is_ok());

    // Verify provider is active
    assert!(client.is_provider_active(&provider));

    // Verify record
    let record = client.get_provider(&provider).unwrap();
    assert_eq!(record.provider_ref, provider_ref);
}

#[test]
fn test_register_provider_duplicate_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);

    // Try to register again
    let res = client.try_register_provider(&provider, &provider_ref);
    assert_eq!(res, Err(Ok(RegistryError::ProviderAlreadyExists)));
}

#[test]
fn test_suspend_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    assert!(client.is_provider_active(&provider));

    // Suspend
    let res = client.try_suspend_provider(&provider);
    assert!(res.is_ok());

    // Provider is no longer active
    assert!(!client.is_provider_active(&provider));
}

#[test]
fn test_suspend_provider_twice_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    client.suspend_provider(&provider);

    // Try to suspend again
    let res = client.try_suspend_provider(&provider);
    assert_eq!(res, Err(Ok(RegistryError::ProviderAlreadySuspended)));
}

#[test]
fn test_reinstate_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    client.suspend_provider(&provider);
    assert!(!client.is_provider_active(&provider));

    // Reinstate
    let res = client.try_reinstate_provider(&provider);
    assert!(res.is_ok());

    // Provider is active again
    assert!(client.is_provider_active(&provider));
}

#[test]
fn test_reinstate_active_provider_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);

    // Try to reinstate an already active provider
    let res = client.try_reinstate_provider(&provider);
    assert_eq!(res, Err(Ok(RegistryError::ProviderAlreadyActive)));
}

#[test]
fn test_revoke_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    assert!(client.is_provider_active(&provider));

    // Revoke
    let res = client.try_revoke_provider(&provider);
    assert!(res.is_ok());

    // Provider is no longer active
    assert!(!client.is_provider_active(&provider));
}

#[test]
fn test_revoked_provider_cannot_be_reinstated() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    client.revoke_provider(&provider);

    // Try to reinstate revoked provider
    let res = client.try_reinstate_provider(&provider);
    assert_eq!(res, Err(Ok(RegistryError::ProviderAlreadyRevoked)));
}

#[test]
fn test_revoke_provider_twice_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    client.revoke_provider(&provider);

    // Try to revoke again
    let res = client.try_revoke_provider(&provider);
    assert_eq!(res, Err(Ok(RegistryError::ProviderAlreadyRevoked)));
}

#[test]
fn test_revoke_suspended_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);

    client.register_provider(&provider, &provider_ref);
    client.suspend_provider(&provider);

    // Revoke suspended provider (should work)
    let res = client.try_revoke_provider(&provider);
    assert!(res.is_ok());

    assert!(!client.is_provider_active(&provider));
}
