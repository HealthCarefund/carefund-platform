package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

func validIntentRequest() createIntentRequest {
	future := time.Now().Add(48 * time.Hour).Unix()
	return createIntentRequest{
		SponsorWallet:             "G" + repeatChar('S', 55),
		ProviderWallet:            "G" + repeatChar('P', 55),
		AttesterWallet:            "G" + repeatChar('T', 55),
		PatientRefCommitment:      strings.Repeat("a", 64),
		ServiceCommitment:         strings.Repeat("b", 64),
		FundingAmount:             "1000000",
		SettlementAmount:          "900000",
		SettlementAssetContractID: "C" + repeatChar('X', 55),
		FundingDeadline:           itoa64(future),
		CareDeadline:              itoa64(future + 3600),
		DisputeWindowSecs:         "86400",
	}
}

func itoa64(v int64) string {
	return jsonNumber(v)
}

func jsonNumber(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func postIntent(t *testing.T, mux http.Handler, req createIntentRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/agreements/intents", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(idempotencyKeyHeader, uniqueIdempotencyKey(t))
	mux.ServeHTTP(rec, httpReq)
	return rec
}

func TestCreateIntent_Success(t *testing.T) {
	_, mux := newTestDeps(t)
	req := validIntentRequest()

	rec := postIntent(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body intentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.ID == "" || body.Status != "pending" || body.AgreementID != "" {
		t.Errorf("body = %+v", body)
	}
	if body.SponsorWallet != req.SponsorWallet || body.FundingAmount != req.FundingAmount {
		t.Errorf("body = %+v, want it to echo the request", body)
	}
}

func TestCreateIntent_DoesNotCreateAnOnChainAgreement(t *testing.T) {
	deps, mux := newTestDeps(t)
	req := validIntentRequest()

	rec := postIntent(t, mux, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// The care_agreements mirror table must remain untouched by this
	// endpoint — creating an intent is purely an off-chain record, never a
	// call to create_agreement.
	if _, err := deps.Store.GetAgreement(context.Background(), 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetAgreement(1) err = %v, want ErrNotFound (no on-chain agreement should exist)", err)
	}
}

func TestCreateIntent_RejectsUnknownFields(t *testing.T) {
	_, mux := newTestDeps(t)
	body := []byte(`{"sponsorWallet":"G","extraField":"nope"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agreements/intents", bytes.NewReader(body))
	req.Header.Set(idempotencyKeyHeader, uniqueIdempotencyKey(t))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateIntent_ValidatesEachField(t *testing.T) {
	base := validIntentRequest()
	cases := map[string]createIntentRequest{
		"bad sponsor address":   withField(base, func(r *createIntentRequest) { r.SponsorWallet = "not-an-address" }),
		"bad provider address":  withField(base, func(r *createIntentRequest) { r.ProviderWallet = "not-an-address" }),
		"bad attester address":  withField(base, func(r *createIntentRequest) { r.AttesterWallet = "not-an-address" }),
		"short patient ref":     withField(base, func(r *createIntentRequest) { r.PatientRefCommitment = "abc" }),
		"uppercase service ref": withField(base, func(r *createIntentRequest) { r.ServiceCommitment = strings.ToUpper(strings.Repeat("b", 64)) }),
		"bad settlement asset":  withField(base, func(r *createIntentRequest) { r.SettlementAssetContractID = "GNOTACONTRACT" }),
		"zero funding amount":   withField(base, func(r *createIntentRequest) { r.FundingAmount = "0" }),
		"negative settlement":   withField(base, func(r *createIntentRequest) { r.SettlementAmount = "-1" }),
		"settlement exceeds fund": withField(base, func(r *createIntentRequest) {
			r.FundingAmount = "100"
			r.SettlementAmount = "200"
		}),
		"funding deadline in past": withField(base, func(r *createIntentRequest) {
			r.FundingDeadline = itoa64(time.Now().Add(-time.Hour).Unix())
		}),
		"care deadline before funding": withField(base, func(r *createIntentRequest) {
			r.CareDeadline = r.FundingDeadline
		}),
	}

	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, mux := newTestDeps(t)
			rec := postIntent(t, mux, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

func withField(base createIntentRequest, mutate func(*createIntentRequest)) createIntentRequest {
	r := base
	mutate(&r)
	return r
}
