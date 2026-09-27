import type { rpc } from "@stellar/stellar-sdk";
import type { ContractErrorCode, ContractName } from "@carefund/types";

/** Base class for every error this SDK throws. Never thrown directly. */
export abstract class SdkError extends Error {
  abstract readonly code: string;
}

/**
 * Configuration failed validation before any network call was made.
 * `issues` lists every problem found, not just the first.
 */
export class ConfigValidationError extends SdkError {
  readonly code = "CONFIG_INVALID";
  constructor(readonly issues: readonly string[]) {
    super(`Invalid Stellar client configuration: ${issues.join("; ")}`);
    this.name = "ConfigValidationError";
  }
}

/**
 * A read (or a simulated write) came back with one of the contract's own
 * `#[contracterror]` variants, carried through unchanged from the
 * generated bindings' `Result.unwrapErr().message` — never re-derived or
 * guessed at this layer.
 */
export class ContractCallError extends SdkError {
  readonly code = "CONTRACT_CALL_FAILED";
  constructor(readonly contract: ContractName, readonly contractErrorCode: ContractErrorCode) {
    super(`${contract} rejected the call: ${contractErrorCode}`);
    this.name = "ContractCallError";
  }
}

/** Simulation reported the transaction would fail. */
export class SimulationFailedError extends SdkError {
  readonly code = "SIMULATION_FAILED";
  constructor(readonly simulationError: string) {
    super(`Transaction simulation failed: ${simulationError}`);
    this.name = "SimulationFailedError";
  }
}

/**
 * `sendTransaction` returned a definitive rejection (`ERROR`) or a status
 * this SDK does not auto-retry (`DUPLICATE`, `TRY_AGAIN_LATER`). The caller
 * decides what, if anything, to do next — this SDK never resubmits on its
 * own; see `lookupTransaction`/`waitForTransaction`.
 */
export class SubmissionRejectedError extends SdkError {
  readonly code = "SUBMISSION_REJECTED";
  constructor(
    readonly status: rpc.Api.SendTransactionStatus,
    readonly errorResultXdr?: string,
  ) {
    super(`Transaction submission was not accepted: ${status}`);
    this.name = "SubmissionRejectedError";
  }
}

/** No Freighter extension was detected in this browser. */
export class WalletNotInstalledError extends SdkError {
  readonly code = "WALLET_NOT_INSTALLED";
  constructor() {
    super("Freighter wallet extension was not detected in this browser");
    this.name = "WalletNotInstalledError";
  }
}

/**
 * The user declined the connect/access-request prompt, or Freighter
 * otherwise refused to grant access. Carries Freighter's own error code
 * and message unchanged.
 */
export class WalletConnectionRejectedError extends SdkError {
  readonly code = "WALLET_CONNECTION_REJECTED";
  constructor(readonly freighterCode: number, readonly freighterMessage: string) {
    super(`Wallet connection was rejected: ${freighterMessage}`);
    this.name = "WalletConnectionRejectedError";
  }
}

/**
 * The wallet is connected to a different network than this app is
 * configured for. Signing must never proceed in this state — a
 * transaction signed for the wrong network is either rejected outright or,
 * worse, valid on a network the user didn't intend to use.
 */
export class WrongNetworkError extends SdkError {
  readonly code = "WRONG_NETWORK";
  constructor(readonly expectedPassphrase: string, readonly actualPassphrase: string) {
    super(
      `Wallet is connected to the wrong network: expected passphrase "${expectedPassphrase}", wallet reports "${actualPassphrase}"`,
    );
    this.name = "WrongNetworkError";
  }
}

/**
 * The user declined to sign, or Freighter otherwise refused to sign, a
 * specific transaction. Distinct from `WalletConnectionRejectedError`
 * (declining the initial connection) and from any submission/confirmation
 * error (which can only happen after a signature was actually obtained).
 */
export class WalletSigningRejectedError extends SdkError {
  readonly code = "WALLET_SIGNING_REJECTED";
  constructor(readonly freighterCode: number, readonly freighterMessage: string) {
    super(`Wallet declined to sign the transaction: ${freighterMessage}`);
    this.name = "WalletSigningRejectedError";
  }
}

/** `getTransaction` confirmed the transaction landed but failed on-chain. */
export class TransactionFailedError extends SdkError {
  readonly code = "TRANSACTION_FAILED";
  constructor(readonly hash: string) {
    super(`Transaction ${hash} failed on-chain`);
    this.name = "TransactionFailedError";
  }
}

/**
 * Confirmation polling exhausted its attempts without a definitive
 * SUCCESS/FAILED status. The transaction's fate is still unknown — it must
 * be looked up again, never blindly resubmitted.
 */
export class TransactionTimeoutError extends SdkError {
  readonly code = "TRANSACTION_TIMEOUT";
  constructor(readonly hash: string, readonly attempts: number) {
    super(`Transaction ${hash} was not confirmed after ${attempts} attempts`);
    this.name = "TransactionTimeoutError";
  }
}
