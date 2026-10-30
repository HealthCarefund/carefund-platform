# Getting Started

This guide explains how to set up, configure, and run the complete CareFund monorepo locally.

## Prerequisites

Before starting, confirm your development environment matches the pinned versions documented in `TOOLCHAIN.md`:

- **Go**: 1.27.2
- **Node.js**: 24.21.0
- **pnpm**: 12.5.1
- **Rust**: 1.98.1 with target `wasm32v1-none`
- **PostgreSQL**: 18.6 (run via Docker)
- **Stellar CLI**: 28.1.0 on the host / 27.0.0 in CI
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
Copy the example environment file, then export its values into the shell that will run the API:

```bash
cp apps/api/.env.example apps/api/.env
set -a
source apps/api/.env
set +a
```

The Go API reads process environment variables with `os.LookupEnv`; it does not parse `.env` files automatically. The example points to the isolated PostgreSQL container and the verified Block 3B Testnet contract instances.

### 4. Configure the Next.js Frontend
Copy the public example configuration:

```bash
cp apps/web/.env.example apps/web/.env.local
```

The values are public Testnet and localhost parameters. The contract IDs reflect the verified Block 3B Testnet deployment recorded in `evidence/testnet-2026-10-09-block3b.md`; the Block 1 deployment remains historical.

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
