# Known Limitations

CareFund maintains strict honesty regarding current software limitations. The following architectural boundaries and incomplete verification paths are deliberately documented:

## 1. No Admin UI for Dispute Resolution
The contract function `resolve_dispute` allows the administrator to override agreement states and redirect escrowed funds. On-chain, this requires `admin.require_auth()`.

The CareFund Go API and web application currently lack an administrative authentication and session management layer. Creating an unauthenticated web form for `resolve_dispute` would expose an escrow control to any user who visits the URL. Dispute resolution UI is therefore omitted until a secure administrative authentication system is introduced.

## 2. Agreement Intents Lack Provider Listing
When a sponsor creates an agreement intent via `/sponsor/agreements/new`, an off-chain record is stored in PostgreSQL. However, there is currently no API endpoint or provider dashboard view allowing clinics to browse incoming intents. Sponsors must communicate the intent or parameters out-of-band to the provider.

## 3. Browser Wallet Live Verification Pending
While `@carefund/sdk` implements full Freighter wallet connection and signing logic, the historical Testnet evidence was generated using the Stellar CLI and local key identities. Signing the complete four-step lifecycle in a live browser session with the Freighter extension installed remains an unverified pre-submission gate.

## 4. Deployed Contract Lacks Dispute Window Fix
Commit `7fdcade` introduced an arithmetic overflow guard in `contracts/care-agreement/src/lib.rs` preventing invalid combinations of `care_deadline` and `dispute_window_secs`. The unit test suite verifies this fix. However, the historical Testnet deployment (`CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS`) was created before this commit was compiled. Rebuilding and redeploying the WASM to Testnet is a mandatory pre-submission gate.

## 5. API Authentication and Rate Limiting
The Go API does not require bearer tokens or API keys. While it inspects payload addresses against expected roles before building unsigned transactions, this is a convenience validation. The final security barrier is the on-chain contract signature check. The API also lacks per-IP or per-wallet rate limiting.

## 6. Manual Accessibility Review
Automated accessibility scans are performed during CI using `@axe-core/playwright`. While this ensures high baseline conformance with WCAG 2.1 standards, manual audits using assistive screen readers and keyboard-only navigation have not been conducted.
