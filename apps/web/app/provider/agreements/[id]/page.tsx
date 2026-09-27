"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useWallet } from "@/lib/wallet-context";
import { useTransactionFlow, prepareViaBackend } from "@/lib/use-transaction-flow";
import { TransactionStatus } from "@/app/components/transaction-status";
import { getAgreement, ApiError, type AgreementResponse } from "@/lib/api-client";

type LoadState =
  | { status: "loading" }
  | { status: "not_found" }
  | { status: "error"; message: string }
  | { status: "loaded"; agreement: AgreementResponse };

const FIELD_LABELS: [keyof AgreementResponse, string][] = [
  ["sponsorWallet", "Sponsor"],
  ["providerWallet", "Provider"],
  ["attesterWallet", "Attester"],
  ["fundingAmount", "Funding amount"],
  ["settlementAmount", "Settlement amount"],
  ["settlementAssetContractId", "Settlement asset contract"],
  ["fundingDeadline", "Funding deadline (unix)"],
  ["careDeadline", "Care deadline (unix)"],
  ["disputeWindowSecs", "Dispute window (seconds)"],
];

export default function ProviderAgreementDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { status: walletStatus, address, getSigner } = useWallet();
  const [load, setLoad] = useState<LoadState>({ status: "loading" });
  const { state, run, reset } = useTransactionFlow(getSigner);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await Promise.resolve();
      if (cancelled) return;
      setLoad({ status: "loading" });
      try {
        const agreement = await getAgreement(id);
        if (cancelled) return;
        setLoad({ status: "loaded", agreement });
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          setLoad({ status: "not_found" });
          return;
        }
        setLoad({ status: "error", message: err instanceof Error ? err.message : "Failed to load agreement." });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  async function cancelAgreement() {
    reset();
    await run(
      prepareViaBackend({
        agreementId: id,
        operation: "cancel",
        sourcePublicKey: address!,
      }),
    );
  }

  if (load.status === "loading") {
    return <p className="mx-auto max-w-3xl px-4 py-12 text-sm text-ink-500 sm:px-6 dark:text-ink-400">Loading&hellip;</p>;
  }
  if (load.status === "not_found") {
    return <p className="mx-auto max-w-3xl px-4 py-12 text-sm text-ink-600 sm:px-6 dark:text-ink-300">Agreement not found.</p>;
  }
  if (load.status === "error") {
    return (
      <p role="alert" className="mx-auto max-w-3xl px-4 py-12 text-sm text-danger-500 sm:px-6">
        {load.message}
      </p>
    );
  }

  const { agreement } = load;
  const isMine = walletStatus === "connected" && address === agreement.providerWallet;

  return (
    <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-3xl font-semibold tracking-tight">Agreement #{agreement.agreementId}</h1>
        <span className="rounded-full border border-[var(--border-subtle)] px-3 py-1 text-xs font-medium">
          {agreement.state}
        </span>
      </div>

      <dl className="mt-8 divide-y divide-[var(--border-subtle)] rounded-lg border border-[var(--border-subtle)]">
        {FIELD_LABELS.map(([key, label]) => (
          <div key={key} className="flex items-center justify-between gap-4 px-4 py-3 text-sm">
            <dt className="text-ink-500 dark:text-ink-400">{label}</dt>
            <dd className="font-mono break-all">{agreement[key]}</dd>
          </div>
        ))}
      </dl>

      {isMine && (
        <div className="mt-8 flex flex-wrap gap-3">
          {agreement.state === "Funded" && (
            <Link
              href={`/provider/agreements/${agreement.agreementId}/attest`}
              className="focus-ring rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700"
            >
              Attest care delivered
            </Link>
          )}
          {agreement.state === "Requested" && (
            <button
              type="button"
              onClick={cancelAgreement}
              disabled={state.phase !== "idle" && state.phase !== "confirmed" && state.phase !== "failed" && state.phase !== "timeout"}
              className="focus-ring rounded-lg border border-danger-500 px-5 py-3 text-sm font-medium text-danger-500 transition-colors hover:bg-danger-500/10 disabled:opacity-50"
            >
              Cancel agreement
            </button>
          )}
        </div>
      )}

      {state.phase !== "idle" && (
        <div className="mt-6">
          <TransactionStatus state={state} />
        </div>
      )}
    </div>
  );
}
