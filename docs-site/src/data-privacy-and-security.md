# Data Privacy and Security Model

CareFund is designed around cryptographic authorization and strict privacy compartmentalization.

## Data Privacy Architecture

Traditional healthcare funding systems centralize patient records, diagnostic codes, and clinical documentation to verify billing claims. This creates significant HIPAA, GDPR, and data breach risks.

CareFund replaces plaintext clinical records with **cryptographic commitments**:

```
+-----------------------------------------------------------+
| Provider Internal EMR / Clinic System (Off-Chain Only)   |
|                                                           |
| Patient Name: John Doe                                    |
| Medical Record: MRN-98421                                 |
| Procedure: Cataract Extraction Protocol B                 |
+-----------------------------+-----------------------------+
                              |
                              v SHA-256 Hash Computation
+-----------------------------------------------------------+
| CareFund System (On-Chain Contracts & Off-Chain Database) |
|                                                           |
| patient_ref_commitment: 0x3f8a... (32-byte opaque hash)   |
| service_commitment:     0x9b12... (32-byte opaque hash)   |
+-----------------------------------------------------------+
```

### Invariants:
1. **Zero Protected Health Information (PHI)**: Neither the Soroban ledger nor the PostgreSQL database ever stores plaintext patient names, clinical notes, procedure details, or medical record numbers.
2. **One-Way Commitments**: Commitments are 32-byte SHA-256 hashes computed locally by clinics within their own internal systems before submitting agreements.
3. **Restricted Logging**: The Go API logging middleware (`withRequestLogging`) explicitly logs only HTTP method, path, response status, and duration. It never captures request bodies, headers, or query parameters.

---

## On-Chain Authorization Model

Soroban smart contracts enforce role-based access control using `Address::require_auth()`:
- `admin.require_auth()`: Restricts provider onboarding, suspension, revocation, and dispute resolution.
- `sponsor.require_auth()`: Enforces that only the designated funder can deposit escrow into the contract.
- `attester.require_auth()`: Enforces that care confirmation can only be signed by the designated attester.
- `provider.require_auth()`: Restricts un-funded agreement cancellations.

---

## Custody and Private Key Boundary

- **No Server Keys**: The Go API backend (`apps/api`) and the Next.js web application (`apps/web`) never receive, store, or transmit private keys.
- **Client Signatures**: All transaction signing occurs in the user's browser wallet extension (Freighter).
- **Direct RPC Submission**: Signed envelopes are submitted directly from the client to the Soroban RPC endpoint without being proxied through backend servers.

---

## Storage Durability and State Archival

CareFund contracts rely on Soroban Protocol 20+ persistent storage for agreement records and instance storage for contract configuration.
- **Proactive TTL Extension**: Every read or write contract operation bumps the Time To Live (TTL) of the agreement entry and the contract instance to ~30 days (518,400 ledgers).
- **Archival Protection**: If an agreement remains dormant beyond 30 days without interaction, Soroban moves the persistent entry to cold archival storage. Escrowed tokens remain in the contract's Stellar Asset Contract account and are never destroyed or lost.
- **Restoration Workflow**: Anyone can submit a standard Stellar transaction containing `RestoreFootprintOp` specifying the archived ledger key. Once restored, all settlement, expiry, and dispute adjudication flows proceed normally.

---

## Dependency Vulnerability Scans

As part of repository hygiene, automated dependency scanners are executed periodically:
- `pnpm audit`: 0 known vulnerabilities.
- `cargo audit`: 0 known vulnerabilities (1 unmaintained crate warning for transitive macro dependency `paste`).
- `govulncheck`: 0 vulnerabilities in called Go packages.

---

## Security Audit Status

**CareFund has not undergone an independent professional third-party security audit.**

All testing and verification performed to date consists of automated unit and integration suites, local failure injections, and developer-executed Testnet lifecycle runs. CareFund is not currently licensed or approved for production clinical or mainnet financial operations.
