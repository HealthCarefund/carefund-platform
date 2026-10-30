# CareFund API database

Migrations for the CareFund API live in `migrations/` as paired
`NNNN_description.up.sql` / `NNNN_description.down.sql` files.

The current schema is applied by the API at startup through
`internal/migrate.Run`. It creates `schema_migrations` and applies each
unrecorded `.up.sql` migration in ascending version order. The repository
currently contains:

- `0001_init`: providers, attesters, care agreements, attestations,
  transaction references, contract events, idempotency keys and audit records.
- `0002_agreement_intents`: off-chain agreement proposal records.
- `0003_reconciliation_state`: the durable reconciliation cursor and retention
  gap record.

The API targets the isolated PostgreSQL 18.6 instance described in the
root `docker-compose.yml` (service `postgres`, container
`carefund-pg18`, host port `5439`). It must never point at the host's
PostgreSQL installation.

For a local API run from a clean checkout:

```bash
docker compose up -d
cp apps/api/.env.example apps/api/.env
set -a
source apps/api/.env
set +a
cd apps/api
go run ./cmd/api
```

The API logs the migration versions applied on first start. Later starts are
no-ops once those versions are recorded in `schema_migrations`.
