import { rpc } from "@stellar/stellar-sdk";
import type { Transaction } from "@stellar/stellar-sdk";
import { SimulationFailedError } from "../errors.js";

/** Narrow dependency so tests can supply a fixture without a real RPC client. */
export type SimulationClient = Pick<rpc.Server, "simulateTransaction" | "prepareTransaction">;

/**
 * Simulates a transaction and returns the exact SDK response type — callers
 * that need footprint/auth/cost details get them unconverted. Throws
 * `SimulationFailedError` when the simulation itself reports failure, so
 * a caller can `await simulate(...)` and know they hold a
 * `SimulateTransactionSuccessResponse` (or a restore response) on return.
 */
export async function simulateContractCall(
  server: SimulationClient,
  transaction: Transaction,
): Promise<rpc.Api.SimulateTransactionSuccessResponse | rpc.Api.SimulateTransactionRestoreResponse> {
  const response = await server.simulateTransaction(transaction);
  if (rpc.Api.isSimulationError(response)) {
    throw new SimulationFailedError(response.error);
  }
  return response;
}

/**
 * Simulates, then assembles the transaction with the resources/auth/fee the
 * simulation reported (`prepareTransaction` in the underlying SDK does
 * both). The result is ready for the wallet to sign — no further mutation
 * should happen after this step.
 */
export async function prepareContractCallTransaction(
  server: SimulationClient,
  transaction: Transaction,
): Promise<Transaction> {
  return server.prepareTransaction(transaction);
}
