import { rpc } from "@stellar/stellar-sdk";
import type { Transaction } from "@stellar/stellar-sdk";
import { SubmissionRejectedError } from "../errors.js";

/** Narrow dependency so tests can supply a fixture without a real RPC client. */
export type TransactionSubmitClient = Pick<rpc.Server, "sendTransaction">;

/**
 * Submits a signed transaction. RPC does not wait for completion — it only
 * validates and enqueues. A `PENDING` result means the transaction hash
 * must be looked up later (`lookupTransaction`/`waitForTransaction`); this
 * function never polls or resubmits.
 */
export async function submitTransaction(
  server: TransactionSubmitClient,
  transaction: Transaction,
): Promise<{ readonly hash: string; readonly status: "PENDING" }> {
  const response = await server.sendTransaction(transaction);

  if (response.status !== "PENDING") {
    throw new SubmissionRejectedError(response.status, response.errorResult?.toXDR("base64"));
  }

  return { hash: response.hash, status: "PENDING" };
}
