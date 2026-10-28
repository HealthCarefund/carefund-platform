package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

const eventRecoveryAuditResource = "contract_events"

// EventRetentionGap is the operator-facing form of a durable retention gap.
type EventRetentionGap struct {
	DetectedAt      string `json:"detected_at"`
	RequestedLedger uint32 `json:"requested_ledger"`
	RPCOldestLedger uint32 `json:"rpc_oldest_ledger"`
	RPCError        string `json:"rpc_error"`
	AcknowledgedAt  string `json:"acknowledged_at,omitempty"`
	ResumedAtLedger uint32 `json:"resumed_at_ledger,omitempty"`
}

// EventRecoverySnapshot contains only read-only reconciliation and RPC state.
// It contains no credentials or transaction material.
type EventRecoverySnapshot struct {
	EventCursor                    string
	Gap                            *EventRetentionGap
	GapID                          string
	GapError                       string
	RPCOldestLedger                uint32
	RPCLatestLedger                uint32
	RPCError                       string
	HistoricalRecoveryPossible     bool
	HistoricalRangeStatus          string
	HistoricalCompletenessVerified bool
	ReconciliationBlocked          bool
	PreviousRecoveryDecisions      []store.AuditRecord
}

// Historical range status values deliberately describe only what this tool
// can establish from durable state and the RPC retention boundary. None of
// them claims that every event in a retained range was indexed.
const (
	HistoricalStatusUnknown             = "unknown"
	HistoricalStatusNoGapUnverified     = "no-gap-recorded; completeness-not-verified"
	HistoricalStatusPendingGap          = "retention-gap-pending; historical-range-unrecoverable"
	HistoricalStatusMalformedGap        = "retention-gap-malformed; reconciliation-blocked"
	HistoricalStatusAcknowledgedGap     = "retention-gap-acknowledged; historical-range-unrecoverable"
	HistoricalStatusCursorIntegrity     = "cursor-integrity-failure"
	HistoricalStatusCursorOutsideWindow = "cursor-outside-retention; historical-replay-not-possible"
	HistoricalStatusRetentionMayPermit  = "retention-may-permit-replay; completeness-not-verified"
	HistoricalStatusRPCUnavailable      = "rpc-retention-unknown"
)

// EventSkippedRange is the exact historical range accepted by a resume
// decision. It uses the current RPC boundary, not the boundary recorded when
// the gap was first detected.
type EventSkippedRange struct {
	FromLedger             uint32
	ThroughLedger          uint32
	OriginalThroughLedger  uint32
	CurrentRPCOldestLedger uint32
	ApprovedResumeLedger   uint32
}

func CalculateCompleteSkippedRange(gap EventRetentionGap, currentRPCOldestLedger, approvedResumeLedger uint32) (EventSkippedRange, error) {
	if gap.RequestedLedger == 0 || gap.RPCOldestLedger <= gap.RequestedLedger {
		return EventSkippedRange{}, fmt.Errorf("invalid original retention gap %d-%d", gap.RequestedLedger, gap.RPCOldestLedger)
	}
	if currentRPCOldestLedger == 0 || approvedResumeLedger == 0 || currentRPCOldestLedger != approvedResumeLedger {
		return EventSkippedRange{}, fmt.Errorf("current RPC oldest ledger and approved resume ledger must match and be positive")
	}
	if approvedResumeLedger <= gap.RequestedLedger {
		return EventSkippedRange{}, fmt.Errorf("approved resume ledger %d does not follow requested ledger %d", approvedResumeLedger, gap.RequestedLedger)
	}
	return EventSkippedRange{
		FromLedger:             gap.RequestedLedger,
		ThroughLedger:          approvedResumeLedger - 1,
		OriginalThroughLedger:  gap.RPCOldestLedger - 1,
		CurrentRPCOldestLedger: currentRPCOldestLedger,
		ApprovedResumeLedger:   approvedResumeLedger,
	}, nil
}

// EventRecoveryGapID returns the stable confirmation identifier for the
// exact serialized gap record stored in PostgreSQL.
func EventRecoveryGapID(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

// InspectEventRecovery reads the durable cursor/gap state and RPC health. It
// does not write migrations, reconciliation state, audit rows, or chain data.
func (r *Runner) InspectEventRecovery(ctx context.Context) (EventRecoverySnapshot, error) {
	snapshot := EventRecoverySnapshot{
		HistoricalRangeStatus: HistoricalStatusUnknown,
	}
	cursor, err := r.store.GetReconciliationCursor(ctx, eventCursorKey)
	if err == nil {
		snapshot.EventCursor = cursor
		if !validStoredEventCursor(cursor) {
			snapshot.ReconciliationBlocked = true
			snapshot.HistoricalRecoveryPossible = false
			snapshot.HistoricalRangeStatus = HistoricalStatusCursorIntegrity
		}
	} else if !isNotFound(err) {
		return snapshot, err
	}

	rawGap, err := r.store.GetReconciliationCursor(ctx, eventGapKey)
	if err == nil {
		snapshot.GapID = EventRecoveryGapID(rawGap)
		var record eventGapRecord
		if unmarshalErr := json.Unmarshal([]byte(rawGap), &record); unmarshalErr != nil {
			snapshot.GapError = unmarshalErr.Error()
			snapshot.ReconciliationBlocked = true
			snapshot.HistoricalRecoveryPossible = false
			snapshot.HistoricalRangeStatus = HistoricalStatusMalformedGap
		} else if validateErr := record.validate(); validateErr != nil {
			snapshot.GapError = validateErr.Error()
			snapshot.ReconciliationBlocked = true
			snapshot.HistoricalRecoveryPossible = false
			snapshot.HistoricalRangeStatus = HistoricalStatusMalformedGap
		} else {
			snapshot.Gap = eventRetentionGap(record)
			snapshot.ReconciliationBlocked = record.AcknowledgedAt == nil || snapshot.ReconciliationBlocked
			// A recorded gap is evidence that historical recovery for that
			// range is no longer possible, even after continuation resumes.
			snapshot.HistoricalRecoveryPossible = false
			if record.AcknowledgedAt == nil {
				snapshot.HistoricalRangeStatus = HistoricalStatusPendingGap
			} else {
				snapshot.HistoricalRangeStatus = HistoricalStatusAcknowledgedGap
			}
		}
	} else if !isNotFound(err) {
		return snapshot, err
	} else {
		snapshot.HistoricalRecoveryPossible = !snapshot.ReconciliationBlocked
		if !snapshot.ReconciliationBlocked {
			snapshot.HistoricalRangeStatus = HistoricalStatusNoGapUnverified
		}
	}

	decisions, err := r.store.ListAuditRecords(ctx, "reconciliation_gap", eventRecoveryAuditResource)
	if err != nil {
		return snapshot, err
	}
	snapshot.PreviousRecoveryDecisions = decisions

	health, err := r.rpc.Health(ctx)
	if err != nil {
		snapshot.RPCError = err.Error()
		if snapshot.Gap == nil && !snapshot.ReconciliationBlocked {
			snapshot.HistoricalRangeStatus = HistoricalStatusRPCUnavailable
		}
		return snapshot, nil
	}
	snapshot.RPCOldestLedger = health.OldestLedger
	snapshot.RPCLatestLedger = health.LatestLedger
	if snapshot.HistoricalRecoveryPossible && snapshot.EventCursor != "" {
		if ledger, ok := storedCursorLedger(snapshot.EventCursor); ok && ledger < snapshot.RPCOldestLedger {
			snapshot.HistoricalRecoveryPossible = false
			snapshot.HistoricalRangeStatus = HistoricalStatusCursorOutsideWindow
		} else if ok {
			snapshot.HistoricalRangeStatus = HistoricalStatusRetentionMayPermit
		}
	}
	return snapshot, nil
}

// AcknowledgeEventRetentionGapAt commits only if the exact gap digest, cursor,
// and current RPC oldest ledger still match the operator's confirmation.
func (r *Runner) AcknowledgeEventRetentionGapAt(ctx context.Context, expectedGapID, expectedCursor string, resumeLedger uint32, audit store.ReconciliationAudit) error {
	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	raw, err := r.store.GetReconciliationCursor(ctx, eventGapKey)
	if err != nil {
		return fmt.Errorf("reading retention gap: %w", err)
	}
	if EventRecoveryGapID(raw) != expectedGapID {
		return fmt.Errorf("retention gap changed since inspection; refusing stale confirmation")
	}
	return r.acknowledgeEventRetentionGapLocked(ctx, raw, expectedCursor, resumeLedger, &audit)
}

func (r *Runner) acknowledgeEventRetentionGapLocked(ctx context.Context, expectedGap, expectedCursor string, resumeLedger uint32, audit *store.ReconciliationAudit) error {
	if resumeLedger == 0 {
		return fmt.Errorf("resume ledger must be positive")
	}
	var record eventGapRecord
	if err := json.Unmarshal([]byte(expectedGap), &record); err != nil {
		return fmt.Errorf("decoding gap record: %w", err)
	}
	if err := record.validate(); err != nil {
		return fmt.Errorf("invalid gap record: %w", err)
	}
	if record.AcknowledgedAt != nil {
		return fmt.Errorf("retention gap was already acknowledged at %s", record.AcknowledgedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	currentCursor, err := r.store.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		return fmt.Errorf("reading event cursor: %w", err)
	}
	if currentCursor != expectedCursor {
		return fmt.Errorf("event cursor changed since inspection; refusing stale confirmation")
	}
	health, err := r.rpc.Health(ctx)
	if err != nil {
		return fmt.Errorf("fetching RPC health: %w", err)
	}
	oldest := health.OldestLedger
	if oldest == 0 {
		oldest = 1
	}
	if oldest != resumeLedger {
		return fmt.Errorf("RPC oldest ledger changed from proposed %d to %d; inspect again", resumeLedger, oldest)
	}

	now := time.Now().UTC()
	record.AcknowledgedAt = &now
	record.ResumedAtLedger = resumeLedger
	acknowledgedGap, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding gap acknowledgement: %w", err)
	}
	if audit != nil {
		if audit.Action == "" || audit.ResourceID == "" {
			return fmt.Errorf("recovery audit action and resource ID are required")
		}
	}
	if audit == nil {
		if err := r.store.AcknowledgeReconciliationGap(ctx, eventGapKey, eventCursorKey, expectedGap, expectedCursor, strconv.FormatUint(uint64(resumeLedger), 10), string(acknowledgedGap)); err != nil {
			return fmt.Errorf("atomically recording gap acknowledgement: %w", err)
		}
	} else if err := r.store.AcknowledgeReconciliationGapWithAudit(ctx, eventGapKey, eventCursorKey, expectedGap, expectedCursor, strconv.FormatUint(uint64(resumeLedger), 10), string(acknowledgedGap), *audit); err != nil {
		return fmt.Errorf("atomically recording gap acknowledgement and audit: %w", err)
	}
	return nil
}

func eventRetentionGap(record eventGapRecord) *EventRetentionGap {
	gap := &EventRetentionGap{
		DetectedAt:      record.DetectedAt.Format("2006-01-02T15:04:05Z07:00"),
		RequestedLedger: record.RequestedLedger,
		RPCOldestLedger: record.RPCOldestLedger,
		RPCError:        record.RPCError,
		ResumedAtLedger: record.ResumedAtLedger,
	}
	if record.AcknowledgedAt != nil {
		gap.AcknowledgedAt = record.AcknowledgedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return gap
}

func validStoredEventCursor(cursor string) bool {
	if cursor == "" {
		return false
	}
	if value, err := strconv.ParseUint(cursor, 10, 32); err == nil {
		return value > 0
	}
	_, err := protocol.ParseCursor(cursor)
	return err == nil
}

func storedCursorLedger(cursor string) (uint32, bool) {
	if value, err := strconv.ParseUint(cursor, 10, 32); err == nil {
		return uint32(value), value > 0
	}
	parsed, err := protocol.ParseCursor(cursor)
	if err != nil {
		return 0, false
	}
	return parsed.Ledger, parsed.Ledger > 0
}

func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound)
}
