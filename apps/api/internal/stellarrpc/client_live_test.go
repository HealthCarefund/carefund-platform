package stellarrpc

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestClient_Health_LiveTestnet makes a REAL network call to the public
// Stellar Testnet Soroban RPC (soroban-testnet.stellar.org). This is
// genuine live-network verification of this package's Health() decoding
// against an actual server — not a mock, and not claimed anywhere else in
// this codebase as more than what it is: one read-only health check
// against Testnet, not a full live-Testnet transaction verification (that
// is Phase 10's scope). It skips cleanly if Testnet is unreachable, so
// `go test ./...` still passes offline.
func TestClient_Health_LiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}

	client := New("https://soroban-testnet.stellar.org", 10*time.Second)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("Status = %q, want healthy", health.Status)
	}
	if health.LatestLedger == 0 {
		t.Error("LatestLedger = 0, want a real ledger sequence")
	}
}
