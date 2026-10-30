# Trust Model and Security Evidence

This document answers four questions for a reviewer: who has to be trusted for what, what the contracts enforce, what is only asserted by people, and where the proof for each guarantee lives.

It consolidates points already spread across [SECURITY.md](../SECURITY.md), [the financial specification](./financial-behavior-specification.md) and the [evidence ledger](../evidence/index.md). It does not replace them. For the system description see [ARCHITECTURE.md](../ARCHITECTURE.md).

**Scope statement.** CareFund has had no third-party security audit. It is a Testnet prototype. It does not claim verified medical accreditation, independently established clinical facts, HIPAA or GDPR compliance, or production readiness. An authorized attester's signed on-chain statement is not independent proof that medical treatment took place.

## 1. Trust assumptions by participant

| Participant | What they must be trusted to do | What the contracts stop them doing |
|---|---|---|
| Admin | Register honest providers and attesters. Resolve disputes fairly and promptly. Protect the admin key. | Send funds anywhere except the agreement's provider or sponsor. Pick amounts. Touch non-disputed agreements. Sweep balances. Upgrade code. |
| Provider | Deliver the care they ask to be paid for. Create agreements with sensible terms. | Fund or attest an agreement. Settle early. Change terms after creation. Receive more than `settlement_amount`. |
| Sponsor | Review terms before funding. | Withdraw escrow outside the refund paths. Block settlement once the dispute window has passed without a dispute. |
| Attester | State truthfully that care was delivered. | Move funds. Attest after `care_deadline` or on an agreement that does not name them. |
| Anyone (settler) | Nothing. | Anything other than triggering the deterministic `settle` or `expire` once conditions hold. |
| Go API and database | Nothing for fund safety. They affect only convenience and the off-chain mirror. | Sign anything. Move funds. |
| Freighter and the user's browser | Display what is being signed and sign only what the user approves. | n/a (outside CareFund's control) |
| Stellar network and native asset contract | Behave as documented. | n/a |

## 2. What the contracts enforce

Each item is enforced on-chain by `require_auth` or by a check in the function body.

- Only the named sponsor can fund their agreement; only the named attester can attest it; only the creating provider can cancel it; only the sponsor or provider can open a dispute; only the admin can resolve one.
- Only a provider that the registry reports `Active` can create an agreement, and the named attester must be `Active` and bound to that provider at creation. The provider must still be `Active` at `fund`.
- Funding amount is positive, settlement amount is positive and not above funding, and deadlines are ordered (`now < funding_deadline < care_deadline`).
- State transitions follow the machine in [ARCHITECTURE.md section 6](../ARCHITECTURE.md#6-agreement-state-machine). Terminal states accept no further transition.
- Time rules, at exact boundaries: funding through `funding_deadline`, attestation through `care_deadline`, disputes in `(care_deadline, care_deadline + window]`, settlement and funded expiry after `care_deadline + window`.
- Payout destinations and amounts are fixed at creation (see section 5).
- Settlement is callable by anyone and needs no signature from either party.
- An attested (`CareConfirmed`) agreement cannot be expired.

## 3. What is only asserted by participants

The contracts record these but cannot verify them:

- That care was delivered. This is the attester's claim.
- That an attester is independent of the provider or sponsor. The contract has no such check, and the same address may appear in more than one role.
- That a provider or attester holds any real-world license. The registry stores an opaque 32-byte `provider_ref` or `credential_ref` chosen by the admin and does not check it against anything.
- That a commitment corresponds to a real patient, service or clinical note, or that two commitments are consistent with each other.
- That a hash is not guessable. Commitments are computed by callers off-chain.
- That an attestation commitment is unique. Nothing stops it being reused across agreements.

## 4. If the attester is dishonest

What happens, from the code:

1. The attester signs `attest_care` for an agreement that names them, before `care_deadline`. The contract records it and moves to `CareConfirmed`. It does not check anything about the claim.
2. The sponsor can only respond by calling `open_dispute` in `(care_deadline, care_deadline + dispute_window_secs]`.
3. If a dispute is opened, the admin decides: `Refund`, `Settle` or `Resume`.
4. If no dispute is opened, anyone can settle after the window and the provider is paid `settlement_amount`.

So a false attestation is stopped only by a sponsor who notices and disputes in time and an admin who rules for them. There is no on-chain evidence check, penalty, or attester bond. The admin can suspend or revoke the attester in the registry. In the current source that stops any new attestation on existing funded agreements, but it does not undo an attestation already recorded and does not claw anything back. The deployed Testnet contract predates this check.

This is the central limit of the design: CareFund moves money based on who signed, not on whether the underlying fact is true.

## 5. Administrative privileges

| Contract | Function | Effect | Constraints |
|---|---|---|---|
| provider-registry | `initialize` | Sets admin once | `AlreadyInitialized` thereafter |
| provider-registry | `register_provider`, `suspend_provider`, `reinstate_provider`, `revoke_provider` | Changes provider status | Revoked is terminal; no re-registration of a revoked address |
| provider-registry | `register_attester`, `suspend_attester`, `reinstate_attester`, `revoke_attester` | Changes attester status | Provider must be `Active` to bind a new attester |
| care-agreement | `initialize` | Sets admin, registry address, settlement asset once | One time |
| care-agreement | `resolve_dispute` | `Resume`, `Settle` or `Refund` a `Disputed` agreement | Funds go only to the agreement's provider or sponsor |

Also in the current source: `transfer_admin` on both contracts (merged to main in PR #39), which needs a signature from the current admin and from the incoming admin and changes only the admin address. It is not in the deployed Testnet contracts (care_agreement WASM `b543e9a7...`, provider_registry WASM `30924b0d...`), which have no admin rotation, so the deployed admins cannot use it. Either admin address can itself be a multisignature Stellar account; this is operational (no further contract change) and was verified with real signatures locally — see [multisig admin verification](../evidence/multisig-admin-verification-2026-10-28.md).

Absent: pause, upgrade, sweep, change of settlement asset, change of registry address, change of any agreement field, and any way to recover a lost admin key.

## 6. How disputes are authorized and resolved

- **Open**: sponsor or provider, signed. Agreement must be `Funded` or `CareConfirmed`. Time must be inside `(care_deadline, care_deadline + dispute_window_secs]`. The opener, time and origin state are recorded.
- **Resolve**: admin only. `Resume` returns to `Funded` or `CareConfirmed` as recorded. `Settle` pays the provider `settlement_amount` and the sponsor the surplus. `Refund` pays the sponsor the full deposit.
- **No timeout.** Nothing resolves a dispute if the admin does nothing.
- **No application UI.** There is no admin authentication in the app, so resolution is done with the Stellar CLI. Open issue #24 tracks an authenticated admin workflow.
- **Ruling discretion.** The admin's choice is not constrained by any evidence on-chain.

## 7. Financial invariants

Listed with where each is shown. For funding `F` and settlement `S`:

| Invariant | Source | Tests | Testnet evidence |
|---|---|---|---|
| `0 < S <= F` at creation | `lib.rs:367-372` | `test_create_agreement_invalid_amounts`, `test_security_invalid_amount_parameters_rejected` | n/a |
| Settlement pays `S` to provider and `F - S` to sponsor; per-agreement liability returns to 0 | `lib.rs:899-931` | `test_red_b3_surplus_retained_in_contract_after_settlement`, `test_per_agreement_conservation_vs_aggregate_contract_balance` | Agreement 1 (`9ae6dc48...`): 30,000,000 to provider, 20,000,000 to sponsor, contract balance 0 |
| Funded expiry refunds the full `F` | `lib.rs:659-681` | `test_red_b1_expire_locks_deposited_escrow` | Agreement 3 (`4d1b15c0...`): 20,000,000 refunded |
| Dispute `Refund` returns `F`; `Settle` splits as above | `lib.rs:1009-1058` | `test_resolve_dispute_refund_success`, `test_resolve_dispute_settle_success` | Agreement 4 (`55bfa433...`): 15,000,000 refunded |
| Agreements do not draw on each other's deposits | Per-agreement stored amounts | `test_cross_contract_multi_agreement_token_balance_lifecycle` | Agreements 3 and 4 funded together: 35,000,000 held, 20,000,000 out, 15,000,000 left, then 0 |
| Surplus uses checked subtraction | `lib.rs:914-917`, `:1023-1026` | covered by the settlement tests above | n/a |
| `care_deadline + window` cannot overflow | `lib.rs:389` | `test_create_agreement_dispute_window_overflow_rejected` | Fixed WASM deployed in Block 3B |
| Attested care cannot be expired | `lib.rs:683-685` | `test_red_b2_unauthorized_party_preempts_attested_settlement` | Agreement 1: expire on `CareConfirmed` returned `Error(Contract, #4)` in simulation |

Caveat: the pooled balance can exceed the sum of open escrows if someone sends tokens to the contract directly, and there is no way to recover them.

## 8. Replay, duplicate submission, double settlement and timing safeguards

| Threat | Safeguard | Evidence |
|---|---|---|
| Replay of a signed transaction | Stellar sequence numbers reject reuse of the same transaction. State machine rejects a repeated operation even in a new transaction. | Platform behavior; state-machine tests below |
| Double funding | `fund` requires `Requested` | `test_security_double_funding_attack_rejected` |
| Double settlement | `settle` requires `CareConfirmed`; after the first call the state is `Settled` | `test_security_double_settlement_rejected`; Testnet Agreement 1: second settle returned `Error(Contract, #4)` in simulation |
| Double dispute resolution | `resolve_dispute` requires `Disputed` | `test_security_double_dispute_resolution_rejected` |
| Settling while a dispute can still be opened | `settle` requires `now > care_deadline + window`; `open_dispute` requires `now <= care_deadline + window` | `test_security_dispute_timing_boundaries`, `test_red_b4_settlement_races_and_abbreviates_dispute_window`; Testnet Agreement 1: settle before care deadline `#11`, during window `#12` |
| Funding right at the deadline vs expiring | `fund` allows `now <= fd`; `expire` requires `now > fd` | `test_funding_deadline_exact_boundary_cannot_expire_but_can_fund`; Testnet Agreement 2: early expire `#9` |
| Duplicate API request builds | `Idempotency-Key` required, persisted in PostgreSQL | `apps/api/internal/api/idempotency_test.go`; [recovery evidence](../evidence/recovery-2026-09-27.md) |
| Resubmitting an uncertain transaction | No code resubmits; uncertain hashes are only looked up again | [recovery evidence](../evidence/recovery-2026-09-27.md) (source review, classified LOGICALLY COVERED) |

## 9. API and browser trust boundaries

- The API has no authentication and no rate limiting. A request for the wrong wallet fails a convenience check, and would fail on-chain anyway because only the right key can sign.
- The API builds unsigned transactions for seven operations. The browser parses the XDR and hands it to Freighter; from reading `apps/web/lib/use-transaction-flow.ts` I found no independent check that the transaction matches the requested operation. The user's protection is Freighter's prompt. This is inferred from the code and was not tested as an attack.
- `create_agreement` is built in the browser, not by the API.
- The browser submits directly to Soroban RPC.
- Request logging records method, path, status and duration only ([SECURITY.md](../SECURITY.md)).
- Wallet signature rejection does not submit and does not invent a hash (`apps/web/lib/transaction-rejection.test.ts`; live observation in Testnet evidence section 8.1).
- The Playwright suite runs without the Freighter extension and does not do live transactions.

## 10. Privacy and metadata correlation

- On-chain: addresses, commitments, amounts, deadlines, state, attester and attestation commitment, all public.
- Off-chain database: the same data plus intents and idempotency records. It holds no plaintext patient data by design; the web form accepts precomputed hex.
- A hash of guessable data is guessable. Salt or high-entropy inputs are the caller's job, and the protocol cannot check this.
- Repeated use of the same sponsor, provider, attester, amount or commitment links agreements together. Timing of transactions is also visible.
- The registry stores a `provider_ref` and `credential_ref` commitment per actor, which are linkable across agreements by design.
- No HIPAA or GDPR compliance is claimed.

## 11. Storage and recovery assumptions

- Agreement and registry entries use persistent storage; contract config uses instance storage. Both are extended to about 30 days on writes (`INSTANCE_TTL_EXTEND_TO` and `PERSISTENT_TTL_EXTEND_TO` = 518,400 ledgers). Entries not touched for longer can be archived.
- Archival does not delete state. Since Protocol 23, simulation includes a restore step, so a normal invocation restores archived entries. This is documented in the [financial specification](./financial-behavior-specification.md) and [Testnet evidence section 7](../evidence/testnet-2026-10-09-block3b.md). The restore flow was not independently exercised on Testnet in the recorded evidence.
- PostgreSQL can be rebuilt from chain state except for agreement intents and idempotency records. The Agreement #7 replay shows recovery from a missed ingestion.
- Event replay is limited by the RPC node's retention window (see [ARCHITECTURE.md section 10](../ARCHITECTURE.md#10-postgresql-as-a-rebuildable-mirror)).

## 12. Known unresolved security risks

1. Attester status is checked at creation and, in the current source, at attestation (tested locally by `test_attester_authorization.rs`). An accepted attestation is not retroactively invalidated after a later suspension or revocation, and provider status is not checked at attestation, `settle` or `expire`. The deployed Testnet contract predates the attestation-time check, so live agreements still allow a revoked attester to attest. Revoking the designated attester leaves the agreement unattestable with no replacement mechanism; the sponsor refund path remains through `expire` after the deadlines unless a dispute is open.
2. Open disputes can lock funds indefinitely. `transfer_admin` (merged to main in PR #39) allows a planned handover but not recovery from a lost key, because it needs the current admin's signature. The deployed Testnet contracts predate it and have no rotation at all. No timeout exists or is approved. See [dispute-recovery-proposal.md](./dispute-recovery-proposal.md).
3. A dishonest or compromised attester can trigger payment for undelivered care unless the sponsor disputes in time.
4. The admin alone decides disputes. Making the admin a multisignature account (verified with real signatures locally; see [multisig admin verification](../evidence/multisig-admin-verification-2026-10-28.md)) protects against losing one of several signing keys, but does not change this: any signer subset meeting the threshold decides disputes exactly as a single admin key would, and colluding signers at or above the threshold are equivalent to a dishonest admin.
5. No independence check between attester, provider and sponsor addresses.
6. Commitment privacy depends entirely on callers.
7. Browser trust in API-supplied transaction XDR (section 9).
8. Reconciler edge cases: RPC retention loss still requires explicit operator acknowledgement and cannot recreate pruned events. PR #42 added event pagination, cursor safety and retention-gap protection; PR #43 added auditable operator recovery tooling; PR #44 added authoritative state reads before checkpointing. These are current Go API changes, not functions deployed inside the historical Testnet contracts.
9. No third-party audit; `cargo audit` reports one allowed unmaintained-crate advisory (`paste`), `govulncheck` one non-critical transitive finding, `pnpm audit` two high advisories in optional transitive packages ([SECURITY.md](../SECURITY.md)).
10. Only native XLM was exercised. Issued assets bring trustline, freeze and clawback considerations that were not tested.
11. The contract emits events with the deprecated publish API.

## 13. Security evidence matrix

Evidence status uses the ledger's vocabulary: VERIFIED LIVE means a Testnet transaction or simulation; TESTED LOCALLY means an automated test; LOGICALLY COVERED means source review. Transaction hashes are abbreviated here; full values are in [evidence/testnet-2026-10-09-block3b.md](../evidence/testnet-2026-10-09-block3b.md).

| Guarantee | Implementation | Tests | Testnet evidence | Status |
|---|---|---|---|---|
| Only the named sponsor can fund | `fund`, `lib.rs:495,511` | `test_fund_agreement_wrong_sponsor_rejected`, `test_security_unauthorized_funder_rejected` | Agreement #8 funding signed by sponsor wallet in Freighter (`d2197fb8...`) | VERIFIED LIVE, TESTED LOCALLY |
| Only the named attester can attest | `attest_care`, `:715,731` | `test_attest_care_wrong_attester_rejected`, `test_security_unauthorized_attester_rejected` | Agreement #8 attestation by attester wallet (`019fc50f...`) | VERIFIED LIVE, TESTED LOCALLY |
| Attester must be `Active` and bound to the agreement's provider when attesting (source only; not in the Testnet deployment) | `attest_care` and `attester_is_currently_authorized` in `lib.rs` | `suspended_attester_cannot_attest_existing_funded_agreement`, `revoked_attester_cannot_attest_existing_funded_agreement`, `reinstated_attester_can_attest_again_before_care_deadline`, `revoked_attester_cannot_be_reinstated_and_stays_blocked`, `registry_that_cannot_return_an_attester_record_fails_closed` | Not deployed | TESTED LOCALLY |
| Attester must be bound to an `Active` provider at creation | `create_agreement`, `:406-420`; registry `check_attester`, `:216` | `test_cross_contract_mismatched_provider_attester_rejected`, `test_cross_contract_attester_revocation_blocks_agreement_creation`, registry `test_check_attester_wrong_provider` | `check_attester` returned true for the registered pair (section 4) | VERIFIED LIVE (positive case), TESTED LOCALLY |
| Provider must be `Active` to create and to fund | `:393-403`, `:522-537` | `test_create_agreement_inactive_provider_rejected`, `test_fund_agreement_inactive_provider_rejected` | Not exercised live | TESTED LOCALLY |
| Attestation only before care deadline | `:737` | `test_security_attestation_after_care_deadline_rejected` | Not exercised live | TESTED LOCALLY |
| Dispute only inside the window | `:820-825` | `test_open_dispute_before_dispute_window_rejected`, `test_open_dispute_after_dispute_window_rejected`, `test_security_dispute_timing_boundaries` | Agreement 4 dispute opened (`cd290e0d...`); 2026-09-27 simulation rejected early dispute with `#12` | VERIFIED LIVE, TESTED LOCALLY |
| Settlement is permissionless | `settle`, no `require_auth` | `test_settle_agreement_permissionless_does_not_require_sponsor_auth`, `test_security_settlement_is_permissionless_after_dispute_window` | Agreement #8 settled by third-party account `GDBPGGUE...` (`b7494722...`); Agreement 1 by `carefund-outsider` (`9ae6dc48...`) | VERIFIED LIVE, TESTED LOCALLY |
| Exact provider payment and surplus refund | `:899-931` | `test_settle_agreement_success`, `test_per_agreement_conservation_vs_aggregate_contract_balance` | Agreement #8: 8,000,000 to provider, 2,000,000 to sponsor; Agreement 1: 30,000,000 and 20,000,000 | VERIFIED LIVE, TESTED LOCALLY |
| Settlement blocked inside the dispute window | `:883-888` | `test_red_b4_settlement_races_and_abbreviates_dispute_window`, `test_settle_agreement_before_deadline_rejected` | Agreement 1 simulations `#11`, `#12` | VERIFIED LIVE, TESTED LOCALLY |
| No double settlement | state check, `:872` | `test_security_double_settlement_rejected` | Agreement 1 second settle `#4` (simulation) | VERIFIED LIVE, TESTED LOCALLY |
| Funded expiry gives a full refund | `:646-682` | `test_red_b1_expire_locks_deposited_escrow`, `test_expire_agreement_from_funded_success` | Agreement 3 (`4d1b15c0...`), 20,000,000 refunded | VERIFIED LIVE, TESTED LOCALLY |
| Unfunded agreement expires only after the funding deadline | `:641` | `test_funding_deadline_exact_boundary_cannot_expire_but_can_fund`, `test_funding_deadline_past_boundary_cannot_fund_but_can_expire` | Agreement 2: early expire `#9`, later expire (`80b818da...`) | VERIFIED LIVE, TESTED LOCALLY |
| Attested care cannot be expired | `:683-685` | `test_red_b2_unauthorized_party_preempts_attested_settlement` | Agreement 1 `#4` (simulation) | VERIFIED LIVE, TESTED LOCALLY |
| Only admin resolves disputes | `:962-968` | `test_resolve_dispute_unauthorized_fails` | Agreement 4 non-admin resolve `#2` (simulation); admin refund (`55bfa433...`) | VERIFIED LIVE, TESTED LOCALLY |
| Disputed agreement cannot be settled or expired | state checks | `test_resolve_dispute_invalid_state_rejected`, `test_expire_agreement_invalid_state_rejected` | Agreement 4: settle `#4`, expire `#4` (simulation) | VERIFIED LIVE, TESTED LOCALLY |
| Admin cannot choose destinations or amounts | `:993-1058`: transfer targets are `agreement.provider` and `agreement.sponsor`, amounts are stored fields | `test_resolve_dispute_settle_success`, `test_resolve_dispute_refund_success` | Agreement 4 | LOGICALLY COVERED, TESTED LOCALLY |
| Escrow accounting per agreement | stored amounts, no balance reads | `test_cross_contract_multi_agreement_token_balance_lifecycle` | Agreements 3 and 4 pooled | VERIFIED LIVE, TESTED LOCALLY |
| Wallet is the only signer | API and web hold no keys | `apps/web/lib/transaction-rejection.test.ts` | Freighter rejection observed, no hash generated (section 8.1) | VERIFIED (live observation), TESTED LOCALLY |
| Mirror recovers from missed ingestion | `apps/api/internal/reconcile/events.go` | `runner_test.go`, `events_test.go` | Agreement #7 replay from ledger 5106190 (section 8.2) | VERIFIED |
| API idempotency | `apps/api/internal/api/idempotency.go` | `idempotency_test.go` | n/a | TESTED LOCALLY |
| Admin cannot pause or upgrade | no such functions exist | n/a | n/a | LOGICALLY COVERED (search of both `lib.rs` files) |
| Admin rotation needs both the current and the incoming admin | `transfer_admin` in both contracts | `test_admin_rotation.rs` in each contract (targeted `mock_auths`, not `mock_all_auths`) | Not deployed | TESTED LOCALLY |

Not in the matrix because no evidence exists: independent audit, attester truthfulness, privacy of commitments, behavior with non-native assets, and a live archival-restore run.

## 14. Test totals

Current counts were rerun against the post-PR #44 source baseline: 135 care-agreement tests, 43 provider-registry tests (178 Rust tests total), 57 passing Go test results (52 top-level tests plus 5 named subtests from `go test -race -p 1 ./...`), 176 TypeScript unit tests (39 types, 116 SDK and 21 web), and 19 Playwright E2E tests. The attestation-time authorization and reconciliation tests are current-source coverage; the historical Block 3B Testnet deployment predates those source changes. The current CI reference is [post-merge run 41070128095](https://github.com/HealthCarefund/carefund-platform/actions/runs/41070128095).
