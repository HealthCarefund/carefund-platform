#![cfg(test)]

use crate::{AgreementError, AgreementState, CareAgreementContract, CareAgreementContractClient};
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient};
use soroban_sdk::{testutils::Address as _, Address, BytesN, Env};

// Bring provider-registry as external crate for testing
extern crate provider_registry;

fn create_client<'a>(env: &Env) -> (CareAgreementContractClient<'a>, Address) {
    let contract_id = env.register(CareAgreementContract, ());
    let client = CareAgreementContractClient::new(env, &contract_id);
    let admin = Address::generate(env);
    (client, admin)
}

fn create_registry_client<'a>(env: &Env) -> (ProviderRegistryContractClient<'a>, Address) {
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

// ---------------------------------------------------------------------------
// Agreement Creation Tests (Commit 9)
// ---------------------------------------------------------------------------

#[test]
fn test_create_agreement_success() {
    let env = Env::default();
    env.mock_all_auths();

    // Setup registry
    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    // Setup care-agreement
    let (client, admin) = create_client(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &registry_client.address, &settlement_asset);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400; // +1 day
    let care_deadline = funding_deadline + 86400; // +2 days

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128, // 1000 tokens (7 decimals)
        &900_0000000i128,  // 900 tokens settlement
        &funding_deadline,
        &care_deadline,
        &3600u64, // 1 hour dispute window
    );

    assert_eq!(agreement_id, 1);

    // Verify agreement
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.sponsor, sponsor);
    assert_eq!(agreement.provider, provider);
    assert_eq!(agreement.attester, attester);
    assert_eq!(agreement.patient_ref_commitment, patient_ref);
    assert_eq!(agreement.service_commitment, service_commitment);
    assert_eq!(agreement.funding_amount, 1000_0000000i128);
    assert_eq!(agreement.settlement_amount, 900_0000000i128);
    assert_eq!(agreement.state, AgreementState::Requested);
}

#[test]
fn test_create_agreement_invalid_amounts() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    let (client, admin) = create_client(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &registry_client.address, &settlement_asset);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Zero funding amount
    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &0i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::InvalidAmount)));

    // Settlement > funding
    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &1100_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::InvalidAmount)));
}

#[test]
fn test_create_agreement_invalid_deadlines() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    let (client, admin) = create_client(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &registry_client.address, &settlement_asset);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();

    // Funding deadline in past/at current time
    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &current_time,
        &(current_time + 86400),
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::InvalidDeadline)));

    // Care deadline <= funding deadline
    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &(current_time + 86400),
        &(current_time + 86400),
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::InvalidDeadline)));
}

#[test]
fn test_create_agreement_inactive_provider_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);
    registry_client.suspend_provider(&provider);

    let attester = Address::generate(&env);

    let (client, admin) = create_client(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &registry_client.address, &settlement_asset);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::ProviderNotActive)));
}

#[test]
fn test_create_agreement_unauthorized_attester_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    // Do NOT register attester

    let (client, admin) = create_client(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &registry_client.address, &settlement_asset);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res, Err(Ok(AgreementError::AttesterNotAuthorized)));
}

#[test]
fn test_get_agreement_not_found() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    let provider_registry = Address::generate(&env);
    let settlement_asset = Address::generate(&env);
    client.initialize(&admin, &provider_registry, &settlement_asset);

    let res = client.try_get_agreement(&999u64);
    assert_eq!(res, Err(Ok(AgreementError::AgreementNotFound)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_unauthorized_create_agreement_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_provider = Address::generate(&unauth_env);
    let unauth_sponsor = Address::generate(&unauth_env);
    let unauth_attester = Address::generate(&unauth_env);
    let unauth_patient_ref = BytesN::from_array(&unauth_env, &[3u8; 32]);
    let unauth_service = BytesN::from_array(&unauth_env, &[4u8; 32]);

    // Call create_agreement directly on unauth env without provider auth
    unauth_care_client.create_agreement(
        &unauth_provider,
        &unauth_sponsor,
        &unauth_attester,
        &unauth_patient_ref,
        &unauth_service,
        &1000_0000000i128,
        &900_0000000i128,
        &(unauth_env.ledger().timestamp() + 86400),
        &(unauth_env.ledger().timestamp() + 172800),
        &3600u64,
    );
}
