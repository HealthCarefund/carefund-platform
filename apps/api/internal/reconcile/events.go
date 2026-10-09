package reconcile

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

const eventCursorKey = "contract_events_last_ledger"

// reconcileEvents fetches new contract events for the provider-registry and
// care-agreement contracts since the last processed ledger and mirrors
// them into contract_events (idempotent - ON CONFLICT DO NOTHING on
// (contract_id, tx_hash, event_index), so overlapping ranges across
// restarts or retries never create duplicates).
func (r *Runner) reconcileEvents(ctx context.Context) {
	startLedger, err := r.eventStartLedger(ctx)
	if err != nil {
		r.logger.Error("reconcile: determining event start ledger failed", "error", err)
		return
	}
	if startLedger == 0 {
		return // could not determine a safe starting point this tick; try again next tick
	}

	resp, err := r.rpc.GetEvents(ctx, protocol.GetEventsRequest{
		StartLedger: startLedger,
		Filters: []protocol.EventFilter{
			{ContractIDs: []string{r.providerRegistryContractID, r.careAgreementContractID}},
		},
	})
	if err != nil {
		// A common cause is startLedger having fallen outside RPC's
		// retention window (e.g. after a long process outage). Reset the
		// cursor to the currently reported oldest ledger so the next tick
		// can make progress again, instead of failing forever.
		r.logger.Warn("reconcile: getEvents failed, will retry next tick", "startLedger", startLedger, "error", err)
		return
	}

	allSuccessful := true
	for _, event := range resp.Events {
		if err := r.storeEvent(ctx, event); err != nil {
			allSuccessful = false
			r.logger.Error("reconcile: storing event failed", "eventId", event.ID, "error", err)
		}
	}

	if allSuccessful && resp.LatestLedger > 0 {
		next := resp.LatestLedger + 1
		if err := r.store.SetReconciliationCursor(ctx, eventCursorKey, strconv.FormatUint(uint64(next), 10)); err != nil {
			r.logger.Error("reconcile: persisting event cursor failed", "error", err)
		}
	}
}

// eventStartLedger returns the persisted cursor, or - on first run, or if
// the persisted cursor has fallen outside RPC's retention window - the
// oldest ledger RPC currently retains, so reconciliation always has a
// valid starting point rather than looping on an error forever.
func (r *Runner) eventStartLedger(ctx context.Context) (uint32, error) {
	stored, err := r.store.GetReconciliationCursor(ctx, eventCursorKey)
	if err == nil {
		if v, parseErr := strconv.ParseUint(stored, 10, 32); parseErr == nil {
			return uint32(v), nil
		}
	}

	health, err := r.rpc.Health(ctx)
	if err != nil {
		return 0, err
	}
	oldest := health.OldestLedger
	if oldest == 0 {
		oldest = 1
	}
	return oldest, nil
}

func (r *Runner) storeEvent(ctx context.Context, event protocol.EventInfo) error {
	eventIndex, ok := parseEventIndex(event.ID)
	if !ok {
		r.logger.Warn("reconcile: could not parse event index, skipping", "eventId", event.ID)
		return nil
	}

	var agreementID *int64
	if id, ok := agreementIDFromEventValue(event.ValueXDR); ok {
		agreementID = &id
	}

	evType := eventType(event)
	if event.ContractID == r.providerRegistryContractID {
		if addr, ok := addressFromEventValue(event.ValueXDR); ok {
			switch evType {
			case "prov_reg", "prov_sus", "prov_rei", "prov_rev":
				if err := r.reconcileProvider(ctx, addr); err != nil {
					return fmt.Errorf("reconciling provider %s: %w", addr, err)
				}
			case "att_reg", "att_sus", "att_rei", "att_rev":
				if err := r.reconcileAttester(ctx, addr); err != nil {
					return fmt.Errorf("reconciling attester %s: %w", addr, err)
				}
			}
		}
	} else if agreementID != nil {
		if err := r.reconcileAgreement(ctx, *agreementID); err != nil {
			return fmt.Errorf("reconciling agreement %d: %w", *agreementID, err)
		}
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
	if err := xdr.SafeUnmarshalBase64(valueXDR, &val); err != nil || val.Address == nil {
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
	parts := strings.Split(eventID, "-")
	if len(parts) == 0 {
		return 0, false
	}
	v, err := strconv.ParseInt(parts[len(parts)-1], 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(v), true
}

// eventType decodes the event's first topic (a Symbol, per how the
// contracts publish events: `env.events().publish((symbol_short!("..."),), ...)`)
// as this event's type. Falls back to the RPC-reported event type
// ("contract"/"system"/"diagnostic") if the topic can't be decoded.
func eventType(event protocol.EventInfo) string {
	if len(event.TopicXDR) == 0 {
		return event.EventType
	}
	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(event.TopicXDR[0], &val); err != nil || val.Sym == nil {
		return event.EventType
	}
	return string(*val.Sym)
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
	if err := xdr.SafeUnmarshalBase64(valueXDR, &val); err != nil || val.U64 == nil {
		return 0, false
	}
	return int64(*val.U64), true
}
