# Known Limitations

CareFund maintains strict honesty regarding current software limitations. The following architectural boundaries and incomplete verification paths are deliberately documented:

## 1. No Admin UI for Dispute Resolution
The contract function `resolve_dispute` allows the administrator to override agreement states and redirect escrowed funds. On-chain, this requires `admin.require_auth()`.

The CareFund Go API and web application currently lack an administrative authentication and session management layer. Creating an unauthenticated web form for `resolve_dispute` would expose an escrow control to any user who visits the URL. Dispute resolution UI is therefore omitted until a secure administrative authentication system is introduced.

## 2. Agreement Intents Lack Provider Listing
When a sponsor creates an agreement intent via `/sponsor/agreements/new`, an off-chain record is stored in PostgreSQL. However, there is currently no API endpoint or provider dashboard view allowing clinics to browse incoming intents. Sponsors must communicate the intent or parameters out-of-band to the provider.

## 3. Browser Wallet Interactive Signing Classification
While `@carefund/sdk` implements full Freighter wallet connection and signing logic (validated through automated Vitest test suites), execution in automated headless CLI environments cannot drive interactive browser extension popups. Human Freighter extension signing is truthfully classified as awaiting maintainer interaction, while all on-chain state machine and token movements are verified live on Testnet via Stellar CLI operator identities.

## 4. Testnet Contract Deployment Evolution
During Block 1 on 2026-09-27, the initial contract instances (`CD6NC4...` and `CCGF5Y...`) were deployed to Testnet. In Block 3B on 2026-10-09, fresh contract instances (`CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` and `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS`) were deployed from current audited WASM bytecode containing all Block 3A financial safety remediations, surplus refunds, strict deadline boundaries, and permissionless settlement. The complete lifecycle and multi-agreement pooling matrices were verified on-chain and recorded in `evidence/testnet-2026-10-09-block3b.md`.

## 5. API Authentication and Rate Limiting
The Go API does not require bearer tokens or API keys. While it inspects payload addresses against expected roles before building unsigned transactions, this is a convenience validation. The final security barrier is the on-chain contract signature check. The API also lacks per-IP or per-wallet rate limiting.

## 6. Manual Accessibility Review
Automated accessibility scans are performed during CI using `@axe-core/playwright`. While this ensures high baseline conformance with WCAG 2.1 standards, manual audits using assistive screen readers and keyboard-only navigation have not been conducted.
