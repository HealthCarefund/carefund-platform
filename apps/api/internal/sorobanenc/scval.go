// Package sorobanenc encodes native Go values into the xdr.ScVal shapes
// the care-agreement and provider-registry contracts expect for their
// arguments. There is no generated Go contract binding (Unit A only
// generates TypeScript bindings, per the approved spec), so this hand-built
// encoder is this backend's equivalent of the TS SDK's
// `Spec.funcArgsToScVals` — each function here is verified by a round-trip
// test against the SDK's own xdr decoding, and the whole package is
// exercised against the real contracts deployed on Testnet (see
// internal/api/transactions_test.go).
package sorobanenc

import (
	"fmt"
	"math/big"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Address encodes a Stellar account (G...) or contract (C...) address as
// an ScVal of type ScvAddress.
func Address(address string) (xdr.ScVal, error) {
	switch {
	case len(address) > 0 && address[0] == 'G':
		accountID, err := xdr.AddressToAccountId(address)
		if err != nil {
			return xdr.ScVal{}, fmt.Errorf("encoding account address %q: %w", address, err)
		}
		scAddress := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &accountID}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &scAddress}, nil
	case len(address) > 0 && address[0] == 'C':
		decoded, err := strkey.Decode(strkey.VersionByteContract, address)
		if err != nil {
			return xdr.ScVal{}, fmt.Errorf("encoding contract address %q: %w", address, err)
		}
		var contractID xdr.ContractId
		copy(contractID[:], decoded)
		scAddress := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &contractID}
		return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &scAddress}, nil
	default:
		return xdr.ScVal{}, fmt.Errorf("%q is neither a G-address nor a C-address", address)
	}
}

// Bytes32 encodes exactly 32 bytes as an ScVal of type ScvBytes — the
// shape every opaque commitment (patient/service/attestation ref) uses.
func Bytes32(b []byte) (xdr.ScVal, error) {
	if len(b) != 32 {
		return xdr.ScVal{}, fmt.Errorf("expected exactly 32 bytes, got %d", len(b))
	}
	scBytes := xdr.ScBytes(b)
	return xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &scBytes}, nil
}

// U64 encodes a non-negative value as an ScVal of type ScvU64.
func U64(v uint64) xdr.ScVal {
	value := xdr.Uint64(v)
	return xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &value}
}

// I128 encodes a decimal integer string as an ScVal of type ScvI128.
// Only non-negative values are supported: every i128 argument the
// care-agreement contract takes (funding_amount, settlement_amount) is
// validated positive before it ever reaches this encoder (see
// internal/api's isPositiveI128), so two's-complement negative encoding is
// deliberately not implemented here — it would be untested, dead code.
func I128(decimal string) (xdr.ScVal, error) {
	n, ok := new(big.Int).SetString(decimal, 10)
	if !ok {
		return xdr.ScVal{}, fmt.Errorf("%q is not a valid decimal integer", decimal)
	}
	if n.Sign() < 0 {
		return xdr.ScVal{}, fmt.Errorf("negative i128 values are not supported by this encoder: %q", decimal)
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	if n.Cmp(max) > 0 {
		return xdr.ScVal{}, fmt.Errorf("%q exceeds the i128 range", decimal)
	}

	mask64 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))
	lo := new(big.Int).And(n, mask64)
	hi := new(big.Int).Rsh(n, 64)

	parts := &xdr.Int128Parts{
		Hi: xdr.Int64(hi.Int64()),
		Lo: xdr.Uint64(lo.Uint64()),
	}
	return xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: parts}, nil
}

// Symbol encodes a bare enum tag (no associated data) as an ScVal of type
// ScvVec containing a single ScvSymbol — the shape the generated
// TypeScript bindings call `{tag, values: void}` and the contract itself
// represents as a unit-variant enum. Used for DisputeResolution
// (Resume/Settle/Refund).
func Symbol(tag string) xdr.ScVal {
	symbol := xdr.ScSymbol(tag)
	symbolVal := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &symbol}
	vec := &xdr.ScVec{symbolVal}
	return xdr.ScVal{Type: xdr.ScValTypeScvVec, Vec: &vec}
}
