# Proposal: Recovering from Stuck Disputes

Status: proposal only. Nothing in this document is implemented or deployed. It needs maintainer review before any contract change.

Related: [ARCHITECTURE.md](../ARCHITECTURE.md), [trust model and evidence](./trust-and-security-evidence.md), [financial specification](./financial-behavior-specification.md).

## 1. The problem

When a sponsor or provider calls `open_dispute`, the agreement moves to `Disputed`. From there only `resolve_dispute` can move it, and only the care-agreement admin can call that. `settle` and `expire` both reject a `Disputed` agreement. There is no timeout.

So a `Disputed` agreement stays locked until the admin acts. Two different failures matter, and they need different answers.

| Failure | What it is | Does `transfer_admin` help? |
|---|---|---|
| Planned handover | The admin wants to hand the role to a successor, or an organization changes its signer. The current admin key still works. | Yes. This is what `transfer_admin` is for. |
| Temporary unavailability | The admin is slow or offline but the key exists. | No code needed. Funds stay safe and wait. Provider and sponsor are both delayed. |
| Lost or compromised key | The admin key is gone, or cannot be trusted. | **No.** `transfer_admin` needs the current admin's signature, so it cannot be used once that key is lost. |

Rotation is therefore a precaution that has to be used before a key is lost. It does not recover disputed funds after a loss. The remaining risk of indefinite lockup stays.

## 2. What the deployed contracts allow

- Neither deployed contract has `transfer_admin` or any upgrade function. The deployed care-agreement WASM (`b543e9a7...`) cannot be changed, so any contract-level fix reaches only newly deployed instances. Disputed agreements in an old instance stay under that instance's admin.
- The admin on Testnet is a single Stellar account. Whether a classic Stellar account can be given extra signers after the fact depends on that account's own master key, so this only helps while the key still exists.

## 3. Options

### A. Rotation only (this change)

What it gives: planned handover without redeploying.
What it does not give: any answer to a lost key.
Applies to: new deployments only.

### B. Make the admin a multi-signature account (no contract change)

The admin address is just an address. If it is a Stellar account with several signers and a threshold, losing one key does not lose the role, and no CareFund code changes.

- Works for new deployments by choosing such an address at `initialize`.
- Works for an existing deployment only if the current admin account is configured with extra signers while its key still exists.
- Trust added: the signer set. Compromise of enough signers is equivalent to compromise of the admin.
- Verified with real signatures (not a live network — see the limitation below): Soroban authorization for a classic account checks only that account's **medium** threshold, regardless of what the contract call does. A 2-of-3 account (three signers, weight 1 each, medium/high threshold 2) behaves as expected — one signer alone cannot authorize anything, losing one signer still leaves a usable 2-of-3, and an unknown key cannot fill the gap. One non-obvious requirement found while testing: the account's own identity key must not be one of the three signers, or its explicit signer entry is shadowed by the (disabled) master-key weight. Full results, what was and was not exercised against a real network, and the financial-safety caveats: [multisig admin verification](../evidence/multisig-admin-verification-2026-10-28.md).
- Cost: operational discipline, no code.

### C. A stale-dispute fallback that resumes, never pays (contract change)

After a fixed period with no admin action, anyone can call a new function that returns a `Disputed` agreement to the state it came from (`Funded` or `CareConfirmed`). It moves no funds itself. From there the ordinary rules apply:

- Attested care (`CareConfirmed`): after the window, `settle` pays the provider `S` and the sponsor `F - S`.
- Unattested funded agreement (`Funded`): after the window, `expire` refunds the sponsor `F`.

The effect is that a dispute delays an outcome the rules could have produced anyway, but resumption is not a neutral dispute-resolution mechanism. It does not itself choose a payment destination, but automatically returning a disputed `CareConfirmed` agreement to its prior state and letting `settle` run can still benefit a provider whose earlier attestation was false: if no admin adjudicates the dispute before the timeout, the timeout lets that disputed agreement settle in the provider's favor despite the sponsor's objection.

Why this is safer than an automatic sponsor refund:

- An automatic refund would let a sponsor receive care, dispute, wait, and recover the payment. Resume cannot do that, because an attested agreement ends in provider payment.
- No address is chosen by the caller. Destinations are still fixed at creation.
- It does not need a recovery key or any new trusted party.

Its cost, stated plainly: if the admin is gone, disputes stop protecting the sponsor against a false attestation. A sponsor who disputes an attested agreement will, after the timeout, see the provider paid. The timeout therefore has to be long enough for a realistic admin response (weeks, not days). The original dispute window is not a protection that survives the fallback: once that window has elapsed, resume is permissionless, so there is no further opportunity for on-chain adjudication before settlement runs. A sponsor who opened a dispute specifically to get adjudication can lose that effective protection entirely once the timeout passes with no admin action.

Requirements and open questions:

- A timeout value. It could be a constant, an `initialize` argument, or a per-agreement field. A per-agreement field changes the `Agreement` storage layout, so it is a new-deployment-only change. A contract-wide value set at `initialize` is simpler.
- The disputed agreement already records `dispute_opened_at`, so no new timestamp is needed. The sum `dispute_opened_at + timeout` must use checked arithmetic.
- The admin must still be able to resolve before the timeout, and after resume the admin cannot resolve a non-disputed agreement (existing behavior).
- A party could re-dispute only if resumption happens inside the original dispute window. After the window, no new dispute can be opened.

### D. A recovery key with delayed authority (contract change)

A second address, set at `initialize`, may call `resolve_dispute` only after the timeout.

- It can choose between pay and refund, so it carries the same power as the admin after a delay.
- It adds a second trusted party and a new way to collude or be compromised.
- It answers lost keys directly, but with a bigger trust assumption than C.

Not recommended as a first step. If chosen, the recovery address should be a multi-signature account, which is option B again.

### E. Automatic refund after a timeout

Rejected. A dishonest sponsor could dispute after receiving care, wait, and take the money back. It also overrides an attestation that the contract accepted.

### F. A per-agreement arbiter

Each agreement names its own resolver instead of the global admin. This spreads the risk but changes the trust model (who picks the arbiter, and does the sponsor agree) and adds fields to `Agreement`. It is a larger design and is not proposed now.

## 4. Maintainer decision

- **Rotation (A) is accepted for merging**, subject to CI verification. `transfer_admin` ships for new deployments so that planned handovers do not require redeploying.
- **Multi-signature admin (B) is the next security task.** It is operational and needs no contract change, but how Soroban authorizes a classic account's signers and thresholds must be verified on a local network before it is relied on.
- **Stale-dispute resume (C) is deferred.** No timeout has been approved. It changes dispute semantics in the way described in sections 1 and the updated discussion above, and ships only with explicit maintainer approval and only in a new deployment.
- **Automatic sponsor refund after a timeout (E) remains rejected**, for the reasons already given.
- **No recovery mechanism in this document is implemented or deployed.** Options B, C, D, and F remain proposals for future work, not shipped behavior, and must not be described as implemented or deployed until they actually are.

What this does not solve: an admin who is present but dishonest. That remains a trust assumption of the design.

## 5. How a fallback would be tested

For option C, with a real registry and a real token contract and several agreements at once:

- Three or more agreements in different states (attested and disputed, unattested and disputed, one undisputed) in the same contract, with token balances checked before and after.
- Resume before the timeout is rejected, including at the last second before it; resume at the first allowed second works.
- After resume of an attested agreement, `settle` pays the provider `S` and the sponsor `F - S`, and the contract balance returns to what the other agreements still hold.
- After resume of an unattested agreement, `expire` refunds `F` and nothing goes to the provider.
- Admin resolution before the timeout still works for each of the three outcomes, and resume afterwards is rejected.
- Resuming one agreement does not change another's state or balance.
- Overflow of `dispute_opened_at + timeout` is rejected at creation or initialization, not at resume.
- A caller who is neither party can trigger resume and gains nothing from the choice of caller.

For option B, a local test network with a multi-signature account as admin, checking that the intended signer threshold is required and that a missing signature fails.

## 6. Status of open questions

Section 4 records the maintainer decision reached so far: rotation (A) proceeds, multi-signature admin (B) is the next task to verify, and stale-dispute resume (C) is deferred pending explicit approval. The questions below remain open for when C is reconsidered:

- Is option C acceptable given that it weakens dispute protection when the admin is absent, as described above?
- If accepted, what timeout, and set where?
- Is a fresh deployment, with migration of any live agreements, acceptable for contract-level changes?
