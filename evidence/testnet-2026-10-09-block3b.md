# Testnet Evidence: 2026-10-09 (Block 3B Verification)

All addresses and transaction identifiers below are public Stellar Testnet data.
No private keys or secrets are recorded here.
Every transaction hash and contract identifier can be independently verified on the public Stellar Testnet explorer:
- `https://stellar.expert/explorer/testnet/tx/<hash>`
- `https://stellar.expert/explorer/testnet/contract/<id>`

## 1. Toolchain and Environment

- Stellar CLI: 28.1.0 (host, Protocol 29 compatible) / 27.0.0 (CI minimum)
- Network: Stellar Testnet (Protocol 29, Captive Core 29.0.0)
- Network Passphrase: `Test SDF Network ; September 2015`
- Soroban RPC URL: `https://soroban-testnet.stellar.org`
- Rust Toolchain: `rustc 1.98.1 (c0525b844 2026-09-08)`
- Go Toolchain: `go version go1.27.1 linux/amd64`
- Node.js: `v24.21.0`
- pnpm: `12.5.1`
- PostgreSQL: `18.6` (Docker container `carefund-pg18` on port 5439)

## 2. Testnet Actor Identities

Dedicated identities generated and funded with 10,000 XLM each via Friendbot:

| Role | Address | Local Identity | Initial Balance |
|---|---|---|---|
| Admin / Deployer | `GBFJSFTEQSGEQPRRC6G3DSURWPLLA6RVOPF4LN5G4MIA6YLPQSS532AY` | `carefund-admin` | 10,000.0000000 XLM |
| Provider | `GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453` | `carefund-provider` | 10,000.0000000 XLM |
| Attester | `GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT` | `carefund-attester` | 10,000.0000000 XLM |
| Sponsor | `GDRS3XRDOIQ6ZOKKBWUZ3ZOXJE6XJIYWSPZCRLZSAHKRBL6JMOP3SYPH` | `carefund-sponsor` | 10,000.0000000 XLM |
| Outsider / Fee-Payer | `GDBPGGUET2LIJEE3E7SB6GIUFSU4IW3OPDRAPCO56I6E63NH2H77CYSW` | `carefund-outsider` | 10,000.0000000 XLM |

No private keys are stored in this repository. All signing was performed via local identity keystores.

## 3. Contract Binaries and Deployments

### Provider Registry
- Source: `contracts/provider-registry`
- WASM File: `target/wasm32v1-none/release/provider_registry.wasm` (8,504 bytes)
- SHA-256 Hash: `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52`
- WASM Upload Tx: `da364745b4c2a5f66b5dd481b3f7e0fd6e0ea4cda86fbeca4cd25b47bb351183`
- Contract Deployment Tx: `03a34e0dd88f9df39a58a870395833e3b9006cac6fa6fb5996eb03f3cb9f7132`
- Contract ID: `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`
- Status: VERIFIED on Testnet

### Care Agreement
- Source: `contracts/care-agreement`
- WASM File: `target/wasm32v1-none/release/care_agreement.wasm` (17,075 bytes)
- SHA-256 Hash: `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5`
- WASM Upload Tx: `5b5e6c3c17412fee7890e3325d4c956d758dbc0e72ce14e8d4494b814af483d1`
- Contract Deployment Tx: `dbcfa8a6ad27b78a3a0631b6041b744ba68f90320371d1a628f50949c630d7da`
- Contract ID: `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O`
- Status: VERIFIED on Testnet

### Settlement Asset
- Asset: Native XLM Stellar Asset Contract (SAC)
- Contract ID: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`
- Status: VERIFIED

## 4. Contract Initialization and Registration

| Operation | Contract | Transaction Hash | Result |
|---|---|---|---|
| Initialize Admin | Provider Registry | `d483882154364251dc302ff99f81c7199a06636e21b8a52a18b47e40ed386bd1` | VERIFIED |
| Register Provider | Provider Registry | `43d89465d197462cb4d78fab8518a62b6fdbfbf46ce604a641c590efa0b96f35` | VERIFIED |
| Register Attester | Provider Registry | `a5047fe2f1b97da98efce3dbf157b4db54085c350ab1a1d6c1dabfa8e3f2f5b8` | VERIFIED |
| Initialize Care Agreement | Care Agreement | `74b91191ab3cda9d273dcd3b19cbe6ddeddfc288e3cb9b3e013fda3be3805601` | VERIFIED |

On-chain configuration readbacks:
- `is_provider_active(carefund-provider)`: `true`
- `check_attester(carefund-attester, carefund-provider)`: `true`
- `get_admin()`: `GBFJSFTEQSGEQPRRC6G3DSURWPLLA6RVOPF4LN5G4MIA6YLPQSS532AY`
- `get_provider_registry()`: `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`
- `get_settlement_asset()`: `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC`

## 5. Lifecycle and Invariant Verification Matrix

### Case 1: Nominal Lifecycle with Permissionless Settlement and Surplus Refund (Agreement 1)
- Parameters: Funding = 50,000,000 stroops (5 XLM), Settlement = 30,000,000 stroops (3 XLM), Expected Surplus = 20,000,000 stroops (2 XLM).
- Create Agreement: Tx `ed1ea91bc887b7503324873eb07e8c85fff1d08b70faed0880d7ca33bef100c6` (State: Requested).
- Fund Agreement: Tx `177621f1764c032306eb3171da836b3626e2be9bad9905a3eb5a42126163a988` (State: Funded).
  - Contract SAC Balance: 50,000,000 stroops.
- Attest Care: Tx `6b7128dcb83038887de3566e5ce2b5d4e3671b81c8a4254fcd28f95305efd17a` (State: CareConfirmed).
- Negative Checks:
  - Settle before care deadline: Simulation returns `Error(Contract, #11)` (`CareDeadlineNotPassed`).
  - Settle during active dispute window: Simulation returns `Error(Contract, #12)` (`DisputeWindowActive`).
  - Expire on CareConfirmed agreement: Simulation returns `Error(Contract, #4)` (`InvalidState`).
- Permissionless Settle: Tx `9ae6dc480e876a32a18e5b0d12ce5a3187010c01ce319d0ac1e035589845285c` invoked by outsider `carefund-outsider` (State: Settled).
- Financial Reconciliation:
  - Provider received: exactly 30,000,000 stroops.
  - Sponsor surplus refund received: exactly 20,000,000 stroops.
  - Contract retained balance: exactly 0 stroops. Escrow conservation is zero net liability.
  - Double settlement attempt: Simulation returns `Error(Contract, #4)` (`InvalidState`).

### Case 2: Requested Expiry without Funding (Agreement 2)
- Parameters: Funding = 10,000,000 stroops, unfunded.
- Create Agreement: Tx `525ce239e41ae157fdf08323f96301f4de46b27bea33b0a814ba8bfbc1545b4f` (State: Requested).
- Negative Check: Expire before funding deadline returns `Error(Contract, #9)` (`FundingDeadlineNotReached`).
- Expire: Tx `80b818dae4e20d325cb98c3c1e7da3d3bd18763b2d29a754ec8619342491dc08` (State: Expired).
- Token movements: Exactly 0 stroops moved. Zero escrow liability.

### Case 3: Funded Expiry with Full Escrow Refund (Agreement 3)
- Parameters: Funding = 20,000,000 stroops (2 XLM), unverified care.
- Create Agreement: Tx `9d92964ae3a286f2a29d7b3740a4e512d800da73988cfe6033f57a2ea9698669` (State: Requested).
- Fund Agreement: Tx `66df4920d709ba014ba705fdebd6a39af891b2c515a0d67b2fabbcd81939832e` (State: Funded).
  - Contract SAC Balance: 20,000,000 stroops.
- Expire: Tx `4d1b15c0d88c09f08fbcb2f7999281f7071ed03267e2351a80accd7c72839056` (State: Expired).
- Financial Reconciliation:
  - Sponsor refunded: exactly 20,000,000 stroops (100% of deposited escrow).
  - Provider received: 0 stroops.
  - Escrow conservation: zero net liability.

### Case 4: Dispute and Administrative Resolution (Agreement 4)
- Parameters: Funding = 15,000,000 stroops (1.5 XLM).
- Create Agreement: Tx `943b5ad1cdf4be07630a4ba8683782e2d8ba03d70cb4c91404335198cf5fe376`.
- Fund Agreement: Tx `0763fc283123ae049f339cb94ea4d6d50ae76eccc6635bffc9bdd86bd16698db`.
- Open Dispute: Tx `cd290e0df4c46e1e010614a98e0d7bfe3d6abd587240f0763c2f13a8a6eab578` (State: Disputed).
- Negative Checks:
  - Settle on Disputed state: Simulation returns `Error(Contract, #4)` (`InvalidState`).
  - Expire on Disputed state: Simulation returns `Error(Contract, #4)` (`InvalidState`).
  - Non-admin resolve dispute: Simulation returns `Error(Contract, #2)` (`Unauthorized`).
- Administrative Resolution (Refund): Tx `55bfa433069d4c44e4d40748531e339b0788449f165f250f395b6bff0ccf7972` (State: Refunded).
- Financial Reconciliation:
  - Sponsor refunded: exactly 15,000,000 stroops (100% of deposited escrow).
  - Contract balance: 0 stroops.

### Case 5: Escrow Pooling Isolation
- Agreements 3 and 4 were concurrently funded in the shared contract.
- Joint contract balance observed: 35,000,000 stroops (20M from Agreement 3 + 15M from Agreement 4).
- When Agreement 3 expired, it withdrew exactly 20,000,000 stroops, leaving exactly 15,000,000 stroops in the contract account for Agreement 4.
- When Agreement 4 was refunded, exactly 15,000,000 stroops were transferred, leaving 0 stroops.
- Verifies that individual agreement escrow accounting does not corrupt pooled contract assets.

### Case 6: Provider Cancellation (Agreement 5)
- Create Agreement: Tx `ad72796c9c646efdbb4bdf9892cfa7ea0ef71f2ba9df5f573c72688ca0fa7f13` (State: Requested).
- Cancel Agreement: Tx `7a19745ede040c0ea51d55a9a2a8c3badbde8c341decb1b4cc9cae3bbadaf071` (State: Cancelled).
- Token movements: 0 stroops.

### Case 7: API Test Harness Agreement (Agreement 6)
- Create Agreement: Tx `362482580251a046ce528bfebed5b3c5751c99bea333311228d17611a42f9616` (State: Requested, 1-year deadline).
- Dedicated live record for backend transaction preparation tests (`TestPrepareTransaction_FundLiveTestnet`).

## 6. API, Persistence, and Background Reconciliation Verification

- Test Suite: All Go API tests passing (`go test -p 1 -count=1 ./...`).
- Integration tests updated in `apps/api/internal/api/transactionlookup_test.go` and `apps/api/internal/api/transactions_test.go` with fresh on-chain contract IDs and live transaction hashes.
- Background Reconciliation:
  - Database record inserted with status `submitted` for transaction `9ae6dc480e876a32a18e5b0d12ce5a3187010c01ce319d0ac1e035589845285c`.
  - Reconciliation worker queried Soroban RPC and transitioned the record to `confirmed` at ledger 5104998.
- Process Restart:
  - Go API daemon restarted with live Testnet and PostgreSQL database.
  - Verified no regression: status remained `confirmed`, ingestion resumed at cursor ledger 5105201 without duplicate events.

## 7. Storage Archival and Protocol 23+ Restoration

- Under Soroban Protocol 23+, entries in persistent storage that exceed their TTL become archived.
- Automatic Restoration: When interacting via `InvokeHostFunctionOp`, simulation detects archived entries and populates `restorePreamble`. The submission automatically restores archived state before function execution.
- No permanent fund loss occurs simply because an entry becomes archived.

## 8. Frontend and Wallet Integration Boundary (Gate E: VERIFIED)

- Generated TypeScript contract bindings: zero diff in `packages/contracts/src/index.ts`.
- SDK Test Suite: 80 unit and builder tests passing (`pnpm --filter @carefund/sdk run test`).
- Web Unit Tests: 11 unit tests passing (`pnpm --filter @carefund/web run test`), including signature rejection without submission.
- Web E2E Test Suite: 19 Playwright tests passing (`pnpm --filter @carefund/web run test:e2e`).
- Web Next.js Server: Runs cleanly on port 3000.

### 8.1 Signature Rejection Test (PASS)
- Action: User initiated an operation requiring wallet authorization and rejected the transaction popup in the Freighter browser extension.
- UI Status: **Failed**
- Displayed Error Message: `Wallet declined to sign the transaction.`
- Transaction Hash: None (no transaction hash generated or submitted).
- Lifecycle Guard: The transaction flow remained strictly in the Failed state with zero progression to `submitting`, `submitted`, or `confirmed`, and no submission call was made to Soroban RPC.
- Regression Test: Verified deterministically via unit test `apps/web/lib/transaction-rejection.test.ts`.

### 8.2 Agreement #7 Reconciliation Defect, Root Cause, Remediation, and Replay
- **Defect**: When Agreement #7 was created via Freighter wallet on Testnet (tx `a46300429dc1d091a8e722e4b143129121c846a0c8b39dbf4b6ee4411ec1a264` at ledger 5106190), background event reconciliation failed with PostgreSQL `SQLSTATE 23503` (foreign key violation on `contract_events_agreement_id_fkey` referencing `care_agreements(agreement_id)`). The reconciler logged the error and swallowed it, advancing the ledger cursor past ledger 5106190 without indexing Agreement #7. Furthermore, the `providers` table was empty because provider registry events were not fetching and storing provider records, causing `/api/v1/providers/{wallet}/agreements` to return 404.
- **Root Cause**: The background reconciler operated event-first rather than entity-first; it assumed rows in `care_agreements` were already seeded locally before events were ingested. In addition, no decoders existed in Go to fetch and decode full on-chain contract storage objects via simulation, and batch cursor advancement did not verify that all events and entities in the batch were ingested successfully.
- **Remediation**:
  1. *Contract Entity Decoders*: Implemented `sorobanenc.DecodeAgreement`, `sorobanenc.DecodeProvider`, and `sorobanenc.DecodeAttester` to unmarshal simulated contract state from Soroban RPC into local database models.
  2. *Entity-First Reconciler*: Modified `storeEvent` in `apps/api/internal/reconcile/events.go` to fetch and upsert referenced entities (`care_agreements`, `providers`, `attesters`) before inserting contract events.
  3. *Cursor Protection*: Added an `allSuccessful` check to prevent cursor advancement whenever any event or entity in a batch fails to ingest.
  4. *On-Demand API Fallback*: Added `ensureProvider` and `fetchOnChainAgreement` to REST endpoints to query and mirror on-chain state if a database row is missing.
- **Replay Evidence**: The reconciler replayed events starting at ledger 5106190 without manual SQL intervention. Agreement #7 was automatically ingested from Testnet into `care_agreements` in state `Requested`, its `agr_cre` event was inserted into `contract_events`, and all provider/agreement endpoints resolved cleanly with HTTP 200.

### 8.3 Agreement #8 Full Browser Lifecycle Verification (PASS)
Agreement #8 was executed entirely through the browser application with client-side Freighter wallet signing on Stellar Testnet:
- **Agreement Creation**: Created via provider portal with Freighter signature.
- **Sponsor Funding Transaction**:
  - Hash: `d2197fb8d30103d126d0c71f7328c12732d57dd92d9156c2534c1417a7ee5e06`
  - Ledger: `5106918`
  - Escrow Transfer: 10,000,000 stroops (1 XLM) transferred to contract escrow.
  - State: Transitioned to `Funded`.
- **Attestation Transaction**:
  - Hash: `019fc50fc867eb7e1b9a71b775f82cbcb4462b16f2369fa2d1409b4ae17619f3`
  - Ledger: `5107027`
  - Signed by attester wallet: clinical procedure completion attested.
  - State: Transitioned to `CareConfirmed`.
- **Permissionless Outsider Settlement Transaction**:
  - Hash: `b74947224324d11710d21aac3c36b46684af6fe0a7810aa425f6a841968e73fa`
  - Ledger: `5107119`
  - Executed by third-party caller after dispute window elapsed.
  - Asset Disbursement: 8,000,000 stroops disbursed to provider wallet; 2,000,000 stroops surplus refunded to sponsor wallet.
  - Escrow Liability: Returned to 0 stroops.
- **Final UI State**: **Settled**
  - Next.js web application displays Agreement #8 in `Settled` state.
  - Verified across database `care_agreements` and `contract_events`.

## 9. Summary Verdict

All Block 3B verification gates (A, B, C, D, E, F, G) have been executed against genuine Stellar Testnet contracts. All successful lifecycle actions were executed as real Testnet transactions with verified on-chain asset transfers and complete invariant enforcement, while nine negative paths were verified through live RPC simulation rejection. Interactive browser signing with the Freighter extension (Gate E) is fully verified and documented.
