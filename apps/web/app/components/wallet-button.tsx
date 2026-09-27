"use client";

import { useWallet } from "@/lib/wallet-context";

function shortenAddress(address: string): string {
  return `${address.slice(0, 4)}…${address.slice(-4)}`;
}

export function WalletButton() {
  const { status, address, error, connect, disconnect } = useWallet();

  if (status === "checking") {
    return (
      <span className="rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm text-ink-400">
        Checking wallet&hellip;
      </span>
    );
  }

  if (status === "connected" && address) {
    return (
      <button
        type="button"
        onClick={disconnect}
        className="focus-ring rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
      >
        <span aria-hidden="true" className="mr-2 inline-block h-2 w-2 rounded-full bg-success-500" />
        {shortenAddress(address)}
      </button>
    );
  }

  if (status === "wrong_network") {
    return (
      <span
        role="alert"
        className="rounded-lg border border-danger-500 px-4 py-2 text-sm text-danger-500"
      >
        Wrong network
      </span>
    );
  }

  if (status === "not_installed") {
    return (
      <a
        href="https://www.freighter.app/"
        target="_blank"
        rel="noreferrer"
        className="focus-ring rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
      >
        Install Freighter
      </a>
    );
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        type="button"
        onClick={connect}
        className="focus-ring rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-700"
      >
        Connect wallet
      </button>
      {error && (
        <p role="alert" className="text-xs text-danger-500">
          {error}
        </p>
      )}
    </div>
  );
}
