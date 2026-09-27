"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useWallet } from "@/lib/wallet-context";
import { getProvider, listProviderAttesters, ApiError, type ProviderResponse, type AttesterResponse } from "@/lib/api-client";

type LoadState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "not_registered" }
  | { status: "error"; message: string }
  | { status: "loaded"; provider: ProviderResponse; attesters: AttesterResponse[] };

export default function ProviderDashboardPage() {
  const { status: walletStatus, address } = useWallet();
  const [load, setLoad] = useState<LoadState>({ status: "idle" });

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await Promise.resolve();
      if (cancelled) return;
      if (walletStatus !== "connected" || !address) {
        setLoad({ status: "idle" });
        return;
      }
      setLoad({ status: "loading" });
      try {
        const [provider, attesters] = await Promise.all([
          getProvider(address),
          listProviderAttesters(address),
        ]);
        if (cancelled) return;
        setLoad({ status: "loaded", provider, attesters });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          setLoad({ status: "not_registered" });
          return;
        }
        setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to load provider." });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [walletStatus, address]);

  return (
    <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Provider dashboard</h1>
      <p className="mt-2 text-ink-600 dark:text-ink-300">
        Manage care agreements you provide, and confirm care once it has been delivered.
      </p>

      {walletStatus !== "connected" && (
        <div className="mt-8 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
          <p className="text-sm text-ink-600 dark:text-ink-300">
            Connect your wallet using the button in the header to view your provider status and
            agreements.
          </p>
        </div>
      )}

      {load.status === "loading" && (
        <p className="mt-8 text-sm text-ink-500 dark:text-ink-400">Loading&hellip;</p>
      )}

      {load.status === "not_registered" && (
        <div className="mt-8 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
          <p className="text-sm text-ink-600 dark:text-ink-300">
            This wallet is not registered as a provider on-chain. Registration is performed
            through the provider-registry contract by an authorized registrar — this app cannot
            grant that status itself.
          </p>
        </div>
      )}

      {load.status === "error" && (
        <div role="alert" className="mt-8 rounded-lg border border-danger-500 p-6 text-sm text-danger-500">
          {load.message}
        </div>
      )}

      {load.status === "loaded" && (
        <div className="mt-8 space-y-6">
          <div className="rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-sm text-ink-500 dark:text-ink-400">Status</p>
                <p className="mt-1 text-lg font-medium">{load.provider.status}</p>
              </div>
              <span className="rounded-full border border-[var(--border-subtle)] px-3 py-1 text-xs font-mono">
                {load.provider.walletAddress}
              </span>
            </div>
          </div>

          <div>
            <h2 className="text-sm font-semibold tracking-wide text-ink-500 uppercase dark:text-ink-400">
              Authorized attesters
            </h2>
            {load.attesters.length === 0 ? (
              <p className="mt-2 text-sm text-ink-600 dark:text-ink-300">
                No attesters are registered for this provider yet.
              </p>
            ) : (
              <ul className="mt-3 space-y-2">
                {load.attesters.map((attester) => (
                  <li
                    key={attester.walletAddress}
                    className="flex items-center justify-between rounded-lg border border-[var(--border-subtle)] px-4 py-3 text-sm"
                  >
                    <span className="font-mono">{attester.walletAddress}</span>
                    <span className="text-ink-500 dark:text-ink-400">{attester.status}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {load.provider.status === "Active" && (
            <div className="flex flex-wrap gap-3">
              <Link
                href="/provider/agreements"
                className="focus-ring rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
              >
                View my agreements
              </Link>
              <Link
                href="/provider/agreements/new"
                className="focus-ring rounded-lg border border-[var(--border-subtle)] px-5 py-3 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
              >
                Propose a new agreement
              </Link>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
