package reconcile

import (
	"context"
	"os"
	"testing"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
)

// TestReconcileEvents_LiveTestnet makes a REAL getEvents call against the
// real provider-registry/care-agreement contracts deployed to Testnet for
// this phase's verification (see Commit 13's message), decoding genuine
// on-chain events (init, prov_reg, att_reg, agr_cre, from the registration
// calls made during that verification) — not fabricated fixtures. Skips
// cleanly if Testnet or its retention window make this unreachable.
func TestReconcileEvents_LiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}

	const (
		providerRegistryID = "CAJ4FM7F4PAGIO35SJIQ4PBK6N2A4MJFSPG46AQQWRZVLSABGBLQGBVJ"
		careAgreementID    = "CCKFWGLHUL2CMX5EKZWOGPGJY6H4LKDGHEFC6JDHWSM2XP5DCUKVJGHS"
	)

	client := stellarrpc.New("https://soroban-testnet.stellar.org", 20*time.Second)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}

	// Soroban RPC retains events for a much shorter window than full
	// ledger/entry history, so start recently rather than at OldestLedger.
	latest, err := client.GetLatestLedger(ctx)
	if err != nil {
		t.Skipf("getLatestLedger failed: %v", err)
	}
	startLedger := health.OldestLedger
	if latest.Sequence > 5000 && latest.Sequence-5000 > startLedger {
		startLedger = latest.Sequence - 5000
	}

	resp, err := client.GetEvents(ctx, protocol.GetEventsRequest{
		StartLedger: startLedger,
		Filters: []protocol.EventFilter{
			{ContractIDs: []string{providerRegistryID, careAgreementID}},
		},
	})
	if err != nil {
		t.Skipf("getEvents failed (likely outside retention window by now): %v", err)
	}
	if len(resp.Events) == 0 {
		t.Skip("no events found in the current retention window — Testnet's retention window has likely rolled past this phase's verification events by now")
	}

	sawKnownEventType := false
	for _, event := range resp.Events {
		idx, ok := parseEventIndex(event.ID)
		if !ok {
			t.Errorf("could not parse event index from real event id %q", event.ID)
			continue
		}
		_ = idx
		et := eventType(event)
		switch et {
		case "init", "prov_reg", "att_reg", "agr_cre":
			sawKnownEventType = true
		}
	}
	if !sawKnownEventType {
		t.Errorf("expected at least one of this phase's known event types among %d real events", len(resp.Events))
	}
}
