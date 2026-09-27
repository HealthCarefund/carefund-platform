import { validateStellarClientConfig, type StellarClientConfig } from "@carefund/sdk";

// Every value here is read from NEXT_PUBLIC_* — genuinely public (network
// name, RPC URL, contract IDs). Never a secret, and this app never holds
// one: the browser wallet is always the signing authority.
function readPublicEnv(): Record<string, unknown> {
  return {
    network: process.env.NEXT_PUBLIC_STELLAR_NETWORK,
    rpcUrl: process.env.NEXT_PUBLIC_STELLAR_RPC_URL,
    networkPassphrase: process.env.NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE,
    providerRegistryContractId: process.env.NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID,
    careAgreementContractId: process.env.NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID,
    settlementAssetContractId: process.env.NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID,
  };
}

let cached: StellarClientConfig | null = null;

/**
 * Validates and returns the app's Stellar network configuration. Throws
 * ConfigValidationError (with every problem found, not just the first) if
 * any NEXT_PUBLIC_* value is missing or malformed — no silent default
 * that could point a signed transaction at the wrong network.
 */
export function getStellarConfig(): StellarClientConfig {
  if (!cached) {
    cached = validateStellarClientConfig(readPublicEnv());
  }
  return cached;
}

export function apiBaseUrl(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (!url) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not configured");
  }
  return url;
}
