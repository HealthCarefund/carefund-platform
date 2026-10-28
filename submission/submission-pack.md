# CareFund Submission Pack

## Project

CareFund

- **Repository**: [https://github.com/HealthCarefund/carefund-platform](https://github.com/HealthCarefund/carefund-platform)
- **Documentation**: [https://healthcarefund.github.io/carefund-platform/](https://healthcarefund.github.io/carefund-platform/)
- **Release**: [https://github.com/HealthCarefund/carefund-platform/releases/tag/v0.1.0](https://github.com/HealthCarefund/carefund-platform/releases/tag/v0.1.0)
- **Maintainer**: [ZeePearl56](https://github.com/ZeePearl56)
- **GitHub**: [https://github.com/ZeePearl56](https://github.com/ZeePearl56)
- **Contact**: `l2ejourney@gmail.com`

## One-line description

CareFund is an open-source conditional funding coordination protocol that holds sponsor capital in Soroban smart-contract escrow on Stellar and releases a capped reimbursement to a registered healthcare provider after an authorized attester records a confirmation and a dispute window passes. The confirmation is an authorized on-chain statement, not independent proof that care took place.

## Problem

Charitable and subsidized healthcare programs face severe coordination frictions: donors advancing upfront grants risk capital misallocation if procedures are not delivered, while clinics delivering treatment upfront bear debilitating cash-flow delays waiting for retrospective donor audits. Furthermore, conventional auditing often exposes sensitive patient identities and clinical records to non-medical donor personnel.

## Solution

CareFund resolves these frictions through deterministic contract escrow and cryptographic proof commitments:

- **Registered Provider**: Healthcare clinics are registered on-chain with administrative status tracking in `provider-registry`.
- **Sponsor**: Funders lock capital in contract escrow upfront, eliminating reimbursement uncertainty for clinics.
- **Authorized Attester**: An address the administrator registered and bound to a specific provider. It records an on-chain attestation of procedure completion. The registry records an administrative decision, not verified accreditation, and the contract does not enforce that the attester is independent of the provider or sponsor.
- **Care Agreement**: State machines defining maximum funding, reimbursement caps, care completion deadlines, and dispute windows.
- **Soroban Escrow**: Smart contracts custody deposited tokens via the Stellar Asset Contract until release conditions are satisfied.
- **Attestation**: The authorized attester submits a 32-byte commitment. The contract records it and the signer; it does not verify what the commitment refers to or that the care happened.
- **Settlement**: Once care is attested and the dispute window (which opens at `care_deadline` and closes `dispute_window_secs` later) has elapsed, settlement transfers funds directly to the provider and atomically refunds any unspent escrow surplus back to the sponsor.
- **Disputes and Refunds**: Counterparties can freeze an agreement during the dispute window for administrative adjudication; expired funded agreements execute full escrow refunds to sponsors.

## Why Stellar

CareFund relies on fundamental, load-bearing architecture features of the Stellar network and Soroban runtime:

- **Soroban Authorization**: Native `Address::require_auth()` guarantees cryptographically that only designated counterparties can create agreements, deposit funds, submit attestations, or open disputes.
- **Deterministic Smart-Contract State**: Soroban's WebAssembly execution environment guarantees verifiable state transitions, checked integer arithmetic, and predictable execution.
- **Token Escrow via Stellar Asset Contract (SAC)**: Standard stroop-precision balance transfers and custody without custom token wrappers or custodial intermediaries. The demonstrated lifecycle used native XLM; no stablecoin lifecycle has been verified.
- **Externally Verifiable Transactions**: Every lifecycle step yields immutable, transparent transaction hashes on the public ledger.
- **Permissionless Final Settlement**: Once the dispute window passes, any external caller can execute settlement, preventing counterparty settlement veto or payout delays.

## Architecture

- `contracts/provider-registry`: CareFund-owned Soroban contract managing provider directory and authorized attesters.
- `contracts/care-agreement`: CareFund-owned Soroban contract governing agreement state machines, escrow custody, surplus refunds, and settlement.
- Native XLM Stellar Asset Contract: network-provided token contract that care-agreement calls. It is not CareFund code.
- Full system description: [ARCHITECTURE.md](../ARCHITECTURE.md).
- `apps/api`: Go REST backend providing PostgreSQL off-chain metadata mirror, unsigned transaction preparation, and chain reconciliation loop.
- `apps/web`: Next.js web application with direct client-side wallet signing across provider, sponsor, and attester roles.
- `packages/sdk`: TypeScript SDK for transaction simulation, building, signing adapter, and Soroban RPC interaction.
- `packages/types`: Shared domain primitives and schema definitions.
- `PostgreSQL`: Isolated relational database mirroring on-chain state and off-chain intents.
- `Freighter`: Non-custodial browser wallet extension for user signing on Stellar Testnet.
- `GitHub Pages`: Static documentation deployment built via mdBook.

## Testnet Contracts

- **Provider Registry**: `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`
- **Care Agreement**: `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O`
- **Native XLM SAC**: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`

## Strongest Live Evidence

### Agreement #8 Complete Browser Lifecycle

Agreement #8 was executed end-to-end on Stellar Testnet via the Next.js web application using the Freighter browser extension:

- **Sponsor Funding**: Tx `d2197fb8d30103d126d0c71f7328c12732d57dd92d9156c2534c1417a7ee5e06` (Ledger 5106918), 10,000,000 stroops (1 XLM) deposited.
- **Attestation**: Tx `019fc50fc867eb7e1b9a71b775f82cbcb4462b16f2369fa2d1409b4ae17619f3` (Ledger 5107027), attested by authorized attester.
- **Permissionless Outsider Settlement**: Tx `b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa` (Ledger 5107119), invoked by external third-party caller `GDBPGGUET2LIJEE3E7SB6GIUFSU4IW3OPDRAPCO56I6E63NH2H77CYSW`. 8,000,000 stroops disbursed to provider; 2,000,000 stroops surplus refunded to sponsor.
- **Final State**: Confirmed `Settled` on ledger with zero net contract liability.

### Additional Verified Accounting and Recovery Scenarios

- **Surplus Refund**: Agreement #1 (tx `9ae6dc480e876a32a18e5b0d12ce5a3187010c01ce319d0ac1e035589845285c`), disbursed 3 XLM to provider and refunded 2 XLM surplus to sponsor.
- **Funded Expiry Refund**: Agreement #3 (tx `4d1b15c0d88c09f08fbcb2f7999281f7071ed03267e2351a80accd7c72839056`), expired without attestation, refunding 100% of escrow (2 XLM) to sponsor.
- **Administrative Dispute Refund**: Agreement #4 (tx `55bfa433069d4c44e4d40748531e339b0788449f165f250f395b6bff0ccf7972`), full 1.5 XLM escrow refunded to sponsor.
- **Pooled Escrow Isolation**: Concurrent deposits across Agreements #3 and #4 verified that isolated agreement balances cannot cross-contaminate.
- **Agreement #7 Reconciliation Recovery**: Ingestion failure replayed from ledger 5106190 via entity-first reconciliation pipeline, restoring consistent database mirror state.

Detailed audit trails and evidence references:
- [evidence/index.md](../evidence/index.md)
- [evidence/testnet-2026-10-09-block3b.md](../evidence/testnet-2026-10-09-block3b.md)
- [evidence/submission-readiness-2026-10-09.md](../evidence/submission-readiness-2026-10-09.md)

## Testing

Verified test totals across repository layers:

- **Rust Contracts**: 117 tests passing (90 care-agreement, 27 provider-registry)
- **Go API**: 127 tests passing (`go test -race -p 1 ./...`)
- **TypeScript Workspace**: 130 tests passing (80 SDK, 11 web unit, 39 fixtures)
- **Playwright E2E**: 19 tests passing (including axe-core accessibility audits)

## Release

- **Version**: `v0.1.0`
- **Release Tag Target**: `1775f08621a246894c799263cae94d52e1274018`
- **Status**: Published on GitHub Releases.
- **Context**: Release `v0.1.0` is an earlier verified release snapshot. Current `main` contains a subsequent maintainer-identity documentation cleanup commit. The release tag has not been rewritten. The release contract WASM artifacts (`provider_registry.wasm` SHA-256 `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`, `care_agreement.wasm` SHA-256 `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`) correspond exactly to the verified Block 3B Testnet deployment.

## Deployment

- **Web Application**: Verified locally at `http://localhost:3000`
- **Go API**: Verified locally at `http://localhost:8080`
- **PostgreSQL**: Runs locally through Docker (`postgres:18.6-bookworm` on port 5439)
- **Smart Contracts**: Live on Stellar Testnet
- **Documentation**: Public through GitHub Pages at `https://healthcarefund.github.io/carefund-platform/`
- **No Vercel deployment is claimed**
- **No Mainnet deployment is claimed**

## Known Limitations

- **Testnet only**: Development prototype deployed solely on Stellar Testnet.
- **No independent third-party smart-contract audit**: Formal third-party code audit has not been conducted.
- **No admin-authenticated dispute resolution UI**: `resolve_dispute` requires `admin.require_auth()`; web UI omits dispute resolution until administrative authentication is implemented.
- **No API bearer authentication**: REST endpoints rely on on-chain Soroban signatures for security rather than API bearer tokens.
- **No per-caller API rate limiting**: Rate limiting is deferred to future API gateway infrastructure.
- **No manual screen-reader audit**: Accessibility validated via automated axe-core scans; dedicated manual assistive technology review remains pending.
- **Deterministic SHA-256 commitments do not establish legal anonymization**: Low-entropy clinical data hashed naively remains vulnerable to correlation; callers must construct high-entropy commitments.
- **No HIPAA/GDPR compliance claim**: CareFund is not certified for handling regulated healthcare records.

## Post-Approval Contributor Runway

Extension areas reserved for community contribution following project approval (documented in [ROADMAP.md](../ROADMAP.md)):

- **Provider and Attester Operations**: Profile metadata proofs, multi-attester threshold authorization, attester reputation metrics.
- **Observability and Reconciliation**: Prometheus metrics, webhook notification service for agreement lifecycle transitions, dead-letter queue.
- **Security and Identity Hardening**: Role-based admin session management for `resolve_dispute`, Passkey / WebAuthn signer support, independent security audit.
- **Developer Ergonomics**: SDK auto-retrying RPC helpers, mock Soroban RPC local test container, CLI playground.

*Update 2026-10-27:* this pack originally stated that no issues had been created before approval. That is no longer accurate. Eight issues (#21 to #28) were opened on 2026-10-13, each corresponding to an area above: provider discovery for sponsor intents, reconciliation metrics, dead-letter handling, an authenticated admin dispute workflow, SDK retry helpers, a manual accessibility review, passkey signing and an interactive playground. They are planned work, not implemented features.

## Reviewer Resources

- [ARCHITECTURE.md](../ARCHITECTURE.md)
- [Differentiation analysis](../docs/differentiation.md)
- [Trust model and security evidence matrix](../docs/trust-and-security-evidence.md)
- [Reviewer Q&A](./reviewer-qa.md)

## Submission Status

READY FOR FINAL ELIGIBILITY CHECK AND MAINTAINER SUBMISSION
