# Stellar Testnet Evidence

This page summarizes the historical on-chain deployment and lifecycle verification executed on the public Stellar Testnet on **2026-09-27**.

All transactions and contract addresses listed below are verifiable on [StellarExpert](https://stellar.expert/explorer/testnet).

## Contract Deployments

| Contract | Address | WASM Hash | Deploy Transaction |
|---|---|---|---|
| `provider-registry` | `CCGF5Y7CYRJSCMXE7NRCAXY4CKYWY22BXWOYILFXZD32EXFHOOTLNMSG` | `51cc875c6e18...` | `43e528729f565574d896f9cb2f43525bd590412d9bfc43f4e2932c79e43d3778` |
| `care-agreement` | `CD6NC44TOSO2G4RCVHULJUUHI4A52MCAYVAPNKQOLQSATWEK3DRDU2BS` | `6525a23931b7...` | `ce155a193b5efdbfd32ca9fc1e5aba93a43bb1a278f3ff4d68565ab964db80f2` |
| Native SAC (XLM) | `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC` | Built-in | System Derived |

---

## On-Chain Lifecycle Verification (Agreement ID 1)

The complete happy-path lifecycle was executed on-chain with real asset transfers (amounts in stroops, where 10,000,000 stroops = 1 XLM):

| Operation | Transaction Hash | Observed State Transition | Verifiable Result |
|---|---|---|---|
| `create_agreement` | `833b4c6e554557317c9afae3a9862321931259565a7ff2917877f062927b6951` | `Requested` | Agreement initialized with 50M stroops funding, 40M settlement |
| `fund` | `57bc0acd72ffd0c68c837f2cc7b73f3e9c205d23785a6e99680f679b2d92d5bf` | `Funded` | Real token transfer event: 5 XLM moved from sponsor to contract escrow |
| `attest_care` | `90f8f02c85fad850836663cf4cc3e686582e01c91e83d24a78cedb00d529edbf` | `CareConfirmed` | Attester commitment recorded on-chain |
| `settle` | `70d8fff005da5d05bdaa583d0a0418a5a6e90c864d9c4724f1aace47710b8438` | `Settled` | Real token transfer event: 4 XLM transferred from contract to provider; 1 XLM retained |

---

## Negative Path Verification (Simulation Rejections)

Five invalid lifecycle scenarios were submitted to Soroban RPC against the live contract state, proving the contract rejects illegal operations:

| Scenario Tested | Returned Error | Classification |
|---|---|---|
| Attester attempts `settle`, pretending to be sponsor | `Error(Contract, #2)` (Unauthorized) | VERIFIED LIVE (Simulation rejected) |
| Sponsor attempts `settle` before `care_deadline` passes | `Error(Contract, #11)` (CareDeadlineNotPassed) | VERIFIED LIVE (Simulation rejected) |
| Sponsor attempts `fund` on already `Funded` agreement | `Error(Contract, #4)` (InvalidState) | VERIFIED LIVE (Simulation rejected) |
| Provider attempts `cancel` on `CareConfirmed` agreement | `Error(Contract, #4)` (InvalidState) | VERIFIED LIVE (Simulation rejected) |
| Provider attempts `open_dispute` before dispute window opens | `Error(Contract, #12)` (DisputeWindowActive) | VERIFIED LIVE (Simulation rejected) |

---

## Crucial Qualifications and Outstanding Gates

Reviewers must note the following truthful qualifications:
1. **WASM Redeployment Gate**: The deployed `care-agreement` contract above was deployed prior to the dispute-window overflow safety fix (commit `7fdcade`). The source code includes the fix, but the deployed contract has not yet been updated. Redeploying the contract and re-verifying a fresh lifecycle is an outstanding pre-submission gate.
2. **Signing Mechanism**: Transactions were signed directly via Stellar CLI keypairs on the deployment machine, not through a live browser Freighter extension.
3. **Network Boundary**: All evidence represents Stellar Testnet. Mainnet deployment has not occurred.
