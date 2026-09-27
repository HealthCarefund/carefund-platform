"use client";

import { useCallback, useEffect, useState } from "react";
import { listProviders, type ProviderResponse } from "@/lib/api-client";

type LoadState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "loaded"; providers: ProviderResponse[]; nextCursor?: string };

export default function AdminProvidersPage() {
  const [load, setLoad] = useState<LoadState>({ status: "loading" });

  const loadPage = useCallback(async (cursor?: string) => {
    setLoad((prev) => (cursor && prev.status === "loaded" ? prev : { status: "loading" }));
    try {
      const page = await listProviders({ cursor, limit: 25 });
      setLoad((prev) => ({
        status: "loaded",
        providers: cursor && prev.status === "loaded" ? [...prev.providers, ...page.providers] : page.providers,
        nextCursor: page.nextCursor,
      }));
    } catch (err) {
      setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to load providers." });
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
      <h1 className="text-3xl font-semibold tracking-tight">Provider directory</h1>
      <p className="mt-2 text-ink-600 dark:text-ink-300">
        Every provider mirrored off-chain from the provider-registry contract. This is a public,
        read-only view — it grants no authority beyond what the contracts themselves enforce.
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
          {load.providers.length === 0 ? (
            <p className="mt-8 text-sm text-ink-600 dark:text-ink-300">No providers registered yet.</p>
          ) : (
            <ul className="mt-8 space-y-3">
              {load.providers.map((provider) => (
                <li
                  key={provider.walletAddress}
                  className="flex items-center justify-between gap-4 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] px-4 py-3 text-sm"
                >
                  <span className="font-mono break-all">{provider.walletAddress}</span>
                  <span className="text-ink-500 dark:text-ink-400">{provider.status}</span>
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
