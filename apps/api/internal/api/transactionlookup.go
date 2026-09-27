package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// Status values this endpoint reports. These mirror the possible outcomes
// of a real getTransaction lookup — never "confirmed" or "success" just
// because a submission was accepted as PENDING; that is a different,
// earlier, and strictly weaker signal than what this endpoint reports.
const (
	lookupStatusSuccess  = "success"
	lookupStatusFailed   = "failed"
	lookupStatusNotFound = "not_found"
)

type reconciliationInfo struct {
	Operation   string `json:"operation"`
	AgreementID string `json:"agreementId,omitempty"`
	FirstSeen   string `json:"firstSeen"`
	LastChecked string `json:"lastChecked,omitempty"`
	ConfirmedAt string `json:"confirmedAt,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	ErrorDetail string `json:"errorDetail,omitempty"`
}

type transactionLookupResponse struct {
	Hash           string              `json:"hash"`
	Status         string              `json:"status"`
	Ledger         string              `json:"ledger,omitempty"`
	Reconciliation *reconciliationInfo `json:"reconciliation,omitempty"`
}

func toReconciliationInfo(ref *store.TransactionRef) *reconciliationInfo {
	info := &reconciliationInfo{
		Operation: ref.Operation,
		FirstSeen: ref.FirstSeen.UTC().Format(time.RFC3339),
	}
	if ref.AgreementID != nil {
		info.AgreementID = strconv.FormatInt(*ref.AgreementID, 10)
	}
	if ref.LastChecked != nil {
		info.LastChecked = ref.LastChecked.UTC().Format(time.RFC3339)
	}
	if ref.ConfirmedAt != nil {
		info.ConfirmedAt = ref.ConfirmedAt.UTC().Format(time.RFC3339)
	}
	if ref.ErrorCode != nil {
		info.ErrorCode = *ref.ErrorCode
	}
	if ref.ErrorDetail != nil {
		info.ErrorDetail = *ref.ErrorDetail
	}
	return info
}

func registerTransactionLookupRoutes(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/v1/transactions/{hash}", func(w http.ResponseWriter, r *http.Request) {
		hash := r.PathValue("hash")
		if !isHex32(hash) {
			httpx.WriteValidationError(w, "invalid transaction hash", []httpx.FieldIssue{
				{Field: "hash", Issue: "must be 64 lowercase hex characters"},
			})
			return
		}

		// The live chain lookup is authoritative for status — a PENDING
		// submission result is never treated as confirmation, and neither
		// is any locally-cached status: this always asks the chain fresh.
		rpcResp, err := deps.RPC.GetTransaction(r.Context(), hash)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "GetTransaction")
			return
		}

		response := transactionLookupResponse{Hash: hash}
		var ledger *int64
		var confirmedAt *time.Time

		switch rpcResp.Status {
		case protocol.TransactionStatusSuccess:
			response.Status = lookupStatusSuccess
			l := int64(rpcResp.Ledger)
			ledger = &l
			response.Ledger = strconv.FormatInt(l, 10)
			now := time.Now().UTC()
			confirmedAt = &now
		case protocol.TransactionStatusFailed:
			response.Status = lookupStatusFailed
			l := int64(rpcResp.Ledger)
			ledger = &l
			response.Ledger = strconv.FormatInt(l, 10)
		default:
			response.Status = lookupStatusNotFound
		}

		ref, err := reconcileLocalTransactionRef(r.Context(), deps, hash, response.Status, ledger, confirmedAt)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "reconcileLocalTransactionRef")
			return
		}
		if ref != nil {
			response.Reconciliation = toReconciliationInfo(ref)
		}

		writeJSON(w, http.StatusOK, response)
	})
}

// reconcileLocalTransactionRef updates this backend's own transaction_refs
// record (if one exists for this hash) to match what the chain just
// reported, and returns the up-to-date record. Returns (nil, nil) if this
// backend never recorded this hash — a lookup for a hash we didn't submit
// is still a valid, useful request, it just carries no reconciliation
// metadata of ours.
//
// lookupStatus (success/failed/not_found, this endpoint's external
// vocabulary) is translated to transaction_refs' own stored vocabulary
// (pending/submitted/confirmed/failed): a NOT_FOUND chain response never
// overwrites an existing local status — it means "still not final",
// which "pending"/"submitted" already say — it only bumps last_checked
// so every reconciliation attempt stays visible.
func reconcileLocalTransactionRef(
	ctx context.Context,
	deps Deps,
	hash, lookupStatus string,
	ledger *int64,
	confirmedAt *time.Time,
) (*store.TransactionRef, error) {
	existing, err := deps.Store.GetTransactionByHash(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	storedStatus := existing.Status
	switch lookupStatus {
	case lookupStatusSuccess:
		storedStatus = "confirmed"
	case lookupStatusFailed:
		storedStatus = "failed"
	}

	if err := deps.Store.UpdateTransactionStatus(ctx, hash, storedStatus, ledger, confirmedAt, nil, nil); err != nil {
		return nil, err
	}

	return deps.Store.GetTransactionByHash(ctx, hash)
}
