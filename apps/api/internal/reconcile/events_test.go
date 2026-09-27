package reconcile

import (
	"testing"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func symbolTopicXDR(t *testing.T, symbol string) string {
	t.Helper()
	sym := xdr.ScSymbol(symbol)
	val := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
	b, err := xdr.MarshalBase64(val)
	if err != nil {
		t.Fatalf("marshaling symbol topic: %v", err)
	}
	return b
}

func u64ValueXDR(t *testing.T, v uint64) string {
	t.Helper()
	value := xdr.Uint64(v)
	val := xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &value}
	b, err := xdr.MarshalBase64(val)
	if err != nil {
		t.Fatalf("marshaling u64 value: %v", err)
	}
	return b
}

func addressValueXDR(t *testing.T) string {
	t.Helper()
	accountID, err := xdr.AddressToAccountId("GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF")
	if err != nil {
		t.Fatalf("building account id: %v", err)
	}
	addr := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &accountID}
	val := xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &addr}
	b, err := xdr.MarshalBase64(val)
	if err != nil {
		t.Fatalf("marshaling address value: %v", err)
	}
	return b
}

func TestEventType_DecodesSymbolTopic(t *testing.T) {
	event := protocol.EventInfo{
		EventType: "contract",
		TopicXDR:  []string{symbolTopicXDR(t, "agr_fun")},
	}
	if got := eventType(event); got != "agr_fun" {
		t.Errorf("eventType = %q, want agr_fun", got)
	}
}

func TestEventType_FallsBackWhenNoTopics(t *testing.T) {
	event := protocol.EventInfo{EventType: "system"}
	if got := eventType(event); got != "system" {
		t.Errorf("eventType = %q, want system (fallback)", got)
	}
}

func TestAgreementIDFromEventValue_DecodesU64(t *testing.T) {
	id, ok := agreementIDFromEventValue(u64ValueXDR(t, 42))
	if !ok {
		t.Fatal("expected ok=true for a u64 value")
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
}

func TestAgreementIDFromEventValue_FalseForAddressValue(t *testing.T) {
	_, ok := agreementIDFromEventValue(addressValueXDR(t))
	if ok {
		t.Error("expected ok=false for a non-u64 (Address) event value, e.g. a provider-registry event")
	}
}

func TestAgreementIDFromEventValue_FalseForEmpty(t *testing.T) {
	_, ok := agreementIDFromEventValue("")
	if ok {
		t.Error("expected ok=false for an empty value")
	}
}

func TestParseEventIndex(t *testing.T) {
	cases := []struct {
		id     string
		want   int32
		wantOK bool
	}{
		{"0000012345-0000000001", 1, true},
		{"0000012345-0000000000", 0, true},
		{"not-a-real-id-format", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseEventIndex(c.id)
		if ok != c.wantOK {
			t.Errorf("parseEventIndex(%q) ok = %v, want %v", c.id, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseEventIndex(%q) = %d, want %d", c.id, got, c.want)
		}
	}
}
