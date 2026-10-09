# System Architecture

CareFund is architected around a strict separation of concerns: **the blockchain is the authoritative source of truth, and cryptographic keys reside exclusively in the user's browser wallet.**

```
+-------------------------------------------------------------------------+
|                               Browser Client                            |
|                                                                         |
|  +-----------------------------+        +----------------------------+  |
|  |     Next.js Application     |        |      Freighter Wallet      |  |
|  |         (apps/web)          |        |     (Browser Extension)    |  |
|  +--------------+--------------+        +--------------+-------------+  |
|                 |                                      |                |
|                 | (Unsigned XDR / API)                 | (Signs XDR)    |
+-----------------|--------------------------------------|----------------+
                  |                                      |
                  v                                      |
+----------------------------------+                     |
|           Go API Backend         |                     |
|             (apps/api)           |                     |
|                                  |                     |
|  - Validates role requirements   |                     |
|  - Assembles unsigned XDR        |                     |
|  - Simulates RPC execution       |                     |
|  - Tracks idempotency keys       |                     |
|  - Background reconciliation     |                     |
+-----------------+----------------+                     |
                  |                                      |
                  v                                      |
+----------------------------------+                     |
|       PostgreSQL 18.6 Database   |                     |
|     (Isolated Port 5439)         |                     |
|                                  |                     |
|  - Off-chain agreement mirrors   |                     |
|  - Agreement intents & audit     |                     |
|  - Idempotency reservations      |                     |
+-----------------+----------------+                     |
                  |                                      |
                  | Reconciles from chain                |
                  v                                      v
+-------------------------------------------------------------------------+
|                        Stellar Soroban Network                          |
|                                                                         |
|     +-------------------------+       +-------------------------+       |
|     |    provider-registry    | <---> |      care-agreement     |       |
|     |        (Contract)       |       |        (Contract)       |       |
|     +-------------------------+       +-------------------------+       |
|                                                    |                    |
|                                                    v                    |
|                                       +-------------------------+       |
|                                       | Stellar Asset Contract  |       |
|                                       |     (Native SAC XLM)    |       |
|                                       +-------------------------+       |
+-------------------------------------------------------------------------+
```

## Key Architectural Principles

### 1. Chain as the Authoritative Source of Truth
The PostgreSQL database maintained by `apps/api` contains only off-chain workflow metadata. It is an index and mirror designed for responsive querying, intent capture, and UI navigation.
- If a record in PostgreSQL conflicts with ledger state, the chain always wins.
- A background worker (`apps/api/internal/reconcile`) continually polls Soroban events and transaction records, updating the PostgreSQL mirror to reflect confirmed ledger realities.

### 2. Strict Wallet Signing Boundary
Neither `apps/api` nor `apps/web` ever stores, generates, or transmits private cryptographic keys.
- **Client-Side Signing**: Transactions are signed directly by the user's browser wallet (Freighter).
- **Direct RPC Submission**: Once signed, transaction envelopes are submitted from the browser directly to the Soroban RPC node.
- **Unsigned Transaction Preparation**: For complex agreement lifecycle operations (`fund`, `cancel`, `attest_care`, `open_dispute`, `expire`, `settle`, `resolve_dispute`), the Go API prepares, simulates, and returns unsigned transaction XDR to the client.
- **Client-Side Creation**: `create_agreement` is generated entirely in the client using `@carefund/sdk` without needing prior agreement IDs from the backend.

### 3. Off-Chain Metadata Isolation
PostgreSQL stores:
- Agreement intents (initial parameters requested before on-chain creation)
- Idempotency reservations preventing duplicate transaction builds
- Audit records tracking API interactions
- Cached copies of on-chain agreement state and event sequences

PostgreSQL never stores:
- Private keys or seeds
- Plaintext patient identifying information (PII)
- Plaintext medical diagnosis or treatment descriptions (PHI)

### 4. Monorepo Structure

- `contracts/provider-registry`: Soroban contract for provider registration records and attester authorization bindings.
- `contracts/care-agreement`: Soroban contract managing agreement state, deposit escrow, deadlines, and multi-signature transitions.
- `packages/types`: Shared TypeScript definitions, branded types (`StellarAddress`, `CommitmentHash`, `AgreementId`), and JSON schemas.
- `packages/sdk`: TypeScript library for Soroban transaction assembly, simulation, Freighter wallet interaction, and contract bindings.
- `apps/api`: Go HTTP service (`net/http`, `pgxpool`, `slog`) providing REST endpoints, idempotency handling, and ledger reconciliation.
- `apps/web`: Next.js 16 frontend with Tailwind CSS, React components, and Playwright end-to-end testing.
- `scripts/`: Toolchain verification, contract binding generation, and deployment helper scripts.
