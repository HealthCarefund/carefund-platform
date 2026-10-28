# Differentiation: CareFund and Existing Stellar Escrow Work

This is a source-based comparison. It states what each project implements, where CareFund overlaps, and where CareFund is weaker. It is not a claim that CareFund is unique.

## Method and limits

- Source inspected by cloning the default branch on 2026-10-27 and reading the contract code, unless noted. Commit SHAs are listed in each section.
- Official documentation was consulted through search results only (`docs.trustlesswork.com`). I did not read every documentation page. Statements about Trustless Work's hosted API, fees and roles come from the contract code and from documentation snippets, and the docs themselves describe roles inconsistently across pages (two different role lists appear).
- I did not run any of the other projects' tests, check their deployments, or assess whether they are audited or used in production. Where I say "not found", I mean not found in the code I read.
- Search was not exhaustive. I searched GitHub and the web for Soroban escrow projects with healthcare, aid or conditional-disbursement angles. Other relevant projects may exist.
- CareFund facts come from this repository at its current commit ([ARCHITECTURE.md](../ARCHITECTURE.md)).

## Summary

| Mechanism | CareFund | Trustless Work contract | KindFi | Soter AidEscrow | Aegis medical-escrow |
|---|---|---|---|---|---|
| Soroban escrow of a token via SAC | Yes | Yes | Via Trustless Work | Yes | Yes |
| Named roles with separate authorization | Sponsor, provider, attester, admin | Approver, service provider, release signer, dispute resolver, platform, receiver | Via Trustless Work | Admin, distributors, recipient | Patient wallet, hospital, admin |
| On-chain registry of who may act | Yes, provider plus provider-bound attesters, checked by cross-contract call | No (roles are per-escrow parameters) | No | Admin-managed distributor list | Admin-managed hospital list (`verify_hospital`) |
| Deadlines enforced by the contract | Funding, care and dispute-window deadlines | Not found | Not found | `expires_at` per package | Not found |
| Dispute window that gates payout | Yes | No | No | No | No |
| Payout needs no trusted actor in the normal case | Yes: after the window, anyone calls `settle` | No: `release_signer` must sign | No | No: admin `disburse` | No: hospital claims, patient can cancel |
| Capped reimbursement with surplus refund in the same `settle` transaction | Yes | No (fixed escrow amount; fees only) | No | No | No (milestone amounts) |
| Outcome after expiry when invoked | After the deadlines pass, anyone can call `expire` on an eligible `Funded` agreement without attestation; the full sponsor refund happens when that transaction executes. A `Disputed` agreement cannot use this path | No expiry or deadline logic found; funds stay until a role acts | n/a | Admin must call `refund` after `expires_at`; the amount goes to the admin address | No expiry found |
| Dispute resolution | Admin picks Resume, Settle or Refund | Resolver distributes to any addresses | n/a | n/a | Admin sends remainder to patient or hospital |
| Milestones / partial release | No | Yes | Via Trustless Work | No | Yes |
| Platform fee model | None | Yes (0.3% protocol fee plus configurable platform fee) | Via Trustless Work | None | None |

## Trustless Work

Source: `Trustless-Work/trustlesswork-smart-contract-stellar`, commit `f0b706eaf6e98aa83bf4f95df42857b5d4c8e847` (2026-09-29). GitHub reported no license for the repository at the time of checking; a LICENSE file was not present in the clone.

1. **What it implements.** A general-purpose escrow contract factory. `tw_new_single_release_escrow` deploys an escrow and initializes it. Each escrow holds one `Escrow` record with `Roles` (approver, service provider, platform, release signer, dispute resolver, receiver), a list of milestones, flags (`disputed`, `released`, `resolved`), a trustline (the asset), and fee settings. The service provider marks milestones, the approver approves them, the release signer calls `release_funds` once all are approved, and the dispute resolver can resolve a disputed escrow by distributing balance across up to 50 addresses. It also has a hosted API, TypeScript SDKs, a backoffice, a Go indexer, and published Testnet evidence using the same native XLM SAC. The inspected contract is single-release; I did not inspect any multi-release variant. KindFi's web app depends on `@trustless-work/escrow`.
2. **Overlap.** Role-separated authorization with `require_auth`, SAC token escrow, a dispute state that blocks release, a funds-must-cover check before payout, off-chain indexing and SDK tooling. The role model (separate party approves, separate party releases) is close in spirit to CareFund's separation of attester and sponsor.
3. **What CareFund does differently.**
   - CareFund has an on-chain registry that the agreement contract calls at creation and funding. Trustless Work roles are plain addresses supplied per escrow, with no registry in the contract I read.
   - CareFund enforces time on-chain: funding deadline, care deadline, dispute window, and a refund path that becomes available after expiry (anyone can call `expire` once its state and timing conditions hold; a `Disputed` agreement cannot use it). I found no timestamp or deadline logic in the Trustless Work escrow contract. Its progress depends on someone calling a role-restricted function.
   - CareFund settlement is permissionless. In Trustless Work the `release_signer` must sign release.
   - CareFund pays a fixed `settlement_amount` and returns the surplus to the sponsor in the same `settle` transaction. Trustless Work releases the escrow `amount` less fees.
   - CareFund keeps many agreements in one contract with per-agreement accounting. Trustless Work deploys one escrow contract per engagement.
   - CareFund's dispute resolution is limited to three outcomes that can only pay the two named parties. Trustless Work's resolver can distribute to arbitrary addresses, which is more flexible and gives the resolver more power.
4. **Application configuration or protocol distinction?** The registry, bounded dispute outcomes and per-agreement accounting are application-specific design choices. The on-chain time-driven state machine, where the happy path and the default refund path both complete without cooperation from any specific trusted party (anyone can submit the transaction, but someone must), is a real behavioral difference from the Trustless Work contract as inspected. It is a different protocol design, not a parameter. It is not a new primitive in general: timed escrows with expiry are common.
5. **Where CareFund is weaker or less complete.**
   - No milestones or partial releases.
   - No fee or platform model, no multi-party funding.
   - Far smaller tooling surface and no hosted API, deployed ecosystem, or third-party integrators.
   - No audit. I did not determine whether Trustless Work has been audited; its repository contains an `evidence-report.md` and a `TW-V2-AUDIT-SCRIPTS` repository exists in the same organization, but I did not review either for audit status.
   - CareFund's attester model is a single attester per agreement. Trustless Work's approver is similarly single.
6. **What could be reused.** The SDKs, React components, indexer and API for the generic escrow lifecycle, and the many-recipient resolution pattern if CareFund ever wants more flexible dispute outcomes. Whether CareFund's policy could be run as a contract sitting in Trustless Work's `release_signer` role is an untested idea; I have not checked how contract-held roles would authorize and do not claim it works. Without on-chain deadlines in the escrow itself, a Trustless Work deployment could only enforce CareFund's timing through a trusted off-chain or wrapper actor.

## KindFi

Source: `kindfi-org/kindfi`, commit `e99e8cc7836e10eeec3e35d73f6fcf7becc95dc9` (2026-10-21), Apache-2.0.

1. **What it implements.** A crowdfunding platform. Its Soroban contracts directory contains auth-contract (account and passkey), governance, NFT, quest, referral, reputation and streak contracts. I found no escrow contract in that directory. Its web app has escrow routes and server actions and depends on `@trustless-work/escrow`, so its escrow is Trustless Work escrow.
2. **Overlap.** Conditional release of pooled donations through an escrow. Passkey account contracts (relevant to CareFund's open issue #27).
3. **What CareFund does differently.** Everything in the Trustless Work comparison applies, because KindFi delegates escrow to it. CareFund targets a named sponsor and a named provider for a specific piece of care, not a public campaign with many donors.
4. **Classification.** Not a competing escrow mechanism. KindFi is an application on Trustless Work.
5. **Where CareFund is weaker.** KindFi supports many donors, a governance and reputation layer, a larger team and a gamified front end. CareFund is a single-sponsor, single-agreement design.
6. **Reusable.** The passkey account and auth-controller contracts could inform CareFund's WebAuthn signer plans. I did not assess their quality or fit.

## General Soroban escrow implementations

A GitHub search for Soroban escrow contracts returns many freelance and milestone projects (for example `Talenttrust/Talenttrust-Contracts`, `laugh-tales/soroban-escrow`, `Earth-Kumar-Roy/SorobanEscrowHub`). I did not read these in detail. Their descriptions (milestone payments, disputes, reputation) match the Trustless Work shape. The reasonable assumption, labeled as an assumption, is that a time-locked refund with a single approver exists in many of them. CareFund should not be presented as the first Soroban escrow with deadlines or disputes.

## Closest domain neighbours

### Soter AidEscrow (humanitarian aid)

Source: `blizzj/Soter`, commit `e95609d9166f490ec464344b08b9f927dd272979` (2026-07-22). The README marks the project work in progress.

1. **Implements.** An `AidEscrow` contract with admin, distributors, packages with `expires_at`, claim, `disburse`, `revoke`, `refund`, pause, versioning and batch creation, plus aggregates.
2. **Overlap.** Per-package expiry and refund, escrow of aid funds on Soroban, event emission, TTL handling.
3. **Different.** Soter is admin-driven: `disburse`, `revoke` and `refund` all require the admin, and `refund` sends the amount to the admin address. Recipients are packages, not providers earning a capped reimbursement. There is no attester role in what I read and no dispute window.
4. **Classification.** Different application model. The admin-centric authorization is a weaker trust model than CareFund's for this use, though it fits aid distribution.
5. **Weaker in CareFund.** Pause and per-action pause controls, contract versioning and migration, batch creation, and aggregate accounting views.
6. **Reusable.** The pause and versioning patterns, if CareFund wants an emergency stop. That would add admin power and needs a deliberate decision.

### Aegis medical-escrow (emergency medical payment)

Source: `aegis-health-protocol/aegis-contracts`, commit `14a14447e842f1a51d1d0088b4e47c3d794ef036` (2026-08-13).

This is the closest healthcare comparator found.

1. **Implements.** A `MedicalEscrow` contract: admin registers and verifies hospitals, a verified hospital opens a vault for a patient with milestones, the patient's passkey wallet locks funds, the hospital completes milestones and is paid per milestone, the patient can cancel and recover remaining funds, either side can dispute after at least one milestone, and the admin resolves by sending the remainder to the patient or the hospital. It also ships a passkey wallet contract.
2. **Overlap.** Admin-maintained hospital registry, healthcare framing, dispute with admin resolution, escrow of a token, an Express and TypeScript backend with a PostgreSQL index.
3. **Different.**
   - Payer: the patient locks their own funds (emergency deposit). CareFund assumes a third-party sponsor.
   - Who confirms care: the hospital marks its own milestones complete and is paid immediately. There is no separate attester. CareFund separates the party paid from the party who confirms, though it does not make the confirmer independent.
   - Time: no funding, care or dispute deadlines and no automatic refund or finalization were found.
   - Settlement: the hospital claims; nothing is permissionless.
   - No surplus cap logic: payment is by preset milestone amounts.
   - Dispute resolution sends the entire remainder to one side.
4. **Classification.** The healthcare-specific registry gating is an overlap, not a difference. The attester separation, deadline-driven state machine and permissionless finalization are protocol-level differences from what is in this contract.
5. **Weaker in CareFund.** Milestones and progressive payment, patient-controlled cancellation, and passkey wallet support (CareFund lists passkeys as issue #27). Aegis targets a patient-centred emergency case; CareFund does not address it.
6. **Reusable.** The passkey wallet contract and the milestone model.

## Reading the combination

The task asked whether the following features, taken together, make CareFund distinctive: on-chain provider registration, provider-bound attesters, agreement-specific commitment hashes, funding and care deadlines, dispute-window enforcement, reimbursement caps, permissionless final settlement, deterministic surplus refunds, full refunds on eligible expiry, and agreement-specific accounting.

Each feature, against what was inspected:

| Feature | Already provided elsewhere? |
|---|---|
| On-chain provider registration | Yes, in Aegis (admin-verified hospitals) and Soter (distributors). |
| Provider-bound attesters via cross-contract check | Not found in the projects read. Registration of roles exists elsewhere; a binding of attester to provider checked by the escrow contract was not. |
| Agreement-specific commitment hashes | Opaque references are a common pattern. Not distinctive. Trustless Work stores free text; Aegis stores a procedure code. |
| Funding and care deadlines | Per-package expiry exists in Soter. Multi-deadline lifecycle not found in the others. |
| Dispute-window enforcement | Not found elsewhere. |
| Reimbursement caps with surplus refund | Not found elsewhere. This is a small mechanism that follows from the settle-less-than-funded model. |
| Permissionless final settlement | Not found in the contracts read. |
| Full refund on eligible expiry | Soter has an expiry refund, to the admin. CareFund refunds the sponsor and anyone may trigger it. |
| Agreement-specific accounting in one contract | Common practice for pooled contracts. Not distinctive. |

Conclusion:

- **Overlap that should be acknowledged.** Registry-gated actors, token escrow, role-separated authorization, admin-resolved disputes and per-package expiry all exist in other projects. CareFund should not claim these as novel.
- **The one distinction I can defend from the code.** CareFund's escrow is a time-driven state machine in which, once an attester has recorded a confirmation, no payout-side actor has to cooperate: after a fixed window anyone can finalize, and if no confirmation arrives and no dispute is opened, anyone can call `expire` to refund the sponsor. Someone must still submit these transactions; nothing runs on a schedule, and a disputed agreement waits for the administrator. The other contracts I read need a specific party (a release signer, a hospital, an admin) to act, or have no deadline logic. This is a design property of the contract, not an empirical result, and it matters only if the rest of the trust model is acceptable.
- **What that distinction does not buy.** The attester is still trusted, and the admin still decides disputes. The permissionless step removes counterparty veto over payment after attestation, but a dispute by either party reintroduces the admin. CareFund does not remove the oracle problem of establishing that care was delivered. See [trust-and-security-evidence.md](./trust-and-security-evidence.md).
- **Serious weaknesses relative to alternatives.** No milestones or partial payment, one attester with no threshold or independence check, admin dispute power with no timeout and no key rotation, and no audit. Trustless Work is the more mature general escrow, and a project needing milestones or a fee model would reasonably use it.
- **Reasonable positioning.** A healthcare-oriented conditional reimbursement design with deadline-driven default outcomes, built as its own contract, rather than a new escrow primitive.
