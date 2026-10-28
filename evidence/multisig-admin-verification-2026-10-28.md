# Multisignature Administrator Verification

Date: 2026-10-28. Branch `security/multisig-admin-verification`. Scope: whether
CareFund's care-agreement and provider-registry admin roles can safely be
operated by a classic Stellar multisignature (`G...`) account, as proposed in
[dispute-recovery-proposal.md section 3, option B](../docs/dispute-recovery-proposal.md).

No contract code changed in this branch. No existing Testnet contract,
admin account, agreement or balance was touched. Everything below is local,
in-process verification using the Soroban test host — see the limitation
in section 5.

## 1. Toolchain and protocol

- Rust `1.98.1`.
- `soroban-sdk = "28.0.0"` (pinned in both contracts' `Cargo.toml`).
- `soroban-env-host 28.0.2` (resolved dependency; source read directly from
  the cached crate to verify authorization behavior — see section 2).
- Stellar CLI `28.1.0`.
- Protocol: the test ledger this SDK version runs against reports
  `protocol_version: 28` (observed directly in a `cargo test` snapshot, and
  traced to `soroban_env_host::meta::INTERFACE_VERSION.protocol` in the
  pinned `soroban-env-host` source). This is what was verified against, not
  assumed from the SDK's major version number.

## 2. The authorization model (3A)

Five things the task asked to distinguish, verified by reading the pinned
`stellar-xdr` 28.0.0 and `soroban-env-host` 28.0.2 source, not assumed:

1. **Transaction-envelope signature.** Authorizes the transaction as a
   whole, checked against its *source account*'s thresholds for whichever
   operation-threshold category (low/medium/high) the transaction's
   operations require.
2. **Soroban authorization entry (`SorobanAuthorizationEntry`).** A
   separate, per-invocation structure carried inside `InvokeHostFunctionOp`.
   It authorizes one invocation tree on behalf of one `Address`, with its
   own nonce and expiration ledger, independent of the transaction's
   sequence number. Its signed payload is
   `HashIdPreimage::SorobanAuthorization { network_id, nonce,
   signature_expiration_ledger, invocation }` (XDR-encoded, SHA-256 hashed),
   not the transaction hash.
3. **A classic G-account with multiple signers.** An `AccountEntry` ledger
   object with a `signers: Signer[]` list (each an Ed25519/PreAuthTx/HashX/
   signed-payload key plus a `u32` weight) and `thresholds: [master_weight,
   low, medium, high]`.
4. **An account used as the transaction source.** Whichever account's
   sequence number the transaction consumes and which pays the fee by
   default. This is unrelated to which `Address` a Soroban authorization
   entry inside that transaction names — a transaction's source account and
   the account authorizing a given contract call can be different accounts.
5. **An admin account that authorizes a contract invocation while another
   account pays transaction fees.** Supported by the XDR structure directly:
   a `SorobanAuthorizationEntry` only names the authorizing `Address`; the
   transaction's source account (or a fee-bump transaction's source) is
   independent of it. This was verified from the type definitions, not
   exercised against a live network (see section 5 for why).

**Key finding (verified in source, not assumed): Soroban authorization for
a classic-account `Address` is checked against that account's MEDIUM
threshold only** (`ThresholdIndexes::Med`), regardless of what the invoked
contract function does, in
`soroban-env-host-28.0.2/src/builtin_contracts/account_contract.rs::check_account_authentication`.
Low and high thresholds are not consulted by `require_auth()` on a classic
account; they only matter for classic transaction operations (e.g.
`SetOptions`, `AllowTrust`) and the transaction-envelope check in item 1
above.

**Frontend finding:** the web app's signing path
(`apps/web/lib/use-transaction-flow.ts`) calls `awaitWalletSignature` once,
against a single configured wallet signer, and submits immediately after.
There is no code path for collecting a partially-signed envelope from one
signer, handing it to a second signer, and merging signatures before
submission. The task's caution not to assume the frontend can collect
multisignature approvals is correct: **it cannot, today.** Any multisig
admin workflow needs an off-chain coordination step (e.g. Stellar Laboratory,
the `stellar` CLI's multi-signer flows, or a purpose-built tool) outside
this application.

## 3. The proposed 2-of-3 policy — is it supported?

Yes, with one verified correction to how the account must be built.

**Configuration tested:** three independent Ed25519 keypairs, each added as
an explicit `Signer` with weight 1; thresholds `(low, medium, high) = (2, 2,
2)`; the account's own identity keypair (whatever key its `AccountId` wraps)
is a fourth, freshly generated, immediately discarded key with
`master_weight = 0` and is never used to sign anything.

**Why a fourth identity key is required — a protocol detail found by
testing, not assumed:** `get_signer_weight_from_account` in
`soroban-env-host-28.0.2/src/host/data_helper.rs` special-cases the
account's own public key: if the signing key equals the account's
`AccountId`, the function returns `thresholds.master_weight` directly and
**never consults the explicit `signers` list for that key**, even if the
same key also appears there with its own weight. The first version of this
test's account builder reused one of the three operational keys as the
account's identity key; every real signature from that key was then
authenticated as the (zero-weight) master key and rejected with "signer
does not belong to account" — not because the signature was invalid, but
because the explicit entry was shadowed. Fixed by giving the account a
separate, inert identity key not in the signer set. This matches standard
Stellar multisig practice (disable the master key, operate entirely through
explicit signers) but is easy to get wrong, and the failure mode is a
cryptic "signer does not belong to account" rather than an obviously wrong
threshold.

**Requirements verified against this policy:**

- A single signer (weight 1) cannot reach any of the three thresholds
  (all are 2) — confirmed: cannot authorize a privileged contract call,
  and (by the same arithmetic, not independently executed against a
  classic `SetOptions` operation — see section 5) could not change the
  signer set or thresholds either, since that requires the high threshold.
- Losing one signer leaves two signers with combined weight 2, which meets
  the medium threshold (contract calls keep working) and the high
  threshold (a `SetOptions` operation to add a replacement signer would
  also be authorizable by the two survivors, by the same threshold value —
  not independently executed; see section 5).
- An unknown key (never added as a signer) cannot contribute weight even
  paired with a valid signer: the host rejects it specifically as "signer
  does not belong to account," distinct from "insufficient weight."

## 4. Real-signature test suites added

No contract code changed. New dev-only test-support modules and test files,
gated `#[cfg(test)]`, using disposable in-process Ed25519 keys generated by
`rand::rngs::OsRng` and discarded at the end of each test process — never
committed, logged, or reused across tests:

- `contracts/care-agreement/src/multisig_testutils.rs` — builds a real
  `AccountEntry` ledger entry via `env.host().add_ledger_entry(...)` and
  signs genuine `SorobanAuthorizationEntry` values (real Ed25519 signatures
  over the real `HashIdPreimage::SorobanAuthorization` payload), then calls
  `env.set_auths(&entries)`. No `mock_all_auths()` or `mock_auths()` is used
  for any operation under test (only for fixture setup, e.g. minting test
  tokens).
- `contracts/care-agreement/src/test_multisig_admin.rs` — 11 tests.
- `contracts/provider-registry/src/multisig_testutils.rs` — same approach,
  duplicated because the registry crate has no dependency on care-agreement.
- `contracts/provider-registry/src/test_multisig_admin.rs` — 5 tests.

Run with: `cargo test --offline -p care-agreement -p provider-registry
test_multisig_admin`. All 16 pass.

### care-agreement (11 tests, all real signatures)

| # | Scenario | Result |
|---|---|---|
| 1 | Two valid signers authorize `resolve_dispute` | real auth accepted |
| 2 | One signer attempts the same call alone | real auth rejected |
| 3 | An unknown signer cannot supply the missing authorization | rejected with "signer does not belong to account" |
| 4 | Losing one signer still leaves two valid signers able to act | accepted |
| 5 | A multisig admin resolves an existing dispute (`Settle` branch, real escrow transfer) | accepted; provider paid `S`, sponsor paid `F-S` |
| 6 | A multisig admin authorizes `transfer_admin` | accepted |
| 7 | The incoming (also multisig) admin must also authorize acceptance | rejected when the incoming admin supplies no authorization |
| 7b | The incoming admin's own signers must also clear *its* threshold | rejected when only 1 of 3 incoming signers signs |
| 8 | The previous admin loses authority after rotation | its old 2-of-3 signature set is rejected post-rotation; the new admin's signers succeed |
| 9 | Escrow balances are unchanged by the handover itself, and the new admin can still settle correctly afterwards | confirmed by token-balance snapshot before/after |
| — | A care agreement is unaffected by a *separate* contract's (provider-registry's) multisig admin rotating | confirmed end to end: create → fund → attest → settle still succeeds |

### provider-registry (5 tests, all real signatures)

| # | Scenario | Result |
|---|---|---|
| 1 | Two authorized signers perform a privileged op (`suspend_provider`) | accepted |
| 2 | One signer cannot perform it | rejected |
| 3 | A multisig admin rotates to an authorized multisig replacement | accepted |
| 4 | Existing providers and attesters are unchanged by the rotation | confirmed by record equality before/after, plus `is_provider_active`/`check_attester` |
| 5 | The new admin can operate the registry after rotation; the old admin's signers (even a full 2-of-3) cannot | confirmed both directions |

## 5. What remains unverified, and why

The in-process Soroban test host (`soroban-sdk`'s `Env`) implements the same
authentication code that runs on Testnet/Mainnet (`soroban-env-host`'s
`check_account_authentication`, confirmed by reading that exact source), so
the signature and threshold checks above are genuine, not mocked. What it
does **not** reproduce is anything handled by `stellar-core` outside that
shared host crate:

- **Classic `SetOptions` behavior** (adding/removing signers, changing
  thresholds) is a classic operation applied by `stellar-core`, not by
  `soroban-env-host`. This was not exercised — the test ledger entries were
  written directly, not produced by submitting a `SetOptions` operation.
  The threshold arithmetic in section 3 is correct by the same formula
  `soroban-env-host` uses for `require_auth`, but the actual `SetOptions`
  application path (e.g. protocol rules about reducing thresholds below the
  number of signers, or self-referential signer entries) was not run.
- **Transaction-envelope-level source-account authorization** (item 1 in
  section 2) and the separation of a fee-paying source account from the
  Soroban-authorizing address (item 5) were verified from the XDR
  structure, not exercised end to end on a network, because:
  - `docker pull stellar/quickstart` (needed for a local standalone
    network) stalled indefinitely in this environment (reachable via
    `docker login`-style auth handshake, but blob downloads never
    completed) — not a design choice, a sandbox network limitation.
  - A fresh Testnet deployment was explicitly out of scope for this task
    (Phase 4), and reusing the existing CareFund Testnet admin or contracts
    for this experiment was explicitly prohibited.
- Transaction-source vs. non-source-account authorization paths were
  therefore not checked "separately" against a live network as the task
  asked; the structural answer in section 2 item 4/5 is as far as this
  environment allows.

None of this was replaced with a mock standing in for the missing proof —
where real network verification was blocked, this document says so instead
of claiming equivalence.

## 6. Financial-safety findings

- Rotating either contract's admin to or from a multisig account touches
  only the stored admin address; no agreement/registry record, escrow
  balance, or error number changes (same code path already verified for a
  single-key admin in PR #39; multisig changes only which `Address` the
  authorization check resolves to).
- A dishonest or colluding **subset** below the medium threshold (here, any
  single signer) cannot authorize anything — confirmed.
- Multisignature administration protects against losing **one** signing
  key out of three, provided the remaining signers' combined weight still
  meets the threshold. It does **not** protect against:
  - losing enough keys to make the threshold unreachable (here, losing 2 of
    3 keys permanently locks the account out of both contract
    authorization and its own recovery via `SetOptions`, since both need
    weight 2);
  - collusion between enough signers to meet the threshold (here, any 2 of
    3 signers can authorize anything the admin role can, including
    `resolve_dispute`'s `Refund`/`Settle`/`Resume` choice and
    `transfer_admin`);
  - a dishonest administrator who legitimately controls enough keys. The
    contracts cannot distinguish a multisig admin that disagrees honestly
    from one that is colluding to extract value within the rules the admin
    role already has.
- This does not change the stale-dispute-timeout question: multisig
  administration makes the admin's key *harder to lose*, not more likely to
  *act*. A multisig admin that is simply unavailable (not lost, just
  unresponsive) still leaves a `Disputed` agreement locked exactly as a
  single-key admin would. The stale-dispute resume mechanism (option C in
  [dispute-recovery-proposal.md](../docs/dispute-recovery-proposal.md)) is
  unrelated to this work and remains deferred, with no timeout approved.
- The historical Testnet contracts (`care_agreement` WASM `b543e9a7...`,
  `provider_registry` WASM `30924b09...`) have no `transfer_admin` at all,
  so none of this — single-key or multisig rotation — is usable on them
  without a fresh deployment, which is out of scope here (Phase 4).

## 7. Files added

- `contracts/care-agreement/src/multisig_testutils.rs`
- `contracts/care-agreement/src/test_multisig_admin.rs`
- `contracts/provider-registry/src/multisig_testutils.rs`
- `contracts/provider-registry/src/test_multisig_admin.rs`
- `ed25519-dalek`, `sha2`, `rand` added as dev-dependencies to both
  contract crates (already present as transitive dependencies in
  `Cargo.lock`; no new crates were fetched).

No secret keys, seed phrases, or private configuration were committed. Every
keypair used is generated fresh in-process by each test run via
`rand::rngs::OsRng` and exists only for that run.
