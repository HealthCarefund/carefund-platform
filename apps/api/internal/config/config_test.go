package config

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":                       "development",
		"APP_ADDR":                      ":8080",
		"DATABASE_URL":                  "postgres://carefund:carefund_dev@localhost:5439/carefund",
		"STELLAR_NETWORK":               "TESTNET",
		"STELLAR_RPC_URL":               "https://soroban-testnet.stellar.org",
		"STELLAR_NETWORK_PASSPHRASE":    "Test SDF Network ; September 2015",
		"PROVIDER_REGISTRY_CONTRACT_ID": "C" + strings.Repeat("A", 55),
		"CARE_AGREEMENT_CONTRACT_ID":    "C" + strings.Repeat("B", 55),
		"SETTLEMENT_ASSET_CONTRACT_ID":  "C" + strings.Repeat("D", 55),
		"RPC_TIMEOUT":                   "5s",
		"REQUEST_TIMEOUT":               "10s",
		"RECONCILIATION_INTERVAL":       "30s",
		"CORS_ALLOWED_ORIGINS":          "https://app.carefund.example",
		"LOG_LEVEL":                     "info",
		"MAX_REQUEST_BODY_BYTES":        "1048576",
	}
}

func getenvFrom(env map[string]string) Getenv {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(getenvFrom(validEnv()))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AppEnv != EnvDevelopment {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, EnvDevelopment)
	}
	if cfg.StellarNetwork != NetworkTestnet {
		t.Errorf("StellarNetwork = %q, want %q", cfg.StellarNetwork, NetworkTestnet)
	}
	if cfg.MaxRequestBodyBytes != 1048576 {
		t.Errorf("MaxRequestBodyBytes = %d, want 1048576", cfg.MaxRequestBodyBytes)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "https://app.carefund.example" {
		t.Errorf("CORSAllowedOrigins = %v", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	env := validEnv()
	delete(env, "APP_ADDR")
	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("expected an error for missing APP_ADDR")
	}
	if !strings.Contains(err.Error(), "APP_ADDR") {
		t.Errorf("error %q does not mention APP_ADDR", err.Error())
	}
}

func TestLoad_ReportsMultipleIssuesAtOnce(t *testing.T) {
	env := validEnv()
	delete(env, "APP_ADDR")
	env["STELLAR_NETWORK"] = "MAINNET"
	env["RPC_TIMEOUT"] = "not-a-duration"

	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("expected an error")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Issues) < 3 {
		t.Errorf("expected at least 3 issues, got %d: %v", len(ve.Issues), ve.Issues)
	}
}

func TestLoad_RejectsPassphraseMismatchedToNetwork(t *testing.T) {
	env := validEnv()
	env["STELLAR_NETWORK"] = "PUBLIC"
	// leave the TESTNET passphrase in place — must be rejected as
	// pointing transactions at the wrong network.
	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("expected an error for mismatched network/passphrase")
	}
	if !strings.Contains(err.Error(), "wrong network") {
		t.Errorf("error %q does not mention the network mismatch", err.Error())
	}
}

func TestLoad_AllowsCustomNetworkWithAnyPassphrase(t *testing.T) {
	env := validEnv()
	env["STELLAR_NETWORK"] = "CUSTOM"
	env["STELLAR_NETWORK_PASSPHRASE"] = "Standalone Network ; February 2017"
	if _, err := Load(getenvFrom(env)); err != nil {
		t.Fatalf("expected no error for CUSTOM network, got %v", err)
	}
}

func TestLoad_RejectsMalformedContractID(t *testing.T) {
	env := validEnv()
	env["CARE_AGREEMENT_CONTRACT_ID"] = "not-a-contract-id"
	_, err := Load(getenvFrom(env))
	if err == nil {
		t.Fatal("expected an error for malformed contract id")
	}
	if !strings.Contains(err.Error(), "CARE_AGREEMENT_CONTRACT_ID") {
		t.Errorf("error %q does not mention the offending field", err.Error())
	}
}

func TestLoad_RejectsAccountAddressAsContractID(t *testing.T) {
	env := validEnv()
	env["CARE_AGREEMENT_CONTRACT_ID"] = "G" + strings.Repeat("A", 55)
	if _, err := Load(getenvFrom(env)); err == nil {
		t.Fatal("expected an error: a G-address is not a valid contract id")
	}
}

func TestLoad_RejectsNonPositiveDuration(t *testing.T) {
	env := validEnv()
	env["RPC_TIMEOUT"] = "0s"
	if _, err := Load(getenvFrom(env)); err == nil {
		t.Fatal("expected an error for a zero duration")
	}
}

func TestLoad_RejectsInvalidRPCURL(t *testing.T) {
	env := validEnv()
	env["STELLAR_RPC_URL"] = "not a url"
	if _, err := Load(getenvFrom(env)); err == nil {
		t.Fatal("expected an error for a malformed RPC URL")
	}
}

func TestLoad_RejectsNonHTTPRPCURLScheme(t *testing.T) {
	env := validEnv()
	env["STELLAR_RPC_URL"] = "ftp://example.com"
	if _, err := Load(getenvFrom(env)); err == nil {
		t.Fatal("expected an error for a non-http(s) RPC URL scheme")
	}
}

func TestLoad_AllowsWildcardCORS(t *testing.T) {
	env := validEnv()
	env["CORS_ALLOWED_ORIGINS"] = "*"
	cfg, err := Load(getenvFrom(env))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "*" {
		t.Errorf("CORSAllowedOrigins = %v", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_RejectsNonPositiveMaxRequestBodyBytes(t *testing.T) {
	env := validEnv()
	env["MAX_REQUEST_BODY_BYTES"] = "-1"
	if _, err := Load(getenvFrom(env)); err == nil {
		t.Fatal("expected an error for a negative MAX_REQUEST_BODY_BYTES")
	}
}
