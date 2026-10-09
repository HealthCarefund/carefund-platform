# Why CareFund

Charitable healthcare grants and targeted health subsidies often face a persistent operational trilemma:

1. **Pre-funding Risk**: If sponsors disburse funds in advance, they risk funds being misallocated, unspent, or diverted without care delivery.
2. **Post-reimbursement Burden**: If providers are forced to deliver care upfront and wait months for donor reimbursement, smaller community clinics face catastrophic cash-flow strain.
3. **Privacy Vulnerability**: Centralized grant databases often gather sensitive patient diagnosis and identification details to prove service delivery, creating compliance and security vulnerabilities.

CareFund addresses this trilemma using Stellar smart contracts to construct a trust-minimized, privacy-preserving funding coordinator.

## The Core Solution

Instead of choosing between trust and administrative friction, CareFund introduces programmable conditional funding:

```
[Sponsor] -- (1. Lock Funds in Escrow) --> [Care Agreement Contract]
                                                    |
[Provider] <-- (2. Commit to Care Plan) ------------+
    |
    v (Care Delivered)
[Attester] -- (3. Attest Delivery On-Chain) --------+
                                                    |
[Provider] <-- (4. Settle / Claim Funds) -----------+
```

### 1. Verified Escrow with Conditional Release
Sponsors fund agreements upfront into the `care-agreement` contract escrow. Providers can see that funds are locked and earmarked specifically for their care delivery, reducing non-payment risk. The contract holds those funds until explicit criteria are satisfied.

### 2. Independent Third-Party Attestation
Neither the sponsor nor the provider unilaterally controls fund release. An authorized attester records an on-chain attestation commitment confirming that care was completed according to protocol.

### 3. Clear Time-Gating and Dispute Windows
Agreements define distinct deadlines:
- **Funding Deadline**: If the sponsor fails to deposit funds before this deadline, the agreement can be expired or cancelled.
- **Care Deadline**: The period during which care must be delivered and attested.
- **Dispute Window**: A defined grace period following attestation or care delivery allowing legitimate objections before settlement executes.

### 4. Off-Chain Commitment Architecture
Neither the Stellar blockchain nor CareFund's off-chain PostgreSQL database stores plaintext patient names, medical record numbers, diagnoses, or procedure descriptions. Instead, parties compute 32-byte cryptographic commitments off-chain:
- `patient_ref_commitment`: 32-byte hash referencing the patient record held in the provider's external system.
- `service_commitment`: 32-byte hash describing the agreed clinical protocol.
- `attestation_commitment`: 32-byte hash verifying clinical completion notes.

Callers are responsible for computing privacy-safe commitments. CareFund makes no claim of HIPAA or GDPR compliance.

## Why Stellar and Soroban?

CareFund relies on foundational capabilities provided by the Stellar network:
- **Predictable, Sub-Cent Transaction Costs**: High fee volatility makes micro-reimbursements unviable on many chains. Stellar fees remain negligible.
- **Fast Deterministic Finality**: Ledgers close every 5 seconds, providing rapid state confirmation for patients and clinics.
- **Stellar Asset Contract (SAC)**: Direct integration with native XLM and regulated stablecoins (such as USDC) without requiring custom token wrappers.
- **Explicit Authorization (`require_auth`)**: Soroban's authorization framework guarantees that only the authentic cryptographic signer can invoke role-protected actions.
