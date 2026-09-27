package api

import "regexp"

var stellarAccountAddressPattern = regexp.MustCompile(`^G[A-Z2-7]{55}$`)
var stellarContractAddressPattern = regexp.MustCompile(`^C[A-Z2-7]{55}$`)
var agreementIDPattern = regexp.MustCompile(`^\d+$`)

func isStellarAccountAddress(s string) bool {
	return stellarAccountAddressPattern.MatchString(s)
}

func isStellarContractAddress(s string) bool {
	return stellarContractAddressPattern.MatchString(s)
}

func isAgreementIDFormat(s string) bool {
	return agreementIDPattern.MatchString(s) && s != "" && !(len(s) > 1 && s[0] == '0')
}
