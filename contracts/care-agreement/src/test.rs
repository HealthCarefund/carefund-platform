#![cfg(test)]

use crate::{AgreementError, AgreementState, CareAgreementContract, CareAgreementContractClient};
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient};
use soroban_sdk::{testutils::Address as _, Address, BytesN, Env};
use soroban_sdk::testutils::Ledger;

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

// ---------------------------------------------------------------------------
// Agreement Funding Tests (Commit 11)
// ---------------------------------------------------------------------------

#[test]
fn test_fund_agreement_success() {
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

    // Setup token contract
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin.clone());

    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor via cross-contract invocation
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    // Setup care-agreement
    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    assert_eq!(agreement_id, 1);
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Requested);

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Verify agreement is now in Funded state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Funded);
}

#[test]
fn test_fund_agreement_wrong_sponsor_rejected() {
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
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Try to fund with wrong sponsor
    let wrong_sponsor = Address::generate(&env);
    let res = client.try_fund(&agreement_id, &wrong_sponsor);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_fund_agreement_after_deadline_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 100; // Very short deadline
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Advance time past funding deadline
    env.ledger().with_mut(|l| {
        l.timestamp = funding_deadline + 1;
    });

    // Try to fund after deadline
    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::FundingDeadlineNotPassed)));
}

#[test]
fn test_fund_agreement_inactive_provider_rejected() {
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
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Suspend provider after agreement creation
    registry_client.suspend_provider(&provider);

    // Try to fund when provider is inactive
    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::ProviderNotActive)));
}

#[test]
fn test_fund_agreement_invalid_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin.clone());

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    // Mint tokens to sponsor via cross-contract invocation
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund once
    client.fund(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Funded);

    // Try to fund again (state is now Funded, not Requested)
    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_fund_agreement_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_sponsor = Address::generate(&unauth_env);

    // Call fund directly without sponsor auth
    unauth_care_client.fund(&1u64, &unauth_sponsor);
}

// ---------------------------------------------------------------------------
// Cancel Tests (Commit 12)
// ---------------------------------------------------------------------------

#[test]
fn test_cancel_agreement_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    assert_eq!(agreement_id, 1);
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Requested);

    // Cancel the agreement
    client.cancel(&agreement_id, &provider);

    // Verify agreement is now in Cancelled state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Cancelled);
}

#[test]
fn test_cancel_agreement_wrong_provider_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Try to cancel with wrong provider
    let wrong_provider = Address::generate(&env);
    let res = client.try_cancel(&agreement_id, &wrong_provider);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_cancel_agreement_funded_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Funded);

    // Try to cancel after funding (should fail - state is no longer Requested)
    let res = client.try_cancel(&agreement_id, &provider);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_cancel_agreement_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_provider = Address::generate(&unauth_env);

    // Call cancel directly without provider auth
    unauth_care_client.cancel(&1u64, &unauth_provider);
}

// ---------------------------------------------------------------------------
// Expire Tests (Commit 12)
// ---------------------------------------------------------------------------

#[test]
fn test_expire_agreement_from_funded_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Funded);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Expire the agreement
    client.expire(&agreement_id);

    // Verify agreement is now in Expired state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Expired);
}

#[test]
fn test_expire_agreement_before_deadline_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Try to expire before care deadline
    let res = client.try_expire(&agreement_id);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
fn test_expire_agreement_invalid_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement but don't fund it
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Try to expire agreement in Requested state (should fail)
    let res = client.try_expire(&agreement_id);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

// ---------------------------------------------------------------------------
// Care Attestation Tests (Commit 13)
// ---------------------------------------------------------------------------

#[test]
fn test_attest_care_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Funded);

    // Attest care
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    // Verify agreement is now in CareConfirmed state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::CareConfirmed);
    assert_eq!(agreement.attestation_commitment, Some(attestation_commitment));
    assert_eq!(agreement.attested_by, Some(attester.clone()));
    assert_eq!(agreement.attested_at, Some(current_time));
}

#[test]
fn test_attest_care_wrong_attester_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Try to attest with wrong attester
    let wrong_attester = Address::generate(&env);
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &wrong_attester, &attestation_commitment);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_attest_care_wrong_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement but don't fund it
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Try to attest care on Requested state (should fail)
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &attester, &attestation_commitment);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_attest_care_after_deadline_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 100; // Very short care window after funding deadline

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Try to attest after care deadline
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &attester, &attestation_commitment);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_attest_care_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_attester = Address::generate(&unauth_env);
    let attestation_commitment = BytesN::from_array(&unauth_env, &[5u8; 32]);

    // Call attest_care directly without attester auth
    unauth_care_client.attest_care(&1u64, &unauth_attester, &attestation_commitment);
}

// ---------------------------------------------------------------------------
// Dispute Opening Tests (Commit 14)
// ---------------------------------------------------------------------------

#[test]
fn test_open_dispute_from_funded_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Funded);

    // Advance time past care deadline but within dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Open dispute from Funded state
    client.open_dispute(&agreement_id, &sponsor);

    // Verify agreement is now in Disputed state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Disputed);
    assert_eq!(agreement.dispute_opened_by, Some(sponsor.clone()));
}

#[test]
fn test_open_dispute_from_care_confirmed_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Attest care (move to CareConfirmed)
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::CareConfirmed);

    // Advance time past care deadline but within dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Open dispute from CareConfirmed state
    client.open_dispute(&agreement_id, &provider);

    // Verify agreement is now in Disputed state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Disputed);
    assert_eq!(agreement.dispute_opened_by, Some(provider.clone()));
}

#[test]
fn test_open_dispute_wrong_opener_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Advance time past care deadline but within dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Try to open dispute with wrong opener (not sponsor or provider)
    let wrong_opener = Address::generate(&env);
    let res = client.try_open_dispute(&agreement_id, &wrong_opener);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_open_dispute_before_dispute_window_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Try to open dispute before care deadline (dispute window not active yet)
    let res = client.try_open_dispute(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::DisputeWindowActive)));
}

#[test]
fn test_open_dispute_after_dispute_window_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Advance time past dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + dispute_window_secs + 1;
    });

    // Try to open dispute after dispute window closes
    let res = client.try_open_dispute(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::DisputeWindowClosed)));
}

#[test]
fn test_open_dispute_invalid_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement but don't fund it (stays in Requested state)
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Advance time past dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Try to open dispute on Requested state (should fail)
    let res = client.try_open_dispute(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_open_dispute_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_opener = Address::generate(&unauth_env);

    // Call open_dispute directly without opener auth
    unauth_care_client.open_dispute(&1u64, &unauth_opener);
}

// ---------------------------------------------------------------------------
// Settlement Tests (Commit 15)
// ---------------------------------------------------------------------------

#[test]
fn test_settle_agreement_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Attest care
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::CareConfirmed);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Settle the agreement
    client.settle(&agreement_id, &sponsor);

    // Verify agreement is now in Settled state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Settled);
}

#[test]
fn test_settle_agreement_wrong_sponsor_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Attest care
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Try to settle with wrong sponsor
    let wrong_sponsor = Address::generate(&env);
    let res = client.try_settle(&agreement_id, &wrong_sponsor);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_settle_agreement_wrong_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement but don't attest
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement but don't attest (stays in Funded state)
    client.fund(&agreement_id, &sponsor);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Try to settle without attestation (should fail - state is Funded, not CareConfirmed)
    let res = client.try_settle(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_settle_agreement_before_deadline_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create and fund agreement
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Fund the agreement
    client.fund(&agreement_id, &sponsor);

    // Attest care
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    // Try to settle before care deadline (should fail)
    let res = client.try_settle(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_settle_agreement_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    let unauth_sponsor = Address::generate(&unauth_env);

    // Call settle directly without sponsor auth
    unauth_care_client.settle(&1u64, &unauth_sponsor);
}

// ---------------------------------------------------------------------------
// Dispute Resolution Tests (Commit 16)
// ---------------------------------------------------------------------------

#[test]
fn test_resolve_dispute_resume_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create, fund, attest, and open dispute
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&agreement_id, &sponsor);

    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    client.open_dispute(&agreement_id, &sponsor);
    assert_eq!(client.get_agreement(&agreement_id).state, AgreementState::Disputed);

    // Resolve dispute with Resume
    client.resolve_dispute(&agreement_id, &crate::DisputeResolution::Resume);

    // Verify agreement is back to CareConfirmed (origin state)
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::CareConfirmed);
}

#[test]
fn test_resolve_dispute_settle_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create, fund, attest, and open dispute
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&agreement_id, &sponsor);

    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    client.open_dispute(&agreement_id, &sponsor);

    // Resolve dispute with Settle
    client.resolve_dispute(&agreement_id, &crate::DisputeResolution::Settle);

    // Verify agreement is now Settled
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Settled);
}

#[test]
fn test_resolve_dispute_refund_success() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    // Mint tokens to sponsor
    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create, fund, and open dispute (no attestation)
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    let agreement_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&agreement_id, &sponsor);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    client.open_dispute(&agreement_id, &sponsor);

    // Resolve dispute with Refund
    client.resolve_dispute(&agreement_id, &crate::DisputeResolution::Refund);

    // Verify agreement is now Refunded
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::Refunded);
}

#[test]
fn test_resolve_dispute_invalid_state_rejected() {
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

    // Setup token
    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    // Create agreement but leave in Requested state
    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
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

    // Try to resolve dispute on non-disputed agreement (should fail)
    let res = client.try_resolve_dispute(&agreement_id, &crate::DisputeResolution::Settle);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
#[should_panic(expected = "HostError")]
fn test_resolve_dispute_unauthorized_fails() {
    let unauth_env = Env::default();
    let unauth_contract_id = unauth_env.register(CareAgreementContract, ());
    let unauth_care_client = CareAgreementContractClient::new(&unauth_env, &unauth_contract_id);

    // Call resolve_dispute directly without admin auth
    unauth_care_client.resolve_dispute(&1u64, &crate::DisputeResolution::Settle);
}

