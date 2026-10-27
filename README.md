<p align="center">
  <img src="assets/carefund-banner.webp" alt="CareFund: Healthcare and Funding Coordination on Stellar" width="100%" />
</p>
<p align="center">
  <em>Illustrative project artwork depicting healthcare and funding coordination on Stellar.</em>
</p>

# CareFund

<p align="center">
  <a href="https://github.com/HealthCarefund/carefund-platform/actions/workflows/ci.yml"><img src="https://github.com/HealthCarefund/carefund-platform/actions/workflows/ci.yml/badge.svg" alt="CI Status" /></a>
  <a href="https://github.com/HealthCarefund/carefund-platform/actions/workflows/docs.yml"><img src="https://github.com/HealthCarefund/carefund-platform/actions/workflows/docs.yml/badge.svg" alt="Documentation Status" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" alt="License: Apache-2.0" /></a>
  <a href="./evidence/testnet-2026-10-09-block3b.md"><img src="https://img.shields.io/badge/stellar-testnet-teal.svg" alt="Stellar Testnet: Evidenced" /></a>
</p>

<p align="center">
  <a href="docs-site/">Documentation Source</a> |
  <a href="https://healthcarefund.github.io/carefund-platform/">Documentation Book</a> |
  <a href="ARCHITECTURE.md">Architecture</a> |
  <a href="docs/differentiation.md">Differentiation</a> |
  <a href="docs/trust-and-security-evidence.md">Trust and Security</a> |
  <a href="submission/reviewer-qa.md">Reviewer Q&amp;A</a> |
  <a href="evidence/index.md">Verification Evidence</a> |
  <a href="SECURITY.md">Security Policy</a> |
  <a href="CONTRIBUTING.md">Contributing</a> |
  <a href="ROADMAP.md">Roadmap</a>
</p>

CareFund coordinates care agreements between healthcare sponsors, medical providers, and independent attesters, holding conditional funding in escrow and executing settlement autonomously through Soroban smart contracts on the Stellar network.

CareFund operates as a funding coordination and verification protocol. CareFund does not custody user signing keys. Users sign with their own wallet, while agreement funds are held temporarily in escrow by the Soroban smart contract under the protocol's conditional release rules. It is not health insurance, an underwriter, an electronic health records system, a patient diagnostic service, or a custodial wallet service.

## Proof at a Glance

**Central flow:** a sponsor funds an agreement, then an authorized attester submits a confirmation, then the contract enforces the timing and dispute conditions, and an eligible settlement pays the provider and refunds any surplus to the sponsor.

**What the attestation means:** the attester is authorized on-chain by the registry, but the protocol does not independently prove the real-world medical claim. A confirmation is a signed statement by an address the administrator registered. It is not verified accreditation or independently established proof that treatment happened. See [Trust and Security](docs/trust-and-security-evidence.md).

Everything below ran on **Stellar Testnet**. Transaction links open the public Stellar Expert Testnet explorer. Full detail is in [evidence/testnet-2026-10-09-block3b.md](evidence/testnet-2026-10-09-block3b.md).

| What was shown | Result | Evidence |
|---|---|---|
| **Agreement #8, browser and Freighter lifecycle** (create, fund, attest, settle) | Signed in the browser with the Freighter extension | [fund](https://stellar.expert/explorer/testnet/tx/d2197fb8d30103d126d0c71f7328c12732d57dd92d9156c2534c1417a7ee5e06), [attest](https://stellar.expert/explorer/testnet/tx/019fc50fc867eb7e1b9a71b775f82cbcb4462b16f2369fa2d1409b4ae17619f3), [settle](https://stellar.expert/explorer/testnet/tx/b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa) |
| **Permissionless third-party settlement** | A third-party account that is neither sponsor, provider nor attester called `settle` after the dispute window | [Agreement #8 settle](https://stellar.expert/explorer/testnet/tx/b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa), [Agreement 1 settle](https://stellar.expert/explorer/testnet/tx/9ae6dc480e876a32a18e5b0d12ce5a3187010c01ce319d0ac1e035589845285c) |
| **Exact provider payment and sponsor surplus refund** | Agreement #8: 8,000,000 stroops to the provider, 2,000,000 back to the sponsor, contract liability 0. Agreement 1: 30,000,000 and 20,000,000. Native XLM only | Same two transactions |
| **Funded agreement expiry with complete refund** | Agreement 3 expired with no attestation; all 20,000,000 stroops returned to the sponsor | [expire](https://stellar.expert/explorer/testnet/tx/4d1b15c0d88c09f08fbcb2f7999281f7071ed03267e2351a80accd7c72839056) |
| **Dispute resolution** | Agreement 4: dispute opened, then the admin refunded all 15,000,000 stroops to the sponsor (resolved with the Stellar CLI; there is no admin UI) | [open dispute](https://stellar.expert/explorer/testnet/tx/cd290e0df4c46e1e010614a98e0d7bfe3d6abd587240f0763c2f13a8a6eab578), [resolve](https://stellar.expert/explorer/testnet/tx/55bfa433069d4c44e4d40748531e339b0788449f165f250f395b6bff0ccf7972) |
| **Agreement #7 reconciliation recovery** | The mirror missed Agreement #7, then was repaired by replay from ledger 5106190 without manual SQL | [creation tx](https://stellar.expert/explorer/testnet/tx/a46300429dc1d091a8e722e4b143129121c846a0c8b39dbf4b6ee4411ec1a264), [section 8.2](evidence/testnet-2026-10-09-block3b.md) |

What is not shown: an independent audit, Mainnet, a public web deployment (the app ran on localhost), a demo video (pending), non-XLM assets, or any check that the attested care really happened.

**Reviewer resources:** [ARCHITECTURE.md](ARCHITECTURE.md) | [Differentiation from other Stellar escrow projects](docs/differentiation.md) | [Trust model and security evidence matrix](docs/trust-and-security-evidence.md) | [Reviewer Q&A](submission/reviewer-qa.md) | [Submission pack](submission/submission-pack.md) | [Evidence ledger](evidence/index.md)

## Why CareFund Exists

Charitable and subsidized healthcare programs frequently confront three systemic obstacles:

1. **Pre-funding Risk**: Donors who advance capital upfront risk funds being misspent or unverified against actual patient care delivery.
2. **Provider Cash-Flow Strain**: Requiring clinics to deliver care upfront and wait months for donor reimbursement strains community health operations.
3. **Privacy Vulnerabilities**: Centralized grant tracking often exposes sensitive patient identities and diagnostic notes to donor auditing staff.

CareFund is designed to reduce these trade-offs by holding donor funds in verifiable smart contract escrow upfront, releasing reimbursement to clinics only when authorized attesters verify clinical completion on-chain, and representing patient and service references through 32-byte cryptographic commitments.

## System Components

```
contracts/
  provider-registry/          On-chain provider status and attester authorization
  care-agreement/             Agreement escrow, state machine, and settlement

packages/
  types/                      Branded domain types (StellarAddress, CommitmentHash, ...)
  sdk/                        Transaction pipeline, simulation, and Freighter adapter
  sdk/generated/*             Typed Soroban contract bindings

apps/
  api/                        Go REST service: PostgreSQL metadata mirror, transaction prep,
                              idempotency control, and background chain reconciliation
  web/                        Next.js application: Provider, sponsor, and attester portals
                              with direct client-side wallet signing
```

- **The blockchain is the source of truth**: PostgreSQL (`apps/api`) stores only off-chain workflow metadata (agreement intents, idempotency reservations, and a queryable mirror of ledger state). Background workers reconcile the mirror from the chain, never the reverse.
- **The wallet is the only signer**: Neither the Go API nor the Next.js web application holds private keys. Transactions are prepared unsigned, signed client-side via the Freighter browser extension, and submitted directly to Soroban RPC.
- **Commitment Hashes**: Plaintext patient and clinical data must never be submitted to CareFund. Callers precompute 32-byte commitment hashes outside the protocol before creating or attesting agreements. The web application accepts precomputed 64-character hexadecimal hashes; it does not transform plaintext patient records. Deterministic SHA-256 hashing alone does not provide legal anonymization, and CareFund does not claim HIPAA or GDPR compliance.

## Core Protocol Workflow

1. **Provider Onboarding**: The contract administrator registers authorized clinics in `provider-registry` (`register_provider`) and authorizes bound attesters (`register_attester`). The registry records administrative authorization; it does not independently verify external medical accreditation.
2. **Agreement Proposal**: A registered provider initializes a care agreement (`create_agreement`) specifying token amounts, deadlines, cryptographic commitment hashes, and authorized counterparty addresses. The contract requires provider authorization.
3. **Escrow Deposit**: The sponsor deposits funds (`fund`) into the `care-agreement` contract via the Stellar Asset Contract (SAC).
4. **Care Attestation**: Upon procedure completion, the designated attester verifies clinical delivery and submits an attestation hash (`attest_care`).
5. **Autonomous Settlement**: Once care is attested and the dispute window elapses, settlement (`settle`) is callable deterministically by anyone, transferring reimbursement directly to the provider wallet and refunding any escrow surplus to the sponsor.
6. **Disputes and Safeguards**: If disagreements arise, either party can open a dispute (`open_dispute`) during the defined dispute window for administrative resolution (`resolve_dispute`).

## Local Quickstart

CareFund pins exact versions of every development tool (see [TOOLCHAIN.md](./TOOLCHAIN.md)). Verify your local environment matches:

```bash
./scripts/check-toolchain.sh
```

### 1. Start the Isolated Database
CareFund uses a dedicated PostgreSQL 18.6 container on host port **5439**, keeping host databases untouched:

```bash
docker compose up -d
```

### 2. Install Dependencies
```bash
pnpm install
```

### 3. Configure and Start the Go API
```bash
cp apps/api/.env.example apps/api/.env
cd apps/api && go run ./cmd/api
```
The API listens at `http://localhost:8080`.

### 4. Start the Web Application
```bash
pnpm --filter @carefund/web run dev
```
The frontend is available at `http://localhost:3000`. Detailed setup and environment documentation is available in the [Documentation Book](https://healthcarefund.github.io/carefund-platform/getting-started.html).

## Testing Strategy

| Layer | Command | Coverage and Boundary |
|---|---|---|
| Contracts | `cargo test --workspace` | State machine rules, deadlines, overflow checks |
| Contracts Lint | `cargo clippy --workspace --all-targets -- -D warnings -A deprecated -A clippy::too-many-arguments` | Strict contract compiler lints |
| Contracts Format | `cargo fmt --all -- --check` | Formatting checks |
| API Units & DB | `cd apps/api && go test -p 1 ./...` | REST routes, idempotency, isolated Postgres tests |
| API Race Check | `cd apps/api && go test -race -p 1 ./...` | Race condition verification across goroutines |
| Web Unit | `pnpm --filter @carefund/web run test` | Vitest tests for form validation logic |
| Web E2E | `pnpm --filter @carefund/web run test:e2e` | Playwright tests for UI rendering and axe-core accessibility |
| Documentation | `cd docs-site && mdbook build && lychee ...` | Static book build and link integrity validation |

*Note on `go test -p 1`*: This flag is mandatory. Tests truncate shared tables in the isolated PostgreSQL database during cleanup; parallel package execution causes data races.

*Scope of Web E2E tests*: Playwright runs in headless Chromium without the Freighter extension installed to verify the genuine missing-wallet user experience and mocks API responses. It does not perform live Testnet transactions.

## Verification Evidence and Status

CareFund documents all claims using a transparent evidence taxonomy in [evidence/index.md](./evidence/index.md):

| Area | Status | Verification Summary |
|---|---|---|
| Toolchain Pinned Versions | VERIFIED | Validated via `scripts/check-toolchain.sh` |
| Contracts Deployed to Testnet | VERIFIED | Block 3B deployed 2026-10-09 (`evidence/testnet-2026-10-09-block3b.md`); historical Block 1 on 2026-09-27 (`evidence/testnet-2026-09-27.md`) |
| Full On-Chain Lifecycle | VERIFIED | Multi-agreement matrix: nominal settlement with surplus refund, requested expiry, funded expiry, dispute refund |
| Negative Path Enforcement | VERIFIED LIVE | 9 invalid lifecycle scenarios simulation-rejected by live contract with exact ScError codes |
| API Live Testnet Roundtrip | VERIFIED LIVE | `TestPrepareTransaction_FundLiveTestnet` and `TestLookupTransaction_SuccessLiveTestnet` verified against live contract |
| Database Restart Recovery | VERIFIED LIVE | Background reconciliation worker confirms on-chain transactions and persists cursor across service restart |
| Continuous Integration | VERIFIED | GitHub Actions workflows passing (Contracts, API, Web including SDK tests, Docs) |
| Dispute Window Overflow Fix | VERIFIED LIVE | Fix deployed to Testnet in Block 3B (`care_agreement` WASM hash `b543e9a7...`) |
| Escrow Conservation & Surplus Refund | VERIFIED LIVE | Real token-conservation accounting verified on Testnet down to stroop precision |
| Browser Wallet Testnet Sign-off | VERIFIED | Live browser Freighter extension signing verified on Testnet (Agreement #8 complete lifecycle & signature rejection) |
| External Security Audit | KNOWN LIMITATION | No independent third-party smart contract audit performed |

### Testnet Contract Instances (Block 3B)
- **Provider Registry**: `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`
- **Care Agreement**: `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O`
- **Settlement Asset (Native XLM SAC)**: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`
Historical Block 1 instances (`CCGF5Y7...` and `CD6NC4...`) remain on Testnet for historical reference.

## Security Model and Limitations

- **Authorization**: On-chain actions strictly require cryptographic signatures via `require_auth()`.
- **No Private Key Custody**: Private signing keys are never handled or held by the backend or web server. Escrow funds are held directly by the Soroban care-agreement contract until release or refund conditions are met.
- **Privacy Boundaries**: Raw patient records and clinical documentation should never be submitted to CareFund. Contracts and databases store only precomputed 32-byte commitments. Predictable or low-entropy identifiers can remain vulnerable to correlation if naively hashed; external callers are responsible for constructing privacy-safe commitments. CareFund makes no HIPAA or GDPR compliance claims.
- **Admin Dispute UI**: Because the application currently lacks an administrative authentication layer, `resolve_dispute` has no web interface to prevent exposing escrow redirection to unauthorized users.
- **No Production Claims**: CareFund is an open-source development prototype deployed on Stellar Testnet. It is not currently audited or approved for production clinical operations.

For vulnerability reporting procedures, review [SECURITY.md](./SECURITY.md).

## Project Governance and Community

- **License**: [Apache-2.0](./LICENSE)
- **Contribution Guidelines**: [CONTRIBUTING.md](./CONTRIBUTING.md)
- **Code of Conduct**: [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- **Maintainer**: [ZeePearl56](https://github.com/ZeePearl56) (`l2ejourney@gmail.com`)
- **Roadmap and Next Steps**: [ROADMAP.md](./ROADMAP.md)
