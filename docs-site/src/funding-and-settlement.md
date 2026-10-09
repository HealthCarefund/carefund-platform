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
1. The contract verifies that `state == AgreementState::CareConfirmed` and the ledger timestamp exceeds `care_deadline`.
2. The contract verifies `sponsor.require_auth()`.
3. The contract invokes `token::Client::new(&env, &settlement_asset).transfer(&contract_address, &provider, &settlement_amount)`.
4. Settlement tokens arrive in the provider's wallet balance.
5. If `funding_amount > settlement_amount`, the remaining balance is retained in the contract or handled according to protocol terms.

### 3. Sponsor Refund (`refund` / `expire`)
If care is not delivered before deadlines or an admin resolves a dispute in favor of refund:
1. The contract verifies that conditions permit refund (e.g., `care_deadline` passed with no attestation).
2. The contract transfers the full deposited `funding_amount` back to the `sponsor`.
3. The agreement state is marked terminal (`Refunded` or `Expired`).

---

## Balance Invariants

CareFund guarantees strict custody invariants:
- **Zero Custodial Discretion**: Contracts hold tokens purely as a deterministic escrow. There is no admin "sweep" or general withdrawal function.
- **Atomic Execution**: Every state transition and accompanying asset transfer executes in a single atomic transaction. If a transfer fails (e.g., insufficient sponsor balance), the state mutation reverts entirely.
- **Traceability**: All transfers generate standard SAC `transfer` events, queryable via Horizon and Soroban RPC.
