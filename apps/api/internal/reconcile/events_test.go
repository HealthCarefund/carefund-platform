package reconcile

import (
	"math"
	"strings"
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

func TestStoreEvent_ValidatesContractPayloadsBeforeCheckpointing(t *testing.T) {
	s := newTestStore(t)
	runner := New(s, nil, testLogger(), 0, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)

	badAgreement := protocol.EventInfo{
		EventType:       protocol.EventTypeContract,
		ContractID:      testCareAgreementID,
		ID:              "0000000000000000001-0000000000",
		TransactionHash: strings.Repeat("a", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "agr_fun")},
		ValueXDR:        addressValueXDR(t),
	}
	if err := runner.storeEvent(t.Context(), badAgreement); err == nil {
		t.Fatal("agreement event with address payload must be rejected")
	}

	overflow := u64ValueXDR(t, math.MaxUint64)
	badOverflow := badAgreement
	badOverflow.TransactionHash = strings.Repeat("b", 64)
	badOverflow.ValueXDR = overflow
	if err := runner.storeEvent(t.Context(), badOverflow); err == nil {
		t.Fatal("agreement ID above int64 range must be rejected")
	}

	badProvider := badAgreement
	badProvider.ContractID = testProviderRegistryID
	badProvider.TransactionHash = strings.Repeat("c", 64)
	badProvider.TopicXDR = []string{symbolTopicXDR(t, "prov_reg")}
	badProvider.ValueXDR = u64ValueXDR(t, 7)
	if err := runner.storeEvent(t.Context(), badProvider); err == nil {
		t.Fatal("provider event with agreement-ID payload must be rejected")
	}

	badTopic := badAgreement
	badTopic.TransactionHash = strings.Repeat("d", 64)
	badTopic.TopicXDR = []string{"not-xdr"}
	if err := runner.storeEvent(t.Context(), badTopic); err == nil {
		t.Fatal("malformed contract topic must be rejected")
	}
}

func TestStoreEvent_AcceptsInitializationAndAdminRotationPayloads(t *testing.T) {
	s := newTestStore(t)
	runner := New(s, nil, testLogger(), 0, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)

	initEvent := protocol.EventInfo{
		EventType:       protocol.EventTypeContract,
		ContractID:      testProviderRegistryID,
		ID:              "0000000000000000002-0000000000",
		TransactionHash: strings.Repeat("e", 64),
		TopicXDR:        []string{symbolTopicXDR(t, "init")},
		ValueXDR:        addressValueXDR(t),
	}
	if err := runner.storeEvent(t.Context(), initEvent); err != nil {
		t.Fatalf("valid init event: %v", err)
	}

	rotation := initEvent
	rotation.ID = "0000000000000000003-0000000000"
	rotation.TransactionHash = strings.Repeat("f", 64)
	rotation.TopicXDR = []string{symbolTopicXDR(t, "adm_rot")}
	rotation.ValueXDR = adminRotationValueXDR(t)
	if err := runner.storeEvent(t.Context(), rotation); err != nil {
		t.Fatalf("valid adm_rot event: %v", err)
	}
}

func adminRotationValueXDR(t *testing.T) string {
	t.Helper()
	accountID, err := xdr.AddressToAccountId("GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF")
	if err != nil {
		t.Fatalf("building account id: %v", err)
	}
	address := func() xdr.ScVal {
		a := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &accountID}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &a}
	}
	vec := xdr.ScVec{address(), address()}
	vecPtr := &vec
	val := xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &vecPtr}
	b, err := xdr.MarshalBase64(val)
	if err != nil {
		t.Fatalf("marshaling admin rotation payload: %v", err)
	}
	return b
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
