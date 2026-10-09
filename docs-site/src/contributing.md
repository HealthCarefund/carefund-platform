# Contributing to CareFund

We welcome contributions to the CareFund project. To maintain repository safety and submission integrity, all contributors must adhere to these standards.

## Code Standards and Workflow

1. **One Logical Unit Per Commit**:
   Group changes into small, coherent commits. Stage only relevant files and write descriptive commit messages explaining the rationale.
2. **Branch Protection**:
   All changes to `main` must arrive via pull request. Required CI checks (`Contracts`, `API`, `Web`) must pass before merging.
3. **Attribution Integrity**:
   The primary repository author is Hollujay. Never add automated AI co-author trailers (`Co-Authored-By`) or bot attribution.
4. **No Factual Exaggerations**:
   Do not introduce unverified test counts, mock adoption metrics, fake hospital affiliations, or unmeasured coverage badges.
5. **Punctuation Discipline**:
   Do not use em dashes or en dashes in newly written documentation. Use standard hyphens, colons, or parentheses.

---

## Toolchain Alignment

Before submitting contributions, verify that your local environment matches the versions pinned in `TOOLCHAIN.md`:

```bash
./scripts/check-toolchain.sh
```

---

## Verification Requirements

Run all relevant tests prior to opening a pull request:

```bash
# Contracts
cargo fmt --all -- --check
cargo clippy --workspace --all-targets -- -D warnings -A deprecated -A clippy::too-many-arguments
cargo test --workspace

# Go API (note -p 1 is mandatory)
cd apps/api
go vet ./...
go test -p 1 ./...
go test -race -p 1 ./...

# Web Frontend
cd ../..
pnpm --filter @carefund/web run lint
pnpm --filter @carefund/web run typecheck
pnpm --filter @carefund/web run test
pnpm --filter @carefund/web run build

# Documentation
cd docs-site
mdbook build
lychee --offline --root-dir book/ --exclude-path book/404.html --exclude-path book/print.html book/
```

---

## Evidence Taxonomy

When contributing tests or verification records, categorize claims according to the standard project taxonomy:
- `VERIFIED`: Independently checkable on-chain or external record.
- `TESTED LOCALLY`: Real automated test executed in the local repository environment.
- `LOGICALLY COVERED`: Unit tested or type checked, but not verified end-to-end against live networks.
- `UNVERIFIED`: Real capability or code path not yet tested against live infrastructure.
- `KNOWN LIMITATION`: Permanent or deliberate architectural boundary.
- `BLOCKED`: Verification blocked by external dependencies.

---

## Reporting Vulnerabilities

Security issues must be reported privately according to the procedures documented in [SECURITY.md](https://github.com/HealthCarefund/carefund-platform/blob/main/SECURITY.md).
