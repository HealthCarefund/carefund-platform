# Contributing to CareFund

Thank you for your interest in contributing to CareFund. CareFund coordinates care agreements between sponsors, providers, and attesters, with funding held and settlement executed by Soroban smart contracts on Stellar.

Please review these guidelines before submitting issues or pull requests.

## Project Scope and Philosophy

CareFund is an on-chain funding and settlement coordinator. It is deliberately:
- Not health insurance or an underwriter
- Not a clinical records system or EHR
- Not a patient diagnosis or telemedicine service
- Not a general payment gateway or custodial wallet

All patient and clinical identifiers exist solely as 32-byte opaque commitments (cryptographic hashes). Never store, transmit, or propose changes that introduce raw protected health information (PHI) or personally identifiable information (PII).

## Toolchain and Prerequisites

CareFund pins exact versions of every development tool. Verify your local environment matches by running:

```bash
./scripts/check-toolchain.sh
```

Pinned tool versions (documented in `TOOLCHAIN.md`):
- Go: 1.27.1
- Node.js: 24.21.0 (managed via `.nvmrc`)
- pnpm: 12.5.1 (enabled via Corepack)
- Rust: 1.98.1 (managed via `rust-toolchain.toml`, target `wasm32v1-none`)
- PostgreSQL: 18.6 (run via isolated Docker container on port 5439)
- Stellar CLI: 27.0.0

## Local Development Setup

1. Start the isolated PostgreSQL 18.6 instance:
   ```bash
   docker compose up -d
   ```
   This runs PostgreSQL on host port 5439 with database `carefund` and user `carefund`. CareFund never touches a host PostgreSQL installation.

2. Install Node.js dependencies:
   ```bash
   pnpm install
   ```

3. Configure the API:
   ```bash
   cp apps/api/.env.example apps/api/.env
   ```

4. Configure the web application:
   Create `apps/web/.env.local` using the public configuration values described in `README.md`.

## Verification and Testing by Layer

Run appropriate checks before submitting code:

### Contracts (Rust / Soroban)
```bash
cargo fmt --all -- --check
cargo clippy --workspace --all-targets -- -D warnings -A deprecated -A clippy::too-many-arguments
cargo test --workspace
```

To rebuild contract WASM binaries:
```bash
stellar contract build --package provider-registry
stellar contract build --package care-agreement
```

If contract interfaces change, regenerate TypeScript bindings:
```bash
./scripts/generate-contract-bindings.sh
```

### API (Go)
```bash
cd apps/api
gofmt -l .
go vet ./...
go test -p 1 ./...
go test -race -p 1 ./...
```

Note: The `-p 1` flag is mandatory. Test suites in `internal/store` and `internal/api` truncate tables in the shared isolated PostgreSQL database during cleanup. Running tests in parallel causes data races.

### Web (Next.js)
```bash
pnpm --filter @carefund/web run lint
pnpm --filter @carefund/web run typecheck
pnpm --filter @carefund/web run test
pnpm --filter @carefund/web run build
```

To run Playwright end-to-end tests:
```bash
cd apps/web
pnpm exec playwright test
```

### Documentation (mdBook)
```bash
cd docs-site
mdbook build
lychee --offline --root-dir book/ --exclude-path book/404.html --exclude-path book/print.html book/
```

## Pull Request and Commit Standards

1. Make one genuine logical commit at a time. Stage only specific files, write clear commit messages, and avoid blanket additions (`git add .`).
2. Never force-push or bypass branch protection on `main`. Branch protection requires all status checks to pass before merging.
3. The human repository author is Hollujay. Do not add `Co-Authored-By` trailers for AI assistants or bots.
4. Keep claims factual and backed by evidence. Do not invent test figures, coverage claims, external partnerships, or production milestones.
5. Avoid em dashes and en dashes in documentation. Use standard hyphens, colons, or parentheses.
6. When introducing or updating verification evidence, follow the project evidence taxonomy:
   - `VERIFIED`: Independently checkable on-chain or external record.
   - `TESTED LOCALLY`: Real automated test executed in the repository environment.
   - `LOGICALLY COVERED`: Unit tested or type checked, but not verified end-to-end against live networks.
   - `UNVERIFIED`: Real capability or code path not yet tested against live infrastructure.
   - `KNOWN LIMITATION`: Permanent or deliberate architectural boundary.
   - `BLOCKED`: Verification blocked by external dependencies.

## Reporting Security Vulnerabilities

Do not open public GitHub issues for security vulnerabilities. Review [SECURITY.md](./SECURITY.md) for reporting procedures.
