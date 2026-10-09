# Funding and Settlement Mechanics

CareFund manages on-chain custody and token transfers using Soroban's Stellar Asset Contract (SAC) standard.

## Settlement Asset Architecture

Rather than deploying a custom proprietary token, CareFund contracts bind to a standardized SAC instance configured during contract initialization:

```rust
pub fn initialize(
    env: Env,
    admin: Address,
    provider_registry: Address,
    settlement_asset: Address,
) -> Result<(), AgreementError>
```

On Stellar Testnet, CareFund binds to the native XLM Stellar Asset Contract:
- **Contract Address**: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`
- **Unit Precision**: 7 decimal places (1 XLM = 10,000,000 stroops)

This design is fully compatible with any regulated asset issued on Stellar (such as fiat-backed stablecoins like USDC) simply by specifying the appropriate SAC address during initialization.

---

## Token Flow Lifecycle

### 1. Deposit Escrow (`fund`)
When an agreement is funded:
1. The contract invokes `token::Client::new(&env, &settlement_asset).transfer(&sponsor, &contract_address, &funding_amount)`.
2. The Soroban host verifies `sponsor.require_auth()`.
3. Tokens move directly from the sponsor's account into the contract's instance balance.
4. Contract records state transition from `Requested` to `Funded`.

### 2. Disbursed Settlement (`settle`)
When care is delivered and attested:
1. The contract verifies that `state == AgreementState::CareConfirmed` and the ledger timestamp exceeds the care deadline plus dispute window (`now > care_deadline + dispute_window_secs`).
2. Settle is permissionless: anyone can call it once the dispute window has elapsed, ensuring deterministic finalization without sponsor counterparty veto.
3. The contract invokes `token::Client::new(&env, &settlement_asset).transfer(&contract_address, &provider, &settlement_amount)`.
4. The contract atomically refunds any surplus escrow (`funding_amount - settlement_amount`) to the sponsor via `token::Client::new(&env, &settlement_asset).transfer(&contract_address, &sponsor, &surplus)`.
5. Agreement transitions to `Settled`, and the contract balance for this agreement drops to zero.

### 3. Expiration and Refunds (`expire` / `resolve_dispute`)
If care is not delivered or an admin resolves a dispute:
1. **Unfunded Expiration**: If an agreement remains in `Requested` past `funding_deadline`, anyone can call `expire`. State transitions to `Expired` with zero token transfers.
2. **Funded Expiration**: If an agreement is `Funded` and the ledger timestamp exceeds `care_deadline + dispute_window_secs` without attestation, anyone can call `expire`. The contract transfers the full deposited `funding_amount` back to the `sponsor`, transitioning state to `Expired`.
3. **Dispute Settle**: If an admin resolves a dispute with `DisputeResolution::Settle`, `settlement_amount` transfers to `provider`, surplus (`funding_amount - settlement_amount`) refunds to `sponsor`, and state transitions to `Settled`.
4. **Dispute Refund**: If an admin resolves a dispute with `DisputeResolution::Refund`, the full `funding_amount` transfers back to `sponsor`, and state transitions to `Refunded`.

---

## Balance Invariants

CareFund guarantees strict custody invariants:
- **Full Escrow Conservation**: For every settled or refunded agreement, the sum of provider disbursement and sponsor refund exactly equals the initial deposit ($\Delta \text{Provider} + \Delta \text{Sponsor} = 0$). No surplus tokens remain stranded in contract custody.
- **Zero Custodial Discretion**: Contracts hold tokens purely as deterministic escrow. There is no admin sweep or arbitrary withdrawal mechanism.
- **Dispute Window Integrity**: Settlement is locked while the dispute window remains active, preventing race conditions against dispute filings.
- **Atomic Execution**: Every state transition and accompanying asset transfer executes in a single atomic transaction. If a transfer fails, the entire transaction reverts.
- **Traceability**: All transfers generate standard SAC `transfer` events, queryable via Horizon and Soroban RPC.
