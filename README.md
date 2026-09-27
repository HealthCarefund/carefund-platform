# CareFund

CareFund coordinates care agreements between sponsors, providers, and
attesters, with funding held and settlement executed by Soroban smart
contracts on Stellar. This repository is the full application layer built
on top of the Phase 6 contracts (`contracts/provider-registry`,
`contracts/care-agreement`): a Go API, a Next.js web app, and the
supporting TypeScript SDK packages.

## Architecture

```
contracts/                  Soroban contracts (Phase 6, unchanged by this work)
  provider-registry/          On-chain provider/attester registration and status
  care-agreement/              On-chain agreement lifecycle, funding, settlement, disputes

packages/
  types/                       Branded primitive types shared by every TS package
                                (StellarAddress, CommitmentHash, AgreementId, ...)
  sdk/                          Build/simulate/sign/submit/confirm transaction pipeline,
                                 Freighter wallet integration, config validation
  sdk/generated/*                Generated TS contract bindings (from each contract's WASM)

apps/
  api/                          Go backend: off-chain workflow metadata, transaction
                                 preparation, background reconciliation with the chain
  web/                          Next.js frontend: provider/sponsor/admin workflows,
                                 wallet-signed transactions submitted directly to RPC
```

**The chain is the source of truth.** Postgres (`apps/api`) stores only
off-chain workflow metadata — a mirror of on-chain state for fast querying,
plus purely off-chain records like agreement intents and idempotency keys.
It never stores patient, clinical, or other PHI: patient and service
identifiers are recorded everywhere (on-chain and in Postgres) only as
32-byte opaque commitments (hashes), computed by the caller before they
ever reach this system. When the mirror and the chain disagree, a
background reconciliation loop (`apps/api/internal/reconcile`) corrects
the mirror from the chain — never the other way around.

**The wallet is the only signer.** Neither `apps/api` nor `apps/web` ever
holds, receives, or transmits a private key. The Go API only builds,
simulates, and returns *unsigned* transaction XDR for six of the seven
agreement-lifecycle operations (`fund`, `cancel`, `attest_care`,
`open_dispute`, `expire`, `settle`, `resolve_dispute`); the frontend has
the browser wallet (Freighter, via `packages/sdk`) sign it and submits the
signed transaction directly to Soroban RPC itself — never proxied through
the backend. `create_agreement` is the one exception: since it doesn't yet
have an `agreementId`, it's built and prepared client-side using the same
SDK primitives instead of going through the backend's per-agreement
endpoint.

## Toolchain

See [`TOOLCHAIN.md`](./TOOLCHAIN.md) for the exact pinned versions (Go,
Node.js, pnpm, Rust, PostgreSQL, Stellar CLI, and the pinned TypeScript
package versions) and `scripts/check-toolchain.sh` to verify your local
environment matches. Nothing in this project silently falls back to a
different version.

## Local setup

1. **Database.** CareFund uses an isolated PostgreSQL 18.6 instance — it
   never touches a host PostgreSQL install:
   ```
   docker compose up -d
   ```
   This starts `carefund-pg18` on host port **5439** (database `carefund`,
   user `carefund`). See `docker-compose.yml`.

2. **Install dependencies:**
   ```
   pnpm install
   ```
   (Also builds the Rust/Soroban toolchain requirements separately — see
   `scripts/generate-contract-bindings.sh` if contract WASM/bindings need
   regenerating after a contract change.)

3. **Configure the API** (`apps/api`). Copy `.env.example` and set, at
   minimum, `DATABASE_URL` (already defaults to the docker-compose
   instance above) plus the Stellar network fields below.

4. **Configure the web app** (`apps/web/.env.local`), all `NEXT_PUBLIC_*`
   since they're genuinely public (network name, RPC URL, contract IDs —
   never a secret):
   ```
   NEXT_PUBLIC_STELLAR_NETWORK=TESTNET
   NEXT_PUBLIC_STELLAR_RPC_URL=https://soroban-testnet.stellar.org
   NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE=Test SDF Network ; September 2015
   NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID=C...
   NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID=C...
   NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID=C...
   NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
   ```
   `packages/sdk`'s `validateStellarClientConfig` refuses to start with a
   missing or malformed value here rather than silently defaulting — an
   incomplete config must fail loudly, not point a signed transaction at
   the wrong network.

5. **Run the API:**
   ```
   cd apps/api && go run ./cmd/api
   ```

6. **Run the web app:**
   ```
   pnpm --filter @carefund/web run dev
   ```

### Network configuration: Testnet vs. local

Every environment field above (`STELLAR_NETWORK`, RPC URL, passphrase,
contract IDs) is required and cross-validated — e.g. a `TESTNET` network
value must pair with the well-known Testnet passphrase, not an
arbitrary one. Pointing this app at a local/standalone Soroban network
instead of Testnet means setting `STELLAR_NETWORK=CUSTOM` with that
network's own RPC URL, passphrase, and freshly deployed contract IDs —
there is no built-in "local mode" beyond that; the API and web app never
know or care which network they're pointed at beyond what's configured.

## Testing

| Layer | Command | What it covers |
|---|---|---|
| Contracts | `cargo test --workspace`, `cargo clippy --workspace --all-targets`, `cargo fmt --all -- --check` | Contract unit/integration tests, lints, formatting |
| API | `cd apps/api && go test -p 1 ./...` (add `-race` for the race build) | Validation, idempotency, DB behavior via the real isolated Postgres instance, one genuine Testnet RPC round-trip (skippable with `SKIP_LIVE_NETWORK_TESTS=1`), transaction-lifecycle and reconciliation logic |
| Web unit | `pnpm --filter @carefund/web run test` (Vitest) | Pure logic only — currently the shared agreement-form validation (`apps/web/lib/agreement-form-validation.ts`) |
| Web E2E | `pnpm --filter @carefund/web run test:e2e` (Playwright) | This app's own UI: navigation, forms, accessibility (automated axe-core scans), and API-response handling |

**`go test -p 1` is mandatory**, not a style preference: `internal/store`
and `internal/api` both truncate shared tables in the same live Postgres
instance during test cleanup, and Go's default cross-package test
parallelism races those truncations against each other.

**What the E2E suite does *not* do**, and never claims to: it runs against
a plain browser with no Freighter extension installed and no real backend
or Testnet RPC reachable at its configured URLs. Tests either exercise
real (if minimal) app behavior that needs neither — like the genuine
"Freighter not installed" state, since no extension actually is — or mock
the backend API response with Playwright's route interception, which is
noted in each such test file. **None of this is, or is claimed to be, live
Testnet verification, real wallet signing, or real settlement.** That is
Phase 10's job, not this suite's; implementing this application layer
(Phase 7) is a distinct milestone from verifying it against live Testnet
infrastructure, and this README does not conflate the two.

## Known limitations

- **Agreement intents have no listing endpoint.** A sponsor's "request a
  new agreement" flow (`/sponsor/agreements/new`) creates an off-chain
  `agreement_intents` row and shows its ID once, but there is currently no
  way for a provider to browse pending intents addressed to them through
  this app — the intent ID must be shared out-of-band. The provider's own
  "propose a new agreement" flow does not read from intents at all; it
  takes the same fields directly.
- **`resolve_dispute` has no UI.** It's admin/deployer-authorized on-chain
  (`admin.require_auth()`), and no admin authentication or authority
  exists in this application layer to gate it behind — building that UI
  would mean either fabricating an admin login this app has no way to
  authenticate, or leaving a highly consequential control (dispute
  resolution, fund release) reachable by anyone who opens the page. This
  is treated as a deliberate scope boundary given the accepted
  architecture, not an oversight.
- **No browser was available to manually click through this app during
  development.** Every UI page was verified via typecheck, lint, a
  production build, and the automated E2E suite above — not by a human
  (or an agent driving a real browser) actually looking at rendered
  pages. Automated axe-core scans catch a meaningful subset of
  accessibility issues, not all of them; manual keyboard/screen-reader
  review has not been performed.
- **This README describes Phase 7 (application layer) completion.** No
  claim is made here about live Testnet verification, real wallet
  signing against production Freighter, real settlement, recovery under
  actual network failure, production readiness, or a completed security
  audit — those are separate, later milestones and would need their own
  evidence, not an extension of this one.
