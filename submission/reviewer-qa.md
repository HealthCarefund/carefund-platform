# Reviewer Q&A

Direct answers to hard questions. Each answer points to the code or evidence it rests on. Where the honest answer is "no" or "it depends", it says so.

Context: [ARCHITECTURE.md](../ARCHITECTURE.md), [docs/trust-and-security-evidence.md](../docs/trust-and-security-evidence.md), [docs/differentiation.md](../docs/differentiation.md), [evidence/index.md](../evidence/index.md).

## Why does CareFund need Stellar?

It uses specific Soroban features: signature-based role checks (`require_auth`), token escrow through the Stellar Asset Contract, atomic multi-transfer settlement, ledger timestamps for deadlines, and cross-contract calls to a registry ([ARCHITECTURE.md section 3](../ARCHITECTURE.md#3-why-stellar-and-soroban)). These are properties of a programmable ledger generally. CareFund does not claim only Stellar can host this design. Stellar was chosen for its ecosystem, low fees and native asset support.

## Why not use Trustless Work or another escrow system?

Trustless Work is the more mature general escrow, with milestones, fees, SDKs and an indexer, and a project that needs those should consider it. The contract inspected has roles but no deadlines, no registry and no permissionless release: a `release_signer` must sign, and nothing happens on its own if no one acts. CareFund wanted deadlines and a default refund enforced on-chain, a dispute window, a payment cap with automatic surplus refund, and payout that needs no cooperating party after attestation. Those could in principle be built around Trustless Work with a trusted off-chain actor, but that would reintroduce the trusted actor. Details and sources: [docs/differentiation.md](../docs/differentiation.md). The comparison covers the projects found; it is not exhaustive.

## What is actually healthcare-specific about the protocol?

Little in the contract mechanics. The healthcare-specific parts are the vocabulary and structure: a registry of providers, attesters bound to a specific provider, a sponsor who is not the patient, patient and service references held only as 32-byte commitments, and a capped reimbursement. The same state machine could model other conditional reimbursements. There is no clinical logic, no medical code validation and no identity verification. Another project, Aegis medical-escrow, also has a hospital registry, so a registry alone is not a differentiator.

## Who confirms care was delivered?

An attester: an address the admin registered in `provider-registry` and bound to one provider, named in the agreement at creation. Only that address can call `attest_care`. Calling it records an on-chain statement. It does not prove the treatment happened. The registry records an administrator's decision and an opaque credential reference, not verified accreditation or independence, and the contract does not stop the attester, provider or sponsor being the same address.

## What happens if the attester lies?

The agreement moves to `CareConfirmed`. The sponsor can open a dispute only between `care_deadline` and `care_deadline + dispute_window_secs`. The admin then decides between `Refund`, `Settle` and `Resume`. If nobody disputes in time, anyone can settle and the provider is paid. There is no evidence check, bond or penalty. See [trust-and-security-evidence.md section 4](../docs/trust-and-security-evidence.md#4-if-the-attester-is-dishonest).

## Can an administrator redirect funds?

Not to an arbitrary address and not in arbitrary amounts. The only function that moves funds at admin discretion is `resolve_dispute`, and only on a `Disputed` agreement. Its transfers go to the agreement's own provider or sponsor in amounts fixed at creation. The admin cannot sweep, pause or upgrade (no such functions exist). The admin can, however, choose whether the provider or the sponsor gets the money in any dispute, with no timeout and no key rotation. See [ARCHITECTURE.md section 5](../ARCHITECTURE.md#5-admin-and-upgrade-surface).

## What happens if the provider never delivers?

If no attestation is made by `care_deadline`, the agreement stays `Funded`. After `care_deadline + dispute_window_secs`, anyone can call `expire` and the full `funding_amount` returns to the sponsor. Live evidence: Agreement 3, tx `4d1b15c0d88c09f08fbcb2f7999281f7071ed03267e2351a80accd7c72839056`, 20,000,000 stroops refunded ([evidence](../evidence/testnet-2026-10-09-block3b.md), Case 3). If a dispute is open, it waits for the admin instead.

## Can funds become stuck in escrow?

Yes, in these cases:

- An agreement in `Disputed` stays that way until the admin resolves it. The admin key cannot be rotated, so a lost key locks those funds.
- Tokens sent directly to the contract address cannot be recovered; there is no sweep.
- For non-native assets, a payout could fail if a trustline is missing or an issuer freezes or claws back. This is reasoning from Stellar asset rules, not tested here.

Normal paths do not get stuck: every non-disputed state has an anyone-can-call exit after its deadline.

## What prevents double settlement?

`settle` requires `CareConfirmed` and sets `Settled` in the same atomic transaction, so a second call fails with `InvalidState`. Test: `test_security_double_settlement_rejected`. Live: on Agreement 1 a second settle returned `Error(Contract, #4)` in simulation. Replaying the same signed transaction is also rejected by Stellar sequence numbers.

## What does CareFund reveal about patients?

The chain shows sponsor, provider and attester addresses, amounts, deadlines, state, and 32-byte commitments for patient reference, service and attestation. It shows no plaintext patient data because callers are told not to submit any. It does reveal who paid whom, how much and when. Repeated use of the same addresses, amounts or patient commitment links agreements together. The off-chain database holds the same data.

## Why are commitment hashes not automatically private?

A hash of a guessable input (name, birth date, record number, service code) can be recovered by trying candidates. CareFund does not compute commitments. The web form accepts a precomputed 64-character hex string and the contract accepts any 32 bytes, so it cannot enforce salting or high entropy. Privacy depends entirely on how the caller builds the commitment. No HIPAA or GDPR claim is made.

## Can CareFund operate without the Go API?

For fund safety, yes. The contracts do not call the API or database, and every transition can be built with the SDK or the Stellar CLI. The Block 3B Testnet matrix recorded signing with local identity keystores rather than the browser, except Agreement #8. What the API adds is unsigned-transaction preparation, idempotency, intent storage and the queryable mirror. The web app gets the unsigned transaction for six operations from the API (`create_agreement` is built in the browser), so the shipped UI depends on it. Registry administration and dispute resolution have no UI at all and already use the CLI.

## How is the off-chain database reconciled?

A background loop in the API process looks up pending transaction hashes and reads contract events from a persisted ledger cursor, upserting the referenced agreement, provider and attester from chain state before storing each event. The cursor advances only if the whole batch was stored. Chain state wins. Agreement #7 was missed by an earlier version of this loop, then recovered by replay from ledger 5106190 ([evidence section 8.2](../evidence/testnet-2026-10-09-block3b.md)). Two possible weaknesses in event retention and pagination are described in [ARCHITECTURE.md section 10](../ARCHITECTURE.md#10-postgresql-as-a-rebuildable-mirror) and were not reproduced.

## Is the application publicly deployed?

No. The web app, API and database were run on localhost for the recorded verification. Only the documentation site is public, on GitHub Pages: <https://healthcarefund.github.io/carefund-platform/>. The contracts are on Stellar Testnet. No web hosting deployment is claimed ([submission-pack.md](./submission-pack.md), Deployment).

## Why is it only on Testnet?

It is a prototype with no independent audit, an unresolved dispute-timeout gap, and no admin authentication for dispute resolution. Moving to Mainnet would put real value at risk before those are addressed. No Mainnet plans or dates are stated here.

## Has it passed an independent audit?

No. There has been no third-party smart-contract or application audit. Verification is the project's own tests, a self-conducted financial safety review, dependency scans, and Testnet runs. The 2026-10-13 "external reviewer simulation" in this repository is a simulated review written for the project, not an independent audit. See [SECURITY.md](../SECURITY.md).

## What is already working versus planned?

Working and evidenced:

- Both contracts deployed on Testnet and exercised through settlement, requested expiry, funded expiry with refund, dispute with admin refund, and cancellation ([evidence](../evidence/testnet-2026-10-09-block3b.md)).
- Agreement #8 created, funded, attested and then settled by a third-party account through the browser with Freighter, with 8,000,000 stroops to the provider and 2,000,000 returned to the sponsor.
- A signature-rejection case that submitted nothing.
- Go API with idempotency and reconciliation, TypeScript SDK, Next.js app.
- 117 contract tests (90 and 27).

Planned, not implemented (open GitHub issues #21 to #28): provider discovery for sponsor intents, reconciliation metrics, dead-letter handling, an authenticated admin workflow for dispute resolution, SDK retry helpers, a manual accessibility review, passkey signing, and an interactive playground.

Not planned or not addressed anywhere: a dispute timeout, admin key rotation, rechecking attester status after creation, milestones, and multi-attester thresholds (the last is listed as post-approval ideas in [ROADMAP.md](../ROADMAP.md)).

## Is there a demo video?

No. The video is marked pending in [demo.md](./demo.md), and no recording has been verified.
