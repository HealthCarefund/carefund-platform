package sorobanenc

import (
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// These tests verify this package's encoders against the SDK's own
// ScVal.String() decoder — round-tripping through independently-written
// SDK code is how correctness is checked here, since there is no
// generated Go contract binding to compare against directly.

func randomAccountAddress(t *testing.T) string {
	t.Helper()
	kp, err := keypair.Random()
	if err != nil {
		t.Fatalf("generating keypair: %v", err)
	}
	return kp.Address()
}

func TestAddress_Account(t *testing.T) {
	addr := randomAccountAddress(t)
	val, err := Address(addr)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	if val.Type != xdr.ScValTypeScvAddress {
		t.Fatalf("Type = %v, want ScvAddress", val.Type)
	}
	decoded, err := val.Address.String()
	if err != nil {
		t.Fatalf("decoding address back: %v", err)
	}
	if decoded != addr {
		t.Errorf("round-trip = %q, want %q", decoded, addr)
	}
}

func TestAddress_Contract(t *testing.T) {
	// A real Testnet contract id (provider-registry, deployed for this
	// phase's verification) — see internal/api/transactions_test.go for
	// where this same value is used against the live network.
	const contractID = "CAJ4FM7F4PAGIO35SJIQ4PBK6N2A4MJFSPG46AQQWRZVLSABGBLQGBVJ"
	val, err := Address(contractID)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}
	decoded, err := val.Address.String()
	if err != nil {
		t.Fatalf("decoding address back: %v", err)
	}
	if decoded != contractID {
		t.Errorf("round-trip = %q, want %q", decoded, contractID)
	}
}

func TestAddress_RejectsInvalidPrefix(t *testing.T) {
	if _, err := Address("XNOTVALID"); err == nil {
		t.Fatal("expected an error for a non-G/C address")
	}
}

func TestBytes32_RoundTrips(t *testing.T) {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	val, err := Bytes32(b)
	if err != nil {
		t.Fatalf("Bytes32: %v", err)
	}
	if len(*val.Bytes) != 32 {
		t.Fatalf("encoded length = %d, want 32", len(*val.Bytes))
	}
	for i, want := range b {
		if (*val.Bytes)[i] != want {
			t.Fatalf("byte %d = %d, want %d", i, (*val.Bytes)[i], want)
		}
	}
}

func TestBytes32_RejectsWrongLength(t *testing.T) {
	if _, err := Bytes32(make([]byte, 31)); err == nil {
		t.Fatal("expected an error for 31 bytes")
	}
	if _, err := Bytes32(make([]byte, 33)); err == nil {
		t.Fatal("expected an error for 33 bytes")
	}
}

func TestU64_RoundTrips(t *testing.T) {
	for _, v := range []uint64{0, 1, 1_700_000_000, 18_446_744_073_709_551_615} {
		val := U64(v)
		if uint64(*val.U64) != v {
			t.Errorf("U64(%d) round-trip = %d", v, uint64(*val.U64))
		}
	}
}

func TestI128_RoundTripsAgainstSDKDecoder(t *testing.T) {
	cases := []string{
		"0",
		"1",
		"1000000",
		"900000",
		"18446744073709551615", // exactly u64 max, spans both words
		"18446744073709551616", // one past u64 max, forces Hi=1
		"170141183460469231731687303715884105727", // i128 max
	}
	for _, decimal := range cases {
		val, err := I128(decimal)
		if err != nil {
			t.Fatalf("I128(%q): %v", decimal, err)
		}
		if val.Type != xdr.ScValTypeScvI128 {
			t.Fatalf("Type = %v, want ScvI128", val.Type)
		}
		// ScVal.String() uses the SDK's own bigIntFromParts decoder.
		got := val.String()
		if got != decimal {
			t.Errorf("I128(%q) decodes back as %q via the SDK's own decoder", decimal, got)
		}
	}
}

func TestI128_RejectsNegative(t *testing.T) {
	if _, err := I128("-1"); err == nil {
		t.Fatal("expected an error for a negative value")
	}
}

func TestI128_RejectsOutOfRange(t *testing.T) {
	tooLarge := "170141183460469231731687303715884105728" // i128 max + 1
	if _, err := I128(tooLarge); err == nil {
		t.Fatal("expected an error for a value exceeding i128 range")
	}
}

func TestI128_RejectsMalformed(t *testing.T) {
	if _, err := I128("not-a-number"); err == nil {
		t.Fatal("expected an error for a malformed decimal string")
	}
}

func TestSymbol_EncodesAsSingleElementVecOfSymbol(t *testing.T) {
	val := Symbol("Resume")
	if val.Type != xdr.ScValTypeScvVec {
		t.Fatalf("Type = %v, want ScvVec", val.Type)
	}
	vec := *val.Vec
	if len(*vec) != 1 {
		t.Fatalf("vec length = %d, want 1", len(*vec))
	}
	elem := (*vec)[0]
	if elem.Type != xdr.ScValTypeScvSymbol {
		t.Fatalf("element type = %v, want ScvSymbol", elem.Type)
	}
	if string(*elem.Sym) != "Resume" {
		t.Errorf("symbol = %q, want Resume", string(*elem.Sym))
	}
	if !strings.Contains(val.String(), "Resume") {
		t.Errorf("String() = %q, want it to mention Resume", val.String())
	}
}
