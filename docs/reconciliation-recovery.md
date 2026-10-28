# Reconciliation-gap recovery

`apps/api/cmd/reconciliation-recovery` is a local maintenance command for inspecting and acknowledging an event-retention gap. It is not registered as an API route, does not submit Stellar transactions, and does not edit PostgreSQL rows directly.

The command uses the same validated environment configuration as the API. The database must already have the application migrations applied; the command does not run migrations so `inspect` remains read-only.

```bash
cd apps/api
go run ./cmd/reconciliation-recovery inspect
go run ./cmd/reconciliation-recovery acknowledge \
  --confirm-gap <gap-id-from-inspect> \
  --resume-ledger <proposed-resume-ledger-from-inspect> \
  --yes
```

`inspect` reports the durable event cursor, the original gap recorded at detection, the current RPC retention boundary, the complete range that would be accepted by a resume decision, whether reconciliation is blocked, and prior recovery decisions. It labels historical status as retention-based and unverified; it never claims that event-history completeness was independently established. A malformed gap is displayed as blocked and cannot be acknowledged.

The acknowledgment command prints the original gap, current retention boundary, complete accepted range, and proposed resume ledger before committing. It requires the exact SHA-256 gap ID, a checked `uint32` resume ledger, and `--yes`. The command re-reads the gap and cursor, rechecks RPC health, and commits the approved cursor, acknowledgment, and append-only `audit_records` entry in one PostgreSQL transaction. If the retention boundary changes after inspection, confirmation is rejected. The audit metadata records the original range, complete accepted range, current boundary, approved resume ledger, operating-system user and host, and that historical events were not recovered.

The actual operating boundary is the host account and database credential used to run the command. Anyone who can read the application environment and connect with write access to the CareFund database can invoke it; the command does not add passwords, recovery secrets, or custom authentication. Operators must restrict those credentials and shell access through the deployment's existing operating-system and database controls.

Acknowledgment only permits ingestion to resume from retained history. It never claims that events in the missing range were recovered and never changes smart-contract state, agreement balances, transaction outcomes, or live deployment state.
