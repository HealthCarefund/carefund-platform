# CareFund Documentation

CareFund coordinates care agreements between sponsors, healthcare providers, and independent attesters, with funding held and settlement executed by Soroban smart contracts on the Stellar network.

This documentation book provides comprehensive guidance on CareFund's smart contract protocols, Go API backend, TypeScript SDK, Next.js web application, local environment setup, and verification evidence.

## Core Objective

In many healthcare scenarios, philanthropic sponsors or grant programs wish to fund critical medical care for beneficiaries directly, while ensuring funds are disbursed only when verified care is delivered by accredited providers. Traditional processes often suffer from opaque fund flows, administrative overhead, and delayed provider reimbursement.

CareFund provides a transparent, non-custodial coordination layer:
- **Conditional Funding**: Sponsors commit funds directly into smart contracts escrowed on Stellar.
- **Independent Attestation**: Designated third-party attesters verify that agreed care was delivered before funds can be released.
- **Autonomous Settlement**: Providers claim agreed funds on-chain once attestation is registered and required time gates pass.
- **Privacy by Design**: Sensitive patient references and clinical service descriptions are never stored on-chain or in backend databases; only 32-byte opaque cryptographic commitments are recorded.

## What CareFund Is Not

To maintain strict operational and legal integrity, CareFund explicitly defines what is out of scope:
- **Not Health Insurance**: CareFund is not an insurance policy, underwriter, or clinical risk pool.
- **Not an Electronic Health Record (EHR)**: CareFund does not store clinical histories, diagnostic imaging, or patient identifying data.
- **Not a Medical Provider**: CareFund does not practice medicine, offer consultations, or provide triage.
- **Not a Custodial Wallet**: Neither the web application nor backend API ever holds, manages, or transmits private keys.

## Repository Overview

CareFund is organized as a unified monorepo:

| Directory | Component | Description |
|---|---|---|
| `contracts/provider-registry` | Soroban Contract | On-chain registry of accredited providers and authorized attesters |
| `contracts/care-agreement` | Soroban Contract | Agreement state machine, escrow custody, time gates, and settlement |
| `packages/types` | TypeScript Package | Shared branded types and domain primitives |
| `packages/sdk` | TypeScript Package | Transaction assembly, simulation, wallet signing, and RPC monitoring |
| `apps/api` | Go Backend | PostgreSQL workflow mirror, transaction prep, and background chain reconciliation |
| `apps/web` | Next.js Application | Role-based portal for sponsors, providers, and attesters |
| `evidence/` | Verification Records | Dated audit reports, Testnet transaction proofs, and recovery tests |

## Quick Navigation

- [Why CareFund](why-carefund.md): The motivation and problem statement.
- [Scope and Roles](scope-and-roles.md): The responsibilities of sponsors, providers, attesters, and admins.
- [System Architecture](architecture.md): High-level topology and component interactions.
- [Getting Started](getting-started.md): Prerequisites and step-by-step local setup.
- [Contract Reference](contract-reference.md): Soroban interfaces, data structures, and errors.
- [API Reference](api-reference.md): Go backend REST endpoints and request schemas.
- [Testnet Evidence](testnet-evidence.md): Dated on-chain verification logs and transaction proofs.
