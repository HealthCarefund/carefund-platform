# CareFund Financial Behavior Specification and Decision Memo

**Date**: 2026-10-09  
**Status**: Proposal for Maintainer Approval (Block 3A Gate)  
**Governing Principle**: Strict Escrow Conservation (`Sum(Obligations) == Held Escrow`, zero orphaned residual)

---

## 1. Escrow Accounting and Financial Invariants

For every agreement initialized with `funding_amount = F` and `settlement_amount = S`:
1. **Preconditions**: `0 < S <= F`. `F` stroops are deposited by `sponsor` upon `fund()`.
2. **Conservation of Value**:
   - On full settlement: Provider receives `S`, Sponsor receives `F - S` (surplus refund). Contract balance delta for this agreement is `-F`. Residual held in contract = 0.
   - On full refund (dispute `Refund` or no-care `expire`): Sponsor receives `F`, Provider receives 0. Contract balance delta for this agreement is `-F`. Residual held in contract = 0.
   - On cancellation or unfunded expiry: No funds deposited, no transfers occur. Residual = 0.
3. **Multi-Agreement Isolation**:
   - Escrow accounting is strictly per-agreement. Payouts for agreement $A_i$ depend solely on $A_i$'s recorded deposit, never using tokens deposited for concurrent agreement $A_j$.

---

## 2. State Transition and Financial Truth Table

| Current State | Target State | Trigger Function | Authorized Caller | Time Condition | Token Movement (Atomic) | Residual Balance | Default / Timeout Behavior |
|---|---|---|---|---|---|---|---|
| `None` | `Requested` | `create_agreement` | `provider` | `now < funding_deadline` | None | 0 | Can be cancelled by provider or expire after deadline |
| `Requested` | `Cancelled` | `cancel` | `provider` | `now < funding_deadline` | None | 0 | Terminal state |
| `Requested` | `Expired` | `expire` | Anyone | `now >= funding_deadline` | None | 0 | Terminal state for un-funded proposals |
| `Requested` | `Funded` | `fund` | `sponsor` | `now < funding_deadline` | Sponsor -> Contract: `F` | `F` | If not funded before deadline, transitions to Expired |
| `Funded` | `CareConfirmed` | `attest_care` | `attester` | `now <= care_deadline` | None | `F` | If not attested before deadline, eligible for refund |
| `Funded` | `Disputed` | `open_dispute` | `sponsor` or `provider` | `care_deadline < now <= care_deadline + dispute_window_secs` | None | `F` | Freezes agreement pending admin adjudication |
| `Funded` | `Expired` | `expire` | Anyone | `now > care_deadline + dispute_window_secs` | Contract -> Sponsor: `F` | 0 | Terminal state: full deposit refunded to sponsor |
| `CareConfirmed` | `Disputed` | `open_dispute` | `sponsor` or `provider` | `care_deadline < now <= care_deadline + dispute_window_secs` | None | `F` | Freezes agreement pending admin adjudication |
| `CareConfirmed` | `Settled` | `settle` | Permissionless (or Sponsor/Provider) | `now > care_deadline + dispute_window_secs` | Contract -> Provider: `S`<br>Contract -> Sponsor: `F - S` | 0 | Terminal state: provider paid, surplus returned |
| `CareConfirmed` | `Expired` | `expire` | **ILLEGAL** | N/A | **REJECTED (`InvalidState`)** | N/A | Attested care can NEVER be expired to cheat provider |
| `Disputed` | `Settled` | `resolve_dispute(Settle)` | `admin` | Disputed | Contract -> Provider: `S`<br>Contract -> Sponsor: `F - S` | 0 | Terminal state |
| `Disputed` | `Refunded` | `resolve_dispute(Refund)` | `admin` | Disputed | Contract -> Sponsor: `F` | 0 | Terminal state |
| `Disputed` | `Origin` | `resolve_dispute(Resume)` | `admin` | Disputed | None | `F` | Resumes to `Funded` or `CareConfirmed` |

---

## 3. Decision Points for Maintainer Authorization

### Decision 1: Surplus Return Policy (`F - S`)
- **Current Behavior**: `settle()` and dispute `Settle` disburse only `S` to provider, permanently stranding `F - S` in the contract address.
- **Recommended Policy**: Atomically disburse `S` to `provider` and return `F - S` to `sponsor` in the same transaction.
- **Trade-off**: Protects sponsor capital and ensures contract balance returns to 0 upon settlement.

### Decision 2: Dispute Window and Settlement Non-Overlap
- **Current Behavior**: `settle()` is permitted whenever `now > care_deadline`, allowing settlement at `care_deadline + 1`, race-cutting the dispute window (`care_deadline < now <= care_deadline + dispute_window_secs`).
- **Recommended Policy**: `settle()` is strictly allowed only after the dispute window has elapsed (`now > care_deadline + dispute_window_secs`).
- **Trade-off**: Protects counterparty objection rights. If an immediate settlement is desired, `dispute_window_secs` should be configured smaller by mutual agreement at creation time.

### Decision 3: Settlement Invocation Authority (Liveness vs Permissioning)
- **Current Behavior**: `settle(agreement_id, sponsor)` strictly requires `sponsor.require_auth()`. If the sponsor is unresponsive, provider reimbursement is permanently locked.
- **Option A (Recommended)**: Make `settle(env, agreement_id: u64)` callable by anyone after the dispute window closes. Because payment parameters (`provider`, `sponsor`, `S`, `F - S`) are immutably pre-set, no caller can divert or modify funds.
- **Option B**: Require `caller.require_auth()` where caller can be either `sponsor` or `provider`.
- **Option C**: Retain sponsor-only `settle()` and introduce a new `finalize_settlement()` method callable by the provider after an extended grace period.

### Decision 4: Admin Dispute Liveness Trust Assumption
- **Current Behavior**: Only `admin` can resolve disputes.
- **Recommended Policy**: Maintain existing governance boundary (admin adjudication) without inventing an unauthorized automated dispute timeout. Document explicitly as a known administrative trust boundary in `SECURITY.md` and `limitations.md`.
