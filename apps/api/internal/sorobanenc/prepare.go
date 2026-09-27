package sorobanenc

import (
	"context"
	"fmt"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// AccountLoader is the narrow RPC dependency PrepareContractCall needs, so
// callers can inject a fixture in tests instead of a live RPC client.
type AccountLoader interface {
	LoadAccount(ctx context.Context, address string) (txnbuild.Account, error)
}

// TransactionSimulator is the narrow RPC dependency PrepareContractCall
// needs for simulation.
type TransactionSimulator interface {
	Simulate(ctx context.Context, envelopeXDR string) (protocol.SimulateTransactionResponse, error)
}

// ContractCallRequest describes one contract method invocation to prepare.
type ContractCallRequest struct {
	ContractID        string
	Method            string
	Args              []xdr.ScVal
	SourcePublicKey   string
	NetworkPassphrase string
	TimeoutSeconds    uint
}

// PrepareContractCall builds an unsigned transaction invoking one contract
// method, simulates it, and applies the simulation's resource
// footprint/fee/auth back onto the transaction — the same
// build->simulate->assemble sequence the TypeScript SDK's
// prepareTransaction performs (see packages/sdk/src/transaction/simulate.ts),
// implemented here in Go since no such helper ships in
// github.com/stellar/go-stellar-sdk. The result is ready for a browser
// wallet to sign; nothing here signs or submits it.
func PrepareContractCall(
	ctx context.Context,
	accounts AccountLoader,
	simulator TransactionSimulator,
	req ContractCallRequest,
) (string, error) {
	account, err := accounts.LoadAccount(ctx, req.SourcePublicKey)
	if err != nil {
		return "", fmt.Errorf("loading source account %s: %w", req.SourcePublicKey, err)
	}

	contractAddress, err := Address(req.ContractID)
	if err != nil {
		return "", fmt.Errorf("encoding contract address: %w", err)
	}

	invoke := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: *contractAddress.Address,
				FunctionName:    xdr.ScSymbol(req.Method),
				Args:            req.Args,
			},
		},
		SourceAccount: req.SourcePublicKey,
	}

	unsimulated, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        account,
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{invoke},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(int64(req.TimeoutSeconds))},
	})
	if err != nil {
		return "", fmt.Errorf("building transaction: %w", err)
	}

	unsimulatedXDR, err := unsimulated.Base64()
	if err != nil {
		return "", fmt.Errorf("encoding transaction for simulation: %w", err)
	}

	simulation, err := simulator.Simulate(ctx, unsimulatedXDR)
	if err != nil {
		return "", fmt.Errorf("simulating transaction: %w", err)
	}
	if simulation.Error != "" {
		return "", &SimulationError{Message: simulation.Error}
	}

	var sorobanData xdr.SorobanTransactionData
	if err := xdr.SafeUnmarshalBase64(simulation.TransactionDataXDR, &sorobanData); err != nil {
		return "", fmt.Errorf("decoding simulated transaction data: %w", err)
	}

	var auth []xdr.SorobanAuthorizationEntry
	if len(simulation.Results) > 0 && simulation.Results[0].AuthXDR != nil {
		for i, entryXDR := range *simulation.Results[0].AuthXDR {
			var entry xdr.SorobanAuthorizationEntry
			if err := xdr.SafeUnmarshalBase64(entryXDR, &entry); err != nil {
				return "", fmt.Errorf("decoding simulated auth entry %d: %w", i, err)
			}
			auth = append(auth, entry)
		}
	}

	invoke.Auth = auth
	invoke.Ext = xdr.TransactionExt{V: 1, SorobanData: &sorobanData}

	// Re-fetch the account so the assembled transaction uses the same
	// sequence number reserved above, not a freshly incremented one.
	preparedAccount := &txnbuild.SimpleAccount{
		AccountID: account.GetAccountID(),
		Sequence:  mustSequence(account),
	}

	prepared, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        preparedAccount,
		IncrementSequenceNum: false,
		Operations:           []txnbuild.Operation{invoke},
		BaseFee:              txnbuild.MinBaseFee + int64(simulation.MinResourceFee),
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(int64(req.TimeoutSeconds))},
	})
	if err != nil {
		return "", fmt.Errorf("assembling prepared transaction: %w", err)
	}

	preparedXDR, err := prepared.Base64()
	if err != nil {
		return "", fmt.Errorf("encoding prepared transaction: %w", err)
	}
	return preparedXDR, nil
}

func mustSequence(account txnbuild.Account) int64 {
	seq, err := account.GetSequenceNumber()
	if err != nil {
		return 0
	}
	return seq
}

// SimulationError means the RPC server itself reported that simulating
// the call would fail — the contract call is invalid or would revert.
type SimulationError struct {
	Message string
}

func (e *SimulationError) Error() string {
	return fmt.Sprintf("simulation failed: %s", e.Message)
}
