package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

const (
	testProviderRegistryID = "CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS"
	testCareAgreementID    = "CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O"
	testSettlementAssetID  = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
)

// makeAgreementEvent builds a synthetic care-agreement contract event whose
// id is a genuine Soroban "<toid>-<index>" cursor string, ordered by ledger
// then index, exactly like real RPC output.
func makeAgreementEvent(t *testing.T, ledger int32, idx int, agreementID uint64, topic string) protocol.EventInfo {
	t.Helper()
	// toid packs (ledger, tx, op) into one ordered int64; a fixed tx/op of 1
	// keeps ordering driven by ledger+idx, which is all these tests need.
	toid := (int64(ledger) << 32) | (int64(1) << 20) | int64(1)
	id := strconv.FormatInt(toid, 10) + "-" + strconv.Itoa(idx)
	// tx_hash must be unique per (ledger, idx) so distinct synthetic events
	// never collide under the real (contract_id, tx_hash, event_index)
	// dedup key - a collision here would hide a real pagination bug behind
	// a false "already ingested" no-op.
	unique := fmt.Sprintf("%08x%08x", ledger, idx)
	txHash := strings.Repeat(unique, 8)[:64]
	return protocol.EventInfo{
		EventType:       protocol.EventTypeContract,
		Ledger:          ledger,
		ContractID:      testCareAgreementID,
		ID:              id,
		TransactionHash: txHash,
		TopicXDR:        []string{symbolTopicXDR(t, topic)},
		ValueXDR:        u64ValueXDR(t, agreementID),
	}
}

// pagedFakeRPC serves a fixed sequence of GetEvents pages, asserting that
// each successive request continues from the cursor the previous response
// handed back (or starts the first page from StartLedger), exactly like the
// real RPC's pagination contract.
type pagedFakeRPC struct {
	pages         [][]protocol.EventInfo
	calls         int
	failOnPage    int // 1-indexed page number to fail on, 0 = never
	health        func() (protocol.GetHealthResponse, error)
	initialCursor string
	lastCursor    string
	requests      []protocol.GetEventsRequest
}

func (p *pagedFakeRPC) GetTransaction(_ context.Context, _ string) (protocol.GetTransactionResponse, error) {
	return protocol.GetTransactionResponse{}, nil
}

func (p *pagedFakeRPC) Simulate(_ context.Context, _ string) (protocol.SimulateTransactionResponse, error) {
	return protocol.SimulateTransactionResponse{}, nil
}

func (p *pagedFakeRPC) Health(ctx context.Context) (protocol.GetHealthResponse, error) {
	if p.health != nil {
		return p.health()
	}
	return protocol.GetHealthResponse{Status: "healthy", OldestLedger: 1}, nil
}

func (p *pagedFakeRPC) GetEvents(_ context.Context, req protocol.GetEventsRequest) (protocol.GetEventsResponse, error) {
	p.calls++
	p.requests = append(p.requests, req)
	if req.Pagination == nil || req.Pagination.Limit != eventsPerPage {
		panic(fmt.Sprintf("invalid pagination options on call %d: %#v", p.calls, req.Pagination))
	}
	if p.calls == 1 {
		if p.initialCursor != "" {
			if req.StartLedger != 0 || req.Pagination.Cursor == nil || req.Pagination.Cursor.String() != mustCursorString(p.initialCursor) {
				panic(fmt.Sprintf("restart request used startLedger=%d cursor=%v, want cursor %q", req.StartLedger, req.Pagination.Cursor, p.initialCursor))
			}
		} else if req.StartLedger == 0 || req.Pagination.Cursor != nil {
			panic(fmt.Sprintf("initial request used startLedger=%d cursor=%v, want a ledger start", req.StartLedger, req.Pagination.Cursor))
		}
	} else if req.StartLedger != 0 || req.Pagination.Cursor == nil || req.Pagination.Cursor.String() != mustCursorString(p.lastCursor) {
		panic(fmt.Sprintf("continuation request used startLedger=%d cursor=%v, want cursor %q", req.StartLedger, req.Pagination.Cursor, p.lastCursor))
	}
	if p.failOnPage == p.calls {
		return protocol.GetEventsResponse{}, context.DeadlineExceeded
	}
	if p.calls > len(p.pages) {
		return protocol.GetEventsResponse{Events: nil, LatestLedger: 99999}, nil
	}
	page := p.pages[p.calls-1]
	resp := protocol.GetEventsResponse{Events: page, LatestLedger: 99999}
	if len(page) > 0 {
		resp.Cursor = page[len(page)-1].ID
		p.lastCursor = resp.Cursor
	}
	return resp, nil
}

func mustCursorString(eventID string) string {
	cursor, err := protocol.ParseCursor(eventID)
	if err != nil {
		panic(err)
	}
	return cursor.String()
}

func TestReconcileEvents_MultiPageIngestion(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Two full pages (eventsPerPage=100) worth of events, split deliberately
	// so that a naive single-request implementation would only ever see
	// page one. This reproduces the original pagination-loss bug: with the
	// old reconcileEvents, only 100 of these 150 events would ever be
	// ingested, and the cursor would still advance to LatestLedger+1 as if
	// everything had been consumed.
	var page1, page2 []protocol.EventInfo
	for i := 0; i < eventsPerPage; i++ {
		page1 = append(page1, makeAgreementEvent(t, 500, i, 7, "agr_fun"))
	}
	for i := 0; i < 50; i++ {
		page2 = append(page2, makeAgreementEvent(t, 501, i, 7, "agr_fun"))
	}

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{page1, page2}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 500)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 150 {
		t.Fatalf("ingested %d events, want all 150 across both pages - pagination loss bug reproduced", len(page.Events))
	}
	if rpc.calls != 2 {
		t.Errorf("getEvents calls = %d, want 2 (one per page)", rpc.calls)
	}
	if rpc.requests[0].StartLedger != 1 || rpc.requests[0].Pagination.Cursor != nil {
		t.Errorf("initial request = %#v, want StartLedger=1 and no cursor", rpc.requests[0])
	}
	if got := rpc.requests[1].Pagination.Cursor.String(); got != mustCursorString(page1[len(page1)-1].ID) {
		t.Errorf("second-page cursor = %q, want %q", got, mustCursorString(page1[len(page1)-1].ID))
	}

	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	lastEvent := page2[len(page2)-1]
	if cursor != lastEvent.ID {
		t.Errorf("cursor = %q, want %q (the last event of the last page actually consumed)", cursor, lastEvent.ID)
	}
}

func TestReconcileEvents_SeveralPagesSameLedger(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	// All events share ledger 500 but have distinct event indices - the
	// fine-grained part of the cursor that a ledger-only scheme cannot
	// represent.
	var page1 []protocol.EventInfo
	for i := 0; i < eventsPerPage; i++ {
		page1 = append(page1, makeAgreementEvent(t, 500, i, 7, "agr_fun"))
	}
	page2 := []protocol.EventInfo{makeAgreementEvent(t, 500, eventsPerPage, 7, "agr_fun")}

	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{page1, page2}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 500)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != eventsPerPage+1 {
		t.Fatalf("ingested %d events, want %d (all events within the same ledger, across pages)", len(page.Events), eventsPerPage+1)
	}
}

func TestReconcileEvents_RepeatedPaginationCursorStopsWithoutLooping(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}
	page := make([]protocol.EventInfo, 0, eventsPerPage)
	for i := 0; i < eventsPerPage; i++ {
		page = append(page, makeAgreementEvent(t, 500, i, 7, "agr_fun"))
	}
	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{page, page}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)
	if rpc.calls != 2 {
		t.Errorf("getEvents calls = %d, want 2 (the repeated cursor must be detected on the second page)", rpc.calls)
	}
	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != page[len(page)-1].ID {
		t.Errorf("cursor = %q, want last successfully stored event %q", cursor, page[len(page)-1].ID)
	}
}

func TestReconcileEvents_CursorContinuationAfterRestart(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	first := makeAgreementEvent(t, 500, 0, 7, "agr_fun")
	rpc1 := &pagedFakeRPC{pages: [][]protocol.EventInfo{{first}}}
	runner1 := New(s, rpc1, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner1.reconcileEvents(ctx)

	// Simulate a process restart: a brand new Runner, sharing only the
	// durable store, must resume exactly where the last one left off.
	second := makeAgreementEvent(t, 501, 0, 7, "agr_fun")
	rpc2 := &pagedFakeRPC{pages: [][]protocol.EventInfo{{second}}, initialCursor: first.ID}
	runner2 := New(s, rpc2, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner2.reconcileEvents(ctx)

	if rpc2.calls != 1 {
		t.Fatalf("expected the restarted runner to make exactly one getEvents call, got %d", rpc2.calls)
	}

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("got %d events after restart, want 2 (no loss, no duplication across the restart)", len(page.Events))
	}
}

func TestReconcileEvents_RepeatedPageDelivery_NoDuplicates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	event := makeAgreementEvent(t, 500, 0, 7, "agr_fun")
	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{{event}}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)

	// Same page redelivered across three ticks (e.g. the RPC re-serves an
	// overlapping range on retry) - idempotent insertion must hold.
	runner.reconcileEvents(ctx)
	rpc.initialCursor = event.ID
	rpc.calls = 0 // re-arm the single-page fixture for each subsequent tick
	rpc.lastCursor = ""
	runner.reconcileEvents(ctx)
	rpc.calls = 0
	rpc.lastCursor = ""
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events after repeated delivery, want exactly 1 (deduplicated)", len(page.Events))
	}
}

func TestReconcileEvents_FailureDuringLaterPage_PreservesEarlierProgress(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	var page1 []protocol.EventInfo
	for i := 0; i < eventsPerPage; i++ {
		page1 = append(page1, makeAgreementEvent(t, 500, i, 7, "agr_fun"))
	}

	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{page1}, failOnPage: 2}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 500)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != eventsPerPage {
		t.Fatalf("got %d events, want %d (page 1's events must survive page 2's RPC failure)", len(page.Events), eventsPerPage)
	}

	lastOfPage1 := page1[len(page1)-1]
	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != lastOfPage1.ID {
		t.Errorf("cursor = %q, want %q", cursor, lastOfPage1.ID)
	}
}

func TestReconcileEvents_MalformedRelevantEvent_HaltsWithoutAdvancingCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	good := makeAgreementEvent(t, 500, 0, 7, "agr_fun")
	bad := makeAgreementEvent(t, 500, 1, 7, "agr_fun")
	bad.ID = "not-a-valid-soroban-event-id" // malformed, relevant (matches our contract)

	if err := s.UpsertAgreement(ctx, store.CareAgreement{
		AgreementID: 7, SponsorWallet: "G" + strings.Repeat("S", 55), ProviderWallet: "G" + strings.Repeat("P", 55),
		AttesterWallet: "G" + strings.Repeat("T", 55), PatientRefCommitment: make32(1), ServiceCommitment: make32(2),
		FundingAmount: "1000000", SettlementAmount: "900000", SettlementAssetContractID: testSettlementAssetID,
		FundingDeadline: 1_700_000_000, CareDeadline: 1_700_100_000, DisputeWindowSecs: 86_400, State: "Requested",
	}); err != nil {
		t.Fatalf("UpsertAgreement: %v", err)
	}

	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{{good, bad}}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)

	page, err := s.ListEventsForAgreement(ctx, 7, 0, 10)
	if err != nil {
		t.Fatalf("ListEventsForAgreement: %v", err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("got %d events, want 1 (the good event before the malformed one)", len(page.Events))
	}

	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != good.ID {
		t.Errorf("cursor = %q, want %q - must not advance past an unprocessable relevant event", cursor, good.ID)
	}
}

func TestReconcileEvents_RPCRetentionGap_RecordsEvidenceAndRequiresAcknowledgement(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Persisted cursor refers to a ledger well before what the RPC now retains.
	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "100"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}

	rpc := &pagedFakeRPC{
		pages:      nil,
		failOnPage: 1,
		health: func() (protocol.GetHealthResponse, error) {
			return protocol.GetHealthResponse{Status: "healthy", OldestLedger: 9000}, nil
		},
	}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)

	gapRaw, err := s.GetReconciliationCursor(ctx, eventGapKey)
	if err != nil || gapRaw == "" {
		t.Fatalf("expected a retention gap record to be persisted, err=%v raw=%q", err, gapRaw)
	}
	if !strings.Contains(gapRaw, `"requested_ledger":100`) || !strings.Contains(gapRaw, `"rpc_oldest_ledger":9000`) {
		t.Errorf("gap record missing expected evidence: %s", gapRaw)
	}

	// Cursor must NOT have been silently reset - reconciliation stays
	// blocked, visibly, until an operator explicitly acknowledges the loss.
	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != "100" {
		t.Errorf("cursor = %q, want unchanged 100 - a retention gap must never be silently papered over", cursor)
	}

	// A successful RPC response cannot bypass the unresolved block, including
	// when a brand-new Runner reads the same durable state after a restart.
	rpc.failOnPage = 0
	rpc.pages = [][]protocol.EventInfo{{}}
	rpc.calls = 0
	rpc.lastCursor = ""
	restarted := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	restarted.reconcileEvents(ctx)
	if rpc.calls != 0 {
		t.Errorf("getEvents calls while gap unresolved = %d, want 0", rpc.calls)
	}

	if err := runner.AcknowledgeEventRetentionGap(ctx); err != nil {
		t.Fatalf("AcknowledgeEventRetentionGap: %v", err)
	}

	cursorAfterAck, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursorAfterAck != "9000" {
		t.Errorf("cursor after acknowledgement = %q, want 9000 (resumed from oldest retained ledger)", cursorAfterAck)
	}

	// Once acknowledged, the approved resume position permits a controlled
	// continuation. The empty page keeps this assertion independent of entity
	// lookup behavior.
	rpc.calls = 0
	rpc.lastCursor = ""
	restarted.reconcileEvents(ctx)
	if rpc.calls != 1 {
		t.Errorf("getEvents calls after acknowledgement = %d, want 1", rpc.calls)
	}

	// A second acknowledgement without a new gap must fail rather than
	// silently succeed again.
	if err := runner.AcknowledgeEventRetentionGap(ctx); err == nil {
		t.Error("expected AcknowledgeEventRetentionGap to fail when the prior gap was already acknowledged")
	}
}

func TestReconcileEvents_MalformedRetentionGapFailsClosed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "100"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}
	if err := s.SetReconciliationCursor(ctx, eventGapKey, "not-json"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}
	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{{}}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)
	if rpc.calls != 0 {
		t.Errorf("getEvents calls = %d, want 0 for malformed gap", rpc.calls)
	}
}

func TestAcknowledgeEventRetentionGap_ConcurrentAttemptsOnlyOneSucceeds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "100"); err != nil {
		t.Fatalf("SetReconciliationCursor cursor: %v", err)
	}
	rec := eventGapRecord{
		DetectedAt:      time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC),
		RequestedLedger: 100,
		RPCOldestLedger: 9000,
		RPCError:        "retained window",
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal gap: %v", err)
	}
	if err := s.SetReconciliationCursor(ctx, eventGapKey, string(raw)); err != nil {
		t.Fatalf("SetReconciliationCursor gap: %v", err)
	}
	rpc := &pagedFakeRPC{health: func() (protocol.GetHealthResponse, error) {
		return protocol.GetHealthResponse{Status: "healthy", OldestLedger: 9000}, nil
	}}
	runner1 := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner2 := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	results := make(chan error, 2)
	go func() { results <- runner1.AcknowledgeEventRetentionGap(ctx) }()
	go func() { results <- runner2.AcknowledgeEventRetentionGap(ctx) }()
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("concurrent acknowledgements returned %v and %v, want exactly one success", first, second)
	}
	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil || cursor != "9000" {
		t.Fatalf("cursor after concurrent acknowledgement = %q, err=%v", cursor, err)
	}
}

func TestReconcileEvents_PersistedCursorCorruption_FailsClosed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "not-a-valid-cursor-or-ledger"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}

	rpc := &pagedFakeRPC{
		pages: [][]protocol.EventInfo{{}},
		health: func() (protocol.GetHealthResponse, error) {
			return protocol.GetHealthResponse{Status: "healthy", OldestLedger: 42}, nil
		},
	}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)

	// The corrupt value remains untouched and neither Health nor GetEvents may
	// be used to invent a new historical starting point.
	runner.reconcileEvents(ctx)
	if rpc.calls != 0 {
		t.Errorf("getEvents calls = %d, want 0", rpc.calls)
	}
	stored, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if stored != "not-a-valid-cursor-or-ledger" {
		t.Errorf("stored cursor = %q, want corrupt value preserved", stored)
	}
}

func TestReconcileEvents_CancelledPass_StopsCleanly(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SetReconciliationCursor(ctx, eventCursorKey, "1"); err != nil {
		t.Fatalf("SetReconciliationCursor: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{{makeAgreementEvent(t, 500, 0, 7, "agr_fun")}}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(cancelledCtx)

	if rpc.calls != 0 {
		t.Errorf("getEvents calls = %d, want 0 - a cancelled pass must not make further RPC calls", rpc.calls)
	}
	cursor, err := s.GetReconciliationCursor(ctx, eventCursorKey)
	if err != nil {
		t.Fatalf("GetReconciliationCursor: %v", err)
	}
	if cursor != "1" {
		t.Errorf("cursor = %q, want unchanged 1 after cancellation", cursor)
	}
}

func TestReconcileEvents_EmptyRangeDoesNotInventCompletedWatermark(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	rpc := &pagedFakeRPC{pages: [][]protocol.EventInfo{{}}}
	runner := New(s, rpc, testLogger(), time.Hour, testProviderRegistryID, testCareAgreementID, testSettlementAssetID)
	runner.reconcileEvents(ctx)
	if rpc.calls != 1 {
		t.Fatalf("getEvents calls = %d, want 1", rpc.calls)
	}
	if _, err := s.GetReconciliationCursor(ctx, eventCursorKey); err == nil {
		t.Fatal("empty range must not create an event cursor claiming the range was fully observed")
	}

	// A later tick still begins from the health watermark. This is deliberate:
	// there is no durable completed-range watermark that could falsely claim
	// an unrecorded interval was consumed.
	rpc.calls = 0
	rpc.lastCursor = ""
	runner.reconcileEvents(ctx)
	if rpc.calls != 1 {
		t.Errorf("second empty-range getEvents calls = %d, want 1", rpc.calls)
	}
}
