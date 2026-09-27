"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { lookupTransaction, type TransactionLookupResponse } from "@/lib/api-client";

type LoadState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "loaded"; result: TransactionLookupResponse };

const STATUS_LABEL: Record<TransactionLookupResponse["status"], string> = {
  success: "Confirmed on-chain",
  failed: "Failed on-chain",
  not_found: "Not found (not yet confirmed, or never submitted)",
};

export default function TransactionStatusPage() {
  const { hash } = useParams<{ hash: string }>();
  const [load, setLoad] = useState<LoadState>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await Promise.resolve();
      if (cancelled) return;
      setLoad({ status: "loading" });
      try {
        const result = await lookupTransaction(hash);
        if (cancelled) return;
        setLoad({ status: "loaded", result });
      } catch (err) {
        if (cancelled) return;
        setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to look up transaction." });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [hash]);

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Transaction status</h1>
      <p className="mt-2 font-mono text-sm break-all text-ink-500 dark:text-ink-400">{hash}</p>

      {load.status === "loading" && (
        <p role="status" aria-live="polite" className="mt-8 text-sm text-ink-500 dark:text-ink-400">Looking up&hellip;</p>
      )}

      {load.status === "error" && (
        <div role="alert" className="mt-8 rounded-lg border border-danger-500 p-6 text-sm text-danger-500">
          {load.message}
        </div>
      )}

      {load.status === "loaded" && (
        <div className="mt-8 space-y-4">
          <div className="rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
            <p className="text-sm text-ink-500 dark:text-ink-400">Status</p>
            <p className="mt-1 text-lg font-medium">{STATUS_LABEL[load.result.status]}</p>
            {load.result.ledger && (
              <p className="mt-1 text-sm text-ink-500 dark:text-ink-400">Ledger {load.result.ledger}</p>
            )}
          </div>

          {load.result.reconciliation && (
            <div className="rounded-lg border border-[var(--border-subtle)] p-6 text-sm">
              <p className="font-medium">Reconciliation</p>
              <dl className="mt-3 space-y-2">
                <div className="flex justify-between gap-4">
                  <dt className="text-ink-500 dark:text-ink-400">Operation</dt>
                  <dd>{load.result.reconciliation.operation}</dd>
                </div>
                {load.result.reconciliation.agreementId && (
                  <div className="flex justify-between gap-4">
                    <dt className="text-ink-500 dark:text-ink-400">Agreement</dt>
                    <dd>#{load.result.reconciliation.agreementId}</dd>
                  </div>
                )}
                <div className="flex justify-between gap-4">
                  <dt className="text-ink-500 dark:text-ink-400">First seen</dt>
                  <dd>{load.result.reconciliation.firstSeen}</dd>
                </div>
                {load.result.reconciliation.confirmedAt && (
                  <div className="flex justify-between gap-4">
                    <dt className="text-ink-500 dark:text-ink-400">Confirmed at</dt>
                    <dd>{load.result.reconciliation.confirmedAt}</dd>
                  </div>
                )}
                {load.result.reconciliation.errorCode && (
                  <div className="flex justify-between gap-4">
                    <dt className="text-ink-500 dark:text-ink-400">Error</dt>
                    <dd className="text-danger-500">
                      {load.result.reconciliation.errorCode}
                      {load.result.reconciliation.errorDetail ? ` — ${load.result.reconciliation.errorDetail}` : ""}
                    </dd>
                  </div>
                )}
              </dl>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
