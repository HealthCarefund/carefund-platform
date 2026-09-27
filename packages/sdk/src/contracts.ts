import { Contract } from "@stellar/stellar-sdk";
import type { StellarClientConfig } from "./config.js";

/**
 * The three CareFund contracts as `Contract` handles (contract ID +
 * `.call(method, ...args)` operation builder). This is the integration
 * boundary generated contract bindings will build on top of — it does not
 * itself implement any CareFund method wrappers.
 */
export interface CareFundContracts {
  readonly providerRegistry: Contract;
  readonly careAgreement: Contract;
}

/**
 * The settlement asset contract ID is exposed as a plain string, not a
 * `Contract` handle: this SDK never calls SEP-41 asset methods directly
 * (issuance/transfer happens inside the care-agreement contract), it only
 * needs to identify the asset for display and simulation-result parsing.
 */
export function settlementAssetContractId(config: StellarClientConfig): string {
  return config.settlementAssetContractId;
}

export function createCareFundContracts(config: StellarClientConfig): CareFundContracts {
  return {
    providerRegistry: new Contract(config.providerRegistryContractId),
    careAgreement: new Contract(config.careAgreementContractId),
  };
}
