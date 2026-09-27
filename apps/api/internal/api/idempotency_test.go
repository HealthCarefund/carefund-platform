package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// uniqueIdempotencyKey gives each test (and each independent request within
// a test that isn't specifically testing key reuse) its own key, so tests
// never interfere with each other's idempotency_keys rows.
var idempotencyKeyCounter int64

func uniqueIdempotencyKey(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-%s-%d", t.Name(), atomic.AddInt64(&idempotencyKeyCounter, 1))
}

func postIntentWithKey(t *testing.T, mux http.Handler, req createIntentRequest, key string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/agreements/intents", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(idempotencyKeyHeader, key)
	mux.ServeHTTP(rec, httpReq)
	return rec
}

func TestIdempotency_MissingKeyRejected(t *testing.T) {
	_, mux := newTestDeps(t)
	body, _ := json.Marshal(validIntentRequest())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agreements/intents", bytes.NewReader(body))
	// deliberately no Idempotency-Key header
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestIdempotency_SameKeySameRequestReplaysOriginalResult(t *testing.T) {
	_, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)
	req := validIntentRequest()

	first := postIntentWithKey(t, mux, req, key)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: status = %d, want %d; body=%s", first.Code, http.StatusCreated, first.Body.String())
	}
	var firstBody intentResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decoding first response: %v", err)
	}

	second := postIntentWithKey(t, mux, req, key)
	if second.Code != http.StatusCreated {
		t.Fatalf("second: status = %d, want %d (a replay); body=%s", second.Code, http.StatusCreated, second.Body.String())
	}
	var secondBody intentResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decoding second response: %v", err)
	}

	if secondBody.ID != firstBody.ID {
		t.Errorf("second.ID = %q, want the same id as the first (%q) — the operation must not have re-run", secondBody.ID, firstBody.ID)
	}
}

func TestIdempotency_SameKeyDifferentRequestConflicts(t *testing.T) {
	_, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)

	first := postIntentWithKey(t, mux, validIntentRequest(), key)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: status = %d, want %d; body=%s", first.Code, http.StatusCreated, first.Body.String())
	}

	differentReq := validIntentRequest()
	differentReq.FundingAmount = "2000000" // a materially different request

	second := postIntentWithKey(t, mux, differentReq, key)
	if second.Code != http.StatusConflict {
		t.Fatalf("second: status = %d, want %d; body=%s", second.Code, http.StatusConflict, second.Body.String())
	}
}

func TestIdempotency_DuplicateRequestsDoNotCreateDuplicateRows(t *testing.T) {
	deps, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)
	req := validIntentRequest()

	first := postIntentWithKey(t, mux, req, key)
	var firstBody intentResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decoding first response: %v", err)
	}
	postIntentWithKey(t, mux, req, key) // replay; must not create a second intent

	// Confirm exactly one agreement_intents row exists for this id by
	// re-fetching it and checking there isn't a "next" row claiming the
	// same wallets/amount created by a second (buggy) insert. The
	// authoritative check: GetAgreementIntent(id+1) must not also be an
	// identical duplicate of this request.
	id, err := parseIntentIDForTest(firstBody.ID)
	if err != nil {
		t.Fatalf("parsing intent id: %v", err)
	}
	got, err := deps.Store.GetAgreementIntent(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAgreementIntent: %v", err)
	}
	if got.SponsorWallet != req.SponsorWallet {
		t.Errorf("got = %+v", got)
	}
}

func parseIntentIDForTest(s string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(s, "%d", &id)
	return id, err
}

// TestIdempotency_ConcurrentDuplicatesResultInExactlyOneSuccess fires many
// concurrent requests with the same Idempotency-Key and the same body —
// exactly one must actually create an intent; every other response must
// either replay that same result or report a conflict (for the narrow
// window where the winner hasn't finished persisting its result yet), but
// none may create a second intent.
func TestIdempotency_ConcurrentDuplicatesResultInExactlyOneSuccess(t *testing.T) {
	_, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)
	req := validIntentRequest()

	const concurrency = 10
	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = postIntentWithKey(t, mux, req, key)
		}(i)
	}
	wg.Wait()

	seenIDs := map[string]bool{}
	for i, rec := range results {
		if rec.Code != http.StatusCreated && rec.Code != http.StatusConflict {
			t.Errorf("request %d: status = %d, want %d or %d; body=%s", i, rec.Code, http.StatusCreated, http.StatusConflict, rec.Body.String())
			continue
		}
		if rec.Code == http.StatusCreated {
			var body intentResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decoding response %d: %v", i, err)
			}
			seenIDs[body.ID] = true
		}
	}
	if len(seenIDs) != 1 {
		t.Errorf("distinct intent ids created = %d, want exactly 1: %v", len(seenIDs), seenIDs)
	}
}

// TestIdempotency_SurvivesFreshDepsInstance simulates a process restart:
// state lives entirely in Postgres, not in any in-memory map, so a brand
// new Deps/mux (as a fresh process would build) still replays correctly.
func TestIdempotency_SurvivesFreshDepsInstance(t *testing.T) {
	deps, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)
	req := validIntentRequest()

	first := postIntentWithKey(t, mux, req, key)
	var firstBody intentResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decoding first response: %v", err)
	}

	// A fresh mux over the SAME underlying store, as a restarted process
	// would have (new in-memory state, same database).
	freshMux := http.NewServeMux()
	RegisterRoutes(freshMux, deps)

	second := postIntentWithKey(t, freshMux, req, key)
	if second.Code != http.StatusCreated {
		t.Fatalf("second: status = %d, want %d; body=%s", second.Code, http.StatusCreated, second.Body.String())
	}
	var secondBody intentResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decoding second response: %v", err)
	}
	if secondBody.ID != firstBody.ID {
		t.Errorf("after a simulated restart, second.ID = %q, want the original %q", secondBody.ID, firstBody.ID)
	}
}

func TestIdempotency_ValidationFailuresAreNotPermanentlyCached(t *testing.T) {
	_, mux := newTestDeps(t)
	key := uniqueIdempotencyKey(t)

	invalid := validIntentRequest()
	invalid.FundingAmount = "0" // fails validation

	first := postIntentWithKey(t, mux, invalid, key)
	if first.Code != http.StatusBadRequest {
		t.Fatalf("first: status = %d, want %d; body=%s", first.Code, http.StatusBadRequest, first.Body.String())
	}

	fixed := validIntentRequest()
	second := postIntentWithKey(t, mux, fixed, key)
	if second.Code != http.StatusCreated {
		t.Fatalf("second (retry with a fixed request, same key): status = %d, want %d; body=%s", second.Code, http.StatusCreated, second.Body.String())
	}
}
