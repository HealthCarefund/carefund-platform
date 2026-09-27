package api

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// createIntentRequest is deliberately closed: json.Decoder rejects any
// field not listed here, so an accidental extra field (which could be an
// attempt to smuggle clinical data through this endpoint) is a 400, not a
// silently-ignored no-op.
type createIntentRequest struct {
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
}

type intentResponse struct {
	ID                        string `json:"id"`
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
	Status                    string `json:"status"`
	AgreementID               string `json:"agreementId,omitempty"`
	CreatedAt                 string `json:"createdAt"`
}

func toIntentResponse(in *store.AgreementIntent) intentResponse {
	resp := intentResponse{
		ID:                        strconv.FormatInt(in.ID, 10),
		SponsorWallet:             in.SponsorWallet,
		ProviderWallet:            in.ProviderWallet,
		AttesterWallet:            in.AttesterWallet,
		PatientRefCommitment:      hex.EncodeToString(in.PatientRefCommitment),
		ServiceCommitment:         hex.EncodeToString(in.ServiceCommitment),
		FundingAmount:             in.FundingAmount,
		SettlementAmount:          in.SettlementAmount,
		SettlementAssetContractID: in.SettlementAssetContractID,
		FundingDeadline:           strconv.FormatInt(in.FundingDeadline, 10),
		CareDeadline:              strconv.FormatInt(in.CareDeadline, 10),
		DisputeWindowSecs:         strconv.FormatInt(in.DisputeWindowSecs, 10),
		Status:                    in.Status,
		CreatedAt:                 in.CreatedAt.UTC().Format(time.RFC3339),
	}
	if in.AgreementID != nil {
		resp.AgreementID = strconv.FormatInt(*in.AgreementID, 10)
	}
	return resp
}

func registerIntentRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("POST /api/v1/agreements/intents", func(w http.ResponseWriter, r *http.Request) {
		var req createIntentRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			httpx.WriteValidationError(w, "malformed request body", []httpx.FieldIssue{
				{Field: "body", Issue: err.Error()},
			})
			return
		}

		issues := validateCreateIntentRequest(req)
		if len(issues) > 0 {
			httpx.WriteValidationError(w, "invalid agreement intent", issues)
			return
		}

		patientRef, _ := hex.DecodeString(req.PatientRefCommitment)
		serviceRef, _ := hex.DecodeString(req.ServiceCommitment)
		fundingDeadline, _ := strconv.ParseInt(req.FundingDeadline, 10, 64)
		careDeadline, _ := strconv.ParseInt(req.CareDeadline, 10, 64)
		disputeWindow, _ := strconv.ParseInt(req.DisputeWindowSecs, 10, 64)

		created, err := deps.Store.CreateAgreementIntent(r.Context(), store.AgreementIntent{
			SponsorWallet:             req.SponsorWallet,
			ProviderWallet:            req.ProviderWallet,
			AttesterWallet:            req.AttesterWallet,
			PatientRefCommitment:      patientRef,
			ServiceCommitment:         serviceRef,
			FundingAmount:             req.FundingAmount,
			SettlementAmount:          req.SettlementAmount,
			SettlementAssetContractID: req.SettlementAssetContractID,
			FundingDeadline:           fundingDeadline,
			CareDeadline:              careDeadline,
			DisputeWindowSecs:         disputeWindow,
		})
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "CreateAgreementIntent")
			return
		}

		writeJSON(w, http.StatusCreated, toIntentResponse(created))
	})
}

// validateCreateIntentRequest checks format, then the same amount/deadline
// ordering rules the create_agreement contract method itself enforces
// (funding_amount > 0, settlement_amount > 0, settlement_amount <=
// funding_amount, funding_deadline in the future, care_deadline after
// funding_deadline) — failing fast here does not replace that on-chain
// check, which remains authoritative when the intent is later turned into
// a real transaction.
func validateCreateIntentRequest(req createIntentRequest) []httpx.FieldIssue {
	var issues []httpx.FieldIssue

	addr := func(field, value string) {
		if !isStellarAccountAddress(value) {
			issues = append(issues, httpx.FieldIssue{Field: field, Issue: "must be a valid Stellar account address (G...)"})
		}
	}
	addr("sponsorWallet", req.SponsorWallet)
	addr("providerWallet", req.ProviderWallet)
	addr("attesterWallet", req.AttesterWallet)

	if !isHex32(req.PatientRefCommitment) {
		issues = append(issues, httpx.FieldIssue{Field: "patientRefCommitment", Issue: "must be 64 lowercase hex characters (32 bytes)"})
	}
	if !isHex32(req.ServiceCommitment) {
		issues = append(issues, httpx.FieldIssue{Field: "serviceCommitment", Issue: "must be 64 lowercase hex characters (32 bytes)"})
	}
	if !isStellarContractAddress(req.SettlementAssetContractID) {
		issues = append(issues, httpx.FieldIssue{Field: "settlementAssetContractId", Issue: "must be a valid Soroban contract address (C...)"})
	}

	fundingOK := isPositiveI128(req.FundingAmount)
	if !fundingOK {
		issues = append(issues, httpx.FieldIssue{Field: "fundingAmount", Issue: "must be a positive integer within the i128 range"})
	}
	settlementOK := isPositiveI128(req.SettlementAmount)
	if !settlementOK {
		issues = append(issues, httpx.FieldIssue{Field: "settlementAmount", Issue: "must be a positive integer within the i128 range"})
	}
	if fundingOK && settlementOK {
		funding, _ := new(big.Int).SetString(req.FundingAmount, 10)
		settlement, _ := new(big.Int).SetString(req.SettlementAmount, 10)
		if settlement.Cmp(funding) > 0 {
			issues = append(issues, httpx.FieldIssue{Field: "settlementAmount", Issue: "must not exceed fundingAmount"})
		}
	}

	fundingDeadlineOK := isStoredU64(req.FundingDeadline)
	if !fundingDeadlineOK {
		issues = append(issues, httpx.FieldIssue{Field: "fundingDeadline", Issue: "must be a non-negative integer within the u64 range"})
	}
	careDeadlineOK := isStoredU64(req.CareDeadline)
	if !careDeadlineOK {
		issues = append(issues, httpx.FieldIssue{Field: "careDeadline", Issue: "must be a non-negative integer within the u64 range"})
	}
	if !isStoredU64(req.DisputeWindowSecs) {
		issues = append(issues, httpx.FieldIssue{Field: "disputeWindowSecs", Issue: "must be a non-negative integer within the u64 range"})
	}

	if fundingDeadlineOK {
		fundingDeadline, _ := strconv.ParseInt(req.FundingDeadline, 10, 64)
		if fundingDeadline <= time.Now().Unix() {
			issues = append(issues, httpx.FieldIssue{Field: "fundingDeadline", Issue: "must be in the future"})
		}
		if careDeadlineOK {
			careDeadline, _ := strconv.ParseInt(req.CareDeadline, 10, 64)
			if careDeadline <= fundingDeadline {
				issues = append(issues, httpx.FieldIssue{Field: "careDeadline", Issue: "must be after fundingDeadline"})
			}
		}
	}

	return issues
}
