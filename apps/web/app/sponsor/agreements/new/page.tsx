"use client";

import { useState, type FormEvent } from "react";
import { useWallet } from "@/lib/wallet-context";
import { getStellarConfig } from "@/lib/stellar-config";
import { createAgreementIntent, ApiError, type IntentResponse } from "@/lib/api-client";
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

type SubmitState =
  | { status: "idle" }
  | { status: "submitting" }
  | { status: "error"; message: string }
  | { status: "created"; intent: IntentResponse };

export default function NewSponsorAgreementPage() {
  const { status: walletStatus, address } = useWallet();
  const [values, setValues] = useState<FormValues>(EMPTY);
  const [errors, setErrors] = useState<Partial<Record<keyof FormValues, string>>>({});
  const [submit, setSubmit] = useState<SubmitState>({ status: "idle" });

  const field = (key: keyof FormValues) => ({
    value: values[key],
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => setValues((v) => ({ ...v, [key]: e.target.value })),
  });

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!address) return;
    const validationErrors = validateAgreementFormValues(values, "provider");
    setErrors(validationErrors);
    if (Object.keys(validationErrors).length > 0) return;

    setSubmit({ status: "submitting" });
    try {
      const intent = await createAgreementIntent({
        sponsorWallet: address,
        providerWallet: values.counterpartyWallet,
        attesterWallet: values.attester,
        patientRefCommitment: values.patientRefCommitment,
        serviceCommitment: values.serviceCommitment,
        fundingAmount: values.fundingAmount,
        settlementAmount: values.settlementAmount,
        settlementAssetContractId: getStellarConfig().settlementAssetContractId,
        fundingDeadline: String(Math.floor(Date.parse(values.fundingDeadline) / 1000)),
        careDeadline: String(Math.floor(Date.parse(values.careDeadline) / 1000)),
        disputeWindowSecs: String(Math.round(Number(values.disputeWindowHours) * 3600)),
      });
      setSubmit({ status: "created", intent });
    } catch (err) {
      const message =
        err instanceof ApiError ? err.body?.message ?? "The request was rejected." : "Failed to create the intent.";
      setSubmit({ status: "error", message });
    }
  }

  if (walletStatus !== "connected" || !address) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
        <h1 className="text-3xl font-semibold tracking-tight">Request a new agreement</h1>
        <p className="mt-4 text-sm text-ink-600 dark:text-ink-300">
          Connect your wallet to request a new care agreement.
        </p>
      </div>
    );
  }

  if (submit.status === "created") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
        <h1 className="text-3xl font-semibold tracking-tight">Request submitted</h1>
        <div className="mt-6 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface)] p-6">
          <p className="text-sm text-ink-600 dark:text-ink-300">
            Intent <span className="font-mono">{submit.intent.id}</span> was recorded off-chain
            with status <span className="font-medium">{submit.intent.status}</span>. Share this
            intent ID with the provider; they must call <code>create_agreement</code> on-chain
            themselves before it will appear as a real agreement. This request does not, by
            itself, create or fund anything on-chain.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="text-3xl font-semibold tracking-tight">Request a new agreement</h1>
      <p className="mt-2 text-sm text-ink-600 dark:text-ink-300">
        This records an off-chain request only. The named provider must still create the
        agreement on-chain before it can be funded, as sponsors cannot create agreements directly.
        Commitments must be pre-computed hashes; never enter patient or clinical data directly.
      </p>

      <form onSubmit={onSubmit} className="mt-8 space-y-5" noValidate>
        <div>
          <label className={labelClass} htmlFor="counterpartyWallet">Provider wallet address</label>
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

        {submit.status === "error" && (
          <p role="alert" className="text-sm text-danger-500">{submit.message}</p>
        )}

        <button
          type="submit"
          disabled={submit.status === "submitting"}
          className="focus-ring w-full rounded-lg bg-brand-600 px-5 py-3 text-sm font-medium text-white transition-colors hover:bg-brand-700 disabled:opacity-50"
        >
          Submit request
        </button>
      </form>
    </div>
  );
}
