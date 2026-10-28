package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

const (
	eventCursorKey = "contract_events_last_ledger"
	eventGapKey    = "contract_events_retention_gap"

	// eventsPerPage bounds each RPC page. Fewer events returned than this
	// means the page was the last one in the queried range/cursor window
	// (the documented Stellar RPC getEvents pagination contract).
	eventsPerPage = 100
	// maxPagesPerPass bounds work done in a single reconciliation tick so
	// one pass can never block the ticker forever; any pages left
	// unconsumed are picked up next tick from the durable cursor left by
	// the last successfully processed event.
	maxPagesPerPass = 20
)

// reconcileEvents fetches new contract events for the provider-registry and
// care-agreement contracts since the last processed event and mirrors them
// into contract_events (idempotent - ON CONFLICT DO NOTHING on
// (contract_id, tx_hash, event_index), so overlapping ranges across
// restarts, retries or repeated page delivery never create duplicates).
//
// Unlike a single-request implementation, this walks every page the RPC
// reports for the current cursor/ledger window, and only ever advances the
// persisted cursor to the ID of an event it has actually stored - never to
// "latestLedger + 1", which proves nothing about whether every page was
// consumed.
func (r *Runner) reconcileEvents(ctx context.Context) {
	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	cursor, startLedger, err := r.eventResumePoint(ctx)
	if err != nil {
		r.logger.Error("reconcile: determining event resume point failed", "error", err)
		return
	}
	if cursor == nil && startLedger == 0 {
		return // could not determine a safe starting point this tick; try again next tick
	}

	for page := 0; page < maxPagesPerPass; page++ {
		if ctx.Err() != nil {
			return // cancelled: the cursor already reflects only fully-handled events
		}

		req := protocol.GetEventsRequest{
			Filters: []protocol.EventFilter{
				{
					EventType:   protocol.EventTypeSet{protocol.EventTypeContract: nil},
					ContractIDs: []string{r.providerRegistryContractID, r.careAgreementContractID},
				},
			},
			Pagination: &protocol.PaginationOptions{Limit: eventsPerPage},
		}
		if cursor != nil {
			req.Pagination.Cursor = cursor
		} else {
			req.StartLedger = startLedger
		}

		resp, err := r.rpc.GetEvents(ctx, req)
		if err != nil {
			r.handleGetEventsFailure(ctx, startLedger, cursor, err)
			return
		}

		for _, event := range resp.Events {
			if !r.isTrackedContract(event.ContractID) {
				// Defensive: the request's own ContractIDs filter should
				// already guarantee this. Fail closed rather than mirror
				// an event from a contract we were not asked to track.
				r.logger.Error("reconcile: RPC returned an event for an untracked contract, stopping pass", "contractId", event.ContractID, "eventId", event.ID)
				return
			}
			if err := r.storeEvent(ctx, event); err != nil {
				// A relevant event that could not be processed must not
				// silently count as ingested, and nothing after it in
				// ledger order can be assumed consumed either.
				r.logger.Error("reconcile: stopping pass - relevant event could not be processed", "eventId", event.ID, "error", err)
				return
			}
			if err := r.store.SetReconciliationCursor(ctx, eventCursorKey, event.ID); err != nil {
				r.logger.Error("reconcile: persisting event cursor failed", "eventId", event.ID, "error", err)
				return
			}
		}

		if len(resp.Events) < eventsPerPage {
			return // last page: fewer than a full page means no more results in range
		}

		if resp.Cursor == "" {
			r.logger.Error("reconcile: full RPC page had no continuation cursor, stopping pass")
			return
		}
		next, perr := protocol.ParseCursor(resp.Cursor)
		if perr != nil {
			r.logger.Error("reconcile: RPC returned an unparseable pagination cursor, stopping pass", "cursor", resp.Cursor, "error", perr)
			return
		}
		lastEvent, lastEventErr := protocol.ParseCursor(resp.Events[len(resp.Events)-1].ID)
		if lastEventErr != nil || next.Cmp(lastEvent) != 0 {
			r.logger.Error("reconcile: RPC pagination cursor does not identify the last event in the page, stopping pass", "cursor", resp.Cursor, "lastEventId", resp.Events[len(resp.Events)-1].ID)
			return
		}
		if cursor != nil && next.Cmp(*cursor) <= 0 {
			r.logger.Error("reconcile: RPC pagination cursor did not advance, stopping pass", "previousCursor", cursor.String(), "nextCursor", resp.Cursor)
			return
		}
		cursor = &next
		startLedger = 0
	}
	r.logger.Warn("reconcile: reached per-pass page bound with more events pending; continuing next tick", "pages", maxPagesPerPass)
}

// eventResumePoint returns where the next GetEvents request should resume
// from: either a precise RPC event cursor (the common case), or a ledger to
// start a fresh range query from (first run, or a legacy pre-pagination
// cursor). Soroban event IDs are themselves "<toid>-<index>" strings - the
// exact format protocol.ParseCursor expects - while a plain integer is the
// old ledger-based cursor this package used before pagination support. A
// malformed value is preserved and stops reconciliation; only a missing key
// is a genuinely new installation.
func (r *Runner) eventResumePoint(ctx context.Context) (*protocol.Cursor, uint32, error) {
	gapRaw, err := r.store.GetReconciliationCursor(ctx, eventGapKey)
	if err == nil {
		var rec eventGapRecord
		if gapRaw == "" {
			return nil, 0, fmt.Errorf("reconciliation retention-gap record is empty")
		}
		if err := json.Unmarshal([]byte(gapRaw), &rec); err != nil {
			return nil, 0, fmt.Errorf("decoding reconciliation retention-gap record: %w", err)
		}
		if err := rec.validate(); err != nil {
			return nil, 0, fmt.Errorf("invalid reconciliation retention-gap record: %w", err)
		}
		if rec.AcknowledgedAt == nil {
			return nil, 0, fmt.Errorf("reconciliation is blocked by an unacknowledged retention gap from ledger %d through %d", rec.RequestedLedger, rec.RPCOldestLedger-1)
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, 0, err
	}

	stored, err := r.store.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, 0, err
	}
	if err == nil {
		if stored == "" {
			return nil, 0, fmt.Errorf("persisted event cursor is empty; explicit operator recovery is required")
		}
		if v, perr := strconv.ParseUint(stored, 10, 32); perr == nil && v > 0 {
			// The old implementation could have stored LatestLedger+1 before
			// all pages were consumed. That loss cannot be reconstructed from
			// the legacy value alone and remains a documented migration limit.
			return nil, uint32(v), nil
		}
		if c, perr := protocol.ParseCursor(stored); perr == nil {
			return &c, 0, nil
		}
		return nil, 0, fmt.Errorf("persisted event cursor %q is corrupt; it was preserved and reconciliation is stopped", stored)
	}

	health, err := r.rpc.Health(ctx)
	if err != nil {
		return nil, 0, err
	}
	oldest := health.OldestLedger
	if oldest == 0 {
		oldest = 1
	}
	return nil, oldest, nil
}

// eventGapRecord is the durable evidence left behind when the persisted
// cursor refers to events older than the RPC currently retains. It is never
// cleared automatically: it is a permanent audit trail of a real gap in
// event history, not a transient error to forget about once the next tick
// happens to succeed.
type eventGapRecord struct {
	DetectedAt      time.Time  `json:"detected_at"`
	RequestedLedger uint32     `json:"requested_ledger"`
	RPCOldestLedger uint32     `json:"rpc_oldest_ledger"`
	RPCError        string     `json:"rpc_error"`
	AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
	ResumedAtLedger uint32     `json:"resumed_at_ledger,omitempty"`
}

func (r eventGapRecord) validate() error {
	if r.DetectedAt.IsZero() {
		return errors.New("missing detected_at")
	}
	if r.RequestedLedger == 0 || r.RPCOldestLedger <= r.RequestedLedger {
		return fmt.Errorf("invalid ledger range %d to %d", r.RequestedLedger, r.RPCOldestLedger)
	}
	if r.RPCError == "" {
		return errors.New("missing RPC error")
	}
	if r.AcknowledgedAt == nil && r.ResumedAtLedger != 0 {
		return errors.New("resume ledger is set before acknowledgement")
	}
	if r.AcknowledgedAt != nil && r.ResumedAtLedger == 0 {
		return errors.New("acknowledged gap is missing its resume ledger")
	}
	return nil
}

// handleGetEventsFailure distinguishes an RPC retention gap (the persisted
// cursor is now older than anything the RPC retains - unrecoverable from
// event history alone) from an ordinary transient failure (network error,
// timeout, momentary RPC unavailability), which is simply retried next
// tick with the cursor untouched.
func (r *Runner) handleGetEventsFailure(ctx context.Context, startLedger uint32, cursor *protocol.Cursor, rpcErr error) {
	requested := startLedger
	if cursor != nil {
		requested = cursor.Ledger
	}

	health, herr := r.rpc.Health(ctx)
	if herr == nil && requested > 0 && requested < health.OldestLedger {
		r.recordEventGap(ctx, requested, health.OldestLedger, rpcErr)
		return
	}

	r.logger.Warn("reconcile: getEvents failed, will retry next tick", "requestedLedger", requested, "error", rpcErr)
}

func (r *Runner) recordEventGap(ctx context.Context, requested, oldest uint32, rpcErr error) {
	rec := eventGapRecord{
		DetectedAt:      time.Now().UTC(),
		RequestedLedger: requested,
		RPCOldestLedger: oldest,
		RPCError:        rpcErr.Error(),
	}
	b, _ := json.Marshal(rec)
	if err := r.store.SetReconciliationCursor(ctx, eventGapKey, string(b)); err != nil {
		r.logger.Error("reconcile: persisting event retention gap evidence failed", "error", err)
	}
	r.logger.Error("reconcile: RPC retention gap detected - events between the persisted cursor and the RPC's oldest retained ledger cannot be recovered from event history; a later on-chain state read can restore an agreement's current state but NOT its missing historical event sequence; operator action required via Runner.AcknowledgeEventRetentionGap",
		"requestedLedger", requested, "rpcOldestLedger", oldest, "rpcError", rpcErr)
}

// AcknowledgeEventRetentionGap is the explicit, operator-invoked recovery
// action for a detected RPC retention gap. It does not recover or invent
// the missing events - it only lets reconciliation resume, from the RPC's
// current oldest retained ledger, with the loss already on record. Calling
// this when no unacknowledged gap is recorded fails rather than silently
// doing nothing.
func (r *Runner) AcknowledgeEventRetentionGap(ctx context.Context) error {
	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	raw, err := r.store.GetReconciliationCursor(ctx, eventGapKey)
	if errors.Is(err, store.ErrNotFound) || raw == "" {
		return fmt.Errorf("no retention gap is currently recorded")
	}
	if err != nil {
		return fmt.Errorf("reading retention gap: %w", err)
	}
	var rec eventGapRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return fmt.Errorf("decoding gap record: %w", err)
	}
	if err := rec.validate(); err != nil {
		return fmt.Errorf("invalid gap record: %w", err)
	}
	if rec.AcknowledgedAt != nil {
		return fmt.Errorf("retention gap was already acknowledged at %s", rec.AcknowledgedAt.Format(time.RFC3339))
	}
	currentCursor, err := r.store.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		return fmt.Errorf("reading event cursor: %w", err)
	}

	health, err := r.rpc.Health(ctx)
	if err != nil {
		return fmt.Errorf("fetching RPC health: %w", err)
	}
	oldest := health.OldestLedger
	if oldest == 0 {
		oldest = 1
	}
	if err := r.acknowledgeEventRetentionGapLocked(ctx, raw, currentCursor, oldest, nil); err != nil {
		return err
	}

	r.logger.Warn("reconcile: operator acknowledged RPC retention gap; resuming from oldest retained ledger - events in the gap are NOT recovered", "resumedAtLedger", oldest)
	return nil
}

func (r *Runner) isTrackedContract(contractID string) bool {
	return contractID == r.providerRegistryContractID || contractID == r.careAgreementContractID
}

func (r *Runner) storeEvent(ctx context.Context, event protocol.EventInfo) error {
	eventIndex, ok := parseEventIndex(event.ID)
	if !ok {
		// A relevant event (it matched our contract filter) with an ID we
		// cannot parse must not be treated as successfully ingested: skip
		// it silently and later events could seem "reconciled" when they
		// never were. Fail the pass instead so it is visible and retried.
		return fmt.Errorf("could not parse event index from event id %q", event.ID)
	}

	evType, err := decodeEventType(event)
	if err != nil {
		return err
	}

	var agreementID *int64
	switch {
	case event.ContractID == r.providerRegistryContractID:
		switch evType {
		case "init", "adm_rot":
			if err := validateAdminEventValue(evType, event.ValueXDR); err != nil {
				return err
			}
		case "prov_reg", "prov_sus", "prov_rei", "prov_rev":
			addr, ok := addressFromEventValue(event.ValueXDR)
			if !ok {
				return fmt.Errorf("event %s requires a valid provider address payload", evType)
			}
			if err := r.reconcileProvider(ctx, addr); err != nil {
				return fmt.Errorf("reconciling provider %s: %w", addr, err)
			}
		case "att_reg", "att_sus", "att_rei", "att_rev":
			addr, ok := addressFromEventValue(event.ValueXDR)
			if !ok {
				return fmt.Errorf("event %s requires a valid attester address payload", evType)
			}
			if err := r.reconcileAttester(ctx, addr); err != nil {
				return fmt.Errorf("reconciling attester %s: %w", addr, err)
			}
		default:
			return fmt.Errorf("unsupported provider-registry event type %q", evType)
		}
	case event.ContractID == r.careAgreementContractID:
		switch evType {
		case "init", "adm_rot":
			if err := validateAdminEventValue(evType, event.ValueXDR); err != nil {
				return err
			}
		case "agr_cre", "agr_fun", "agr_can", "agr_exp", "agr_att", "agr_dis", "agr_set", "agr_res":
			id, ok := agreementIDFromEventValue(event.ValueXDR)
			if !ok {
				return fmt.Errorf("event %s requires a representable u64 agreement ID", evType)
			}
			agreementID = &id
			if err := r.reconcileAgreement(ctx, id); err != nil {
				return fmt.Errorf("reconciling agreement %d: %w", id, err)
			}
		default:
			return fmt.Errorf("unsupported care-agreement event type %q", evType)
		}
	default:
		return fmt.Errorf("event belongs to untracked contract %q", event.ContractID)
	}

	inserted, err := r.store.InsertContractEvent(ctx, store.ContractEvent{
		ContractID:  event.ContractID,
		TxHash:      event.TransactionHash,
		EventIndex:  eventIndex,
		EventType:   evType,
		AgreementID: agreementID,
		Ledger:      int64(event.Ledger),
	})
	if err != nil {
		return fmt.Errorf("inserting contract event: %w", err)
	}
	if inserted {
		r.logger.Info("reconcile: observed contract event", "eventId", event.ID, "type", evType, "ledger", event.Ledger)
	}
	return nil
}

// addressFromEventValue best-effort decodes the event's value as an Address string.
func addressFromEventValue(valueXDR string) (string, bool) {
	if valueXDR == "" {
		return "", false
	}
	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(valueXDR, &val); err != nil || val.Type != xdr.ScValTypeScvAddress || val.Address == nil {
		return "", false
	}
	addr, err := val.Address.String()
	if err != nil {
		return "", false
	}
	return addr, true
}

// parseEventIndex extracts a stable integer event index from Soroban's own
// event ID (a composite "<toid>-<index>" string), so dedup identity is
// derived from the event itself, not from this batch's processing order.
func parseEventIndex(eventID string) (int32, bool) {
	cursor, err := protocol.ParseCursor(eventID)
	if err != nil || cursor.Event > math.MaxInt32 {
		return 0, false
	}
	return int32(cursor.Event), true
}

// eventType decodes the event's first topic (a Symbol, per how the
// contracts publish events: `env.events().publish((symbol_short!("..."),), ...)`)
// as this event's type. Falls back to the RPC-reported event type
// ("contract"/"system"/"diagnostic") if the topic can't be decoded.
func eventType(event protocol.EventInfo) string {
	decoded, err := decodeEventType(event)
	if err != nil {
		return event.EventType
	}
	return decoded
}

func decodeEventType(event protocol.EventInfo) (string, error) {
	if len(event.TopicXDR) != 1 {
		return "", fmt.Errorf("contract event must contain exactly one symbol topic")
	}
	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(event.TopicXDR[0], &val); err != nil || val.Type != xdr.ScValTypeScvSymbol || val.Sym == nil {
		return "", fmt.Errorf("contract event topic is not a valid symbol")
	}
	if *val.Sym == "" {
		return "", fmt.Errorf("contract event topic symbol is empty")
	}
	return string(*val.Sym), nil
}

// agreementIDFromEventValue best-effort decodes the event's value as a u64
// agreement id - true for every care-agreement event (they all publish the
// agreement_id as the event data), false for provider-registry events
// (which publish an Address instead).
func agreementIDFromEventValue(valueXDR string) (int64, bool) {
	if valueXDR == "" {
		return 0, false
	}
	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(valueXDR, &val); err != nil || val.Type != xdr.ScValTypeScvU64 || val.U64 == nil || *val.U64 > math.MaxInt64 {
		return 0, false
	}
	return int64(*val.U64), true
}

func validateAdminEventValue(eventType, valueXDR string) error {
	if eventType == "init" {
		if _, ok := addressFromEventValue(valueXDR); !ok {
			return fmt.Errorf("event init requires a valid administrator address payload")
		}
		return nil
	}
	if eventType == "adm_rot" && !twoAddressTuple(valueXDR) {
		return fmt.Errorf("event adm_rot requires an administrator address tuple payload")
	}
	return nil
}

func twoAddressTuple(valueXDR string) bool {
	if valueXDR == "" {
		return false
	}
	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(valueXDR, &val); err != nil || val.Type != xdr.ScValTypeScvVec || val.Vec == nil || *val.Vec == nil || len(**val.Vec) != 2 {
		return false
	}
	for _, item := range **val.Vec {
		if item.Type != xdr.ScValTypeScvAddress || item.Address == nil {
			return false
		}
		if _, err := item.Address.String(); err != nil {
			return false
		}
	}
	return true
}
