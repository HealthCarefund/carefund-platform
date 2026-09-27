package reconcile

import (
	"context"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// reconcileTransactions looks up every locally-known pending/submitted
// transaction and updates its stored status to match the chain. A lookup
// failure for one hash (RPC hiccup, timeout) is logged and skipped —
// it does not abort reconciling the rest, and it is retried on the next
// tick since the transaction stays "pending"/"submitted" until resolved.
func (r *Runner) reconcileTransactions(ctx context.Context) {
	pending, err := r.store.ListPendingTransactions(ctx)
	if err != nil {
		r.logger.Error("reconcile: listing pending transactions failed", "error", err)
		return
	}

	for _, ref := range pending {
		resp, err := r.rpc.GetTransaction(ctx, ref.TxHash)
		if err != nil {
			r.logger.Warn("reconcile: transaction lookup failed, will retry next tick",
				"hash", ref.TxHash, "error", err)
			continue
		}

		var status string
		var ledger *int64
		var confirmedAt *time.Time
		switch resp.Status {
		case protocol.TransactionStatusSuccess:
			status = "confirmed"
			l := int64(resp.Ledger)
			ledger = &l
			now := time.Now().UTC()
			confirmedAt = &now
		case protocol.TransactionStatusFailed:
			status = "failed"
			l := int64(resp.Ledger)
			ledger = &l
		default:
			// Still NOT_FOUND: leave status as-is, just bump last_checked
			// so the attempt is visible. Never resubmitted from here or
			// anywhere else — only ever looked up again.
			status = ref.Status
		}

		if err := r.store.UpdateTransactionStatus(ctx, ref.TxHash, status, ledger, confirmedAt, nil, nil); err != nil {
			r.logger.Error("reconcile: updating transaction status failed", "hash", ref.TxHash, "error", err)
		}
	}
}
