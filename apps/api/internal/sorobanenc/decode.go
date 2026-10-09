package sorobanenc

import (
	"fmt"
	"math/big"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

// DecodeAgreement decodes a Soroban ScVal (ScvMap) representing an Agreement struct
// into store.CareAgreement.
func DecodeAgreement(val xdr.ScVal, agreementID int64, settlementAssetContractID string) (*store.CareAgreement, error) {
	entries, ok := val.GetMap()
	if !ok || entries == nil {
		return nil, fmt.Errorf("expected ScvMap for Agreement, got %v", val.Type)
	}

	agr := &store.CareAgreement{
		AgreementID:               agreementID,
		SettlementAssetContractID: settlementAssetContractID,
	}

	for _, entry := range *entries {
		if entry.Key.Sym == nil {
			continue
		}
		key := string(*entry.Key.Sym)
		switch key {
		case "sponsor":
			if entry.Val.Address == nil {
				return nil, fmt.Errorf("sponsor missing Address")
			}
			addr, err := entry.Val.Address.String()
			if err != nil {
				return nil, fmt.Errorf("decoding sponsor address: %w", err)
			}
			agr.SponsorWallet = addr
		case "provider":
			if entry.Val.Address == nil {
				return nil, fmt.Errorf("provider missing Address")
			}
			addr, err := entry.Val.Address.String()
			if err != nil {
				return nil, fmt.Errorf("decoding provider address: %w", err)
			}
			agr.ProviderWallet = addr
		case "attester":
			if entry.Val.Address == nil {
				return nil, fmt.Errorf("attester missing Address")
			}
			addr, err := entry.Val.Address.String()
			if err != nil {
				return nil, fmt.Errorf("decoding attester address: %w", err)
			}
			agr.AttesterWallet = addr
		case "patient_ref_commitment":
			if entry.Val.Bytes == nil {
				return nil, fmt.Errorf("patient_ref_commitment missing Bytes")
			}
			b := make([]byte, len(*entry.Val.Bytes))
			copy(b, *entry.Val.Bytes)
			agr.PatientRefCommitment = b
		case "service_commitment":
			if entry.Val.Bytes == nil {
				return nil, fmt.Errorf("service_commitment missing Bytes")
			}
			b := make([]byte, len(*entry.Val.Bytes))
			copy(b, *entry.Val.Bytes)
			agr.ServiceCommitment = b
		case "funding_amount":
			if entry.Val.I128 == nil {
				return nil, fmt.Errorf("funding_amount missing I128")
			}
			agr.FundingAmount = decodeI128(entry.Val.I128)
		case "settlement_amount":
			if entry.Val.I128 == nil {
				return nil, fmt.Errorf("settlement_amount missing I128")
			}
			agr.SettlementAmount = decodeI128(entry.Val.I128)
		case "funding_deadline":
			if entry.Val.U64 == nil {
				return nil, fmt.Errorf("funding_deadline missing U64")
			}
			agr.FundingDeadline = int64(*entry.Val.U64)
		case "care_deadline":
			if entry.Val.U64 == nil {
				return nil, fmt.Errorf("care_deadline missing U64")
			}
			agr.CareDeadline = int64(*entry.Val.U64)
		case "dispute_window_secs":
			if entry.Val.U64 == nil {
				return nil, fmt.Errorf("dispute_window_secs missing U64")
			}
			agr.DisputeWindowSecs = int64(*entry.Val.U64)
		case "state":
			stateStr, err := decodeEnumSymbol(entry.Val)
			if err != nil {
				return nil, fmt.Errorf("decoding state: %w", err)
			}
			agr.State = stateStr
		}
	}

	if agr.SponsorWallet == "" || agr.ProviderWallet == "" || agr.AttesterWallet == "" || agr.State == "" {
		return nil, fmt.Errorf("decoded agreement missing required fields: %+v", agr)
	}

	return agr, nil
}

func decodeI128(parts *xdr.Int128Parts) string {
	hi := big.NewInt(int64(parts.Hi))
	lo := new(big.Int).SetUint64(uint64(parts.Lo))
	val := new(big.Int).Lsh(hi, 64)
	val.Or(val, lo)
	return val.String()
}

func decodeEnumSymbol(val xdr.ScVal) (string, error) {
	if val.Sym != nil {
		return string(*val.Sym), nil
	}
	if vec, ok := val.GetVec(); ok && vec != nil && len(*vec) > 0 {
		first := (*vec)[0]
		if first.Sym != nil {
			return string(*first.Sym), nil
		}
	}
	return "", fmt.Errorf("unexpected enum ScVal shape: %v", val.Type)
}

// BuildGetAgreementTransaction builds an unsimulated transaction envelope invoking get_agreement(agreementID).
func BuildGetAgreementTransaction(contractID string, agreementID int64) (string, error) {
	contractAddress, err := Address(contractID)
	if err != nil {
		return "", fmt.Errorf("encoding contract address: %w", err)
	}

	invoke := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: *contractAddress.Address,
				FunctionName:    xdr.ScSymbol("get_agreement"),
				Args:            []xdr.ScVal{U64(uint64(agreementID))},
			},
		},
		SourceAccount: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
			Sequence:  0,
		},
		IncrementSequenceNum: false,
		Operations:           []txnbuild.Operation{invoke},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
	})
	if err != nil {
		return "", fmt.Errorf("building transaction: %w", err)
	}

	return tx.Base64()
}

// DecodeProvider decodes a Soroban ScVal (ScvMap) representing a ProviderRecord
// into store.Provider.
func DecodeProvider(val xdr.ScVal, wallet string) (*store.Provider, error) {
	entries, ok := val.GetMap()
	if !ok || entries == nil {
		return nil, fmt.Errorf("expected ScvMap for ProviderRecord, got %v", val.Type)
	}

	p := &store.Provider{
		WalletAddress: wallet,
	}

	for _, entry := range *entries {
		if entry.Key.Sym == nil {
			continue
		}
		key := string(*entry.Key.Sym)
		switch key {
		case "provider_ref":
			if entry.Val.Bytes == nil {
				return nil, fmt.Errorf("provider_ref missing Bytes")
			}
			b := make([]byte, len(*entry.Val.Bytes))
			copy(b, *entry.Val.Bytes)
			p.ProviderRef = b
		case "status":
			status, err := decodeEnumSymbol(entry.Val)
			if err != nil {
				return nil, fmt.Errorf("decoding status: %w", err)
			}
			p.Status = status
		}
	}

	if len(p.ProviderRef) != 32 || p.Status == "" {
		return nil, fmt.Errorf("decoded provider missing required fields: %+v", p)
	}

	return p, nil
}

// DecodeAttester decodes a Soroban ScVal (ScvMap) representing an AttesterRecord
// into store.Attester.
func DecodeAttester(val xdr.ScVal, wallet string) (*store.Attester, error) {
	entries, ok := val.GetMap()
	if !ok || entries == nil {
		return nil, fmt.Errorf("expected ScvMap for AttesterRecord, got %v", val.Type)
	}

	a := &store.Attester{
		WalletAddress: wallet,
	}

	for _, entry := range *entries {
		if entry.Key.Sym == nil {
			continue
		}
		key := string(*entry.Key.Sym)
		switch key {
		case "provider":
			if entry.Val.Address == nil {
				return nil, fmt.Errorf("provider missing Address")
			}
			addr, err := entry.Val.Address.String()
			if err != nil {
				return nil, fmt.Errorf("decoding provider address: %w", err)
			}
			a.ProviderWallet = addr
		case "credential_ref":
			if entry.Val.Bytes == nil {
				return nil, fmt.Errorf("credential_ref missing Bytes")
			}
			b := make([]byte, len(*entry.Val.Bytes))
			copy(b, *entry.Val.Bytes)
			a.CredentialRef = b
		case "status":
			status, err := decodeEnumSymbol(entry.Val)
			if err != nil {
				return nil, fmt.Errorf("decoding status: %w", err)
			}
			a.Status = status
		}
	}

	if len(a.CredentialRef) != 32 || a.Status == "" || a.ProviderWallet == "" {
		return nil, fmt.Errorf("decoded attester missing required fields: %+v", a)
	}

	return a, nil
}

// BuildGetProviderTransaction builds an unsimulated transaction envelope invoking get_provider(provider).
func BuildGetProviderTransaction(contractID, providerAddress string) (string, error) {
	contractAddr, err := Address(contractID)
	if err != nil {
		return "", fmt.Errorf("encoding contract address: %w", err)
	}
	providerArg, err := Address(providerAddress)
	if err != nil {
		return "", fmt.Errorf("encoding provider address: %w", err)
	}

	invoke := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: *contractAddr.Address,
				FunctionName:    xdr.ScSymbol("get_provider"),
				Args:            []xdr.ScVal{providerArg},
			},
		},
		SourceAccount: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
			Sequence:  0,
		},
		IncrementSequenceNum: false,
		Operations:           []txnbuild.Operation{invoke},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
	})
	if err != nil {
		return "", fmt.Errorf("building transaction: %w", err)
	}

	return tx.Base64()
}

// BuildGetAttesterTransaction builds an unsimulated transaction envelope invoking get_attester(attester).
func BuildGetAttesterTransaction(contractID, attesterAddress string) (string, error) {
	contractAddr, err := Address(contractID)
	if err != nil {
		return "", fmt.Errorf("encoding contract address: %w", err)
	}
	attesterArg, err := Address(attesterAddress)
	if err != nil {
		return "", fmt.Errorf("encoding attester address: %w", err)
	}

	invoke := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: *contractAddr.Address,
				FunctionName:    xdr.ScSymbol("get_attester"),
				Args:            []xdr.ScVal{attesterArg},
			},
		},
		SourceAccount: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{
			AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
			Sequence:  0,
		},
		IncrementSequenceNum: false,
		Operations:           []txnbuild.Operation{invoke},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
	})
	if err != nil {
		return "", fmt.Errorf("building transaction: %w", err)
	}

	return tx.Base64()
}
