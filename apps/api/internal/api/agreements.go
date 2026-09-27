package api

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

type agreementResponse struct {
	AgreementID               string `json:"agreementId"`
	SponsorWallet             string `json:"sponsorWallet"`
	ProviderWallet            string `json:"providerWallet"`
	AttesterWallet            string `json:"attesterWallet"`
	PatientRefCommitment      string `json:"patientRefCommitment"`
	ServiceCommitment         string `json:"serviceCommitment"`
	FundingAmount             string `json:"fundingAmount"`
	SettlementAmount          string `json:"settlementAmount"`
	SettlementAssetContractID string `json:"settlementAssetContractId"`
	FundingDeadline           string `json:"fundingDeadline"`
	CareDeadline              string `json:"careDeadline"`
	DisputeWindowSecs         string `json:"disputeWindowSecs"`
	State                     string `json:"state"`
	UpdatedAt                 string `json:"updatedAt"`
}

func toAgreementResponse(a *store.CareAgreement) agreementResponse {
	return agreementResponse{
		AgreementID:               strconv.FormatInt(a.AgreementID, 10),
		SponsorWallet:             a.SponsorWallet,
		ProviderWallet:            a.ProviderWallet,
		AttesterWallet:            a.AttesterWallet,
		PatientRefCommitment:      hex.EncodeToString(a.PatientRefCommitment),
		ServiceCommitment:         hex.EncodeToString(a.ServiceCommitment),
		FundingAmount:             a.FundingAmount,
		SettlementAmount:          a.SettlementAmount,
		SettlementAssetContractID: a.SettlementAssetContractID,
		FundingDeadline:           strconv.FormatInt(a.FundingDeadline, 10),
		CareDeadline:              strconv.FormatInt(a.CareDeadline, 10),
		DisputeWindowSecs:         strconv.FormatInt(a.DisputeWindowSecs, 10),
		State:                     a.State,
		UpdatedAt:                 a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type contractEventResponse struct {
	ID          string `json:"id"`
	ContractID  string `json:"contractId"`
	TxHash      string `json:"txHash"`
	EventIndex  int32  `json:"eventIndex"`
	EventType   string `json:"eventType"`
	AgreementID string `json:"agreementId,omitempty"`
	Ledger      string `json:"ledger"`
	ObservedAt  string `json:"observedAt"`
}

func toContractEventResponse(e store.ContractEvent) contractEventResponse {
	resp := contractEventResponse{
		ID:         strconv.FormatInt(e.ID, 10),
		ContractID: e.ContractID,
		TxHash:     e.TxHash,
		EventIndex: e.EventIndex,
		EventType:  e.EventType,
		Ledger:     strconv.FormatInt(e.Ledger, 10),
		ObservedAt: e.ObservedAt.UTC().Format(time.RFC3339),
	}
	if e.AgreementID != nil {
		resp.AgreementID = strconv.FormatInt(*e.AgreementID, 10)
	}
	return resp
}

type eventsPageResponse struct {
	Events     []contractEventResponse `json:"events"`
	NextCursor string                  `json:"nextCursor,omitempty"`
}

const defaultEventsPageLimit = 50

func registerAgreementRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/v1/agreements/{agreementId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseAgreementID(w, r.PathValue("agreementId"))
		if !ok {
			return
		}

		agreement, err := deps.Store.GetAgreement(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteNotFound(w, "agreement not found")
			return
		}
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "GetAgreement")
			return
		}

		writeJSON(w, http.StatusOK, toAgreementResponse(agreement))
	})

	mux.HandleFunc("GET /api/v1/agreements/{agreementId}/events", func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseAgreementID(w, r.PathValue("agreementId"))
		if !ok {
			return
		}

		if _, err := deps.Store.GetAgreement(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				httpx.WriteNotFound(w, "agreement not found")
				return
			}
			httpx.WriteInternal(w, deps.Logger, err, "GetAgreement")
			return
		}

		cursor, ok := parseEventsCursor(w, r.URL.Query().Get("cursor"))
		if !ok {
			return
		}
		limit, ok := parseEventsLimit(w, r.URL.Query().Get("limit"))
		if !ok {
			return
		}

		page, err := deps.Store.ListEventsForAgreement(r.Context(), id, cursor, limit)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListEventsForAgreement")
			return
		}

		response := eventsPageResponse{Events: make([]contractEventResponse, 0, len(page.Events)), NextCursor: page.NextCursor}
		for _, e := range page.Events {
			response.Events = append(response.Events, toContractEventResponse(e))
		}
		writeJSON(w, http.StatusOK, response)
	})
}

func parseAgreementID(w http.ResponseWriter, raw string) (int64, bool) {
	if !isAgreementIDFormat(raw) {
		httpx.WriteValidationError(w, "invalid agreement id", []httpx.FieldIssue{
			{Field: "agreementId", Issue: "must be a non-negative integer with no leading zeros"},
		})
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		httpx.WriteValidationError(w, "invalid agreement id", []httpx.FieldIssue{
			{Field: "agreementId", Issue: "must fit in a 64-bit integer"},
		})
		return 0, false
	}
	return id, true
}

func parseEventsCursor(w http.ResponseWriter, raw string) (int64, bool) {
	if raw == "" {
		return 0, true
	}
	cursor, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || cursor < 0 {
		httpx.WriteValidationError(w, "invalid cursor", []httpx.FieldIssue{
			{Field: "cursor", Issue: "must be a non-negative integer"},
		})
		return 0, false
	}
	return cursor, true
}

func parseEventsLimit(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return defaultEventsPageLimit, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		httpx.WriteValidationError(w, "invalid limit", []httpx.FieldIssue{
			{Field: "limit", Issue: "must be a positive integer"},
		})
		return 0, false
	}
	return limit, true
}
