package sorobanenc

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
)

// These constants are the real contracts deployed to Testnet while
// verifying this phase's work (see the commit message for the deployment
// transaction links). They are not secrets — contract IDs and public keys
// are, by design, public on-chain identifiers.
const (
	liveProviderRegistryContractID = "CAJ4FM7F4PAGIO35SJIQ4PBK6N2A4MJFSPG46AQQWRZVLSABGBLQGBVJ"
	liveDeployerAddress            = "GAKEZNJV5AB52YBXLI3BMVQX65TADGZ6MB5UMBPWTXPXIT7O4L3PDYP5"
	liveTestnetPassphrase          = "Test SDF Network ; September 2015"
	liveTestnetRPCURL              = "https://soroban-testnet.stellar.org"
)

// TestPrepareContractCall_LiveTestnet makes REAL network calls — loading a
// real account and simulating a real contract invocation
// (is_provider_active, a read-only call) against the actual
// provider-registry contract deployed to Testnet for this phase's
// verification. This is genuine live-network evidence for this package's
// build->simulate->assemble logic, not a mock. It is a single simulated
// read, not a signed/submitted transaction, and is not represented as
// anything beyond that — real settlement/live wallet-signing verification
// remains Phase 10's scope. Skips cleanly if Testnet is unreachable.
func TestPrepareContractCall_LiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}

	client := stellarrpc.New(liveTestnetRPCURL, 15*time.Second)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if _, err := client.Health(ctx); err != nil {
		t.Skipf("Testnet RPC not reachable from this environment: %v", err)
	}

	providerArg, err := Address(liveDeployerAddress)
	if err != nil {
		t.Fatalf("encoding provider address: %v", err)
	}

	preparedXDR, err := PrepareContractCall(ctx, client, client, ContractCallRequest{
		ContractID:        liveProviderRegistryContractID,
		Method:            "is_provider_active",
		Args:              []xdr.ScVal{providerArg},
		SourcePublicKey:   liveDeployerAddress,
		NetworkPassphrase: liveTestnetPassphrase,
		TimeoutSeconds:    60,
	})
	if err != nil {
		t.Fatalf("PrepareContractCall against live Testnet: %v", err)
	}

	if !strings.HasPrefix(preparedXDR, "AAAA") {
		t.Errorf("preparedXDR = %q, does not look like a base64 TransactionEnvelope", preparedXDR)
	}

	// The prepared envelope must decode back into a valid, unsigned
	// transaction with the resource footprint/fee the real simulation
	// assigned it.
	var envelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshalBase64(preparedXDR, &envelope); err != nil {
		t.Fatalf("decoding prepared envelope: %v", err)
	}
	v1, ok := envelope.GetV1()
	if !ok {
		t.Fatal("expected a V1 transaction envelope")
	}
	if len(v1.Tx.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(v1.Tx.Operations))
	}
	if len(v1.Signatures) != 0 {
		t.Errorf("signatures = %d, want 0 (this is an unsigned envelope)", len(v1.Signatures))
	}
	if v1.Tx.Ext.V != 1 || v1.Tx.Ext.SorobanData == nil {
		t.Error("expected the real simulation's SorobanTransactionData to be attached")
	}
	if int64(v1.Tx.Fee) <= txnbuildMinBaseFee {
		t.Errorf("Fee = %d, want more than the bare base fee (the real simulated resource fee should be added)", v1.Tx.Fee)
	}
}

const txnbuildMinBaseFee = 100
