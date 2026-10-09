package sorobanenc

import (
	"context"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
)

func TestDecodeAgreement_GenuineTestnetXDR(t *testing.T) {
	// Exact base64 ScVal returned by get_agreement(7) on Stellar Testnet
	const rawXDR = "AAAAEQAAAAEAAAASAAAADwAAABZhdHRlc3RhdGlvbl9jb21taXRtZW50AAAAAAABAAAADwAAAAthdHRlc3RlZF9hdAAAAAABAAAADwAAAAthdHRlc3RlZF9ieQAAAAABAAAADwAAAAhhdHRlc3RlcgAAABIAAAAAAAAAAH3/zCGtzLQUGKrxls38wdh5L06xwQJSVYDbwek4BYYnAAAADwAAAA1jYXJlX2RlYWRsaW5lAAAAAAAABQAAAABqyPdcAAAADwAAAApjcmVhdGVkX2F0AAAAAAAFAAAAAGrI8+kAAAAPAAAAEWRpc3B1dGVfb3BlbmVkX2F0AAAAAAAAAQAAAA8AAAARZGlzcHV0ZV9vcGVuZWRfYnkAAAAAAAABAAAADwAAAA5kaXNwdXRlX29yaWdpbgAAAAAAEAAAAAEAAAABAAAADwAAAAROb25lAAAADwAAABNkaXNwdXRlX3dpbmRvd19zZWNzAAAAAAUAAAAAAAAASAAAAA8AAAAOZnVuZGluZ19hbW91bnQAAAAAAAoAAAAAAAAAAAAAAAAAmJaAAAAADwAAABBmdW5kaW5nX2RlYWRsaW5lAAAABQAAAABqyPbkAAAADwAAABZwYXRpZW50X3JlZl9jb21taXRtZW50AAAAAAANAAAAIBERERERERERERERERERERERERERERERERERERERERERAAAADwAAAAhwcm92aWRlcgAAABIAAAAAAAAAAGz0qyL2+h41Qg2CM5P5+WYAJbEhf3sL3HL3fbE4gaikAAAADwAAABJzZXJ2aWNlX2NvbW1pdG1lbnQAAAAAAA0AAAAgIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIiIAAAAPAAAAEXNldHRsZW1lbnRfYW1vdW50AAAAAAAACgAAAAAAAAAAAAAAAAB6EgAAAAAPAAAAB3Nwb25zb3IAAAAAEgAAAAAAAAAA4y3eI3Ih7LlKDamd5ddJPXSjFpPyKK8yAdUQr8ljn7kAAAAPAAAABXN0YXRlAAAAAAAAEAAAAAEAAAABAAAADwAAAAlSZXF1ZXN0ZWQAAAA="

	var val xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(rawXDR, &val); err != nil {
		t.Fatalf("SafeUnmarshalBase64: %v", err)
	}

	const agreementID = int64(7)
	const settlementAsset = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

	agr, err := DecodeAgreement(val, agreementID, settlementAsset)
	if err != nil {
		t.Fatalf("DecodeAgreement: %v", err)
	}

	if agr.AgreementID != 7 {
		t.Errorf("AgreementID = %d, want 7", agr.AgreementID)
	}
	if agr.ProviderWallet != "GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453" {
		t.Errorf("ProviderWallet = %q", agr.ProviderWallet)
	}
	if agr.SponsorWallet != "GDRS3XRDOIQ6ZOKKBWUZ3ZOXJE6XJIYWSPZCRLZSAHKRBL6JMOP3SYPH" {
		t.Errorf("SponsorWallet = %q", agr.SponsorWallet)
	}
	if agr.AttesterWallet != "GB677TBBVXGLIFAYVLYZNTP4YHMHSL2OWHAQEUSVQDN4D2JYAWDCPFRT" {
		t.Errorf("AttesterWallet = %q", agr.AttesterWallet)
	}
	if agr.FundingAmount != "10000000" {
		t.Errorf("FundingAmount = %q, want 10000000", agr.FundingAmount)
	}
	if agr.SettlementAmount != "8000000" {
		t.Errorf("SettlementAmount = %q, want 8000000", agr.SettlementAmount)
	}
	if agr.FundingDeadline != 1791555300 {
		t.Errorf("FundingDeadline = %d, want 1791555300", agr.FundingDeadline)
	}
	if agr.CareDeadline != 1791555420 {
		t.Errorf("CareDeadline = %d, want 1791555420", agr.CareDeadline)
	}
	if agr.DisputeWindowSecs != 72 {
		t.Errorf("DisputeWindowSecs = %d, want 72", agr.DisputeWindowSecs)
	}
	if agr.State != "Requested" {
		t.Errorf("State = %q, want Requested", agr.State)
	}
	if hex.EncodeToString(agr.PatientRefCommitment) != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("PatientRefCommitment mismatch: %x", agr.PatientRefCommitment)
	}
	if hex.EncodeToString(agr.ServiceCommitment) != "2222222222222222222222222222222222222222222222222222222222222222" {
		t.Errorf("ServiceCommitment mismatch: %x", agr.ServiceCommitment)
	}
}

func TestBuildGetAgreementTransaction(t *testing.T) {
	const contractID = "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	txBase64, err := BuildGetAgreementTransaction(contractID, 7)
	if err != nil {
		t.Fatalf("BuildGetAgreementTransaction: %v", err)
	}
	if txBase64 == "" {
		t.Fatal("expected non-empty base64 string")
	}

	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshalBase64(txBase64, &env); err != nil {
		t.Fatalf("decoding generated transaction envelope: %v", err)
	}
}

func TestSimulateGetAgreement_LiveTestnet(t *testing.T) {
	if os.Getenv("SKIP_LIVE_NETWORK_TESTS") != "" {
		t.Skip("SKIP_LIVE_NETWORK_TESTS is set")
	}

	const contractID = "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	const settlementAsset = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	const liveTestnetRPCURL = "https://soroban-testnet.stellar.org"

	txBase64, err := BuildGetAgreementTransaction(contractID, 7)
	if err != nil {
		t.Fatalf("BuildGetAgreementTransaction: %v", err)
	}

	client := stellarrpc.New(liveTestnetRPCURL, 15*time.Second)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sim, err := client.Simulate(ctx, txBase64)
	if err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if sim.Error != "" {
		t.Fatalf("simulation error: %s", sim.Error)
	}
	if len(sim.Results) == 0 {
		t.Fatal("simulation returned no results")
	}

	if sim.Results[0].ReturnValueXDR == nil {
		t.Fatal("simulation returned nil ReturnValueXDR")
	}

	var retVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(*sim.Results[0].ReturnValueXDR, &retVal); err != nil {
		t.Fatalf("decoding simulation ScVal: %v", err)
	}

	agr, err := DecodeAgreement(retVal, 7, settlementAsset)
	if err != nil {
		t.Fatalf("DecodeAgreement: %v", err)
	}

	if agr.AgreementID != 7 {
		t.Errorf("AgreementID = %d, want 7", agr.AgreementID)
	}
	if agr.State != "Requested" {
		t.Errorf("State = %q, want Requested", agr.State)
	}
	if agr.ProviderWallet != "GBWPJKZC635B4NKCBWBDHE7Z7FTAAJNREF7XWC64OL3X3MJYQGUKI453" {
		t.Errorf("ProviderWallet = %q", agr.ProviderWallet)
	}
}
