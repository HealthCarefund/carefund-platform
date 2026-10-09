# Security

## Reporting a vulnerability

If you find a security issue in this repository, please report it
privately rather than opening a public issue. Open a private security
advisory on this GitHub repository, or contact the repository
maintainer directly. Please include steps to reproduce, the affected
component (contract, API, or web app), and, where relevant, the network
(Testnet or otherwise) you observed it on.

## Scope

This document covers `contracts/provider-registry`,
`contracts/care-agreement`, `apps/api`, `apps/web`, and
`packages/sdk`/`packages/types` in this repository. It does not cover
the Stellar network itself, Soroban, Freighter, or any third party
dependency; report issues in those projects to their own maintainers.

## Supported versions

This project has not reached a tagged release. Security fixes are made
against `main` only.

## Wallet signing boundary

Neither `apps/api` nor `apps/web` ever holds, generates, receives, or
transmits a private key. Six of the seven agreement lifecycle
operations (`fund`, `cancel`, `attest_care`, `open_dispute`, `expire`,
`settle`) plus `resolve_dispute` are prepared unsigned by the Go API and
signed by the caller's own wallet (Freighter, via `packages/sdk`) in the
browser; the signed transaction is submitted directly from the browser
to Soroban RPC, never proxied through the backend. `create_agreement` is
prepared and simulated entirely client side for the same reason.

## Private key custody

No private key for any real account, Testnet or otherwise, is stored in
this repository, in `apps/api`, in `apps/web`, or in any environment
file committed here. `.env` and `.env.local` are git ignored. Test and
demonstration Stellar identities used during development and
verification are held in the Stellar CLI's local keystore on the
machine performing that work, outside version control.

## API authentication limitations

`apps/api` does not authenticate callers. It validates that a request's
claimed wallet address matches the role a given operation requires
(for example, that `fund` is only prepared for the agreement's own
sponsor wallet) before it will build a transaction, but this is a
convenience check, not an authorization boundary: the contract's own
`require_auth()` calls are the real, final authority, because only the
named wallet can actually produce a valid signature. A request to
prepare a transaction for the wrong wallet fails the API's check, and
even if it did not, the resulting transaction would still fail on
chain, unsigned or signed by the wrong key. The API also has no admin
authentication of any kind, which is why `resolve_dispute` (an
admin-only contract call) has no interface in this application; adding
one without a real admin authentication and authorization system in
front of it would be exposing a highly consequential control to anyone
who could load the page.

## Provider and attester authorization

Provider registration, suspension, reinstatement, and revocation, and
attester registration and status changes, are all admin only calls on
`contracts/provider-registry`, enforced by that contract's own
`require_auth()`, not by `apps/api`. `contracts/care-agreement` checks a
provider's active status and an attester's authorization for that
provider through real cross contract calls into the registry at
`create_agreement` and `fund` time, not through any value the caller
supplies about themselves.

## Data privacy

Patient and service identifiers are never stored as raw data anywhere in
this system, on chain or in `apps/api`'s Postgres mirror. They are
recorded only as 32 byte opaque commitments (hashes), computed by the
caller before they ever reach this system. Postgres stores off-chain
workflow metadata only (agreement mirrors for querying, agreement
intents, idempotency records, audit records, transaction references);
when it disagrees with chain state, chain state wins and background
reconciliation corrects the mirror. Request logging records method,
path, status, and duration only, deliberately never request or response
bodies, headers, or query strings.

## Dependency scanning

- `pnpm audit` (both with and without `--prod`): no known vulnerabilities
  found, as of 2026-09-27.
- `cargo audit`: no vulnerabilities found, as of 2026-09-27. One
  unmaintained-crate warning (`paste`, RUSTSEC-2024-0436), pulled in
  transitively through `soroban-sdk`'s cryptography dependencies, not a
  crate this project chooses directly and not a reported vulnerability.
- `govulncheck` (Go): found and fixed GO-2026-5970 (an infinite loop on
  invalid input in `golang.org/x/text`, reachable through
  `pgxpool.New`), remediated by upgrading to v0.39.0. A rerun afterward
  reports 0 vulnerabilities in code this project actually calls.

None of the above constitutes a full software composition analysis
program; they are point in time scans run manually as part of this
project's Block 1 security review.

## Audit status

**No independent, professional security audit of these contracts or
this application has been performed.** The verification performed in
this repository is automated testing (unit, integration, and one real
Stellar Testnet deployment and lifecycle execution, see
`evidence/testnet-2026-09-27.md`) plus a manual, non-exhaustive security
review by whoever is working on this repository at the time. Do not
treat this project as production ready or audited on the strength of
this document.

## Known limitations

- No admin authentication exists in `apps/api` or `apps/web`;
  `resolve_dispute` has no application interface for this reason.
- `apps/api` has no per-caller rate limiting.
- Dependency scanning is manual and point in time, not continuous;
  Dependabot is configured for automated update pull requests (see
  `.github/dependabot.yml`) but does not itself run a vulnerability
  scan on every commit.
- The contracts use `soroban_sdk::events::Events::publish`, which is
  deprecated in favor of the `#[contractevent]` macro. This is a real,
  tracked migration, not a security issue.

## Escrow safety and financial invariants

In Block 3A, the protocol established and verified five core financial invariants:
- **Full Escrow Conservation**: Settlement atomically disburses the agreed settlement amount to the provider and refunds any surplus escrow (funding amount minus settlement amount) back to the sponsor. No tokens remain stranded in contract escrow.
- **Dispute Window Non-Overlap**: Settlement is strictly prohibited while the dispute window remains active, preventing race conditions against dispute filings.
- **Permissionless Finalization**: Settlement parameters are immutable on-chain; any caller may trigger settlement once the dispute window elapses, preventing counterparty settlement veto.
- **Escrow Refund on Expiration**: Agreements expiring in the Funded state without attestation automatically refund the full deposit back to the sponsor. Agreements with attested care cannot be expired.
- **Conserved Dispute Resolution**: Dispute resolution via `Settle` disburses the settlement amount to the provider and refunds surplus to the sponsor; resolution via `Refund` returns the full deposit to the sponsor.

## Testnet status

Real, currently deployed Testnet contracts and a complete real lifecycle
verification (deployment, initialization, registration, creation,
funding, attestation, and settlement, each with an actual on chain
transaction hash and an independently checked resulting balance) are
recorded in `evidence/testnet-2026-09-27.md`. This is Testnet only. No
claim is made here about mainnet, and no claim is made about production
readiness.
