# CareFund Submission Readiness Assessment

Date: 2026-10-09
Repository: HealthCarefund/carefund-platform
Baseline: Block 3B Closeout and Block 3C Readiness Audit

## 1. Project Summary

CareFund is an open-source funding coordination protocol designed for subsidized and charitable medical programs. It coordinates agreements between healthcare sponsors, delivery clinics, and independent verification attesters. Funds are held in smart contract escrow on Stellar and disbursed conditionally once care delivery is verified on-chain, returning any unspent escrow surplus back to the donor.

## 2. Problem

Philanthropic donors and community clinics face persistent coordination frictions:
- Upfront grants risk misallocation if procedures are not delivered as agreed.
- Post-care reimbursement forces resource-constrained clinics to finance treatment upfront and wait months for donor audits.
- Administrative auditing often exposes patient identities and medical documentation to donor staff.

## 3. Solution

CareFund resolves these frictions through deterministic contract escrow:
- **Registered Providers**: Healthcare providers maintain an on-chain registration record authorized by the contract administrator.
- **Sponsors**: Funders commit capital into contract escrow upfront, eliminating reimbursement uncertainty for clinics.
- **Authorized Attesters**: Independent verification officers bound to the provider attest to care completion on-chain.
- **Care Agreements**: On-chain state machines enforcing agreed funding amounts, reimbursement amounts, and strict deadlines.
- **Escrow**: Smart contracts custody deposited funds via the Stellar Asset Contract until release criteria are met.
- **Autonomous Settlement**: Once care is attested and the dispute window elapses, settlement transfers funds directly to the provider and refunds any surplus to the sponsor.
- **Dispute Adjudication**: Counterparties can freeze an agreement during the dispute window for administrative resolution.

## 4. Why Stellar and Soroban

CareFund relies on fundamental capabilities of the Stellar network and Soroban:
- **Built-in Token Primitives**: The Stellar Asset Contract (SAC) provides standard token functionality with stroop-level precision.
- **Deterministic Smart Contracts**: Soroban's WebAssembly execution environment guarantees verifiable state transitions and checked math.
- **Cryptographic Authorization**: Soroban `Address::require_auth()` enforces that only designated parties can execute privileged operations.
- **Predictable Execution Costs and Speed**: Fast ledger close times (~5 seconds) and low fees make micro-agreements economically viable.

## 5. Repository Architecture

```
contracts/
  provider-registry/   Soroban contract managing provider and attester authorizations
  care-agreement/      Soroban contract managing agreement escrow, state machine, and settlement

apps/
  api/                 Go REST API, PostgreSQL mirror, transaction prep, and reconciliation
  web/                 Next.js web application with direct client-side wallet signing

packages/
  types/               Shared TypeScript domain primitives and schemas
  sdk/                 Transaction building, simulation, and Freighter wallet integration

docs-site/             mdBook documentation source published to GitHub Pages
evidence/              Verifiable Testnet audit trails, transaction proofs, and safety reviews
```

## 6. Verified Testnet Deployment (Block 3B)

The current live contracts were deployed on Stellar Testnet during Block 3B on 2026-10-09:

| Contract | Address |
|---|---|
| Provider Registry | `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS` |
| Care Agreement | `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` |
| Native XLM Asset Contract | `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC` |

WASM Hashes:
- Provider Registry: `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52` (8,504 bytes)
- Care Agreement: `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5` (17,075 bytes)

## 7. Browser Wallet Verification

Agreement #8 was executed entirely through the browser application with client-side Freighter extension signing on Stellar Testnet:
- **Agreement Proposal**: Initialized by provider `GBWPJKZC...`.
- **Sponsor Funding**: Tx `d2197fb8d30103d126d0c71f7328c12732d57dd92d9156c2534c1417a7ee5e06` (Ledger 5106918).
- **Care Attestation**: Tx `019fc50fc867eb7e1b9a71b775f82cbcb4462b16f2369fa2d1409b4ae17619f3` (Ledger 5107027).
- **Permissionless Outsider Settlement**: Tx `b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa` (Ledger 5107119), signed by outsider `GBN3S...` after dispute window elapsed.
- **Final Agreement State**: Confirmed `Settled` on ledger with reimbursement transferred to provider and surplus refunded to sponsor.
- **Signature Rejection Test**: When a user rejects a signing request in Freighter, the UI transitions to `Failed` with "Wallet declined to sign the transaction", no transaction hash is generated, and no RPC submission is attempted.

## 8. Financial Verification

Core accounting invariants were verified both in unit suites and live on Testnet:
- **Full Escrow Conservation**: Per-agreement escrow liability returns to exactly zero upon settlement or refund.
- **Surplus Refund**: Agreement #1 on Testnet (tx `9ae6dc48...`) locked 10 XLM and settled 8.5 XLM, atomically refunding 1.5 XLM back to the sponsor.
- **Funded Expiry Refund**: Agreement #3 on Testnet (tx `4d1b15c0...`) expired in Funded state without attestation, refunding 100% of deposited escrow (10 XLM) to the sponsor.
- **Dispute Resolution**: Agreement #4 on Testnet (tx `55bfa433...`) was adjudicated by the administrator, executing a full escrow refund to the sponsor.
- **Pooled Escrow Isolation**: Concurrent deposits across Agreements #3 and #4 verified that distinct agreement escrows remain isolated and that operations on one agreement cannot affect balances of another.
- **Permissionless Settlement**: Settlement parameters are immutable on-chain; any caller may execute settlement once the dispute window elapses, preventing counterparty settlement veto.

## 9. Reconciliation and Recovery Verification

During Block 3B live browser verification, Agreement #7 exposed an ingestion defect:
- **Incident**: An agreement creation event failed insertion due to a foreign key constraint referencing `care_agreements(agreement_id)` before the agreement row existed. The reconciler swallowed the error and advanced the ledger cursor.
- **Remediation**:
  1. Implemented on-chain entity decoders (`DecodeAgreement`, `DecodeProvider`, `DecodeAttester`).
  2. Transitioned background reconciliation from event-first to entity-first processing.
  3. Added cursor protection ensuring that the ledger cursor never advances past any failed event or entity.
  4. Added on-demand API fallback to fetch and mirror contract entities from chain if absent in the local mirror.
- **Replay Verification**: The reconciler successfully replayed the ledger range from 5106190 without manual database seeding, restoring Agreement #7 and its events into the database.

## 10. Testing and CI Verification

Automated test suites pass across all layers:
- **Rust Contracts**: 117 tests passing (90 care-agreement, 27 provider-registry).
- **Go API**: 125 test functions passing, including race detection (`go test -race -p 1`).
- **TypeScript Workspace**: 98 unit tests passing (80 SDK, 11 web, 7 fixtures).
- **Web E2E**: 19 Playwright tests passing, including axe-core WCAG 2.1 accessibility audits.
- **Documentation**: mdBook build clean; lychee link check clean (508 OK, 0 errors).
- **CI Workflows**: GitHub Actions workflows green on `main` for Contracts, API, Web, and Docs.

## 11. Public Documentation

Public documentation book is deployed and verified:
https://healthcarefund.github.io/carefund-platform/

## 12. Application Deployment Model

The deployment model for this verification is explicit:
- **Next.js Web Application**: Runs locally at `http://localhost:3000`.
- **Go Backend API**: Runs locally at `http://localhost:8080`.
- **PostgreSQL Database**: Runs in local isolated Docker container on port 5439 (`postgres:18.6-bookworm`).
- **Smart Contracts**: Live on Stellar Testnet.
- **Public Documentation**: Hosted on GitHub Pages.
- **Public Code and Evidence**: Hosted on GitHub.

No Vercel deployment is claimed or used.
No Stellar Mainnet deployment is claimed or used.

## 13. Security Model and Known Limitations

1. **No External Audit**: Contracts and application have not undergone an independent third-party security audit.
2. **No Admin UI for Disputes**: Contract operation `resolve_dispute` requires administrator authorization (`admin.require_auth()`). The web application omits a dispute resolution interface because an administrative authentication layer has not yet been implemented.
3. **Privacy Model**: The system records 32-byte cryptographic commitments rather than plaintext patient data. Deterministic SHA-256 alone does not provide legal anonymization, and CareFund does not claim HIPAA or GDPR compliance. Callers must construct privacy-safe commitments.
4. **API Authentication and Rate Limiting**: The Go API does not enforce bearer tokens or rate limits; cryptographic access control is enforced on-chain by Soroban contracts.
5. **Screen-Reader Audits**: Accessibility is validated via automated axe-core scans; dedicated manual screen-reader reviews have not been conducted.
6. **Testnet Only**: CareFund is a pre-release prototype deployed to Stellar Testnet; it is not approved for production clinical operations.

## 14. Evidence Mapping Matrix

| Claim | Evidence Reference | Ledger / Hash |
|---|---|---|
| Provider Registry Deployment | `evidence/testnet-2026-10-09-block3b.md` Section 1 | `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS` |
| Care Agreement Deployment | `evidence/testnet-2026-10-09-block3b.md` Section 1 | `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` |
| Settlement with Surplus Refund | `evidence/testnet-2026-10-09-block3b.md` Section 3 | Tx `9ae6dc480c558c4f03932fa5a7a7815cfc7a76059344445330364bbaf5a5a0d3` |
| Funded Expiry Full Refund | `evidence/testnet-2026-10-09-block3b.md` Section 4 | Tx `4d1b15c0e181c009f400787e8346df8bc45bc493540203f16c116dff983e26bb` |
| Dispute Adjudication Refund | `evidence/testnet-2026-10-09-block3b.md` Section 5 | Tx `55bfa43389da4b6d080182449fc61a4947ba949b28a2a5ef1f63cb5dfd5c90d5` |
| Live Browser Lifecycle (Agr #8) | `evidence/testnet-2026-10-09-block3b.md` Section 8.3 | Tx `d2197fb8...`, `019fc50f...`, `b7494722...` |
| Signature Rejection Guard | `apps/web/lib/transaction-rejection.test.ts` | Test suite passing; 0 submissions |
| Replay and Cursor Protection | `apps/api/internal/reconcile/runner_test.go` | `TestReconcileEvents_ReplayRecovery` passing |
| On-Demand API Fallback | `apps/api/internal/api/agreements_test.go` | `TestGetAgreement_OnDemandFallback` passing |
| WASM Parity Verification | `target/wasm32v1-none/release/*.wasm` | Exact SHA-256 match (`30924b0d...`, `b543e9a7...`) |
