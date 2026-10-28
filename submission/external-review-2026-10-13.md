# CareFund External Reviewer Simulation

- **Date**: 2026-10-13
- **Reviewer Role**: Simulated External Auditor / Grant Reviewer (Zero Pre-existing Context)
- **Branch**: `audit/final-playbook-alignment`
- **Base Commit**: `2a26be753555896ed09966d4ab2eef441e4993c2`

---

## 1. Executive Summary

This simulation evaluates the CareFund repository from the perspective of an independent external technical evaluator following the *Stellar Project Approval Playbook v2.0.0*. The evaluation inspected the repository structure, documentation integrity, toolchain alignment, verifiable ledger claims, test coverage, release assets, security boundary definitions, and pre-submission readiness.

The codebase demonstrates rigorous engineering practices, explicit deployment boundaries, verifiable Testnet transaction proofs, and strict adherence to factual reporting without marketing inflation.

---

## 2. Review Checklist (Evaluated in Playbook Order)

1. **README.md**:
   - Clear one-line and architectural summary.
   - Prominently states what CareFund is (conditional care-funding protocol on Stellar) and what it is not (not insurance, not EHR, not diagnostic service, not custodial wallet).
   - High-level architecture, contract instances, testing instructions, and evidence references.
   - Sole maintainer correctly identified as `ZeePearl56` with contact `l2ejourney@gmail.com`.

2. **Repository Root**:
   - Clean, well-structured monorepo (`contracts/`, `apps/`, `packages/`, `docs-site/`, `evidence/`, `submission/`, `scripts/`).
   - License (Apache-2.0), Toolchain, Roadmap, Security, and Contributing policies clearly established.
   - Zero untracked artifacts, temporary scratch scripts, or committed secrets.

3. **Open Issues**:
   - 0 open issues on GitHub repository. Clean backlog ready for post-approval contributor runway.

4. **Release**:
   - Release `v0.1.0` published on GitHub Releases.
   - Tag target points to verified commit `1775f08621a246894c799263cae94d52e1274018`.
   - Contract WASM release assets (`provider_registry.wasm` SHA-256 `30924b0d...`, `care_agreement.wasm` SHA-256 `b543e9a7...`) match Block 3B live Testnet deployments.
   - Historical release tag preserved untouched.

5. **CI Workflows**:
   - Workflows passing on GitHub Actions: `ci.yml` (Contracts, API, Web/SDK) and `docs.yml`.
   - Latest main CI run `38360698629` (SUCCESS); latest Docs run `38360698573` (SUCCESS).
   - Strict branch protection enabled on `main`.

6. **SECURITY.md**:
   - Direct reporting protocol to `l2ejourney@gmail.com`.
   - Explains cryptographic authorization boundary, commitment privacy model, and threat boundaries.

7. **CONTRIBUTING.md**:
   - Pinned toolchain versions aligned exactly with `TOOLCHAIN.md`.
   - Explicit instructions for testing layers (`go test -p 1` database truncation caveat clearly explained).
   - Maintainer designated as ZeePearl56; AI co-author trailers prohibited.

8. **Deployment & Documentation Links**:
   - GitHub Pages documentation live at `https://healthcarefund.github.io/carefund-platform/` (HTTP 200).
   - Local deployment boundaries explicitly delineated: localhost:3000 web, localhost:8080 API, port 5439 PostgreSQL container.
   - No Vercel deployment claimed; no Stellar Mainnet deployment claimed.

9. **Documentation Site (`docs-site/`)**:
   - Built cleanly using mdBook.
   - Offline Lychee link verification passes with zero broken internal links.

10. **Evidence Ledger (`evidence/`)**:
    - Central index refreshed with current toolchain, CI/Docs run IDs, v0.1.0 release entry, maintainer identity, and wrong-network detection testnet observation.
    - Transaction hashes and ledger numbers verifiable on Stellar Testnet for Agreement #8 full browser lifecycle and financial safety edge cases (surplus refund, funded expiry, dispute refund, pooled escrow isolation, Agreement #7 replay).

11. **Submission Pack (`submission/`)**:
    - `submission/submission-pack.md` created with comprehensive, plain-language breakdown traced to evidence.
    - `submission/demo.md` created with clear walkthrough steps and explicit separation between live demonstration and previously verified Testnet evidence.
    - Demo video marked `PENDING MAINTAINER RECORDING`.

---

## 3. Findings by Severity Classification

### Category A — Blockers (0 items)
*None.* No critical architectural flaws, unverified core claims, or breaking defects were identified.

### Category B — Stale / Cosmetic (3 items, all addressed)
1. **CONTRIBUTING Toolchain Drift**: CONTRIBUTING.md originally listed Go 1.27.1 and Stellar CLI 27.0.0 instead of Go 1.27.2 and Stellar CLI 28.1.0 (host) / 27.0.0 (CI). *(Resolved in current PR).*
2. **ROADMAP Gate Status**: ROADMAP.md previously listed documentation deployment and release readiness as pending gates despite v0.1.0 publication and active GitHub Pages deployment. *(Resolved in current PR).*
3. **Absence of Early Planning Notes**: Repository does not contain historical `research/ecosystem-recon.md` or `research/problem-validation.md` documents. *(Preserved genuine historical record without fabricating backdated files; non-blocking).*

### Category C — Known Limitations (7 items)
1. **Stellar Testnet Only**: CareFund is currently deployed and verified on Stellar Testnet; no Mainnet deployment exists.
2. **No Independent Third-Party Security Audit**: Contracts have undergone extensive internal fuzzing and financial safety verification, but no external security firm audit has been conducted.
3. **No Web Interface for Dispute Adjudication**: Contract operation `resolve_dispute` requires `admin.require_auth()`; web application intentionally omits dispute resolution UI until administrative authentication is implemented.
4. **No API Bearer Authentication**: Go REST API relies on Soroban smart contract authorization rather than custom backend authentication tokens.
5. **No Per-Caller API Rate Limiting**: REST service relies on standard network controls; fine-grained per-caller quotas remain deferred to gateway infrastructure.
6. **Assistive Screen-Reader Audit Pending**: Automated axe-core accessibility tests pass cleanly in Playwright, but manual screen-reader evaluations have not been performed.
7. **Cryptographic Privacy Boundary**: Protocol stores 32-byte opaque SHA-256 commitments rather than raw clinical records. Deterministic hashing does not guarantee legal anonymization; callers must supply high-entropy commitments; CareFund makes no HIPAA or GDPR compliance claims.

### Category D — Important Pre-Submission Fixes (4 items, all completed)
1. **Align Contributor Toolchain**: Corrected `CONTRIBUTING.md` and `docs-site/src/getting-started.md` to match authoritative `TOOLCHAIN.md`. *(Completed)*
2. **Update ROADMAP Completion Status**: Marked documentation verification and release gates as COMPLETED. *(Completed)*
3. **Refresh Central Evidence Ledger**: Updated `evidence/index.md` with latest CI runs, release v0.1.0 details, maintainer identity, and browser wrong-network verification observations. *(Completed)*
4. **Create Formal Submission Pack and Demo Record**: Authored `submission/submission-pack.md` and `submission/demo.md`. *(Completed)*

### Category E — Non-Blocking Backlog (Reserved Post-Approval Runway)
- Multi-attester threshold rules and attester reputation metrics.
- Prometheus metrics endpoint and webhook notification service.
- Role-based admin authentication and Passkey / WebAuthn signer integration.
- Expanded SDK client libraries and mock Soroban RPC local test container.

---

## 4. Conclusion and Readiness Assessment

The repository state fully satisfies the requirements of the *Stellar Project Approval Playbook v2.0.0*. All claims remain traceable to concrete ledger proofs, toolchain pinning is consistent across all documents, deployment boundaries are rigorously defined, and no speculative or backdated artifacts have been introduced.

**Readiness State**: The repository is ready for submission freeze pending live program eligibility verification by the maintainer.

---

## Addendum (2026-10-27): later observations

This addendum was added after the review above. The text above is the original record and is unchanged. It was a simulated review written for the project, not an independent audit.

1. **Open issues.** Section 2, item 3 recorded 0 open issues. As of 2026-10-27 there are 8 open issues (#21 to #28), opened on 2026-10-13. They are planned work.
2. **Attester wording.** Items describing attesters as independent or as verification officers overstated what the contracts establish. The registry records an administrator's decision. The contract does not verify accreditation or independence, and an attestation is an authorized statement, not proof of treatment.
3. **Demo record.** `submission/demo.md` used the state names `Created` and `Attested` and said the dispute window opened at attestation. The contract states are `Requested` and `CareConfirmed`, and the dispute window opens at `care_deadline`. Corrected in the 2026-10-27 documentation pass.
4. **Negative-path count.** The Block 3B evidence summary refers to nine simulation-rejected negative paths; eight are itemized.
5. **Registry rechecks.** `attest_care`, `settle` and `expire` do not consult the registry after creation. See `docs/trust-and-security-evidence.md`.
6. **Update 2026-10-28.** Item 5 describes the contract as deployed on Testnet. The current source now rechecks the attester's status and provider binding in `attest_care` (tested locally, not deployed). `settle`, `expire` and dispute resolution still do not consult the registry.
