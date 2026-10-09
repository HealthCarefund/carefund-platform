# Financial Safety and Contract Correctness Review

**Date**: 2026-10-09  
**Repository**: `HealthCarefund/carefund-platform`  
**Starting Commit Baseline**: `984b305b5a5b0c829b00568d82eeeddf1fdac40f`  
**Scope**: `contracts/care-agreement`, `contracts/provider-registry`, `@carefund/sdk`, `apps/api`, `apps/web`, and documentation

---

## Executive Summary

A financial safety and invariants audit of the CareFund smart contracts identified critical escrow custody, authorization, and timing defects in `contracts/care-agreement/src/lib.rs`. Under the pre-remediation implementation, token balances can become permanently stranded in the contract address, legitimate settlements can be maliciously preempted, surplus funding is orphaned, and settlement overlaps the dispute window.

This document records the initial findings matrix before code modifications.

---

## Findings Matrix

| ID | Title | File & Line References | Impacted States | Financial Effect | Existing Tests | Severity |
|---|---|---|---|---|---|---|
| **B1** | Expiration locks deposited escrow | `contracts/care-agreement/src/lib.rs:534-573` | `Funded`, `CareConfirmed` -> `Expired` | Permanent loss of funds: deposited tokens remain in contract address with no refund transfer. | `test_expire_agreement_from_funded_success` asserts state `Expired` only, without checking SAC token balances. | **Critical** |
| **B2** | Permissionless preemption of attested settlement | `contracts/care-agreement/src/lib.rs:534-573` | `CareConfirmed` -> `Expired` | Deprivation of earned payment: any third party can call `expire()` on attested care after `care_deadline`, blocking provider payout. | `test_expiry_transitions` exercises permissionless expiry without verifying role authorization or protecting provider rights. | **Critical** |
| **B3** | Unclaimed surplus retained in contract | `contracts/care-agreement/src/lib.rs:748-760`, `839-851` | `CareConfirmed` -> `Settled`, `Disputed` -> `Settled` | Trapped capital: when `funding_amount > settlement_amount`, difference (`F - S`) is retained with no withdrawal or refund path. | `test_cross_contract_multi_agreement_token_balance_lifecycle` explicitly observes and asserts remaining balance as normal behavior. | **High** |
| **B4** | Dispute window and settlement race condition | `contracts/care-agreement/src/lib.rs:668-675`, `734-737` | `CareConfirmed`, `Disputed`, `Settled` | Truncated objection rights: `settle()` allows immediate termination when `now > care_deadline`, racing with `open_dispute()`. | `test_settle_agreement_success` runs immediately after care deadline, ignoring open dispute window. | **High** |
| **B5** | Settlement liveness dependency on sponsor | `contracts/care-agreement/src/lib.rs:712-713` | `CareConfirmed` -> `Settled` | Potential fund lock: `settle()` requires `sponsor.require_auth()`. An uncooperative or lost sponsor permanently blocks provider reimbursement. | `test_settle_agreement_success` always mocks sponsor authorization; no uncooperative sponsor path tested. | **High** |
| **B6** | Admin dispute resolution liveness | `contracts/care-agreement/src/lib.rs:784-887` | `Disputed` -> terminal | Trust bottleneck: only admin can resolve disputes. If admin is inactive, funds in `Disputed` state remain locked indefinitely. | `test_resolve_dispute_*` tests assume responsive admin; no timeout or default mechanism exists. | **Medium** |
| **B7** | Requested agreement expiry divergence | `contracts/care-agreement/src/lib.rs:544-548` | `Requested` | Operational friction: `expire()` rejects `Requested` agreements, contradicting documented protocol lifecycle. | `test_expire_agreement_invalid_state_rejected` explicitly tests rejection of `Requested`. | **Low** |
| **B8** | Documentation claims divergence from source | `docs-site/`, `README.md` | Multiple | Misleading claims: documentation asserts automatic refunds, nonexistent methods (`refund()`), and inaccurate authorization requirements. | N/A (Documentation divergence). | **Medium** |

---

## Detailed Finding Descriptions

### B1. Expiration Locks Previously Deposited Tokens
When an agreement is funded, the sponsor transfers `funding_amount` into the contract instance via `SAC.transfer`. When `expire()` is invoked after `care_deadline`, the contract transitions `agreement.state` to `AgreementState::Expired` without invoking `token::Client::transfer`. Because `Expired` is a terminal state, neither sponsor nor provider can retrieve the funds.

### B2. Unauthorized Third Party Preemption
`expire()` requires no caller authorization (`require_auth`). If an agreement has reached `CareConfirmed`, anyone can invoke `expire()` once `now > care_deadline`. This moves the agreement to `Expired` and permanently blocks `settle()` (which strictly requires `CareConfirmed`), depriving the provider of earned compensation.

### B3. Surplus Trapped in Contract
The protocol permits `funding_amount >= settlement_amount > 0`. In `settle()` and `resolve_dispute(Settle)`, the contract only transfers `settlement_amount` to the provider. The remaining surplus (`funding_amount - settlement_amount`) stays in the contract address. There is no administrative sweep or refund function, permanently orphaning the surplus.

### B4. Dispute Window Race Condition
`open_dispute()` is valid during `care_deadline < now <= care_deadline + dispute_window_secs`. However, `settle()` is also valid whenever `now > care_deadline`. A sponsor can call `settle()` immediately at `care_deadline + 1`, cutting off the provider's window to raise a dispute regarding service adjustments or terms.

### B5. Settlement Liveness Risk
`settle()` requires `sponsor.require_auth()`. Once care is attested and the dispute window has elapsed without dispute, the provider has an unconditional claim to `settlement_amount`. Requiring the sponsor to sign `settle` creates counterparty risk where an unresponsive sponsor permanently withholds payment.

### B6. Admin Dispute Liveness
`resolve_dispute()` can only be called by the admin. If an agreement enters `Disputed` and the admin key is lost or abandoned, the escrowed funds remain locked in the contract forever.

### B7. Divergence on Requested Expiry
The protocol documentation states that unfunded `Requested` agreements can expire after `funding_deadline`. However, `expire()` in code strictly checks `state == Funded || state == CareConfirmed`, returning `AgreementError::InvalidState` for `Requested` agreements.

### B8. Documentation Inaccuracies
Public documentation states that `expire()` automatically returns funds, references an un-implemented `refund()` function, claims `create_agreement` accepts sponsor-or-provider authorization (when code requires provider), and describes settlement as autonomous despite requiring sponsor authorization.
