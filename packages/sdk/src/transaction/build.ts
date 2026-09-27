import { Account, BASE_FEE, Contract, TransactionBuilder, xdr } from "@stellar/stellar-sdk";
import type { Transaction } from "@stellar/stellar-sdk";
import type { StellarClientConfig } from "../config.js";

/** Inputs needed to build a single, unsigned contract-invocation transaction. */
export interface ContractCallRequest {
  readonly contract: Contract;
  readonly method: string;
  readonly args: readonly xdr.ScVal[];
  /** The invoking account (source), with its current sequence number. */
  readonly sourceAccount: Account;
  /** Seconds until the transaction's time bounds expire. Required — no infinite default. */
  readonly timeoutSeconds: number;
  /** Base fee in stroops, before simulation-derived resource fee padding. Defaults to `BASE_FEE`. */
  readonly baseFee?: string;
}

/**
 * Builds an unsigned transaction invoking a single contract method. This is
 * the "transaction preparation" step only: not simulated, not fee-padded,
 * not signed. Call `simulateTransaction`/`prepareTransaction` next.
 */
export function buildContractCallTransaction(
  config: StellarClientConfig,
  request: ContractCallRequest,
): Transaction {
  return new TransactionBuilder(request.sourceAccount, {
    fee: request.baseFee ?? BASE_FEE,
    networkPassphrase: config.networkPassphrase,
  })
    .addOperation(request.contract.call(request.method, ...request.args))
    .setTimeout(request.timeoutSeconds)
    .build();
}
