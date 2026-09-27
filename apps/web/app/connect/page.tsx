"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useWallet } from "@/lib/wallet-context";

export default function ConnectPage() {
  const router = useRouter();
  const { status, address, error, connect } = useWallet();

  useEffect(() => {
    if (status === "connected" && address) {
      router.replace("/sponsor");
    }
  }, [status, address, router]);

  return (
    <div className="mx-auto max-w-lg px-4 py-16 text-center sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Connect your wallet</h1>
      <p className="mt-4 text-sm text-ink-600 dark:text-ink-300">
        CareFund never holds your keys or signs on your behalf. Connect Freighter to sponsor,
        fund, or manage care agreements — every transaction is signed in your own wallet.
      </p>

      {status === "not_installed" && (
        <div className="mt-8 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6 text-sm">
          <p>Freighter isn&rsquo;t installed in this browser.</p>
          <a
            href="https://www.freighter.app/"
            target="_blank"
            rel="noreferrer"
            className="focus-ring mt-4 inline-block rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
          >
            Install Freighter
          </a>
        </div>
      )}

      {status === "wrong_network" && (
        <p role="alert" className="mt-8 rounded-lg border border-danger-500 p-6 text-sm text-danger-500">
          Freighter is set to the wrong network. Switch it to match this app&rsquo;s configured
          network, then try again.
        </p>
      )}

      {(status === "disconnected" || status === "checking") && (
        <button
          type="button"
          onClick={connect}
          disabled={status === "checking"}
          className="focus-ring mt-8 rounded-lg bg-brand-600 px-6 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700 disabled:opacity-50"
        >
          {status === "checking" ? "Checking…" : "Connect wallet"}
        </button>
      )}

      {error && (
        <p role="alert" className="mt-4 text-xs text-danger-500">
          {error}
        </p>
      )}
    </div>
  );
}
