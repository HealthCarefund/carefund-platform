#![cfg(test)]

use crate::{AgreementError, AgreementState, CareAgreementContract, CareAgreementContractClient};
use provider_registry::{ProviderRegistryContract, ProviderRegistryContractClient};
use soroban_sdk::testutils::Ledger;
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
fn test_create_agreement_dispute_window_overflow_rejected() {
    // care_deadline + dispute_window_secs is computed as a plain u64
    // addition later, in open_dispute. A dispute_window_secs near
    // u64::MAX must be rejected here at creation time, not left to trap
    // the first time anyone calls open_dispute on this agreement.
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

    let res = client.try_create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &(current_time + 3600),
        &(current_time + 7200),
        &u64::MAX,
    );
    assert_eq!(res, Err(Ok(AgreementError::ArithmeticOverflow)));
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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

    // Attest care
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    // Verify agreement is now in CareConfirmed state
    let agreement = client.get_agreement(&agreement_id);
    assert_eq!(agreement.state, AgreementState::CareConfirmed);
    assert_eq!(
        agreement.attestation_commitment,
        Some(attestation_commitment)
    );
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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::CareConfirmed
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::CareConfirmed
    );

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
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Disputed
    );

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

// ---------------------------------------------------------------------------
// Full State-Machine Tests (Commit 17)
// ---------------------------------------------------------------------------

#[test]
fn test_complete_lifecycle_requested_to_settled() {
    let env = Env::default();
    env.mock_all_auths();

    // Setup registry and token
    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (2000_0000000i128).into_val(&env),
            ],
        ),
    );

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

    // Verify Requested state
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Requested
    );

    // Fund agreement -> Funded
    client.fund(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

    // Attest care -> CareConfirmed
    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::CareConfirmed
    );

    // Advance past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Settle -> Settled
    client.settle(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Settled
    );
}

#[test]
fn test_complete_lifecycle_with_dispute_and_resolution() {
    let env = Env::default();
    env.mock_all_auths();

    // Setup registry and token
    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (2000_0000000i128).into_val(&env),
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

    client.fund(&agreement_id, &sponsor);

    let attestation_commitment = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_commitment);

    // Advance past care deadline but within dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Open dispute -> Disputed
    client.open_dispute(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Disputed
    );

    // Resolve dispute with Settle -> Settled
    client.resolve_dispute(&agreement_id, &crate::DisputeResolution::Settle);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Settled
    );
}

#[test]
fn test_cancellation_from_requested_state() {
    let env = Env::default();
    env.mock_all_auths();

    // Setup registry and token
    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

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

    // Cancel from Requested -> Cancelled
    client.cancel(&agreement_id, &provider);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Cancelled
    );
}

#[test]
fn test_expiry_transitions() {
    let env = Env::default();
    env.mock_all_auths();

    // Setup registry and token
    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let provider = Address::generate(&env);
    let provider_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider, &provider_ref);

    let attester = Address::generate(&env);
    let credential_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester, &provider, &credential_ref);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    // Advance past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Expire from Funded -> Expired
    client.expire(&agreement_id);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Expired
    );
}

// ---------------------------------------------------------------------------
// Cross-Contract Integration Tests (Commit 18)
// ---------------------------------------------------------------------------

#[test]
fn test_cross_contract_provider_suspension_blocks_agreement_creation() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Suspend provider in registry
    registry_client.suspend_provider(&provider);

    // Agreement creation must fail because provider is suspended
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

    // Reinstate provider in registry
    registry_client.reinstate_provider(&provider);

    // Agreement creation succeeds after reinstatement
    let res_ok = client.try_create_agreement(
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
    assert!(res_ok.is_ok());
    assert_eq!(res_ok.unwrap().unwrap(), 1u64);
}

#[test]
fn test_cross_contract_provider_revocation_blocks_agreement_creation() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Revoke provider in registry
    registry_client.revoke_provider(&provider);

    // Agreement creation must fail because provider is revoked
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
fn test_cross_contract_provider_suspension_and_reinstatement_for_funding() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Create agreement while provider is active
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

    // Suspend provider before funding
    registry_client.suspend_provider(&provider);

    // Funding attempt should fail due to inactive provider
    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::ProviderNotActive)));

    // Reinstate provider
    registry_client.reinstate_provider(&provider);

    // Funding succeeds after reinstatement
    let res_ok = client.try_fund(&agreement_id, &sponsor);
    assert!(res_ok.is_ok());
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );
}

#[test]
fn test_cross_contract_attester_suspension_and_reinstatement() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Suspend attester in registry
    registry_client.suspend_attester(&attester);

    // Agreement creation fails because attester is suspended
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

    // Reinstate attester in registry
    registry_client.reinstate_attester(&attester);

    // Agreement creation succeeds after reinstatement
    let res_ok = client.try_create_agreement(
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
    assert!(res_ok.is_ok());
}

#[test]
fn test_cross_contract_attester_revocation_blocks_agreement_creation() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Revoke attester in registry
    registry_client.revoke_attester(&attester);

    // Agreement creation fails because attester is revoked
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
fn test_cross_contract_mismatched_provider_attester_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    // Register Provider A and Attester A
    let provider_a = Address::generate(&env);
    let provider_a_ref = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider_a, &provider_a_ref);

    let attester_a = Address::generate(&env);
    let credential_a_ref = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester_a, &provider_a, &credential_a_ref);

    // Register Provider B and Attester B
    let provider_b = Address::generate(&env);
    let provider_b_ref = BytesN::from_array(&env, &[3u8; 32]);
    registry_client.register_provider(&provider_b, &provider_b_ref);

    let attester_b = Address::generate(&env);
    let credential_b_ref = BytesN::from_array(&env, &[4u8; 32]);
    registry_client.register_attester(&attester_b, &provider_b, &credential_b_ref);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let sponsor = Address::generate(&env);
    let patient_ref = BytesN::from_array(&env, &[5u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[6u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Cross-match: Provider A attempts to create agreement with Attester B (registered for Provider B)
    let res_a_b = client.try_create_agreement(
        &provider_a,
        &sponsor,
        &attester_b,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res_a_b, Err(Ok(AgreementError::AttesterNotAuthorized)));

    // Cross-match: Provider B attempts to create agreement with Attester A (registered for Provider A)
    let res_b_a = client.try_create_agreement(
        &provider_b,
        &sponsor,
        &attester_a,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert_eq!(res_b_a, Err(Ok(AgreementError::AttesterNotAuthorized)));

    // Correct pairs succeed
    let res_a_a = client.try_create_agreement(
        &provider_a,
        &sponsor,
        &attester_a,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert!(res_a_a.is_ok());

    let res_b_b = client.try_create_agreement(
        &provider_b,
        &sponsor,
        &attester_b,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );
    assert!(res_b_b.is_ok());
}

#[test]
fn test_cross_contract_multi_agreement_token_balance_lifecycle() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let sponsor_1 = Address::generate(&env);
    let sponsor_2 = Address::generate(&env);

    use soroban_sdk::IntoVal;
    // Mint 5000 to Sponsor 1 and 5000 to Sponsor 2
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor_1.clone().into_val(&env),
                (5000_0000000i128).into_val(&env),
            ],
        ),
    );
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor_2.clone().into_val(&env),
                (5000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let get_balance = |addr: &Address| -> i128 {
        env.invoke_contract(
            &token_contract_id,
            &soroban_sdk::Symbol::new(&env, "balance"),
            soroban_sdk::Vec::from_array(&env, [addr.clone().into_val(&env)]),
        )
    };

    assert_eq!(get_balance(&sponsor_1), 5000_0000000i128);
    assert_eq!(get_balance(&sponsor_2), 5000_0000000i128);
    assert_eq!(get_balance(&client.address), 0i128);
    assert_eq!(get_balance(&provider), 0i128);

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Create Agreement 1 (2000 funding, 1800 settlement)
    let ag1 = client.create_agreement(
        &provider,
        &sponsor_1,
        &attester,
        &patient_ref,
        &service_commitment,
        &2000_0000000i128,
        &1800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );

    // Create Agreement 2 (3000 funding, 2700 settlement)
    let ag2 = client.create_agreement(
        &provider,
        &sponsor_2,
        &attester,
        &patient_ref,
        &service_commitment,
        &3000_0000000i128,
        &2700_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );

    // Fund Ag1
    client.fund(&ag1, &sponsor_1);
    assert_eq!(get_balance(&sponsor_1), 3000_0000000i128);
    assert_eq!(get_balance(&client.address), 2000_0000000i128);

    // Fund Ag2
    client.fund(&ag2, &sponsor_2);
    assert_eq!(get_balance(&sponsor_2), 2000_0000000i128);
    assert_eq!(get_balance(&client.address), 5000_0000000i128);

    // Attest care for Ag1
    let attestation_ref = BytesN::from_array(&env, &[7u8; 32]);
    client.attest_care(&ag1, &attester, &attestation_ref);

    // Advance time past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Settle Ag1 -> provider receives 1800
    client.settle(&ag1, &sponsor_1);
    assert_eq!(get_balance(&provider), 1800_0000000i128);
    assert_eq!(get_balance(&client.address), 3200_0000000i128);

    // Ag2 disputed from Funded state and settled via dispute resolution
    client.open_dispute(&ag2, &sponsor_2);
    assert_eq!(client.get_agreement(&ag2).state, AgreementState::Disputed);

    // Resolve Ag2 dispute with Settle -> provider receives another 2700
    client.resolve_dispute(&ag2, &crate::DisputeResolution::Settle);
    assert_eq!(get_balance(&provider), 4500_0000000i128);
    assert_eq!(get_balance(&client.address), 500_0000000i128);
}

#[test]
fn test_cross_contract_dispute_refund_token_balance_lifecycle() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (2000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let get_balance = |addr: &Address| -> i128 {
        env.invoke_contract(
            &token_contract_id,
            &soroban_sdk::Symbol::new(&env, "balance"),
            soroban_sdk::Vec::from_array(&env, [addr.clone().into_val(&env)]),
        )
    };

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
        &2000_0000000i128,
        &1800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    // Fund agreement
    client.fund(&agreement_id, &sponsor);
    assert_eq!(get_balance(&sponsor), 0i128);
    assert_eq!(get_balance(&client.address), 2000_0000000i128);

    // Advance time past care deadline but within dispute window
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    // Open dispute from Funded state
    client.open_dispute(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Disputed
    );

    // Resolve dispute with Refund -> sponsor gets full 2000 back
    client.resolve_dispute(&agreement_id, &crate::DisputeResolution::Refund);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Refunded
    );

    assert_eq!(get_balance(&sponsor), 2000_0000000i128);
    assert_eq!(get_balance(&client.address), 0i128);
    assert_eq!(get_balance(&provider), 0i128);
}

#[test]
fn test_cross_contract_multiple_providers_and_attesters_isolation() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    // Provider 1 + Attester 1
    let provider_1 = Address::generate(&env);
    let provider_ref_1 = BytesN::from_array(&env, &[1u8; 32]);
    registry_client.register_provider(&provider_1, &provider_ref_1);

    let attester_1 = Address::generate(&env);
    let credential_ref_1 = BytesN::from_array(&env, &[2u8; 32]);
    registry_client.register_attester(&attester_1, &provider_1, &credential_ref_1);

    // Provider 2 + Attester 2
    let provider_2 = Address::generate(&env);
    let provider_ref_2 = BytesN::from_array(&env, &[3u8; 32]);
    registry_client.register_provider(&provider_2, &provider_ref_2);

    let attester_2 = Address::generate(&env);
    let credential_ref_2 = BytesN::from_array(&env, &[4u8; 32]);
    registry_client.register_attester(&attester_2, &provider_2, &credential_ref_2);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor_1 = Address::generate(&env);
    let sponsor_2 = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor_1.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor_2.clone().into_val(&env),
                (1000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let patient_ref = BytesN::from_array(&env, &[5u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[6u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Create agreements for both
    let ag1 = client.create_agreement(
        &provider_1,
        &sponsor_1,
        &attester_1,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );

    let ag2 = client.create_agreement(
        &provider_2,
        &sponsor_2,
        &attester_2,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );

    // Fund both
    client.fund(&ag1, &sponsor_1);
    client.fund(&ag2, &sponsor_2);

    // Suspend Provider 1
    registry_client.suspend_provider(&provider_1);

    // Provider 2 lifecycle proceeds completely unaffected
    let attestation_ref = BytesN::from_array(&env, &[7u8; 32]);
    client.attest_care(&ag2, &attester_2, &attestation_ref);
    assert_eq!(
        client.get_agreement(&ag2).state,
        AgreementState::CareConfirmed
    );

    // Advance past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Settle ag2 to Provider 2
    client.settle(&ag2, &sponsor_2);
    assert_eq!(client.get_agreement(&ag2).state, AgreementState::Settled);

    let get_balance = |addr: &Address| -> i128 {
        env.invoke_contract(
            &token_contract_id,
            &soroban_sdk::Symbol::new(&env, "balance"),
            soroban_sdk::Vec::from_array(&env, [addr.clone().into_val(&env)]),
        )
    };

    assert_eq!(get_balance(&provider_2), 900_0000000i128);
    assert_eq!(get_balance(&provider_1), 0i128);
}

// ---------------------------------------------------------------------------
// Commit 19: Security and Authorization Boundary Tests
// ---------------------------------------------------------------------------

#[test]
fn test_security_double_initialization_blocked() {
    let env = Env::default();
    env.mock_all_auths();

    let (client, admin) = create_client(&env);
    let provider_registry = Address::generate(&env);
    let settlement_asset = Address::generate(&env);

    assert!(client
        .try_initialize(&admin, &provider_registry, &settlement_asset)
        .is_ok());

    let attacker = Address::generate(&env);
    let bogus_registry = Address::generate(&env);
    let bogus_asset = Address::generate(&env);

    let res = client.try_initialize(&attacker, &bogus_registry, &bogus_asset);
    assert_eq!(res, Err(Ok(AgreementError::AlreadyInitialized)));
    assert_eq!(client.get_admin(), admin);
}

#[test]
fn test_security_double_funding_attack_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (5000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    // First funding succeeds
    assert!(client.try_fund(&agreement_id, &sponsor).is_ok());
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

    // Second funding attempt rejected with InvalidState
    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_unauthorized_funder_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let legitimate_sponsor = Address::generate(&env);
    let attacker = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    let agreement_id = client.create_agreement(
        &provider,
        &legitimate_sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &900_0000000i128,
        &funding_deadline,
        &care_deadline,
        &3600u64,
    );

    let res = client.try_fund(&agreement_id, &attacker);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_security_funding_after_deadline_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    // Fast-forward past funding deadline
    env.ledger().with_mut(|l| {
        l.timestamp = funding_deadline + 1;
    });

    let res = client.try_fund(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::FundingDeadlineNotPassed)));
}

#[test]
fn test_security_double_settlement_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (2000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    client.fund(&agreement_id, &sponsor);

    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_ref);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // First settle succeeds
    assert!(client.try_settle(&agreement_id, &sponsor).is_ok());
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Settled
    );

    // Second settle attempt rejected with InvalidState
    let res = client.try_settle(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_unauthorized_settler_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);
    let attacker = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_ref);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    let res = client.try_settle(&agreement_id, &attacker);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_security_settlement_before_care_deadline_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    client.attest_care(&agreement_id, &attester, &attestation_ref);

    // Timestamp is exactly at care deadline (not strictly past)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline;
    });

    let res = client.try_settle(&agreement_id, &sponsor);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
fn test_security_unauthorized_attester_rejected() {
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

    let unauthorized_attester = Address::generate(&env);
    let credential_ref_2 = BytesN::from_array(&env, &[9u8; 32]);
    registry_client.register_attester(&unauthorized_attester, &provider, &credential_ref_2);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &unauthorized_attester, &attestation_ref);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_security_attestation_after_care_deadline_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    // Fast-forward past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &attester, &attestation_ref);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
fn test_security_attestation_wrong_state_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    // Agreement is in Requested state, not Funded
    let attestation_ref = BytesN::from_array(&env, &[5u8; 32]);
    let res = client.try_attest_care(&agreement_id, &attester, &attestation_ref);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_unauthorized_dispute_opener_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);
    let attacker = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    let res = client.try_open_dispute(&agreement_id, &attacker);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_security_dispute_timing_boundaries() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    use soroban_sdk::IntoVal;
    env.invoke_contract::<()>(
        &token_contract_id,
        &soroban_sdk::symbol_short!("mint"),
        soroban_sdk::Vec::from_array(
            &env,
            [
                sponsor.clone().into_val(&env),
                (4000_0000000i128).into_val(&env),
            ],
        ),
    );

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;
    let dispute_window_secs = 3600u64;

    // Create and fund all agreements upfront before advancing time
    let ag1 = client.create_agreement(
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
    client.fund(&ag1, &sponsor);

    let ag2 = client.create_agreement(
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
    client.fund(&ag2, &sponsor);

    let ag3 = client.create_agreement(
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
    client.fund(&ag3, &sponsor);

    let ag4 = client.create_agreement(
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
    client.fund(&ag4, &sponsor);

    // Test 1: Attempt at exact care deadline (fails: DisputeWindowActive)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline;
    });
    assert_eq!(
        client.try_open_dispute(&ag1, &sponsor),
        Err(Ok(AgreementError::DisputeWindowActive))
    );

    // Test 2: Attempt at care_deadline + 1 (succeeds)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });
    assert!(client.try_open_dispute(&ag2, &sponsor).is_ok());

    // Test 3: Attempt at exact end of dispute window (care_deadline + dispute_window_secs) (succeeds)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + dispute_window_secs;
    });
    assert!(client.try_open_dispute(&ag3, &sponsor).is_ok());

    // Test 4: Attempt after dispute window closed (care_deadline + dispute_window_secs + 1) (fails: DisputeWindowClosed)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + dispute_window_secs + 1;
    });
    assert_eq!(
        client.try_open_dispute(&ag4, &sponsor),
        Err(Ok(AgreementError::DisputeWindowClosed))
    );
}

#[test]
fn test_security_double_dispute_resolution_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 100;
    });

    client.open_dispute(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Disputed
    );

    // Resolve dispute with Refund
    assert!(client
        .try_resolve_dispute(&agreement_id, &crate::DisputeResolution::Refund)
        .is_ok());
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Refunded
    );

    // Second resolution attempt rejected with InvalidState
    let res = client.try_resolve_dispute(&agreement_id, &crate::DisputeResolution::Settle);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_unauthorized_cancellation_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);
    let attacker = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    let res = client.try_cancel(&agreement_id, &attacker);
    assert_eq!(res, Err(Ok(AgreementError::Unauthorized)));
}

#[test]
fn test_security_cancellation_wrong_state_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);
    assert_eq!(
        client.get_agreement(&agreement_id).state,
        AgreementState::Funded
    );

    // Cancel on Funded agreement rejected with InvalidState
    let res = client.try_cancel(&agreement_id, &provider);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_expiry_before_care_deadline_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    client.fund(&agreement_id, &sponsor);

    // Current time is before care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline;
    });

    let res = client.try_expire(&agreement_id);
    assert_eq!(res, Err(Ok(AgreementError::CareDeadlineNotPassed)));
}

#[test]
fn test_security_expiry_wrong_state_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

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

    // Advance past care deadline on unfunded Requested agreement
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    let res = client.try_expire(&agreement_id);
    assert_eq!(res, Err(Ok(AgreementError::InvalidState)));
}

#[test]
fn test_security_invalid_amount_parameters_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 86400;
    let care_deadline = funding_deadline + 86400;

    // Zero funding amount
    assert_eq!(
        client.try_create_agreement(
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
        ),
        Err(Ok(AgreementError::InvalidAmount))
    );

    // Negative funding amount
    assert_eq!(
        client.try_create_agreement(
            &provider,
            &sponsor,
            &attester,
            &patient_ref,
            &service_commitment,
            &-100i128,
            &900_0000000i128,
            &funding_deadline,
            &care_deadline,
            &3600u64,
        ),
        Err(Ok(AgreementError::InvalidAmount))
    );

    // Zero settlement amount
    assert_eq!(
        client.try_create_agreement(
            &provider,
            &sponsor,
            &attester,
            &patient_ref,
            &service_commitment,
            &1000_0000000i128,
            &0i128,
            &funding_deadline,
            &care_deadline,
            &3600u64,
        ),
        Err(Ok(AgreementError::InvalidAmount))
    );

    // Negative settlement amount
    assert_eq!(
        client.try_create_agreement(
            &provider,
            &sponsor,
            &attester,
            &patient_ref,
            &service_commitment,
            &1000_0000000i128,
            &-50i128,
            &funding_deadline,
            &care_deadline,
            &3600u64,
        ),
        Err(Ok(AgreementError::InvalidAmount))
    );

    // Settlement amount > funding amount
    assert_eq!(
        client.try_create_agreement(
            &provider,
            &sponsor,
            &attester,
            &patient_ref,
            &service_commitment,
            &1000_0000000i128,
            &1500_0000000i128,
            &funding_deadline,
            &care_deadline,
            &3600u64,
        ),
        Err(Ok(AgreementError::InvalidAmount))
    );
}

#[test]
fn test_security_invalid_deadline_parameters_rejected() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();

    // funding_deadline <= current_time
    assert_eq!(
        client.try_create_agreement(
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
        ),
        Err(Ok(AgreementError::InvalidDeadline))
    );

    // care_deadline <= funding_deadline
    let funding_deadline = current_time + 86400;
    assert_eq!(
        client.try_create_agreement(
            &provider,
            &sponsor,
            &attester,
            &patient_ref,
            &service_commitment,
            &1000_0000000i128,
            &900_0000000i128,
            &funding_deadline,
            &funding_deadline,
            &3600u64,
        ),
        Err(Ok(AgreementError::InvalidDeadline))
    );
}

#[test]
fn test_security_nonexistent_agreement_operations_rejected() {
    let env = Env::default();
    env.mock_all_auths();

    let (registry_client, registry_admin) = create_registry_client(&env);
    registry_client.initialize(&registry_admin);

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let dummy_addr = Address::generate(&env);
    let dummy_bytes = BytesN::from_array(&env, &[0u8; 32]);
    let nonexistent_id = 99999u64;

    assert_eq!(
        client.try_get_agreement(&nonexistent_id),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_fund(&nonexistent_id, &dummy_addr),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_cancel(&nonexistent_id, &dummy_addr),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_expire(&nonexistent_id),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_attest_care(&nonexistent_id, &dummy_addr, &dummy_bytes),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_open_dispute(&nonexistent_id, &dummy_addr),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_settle(&nonexistent_id, &dummy_addr),
        Err(Ok(AgreementError::AgreementNotFound))
    );
    assert_eq!(
        client.try_resolve_dispute(&nonexistent_id, &crate::DisputeResolution::Settle),
        Err(Ok(AgreementError::AgreementNotFound))
    );
}

// ---------------------------------------------------------------------------
// Block 3A Red Regression Tests (Reproducing Financial Defects B1-B7)
// ---------------------------------------------------------------------------

#[test]
fn test_red_b1_expire_locks_deposited_escrow() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);

    let sponsor = Address::generate(&env);
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

    let get_balance = |addr: &Address| -> i128 {
        env.invoke_contract(
            &token_contract_id,
            &soroban_sdk::Symbol::new(&env, "balance"),
            soroban_sdk::Vec::from_array(&env, [addr.clone().into_val(&env)]),
        )
    };

    let patient_ref = BytesN::from_array(&env, &[3u8; 32]);
    let service_commitment = BytesN::from_array(&env, &[4u8; 32]);
    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 1000;
    let care_deadline = funding_deadline + 1000;
    let dispute_window_secs = 500u64;

    let ag_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &patient_ref,
        &service_commitment,
        &1000_0000000i128,
        &800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&ag_id, &sponsor);
    assert_eq!(get_balance(&client.address), 1000_0000000i128);
    assert_eq!(get_balance(&sponsor), 0i128);

    // Advance past care deadline and dispute window without attestation
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + dispute_window_secs + 1;
    });

    client.expire(&ag_id);
    assert_eq!(client.get_agreement(&ag_id).state, AgreementState::Expired);

    // Defect B1: Under old code, expire does NOT refund sponsor. Funds remain trapped in contract!
    // This assertion fails under old code:
    assert_eq!(get_balance(&sponsor), 1000_0000000i128);
    assert_eq!(get_balance(&client.address), 0i128);
}

#[test]
fn test_red_b2_unauthorized_party_preempts_attested_settlement() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 1000;
    let care_deadline = funding_deadline + 1000;
    let dispute_window_secs = 500u64;

    let ag_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &1000_0000000i128,
        &800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&ag_id, &sponsor);
    client.attest_care(&ag_id, &attester, &BytesN::from_array(&env, &[5u8; 32]));
    assert_eq!(client.get_agreement(&ag_id).state, AgreementState::CareConfirmed);

    // Advance past care deadline
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Defect B2: Under old code, expire accepts CareConfirmed and transitions to Expired,
    // permanently destroying provider's ability to settle!
    // Correct behavior: expire on CareConfirmed must be rejected (InvalidState).
    // This assertion fails under old code (it returns Ok(Ok(()))):
    assert_eq!(
        client.try_expire(&ag_id),
        Err(Ok(AgreementError::InvalidState))
    );
}

#[test]
fn test_red_b3_surplus_retained_in_contract_after_settlement() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    let get_balance = |addr: &Address| -> i128 {
        env.invoke_contract(
            &token_contract_id,
            &soroban_sdk::Symbol::new(&env, "balance"),
            soroban_sdk::Vec::from_array(&env, [addr.clone().into_val(&env)]),
        )
    };

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 1000;
    let care_deadline = funding_deadline + 1000;
    let dispute_window_secs = 500u64;

    let ag_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &1000_0000000i128, // Funding: 1000
        &800_0000000i128,  // Settlement: 800 (Surplus = 200)
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&ag_id, &sponsor);
    client.attest_care(&ag_id, &attester, &BytesN::from_array(&env, &[5u8; 32]));

    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + dispute_window_secs + 1;
    });

    client.settle(&ag_id, &sponsor);
    assert_eq!(get_balance(&provider), 800_0000000i128);

    // Defect B3: Under old code, surplus (200) is stranded in contract. Sponsor gets 0.
    // Correct behavior: Sponsor receives surplus refund of 200, contract balance = 0.
    // This assertion fails under old code:
    assert_eq!(get_balance(&sponsor), 200_0000000i128);
    assert_eq!(get_balance(&client.address), 0i128);
}

#[test]
fn test_red_b4_settlement_races_and_abbreviates_dispute_window() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

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

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 1000;
    let care_deadline = funding_deadline + 1000;
    let dispute_window_secs = 500u64;

    let ag_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &1000_0000000i128,
        &800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &dispute_window_secs,
    );

    client.fund(&ag_id, &sponsor);
    client.attest_care(&ag_id, &attester, &BytesN::from_array(&env, &[5u8; 32]));

    // Advance to 1 second after care deadline (dispute window is active until care_deadline + 500)
    env.ledger().with_mut(|l| {
        l.timestamp = care_deadline + 1;
    });

    // Defect B4: Under old code, settle succeeds here, terminating agreement and killing dispute window!
    // Correct behavior: settlement must be rejected while dispute window is active.
    // This assertion fails under old code (it returns Ok(Ok(()))):
    assert_eq!(
        client.try_settle(&ag_id, &sponsor),
        Err(Ok(AgreementError::DisputeWindowActive))
    );
}

#[test]
fn test_red_b7_expire_rejects_unfunded_requested_agreement() {
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

    let token_admin = Address::generate(&env);
    let token_contract_id = env.register_stellar_asset_contract(token_admin);
    let sponsor = Address::generate(&env);

    let (client, admin) = create_client(&env);
    client.initialize(&admin, &registry_client.address, &token_contract_id);

    let current_time = env.ledger().timestamp();
    let funding_deadline = current_time + 1000;
    let care_deadline = funding_deadline + 1000;

    let ag_id = client.create_agreement(
        &provider,
        &sponsor,
        &attester,
        &BytesN::from_array(&env, &[3u8; 32]),
        &BytesN::from_array(&env, &[4u8; 32]),
        &1000_0000000i128,
        &800_0000000i128,
        &funding_deadline,
        &care_deadline,
        &500u64,
    );

    // Advance past funding deadline without funding
    env.ledger().with_mut(|l| {
        l.timestamp = funding_deadline + 1;
    });

    // Defect B7: Under old code, expire returns Err(Ok(AgreementError::InvalidState)) on Requested agreements!
    // Correct behavior: Requested agreement past funding deadline should expire cleanly.
    // This assertion fails under old code:
    assert!(client.try_expire(&ag_id).is_ok());
    assert_eq!(client.get_agreement(&ag_id).state, AgreementState::Expired);
}

