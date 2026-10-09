# Project Roadmap

This roadmap outlines the verified baseline of CareFund, mandatory pre-submission gates, and reserved post-approval contributor extensions.

## 1. Current Verified MVP

CareFund has implemented and tested:
- **Soroban Contracts**: `provider-registry` and `care-agreement` with role-based authorization, time-gated deadlines, and multi-status lifecycle management.
- **Go Backend API**: PostgreSQL workflow mirror, unsigned transaction assembly, idempotency handling, and chain event reconciliation.
- **TypeScript Packages**: `@carefund/types` primitives and `@carefund/sdk` transaction pipeline.
- **Next.js Web Portal**: Provider, sponsor, and attester interfaces.
- **Testnet Verification**: Historical lifecycle verified on Stellar Testnet on 2026-09-27.

---

## 2. Pre-Submission Verification Gates

Before submitting CareFund for formal ecosystem review or production consideration, the following technical gates must be closed:

1. **WASM Redeployment and Lifecycle Re-Verification**:
   - Recompile `care-agreement` including the dispute-window overflow fix (commit `7fdcade`).
   - Deploy new WASM to Stellar Testnet.
   - Execute a complete creation, funding, attestation, and settlement lifecycle.
2. **Configuration Synchronization**:
   - Update contract IDs and transaction hashes in `apps/web/.env.local`, `apps/api/.env`, `evidence/index.md`, and documentation.
3. **Live Browser Wallet Sign-off** (COMPLETED in Block 3B closeout):
   - Confirmed transaction signing in a live browser with Freighter extension on Testnet for Agreement #8 full lifecycle (funding, attestation, outsider settlement) and signature rejection guard path.
4. **Documentation Publication Sign-off**:
   - Confirm GitHub Pages deployment is active and verified at `https://healthcarefund.github.io/carefund-platform/`.
5. **Release Tagging**:
   - Execute release readiness checklist and create the official v0.1.0 tag.

---

## 3. Reserved Post-Approval Contributor Runway

The following roadmap items are intentionally deferred for open-source contributors after initial project approval:

- **Provider Operations**: Clinic profile metadata and multi-attester threshold rules.
- **Observability**: Prometheus metrics for Go API and webhook event notifications for agreement state transitions.
- **Administrative Security**: Role-based authentication and secure session management for `resolve_dispute`.
- **Alternative Signers**: Passkey and WebAuthn signing adapters.
- **Developer Tools**: Standalone local mock Soroban RPC Docker integration and interactive CLI playground.
