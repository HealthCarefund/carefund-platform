# Database migrations

Migrations for the CareFund API live in `migrations/` as paired
`NNNN_description.up.sql` / `NNNN_description.down.sql` files.

No migrations exist yet — this directory is scaffolding for Phase 7.

The API targets the isolated PostgreSQL 18.6 instance described in the
root `docker-compose.yml` (service `postgres`, container
`carefund-pg18`, host port `5439`). It must never point at the host's
PostgreSQL 16 installation.
