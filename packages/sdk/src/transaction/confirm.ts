import { rpc } from "@stellar/stellar-sdk";
import { TransactionFailedError, TransactionTimeoutError } from "../errors.js";

/**
 * The narrow slice of `rpc.Server` these functions depend on, so tests can
 * supply a controlled fixture for `getTransaction` without constructing a
 * real RPC client — this is not a stand-in for live blockchain behavior,
 * just dependency narrowing.
 */
export type TransactionLookupClient = Pick<rpc.Server, "getTransaction">;

/**
 * Looks up a submitted transaction by hash exactly once. This is the only
 * way this SDK reconciles an uncertain submission — it never resubmits.
 */
export async function lookupTransaction(
  server: TransactionLookupClient,
  hash: string,
): Promise<rpc.Api.GetTransactionResponse> {
  return server.getTransaction(hash);
}

export interface WaitForTransactionOptions {
  /** Maximum number of `getTransaction` polls before giving up. */
  readonly attempts: number;
  /** Milliseconds to wait between polls. */
  readonly intervalMs: number;
  /** Injectable for tests; defaults to `setTimeout`-based real sleep. */
  readonly sleep?: (ms: number) => Promise<void>;
}

const defaultSleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Polls `getTransaction` for a bounded number of attempts, looking for a
 * definitive SUCCESS or FAILED status.
 *
 * - SUCCESS resolves with the full success response (exact SDK type).
 * - FAILED throws `TransactionFailedError`.
 * - NOT_FOUND after every attempt is exhausted throws
 *   `TransactionTimeoutError` — the transaction's fate is still unknown at
 *   that point; it must be looked up again later, and must never be
 *   silently resubmitted, since a still-pending submission could yet land.
 */
export async function waitForTransaction(
  server: TransactionLookupClient,
  hash: string,
  options: WaitForTransactionOptions,
): Promise<rpc.Api.GetSuccessfulTransactionResponse> {
  const sleep = options.sleep ?? defaultSleep;

  for (let attempt = 1; attempt <= options.attempts; attempt += 1) {
    const response = await server.getTransaction(hash);

    if (response.status === rpc.Api.GetTransactionStatus.SUCCESS) {
      return response;
    }

    if (response.status === rpc.Api.GetTransactionStatus.FAILED) {
      throw new TransactionFailedError(hash);
    }

    if (attempt < options.attempts) {
      await sleep(options.intervalMs);
    }
  }

  throw new TransactionTimeoutError(hash, options.attempts);
}
