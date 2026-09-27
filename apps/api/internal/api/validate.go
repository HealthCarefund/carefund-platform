package api

import (
	"math/big"
	"regexp"
	"strconv"
)

var stellarAccountAddressPattern = regexp.MustCompile(`^G[A-Z2-7]{55}$`)
var stellarContractAddressPattern = regexp.MustCompile(`^C[A-Z2-7]{55}$`)
var agreementIDPattern = regexp.MustCompile(`^\d+$`)
var hex32Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var nonNegativeIntegerPattern = regexp.MustCompile(`^(0|[1-9]\d*)$`)

func isStellarAccountAddress(s string) bool {
	return stellarAccountAddressPattern.MatchString(s)
}

func isStellarContractAddress(s string) bool {
	return stellarContractAddressPattern.MatchString(s)
}

func isAgreementIDFormat(s string) bool {
	return agreementIDPattern.MatchString(s) && s != "" && !(len(s) > 1 && s[0] == '0')
}

// isHex32 reports whether s is exactly 64 lowercase hex characters — a
// 32-byte opaque commitment, matching what the contracts themselves store.
func isHex32(s string) bool {
	return hex32Pattern.MatchString(s)
}

var i128Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))

// isPositiveI128 reports whether s is a canonical decimal integer string,
// strictly greater than zero, and within the i128 range — the shape
// funding_amount/settlement_amount must have.
func isPositiveI128(s string) bool {
	if !nonNegativeIntegerPattern.MatchString(s) {
		return false
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() <= 0 {
		return false
	}
	return n.Cmp(i128Max) <= 0
}

// isStoredU64 reports whether s is a canonical non-negative decimal
// integer string that also fits in an int64 — the actual representable
// range of this schema's BIGINT columns (funding_deadline, care_deadline,
// dispute_window_secs), which is narrower than the full u64 range the
// on-chain fields themselves permit. A ledger timestamp/duration this
// large is not a realistic value in practice, so this bound rejects it
// explicitly rather than silently truncating it on parse.
func isStoredU64(s string) bool {
	if !nonNegativeIntegerPattern.MatchString(s) {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}
