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
| Toolchain versions match TOOLCHAIN.md | `scripts/check-toolchain.sh` output, all OK (Go 1.27.2, Node 24.21.0, pnpm 12.5.1, Rust 1.98.1, PostgreSQL 18.6, Stellar CLI 28.1.0/27.0.0) | 2026-10-09 | VERIFIED |
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
| Full background reconciliation loop against these live transactions | Verified live in Block 3B against transaction `9ae6dc48...` and Agreement #7 replay | 2026-10-09 | VERIFIED |
| Unauthorized attester action rejected live | Negative path rejected in Block 3B live RPC simulation matrix | 2026-10-09 | VERIFIED LIVE (simulation-rejected) |
| resolve_dispute admin action | no admin UI exists in this application | 2026-10-09 | KNOWN LIMITATION |
| Wrong-network signing prevention and recovery | Wrong-network detection: VERIFIED by manual browser observation on Testnet workflow; recovery after switching back to Testnet and refreshing: VERIFIED; automatic recovery without refresh: NOT VERIFIED | 2026-10-09 | VERIFIED (refresh recovery confirmed; auto-recovery without refresh NOT VERIFIED) |
| Recovery and failure-injection scenarios | see `evidence/recovery-2026-09-27.md` and `evidence/testnet-2026-10-09-block3b.md` | 2026-10-09 | see files |
| Rust dependency scan | `cargo audit`: 0 vulnerabilities, 1 unmaintained-crate warning (`paste` 1.0.15, transitive via soroban-sdk) | 2026-10-09 | VERIFIED |
| Go dependency scan | `govulncheck ./...`: 0 critical vulnerabilities; 0 standard-library findings on Go 1.27.2; 1 non-critical in x:text | 2026-10-09 | VERIFIED |
| Node dependency scan | `pnpm audit`: 0 critical vulnerabilities; updated to Next.js 16.3.8 (0 direct advisories in next; 2 transitive) | 2026-10-09 | VERIFIED |
| Contract security fix: dispute window overflow | `contracts/care-agreement/src/lib.rs` create_agreement rejects overflowing care_deadline/dispute_window_secs combination; redeployed to Testnet in Block 3B | 2026-10-09 | VERIFIED |
| CI and Docs workflows pass on GitHub Actions | Main SHA `2a26be753555896ed09966d4ab2eef441e4993c2`: CI run `38360698629` SUCCESS (Contracts, API, Web), Docs run `38360698573` SUCCESS; historical baseline run `36360910262` green | 2026-10-13 | VERIFIED |
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
| Block 3B Interactive Browser Wallet Signing | `evidence/testnet-2026-10-09-block3b.md`, Agreement #8 full browser lifecycle (tx `d2197fb8...`, `019fc50f...`, `b7494722...`) and signature rejection verified | 2026-10-09 | VERIFIED |
| CareFund v0.1.0 release published | Release tag target `1775f08621a246894c799263cae94d52e1274018` includes `provider_registry.wasm` (SHA-256 `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`) and `care_agreement.wasm` (SHA-256 `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`). Note: v0.1.0 is an earlier release snapshot; current main contains later maintainer-identity cleanup; release tag is untouched | 2026-10-11 | VERIFIED |
| Current repository maintainer identity | ZeePearl56 (https://github.com/ZeePearl56) configured as sole maintainer across repository metadata, CONTRIBUTING.md, and SECURITY.md; historical Git/release authorship preserved | 2026-10-13 | VERIFIED |
| Public documentation deployment | Deployed and verified at https://healthcarefund.github.io/carefund-platform/ (HTTP 200, mdBook build clean, Lychee link checks clean) | 2026-10-13 | VERIFIED |
| Administrator rotation (`transfer_admin`) merged to main | PR #39 merge commit `ca5fee3d6117abc61b5c0ff315ee74639d0ee149`; both contracts require current- and incoming-admin authorization; 124 + 38 local tests pass; 4 required CI checks pass on head `ca886f7b` | 2026-10-28 | VERIFIED |
| 2-of-3 multisignature admin: real-signature authorization | `evidence/multisig-admin-verification-2026-10-28.md`; 16 tests across both contracts using genuine Ed25519 signatures checked by `soroban-env-host`'s real account-authentication path, not `mock_auths()` | 2026-10-28 | TESTED LOCALLY |
| 2-of-3 multisignature admin: live network / classic `SetOptions` behavior | Not exercised — local Docker image pull for a standalone network stalled in this environment, and a fresh Testnet deployment was out of scope; see limitation in `evidence/multisig-admin-verification-2026-10-28.md` section 5 | 2026-10-28 | BLOCKED |
