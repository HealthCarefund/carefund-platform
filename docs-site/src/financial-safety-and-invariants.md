# Financial Safety and Invariants

CareFund enforces mathematical escrow conservation, timing boundaries, and deterministic settlement across all smart contract operations.

## Core Financial Invariants

The protocol guarantees five fundamental financial invariants:

### 1. Escrow Conservation and Surplus Refund
Every deposited unit of currency must be accounted for upon terminal state transition:
- When an agreement transitions to `Settled` via `settle` or `resolve_dispute(Settle)`, the provider receives the agreed `settlement_amount` ($S$).
- The sponsor atomically receives a refund of the escrow surplus ($F - S$, where $F$ is `funding_amount`).
- The contract balance for the agreement returns to exactly zero.
- No excess tokens remain stranded in contract custody.

$$\Delta \text{Provider} + \Delta \text{Sponsor} = 0 \quad (\text{relative to original deposit})$$

### 2. Dispute Window Non-Overlap
Settlement and dispute windows are mutually exclusive:
- A dispute may be opened only while the dispute window is active: `care_deadline < now <= care_deadline + dispute_window_secs`.
- Settlement may execute only after the dispute window has elapsed: `now > care_deadline + dispute_window_secs`.
- Settlement is strictly prohibited while the dispute window is active, preventing settlement race conditions that could prematurely close the dispute period.

### 3. Permissionless Settlement Finalization
Once care has been attested and the dispute window has elapsed:
- Settlement parameters ($S$, $F$, provider, sponsor) are immutable on-chain.
- Any caller (provider, sponsor, or third-party relayer) can invoke `settle(agreement_id)`.
- The sponsor cannot veto or block settlement after clinical care has been attested and the objection period has elapsed.
- Funds route deterministically to the provider (settlement amount) and sponsor (surplus refund).

### 4. Escrow Refund on Unverified Expiration
Agreements that lapse without completion are safely unwound:
- An agreement in `Funded` state that reaches `care_deadline + dispute_window_secs` without attestation can be expired via `expire(agreement_id)`.
- Upon expiration, the contract atomically refunds the full deposited `funding_amount` ($F$) back to the sponsor.
- An agreement in `Requested` state past `funding_deadline` can be expired cleanly with zero token transfers.
- An agreement in `CareConfirmed` state cannot be expired (`InvalidState`), protecting attested clinical delivery.

### 5. Dispute Resolution Invariants
When an administrator adjudicates a dispute via `resolve_dispute`:
- `DisputeResolution::Settle`: Disburses `settlement_amount` to provider, refunds `funding_amount - settlement_amount` surplus to sponsor, and transitions to `Settled`.
- `DisputeResolution::Refund`: Disburses full `funding_amount` back to sponsor and transitions to `Refunded`.
- `DisputeResolution::Resume`: Restores agreement to its pre-dispute state (`Funded` or `CareConfirmed`).

---

## State Transition Truth Table

| From State | Action | Guard Conditions | Token Transfer | To State |
|---|---|---|---|---|
| None | `create_agreement` | Valid deadlines and amounts ($F \ge S > 0$) | 0 | `Requested` |
| `Requested` | `fund` | `now <= funding_deadline`, sponsor auth | $F$ from Sponsor to Contract | `Funded` |
| `Requested` | `cancel` | Provider auth | 0 | `Cancelled` |
| `Requested` | `expire` | `now >= funding_deadline` | 0 | `Expired` |
| `Funded` | `attest_care` | `now <= care_deadline`, attester auth | 0 | `CareConfirmed` |
| `Funded` | `open_dispute` | `care_deadline < now <= care_deadline + dispute_window_secs`, sponsor or provider auth | 0 | `Disputed` |
| `Funded` | `expire` | `now > care_deadline + dispute_window_secs` | Full $F$ from Contract to Sponsor | `Expired` |
| `CareConfirmed` | `open_dispute` | `care_deadline < now <= care_deadline + dispute_window_secs`, sponsor or provider auth | 0 | `Disputed` |
| `CareConfirmed` | `settle` | `now > care_deadline + dispute_window_secs`, callable by anyone | $S$ to Provider, $F - S$ to Sponsor | `Settled` |
| `CareConfirmed` | `expire` | Any time | Disallowed (returns `InvalidState`) | Rejected |
| `Disputed` | `resolve_dispute(Resume)` | Admin auth | 0 | Pre-dispute state (`Funded` or `CareConfirmed`) |
| `Disputed` | `resolve_dispute(Settle)` | Admin auth | $S$ to Provider, $F - S$ to Sponsor | `Settled` |
| `Disputed` | `resolve_dispute(Refund)` | Admin auth | Full $F$ to Sponsor | `Refunded` |
