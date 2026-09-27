import {
  getAddress,
  getNetworkDetails,
  isBrowser,
  isConnected,
  requestAccess,
  signTransaction as freighterSignTransaction,
} from "@stellar/freighter-api";
import type { StellarAddress } from "@carefund/types";
import { toStellarAddress } from "@carefund/types";
import type { StellarClientConfig } from "../config.js";
import type { WalletSigner } from "../transaction/sign.js";
import {
  WalletConnectionRejectedError,
  WalletNotInstalledError,
  WalletSigningRejectedError,
  WrongNetworkError,
} from "../errors.js";

/**
 * True when the Freighter browser extension is present. `isBrowser` from
 * the Freighter API is false in any non-browser environment (SSR, Node
 * tests), so this also guards against calling into it there.
 */
export async function isFreighterInstalled(): Promise<boolean> {
  if (!isBrowser) {
    return false;
  }
  const result = await isConnected();
  return !result.error && result.isConnected;
}

/**
 * Requests wallet access, prompting the user if this origin hasn't already
 * been granted it. Throws `WalletNotInstalledError` if the extension isn't
 * present, or `WalletConnectionRejectedError` if the user declines.
 */
export async function connectFreighterWallet(): Promise<StellarAddress> {
  if (!(await isFreighterInstalled())) {
    throw new WalletNotInstalledError();
  }
  const result = await requestAccess();
  if (result.error) {
    throw new WalletConnectionRejectedError(result.error.code, result.error.message);
  }
  return toStellarAddress(result.address);
}

/**
 * "Disconnecting" is purely local application state — Freighter has no
 * revoke API; the extension itself owns that grant, and a page can only
 * ask it to prompt again. This exists so wallet lifecycle handling has an
 * explicit, symmetric counterpart to `connectFreighterWallet` for callers
 * to clear their own held `publicKey`/signer state.
 */
export function disconnectFreighterWallet(): void {}

/** The currently authorized address, or `undefined` if none / not in a browser. */
export async function getConnectedFreighterAddress(): Promise<StellarAddress | undefined> {
  if (!isBrowser) {
    return undefined;
  }
  const result = await getAddress();
  if (result.error || !result.address) {
    return undefined;
  }
  return toStellarAddress(result.address);
}

/**
 * Throws `WrongNetworkError` if the wallet is not on the network this
 * config is validated for. Call this before every signing request, not
 * just at connect time — the user can switch networks in the extension at
 * any point after connecting.
 */
export async function verifyFreighterNetwork(config: StellarClientConfig): Promise<void> {
  const result = await getNetworkDetails();
  if (result.error) {
    throw new WalletConnectionRejectedError(result.error.code, result.error.message);
  }
  if (result.networkPassphrase !== config.networkPassphrase) {
    throw new WrongNetworkError(config.networkPassphrase, result.networkPassphrase);
  }
}

/**
 * A `WalletSigner` (see transaction/sign.ts) backed by Freighter. Verifies
 * the wallet's current network before every single signing request — this
 * check cannot be skipped by a caller — and maps a user's decline, or any
 * other Freighter-reported error, to `WalletSigningRejectedError` rather
 * than leaving it as an ambiguous `{error}` field.
 */
export function createFreighterSigner(config: StellarClientConfig, publicKey: StellarAddress): WalletSigner {
  return {
    publicKey,
    async signTransaction(transactionXdr, { networkPassphrase }) {
      if (networkPassphrase !== config.networkPassphrase) {
        throw new WrongNetworkError(config.networkPassphrase, networkPassphrase);
      }
      await verifyFreighterNetwork(config);

      const result = await freighterSignTransaction(transactionXdr, {
        networkPassphrase,
        address: publicKey,
      });
      if (result.error) {
        throw new WalletSigningRejectedError(result.error.code, result.error.message);
      }
      return { signedTransactionXdr: result.signedTxXdr };
    },
  };
}
