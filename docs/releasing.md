# Release Procedure

This document specifies the exact procedure for repository maintainers to prepare and publish releases of CareFund, beginning with `v0.1.0`.

Releases are created manually by authorized repository maintainers from a verified `main` branch commit. No automated workflow publishes releases from unreviewed branches.

## Prerequisites

Ensure your local development environment satisfies all pinned toolchain versions:

```bash
./scripts/check-toolchain.sh
```

Required versions:
- Go 1.27.1
- Node.js 24.21.0
- pnpm 12.5.1
- Rust 1.98.1
- PostgreSQL 18.6 (local isolated Docker container `carefund-pg18`, port 5439)
- Stellar CLI 28.1.0 (host) / 27.0.0 (CI)

## Release Step-by-Step Procedure

### 1. Verify Clean Main Branch
Ensure your local checkout is on `main` and fully up to date with `origin/main`:

```bash
git checkout main
git pull --ff-only origin main
git status
```
Working tree must be completely clean.

### 2. Verify GitHub Actions Status
Verify that all required status checks on `main` are green for the target commit SHA:
- Contracts (Rust/Soroban)
- API (Go)
- Web (Next.js)
- Documentation (mdBook)

Check status via GitHub CLI:
```bash
gh run list --branch main -L 5
```

### 3. Run Fresh Security and Dependency Scans
Execute security scans locally and verify there are no unresolved critical vulnerabilities:

```bash
cargo audit
govulncheck ./...
pnpm audit
pnpm audit --prod
```

### 4. Rebuild Contract WASM Artifacts
Rebuild both contracts in release mode using the pinned toolchain:

```bash
stellar contract build
```

Verify that the compiled artifacts exist:
- `target/wasm32v1-none/release/provider_registry.wasm`
- `target/wasm32v1-none/release/care_agreement.wasm`

### 5. Verify WASM Checksums
Compute SHA-256 checksums of the compiled WASM files:

```bash
sha256sum target/wasm32v1-none/release/provider_registry.wasm target/wasm32v1-none/release/care_agreement.wasm
```

Expected hashes for Block 3B verified deployment:
- `provider_registry.wasm`:
  `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`
- `care_agreement.wasm`:
  `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`

If the computed hashes do not match, STOP. Do not publish a release without explaining the difference or completing a verified redeployment.

### 6. Verify TypeScript Contract Bindings
Run the contract bindings generation script and verify no code drift occurred:

```bash
./scripts/generate-contract-bindings.sh
git checkout packages/sdk/generated/*/package.json packages/sdk/generated/*/tsconfig.json
git status
```
Working tree must remain clean with zero diff in `packages/sdk/generated/*/src/`.

### 7. Run Full Verification Suite
Execute all verification commands before tagging:

```bash
cargo fmt --all -- --check
cargo clippy --workspace --all-targets -- -D warnings -A deprecated -A clippy::too-many-arguments
cargo test --workspace

cd apps/api
go vet ./...
go build ./...
go test -p 1 -count=1 ./...
go test -race -p 1 -count=1 ./...
cd ../..

pnpm -r run typecheck
pnpm -r run build
pnpm -r run test
pnpm --filter @carefund/web run lint
pnpm --filter @carefund/web run test:e2e

cd docs-site
mdbook build
lychee --offline --root-dir book/ --exclude-path book/404.html --exclude-path book/print.html book/
cd ..
```

### 8. Review Changelog and Documentation
Verify that `CHANGELOG.md` accurately describes the release version and dates. Verify that known limitations are documented transparently without overstating production or regulatory readiness.

### 9. Create Release Tag
Once all checks pass, create the annotated release tag:

```bash
git tag -a v0.1.0 -m "CareFund v0.1.0: Verified MVP on Stellar Testnet"
git push origin v0.1.0
```

### 10. Publish GitHub Release
Create the GitHub release attaching the contract WASM artifacts and documenting their SHA-256 hashes:

```bash
gh release create v0.1.0 \
  target/wasm32v1-none/release/provider_registry.wasm \
  target/wasm32v1-none/release/care_agreement.wasm \
  --title "CareFund v0.1.0" \
  --notes-file <(cat <<'EOF'
## CareFund v0.1.0

CareFund is an open-source funding coordination protocol on Stellar Testnet using Soroban smart contracts, a Go API backend, and a Next.js web application.

### Deployed Testnet Contracts (Block 3B)
- **Provider Registry**: `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`
- **Care Agreement**: `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O`
- **Settlement Asset (Native XLM SAC)**: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`

### Contract Artifacts and SHA-256 Checksums
- `provider_registry.wasm` (8,504 bytes): `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`
- `care_agreement.wasm` (17,075 bytes): `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`

### Public Documentation
- Documentation: https://healthcarefund.github.io/carefund-platform/
- Verification Evidence: https://github.com/HealthCarefund/carefund-platform/blob/main/evidence/index.md

### Known Limitations
- Deployed to Stellar Testnet only; not approved for production clinical operations.
- No administrative web UI for dispute resolution (admin authorization required).
- No external third-party security audit.
- Deterministic SHA-256 commitments do not constitute HIPAA or GDPR compliance.
EOF
)
```

### 11. Post-Publication Verification
Verify that:
1. The release appears on GitHub under Releases.
2. The attached WASM files can be downloaded and match the documented checksums.
3. The tag points to the expected commit SHA on `main`.
4. GitHub Pages documentation reflects the release.
