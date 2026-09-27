-- Tracks the background reconciliation loop's own cursor state (which
-- ledger it has already scanned for contract events). Purely internal
-- bookkeeping with no on-chain counterpart — not part of the approved
-- domain schema, the same way schema_migrations isn't.

CREATE TABLE reconciliation_state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
