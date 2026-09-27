"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useWallet } from "@/lib/wallet-context";
import { listSponsorAgreements, type AgreementResponse } from "@/lib/api-client";

type LoadState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "loaded"; agreements: AgreementResponse[]; nextCursor?: string };

export default function SponsorAgreementsPage() {
  const { status: walletStatus, address } = useWallet();
  const [load, setLoad] = useState<LoadState>({ status: "idle" });

  const loadPage = useCallback(
    async (wallet: string, cursor?: string) => {
      setLoad({ status: "loading" });
      try {
        const page = await listSponsorAgreements(wallet, { cursor, limit: 20 });
        setLoad((prev) => ({
          status: "loaded",
          agreements: cursor && prev.status === "loaded" ? [...prev.agreements, ...page.agreements] : page.agreements,
          nextCursor: page.nextCursor,
        }));
      } catch (err) {
        setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to load agreements." });
      }
    },
    [],
  );

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await Promise.resolve();
      if (cancelled) return;
      if (walletStatus !== "connected" || !address) {
        setLoad({ status: "idle" });
        return;
      }
      await loadPage(address);
    })();
    return () => {
      cancelled = true;
    };
  }, [walletStatus, address, loadPage]);

  return (
    <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-3xl font-semibold tracking-tight">My agreements</h1>
        <Link
          href="/sponsor/agreements/new"
          className="focus-ring rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-700"
        >
          Request new
        </Link>
      </div>

      {walletStatus !== "connected" && (
        <p className="mt-8 text-sm text-ink-600 dark:text-ink-300">Connect your wallet to view your agreements.</p>
      )}

      {load.status === "loading" && (
        <p role="status" aria-live="polite" className="mt-8 text-sm text-ink-500 dark:text-ink-400">Loading&hellip;</p>
      )}

      {load.status === "error" && (
        <div role="alert" className="mt-8 rounded-lg border border-danger-500 p-6 text-sm text-danger-500">
          {load.message}
        </div>
      )}

      {load.status === "loaded" && (
        <>
          {load.agreements.length === 0 ? (
            <p className="mt-8 text-sm text-ink-600 dark:text-ink-300">
              No agreements found for this wallet yet. A provider must create the agreement
              on-chain before it will appear here.
            </p>
          ) : (
            <ul className="mt-8 space-y-3">
              {load.agreements.map((agreement) => (
                <li key={agreement.agreementId}>
                  <Link
                    href={`/sponsor/agreements/${agreement.agreementId}`}
                    className="focus-ring flex items-center justify-between rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] px-4 py-3 text-sm transition-colors hover:bg-[var(--background)]"
                  >
                    <span className="font-medium">Agreement #{agreement.agreementId}</span>
                    <span className="text-ink-500 dark:text-ink-400">{agreement.state}</span>
                  </Link>
                </li>
              ))}
            </ul>
          )}

          {load.nextCursor && address && (
            <button
              type="button"
              onClick={() => loadPage(address, load.nextCursor)}
              className="focus-ring mt-6 rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
            >
              Load more
            </button>
          )}
        </>
      )}
    </div>
  );
}
