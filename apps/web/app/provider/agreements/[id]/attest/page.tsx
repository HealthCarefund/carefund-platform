"use client";

import { useState, type FormEvent } from "react";
import { useParams, useRouter } from "next/navigation";
import { isCommitmentHash, toCommitmentHash } from "@carefund/types";
import { useWallet } from "@/lib/wallet-context";
import { useTransactionFlow, prepareViaBackend } from "@/lib/use-transaction-flow";
import { TransactionStatus } from "@/app/components/transaction-status";

export default function AttestCarePage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { status: walletStatus, address, getSigner } = useWallet();
  const [commitment, setCommitment] = useState("");
  const [error, setError] = useState<string | null>(null);
  const { state, run, reset } = useTransactionFlow(getSigner);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!isCommitmentHash(commitment)) {
      setError("Enter a 64-character lowercase hex attestation commitment.");
      return;
    }
    setError(null);
    reset();
    await run(
      prepareViaBackend({
        agreementId: id,
        operation: "attest_care",
        sourcePublicKey: address!,
        attestationCommitment: toCommitmentHash(commitment),
      }),
    );
  }

  if (walletStatus !== "connected" || !address) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
        <h1 className="text-3xl font-semibold tracking-tight">Attest care delivered</h1>
        <p className="mt-4 text-sm text-ink-600 dark:text-ink-300">
          Connect your wallet to attest. Only the attester named on agreement #{id} may submit
          this call.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Attest care delivered</h1>
      <p className="mt-2 text-sm text-ink-600 dark:text-ink-300">
        Confirms, on-chain, that care under agreement #{id} was delivered. This must be signed by
        the attester wallet named on the agreement, as the contract will reject any other signer.
      </p>

      <form onSubmit={onSubmit} className="mt-8 space-y-5" noValidate>
        <div>
          <label className="text-sm font-medium" htmlFor="attestationCommitment">
            Attestation commitment (hex)
          </label>
          <input
            id="attestationCommitment"
            className="mt-1 w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--background)] px-3 py-2 font-mono text-sm focus-ring"
            value={commitment}
            onChange={(e) => setCommitment(e.target.value)}
            placeholder="64-character hex hash"
          />
          {error && <p className="mt-1 text-xs text-danger-500">{error}</p>}
        </div>

        <button
          type="submit"
          disabled={state.phase !== "idle" && state.phase !== "confirmed" && state.phase !== "failed" && state.phase !== "timeout"}
          className="focus-ring w-full rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700 disabled:opacity-50"
        >
          Sign and attest
        </button>
      </form>

      {state.phase !== "idle" && (
        <div className="mt-6">
          <TransactionStatus state={state} />
        </div>
      )}

      {state.phase === "confirmed" && (
        <button
          type="button"
          onClick={() => router.push(`/provider/agreements/${id}`)}
          className="focus-ring mt-4 rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
        >
          Back to agreement
        </button>
      )}
    </div>
  );
}
