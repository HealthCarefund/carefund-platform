import { Networks } from "@stellar/stellar-sdk";
import { isStellarAddress } from "@carefund/types";
import { ConfigValidationError } from "./errors.js";

/**
 * Named Stellar networks this SDK will validate a passphrase against.
 * `CUSTOM` covers private/local networks (e.g. a standalone quickstart
 * container) whose passphrase isn't one of the well-known constants.
 */
export type StellarNetwork = "PUBLIC" | "TESTNET" | "FUTURENET" | "CUSTOM";

const WELL_KNOWN_PASSPHRASES: Readonly<Record<Exclude<StellarNetwork, "CUSTOM">, string>> = {
  PUBLIC: Networks.PUBLIC,
  TESTNET: Networks.TESTNET,
  FUTURENET: Networks.FUTURENET,
};

/**
 * Everything the SDK needs to talk to a specific Stellar network and the
 * three CareFund contracts on it. Deliberately excludes anything that could
 * hold or transmit a private key — see `assertNoSecretMaterial`. The
 * browser wallet is always the signing authority; this config never is.
 */
export interface StellarClientConfig {
  readonly network: StellarNetwork;
  readonly rpcUrl: string;
  readonly networkPassphrase: string;
  readonly providerRegistryContractId: string;
  readonly careAgreementContractId: string;
  readonly settlementAssetContractId: string;
}

const SECRET_KEY_PATTERN = /secret|private[_-]?key|seed[_-]?phrase|mnemonic/i;

/**
 * Refuses to proceed if the raw source object (e.g. `process.env`, or a
 * config object assembled from it) carries anything that looks like key
 * material. This SDK must never accept, store, or forward a secret key —
 * signing is the browser wallet's job, never this SDK's.
 */
export function assertNoSecretMaterial(raw: Record<string, unknown>): void {
  const offending = Object.keys(raw).filter((key) => SECRET_KEY_PATTERN.test(key));
  if (offending.length > 0) {
    throw new ConfigValidationError(
      offending.map(
        (key) => `field "${key}" looks like secret key material and must not be present`,
      ),
    );
  }
}

function isContractId(value: unknown): value is string {
  return typeof value === "string" && isStellarAddress(value) && value.startsWith("C");
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function isHttpUrl(value: unknown): value is string {
  if (!isNonEmptyString(value)) {
    return false;
  }
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

const STELLAR_NETWORKS: readonly StellarNetwork[] = ["PUBLIC", "TESTNET", "FUTURENET", "CUSTOM"];

/**
 * Validates a raw configuration object field-by-field and returns a typed
 * `StellarClientConfig` only if every field is present and well-formed.
 * There is no fallback network, RPC URL, or passphrase — an incomplete or
 * inconsistent configuration must fail loudly rather than silently target
 * the wrong network.
 */
export function validateStellarClientConfig(raw: Record<string, unknown>): StellarClientConfig {
  assertNoSecretMaterial(raw);

  const issues: string[] = [];

  const network = raw.network;
  if (typeof network !== "string" || !STELLAR_NETWORKS.includes(network as StellarNetwork)) {
    issues.push(
      `"network" must be one of ${STELLAR_NETWORKS.join(", ")}, got ${JSON.stringify(network)}`,
    );
  }

  if (!isHttpUrl(raw.rpcUrl)) {
    issues.push(`"rpcUrl" must be an absolute http(s) URL, got ${JSON.stringify(raw.rpcUrl)}`);
  }

  if (!isNonEmptyString(raw.networkPassphrase)) {
    issues.push(`"networkPassphrase" must be a non-empty string`);
  } else if (
    typeof network === "string" &&
    network !== "CUSTOM" &&
    STELLAR_NETWORKS.includes(network as StellarNetwork) &&
    raw.networkPassphrase !== WELL_KNOWN_PASSPHRASES[network as Exclude<StellarNetwork, "CUSTOM">]
  ) {
    issues.push(
      `"networkPassphrase" does not match the well-known passphrase for network "${network}"; ` +
        `this would send transactions to the wrong network`,
    );
  }

  for (const field of [
    "providerRegistryContractId",
    "careAgreementContractId",
    "settlementAssetContractId",
  ] as const) {
    if (!isContractId(raw[field])) {
      issues.push(`"${field}" must be a valid Soroban contract address (C...), got ${JSON.stringify(raw[field])}`);
    }
  }

  if (issues.length > 0) {
    throw new ConfigValidationError(issues);
  }

  return {
    network: network as StellarNetwork,
    rpcUrl: raw.rpcUrl as string,
    networkPassphrase: raw.networkPassphrase as string,
    providerRegistryContractId: raw.providerRegistryContractId as string,
    careAgreementContractId: raw.careAgreementContractId as string,
    settlementAssetContractId: raw.settlementAssetContractId as string,
  };
}
