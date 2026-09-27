package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/sorobanenc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// prepareTransactionRequest is deliberately closed (DisallowUnknownFields):
// only the fields a given operation actually needs should ever be sent.
type prepareTransactionRequest struct {
	Operation             string `json:"operation"`
	SourcePublicKey       string `json:"sourcePublicKey"`
	AttestationCommitment string `json:"attestationCommitment,omitempty"`
	Resolution            string `json:"resolution,omitempty"`
}

type prepareTransactionResponse struct {
	UnsignedTransactionXDR string `json:"unsignedTransactionXdr"`
	Network                string `json:"network"`
	NetworkPassphrase      string `json:"networkPassphrase"`
}

const transactionPrepTimeoutSeconds = 120

// The operations this endpoint can prepare, and the on-chain method +
// argument shape each one maps to exactly (see contracts/care-agreement).
// create_agreement is deliberately not here: it belongs to an
// agreement_intent, not an existing agreementId, so it doesn't fit this
// endpoint's URL shape and isn't part of the approved spec for Unit J.
const (
	operationFund           = "fund"
	operationCancel         = "cancel"
	operationAttestCare     = "attest_care"
	operationOpenDispute    = "open_dispute"
	operationExpire         = "expire"
	operationSettle         = "settle"
	operationResolveDispute = "resolve_dispute"
)

const operationPrepareTransaction = "prepare_agreement_transaction"

func registerTransactionPrepRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("POST /api/v1/agreements/{agreementId}/transactions", withIdempotency(deps, operationPrepareTransaction,
		func(w http.ResponseWriter, r *http.Request, body []byte) (int, any) {
			id, ok := parseAgreementID(w, r.PathValue("agreementId"))
			if !ok {
				return 0, nil
			}

			agreement, err := deps.Store.GetAgreement(r.Context(), id)
			if errors.Is(err, store.ErrNotFound) {
				httpx.WriteNotFound(w, "agreement not found")
				return 0, nil
			}
			if err != nil {
				httpx.WriteInternal(w, deps.Logger, err, "GetAgreement")
				return 0, nil
			}

			var req prepareTransactionRequest
			decoder := json.NewDecoder(bytes.NewReader(body))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&req); err != nil {
				httpx.WriteValidationError(w, "malformed request body", []httpx.FieldIssue{
					{Field: "body", Issue: err.Error()},
				})
				return 0, nil
			}

			args, issues := buildOperationArgs(id, agreement, req)
			if len(issues) > 0 {
				httpx.WriteValidationError(w, "invalid transaction preparation request", issues)
				return 0, nil
			}

			unsignedXDR, err := sorobanenc.PrepareContractCall(r.Context(), deps.RPC, deps.RPC, sorobanenc.ContractCallRequest{
				ContractID:        deps.Config.CareAgreementContractID,
				Method:            req.Operation,
				Args:              args,
				SourcePublicKey:   req.SourcePublicKey,
				NetworkPassphrase: deps.Config.StellarNetworkPassphrase,
				TimeoutSeconds:    transactionPrepTimeoutSeconds,
			})
			var simErr *sorobanenc.SimulationError
			if errors.As(err, &simErr) {
				httpx.WriteContractError(w, simErr.Error())
				return 0, nil
			}
			if err != nil {
				httpx.WriteInternal(w, deps.Logger, err, "PrepareContractCall")
				return 0, nil
			}

			return http.StatusOK, prepareTransactionResponse{
				UnsignedTransactionXDR: unsignedXDR,
				Network:                string(deps.Config.StellarNetwork),
				NetworkPassphrase:      deps.Config.StellarNetworkPassphrase,
			}
		}))
}

// buildOperationArgs validates the request against the current agreement
// state and the exact argument shape the requested contract method
// expects, encoding those arguments only once everything checks out. The
// on-chain require_auth/state checks remain authoritative regardless —
// this only avoids obviously-wrong preparation attempts.
func buildOperationArgs(agreementID int64, agreement *store.CareAgreement, req prepareTransactionRequest) ([]xdr.ScVal, []httpx.FieldIssue) {
	var issues []httpx.FieldIssue

	if !isStellarAccountAddress(req.SourcePublicKey) {
		issues = append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: "must be a valid Stellar account address (G...)"})
	}

	idArg := sorobanenc.U64(uint64(agreementID))

	switch req.Operation {
	case operationFund:
		requireWallet(&issues, "sourcePublicKey", req.SourcePublicKey, agreement.SponsorWallet, "sponsor")
		if len(issues) > 0 {
			return nil, issues
		}
		sponsorArg, err := sorobanenc.Address(req.SourcePublicKey)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: err.Error()})
		}
		return []xdr.ScVal{idArg, sponsorArg}, nil

	case operationCancel:
		requireWallet(&issues, "sourcePublicKey", req.SourcePublicKey, agreement.ProviderWallet, "provider")
		if len(issues) > 0 {
			return nil, issues
		}
		providerArg, err := sorobanenc.Address(req.SourcePublicKey)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: err.Error()})
		}
		return []xdr.ScVal{idArg, providerArg}, nil

	case operationSettle:
		requireWallet(&issues, "sourcePublicKey", req.SourcePublicKey, agreement.SponsorWallet, "sponsor")
		if len(issues) > 0 {
			return nil, issues
		}
		sponsorArg, err := sorobanenc.Address(req.SourcePublicKey)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: err.Error()})
		}
		return []xdr.ScVal{idArg, sponsorArg}, nil

	case operationOpenDispute:
		if req.SourcePublicKey != agreement.SponsorWallet && req.SourcePublicKey != agreement.ProviderWallet {
			issues = append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: "must be the agreement's sponsor or provider"})
		}
		if len(issues) > 0 {
			return nil, issues
		}
		openerArg, err := sorobanenc.Address(req.SourcePublicKey)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: err.Error()})
		}
		return []xdr.ScVal{idArg, openerArg}, nil

	case operationExpire:
		// Permissionless on-chain: any funded account may submit it.
		if len(issues) > 0 {
			return nil, issues
		}
		return []xdr.ScVal{idArg}, nil

	case operationAttestCare:
		requireWallet(&issues, "sourcePublicKey", req.SourcePublicKey, agreement.AttesterWallet, "attester")
		if !isHex32(req.AttestationCommitment) {
			issues = append(issues, httpx.FieldIssue{Field: "attestationCommitment", Issue: "must be 64 lowercase hex characters (32 bytes)"})
		}
		if len(issues) > 0 {
			return nil, issues
		}
		attesterArg, err := sorobanenc.Address(req.SourcePublicKey)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "sourcePublicKey", Issue: err.Error()})
		}
		commitmentBytes, _ := hex.DecodeString(req.AttestationCommitment)
		commitmentArg, err := sorobanenc.Bytes32(commitmentBytes)
		if err != nil {
			return nil, append(issues, httpx.FieldIssue{Field: "attestationCommitment", Issue: err.Error()})
		}
		return []xdr.ScVal{idArg, attesterArg, commitmentArg}, nil

	case operationResolveDispute:
		if !isStellarAccountAddress(req.SourcePublicKey) {
			return nil, issues
		}
		switch req.Resolution {
		case "Resume", "Settle", "Refund":
		default:
			issues = append(issues, httpx.FieldIssue{Field: "resolution", Issue: "must be one of Resume, Settle, Refund"})
		}
		if len(issues) > 0 {
			return nil, issues
		}
		return []xdr.ScVal{idArg, sorobanenc.Symbol(req.Resolution)}, nil

	default:
		return nil, append(issues, httpx.FieldIssue{
			Field: "operation",
			Issue: "must be one of fund, cancel, attest_care, open_dispute, expire, settle, resolve_dispute",
		})
	}
}

func requireWallet(issues *[]httpx.FieldIssue, field, got, want, role string) {
	if got != want {
		*issues = append(*issues, httpx.FieldIssue{Field: field, Issue: "must be the agreement's " + role + " wallet"})
	}
}
