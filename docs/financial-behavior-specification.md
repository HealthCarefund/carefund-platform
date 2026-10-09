# CareFund Financial Behavior Specification and Decision Memo

**Date**: 2026-10-09  
**Status**: Adopted Specification (Block 3A Baseline and Pre-Deployment Hardening)  
**Governing Principle**: Strict Escrow Conservation (`Sum(Obligations) == Held Escrow`, zero orphaned residual)

---

## 1. Escrow Accounting and Financial Invariants

For every agreement initialized with `funding_amount = F` and `settlement_amount = S`:
1. **Preconditions**: `0 < S <= F`. `F` stroops are deposited by `sponsor` upon `fund()`.
2. **Conservation of Value**:
   - On full settlement: Provider receives `S`, Sponsor receives `F - S` (surplus refund via checked subtraction). Contract balance delta for this agreement is `-F`. Residual liability held in contract for this agreement = 0.
   - On full refund (dispute `Refund` or no-care `expire`): Sponsor receives `F`, Provider receives 0. Contract balance delta for this agreement is `-F`. Residual liability held in contract = 0.
   - On cancellation or unfunded expiry: No funds deposited, no transfers occur. Residual = 0.
3. **Multi-Agreement Isolation vs Aggregate Contract Balance**:
   - Escrow accounting is strictly per-agreement. Payouts for agreement $A_i$ depend solely on $A_i$'s recorded deposit, never drawing from tokens deposited for concurrent agreement $A_j$.
   - The aggregate token balance held by the shared contract address equals $\sum_{k \in \text{active}} F_k$. It returns to zero if and only if all agreements across the contract have finalized.
4. **Arithmetic Invariants and Checked Math**:
   - **Creation Invariant**: Validated at agreement creation (`settlement_amount <= funding_amount`), guaranteeing that $F - S \ge 0$.
   - **Checked Subtraction**: At runtime, surplus calculations in `settle()` and `resolve_dispute(Settle)` enforce `funding_amount.checked_sub(settlement_amount).ok_or(AgreementError::ArithmeticOverflow)?`, guarding against underflow under all conditions.

---

## 2. State Transition and Financial Truth Table

| Current State | Target State | Trigger Function | Authorized Caller | Time Condition | Token Movement (Atomic) | Residual Balance (Per Agreement) | Default / Timeout Behavior |
|---|---|---|---|---|---|---|---|
| `None` | `Requested` | `create_agreement` | `provider` | `now < funding_deadline` | None | 0 | Can be cancelled by provider or expire after deadline |
| `Requested` | `Cancelled` | `cancel` | `provider` | `now < funding_deadline` | None | 0 | Terminal state |
| `Requested` | `Expired` | `expire` | Anyone | `now > funding_deadline` | None | 0 | At `now == funding_deadline`, funding is permitted and expiry is rejected (`FundingDeadlineNotReached`) |
| `Requested` | `Funded` | `fund` | `sponsor` | `now <= funding_deadline` | Sponsor -> Contract: `F` | `F` | If not funded before or at deadline, transitions to Expired |
| `Funded` | `CareConfirmed` | `attest_care` | `attester` | `now <= care_deadline` | None | `F` | If not attested before deadline, eligible for refund |
| `Funded` | `Disputed` | `open_dispute` | `sponsor` or `provider` | `care_deadline < now <= care_deadline + dispute_window_secs` | None | `F` | Freezes agreement pending admin adjudication |
| `Funded` | `Expired` | `expire` | Anyone | `now > care_deadline + dispute_window_secs` | Contract -> Sponsor: `F` | 0 | Terminal state: full deposit refunded to sponsor |
| `CareConfirmed` | `Disputed` | `open_dispute` | `sponsor` or `provider` | `care_deadline < now <= care_deadline + dispute_window_secs` | None | `F` | Freezes agreement pending admin adjudication |
| `CareConfirmed` | `Settled` | `settle` | Permissionless | `now > care_deadline + dispute_window_secs` | Contract -> Provider: `S`<br>Contract -> Sponsor: `F - S` | 0 | Terminal state: provider paid, surplus returned |
| `CareConfirmed` | `Expired` | `expire` | **ILLEGAL** | N/A | **REJECTED (`InvalidState`)** | N/A | Attested care can NEVER be expired to cheat provider |
| `Disputed` | `Settled` | `resolve_dispute(Settle)` | `admin` | Disputed | Contract -> Provider: `S`<br>Contract -> Sponsor: `F - S` | 0 | Terminal state: provider paid, surplus returned |
| `Disputed` | `Refunded` | `resolve_dispute(Refund)` | `admin` | Disputed | Contract -> Sponsor: `F` | 0 | Terminal state: full deposit refunded |
| `Disputed` | `Origin` | `resolve_dispute(Resume)` | `admin` | Disputed | None | `F` | Resumes to `Funded` or `CareConfirmed` |

---

## 3. Storage TTL and Archival Restoration Model

1. **Storage Lifetimes**:
   - Instance storage threshold: 120,960 ledgers (~7 days); extended to 518,400 ledgers (~30 days).
   - Persistent storage threshold: 120,960 ledgers (~7 days); extended to 518,400 ledgers (~30 days).
   - Proactive TTL extension is called on every agreement operation.
2. **Archival Resilience**:
   - Dormant agreements past 30 days are moved to cold archival storage by Soroban Protocol 20+ host rules.
   - Escrow funds held in the contract account by the Stellar Asset Contract remain completely secure and intact.
   - Any actor can submit a Stellar `RestoreFootprintOp` transaction to restore the archived persistent key to live state, after which `settle()`, `expire()`, or `resolve_dispute()` executes normally.
   - Archival never causes permanent fund loss.
