import { rpc } from "@stellar/stellar-sdk";
import type { StellarClientConfig } from "./config.js";

/** Narrow dependency so tests can supply a fixture without a real RPC client. */
export type AccountLookupClient = Pick<rpc.Server, "getAccount">;

/**
 * Creates the Soroban RPC client for a validated config. `allowHttp` is
 * derived, never asked for separately, so a plain-HTTP RPC URL can only
 * ever be used on a `CUSTOM` (local/private) network.
 */
export function createRpcServer(config: StellarClientConfig): rpc.Server {
  return new rpc.Server(config.rpcUrl, {
    allowHttp: config.network === "CUSTOM" && new URL(config.rpcUrl).protocol === "http:",
  });
}
