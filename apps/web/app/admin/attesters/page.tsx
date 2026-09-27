"use client";

import { useCallback, useEffect, useState } from "react";
import { listAttesters, type AttesterResponse } from "@/lib/api-client";

type LoadState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "loaded"; attesters: AttesterResponse[]; nextCursor?: string };

export default function AdminAttestersPage() {
  const [load, setLoad] = useState<LoadState>({ status: "loading" });

  const loadPage = useCallback(async (cursor?: string) => {
    setLoad((prev) => (cursor && prev.status === "loaded" ? prev : { status: "loading" }));
    try {
      const page = await listAttesters({ cursor, limit: 25 });
      setLoad((prev) => ({
        status: "loaded",
        attesters: cursor && prev.status === "loaded" ? [...prev.attesters, ...page.attesters] : page.attesters,
        nextCursor: page.nextCursor,
      }));
    } catch (err) {
      setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to load attesters." });
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await Promise.resolve();
      if (cancelled) return;
      await loadPage();
    })();
    return () => {
      cancelled = true;
    };
  }, [loadPage]);

  return (
    <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Attester directory</h1>
      <p className="mt-2 text-ink-600 dark:text-ink-300">
        Every attester mirrored off-chain from the provider-registry contract, across all
        providers. A public, read-only view.
      </p>

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
          {load.attesters.length === 0 ? (
            <p className="mt-8 text-sm text-ink-600 dark:text-ink-300">No attesters registered yet.</p>
          ) : (
            <ul className="mt-8 space-y-3">
              {load.attesters.map((attester) => (
                <li
                  key={attester.walletAddress}
                  className="rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] px-4 py-3 text-sm"
                >
                  <div className="flex items-center justify-between gap-4">
                    <span className="font-mono break-all">{attester.walletAddress}</span>
                    <span className="text-ink-500 dark:text-ink-400">{attester.status}</span>
                  </div>
                  <p className="mt-1 text-xs text-ink-500 dark:text-ink-400">
                    Provider: <span className="font-mono">{attester.providerWallet}</span>
                  </p>
                </li>
              ))}
            </ul>
          )}

          {load.nextCursor && (
            <button
              type="button"
              onClick={() => loadPage(load.nextCursor)}
              className="focus-ring mt-6 rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--background)]"
            >
              Load more
            </button>
          )}
        </>
      )}
    </div>
  );
}
