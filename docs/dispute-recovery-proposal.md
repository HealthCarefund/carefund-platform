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
- Open point to verify before relying on it: how Soroban authorization maps a classic account's signers and thresholds. This should be tested on a local network, not assumed.
- Cost: operational discipline, no code.

### C. A stale-dispute fallback that resumes, never pays (contract change)

After a fixed period with no admin action, anyone can call a new function that returns a `Disputed` agreement to the state it came from (`Funded` or `CareConfirmed`). It moves no funds itself. From there the ordinary rules apply:

- Attested care (`CareConfirmed`): after the window, `settle` pays the provider `S` and the sponsor `F - S`.
- Unattested funded agreement (`Funded`): after the window, `expire` refunds the sponsor `F`.

The effect is that a dispute can only delay an outcome that the rules would have produced anyway. Nobody gains from a timeout by opening a dispute.

Why this is safer than an automatic sponsor refund:

- An automatic refund would let a sponsor receive care, dispute, wait, and recover the payment. Resume cannot do that, because an attested agreement ends in provider payment.
- No address is chosen by the caller. Destinations are still fixed at creation.
- It does not need a recovery key or any new trusted party.

Its cost, stated plainly: if the admin is gone, disputes stop protecting the sponsor against a false attestation. A sponsor who disputes an attested agreement will, after the timeout, see the provider paid. The timeout therefore has to be long enough for a realistic admin response (weeks, not days), and the sponsor's protection after that point is only the dispute window itself.

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

## 4. Recommendation

The smallest defensible path, in order:

1. **Rotation (this PR).** Ship `transfer_admin` for new deployments so that planned handovers do not require redeploying.
2. **Multi-signature admin (B).** For any further Testnet deployment, initialize both contracts with multi-signature accounts. This is operational and needs no contract change. Test how Soroban authorizes such accounts first.
3. **Stale-dispute resume (C), if maintainers accept its trade-off.** It is the smallest code change that removes indefinite lockup without adding a trusted party or a payout choice. It changes dispute semantics, so it needs explicit approval and should ship only in a new deployment.

Not recommended: automatic refund (E), and a recovery key with full resolution power (D) before B and C have been weighed.

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

## 6. Decisions needed from maintainers

- Is option C acceptable given that it weakens dispute protection when the admin is absent?
- If yes, what timeout, and set where?
- Should further Testnet deployments use multi-signature admins (B)?
- Is a fresh deployment, with migration of any live agreements, acceptable for contract-level changes?
