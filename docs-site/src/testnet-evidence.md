# Stellar Testnet Evidence

This page documents the verifiable on-chain deployments and lifecycle executions on the public Stellar Testnet.

All transactions and contract addresses listed below are verifiable on [StellarExpert](https://stellar.expert/explorer/testnet).

---

## Block 3B Live Testnet Deployment (2026-10-09)

Current deployment implementing full escrow conservation, surplus refunds, strict funding boundaries, and permissionless settlement (see `evidence/testnet-2026-10-09-block3b.md`).

| Contract | Address | WASM SHA-256 Hash | Upload / Deploy Transactions |
|---|---|---|---|
| `provider-registry` | `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS` | `30924b0d33baf349f93dc36275e492e1803ef3dc738fd67b5ce91ee94974ab52` | Upload: `da364745b4c2...`<br>Deploy: `03a34e0dd88f...` |
| `care-agreement` | `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` | `b543e9a7084ddaaf9b6377971914b5fb5c44d010c627dc5937e0a88b031f9eb5` | Upload: `5b5e6c3c1741...`<br>Deploy: `dbcfa8a6ad27...` |
| Native SAC (XLM) | `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC` | Built-in | System Derived |

### Multi-Agreement Lifecycle Verification Matrix

| Agreement | Flow Tested | Transaction Hashes | State Result | Financial Accounting |
|---|---|---|---|---|
| ID 1 | Nominal lifecycle with surplus refund | Create: `ed1ea91b...`<br>Fund: `177621f1...`<br>Attest: `6b7128dc...`<br>Settle: `9ae6dc48...` | `Settled` | F = 50M stroops (5 XLM). S = 30M stroops paid to provider; F - S = 20M stroops surplus refunded to sponsor. Contract escrow obligation = 0. |
| ID 2 | Requested expiry | Create: `525ce239...`<br>Expire: `80b818da...` | `Expired` | F = 10M stroops. Expired after funding deadline. Zero asset transfers. |
| ID 3 | Funded expiry | Create: `9d92964a...`<br>Fund: `66df4920...`<br>Expire: `4d1b15c0...` | `Expired` | F = 20M stroops. Expired after dispute window without attestation. 100% of F refunded to sponsor; 0 to provider. Contract obligation = 0. |
| ID 4 | Dispute & Admin Resolution (Refund) | Create: `943b5ad1...`<br>Fund: `0763fc28...`<br>Dispute: `cd290e0d...`<br>Resolve: `55bfa433...` | `Refunded` | F = 15M stroops. Dispute opened within window, resolved as Refund by admin. Full 15M stroops refunded to sponsor. Contract obligation = 0. |
| ID 5 | Provider cancellation | Create: `ad72796c...`<br>Cancel: `7a19745e...` | `Cancelled` | Cancelled by provider before funding. Zero asset transfers. |
| ID 3 & 4 | Multi-agreement escrow pooling | N/A (simultaneous on-chain state) | Pooled | Simultaneously funded: contract held 35M stroops (20M + 15M). Finalizing ID 3 left exactly 15M stroops for ID 4. Zero cross-agreement leakage. |

---

## Historical Block 1 Baseline (2026-09-27)

This section preserves the original deployment and verification record from Block 1 (`evidence/testnet-2026-09-27.md`):

| Contract | Address | WASM Hash | Deploy Transaction |
|---|---|---|---|
| `provider-registry` | `CCGF5Y7CYRJSCMXE7NRCAXY4CKYWY22BXWOYILFXZD32EXFHOOTLNMSG` | `51cc875c6e18...` | `43e528729f565574d896f9cb2f43525bd590412d9bfc43f4e2932c79e43d3778` |
| `care-agreement` | `CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS` | `6525a23931b7...` | `ce155a193b5efdbfd32ca9fc1e5aba93a43bb1a278f3ff4d68565ab964db80f2` |
| Native SAC (XLM) | `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC` | Built-in | System Derived |

---

## Crucial Qualifications and Outstanding Gates

1. **WASM Redeployment Gate Resolved**: In Block 3B, the updated contracts were deployed and verified with the current financial safety invariants. The historical Block 1 contracts remain on Testnet as historical reference.
2. **Signing Mechanism**: All lifecycle transactions above were submitted to the live Testnet ledger using authorized Stellar CLI operator identities. Browser extension signing with Freighter remains classified as awaiting maintainer interaction.
3. **Network Boundary**: All evidence represents Stellar Testnet. Mainnet deployment has not occurred.

