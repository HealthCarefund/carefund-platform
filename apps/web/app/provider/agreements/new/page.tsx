"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import {
  toStellarAddress,
  toCommitmentHash,
  toStellarAmount,
  toLedgerTimestamp,
  toDurationSeconds,
} from "@carefund/types";
import { useWallet } from "@/lib/wallet-context";
import { useTransactionFlow, prepareCreateAgreement } from "@/lib/use-transaction-flow";
import { TransactionStatus } from "@/app/components/transaction-status";
import { validateAgreementFormValues, type AgreementFormValues } from "@/lib/agreement-form-validation";

type FormValues = AgreementFormValues;

const EMPTY: FormValues = {
  counterpartyWallet: "",
  attester: "",
  patientRefCommitment: "",
  serviceCommitment: "",
  fundingAmount: "",
  settlementAmount: "",
  fundingDeadline: "",
  careDeadline: "",
  disputeWindowHours: "72",
};

const inputClass =
  "mt-1 w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--background)] px-3 py-2 text-sm focus-ring";
const labelClass = "text-sm font-medium";
const errorClass = "mt-1 text-xs text-danger-500";

export default function NewAgreementPage() {
  const router = useRouter();
  const { status: walletStatus, address, getSigner } = useWallet();
  const [values, setValues] = useState<FormValues>(EMPTY);
  const [errors, setErrors] = useState<Partial<Record<keyof FormValues, string>>>({});
  const { state, run, reset } = useTransactionFlow(getSigner);

  const field = (key: keyof FormValues) => ({
    value: values[key],
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => setValues((v) => ({ ...v, [key]: e.target.value })),
  });

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!address) return;
    const validationErrors = validateAgreementFormValues(values, "sponsor");
    setErrors(validationErrors);
    if (Object.keys(validationErrors).length > 0) return;

    reset();
    await run(
      prepareCreateAgreement({
        provider: toStellarAddress(address),
        sponsor: toStellarAddress(values.counterpartyWallet),
        attester: toStellarAddress(values.attester),
        patientRefCommitment: toCommitmentHash(values.patientRefCommitment),
        serviceCommitment: toCommitmentHash(values.serviceCommitment),
        fundingAmount: toStellarAmount(values.fundingAmount),
        settlementAmount: toStellarAmount(values.settlementAmount),
        fundingDeadline: toLedgerTimestamp(String(Math.floor(Date.parse(values.fundingDeadline) / 1000))),
        careDeadline: toLedgerTimestamp(String(Math.floor(Date.parse(values.careDeadline) / 1000))),
        disputeWindowSecs: toDurationSeconds(String(Math.round(Number(values.disputeWindowHours) * 3600))),
        sourcePublicKey: toStellarAddress(address),
        timeoutSeconds: 60,
      }),
    );
  }

  if (walletStatus !== "connected" || !address) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
        <h1 className="text-3xl font-semibold tracking-tight">Propose a new agreement</h1>
        <p className="mt-4 text-sm text-ink-600 dark:text-ink-300">
          Connect your wallet to propose a new care agreement. Only the provider named in the
          agreement may submit this call.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Propose a new agreement</h1>
      <p className="mt-2 text-sm text-ink-600 dark:text-ink-300">
        This creates the agreement directly on-chain, signed by your connected wallet as the
        provider. Commitments must be pre-computed hashes; never enter patient or clinical data
        directly.
      </p>

      <form onSubmit={onSubmit} className="mt-8 space-y-5" noValidate>
        <div>
          <label className={labelClass} htmlFor="counterpartyWallet">Sponsor wallet address</label>
          <input id="counterpartyWallet" className={inputClass} {...field("counterpartyWallet")} placeholder="G..." />
          {errors.counterpartyWallet && <p className={errorClass}>{errors.counterpartyWallet}</p>}
        </div>

        <div>
          <label className={labelClass} htmlFor="attester">Attester wallet address</label>
          <input id="attester" className={inputClass} {...field("attester")} placeholder="G..." />
          {errors.attester && <p className={errorClass}>{errors.attester}</p>}
        </div>

        <div>
          <label className={labelClass} htmlFor="patientRefCommitment">Patient reference commitment (hex)</label>
          <input
            id="patientRefCommitment"
            className={`${inputClass} font-mono`}
            {...field("patientRefCommitment")}
            placeholder="64-character hex hash"
          />
          {errors.patientRefCommitment && <p className={errorClass}>{errors.patientRefCommitment}</p>}
        </div>

        <div>
          <label className={labelClass} htmlFor="serviceCommitment">Service commitment (hex)</label>
          <input
            id="serviceCommitment"
            className={`${inputClass} font-mono`}
            {...field("serviceCommitment")}
            placeholder="64-character hex hash"
          />
          {errors.serviceCommitment && <p className={errorClass}>{errors.serviceCommitment}</p>}
        </div>

        <div className="grid gap-5 sm:grid-cols-2">
          <div>
            <label className={labelClass} htmlFor="fundingAmount">Funding amount</label>
            <input id="fundingAmount" className={inputClass} {...field("fundingAmount")} placeholder="1000000" inputMode="numeric" />
            {errors.fundingAmount && <p className={errorClass}>{errors.fundingAmount}</p>}
          </div>
          <div>
            <label className={labelClass} htmlFor="settlementAmount">Settlement amount</label>
            <input id="settlementAmount" className={inputClass} {...field("settlementAmount")} placeholder="900000" inputMode="numeric" />
            {errors.settlementAmount && <p className={errorClass}>{errors.settlementAmount}</p>}
          </div>
        </div>

        <div className="grid gap-5 sm:grid-cols-2">
          <div>
            <label className={labelClass} htmlFor="fundingDeadline">Funding deadline</label>
            <input id="fundingDeadline" type="datetime-local" className={inputClass} {...field("fundingDeadline")} />
            {errors.fundingDeadline && <p className={errorClass}>{errors.fundingDeadline}</p>}
          </div>
          <div>
            <label className={labelClass} htmlFor="careDeadline">Care deadline</label>
            <input id="careDeadline" type="datetime-local" className={inputClass} {...field("careDeadline")} />
            {errors.careDeadline && <p className={errorClass}>{errors.careDeadline}</p>}
          </div>
        </div>

        <div>
          <label className={labelClass} htmlFor="disputeWindowHours">Dispute window (hours)</label>
          <input id="disputeWindowHours" className={inputClass} {...field("disputeWindowHours")} inputMode="numeric" />
          {errors.disputeWindowHours && <p className={errorClass}>{errors.disputeWindowHours}</p>}
        </div>

        <button
          type="submit"
          disabled={state.phase !== "idle" && state.phase !== "confirmed" && state.phase !== "failed" && state.phase !== "timeout"}
          className="focus-ring w-full rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700 disabled:opacity-50"
        >
          Sign and create agreement
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
          onClick={() => router.push("/provider/agreements")}
          className="focus-ring mt-4 rounded-lg border border-[var(--border-subtle)] px-4 py-2 text-sm font-medium transition-colors hover:bg-[var(--surface)]"
        >
          View my agreements
        </button>
      )}
    </div>
  );
}
