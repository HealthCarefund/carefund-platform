# Testing Strategy

CareFund employs a layered testing strategy covering smart contracts, Go backend services, TypeScript SDK primitives, web UI components, and static documentation.

## Test Summary by Layer

| Layer | Primary Command | Scope and Execution Details |
|---|---|---|
| Contracts | `cargo test --workspace` | Contract unit tests, state machine transitions, math overflow guards |
| Contracts Lints | `cargo clippy --workspace --all-targets -- -D warnings -A deprecated -A clippy::too-many-arguments` | Strict linting across contract packages |
| Contracts Format | `cargo fmt --all -- --check` | Formatting conformance |
| API Units & DB | `cd apps/api && go test -p 1 ./...` | REST routes, idempotency, isolated PostgreSQL interactions |
| API Race Detection | `cd apps/api && go test -race -p 1 ./...` | Race condition verification across goroutines |
| Web Unit | `pnpm --filter @carefund/web run test` | Vitest tests for form validation and pure logic |
| Web E2E | `pnpm --filter @carefund/web run test:e2e` | Playwright tests for UI rendering, routes, and axe-core accessibility |
| Documentation | `mdbook build && lychee ...` | Static book compilation and link integrity |

---

## Critical Execution Requirements

### Why `go test -p 1` is Mandatory
The Go test suite interacts with the real, isolated PostgreSQL database container (`carefund-pg18` on port 5439). Test fixtures in `internal/store` and `internal/api` truncate database tables between test runs to guarantee deterministic fixtures.

Because Go by default executes tests across different packages in parallel, running without `-p 1` creates fatal race conditions where one package truncates tables while another is asserting query results. Always execute Go tests with `-p 1`.

### Live Network Tests
The test `TestPrepareTransaction_FundLiveTestnet` in `apps/api/internal/api/transactions_test.go` performs a genuine JSON-RPC call against the live Stellar Testnet contract.

To avoid introducing external network dependencies into CI, set the environment flag:
```bash
export SKIP_LIVE_NETWORK_TESTS=1
```
When this flag is set, the test cleanly skips itself.

### Scope and Boundaries of Web E2E Tests
The Playwright end-to-end suite (`apps/web/e2e/`):
- Runs in an automated headless Chromium browser without the Freighter wallet extension installed.
- Validates the genuine "Wallet Not Installed" error handling state.
- Mocks backend API endpoints to test UI form validation, page transitions, and error toast states.
- Executes automated accessibility scans using `@axe-core/playwright`.

The E2E suite does not claim to execute live Testnet transactions or real browser wallet signatures. Live Testnet verification is recorded separately in `evidence/testnet-2026-09-27.md`.
