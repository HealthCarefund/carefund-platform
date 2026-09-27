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
