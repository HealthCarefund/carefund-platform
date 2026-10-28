# CareFund Product Demo Record

## 1. Demo Overview and Scope

This document details the live product demonstration structure, environment boundaries, and evidentiary backing for CareFund on Stellar Testnet.

- **Application Environment**: Local Next.js web application (`http://localhost:3000`) communicating with local Go REST API (`http://localhost:8080`) and PostgreSQL Docker container (`postgres:18.6-bookworm` on port 5439).
- **Wallet and Network**: Freighter browser extension connected to Stellar Testnet (`Test SDF Network ; September 2015`).
- **Live Contracts**: Provider Registry (`CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`) and Care Agreement (`CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O`).
- **Demo Video**:
  PENDING MAINTAINER RECORDING

*(No fabricated demo URL is provided).*

**Status of this document.** Section 3 is a plan for a live demonstration that has not been recorded. Section 4 and the transaction hashes in it are an already completed and documented Testnet lifecycle (Agreement #8). The walkthrough in section 3 describes how the same flow would be shown live; it is not itself evidence that it ran.

**Timing note for any live run.** The dispute window opens at `care_deadline`, not at attestation. Settlement is possible only after `care_deadline + dispute_window_secs`, even if the attestation was submitted early. The deadlines entered in Step 2 must be chosen with that in mind.

---

## 2. Demarcation of Verification Modes

To adhere strictly to evidence standards, this demonstration record enforces an unambiguous separation between real-time UI interaction and immutable ledger proofs:

### LIVE DEMONSTRATION
Refers to real-time interactive walkthrough of the running web application on `http://localhost:3000`:
- Connecting the Freighter wallet extension.
- Detecting network parameters and verifying correct network status indicators.
- Stepping through the multi-role interface (Provider, Sponsor, Attester).
- Client-side transaction simulation and signature prompt handling.
- Graceful handling of user signature rejection without submitting RPC transactions.

### PREVIOUSLY VERIFIED TESTNET EVIDENCE
Refers to finalized, on-chain cryptographic transaction proofs executed on Stellar Testnet:
- Agreement #8 full browser lifecycle transactions recorded in Block 3B.
- Additional financial safety transactions (surplus refund, funded expiry refund, dispute resolution) verified across Agreements #1, #3, and #4.
- Exact ledger sequence numbers, transaction hashes, and contract event logs.

---

## 3. Real Product Walkthrough Plan

The demo execution follows the critical end-to-end CareFund protocol lifecycle:

### Step 1: Localhost Application & Network Status Check
1. Launch local stack: PostgreSQL on port 5439, Go API on port 8080, Next.js web app on port 3000.
2. Open `http://localhost:3000` in a Chromium browser with Freighter extension active.
3. Connect Freighter wallet. The header displays the active Stellar address and a network indicator pill confirming `TESTNET`.
4. *Wrong-network safeguard*: If Freighter is switched away from Testnet, the interface detects network mismatch, displays a warning banner, and halts transaction signing until switched back to Testnet and refreshed.

### Step 2: Provider Agreement Flow
1. Connect with registered provider address `GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453`.
2. Navigate to provider agreement initiation interface (`/provider/agreements/new`).
3. Enter agreement parameters:
   - Sponsor address and authorized attester address.
   - Total funding amount (e.g., 1.0 XLM) and reimbursement cap (e.g., 0.8 XLM).
   - Funding deadline, care completion deadline, and dispute window duration.
   - 32-byte cryptographic commitments representing patient and service identifiers (precomputed SHA-256 hashes; no plaintext PHI).
4. Initiate agreement creation. Freighter prompts for transaction signature (`create_agreement` on `care-agreement` contract).
5. Transaction executes and the agreement is created in the `Requested` state.

### Step 3: Sponsor Escrow Funding
1. Connect sponsor wallet `GDRS3XRDOIQ6ZOKKBWUZ3ZOXJE6XJIYWSPZCRLZSAHKRBL6JMOP3SYPH`.
2. Inspect pending agreement details and required escrow balance.
3. Click "Fund Escrow". The client-side SDK simulates the transaction against Soroban RPC, constructs the invocation for `fund`, and prompts Freighter.
4. Sponsor signs the authorization; funds are transferred into contract escrow via the Stellar Asset Contract (SAC).
5. Agreement state updates to `Funded`.

### Step 4: Attester Confirmation
1. Connect authorized attester wallet `GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT`.
2. Locate the funded agreement.
3. Submit a 32-byte attestation commitment hash (`attest_care`). The contract does not check what the hash refers to.
4. Attester signs in Freighter; the contract checks that the signer is the attester named in the agreement. (The registry bound that attester to the provider when the agreement was created. The current source also rechecks the attester's `Active` status and provider binding at attestation, but the Testnet contracts used for the recorded evidence predate that check.)
5. Agreement status updates to `CareConfirmed`. This records an authorized attester's statement; it is not independent proof that care was delivered. No dispute clock starts at this point: the dispute window opens at `care_deadline`.

### Step 5: Transaction Confirmation & Final Agreement State
1. The ledger time passes `care_deadline + dispute_window_secs` with no dispute opened.
2. Settle agreement (`settle`): executed permissionlessly by any account (including external third-party caller).
3. Contract executes stroop-precise settlement:
   - Provider receives the agreed reimbursement amount (e.g., 8,000,000 stroops).
   - Sponsor automatically receives any remaining escrow surplus (e.g., 2,000,000 stroops).
4. Agreement state reaches `Settled`. Per-agreement escrow liability returns to zero.
5. Off-chain PostgreSQL mirror reconciles finalized status via background reconciler worker.

---

## 4. Previously Verified On-Chain Evidence: Agreement #8

The full live browser lifecycle was verified on Stellar Testnet during Block 3B under Agreement #8:

| Step | Caller Address | Transaction Hash | Ledger | Verified Outcome |
|---|---|---|---|---|
| Sponsor Funding | `GDRS3XRDOIQ6ZOKKBWUZ3ZOXJE6XJIYWSPZCRLZSAHKRBL6JMOP3SYPH` | `d2197fb8d30103d126d0c71f7328c12732d57dd92d9156c2534c1417a7ee5e06` | 5106918 | 10,000,000 stroops (1 XLM) transferred to contract escrow |
| Attestation | `GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT` | `019fc50fc867eb7e1b9a71b775f82cbcb4462b16f2369fa2d1409b4ae17619f3` | 5107027 | Attestation commitment recorded; state `CareConfirmed` |
| Outsider Settlement | `GDBPGGUET2LIJEE3E7SB6GIUFSU4IW3OPDRAPCO56I6E63NH2H77CYSW` | `b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa` | 5107119 | 8M stroops to provider; 2M stroops surplus refunded to sponsor |

Final Agreement State: **Settled** (Net contract liability: 0 stroops).

### Supporting Verification References
- Complete Testnet Audit Trail: [evidence/testnet-2026-10-09-block3b.md](../evidence/testnet-2026-10-09-block3b.md)
- Central Evidence Ledger: [evidence/index.md](../evidence/index.md)
- Formal Submission Pack: [submission/submission-pack.md](./submission-pack.md)
