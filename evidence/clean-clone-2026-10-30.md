# Clean-clone reproducibility verification: 2026-10-30

This record covers the local setup path corrected after the Stellar Project
Approval Playbook v2.0 audit. It verifies the current source after PR #44 and
does not deploy the API, database or Stellar contracts.

## Source and prerequisites

- Source baseline: `main` merge commit `6c84c42cce4e0b1ec0d05a9326cbef59d9c68452`
- Clean-clone revision: `b891bf61fc82c9db6b67aa2bdd1a791e62ad580b`
- PostgreSQL: the repository's `postgres:18.6-bookworm` Docker service,
  container `carefund-pg18`, host port `5439`
- API: Go process at `http://localhost:8080`
- Web: Next.js development process at `http://localhost:3000`

## Verified clean-clone procedure

From a fresh checkout with the pinned toolchain installed:

```bash
docker compose up -d
pnpm install --frozen-lockfile

cp apps/api/.env.example apps/api/.env
set -a
source apps/api/.env
set +a
cd apps/api
go run ./cmd/api
```

The API starts only after the copied environment is sourced because the Go
entry point reads process variables with `os.LookupEnv`; it does not parse a
`.env` file automatically. The existing isolated database reported
migrations `1:init`, `2:agreement_intents` and `3:reconciliation_state` in
`schema_migrations`, so the API logged `count: 0` on this run, as expected for
an already initialized database. A new database applies those same embedded
migrations on its first API start.

Observed responses from this run:

```text
GET /healthz -> 200 {"status":"ok"}
GET /readyz  -> 200 {"status":"ready"}
```

In a second shell from the checkout:

```bash
cp apps/web/.env.example apps/web/.env.local
pnpm --filter @carefund/web run dev
```

The web process started at `http://localhost:3000` using only public Testnet
contract parameters and the local API URL from the checked-in example; `GET /`
returned HTTP 200.

The API also reported the existing database's previously recorded, unacknowledged
RPC retention gap while the reconciliation worker ran. This is expected state
from the reused verification database and does not affect `/healthz` or
`/readyz`; it confirms the documented retention-gap safety behavior rather than
silently discarding the gap.

## Scope boundary

The verification uses the existing isolated PostgreSQL service and the
historical Block 3B Testnet RPC/contract parameters. It does not submit a
transaction, deploy a contract, or claim a public API/web deployment.
