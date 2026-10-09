package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/sorobanenc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

type providersPageResponse struct {
	Providers  []providerResponse `json:"providers"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

type attestersPageResponse struct {
	Attesters  []attesterResponse `json:"attesters"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

type providerResponse struct {
	WalletAddress string `json:"walletAddress"`
	ProviderRef   string `json:"providerRef"`
	Status        string `json:"status"`
	UpdatedAt     string `json:"updatedAt"`
}

func toProviderResponse(p *store.Provider) providerResponse {
	return providerResponse{
		WalletAddress: p.WalletAddress,
		ProviderRef:   hex.EncodeToString(p.ProviderRef),
		Status:        p.Status,
		UpdatedAt:     p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type attesterResponse struct {
	WalletAddress  string `json:"walletAddress"`
	ProviderWallet string `json:"providerWallet"`
	CredentialRef  string `json:"credentialRef"`
	Status         string `json:"status"`
	UpdatedAt      string `json:"updatedAt"`
}

func toAttesterResponse(a store.Attester) attesterResponse {
	return attesterResponse{
		WalletAddress:  a.WalletAddress,
		ProviderWallet: a.ProviderWallet,
		CredentialRef:  hex.EncodeToString(a.CredentialRef),
		Status:         a.Status,
		UpdatedAt:      a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// registerProviderRoutes also serves the public provider/attester
// directory (GET /api/v1/providers, GET /api/v1/attesters) - not part of
// the original approved endpoint list, but needed for the Unit R admin
// directory views (/admin/providers, /admin/attesters), which have no
// other way to discover which wallets exist off a single known wallet.
// Same read-only, off-chain mirror shape and cursor-pagination
// convention as every other listing endpoint already built.
func registerProviderRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/v1/providers", func(w http.ResponseWriter, r *http.Request) {
		cursor, ok := parseEventsCursor(w, r.URL.Query().Get("cursor"))
		if !ok {
			return
		}
		limit, ok := parseEventsLimit(w, r.URL.Query().Get("limit"))
		if !ok {
			return
		}
		page, err := deps.Store.ListProviders(r.Context(), cursor, limit)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListProviders")
			return
		}
		response := providersPageResponse{
			Providers:  make([]providerResponse, 0, len(page.Providers)),
			NextCursor: page.NextCursor,
		}
		for i := range page.Providers {
			response.Providers = append(response.Providers, toProviderResponse(&page.Providers[i]))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/attesters", func(w http.ResponseWriter, r *http.Request) {
		cursor, ok := parseEventsCursor(w, r.URL.Query().Get("cursor"))
		if !ok {
			return
		}
		limit, ok := parseEventsLimit(w, r.URL.Query().Get("limit"))
		if !ok {
			return
		}
		page, err := deps.Store.ListAttesters(r.Context(), cursor, limit)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListAttesters")
			return
		}
		response := attestersPageResponse{
			Attesters:  make([]attesterResponse, 0, len(page.Attesters)),
			NextCursor: page.NextCursor,
		}
		for i := range page.Attesters {
			response.Attesters = append(response.Attesters, toAttesterResponse(page.Attesters[i]))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/providers/{wallet}", func(w http.ResponseWriter, r *http.Request) {
		wallet := r.PathValue("wallet")
		if !isStellarAccountAddress(wallet) {
			httpx.WriteValidationError(w, "invalid provider wallet address", []httpx.FieldIssue{
				{Field: "wallet", Issue: "must be a valid Stellar account address (G...)"},
			})
			return
		}

		provider, err := ensureProvider(r.Context(), deps, wallet)
		if errors.Is(err, store.ErrNotFound) {
			httpx.WriteNotFound(w, "provider not found")
			return
		}
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "GetProviderByWallet")
			return
		}

		writeJSON(w, http.StatusOK, toProviderResponse(provider))
	})

	mux.HandleFunc("GET /api/v1/providers/{wallet}/attesters", func(w http.ResponseWriter, r *http.Request) {
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

		attesters, err := deps.Store.ListAttestersByProvider(r.Context(), wallet)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "ListAttestersByProvider")
			return
		}

		response := make([]attesterResponse, 0, len(attesters))
		for _, a := range attesters {
			response = append(response, toAttesterResponse(a))
		}
		writeJSON(w, http.StatusOK, response)
	})
}

func ensureProvider(ctx context.Context, deps Deps, wallet string) (*store.Provider, error) {
	provider, err := deps.Store.GetProviderByWallet(ctx, wallet)
	if errors.Is(err, store.ErrNotFound) && deps.RPC != nil && deps.Config != nil {
		if onChain, fetchErr := fetchOnChainProvider(ctx, deps, wallet); fetchErr == nil && onChain != nil {
			_ = deps.Store.UpsertProvider(ctx, onChain.WalletAddress, onChain.ProviderRef, onChain.Status)
			return onChain, nil
		}
	}
	return provider, err
}

func fetchOnChainProvider(ctx context.Context, deps Deps, wallet string) (*store.Provider, error) {
	if deps.Config.ProviderRegistryContractID == "" {
		return nil, errors.New("provider registry contract ID not configured")
	}
	txBase64, err := sorobanenc.BuildGetProviderTransaction(deps.Config.ProviderRegistryContractID, wallet)
	if err != nil {
		return nil, err
	}
	sim, err := deps.RPC.Simulate(ctx, txBase64)
	if err != nil || sim.Error != "" || len(sim.Results) == 0 || sim.Results[0].ReturnValueXDR == nil {
		return nil, errors.New("simulation failed or returned empty result")
	}
	var retVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(*sim.Results[0].ReturnValueXDR, &retVal); err != nil {
		return nil, err
	}
	return sorobanenc.DecodeProvider(retVal, wallet)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
