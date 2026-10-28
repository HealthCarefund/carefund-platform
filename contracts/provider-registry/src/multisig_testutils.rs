#![cfg(test)]
// Genuine-signature multisignature test support for a classic Stellar
// G-account used as a contract admin.
//
// This builds a real `AccountEntry` ledger entry (with signers and
// thresholds) and signs Soroban authorization entries with real Ed25519
// keys, then calls `env.set_auths(&entries)`. The host verifies those
// signatures and the account's signer weights against its medium
// threshold exactly as it would for a real network transaction
// (`soroban-env-host`'s `check_account_authentication` in
// `builtin_contracts/account_contract.rs`, pinned version 28.0.2). It
// does not use `mock_all_auths()` or `mock_auths()`.
extern crate std;

use ed25519_dalek::{Signer as _, SigningKey};
use sha2::{Digest, Sha256};
use soroban_sdk::xdr::{
    self, AccountEntry, AccountEntryExt, AccountId, Hash, HashIdPreimage,
    HashIdPreimageSorobanAuthorization, InvokeContractArgs, LedgerEntry, LedgerEntryData,
    LedgerEntryExt, LedgerKey, LedgerKeyAccount, PublicKey, ScAddress, ScBytes, ScMap, ScMapEntry,
    ScSymbol, ScVal, ScVec, SequenceNumber, Signer, SignerKey, SorobanAddressCredentials,
    SorobanAuthorizationEntry, SorobanAuthorizedFunction, SorobanAuthorizedInvocation,
    SorobanCredentials, Thresholds, Uint256, WriteXdr,
};
use soroban_sdk::{Address, Env, IntoVal, TryFromVal, TryIntoVal, Val};
use std::rc::Rc;
use std::vec::Vec as StdVec;

/// Generates a fresh, disposable Ed25519 keypair for use as one signer of a
/// test multisignature account. Never used for anything outside this
/// in-process test environment, and never logged or persisted.
pub fn generate_signer() -> SigningKey {
    let mut rng = rand::rngs::OsRng;
    SigningKey::generate(&mut rng)
}

fn account_id_of(key: &SigningKey) -> AccountId {
    AccountId(PublicKey::PublicKeyTypeEd25519(Uint256(
        key.verifying_key().to_bytes(),
    )))
}

fn address_to_account_id(_env: &Env, address: &Address) -> AccountId {
    let sc_address: ScAddress = address.clone().into();
    match sc_address {
        ScAddress::Account(account_id) => account_id,
        _ => panic!("expected a classic account address"),
    }
}

/// Installs a classic Stellar account ledger entry whose weighted signers
/// are exactly `signers` (each a disposable keypair, weight 1) and whose
/// threshold configuration is `(low, med, high)`.
///
/// The account's own identity key (what determines its `AccountId`/address)
/// is a freshly generated, immediately discarded keypair with master
/// weight 0 — it is not one of `signers` and can never authorize anything.
/// This mirrors real Stellar multisig practice: a dedicated G-account whose
/// authority lives entirely in its explicit signer list, not in a "primary"
/// keyholder. (Soroban's account-authentication host code gives the
/// account's own public key special treatment — if it were also used as
/// one of the operational signers, that signer's entry in the explicit
/// list would be shadowed and only the master weight would apply to it;
/// see `soroban-env-host` 28.0.2
/// `builtin_contracts/data_helper.rs::get_signer_weight_from_account`.)
///
/// Returns the account's `Address`, usable anywhere a contract expects an
/// `Address` (e.g. as the `admin` passed to `initialize`/`transfer_admin`).
pub fn create_multisig_account(
    env: &Env,
    signers: &[&SigningKey],
    thresholds: (u8, u8, u8),
) -> Address {
    let identity = generate_signer();
    let account_id = account_id_of(&identity);

    let mut signer_entries: StdVec<Signer> = signers
        .iter()
        .map(|k| Signer {
            key: SignerKey::Ed25519(Uint256(k.verifying_key().to_bytes())),
            weight: 1,
        })
        .collect();
    signer_entries.sort_by(|a, b| a.key.cmp(&b.key));

    let entry = AccountEntry {
        account_id: account_id.clone(),
        balance: 0,
        seq_num: SequenceNumber(0),
        num_sub_entries: 0,
        inflation_dest: None,
        flags: 0,
        home_domain: Default::default(),
        thresholds: Thresholds([0, thresholds.0, thresholds.1, thresholds.2]),
        signers: signer_entries.try_into().unwrap(),
        ext: AccountEntryExt::V0,
    };

    let key = Rc::new(LedgerKey::Account(LedgerKeyAccount {
        account_id: account_id.clone(),
    }));
    let ledger_entry = Rc::new(LedgerEntry {
        last_modified_ledger_seq: 0,
        data: LedgerEntryData::Account(entry),
        ext: LedgerEntryExt::V0,
    });
    env.host()
        .add_ledger_entry(&key, &ledger_entry, None)
        .unwrap();

    ScAddress::Account(account_id).try_into_val(env).unwrap()
}

/// Looks up the current medium threshold weight actually stored for the
/// given account's ledger entry, so tests assert against the real stored
/// configuration rather than the value they intended to set.
pub fn medium_threshold(env: &Env, account: &Address) -> u8 {
    let account_id = address_to_account_id(env, account);
    let key = Rc::new(LedgerKey::Account(LedgerKeyAccount { account_id }));
    let (entry, _) = env.host().get_ledger_entry(&key).unwrap().unwrap();
    match &entry.data {
        LedgerEntryData::Account(a) => a.thresholds.0[2],
        _ => panic!("expected account entry"),
    }
}

fn to_scval<T: IntoVal<Env, Val>>(env: &Env, v: T) -> ScVal {
    let val: Val = v.into_val(env);
    ScVal::try_from_val(env, &val).unwrap()
}

fn signature_scval(sigs: &[(SigningKey, [u8; 64])]) -> ScVal {
    let mut entries: StdVec<(StdVec<u8>, StdVec<u8>)> = sigs
        .iter()
        .map(|(k, sig)| (k.verifying_key().to_bytes().to_vec(), sig.to_vec()))
        .collect();
    entries.sort_by(|a, b| a.0.cmp(&b.0));

    let scvals: StdVec<ScVal> = entries
        .into_iter()
        .map(|(pk, sig)| {
            let map = ScMap(
                StdVec::from([
                    ScMapEntry {
                        key: ScVal::Symbol(ScSymbol("public_key".try_into().unwrap())),
                        val: ScVal::Bytes(ScBytes(pk.try_into().unwrap())),
                    },
                    ScMapEntry {
                        key: ScVal::Symbol(ScSymbol("signature".try_into().unwrap())),
                        val: ScVal::Bytes(ScBytes(sig.try_into().unwrap())),
                    },
                ])
                .try_into()
                .unwrap(),
            );
            ScVal::Map(Some(map))
        })
        .collect();

    ScVal::Vec(Some(ScVec(scvals.try_into().unwrap())))
}

/// Builds a genuinely signed `SorobanAuthorizationEntry` authorizing
/// `contract.fn_name(args)` on behalf of `account` (a multisig account
/// created with [`create_multisig_account`]), using real Ed25519
/// signatures from exactly `signers` (a subset, or all, of the account's
/// registered keys). Passing too few signers to clear the account's
/// medium threshold produces an entry the host will genuinely reject at
/// authentication time — that rejection is itself the thing under test in
/// the "one signer is rejected" scenarios.
pub fn sign_entry(
    env: &Env,
    account: &Address,
    signers: &[&SigningKey],
    nonce: i64,
    contract: &Address,
    fn_name: &str,
    args: soroban_sdk::Vec<Val>,
) -> SorobanAuthorizationEntry {
    let sc_address: ScAddress = account.clone().into();
    let contract_address: ScAddress = contract.clone().into();

    let scval_args: StdVec<ScVal> = args.iter().map(|v| to_scval(env, v)).collect();

    let invocation = SorobanAuthorizedInvocation {
        function: SorobanAuthorizedFunction::ContractFn(InvokeContractArgs {
            contract_address,
            function_name: xdr::StringM::try_from(fn_name).unwrap().into(),
            args: scval_args.try_into().unwrap(),
        }),
        sub_invocations: Default::default(),
    };

    let signature_expiration_ledger = env.ledger().sequence() + 1000;
    let network_id_bytes = env.ledger().network_id().to_array();

    let preimage = HashIdPreimage::SorobanAuthorization(HashIdPreimageSorobanAuthorization {
        network_id: Hash(network_id_bytes),
        nonce,
        signature_expiration_ledger,
        invocation: invocation.clone(),
    });
    let payload = Sha256::digest(preimage.to_xdr(xdr::Limits::none()).unwrap());

    let sigs: StdVec<(SigningKey, [u8; 64])> = signers
        .iter()
        .map(|k| {
            let sig = k.sign(&payload);
            ((*k).clone(), sig.to_bytes())
        })
        .collect();

    SorobanAuthorizationEntry {
        credentials: SorobanCredentials::Address(SorobanAddressCredentials {
            address: sc_address,
            nonce,
            signature_expiration_ledger,
            signature: signature_scval(&sigs),
        }),
        root_invocation: invocation,
    }
}
