import type { rpc } from "@stellar/stellar-sdk";

/**
 * The lifecycle of a single contract-invocation transaction, from build
 * through confirmation. Each variant carries exactly the data available at
 * that point — nothing is optional-and-usually-undefined. This is a
 * client-side bookkeeping primitive; it is not, and must not be conflated
 * with, on-chain `AgreementState`.
 */
export type TransactionLifecycleState =
  | { readonly phase: "building" }
  | { readonly phase: "simulating" }
  | { readonly phase: "awaiting_signature" }
  | { readonly phase: "submitting" }
  | { readonly phase: "submitted"; readonly hash: string }
  | { readonly phase: "confirming"; readonly hash: string; readonly attempt: number }
  | {
      readonly phase: "success";
      readonly hash: string;
      readonly response: rpc.Api.GetSuccessfulTransactionResponse;
    }
  | { readonly phase: "failed"; readonly hash: string; readonly reason: string }
  | { readonly phase: "timeout"; readonly hash: string; readonly attempts: number };

export type TransactionLifecyclePhase = TransactionLifecycleState["phase"];

/**
 * The only phase transitions this SDK considers valid. Enforced by
 * `assertValidLifecycleTransition` so a caller wiring up UI state can't
 * accidentally (e.g.) jump from "building" straight to "success", or move
 * a "failed"/"timeout"/"success" transaction anywhere else — those are
 * terminal.
 */
const VALID_TRANSITIONS: Readonly<Record<TransactionLifecyclePhase, readonly TransactionLifecyclePhase[]>> = {
  building: ["simulating"],
  simulating: ["awaiting_signature", "failed"],
  awaiting_signature: ["submitting"],
  submitting: ["submitted", "failed"],
  submitted: ["confirming"],
  confirming: ["confirming", "success", "failed", "timeout"],
  success: [],
  failed: [],
  timeout: [],
};

export function isValidLifecycleTransition(
  from: TransactionLifecyclePhase,
  to: TransactionLifecyclePhase,
): boolean {
  return VALID_TRANSITIONS[from].includes(to);
}

export class InvalidLifecycleTransitionError extends Error {
  constructor(
    readonly from: TransactionLifecyclePhase,
    readonly to: TransactionLifecyclePhase,
  ) {
    super(`Invalid transaction lifecycle transition: ${from} -> ${to}`);
    this.name = "InvalidLifecycleTransitionError";
  }
}

export function assertValidLifecycleTransition(
  from: TransactionLifecycleState,
  to: TransactionLifecycleState,
): void {
  if (!isValidLifecycleTransition(from.phase, to.phase)) {
    throw new InvalidLifecycleTransitionError(from.phase, to.phase);
  }
}
