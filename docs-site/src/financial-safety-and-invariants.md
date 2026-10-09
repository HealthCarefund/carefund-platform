# Financial Safety and Invariants

CareFund enforces mathematical escrow conservation, timing boundaries, deterministic settlement, and storage resilience across all smart contract operations.

## Core Financial Invariants

The protocol guarantees five fundamental financial invariants:

### 1. Escrow Conservation and Surplus Refund
Every deposited unit of currency is accounted for upon terminal state transition:
- When an agreement transitions to `Settled` via `settle` or `resolve_dispute(Settle)`, the provider receives the agreed `settlement_amount` ($S$).
- The sponsor atomically receives a refund of the escrow surplus ($F - S$, where $F$ is `funding_amount`).
- The per-agreement escrow liability drops to exactly zero. No excess tokens remain stranded in contract custody for that agreement.

$$\Delta \text{Provider} + \Delta \text{Sponsor} = F \quad (\text{full conservation of deposited escrow})$$

#### Per-Agreement Conservation vs Aggregate Contract Balance
It is essential to distinguish per-agreement escrow accounting from the aggregate token balance of the contract address:
- **Per-Agreement Escrow Accounting**: Each agreement accounts for its own deposit $F$. Inflows equal outflows for every agreement independently. When an agreement finalizes, its specific liability returns to zero.
- **Aggregate Contract Token Balance**: In a multi-agreement deployment, the contract address holds a pooled token balance equal to the sum of all currently active funded agreements:

$$\text{Contract Balance} = \sum_{k \in \text{active}} F_k$$

When an individual agreement settles or expires, the aggregate contract balance decreases by exactly that agreement's $F$, returning to zero if and only if all agreements across the contract have finalized.

#### Arithmetic Safety and Validated Invariants
The surplus calculation $F - S$ is defended at multiple layers:
1. **Creation Invariant**: At agreement creation, `create_agreement` strictly validates `0 < settlement_amount <= funding_amount` (`if settlement_amount > funding_amount { return Err(AgreementError::InvalidAmount); }`). This establishes the mathematical invariant that $F - S \ge 0$.
2. **Defensive Checked Arithmetic**: During execution in `settle()` and `resolve_dispute(Settle)`, surplus is calculated via checked subtraction: `agreement.funding_amount.checked_sub(agreement.settlement_amount).ok_or(AgreementError::ArithmeticOverflow)?`. Any unexpected invariant violation halts execution rather than causing arithmetic underflow.

### 2. Dispute Window Non-Overlap
Settlement and dispute windows are mutually exclusive:
- A dispute may be opened only while the dispute window is active: `care_deadline < now <= care_deadline + dispute_window_secs`.
- Settlement may execute only after the dispute window has elapsed: `now > care_deadline + dispute_window_secs`.
- Settlement is strictly prohibited while the dispute window is active (`DisputeWindowActive`), preventing settlement race conditions that could prematurely close the dispute period.

### 3. Permissionless Settlement Finalization
Once care has been attested and the dispute window has elapsed:
- Settlement parameters ($S$, $F$, provider, sponsor) are immutable on-chain.
- Any caller (provider, sponsor, or third-party relayer) can invoke `settle(agreement_id)`.
- The sponsor cannot veto or block settlement after clinical care has been attested and the objection period has elapsed.
- Funds route deterministically to the provider (settlement amount) and sponsor (surplus refund).

### 4. Escrow Refund on Unverified Expiration and Boundary Precision
Agreements that lapse without completion are safely unwound:
- An agreement in `Funded` state that reaches `care_deadline + dispute_window_secs` without attestation can be expired via `expire(agreement_id)`.
- Upon expiration, the contract atomically refunds the full deposited `funding_amount` ($F$) back to the sponsor.
- An agreement in `Requested` state past `funding_deadline` can be expired cleanly with zero token transfers.
- **Funding Deadline Boundary Precision**: Funding is permitted while `now <= funding_deadline`. Expiration of a `Requested` agreement is strictly permitted only after the funding deadline has passed (`now > funding_deadline`). At the exact timestamp `now == funding_deadline`, the sponsor retains the right to fund, and calls to `expire` are rejected with `FundingDeadlineNotReached`.
- An agreement in `CareConfirmed` state cannot be expired (`InvalidState`), protecting attested clinical delivery.

### 5. Dispute Resolution Invariants
When an administrator adjudicates a dispute via `resolve_dispute`:
- `DisputeResolution::Settle`: Disburses `settlement_amount` to provider, refunds `funding_amount - settlement_amount` surplus to sponsor via checked subtraction, and transitions to `Settled`.
- `DisputeResolution::Refund`: Disburses full `funding_amount` back to sponsor and transitions to `Refunded`.
- `DisputeResolution::Resume`: Restores agreement to its pre-dispute state (`Funded` or `CareConfirmed`).

---

## State Transition Truth Table

| From State | Action | Guard Conditions | Token Transfer | To State |
|---|---|---|---|---|
| None | `create_agreement` | Valid deadlines and amounts ($F \ge S > 0$) | 0 | `Requested` |
| `Requested` | `fund` | `now <= funding_deadline`, sponsor auth | $F$ from Sponsor to Contract | `Funded` |
| `Requested` | `cancel` | Provider auth | 0 | `Cancelled` |
| `Requested` | `expire` | `now > funding_deadline` | 0 | `Expired` |
| `Funded` | `attest_care` | `now <= care_deadline`, attester auth | 0 | `CareConfirmed` |
| `Funded` | `open_dispute` | `care_deadline < now <= care_deadline + dispute_window_secs`, sponsor or provider auth | 0 | `Disputed` |
| `Funded` | `expire` | `now > care_deadline + dispute_window_secs` | Full $F$ from Contract to Sponsor | `Expired` |
| `CareConfirmed` | `open_dispute` | `care_deadline < now <= care_deadline + dispute_window_secs`, sponsor or provider auth | 0 | `Disputed` |
| `CareConfirmed` | `settle` | `now > care_deadline + dispute_window_secs`, callable by anyone | $S$ to Provider, $F - S$ to Sponsor | `Settled` |
| `CareConfirmed` | `expire` | Any time | Disallowed (returns `InvalidState`) | Rejected |
| `Disputed` | `resolve_dispute(Resume)` | Admin auth | 0 | Pre-dispute state (`Funded` or `CareConfirmed`) |
| `Disputed` | `resolve_dispute(Settle)` | Admin auth | $S$ to Provider, $F - S$ to Sponsor | `Settled` |
| `Disputed` | `resolve_dispute(Refund)` | Admin auth | Full $F$ to Sponsor | `Refunded` |

---

## Soroban Storage Lifetimes and State Archival Restoration

CareFund utilizes Soroban storage tiers designed for security, predictable rent costs, and data preservation:

### Storage Architecture
- **Instance Storage**: Stores global contract configuration (`Admin`, `ProviderRegistry`, `SettlementAsset`, `NextAgreementId`).
- **Persistent Storage**: Stores individual agreement records (`DataKey::Agreement(u64)`).
- **Temporary Storage**: Not used for financial or agreement records.

### TTL Configuration and Proactive Extension
- `INSTANCE_TTL_THRESHOLD` / `PERSISTENT_TTL_THRESHOLD`: 120,960 ledgers (~7 days at 5 seconds per ledger).
- `INSTANCE_TTL_EXTEND_TO` / `PERSISTENT_TTL_EXTEND_TO`: 518,400 ledgers (~30 days at 5 seconds per ledger).
- Every state-reading or state-modifying entry point (`create_agreement`, `get_agreement`, `fund`, `attest_care`, `open_dispute`, `settle`, `resolve_dispute`, `expire`) automatically invokes `extend_ttl` on both the agreement persistent key and the contract instance.

### State Archival and Restoration Model
Under Soroban Protocol 20+ State Archival (CAP-0046):
1. **No Permanent Fund Loss**: If an agreement remains inactive for longer than its 30-day TTL (for instance, during a prolonged dispute or long-horizon care deadline), its persistent storage entry enters the `Archived` state. Deposited tokens held in the Stellar Asset Contract (SAC) escrow remain completely secure and unaffected.
2. **Restoration via `RestoreFootprintOp`**: Archived persistent entries can be restored at any time. When a transaction simulation indicates that an entry is archived, the client or keeper submits a standard Stellar transaction containing `RestoreFootprintOp` with the archived ledger key.
3. **Resumption of Settlement**: Once restored to live persistent storage, contract execution (`settle`, `expire`, or `resolve_dispute`) proceeds immediately and finalizes the financial disbursement as specified.
