export type { StellarNetwork, StellarClientConfig } from "./config.js";
export { validateStellarClientConfig, assertNoSecretMaterial } from "./config.js";

export { createRpcServer } from "./rpc.js";

export type { CareFundContracts } from "./contracts.js";
export { createCareFundContracts, settlementAssetContractId } from "./contracts.js";

export type { AccountLookupClient } from "./rpc.js";

export {
  createProviderRegistryReader,
  createCareAgreementReader,
  getProvider,
  getAttester,
  isProviderActive,
  isAttesterAuthorizedForProvider,
  getAgreement,
} from "./reads.js";

export type {
  CreateAgreementParams,
  FundAgreementParams,
  CancelAgreementParams,
  AttestCareParams,
  OpenDisputeParams,
  ExpireAgreementParams,
  SettleAgreementParams,
  ResolveDisputeParams,
} from "./transaction/builders.js";
export {
  createAgreementTransaction,
  fundAgreementTransaction,
  cancelAgreementTransaction,
  attestCareTransaction,
  openDisputeTransaction,
  expireAgreementTransaction,
  settleAgreementTransaction,
  resolveDisputeTransaction,
} from "./transaction/builders.js";

export type { ContractCallRequest } from "./transaction/build.js";
export { buildContractCallTransaction } from "./transaction/build.js";

export type { SimulationClient } from "./transaction/simulate.js";
export { simulateContractCall, prepareContractCallTransaction } from "./transaction/simulate.js";

export type { WalletSigner } from "./transaction/sign.js";
export { awaitWalletSignature } from "./transaction/sign.js";

export type { TransactionSubmitClient } from "./transaction/submit.js";
export { submitTransaction } from "./transaction/submit.js";

export type { WaitForTransactionOptions, TransactionLookupClient } from "./transaction/confirm.js";
export { lookupTransaction, waitForTransaction } from "./transaction/confirm.js";

export type { TransactionLifecycleState, TransactionLifecyclePhase } from "./transaction/lifecycle.js";
export {
  isValidLifecycleTransition,
  assertValidLifecycleTransition,
  InvalidLifecycleTransitionError,
} from "./transaction/lifecycle.js";

export {
  SdkError,
  ConfigValidationError,
  ContractCallError,
  SimulationFailedError,
  SubmissionRejectedError,
  TransactionFailedError,
  TransactionTimeoutError,
} from "./errors.js";
