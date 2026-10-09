# Changelog

All notable changes to the CareFund platform are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - Unreleased

### Overview
CareFund 0.1.0 is an open-source prototype coordinating conditional healthcare funding on Stellar Testnet. It provides smart contract escrow, independent verification attestation, and deterministic on-chain settlement, backed by a Go API metadata mirror and a Next.js web application.

This release represents an MVP verified on Stellar Testnet. It is not approved for clinical production operations and has not undergone an independent third-party security audit.

### Added

#### Smart Contracts (`contracts/`)
- `provider-registry`: On-chain registry tracking authorized providers and designated attesters. Supports administrator-controlled registration, suspension, reinstatement, and revocation with cross-contract status checks.
- `care-agreement`: Core agreement state machine (`Requested`, `Funded`, `CareConfirmed`, `Settled`, `Disputed`, `Cancelled`, `Expired`) managing escrow deposits via the Stellar Asset Contract (SAC).
- Strict funding deadline boundary timing enforcing that expiration only occurs strictly after funding deadline elapsed.
- Checked arithmetic on all escrow balance adjustments and token transfers.
- Full escrow conservation returning per-agreement liability to zero upon settlement or refund.
- Automated surplus refund returning excess escrow (funding amount minus agreed settlement amount) to the sponsor upon settlement.
- Funded expiry refund returning 100% of escrowed tokens to the sponsor if care is not attested prior to expiration.
- Dispute adjudication supporting administrative resolution to settlement or refund with exact escrow distribution.
- One-argument permissionless settlement (`settle({ agreement_id })`) callable by any party once the dispute window elapses.

#### Go Backend API (`apps/api/`)
- REST service implementing agreement queries, provider listings, and transaction preparation.
- PostgreSQL metadata mirror storing off-chain agreement intents, idempotency tokens, and an indexed projection of contract ledger state.
- Background chain reconciler indexing contract events and transaction hashes with entity-first ingestion and cursor protection against partial failures.
- On-demand RPC fallback (`fetchOnChainAgreement`, `ensureProvider`) resolving chain entities when local mirror records are missing.
- Structured logging omitting request and response bodies to protect privacy boundaries.

#### TypeScript SDK and Types (`packages/`)
- `@carefund/types`: Branded primitives (`StellarAddress`, `CommitmentHash`, `AgreementId`) and contract event schemas.
- `@carefund/sdk`: Transaction builder, simulation pipeline, Freighter wallet integration adapter, and transaction status confirmation poller.
- Type-safe contract client bindings generated directly from compiled contract WASM artifacts.

#### Web Application (`apps/web/`)
- Next.js application providing dedicated workflows for healthcare providers, sponsors, and attesters.
- Pure client-side wallet signing using the Freighter browser extension without exposing signing keys to backend servers.
- Agreement proposal, funding, care attestation, and settlement transaction flows with transaction status tracking.
- Accessible UI components validated against WCAG 2.1 A/AA standards via automated axe-core scans.

#### Documentation and Governance (`docs-site/`, `.github/`)
- mdBook documentation site deployed to GitHub Pages covering architecture, financial invariants, protocol lifecycle, SDK usage, and troubleshooting.
- Comprehensive security policy (`SECURITY.md`) and testing guide.
- Continuous integration workflows covering Rust contract tests and lints, Go vet, build, test, and race detection, TypeScript typechecking and unit tests, and Next.js Playwright E2E suites.
- Complete on-chain verification record in `evidence/testnet-2026-10-09-block3b.md` and `evidence/index.md`.

### Known Limitations
- No administrative web UI exists for `resolve_dispute` due to the absence of an administrative authentication and session layer.
- Sponsor agreement intents recorded off-chain are not listed in a dedicated provider discovery inbox.
- The Go API validates wallet roles for convenience but does not enforce bearer authentication or per-caller rate limiting. Cryptographic access control is enforced on-chain via contract `require_auth()`.
- Accessibility testing is automated; dedicated manual screen-reader audits have not been completed.
- Commitments are 32-byte cryptographic hashes; deterministic SHA-256 does not constitute legal anonymization, and no HIPAA or GDPR compliance is claimed.
- No external third-party security audit has been conducted.
