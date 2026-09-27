"use client";

import Link from "next/link";
import { useWallet } from "@/lib/wallet-context";

export default function SponsorDashboardPage() {
  const { status, address } = useWallet();

  return (
    <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Sponsor dashboard</h1>
      <p className="mt-2 text-ink-600 dark:text-ink-300">
        Request care agreements, fund them once a provider has created them on-chain, and settle
        once care is confirmed.
      </p>

      {status !== "connected" || !address ? (
        <div className="mt-8 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
          <p className="text-sm text-ink-600 dark:text-ink-300">
            Connect your wallet to continue.
          </p>
          <Link
            href="/connect"
            className="focus-ring mt-4 inline-block rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
          >
            Connect wallet
          </Link>
        </div>
      ) : (
        <div className="mt-8 space-y-6">
          <div className="rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
            <p className="text-sm text-ink-500 dark:text-ink-400">Connected wallet</p>
            <p className="mt-1 font-mono text-sm break-all">{address}</p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link
              href="/sponsor/agreements"
              className="focus-ring rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
            >
              View my agreements
            </Link>
            <Link
              href="/sponsor/agreements/new"
              className="focus-ring rounded-lg border border-[var(--border-subtle)] px-5 py-3 text-sm font-medium transition-colors hover:bg-[var(--background)]"
            >
              Request a new agreement
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
