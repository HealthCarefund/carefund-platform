package store

import (
	"context"
	"fmt"
)

// InsertAuditRecord appends one immutable audit-log entry. metadataJSON
// must be valid JSON (or nil); it is never interpreted by this layer, only
// stored — callers are responsible for keeping it free of PHI/secrets,
// same as every other field in this package.
func (s *Store) InsertAuditRecord(ctx context.Context, actorWallet *string, action, resourceType, resourceID, result string, metadataJSON []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_records (actor_wallet, action, resource_type, resource_id, result, metadata_json)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, actorWallet, action, resourceType, resourceID, result, metadataJSON)
	if err != nil {
		return fmt.Errorf("inserting audit record for %s %s: %w", resourceType, resourceID, err)
	}
	return nil
}

// ListAuditRecords returns immutable audit evidence in creation order.
func (s *Store) ListAuditRecords(ctx context.Context, resourceType, resourceID string) ([]AuditRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_wallet, action, resource_type, resource_id, result, created_at, metadata_json
		FROM audit_records
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY id
	`, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("listing audit records for %s %s: %w", resourceType, resourceID, err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var record AuditRecord
		if err := rows.Scan(&record.ID, &record.ActorWallet, &record.Action, &record.ResourceType, &record.ResourceID, &record.Result, &record.CreatedAt, &record.MetadataJSON); err != nil {
			return nil, fmt.Errorf("scanning audit record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit records: %w", err)
	}
	return records, nil
}
