# Evidence index

This file tracks the status of every significant verification claim made
about CareFund. Each entry states the claim, the evidence backing it, the
date it was checked, and its status using one of:

`VERIFIED` (real, externally checkable evidence exists), `TESTED LOCALLY`
(a real automated test ran on this machine, not against a fully live
external system), `LOGICALLY COVERED` (the code path is unit tested or
type checked but not exercised end to end), `UNVERIFIED` (not yet
checked), `KNOWN LIMITATION` (a real, permanent gap), `BLOCKED` (checked,
and something outside this session's control prevents verification).

| Claim | Evidence | Date checked | Status |
|---|---|---|---|
| Toolchain versions match TOOLCHAIN.md | `scripts/check-toolchain.sh` output, all OK | 2026-09-27 | VERIFIED |
| provider-registry deployed to Testnet | `evidence/testnet-2026-09-27.md`, contract `CCGF5Y7CYRJSCMXE7NRCAXY4CKYWY22BXWOYILFXZD32EXFHOOTLNMSG` | 2026-09-27 | VERIFIED |
| care-agreement deployed to Testnet | `evidence/testnet-2026-09-27.md`, contract `CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS` | 2026-09-27 | VERIFIED |
| Both contracts initialized with correct configuration | `evidence/testnet-2026-09-27.md` initialization table | 2026-09-27 | VERIFIED |
| Provider registered and active on-chain | `evidence/testnet-2026-09-27.md`, `is_provider_active` returned true | 2026-09-27 | VERIFIED |
| Attester registered and authorized on-chain | `evidence/testnet-2026-09-27.md`, `check_attester` returned true | 2026-09-27 | VERIFIED |
| Agreement creation on-chain | `evidence/testnet-2026-09-27.md`, tx `833b4c6e...` | 2026-09-27 | VERIFIED |
| Funding transfers real asset on-chain | `evidence/testnet-2026-09-27.md`, tx `57bc0acd...`, contract balance confirmed `50000000` | 2026-09-27 | VERIFIED |
| Attestation on-chain | `evidence/testnet-2026-09-27.md`, tx `90f8f02c...` | 2026-09-27 | VERIFIED |
| Settlement transfers real asset on-chain | `evidence/testnet-2026-09-27.md`, tx `70d8fff0...`, provider Horizon balance increased | 2026-09-27 | VERIFIED |
| Contract rejects unauthorized/invalid-state/timing violations | `evidence/testnet-2026-09-27.md` failure paths table, 5 scenarios | 2026-09-27 | VERIFIED LIVE (simulation-rejected) |
| API transaction-preparation pipeline works against live contract | `apps/api/internal/api/transactions_test.go` `TestPrepareTransaction_FundLiveTestnet`, run without `SKIP_LIVE_NETWORK_TESTS` | 2026-09-27 | TESTED LOCALLY |
| Full background reconciliation loop against these live transactions | not exercised this block | 2026-09-27 | UNVERIFIED |
| Unauthorized attester action rejected live | not exercised this block | 2026-09-27 | UNVERIFIED |
| resolve_dispute admin action | no admin UI exists in this application | 2026-09-27 | KNOWN LIMITATION |
| Wrong-network signing prevention | unit tested in `packages/sdk/src/config.ts`, not exercised against a second live network | 2026-09-27 | LOGICALLY COVERED |
| Recovery and failure-injection scenarios | see `evidence/recovery-2026-09-27.md` | 2026-09-27 | see file |
| Rust dependency scan | `cargo audit`: 0 vulnerabilities, 1 unmaintained-crate warning (`paste`, transitive via soroban-sdk) | 2026-09-27 | VERIFIED |
| Go dependency scan | `govulncheck ./...`: found and fixed GO-2026-5970 (`golang.org/x/text`), 0 vulnerabilities in called code after the fix | 2026-09-27 | VERIFIED |
| Node dependency scan | `pnpm audit` and `pnpm audit --prod`: no known vulnerabilities | 2026-09-27 | VERIFIED |
| Contract security fix: dispute window overflow | `contracts/care-agreement/src/lib.rs` create_agreement now rejects an overflowing care_deadline/dispute_window_secs combination, regression test added | 2026-09-27 | VERIFIED (fixed, tested, deployed WASM not yet redeployed to the Testnet contract from this block; see limitations) |
| CI actually passes on GitHub Actions | run `36360910262` on commit `5908905`, all three jobs (Contracts, API, Web) green, observed via `gh run watch` and `gh api .../check-runs`, not inferred from YAML | 2026-09-27 | VERIFIED |
| Branch protection enabled on main | `gh api repos/HealthCarefund/carefund-platform/branches/main/protection`, read back fresh after configuring: required status checks (strict, the three real check names above), PR required, force push disabled, deletion disabled, admin bypass left available (not enforced) | 2026-09-27 | VERIFIED |
| Block 3A Financial Invariants Review | `evidence/financial-safety-review-2026-10-09.md` defect reproduction and remediation matrix | 2026-10-09 | VERIFIED |
| Block 3A Escrow Conservation and Surplus Refund | `contracts/care-agreement/src/test.rs` test_red_b3 and test_per_agreement_conservation_vs_aggregate_contract_balance verified; per-agreement escrow liability returns to zero on settlement | 2026-10-09 | TESTED LOCALLY |
| Block 3A Dispute Window Non-Overlap | `contracts/care-agreement/src/test.rs` test_red_b4 verified; settlement blocked while dispute window active | 2026-10-09 | TESTED LOCALLY |
| Block 3A Unverified Expiration Escrow Refund | `contracts/care-agreement/src/test.rs` test_red_b1 verified; full refund on Funded expiration | 2026-10-09 | TESTED LOCALLY |
| Block 3A Permissionless Settlement | `contracts/care-agreement/src/test.rs`, SDK `builders.test.ts`, API `transactions_test.go` | 2026-10-09 | TESTED LOCALLY |
| Pre-Deployment Hardening: Funding Deadline Boundary | `contracts/care-agreement/src/test.rs` exact boundary tests verified; funding valid at now == fd, expire strictly past fd | 2026-10-09 | TESTED LOCALLY |
| Pre-Deployment Hardening: Checked Surplus Math | `contracts/care-agreement/src/lib.rs` checked_sub enforced in settle and resolve_dispute | 2026-10-09 | TESTED LOCALLY |
| Pre-Deployment Hardening: CI SDK Automation | `.github/workflows/ci.yml` builds bindings, runs SDK typecheck and unit tests in Web (Next.js) job | 2026-10-09 | TESTED LOCALLY |
| Block 3B Provider Registry Deployment | `evidence/testnet-2026-10-09-block3b.md`, contract `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS` | 2026-10-09 | VERIFIED |
| Block 3B Care Agreement Deployment | `evidence/testnet-2026-10-09-block3b.md`, contract `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` | 2026-10-09 | VERIFIED |
| Block 3B Live On-Chain Settlement with Surplus Refund | `evidence/testnet-2026-10-09-block3b.md`, agreement 1, tx `9ae6dc48...` permissionless settle | 2026-10-09 | VERIFIED |
| Block 3B Live On-Chain Funded Expiry Refund | `evidence/testnet-2026-10-09-block3b.md`, agreement 3, tx `4d1b15c0...` 100% escrow refund | 2026-10-09 | VERIFIED |
| Block 3B Live On-Chain Dispute Resolution | `evidence/testnet-2026-10-09-block3b.md`, agreement 4, tx `55bfa433...` admin refund | 2026-10-09 | VERIFIED |
| Block 3B Live Escrow Pooling Isolation | `evidence/testnet-2026-10-09-block3b.md`, agreements 3 and 4 concurrent escrow verification | 2026-10-09 | VERIFIED |
| Block 3B API Background Reconciliation Live Testnet | `evidence/testnet-2026-10-09-block3b.md`, Go API worker reconciled tx `9ae6dc48...` to confirmed | 2026-10-09 | VERIFIED |
| Block 3B Interactive Browser Wallet Signing | `evidence/testnet-2026-10-09-block3b.md`, requires manual maintainer interaction with browser extension | 2026-10-09 | AWAITING MAINTAINER INTERACTION |
