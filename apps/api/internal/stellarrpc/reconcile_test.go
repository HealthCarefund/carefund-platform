package stellarrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// These tests exercise this package's own polling/status-mapping logic
// against a controlled in-memory fixture — no RPC call, network, or
// blockchain interaction happens anywhere in this file.

type fakeLookup struct {
	responses []protocol.GetTransactionResponse
	errs      []error
	calls     int
}

func (f *fakeLookup) GetTransaction(_ context.Context, _ string) (protocol.GetTransactionResponse, error) {
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return protocol.GetTransactionResponse{}, f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return f.responses[len(f.responses)-1], nil
}

func noSleep(time.Duration) {}

func TestWaitForTransaction_ResolvesOnSuccess(t *testing.T) {
	lookup := &fakeLookup{responses: []protocol.GetTransactionResponse{
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusSuccess}},
	}}

	resp, err := WaitForTransaction(context.Background(), lookup, "hash1", ReconcileOptions{Attempts: 5, Interval: 0, Sleep: noSleep})
	if err != nil {
		t.Fatalf("WaitForTransaction: %v", err)
	}
	if resp.Status != protocol.TransactionStatusSuccess {
		t.Errorf("Status = %q", resp.Status)
	}
	if lookup.calls != 1 {
		t.Errorf("calls = %d, want 1", lookup.calls)
	}
}

func TestWaitForTransaction_PollsThroughNotFoundUntilSuccess(t *testing.T) {
	lookup := &fakeLookup{responses: []protocol.GetTransactionResponse{
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusNotFound}},
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusNotFound}},
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusSuccess}},
	}}
	slept := 0
	sleep := func(time.Duration) { slept++ }

	resp, err := WaitForTransaction(context.Background(), lookup, "hash1", ReconcileOptions{Attempts: 5, Interval: time.Millisecond, Sleep: sleep})
	if err != nil {
		t.Fatalf("WaitForTransaction: %v", err)
	}
	if resp.Status != protocol.TransactionStatusSuccess {
		t.Errorf("Status = %q", resp.Status)
	}
	if lookup.calls != 3 {
		t.Errorf("calls = %d, want 3", lookup.calls)
	}
	if slept != 2 {
		t.Errorf("slept = %d, want 2", slept)
	}
}

func TestWaitForTransaction_ReturnsErrTransactionFailedWithoutFurtherPolling(t *testing.T) {
	lookup := &fakeLookup{responses: []protocol.GetTransactionResponse{
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusNotFound}},
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusFailed}},
	}}

	_, err := WaitForTransaction(context.Background(), lookup, "hash1", ReconcileOptions{Attempts: 5, Interval: 0, Sleep: noSleep})
	if !errors.Is(err, ErrTransactionFailed) {
		t.Fatalf("err = %v, want ErrTransactionFailed", err)
	}
	if lookup.calls != 2 {
		t.Errorf("calls = %d, want 2 (must stop polling once FAILED is seen)", lookup.calls)
	}
}

func TestWaitForTransaction_TimesOutAfterExhaustingAttemptsWithoutResubmitting(t *testing.T) {
	lookup := &fakeLookup{responses: []protocol.GetTransactionResponse{
		{TransactionDetails: protocol.TransactionDetails{Status: protocol.TransactionStatusNotFound}},
	}}

	_, err := WaitForTransaction(context.Background(), lookup, "hash1", ReconcileOptions{Attempts: 3, Interval: 0, Sleep: noSleep})
	var timeoutErr *TransactionTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("err = %v, want *TransactionTimeoutError", err)
	}
	if timeoutErr.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", timeoutErr.Attempts)
	}
	// This function has no way to submit anything at all — "never
	// resubmits" here means it never exceeds the configured attempt bound.
	if lookup.calls != 3 {
		t.Errorf("calls = %d, want exactly 3", lookup.calls)
	}
}

func TestWaitForTransaction_PropagatesLookupErrors(t *testing.T) {
	lookup := &fakeLookup{errs: []error{errors.New("network blip")}}

	_, err := WaitForTransaction(context.Background(), lookup, "hash1", ReconcileOptions{Attempts: 3, Interval: 0, Sleep: noSleep})
	if err == nil {
		t.Fatal("expected an error")
	}
}
