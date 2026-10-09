package api

import (
	"errors"
	"net/http"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

type agreementsPageResponse struct {
	Agreements []agreementResponse `json:"agreements"`
	NextCursor string              `json:"nextCursor,omitempty"`
}

func toAgreementsPageResponse(page store.AgreementPage) agreementsPageResponse {
	response := agreementsPageResponse{
		Agreements: make([]agreementResponse, 0, len(page.Agreements)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Agreements {
		response.Agreements = append(response.Agreements, toAgreementResponse(&page.Agreements[i]))
	}
	return response
}

// registerWalletAgreementRoutes adds the agreement-listing endpoints the
// provider and sponsor dashboards need (later units) — GET
// /api/v1/agreements/{id} only supports looking up one already-known id,
// with no way to discover which agreements a given wallet is party to.
// Not part of the original approved endpoint list (Section 6 lists only
// single-agreement lookup); added here because without it those explicitly
// required frontend routes cannot function. Same read-only, off-chain
// mirror shape and pagination convention as every other listing endpoint
// already built (attesters, events) - no new domain concept.
func registerWalletAgreementRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/v1/providers/{wallet}/agreements", func(w http.ResponseWriter, r *http.Request) {
		wallet := r.PathValue("wallet")
		if !isStellarAccountAddress(wallet) {
			httpx.WriteValidationError(w, "invalid provider wallet address", []httpx.FieldIssue{
				{Field: "wallet", Issue: "must be a valid Stellar account address (G...)"},
			})
			return
		}
		if _, err := ensureProvider(r.Context(), deps, wallet); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				httpx.WriteNotFound(w, "provider not found")
				return
			}
			httpx.WriteInternal(w, deps.Logger, err, "GetProviderByWallet")
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

		page, err := deps.Store.ListAgreementsByProviderWallet(r.Context(), wallet, cursor, limit)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListAgreementsByProviderWallet")
			return
		}
		writeJSON(w, http.StatusOK, toAgreementsPageResponse(page))
	})

	mux.HandleFunc("GET /api/v1/sponsors/{wallet}/agreements", func(w http.ResponseWriter, r *http.Request) {
		wallet := r.PathValue("wallet")
		if !isStellarAccountAddress(wallet) {
			httpx.WriteValidationError(w, "invalid sponsor wallet address", []httpx.FieldIssue{
				{Field: "wallet", Issue: "must be a valid Stellar account address (G...)"},
			})
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

		// Sponsors are not a registered/verified actor on-chain the way
		// providers are (there is no sponsor registry to check membership
		// against), so - unlike the provider listing above - there is no
		// existence check to perform first: any wallet may sponsor an
		// agreement, and an empty result for a wallet that has never
		// sponsored one is a perfectly valid answer, not a 404.
		page, err := deps.Store.ListAgreementsBySponsorWallet(r.Context(), wallet, cursor, limit)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListAgreementsBySponsorWallet")
			return
		}
		writeJSON(w, http.StatusOK, toAgreementsPageResponse(page))
	})
}
