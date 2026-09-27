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
