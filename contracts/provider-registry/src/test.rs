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

// ---------------------------------------------------------------------------
// Attester Lifecycle Tests
// ---------------------------------------------------------------------------

#[test]
fn test_register_attester() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);

    let res = client.try_register_attester(&attester, &provider, &credential_ref);
    assert!(res.is_ok());

    // Check authorization query
    assert!(client.check_attester(&attester, &provider));

    // Check get_attester
    let record = client.get_attester(&attester).unwrap();
    assert_eq!(record.provider, provider);
    assert_eq!(record.credential_ref, credential_ref);
}

#[test]
fn test_register_attester_inactive_provider_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);
    client.suspend_provider(&provider);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);

    let res = client.try_register_attester(&attester, &provider, &credential_ref);
    assert_eq!(res, Err(Ok(RegistryError::ProviderNotActive)));
}

#[test]
fn test_register_attester_nonexistent_provider_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let nonexistent_provider = Address::generate(&env);
    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);

    let res = client.try_register_attester(&attester, &nonexistent_provider, &credential_ref);
    assert_eq!(res, Err(Ok(RegistryError::ProviderNotFound)));
}

#[test]
fn test_register_attester_duplicate_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);

    client.register_attester(&attester, &provider, &credential_ref);

    let res = client.try_register_attester(&attester, &provider, &credential_ref);
    assert_eq!(res, Err(Ok(RegistryError::AttesterAlreadyExists)));
}

#[test]
fn test_suspend_reinstate_revoke_attester() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    client.register_attester(&attester, &provider, &credential_ref);

    assert!(client.check_attester(&attester, &provider));

    // Suspend attester
    let res = client.try_suspend_attester(&attester);
    assert!(res.is_ok());
    assert!(!client.check_attester(&attester, &provider));

    // Suspend again fails
    assert_eq!(
        client.try_suspend_attester(&attester),
        Err(Ok(RegistryError::AttesterAlreadySuspended))
    );

    // Reinstate attester
    let res = client.try_reinstate_attester(&attester);
    assert!(res.is_ok());
    assert!(client.check_attester(&attester, &provider));

    // Reinstate again fails
    assert_eq!(
        client.try_reinstate_attester(&attester),
        Err(Ok(RegistryError::AttesterAlreadyActive))
    );

    // Revoke attester
    let res = client.try_revoke_attester(&attester);
    assert!(res.is_ok());
    assert!(!client.check_attester(&attester, &provider));

    // Revoked attester cannot be reinstated
    assert_eq!(
        client.try_reinstate_attester(&attester),
        Err(Ok(RegistryError::AttesterAlreadyRevoked))
    );

    // Revoking again fails
    assert_eq!(
        client.try_revoke_attester(&attester),
        Err(Ok(RegistryError::AttesterAlreadyRevoked))
    );
}

#[test]
fn test_check_attester_wrong_provider() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider1 = Address::generate(&env);
    let provider2 = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider1, &provider_ref);
    client.register_provider(&provider2, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    client.register_attester(&attester, &provider1, &credential_ref);

    // Bound to provider1, should be false for provider2
    assert!(client.check_attester(&attester, &provider1));
    assert!(!client.check_attester(&attester, &provider2));
}

#[test]
fn test_check_attester_when_provider_suspended() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    client.register_attester(&attester, &provider, &credential_ref);

    assert!(client.check_attester(&attester, &provider));

    // Suspend provider -> check_attester should return false even though attester is Active
    client.suspend_provider(&provider);
    assert!(!client.check_attester(&attester, &provider));
}

// ---------------------------------------------------------------------------
// Authorization Boundary Tests (Commit 6)
// ---------------------------------------------------------------------------

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_initialize_fails() {
    let env = Env::default();
    // Do not call env.mock_all_auths() -> require_auth will panic
    let (client, admin) = create_client(&env);
    client.initialize(&admin);
}

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_register_provider_fails() {
    let env = Env::default();
    let (client, admin) = create_client(&env);
    env.mock_all_auths();
    client.initialize(&admin);

    // Create new client without mocked auth
    let unauth_env = Env::default();
    let contract_id = unauth_env.register(ProviderRegistryContract, ());
    let unauth_client = ProviderRegistryContractClient::new(&unauth_env, &contract_id);
    unauth_client.initialize(&admin);

    let provider = Address::generate(&unauth_env);
    let provider_ref = BytesN::from_array(&unauth_env, &[1u8; 32]);
    unauth_client.register_provider(&provider, &provider_ref);
}

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_register_attester_fails() {
    let env = Env::default();
    let (client, admin) = create_client(&env);
    env.mock_all_auths();
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let unauth_env = Env::default();
    let contract_id = unauth_env.register(ProviderRegistryContract, ());
    let unauth_client = ProviderRegistryContractClient::new(&unauth_env, &contract_id);
    unauth_client.initialize(&admin);

    let attester = Address::generate(&unauth_env);
    let credential_ref = BytesN::from_array(&unauth_env, &[2u8; 32]);
    unauth_client.register_attester(&attester, &provider, &credential_ref);
}

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_suspend_provider_fails() {
    let env = Env::default();
    let (client, admin) = create_client(&env);
    env.mock_all_auths();
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let unauth_env = Env::default();
    let contract_id = unauth_env.register(ProviderRegistryContract, ());
    let unauth_client = ProviderRegistryContractClient::new(&unauth_env, &contract_id);
    unauth_client.initialize(&admin);

    unauth_client.suspend_provider(&provider);
}

#[test]
fn test_revoked_provider_disables_all_attesters() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester1 = Address::generate(&env);
    let attester2 = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    client.register_attester(&attester1, &provider, &credential_ref);
    client.register_attester(&attester2, &provider, &credential_ref);

    assert!(client.check_attester(&attester1, &provider));
    assert!(client.check_attester(&attester2, &provider));

    // Revoke provider
    client.revoke_provider(&provider);

    // Both attesters are no longer valid for this provider
    assert!(!client.check_attester(&attester1, &provider));
    assert!(!client.check_attester(&attester2, &provider));

    // Cannot register new attester for revoked provider
    let attester3 = Address::generate(&env);
    let res = client.try_register_attester(&attester3, &provider, &credential_ref);
    assert_eq!(res, Err(Ok(RegistryError::ProviderNotActive)));
}

#[test]
fn test_revoked_attester_check_fails() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    client.initialize(&admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    client.register_attester(&attester, &provider, &credential_ref);

    assert!(client.check_attester(&attester, &provider));

    client.revoke_attester(&attester);
    assert!(!client.check_attester(&attester, &provider));
}
