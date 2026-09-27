"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  connectFreighterWallet,
  disconnectFreighterWallet,
  getConnectedFreighterAddress,
  isFreighterInstalled,
  verifyFreighterNetwork,
  createFreighterSigner,
  type WalletSigner,
  WalletNotInstalledError,
  WalletConnectionRejectedError,
  WrongNetworkError,
} from "@carefund/sdk";
import type { StellarAddress } from "@carefund/types";
import { getStellarConfig } from "./stellar-config";

export type WalletStatus =
  | "checking"
  | "not_installed"
  | "disconnected"
  | "wrong_network"
  | "connected";

interface WalletState {
  status: WalletStatus;
  address: StellarAddress | null;
  error: string | null;
  connect: () => Promise<void>;
  disconnect: () => void;
  getSigner: () => WalletSigner;
}

const WalletContext = createContext<WalletState | null>(null);

export function WalletProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<WalletStatus>("checking");
  const [address, setAddress] = useState<StellarAddress | null>(null);
  const [error, setError] = useState<string | null>(null);

  const checkNetwork = useCallback(async () => {
    try {
      await verifyFreighterNetwork(getStellarConfig());
      return true;
    } catch (err) {
      if (err instanceof WrongNetworkError) {
        setStatus("wrong_network");
        setError(
          `Wallet is on the wrong network. Switch Freighter to match this app's configured network before continuing.`,
        );
        return false;
      }
      throw err;
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const installed = await isFreighterInstalled();
      if (cancelled) return;
      if (!installed) {
        setStatus("not_installed");
        return;
      }
      const existing = await getConnectedFreighterAddress();
      if (cancelled) return;
      if (!existing) {
        setStatus("disconnected");
        return;
      }
      setAddress(existing);
      const networkOK = await checkNetwork();
      if (cancelled) return;
      if (networkOK) setStatus("connected");
    })();
    return () => {
      cancelled = true;
    };
  }, [checkNetwork]);

  const connect = useCallback(async () => {
    setError(null);
    try {
      const connectedAddress = await connectFreighterWallet();
      setAddress(connectedAddress);
      const networkOK = await checkNetwork();
      if (networkOK) setStatus("connected");
    } catch (err) {
      if (err instanceof WalletNotInstalledError) {
        setStatus("not_installed");
        return;
      }
      if (err instanceof WalletConnectionRejectedError) {
        setStatus("disconnected");
        setError("Wallet connection was declined.");
        return;
      }
      setStatus("disconnected");
      setError(err instanceof Error ? err.message : "Failed to connect wallet.");
    }
  }, [checkNetwork]);

  const disconnect = useCallback(() => {
    disconnectFreighterWallet();
    setAddress(null);
    setStatus("disconnected");
    setError(null);
  }, []);

  const getSigner = useCallback((): WalletSigner => {
    if (!address) {
      throw new Error("Cannot sign: no wallet is connected");
    }
    return createFreighterSigner(getStellarConfig(), address);
  }, [address]);

  const value = useMemo(
    () => ({ status, address, error, connect, disconnect, getSigner }),
    [status, address, error, connect, disconnect, getSigner],
  );

  return <WalletContext.Provider value={value}>{children}</WalletContext.Provider>;
}

export function useWallet(): WalletState {
  const ctx = useContext(WalletContext);
  if (!ctx) {
    throw new Error("useWallet must be used within a WalletProvider");
  }
  return ctx;
}
