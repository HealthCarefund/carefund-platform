// Package stellarrpc is CareFund's typed Soroban RPC access layer. It wraps
// github.com/stellar/go-stellar-sdk's rpcclient with explicit per-call
// timeouts and this repo's own submission/reconciliation rules: a
// transaction is submitted once, its hash recorded, and its fate is only
// ever learned by looking that hash up — never by resubmitting.
package stellarrpc

import (
	"context"
	"net/http"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// Client is the typed RPC access layer used by the rest of apps/api.
type Client struct {
	rpc     *rpcclient.Client
	timeout time.Duration
}

// New builds a Client against rpcURL. timeout bounds every individual RPC
// call (via context), independent of whatever deadline the caller's own
// context carries.
func New(rpcURL string, timeout time.Duration) *Client {
	return &Client{
		rpc:     rpcclient.NewClient(rpcURL, http.DefaultClient),
		timeout: timeout,
	}
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

// Close releases the underlying RPC client's resources.
func (c *Client) Close() error {
	return c.rpc.Close()
}

// Health checks Soroban RPC server health and ledger retention.
func (c *Client) Health(ctx context.Context) (protocol.GetHealthResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.GetHealth(ctx)
}

// Simulate simulates a base64-encoded transaction envelope without
// submitting it.
func (c *Client) Simulate(ctx context.Context, envelopeXDR string) (protocol.SimulateTransactionResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.SimulateTransaction(ctx, protocol.SimulateTransactionRequest{Transaction: envelopeXDR})
}

// Submission statuses, as returned by sendTransaction. Soroban RPC returns
// these as plain strings; the SDK does not export them as named
// constants, so they are declared here instead.
const (
	SubmissionPending       = "PENDING"
	SubmissionDuplicate     = "DUPLICATE"
	SubmissionTryAgainLater = "TRY_AGAIN_LATER"
	SubmissionError         = "ERROR"
)

// Submit sends a signed, base64-encoded transaction envelope exactly once.
// The caller is responsible for recording the returned hash before doing
// anything else with it — this method never retries or resubmits.
func (c *Client) Submit(ctx context.Context, signedEnvelopeXDR string) (protocol.SendTransactionResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.SendTransaction(ctx, protocol.SendTransactionRequest{Transaction: signedEnvelopeXDR})
}

// GetTransaction looks up a transaction by hash exactly once. This is the
// only way this package reconciles an uncertain submission.
func (c *Client) GetTransaction(ctx context.Context, hash string) (protocol.GetTransactionResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.GetTransaction(ctx, protocol.GetTransactionRequest{Hash: hash})
}

// GetLedgerEntries fetches ledger entries by their base64-encoded XDR
// LedgerKey values.
func (c *Client) GetLedgerEntries(ctx context.Context, keysBase64 []string) (protocol.GetLedgerEntriesResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{Keys: keysBase64})
}

// LoadAccount fetches an account's current sequence number, for building
// a transaction with that account as the source.
func (c *Client) LoadAccount(ctx context.Context, address string) (txnbuild.Account, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rpc.LoadAccount(ctx, address)
}
