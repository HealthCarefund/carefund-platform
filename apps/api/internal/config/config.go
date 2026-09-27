// Package config loads and validates the CareFund API's environment
// configuration. There are no silent defaults for anything that could send
// a transaction, or a contract read, to the wrong network — those fields
// are required and validated explicitly, matching the same rule already
// enforced client-side in packages/sdk/src/config.ts.
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Env is the deployment environment the API is running in.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Network is a Stellar network identifier, mirroring
// packages/sdk/src/config.ts's StellarNetwork.
type Network string

const (
	NetworkPublic    Network = "PUBLIC"
	NetworkTestnet   Network = "TESTNET"
	NetworkFuturenet Network = "FUTURENET"
	NetworkCustom    Network = "CUSTOM"
)

var wellKnownPassphrases = map[Network]string{
	NetworkPublic:    "Public Global Stellar Network ; September 2015",
	NetworkTestnet:   "Test SDF Network ; September 2015",
	NetworkFuturenet: "Test SDF Future Network ; October 2022",
}

// LogLevel is one of the levels log/slog understands.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Config is the fully validated API configuration.
type Config struct {
	AppEnv  Env
	AppAddr string

	DatabaseURL string

	StellarNetwork             Network
	StellarRPCURL              string
	StellarNetworkPassphrase   string
	ProviderRegistryContractID string
	CareAgreementContractID    string
	SettlementAssetContractID  string

	RPCTimeout             time.Duration
	RequestTimeout         time.Duration
	ReconciliationInterval time.Duration

	CORSAllowedOrigins []string

	LogLevel LogLevel

	MaxRequestBodyBytes int64
}

// Getenv matches os.LookupEnv's signature, so Load can be tested without
// touching real process environment variables.
type Getenv func(key string) (string, bool)

var contractIDPattern = regexp.MustCompile(`^C[A-Z2-7]{55}$`)

// Load reads and validates every required variable via get, collecting
// every problem found (not just the first) into a single error.
func Load(get Getenv) (Config, error) {
	var cfg Config
	var issues []string

	require := func(key string) string {
		v, ok := get(key)
		if !ok || strings.TrimSpace(v) == "" {
			issues = append(issues, fmt.Sprintf("%s is required", key))
			return ""
		}
		return v
	}

	appEnvRaw := require("APP_ENV")
	switch Env(appEnvRaw) {
	case EnvDevelopment, EnvStaging, EnvProduction:
		cfg.AppEnv = Env(appEnvRaw)
	case "":
		// already recorded by require
	default:
		issues = append(issues, fmt.Sprintf("APP_ENV must be one of development, staging, production; got %q", appEnvRaw))
	}

	cfg.AppAddr = require("APP_ADDR")

	cfg.DatabaseURL = require("DATABASE_URL")
	if cfg.DatabaseURL != "" {
		if _, err := url.Parse(cfg.DatabaseURL); err != nil {
			issues = append(issues, fmt.Sprintf("DATABASE_URL is not a valid URL: %v", err))
		}
	}

	networkRaw := require("STELLAR_NETWORK")
	switch Network(networkRaw) {
	case NetworkPublic, NetworkTestnet, NetworkFuturenet, NetworkCustom:
		cfg.StellarNetwork = Network(networkRaw)
	case "":
	default:
		issues = append(issues, fmt.Sprintf("STELLAR_NETWORK must be one of PUBLIC, TESTNET, FUTURENET, CUSTOM; got %q", networkRaw))
	}

	cfg.StellarRPCURL = require("STELLAR_RPC_URL")
	if cfg.StellarRPCURL != "" {
		parsed, err := url.Parse(cfg.StellarRPCURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			issues = append(issues, fmt.Sprintf("STELLAR_RPC_URL must be an absolute http(s) URL, got %q", cfg.StellarRPCURL))
		}
	}

	cfg.StellarNetworkPassphrase = require("STELLAR_NETWORK_PASSPHRASE")
	if cfg.StellarNetworkPassphrase != "" && cfg.StellarNetwork != "" && cfg.StellarNetwork != NetworkCustom {
		if want, ok := wellKnownPassphrases[cfg.StellarNetwork]; ok && want != cfg.StellarNetworkPassphrase {
			issues = append(issues, fmt.Sprintf(
				"STELLAR_NETWORK_PASSPHRASE does not match the well-known passphrase for network %q; this would send transactions to the wrong network",
				cfg.StellarNetwork,
			))
		}
	}

	for _, field := range []struct {
		key string
		dst *string
	}{
		{"PROVIDER_REGISTRY_CONTRACT_ID", &cfg.ProviderRegistryContractID},
		{"CARE_AGREEMENT_CONTRACT_ID", &cfg.CareAgreementContractID},
		{"SETTLEMENT_ASSET_CONTRACT_ID", &cfg.SettlementAssetContractID},
	} {
		v := require(field.key)
		*field.dst = v
		if v != "" && !contractIDPattern.MatchString(v) {
			issues = append(issues, fmt.Sprintf("%s must be a valid Soroban contract address (C...), got %q", field.key, v))
		}
	}

	parseDuration := func(key string, dst *time.Duration) {
		v := require(key)
		if v == "" {
			return
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s must be a valid Go duration (e.g. \"5s\"), got %q: %v", key, v, err))
			return
		}
		if d <= 0 {
			issues = append(issues, fmt.Sprintf("%s must be a positive duration, got %q", key, v))
			return
		}
		*dst = d
	}
	parseDuration("RPC_TIMEOUT", &cfg.RPCTimeout)
	parseDuration("REQUEST_TIMEOUT", &cfg.RequestTimeout)
	parseDuration("RECONCILIATION_INTERVAL", &cfg.ReconciliationInterval)

	corsRaw := require("CORS_ALLOWED_ORIGINS")
	if corsRaw != "" {
		for _, origin := range strings.Split(corsRaw, ",") {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				continue
			}
			if origin != "*" {
				if parsed, err := url.Parse(origin); err != nil || parsed.Scheme == "" || parsed.Host == "" {
					issues = append(issues, fmt.Sprintf("CORS_ALLOWED_ORIGINS entry %q must be \"*\" or an absolute origin", origin))
					continue
				}
			}
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, origin)
		}
		if len(cfg.CORSAllowedOrigins) == 0 {
			issues = append(issues, "CORS_ALLOWED_ORIGINS must contain at least one origin")
		}
	}

	logLevelRaw := require("LOG_LEVEL")
	switch LogLevel(strings.ToLower(logLevelRaw)) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		cfg.LogLevel = LogLevel(strings.ToLower(logLevelRaw))
	case "":
	default:
		issues = append(issues, fmt.Sprintf("LOG_LEVEL must be one of debug, info, warn, error; got %q", logLevelRaw))
	}

	maxBodyRaw := require("MAX_REQUEST_BODY_BYTES")
	if maxBodyRaw != "" {
		n, err := strconv.ParseInt(maxBodyRaw, 10, 64)
		if err != nil || n <= 0 {
			issues = append(issues, fmt.Sprintf("MAX_REQUEST_BODY_BYTES must be a positive integer, got %q", maxBodyRaw))
		} else {
			cfg.MaxRequestBodyBytes = n
		}
	}

	if len(issues) > 0 {
		return Config{}, &ValidationError{Issues: issues}
	}
	return cfg, nil
}

// ValidationError lists every configuration problem found, not just the
// first, so a misconfigured deployment fails with one complete report.
type ValidationError struct {
	Issues []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid configuration: %s", strings.Join(e.Issues, "; "))
}
