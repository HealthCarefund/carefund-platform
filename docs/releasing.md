# Release Procedure

This document specifies the exact procedure for repository maintainers to prepare and publish future CareFund releases. Release `v0.1.0` is already published and is a historical verified snapshot; do not recreate, move or overwrite its tag or release assets.

Releases are created manually by authorized repository maintainers from a verified `main` branch commit. No automated workflow publishes releases from unreviewed branches.

## Prerequisites

Ensure your local development environment satisfies all pinned toolchain versions:

```bash
./scripts/check-toolchain.sh
```

Required versions:
- Go 1.27.2
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

The Documentation (mdBook) workflow is separate and path-triggered. Run it when documentation or `.github/workflows/docs.yml` changes; it is not one of the three required branch-protection checks.

Check status via GitHub CLI:
```bash
gh run list --branch main --limit 10
gh run view <ci-run-id> --json headSha,status,conclusion,jobs,url
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
stellar contract build --package provider-registry
stellar contract build --package care-agreement
```

Verify that the compiled artifacts exist:
- `target/wasm32v1-none/release/provider_registry.wasm`
- `target/wasm32v1-none/release/care_agreement.wasm`

### 5. Verify WASM Checksums
Compute SHA-256 checksums of the compiled WASM files:

```bash
sha256sum target/wasm32v1-none/release/provider_registry.wasm target/wasm32v1-none/release/care_agreement.wasm
```

These are the historical hashes for the Block 3B deployment and the `v0.1.0` release artifacts:
- `provider_registry.wasm`:
  `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`
- `care_agreement.wasm`:
  `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`

If verifying `v0.1.0`, the hashes must match. A future source release is expected to have different hashes after contract changes; document the new build and deployment evidence rather than comparing it to the historical values or overwriting the old tag.

### 6. Verify TypeScript Contract Bindings
Run the contract bindings generation script and verify no code drift occurred:

```bash
./scripts/generate-contract-bindings.sh
git restore packages/sdk/generated/*/package.json packages/sdk/generated/*/tsconfig.json
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

### 9. Create a New Release Tag
Once all checks pass, choose a new version that does not already exist. Never reuse `v0.1.0`:

```bash
RELEASE_TAG=v0.2.0  # replace with the next approved version
git tag -a "$RELEASE_TAG" -m "CareFund $RELEASE_TAG"
git push origin "$RELEASE_TAG"
```

### 10. Publish GitHub Release
Create the GitHub release for the new tag, attaching only artifacts built and verified for that same source commit:

```bash
gh release create "$RELEASE_TAG" \
  target/wasm32v1-none/release/provider_registry.wasm \
  target/wasm32v1-none/release/care_agreement.wasm \
  --title "CareFund $RELEASE_TAG" \
  --notes-file release-notes.md
```

The release notes must identify the exact source commit, whether a new Testnet deployment exists, each artifact checksum, the local/CI verification performed, and any remaining limitations. If no new Testnet deployment was made, say so explicitly and preserve the historical Block 3B references.

### 11. Post-Publication Verification
Verify that:
1. The release appears on GitHub under Releases.
2. The attached WASM files can be downloaded and match the documented checksums.
3. The tag points to the expected commit SHA on `main`.
4. GitHub Pages documentation reflects the release.
