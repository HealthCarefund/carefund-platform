import { Networks } from "@stellar/stellar-sdk";
import { describe, expect, it } from "vitest";
import { assertNoSecretMaterial, validateStellarClientConfig } from "./config.js";
import { ConfigValidationError } from "./errors.js";

const PROVIDER_REGISTRY_ID = "C" + "A".repeat(55);
const CARE_AGREEMENT_ID = "C" + "B".repeat(55);
const SETTLEMENT_ASSET_ID = "C" + "D".repeat(55);
const ACCOUNT_ADDRESS = "G" + "A".repeat(55);

function validRaw(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    network: "TESTNET",
    rpcUrl: "https://soroban-testnet.stellar.org",
    networkPassphrase: Networks.TESTNET,
    providerRegistryContractId: PROVIDER_REGISTRY_ID,
    careAgreementContractId: CARE_AGREEMENT_ID,
    settlementAssetContractId: SETTLEMENT_ASSET_ID,
    ...overrides,
  };
}

describe("validateStellarClientConfig", () => {
  it("accepts a fully valid TESTNET configuration", () => {
    const config = validateStellarClientConfig(validRaw());
    expect(config).toEqual({
      network: "TESTNET",
      rpcUrl: "https://soroban-testnet.stellar.org",
      networkPassphrase: Networks.TESTNET,
      providerRegistryContractId: PROVIDER_REGISTRY_ID,
      careAgreementContractId: CARE_AGREEMENT_ID,
      settlementAssetContractId: SETTLEMENT_ASSET_ID,
    });
  });

  it("accepts a CUSTOM network with a non-well-known passphrase", () => {
    const config = validateStellarClientConfig(
      validRaw({ network: "CUSTOM", networkPassphrase: "Standalone Network ; February 2017" }),
    );
    expect(config.network).toBe("CUSTOM");
  });

  it("rejects a missing network", () => {
    expect(() => validateStellarClientConfig(validRaw({ network: undefined }))).toThrow(
      ConfigValidationError,
    );
  });

  it("rejects an unknown network name", () => {
    expect(() => validateStellarClientConfig(validRaw({ network: "MAINNET" }))).toThrow(
      ConfigValidationError,
    );
  });

  it("rejects a passphrase that does not match the declared well-known network", () => {
    expect(() =>
      validateStellarClientConfig(validRaw({ network: "PUBLIC", networkPassphrase: Networks.TESTNET })),
    ).toThrow(ConfigValidationError);
  });

  it("rejects a non-http(s) rpcUrl", () => {
    expect(() => validateStellarClientConfig(validRaw({ rpcUrl: "ftp://example.com" }))).toThrow(
      ConfigValidationError,
    );
  });

  it("rejects a malformed rpcUrl", () => {
    expect(() => validateStellarClientConfig(validRaw({ rpcUrl: "not a url" }))).toThrow(
      ConfigValidationError,
    );
  });

  it("rejects a contract id that is actually an account address", () => {
    expect(() =>
      validateStellarClientConfig(validRaw({ providerRegistryContractId: ACCOUNT_ADDRESS })),
    ).toThrow(ConfigValidationError);
  });

  it("rejects a malformed contract id", () => {
    expect(() =>
      validateStellarClientConfig(validRaw({ careAgreementContractId: "not-a-contract-id" })),
    ).toThrow(ConfigValidationError);
  });

  it("reports every invalid field at once, not just the first", () => {
    try {
      validateStellarClientConfig(
        validRaw({ network: "MAINNET", rpcUrl: "nope", providerRegistryContractId: "nope" }),
      );
      expect.unreachable("expected validateStellarClientConfig to throw");
    } catch (error) {
      expect(error).toBeInstanceOf(ConfigValidationError);
      const issues = (error as ConfigValidationError).issues;
      expect(issues.length).toBeGreaterThanOrEqual(3);
    }
  });

  it("refuses configuration carrying anything that looks like a secret key", () => {
    expect(() =>
      validateStellarClientConfig(validRaw({ secretKey: "SA".padEnd(56, "A") })),
    ).toThrow(ConfigValidationError);
  });
});

describe("assertNoSecretMaterial", () => {
  it("passes for an object with no secret-shaped keys", () => {
    expect(() => assertNoSecretMaterial({ rpcUrl: "https://example.com" })).not.toThrow();
  });

  it.each(["secretKey", "SECRET_KEY", "privateKey", "private_key", "seedPhrase", "mnemonic"])(
    "rejects a field named %s",
    (key) => {
      expect(() => assertNoSecretMaterial({ [key]: "x" })).toThrow(ConfigValidationError);
    },
  );
});
