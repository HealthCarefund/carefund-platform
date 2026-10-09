# Getting Started

This guide explains how to set up, configure, and run the complete CareFund monorepo locally.

## Prerequisites

Before starting, confirm your development environment matches the pinned versions documented in `TOOLCHAIN.md`:

- **Go**: 1.27.1
- **Node.js**: 24.21.0
- **pnpm**: 12.5.1
- **Rust**: 1.98.1 with target `wasm32v1-none`
- **PostgreSQL**: 18.6 (run via Docker)
- **Stellar CLI**: 27.0.0
- **Docker & Docker Compose**: For running the isolated PostgreSQL container

Verify your local toolchain using the repository verification script:

```bash
./scripts/check-toolchain.sh
```

---

## Step-by-Step Setup

### 1. Launch Isolated Database
CareFund uses a dedicated PostgreSQL 18.6 container isolated from any host database:

```bash
docker compose up -d
```

This starts `carefund-pg18` listening on host port **5439**, with database `carefund` and user `carefund`.

### 2. Install Node.js Dependencies
Install all workspace packages and frontend dependencies:

```bash
pnpm install
```

### 3. Configure the Go API
Navigate to `apps/api` and copy the example environment file:

```bash
cp apps/api/.env.example apps/api/.env
```

Ensure `DATABASE_URL` points to `postgres://carefund:carefund_dev@localhost:5439/carefund?sslmode=disable`.

### 4. Configure the Next.js Frontend
Create `apps/web/.env.local` with the following public parameters:

```bash
NEXT_PUBLIC_STELLAR_NETWORK=TESTNET
NEXT_PUBLIC_STELLAR_RPC_URL=https://soroban-testnet.stellar.org
NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE=Test SDF Network ; September 2015
NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID=CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS
NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID=CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O
NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID=CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

Note: The contract IDs above reflect the verified Block 3B Testnet deployment recorded in `evidence/testnet-2026-10-09-block3b.md` (historical Block 1 baseline recorded in `evidence/testnet-2026-09-27.md`).

### 5. Start the Go API Server
Run the API daemon:

```bash
cd apps/api
go run ./cmd/api
```

The API starts on `http://localhost:8080`. Verify health:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### 6. Start the Next.js Web Application
In another terminal, run the web development server:

```bash
pnpm --filter @carefund/web run dev
```

The application is accessible at `http://localhost:3000`.
