# Data Privacy and Security Model

CareFund is designed around cryptographic authorization and strict privacy compartmentalization.

## Data Privacy Architecture

Traditional healthcare funding systems often centralize patient records, diagnostic codes, and clinical documentation to verify billing claims.

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

### Commitments and Boundaries:
1. **No Raw Clinical Data**: Plaintext patient names, diagnostic notes, procedure descriptions, and medical record numbers must never be submitted to CareFund. Contracts and databases store only 32-byte commitment hashes.
2. **Caller Privacy Responsibility**: CareFund does not claim that deterministic SHA-256 hashing alone provides legal anonymization. Predictable or low-entropy identifiers remain vulnerable to correlation or dictionary attacks if hashed naively without external salting or entropy management. Callers must construct privacy-safe commitments before submitting them. CareFund makes no claim of HIPAA or GDPR compliance.
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

- **No Private Key Custody**: Neither the Go API backend (`apps/api`) nor the Next.js web application (`apps/web`) receives, stores, or transmits private keys.
- **Smart Contract Escrow Custody**: Agreement funds are held temporarily in escrow by the Soroban care-agreement contract until release, refund, or dispute resolution conditions are satisfied on-chain.
- **Client Signatures**: All transaction signing occurs in the user's browser wallet extension (Freighter).
- **Direct RPC Submission**: Signed envelopes are submitted directly from the client to the Soroban RPC endpoint without being proxied through backend servers.

---

## Storage Durability and State Archival

CareFund contracts rely on Soroban Protocol 20+ persistent storage for agreement records and instance storage for contract configuration.
- **Proactive TTL Extension**: Every read or write contract operation bumps the Time To Live (TTL) of the agreement entry and the contract instance to ~30 days (518,400 ledgers).
- **Archival Protection**: If an agreement remains dormant beyond 30 days without interaction, Soroban moves the persistent entry to cold archival storage. Escrowed tokens remain in the contract's Stellar Asset Contract account and are never destroyed or lost.
- **Restoration Workflow**: Under current Soroban protocols (Protocol 23+), transaction simulation can identify archived entries and include restoration footprints automatically within `InvokeHostFunctionOp`. Standalone `RestoreFootprintOp` transactions remain available as an operator or fallback workflow.

---

## Dependency Vulnerability Scans

As part of repository hygiene, automated dependency scanners are executed periodically:
- `cargo audit`: 0 vulnerabilities (1 allowed unmaintained crate warning for transitive macro dependency `paste`).
- `govulncheck`: 0 critical vulnerabilities; 10 non-critical advisories in Go 1.27.1 standard library / x:text.
- `pnpm audit` and `pnpm audit --prod`: 0 critical vulnerabilities; 9 advisories (1 low, 4 moderate, 4 high) in Next.js versions prior to 16.3.8. The application is run locally and is not exposed to public hosting.

---

## Security Audit Status

**CareFund has not undergone an independent professional third-party security audit.**

All testing and verification performed to date consists of automated unit and integration suites, local failure injections, and developer-executed Testnet lifecycle runs (documented in `evidence/testnet-2026-10-09-block3b.md` and historical `evidence/testnet-2026-09-27.md`). CareFund is not currently licensed or approved for production clinical or mainnet financial operations.
