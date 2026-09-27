package stellarrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// TransactionLookup is the narrow dependency WaitForTransaction needs, so
// tests can supply a controlled fixture instead of a real RPC client.
// *Client satisfies this.
type TransactionLookup interface {
	GetTransaction(ctx context.Context, hash string) (protocol.GetTransactionResponse, error)
}

// ErrTransactionFailed means getTransaction confirmed the transaction
// landed but failed on-chain.
var ErrTransactionFailed = errors.New("stellarrpc: transaction failed on-chain")

// TransactionTimeoutError means confirmation polling exhausted its
// attempts without a definitive SUCCESS/FAILED status. The transaction's
// fate is still unknown — it must be looked up again, never resubmitted.
type TransactionTimeoutError struct {
	Hash     string
	Attempts int
}

func (e *TransactionTimeoutError) Error() string {
	return fmt.Sprintf("stellarrpc: transaction %s was not confirmed after %d attempts", e.Hash, e.Attempts)
}

// ReconcileOptions bounds WaitForTransaction's polling.
type ReconcileOptions struct {
	Attempts int
	Interval time.Duration
	// Sleep is injectable for tests; defaults to a real time.Sleep-based wait.
	Sleep func(time.Duration)
}

// WaitForTransaction polls GetTransaction for a bounded number of attempts
// looking for a definitive SUCCESS or FAILED status.
//
//   - SUCCESS returns the full response, nil error.
//   - FAILED returns the full response and ErrTransactionFailed.
//   - NOT_FOUND after every attempt is exhausted returns a
//     *TransactionTimeoutError — the transaction must be looked up again
//     later; it must never be silently resubmitted, since a still-pending
//     submission could yet land.
func WaitForTransaction(ctx context.Context, lookup TransactionLookup, hash string, opts ReconcileOptions) (protocol.GetTransactionResponse, error) {
	sleep := opts.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	for attempt := 1; attempt <= opts.Attempts; attempt++ {
		response, err := lookup.GetTransaction(ctx, hash)
		if err != nil {
			return protocol.GetTransactionResponse{}, fmt.Errorf("looking up transaction %s (attempt %d): %w", hash, attempt, err)
		}

		switch response.Status {
		case protocol.TransactionStatusSuccess:
			return response, nil
		case protocol.TransactionStatusFailed:
			return response, ErrTransactionFailed
		}

		if attempt < opts.Attempts {
			sleep(opts.Interval)
		}
	}

	return protocol.GetTransactionResponse{}, &TransactionTimeoutError{Hash: hash, Attempts: opts.Attempts}
}
