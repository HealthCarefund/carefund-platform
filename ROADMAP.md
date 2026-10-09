# CareFund Project Roadmap

This document outlines the current verified status of CareFund, mandatory pre-submission gates, and reserved extension areas for future post-approval contributor work.

## 1. Current Verified MVP and Operational Baseline

CareFund coordinates care agreements and conditional funding on Stellar using Soroban smart contracts, a Go API backend, and a Next.js web application.

### Verified Deliverables
- **Smart Contracts**: `contracts/provider-registry` and `contracts/care-agreement` implementing provider management, attester authorizations, agreement lifecycle (create, fund, attest_care, settle, dispute, expire, cancel), and cross-contract status validation.
- **Go Backend API**: Off-chain metadata mirror in PostgreSQL, unsigned transaction preparation, idempotency tracking, and chain reconciliation loop.
- **TypeScript Packages**: `@carefund/types` for domain primitives, `@carefund/sdk` for simulation, signing, submission, and transaction status polling.
- **Next.js Web Application**: Provider, sponsor, and attester workflows with client-side Freighter signing.
- **Historical Testnet Evidence**: Real lifecycle execution on Stellar Testnet on 2026-09-27 (creation, funding with native XLM SAC, care attestation, time-gated settlement, and simulation-rejected failure paths).

### Existing Known Limitations
- **Dispute Resolution UI**: The contract operation `resolve_dispute` requires admin authentication (`admin.require_auth()`). The application currently lacks an admin authentication system, so no UI exists for dispute resolution to prevent unauthorized access.
- **Agreement Intents Listing**: Sponsor intents (`/sponsor/agreements/new`) create off-chain records, but there is no provider discovery endpoint for pending intents. Providers currently receive agreement parameters out-of-band.
- **Browser Wallet Testnet Verification**: Live Testnet transactions in Block 1 were signed using Stellar CLI local key identities. End-to-end execution through a live browser Freighter extension has not yet been independently verified.
- **Automated Accessibility Testing**: Scans are automated via axe-core in Playwright. Dedicated manual keyboard and screen-reader audits have not yet been performed.

---

## 2. Pre-Submission Verification Gates

These gates must be satisfied before final grant or project submission:

1. **Care Agreement Contract Redeployment**:
   - The historical Testnet deployment (`CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS`) precedes the dispute-window overflow safety fix (commit `7fdcade`).
   - The updated WASM must be built, deployed to Testnet, and a fresh lifecycle verified.
2. **Configuration and Evidence Synchronization**:
   - Synchronize new contract IDs, WASM hashes, and transaction proofs across `apps/web/.env.local`, `apps/api/.env`, `evidence/`, and documentation.
3. **Live Browser Wallet Sign-off**:
   - Verify transaction signing in a live browser using the Freighter extension on Testnet for create, fund, attest, and settle flows.
4. **Documentation Publication Verification**:
   - Verify that the static mdBook documentation site deploys cleanly via GitHub Pages to `https://healthcarefund.github.io/carefund-platform/` and all internal links resolve.
5. **Release Readiness and Tagging**:
   - Prepare version checklist and tag an official v0.1.0 release only after all above verification gates pass.

---

## 3. Reserved Post-Approval Contributor Runway

These extension areas are reserved for future open source contributors after project approval. They are intentionally deferred to maintain a focused, truthful submission:

### Provider and Attester Operations
- Provider profile metadata management with off-chain verification proofs.
- Multi-attester threshold rules for high-value care agreements.
- Attester reputation and historical performance metrics.

### Observability and Reconciliation
- Prometheus metrics endpoint for the Go API and reconciliation worker.
- Webhook notification service for on-chain agreement state transitions.
- Dead-letter queue and operator alerts for persistent reconciliation lag.

### Security and Identity Hardening
- Role-based admin authentication and session management for `resolve_dispute`.
- Passkey / WebAuthn signer support alongside Freighter wallet.
- Independent external smart contract audit.

### Developer and Contributor Ergonomics
- Expanded SDK client libraries with auto-retrying RPC helpers.
- Local mock Soroban RPC container integration for standalone offline development.
- Interactive tutorial and CLI playground for agreement simulation.
