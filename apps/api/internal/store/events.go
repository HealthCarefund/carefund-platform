package store

import (
	"context"
	"fmt"
)

// InsertContractEvent records an observed on-chain event. Ingestion is
// idempotent: the same (contract_id, tx_hash, event_index) observed twice
// (e.g. after a reconciliation re-scan) is silently a no-op, never a
// duplicate row or an error. Returns whether a new row was actually
// inserted, so callers can distinguish "already had this" from "new".
func (s *Store) InsertContractEvent(ctx context.Context, e ContractEvent) (inserted bool, err error) {
	cmd, err := s.pool.Exec(ctx, `
		INSERT INTO contract_events (contract_id, tx_hash, event_index, event_type, agreement_id, ledger)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (contract_id, tx_hash, event_index) DO NOTHING
	`, e.ContractID, e.TxHash, e.EventIndex, e.EventType, e.AgreementID, e.Ledger)
	if err != nil {
		return false, fmt.Errorf("inserting contract event %s/%s#%d: %w", e.ContractID, e.TxHash, e.EventIndex, err)
	}
	return cmd.RowsAffected() > 0, nil
}

// EventPage is one page of a cursor-paginated event listing. NextCursor is
// empty when there is no further page.
type EventPage struct {
	Events     []ContractEvent
	NextCursor string
}

// ListEventsForAgreement returns events for one agreement in ascending id
// order (their insertion/observation order), paginated by an opaque
// numeric cursor (the last-seen event id). limit is clamped to [1, 200].
func (s *Store) ListEventsForAgreement(ctx context.Context, agreementID int64, cursor int64, limit int) (EventPage, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, tx_hash, event_index, event_type, agreement_id, ledger, observed_at
		FROM contract_events
		WHERE agreement_id = $1 AND id > $2
		ORDER BY id
		LIMIT $3
	`, agreementID, cursor, limit)
	if err != nil {
		return EventPage{}, fmt.Errorf("listing events for agreement %d: %w", agreementID, err)
	}
	defer rows.Close()

	var page EventPage
	for rows.Next() {
		var e ContractEvent
		if err := rows.Scan(&e.ID, &e.ContractID, &e.TxHash, &e.EventIndex, &e.EventType, &e.AgreementID, &e.Ledger, &e.ObservedAt); err != nil {
			return EventPage{}, fmt.Errorf("scanning contract event row: %w", err)
		}
		page.Events = append(page.Events, e)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, fmt.Errorf("iterating events for agreement %d: %w", agreementID, err)
	}
	if len(page.Events) == limit {
		page.NextCursor = fmt.Sprintf("%d", page.Events[len(page.Events)-1].ID)
	}
	return page, nil
}
