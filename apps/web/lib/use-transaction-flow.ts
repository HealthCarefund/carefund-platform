"use client";

import { useCallback, useRef, useState } from "react";
import { TransactionBuilder, type Transaction } from "@stellar/stellar-sdk";
import {
  createRpcServer,
  submitTransaction as submitToRpc,
  awaitWalletSignature,
  WalletSigningRejectedError,
  SubmissionRejectedError,
  type WalletSigner,
} from "@carefund/sdk";
import { getStellarConfig } from "./stellar-config";
import {
  prepareAgreementTransaction,
  lookupTransaction,
  ApiError,
  type PrepareTransactionOperation,
} from "./api-client";

/**
 * The transaction pipeline's stages, kept strictly distinct so the UI can
 * never show "Submitted" as "Confirmed" — those are different, and the
 * difference is the whole point of this hook. Mirrors the phase
 * vocabulary packages/sdk/src/transaction/lifecycle.ts already
 * establishes (building/simulating/awaiting_signature/submitting/
 * submitted/confirming/success/failed/timeout), adapted so failures that
 * happen before a hash exists (preparation, simulation, a declined
 * signature) don't need to fabricate one.
 */
export type TransactionFlowState =
  | { phase: "idle" }
  | { phase: "preparing" }
  | { phase: "simulating" }
  | { phase: "awaiting_signature" }
  | { phase: "submitting" }
  | { phase: "submitted"; hash: string }
  | { phase: "confirming"; hash: string; attempt: number }
  | { phase: "confirmed"; hash: string; ledger?: string }
  | { phase: "failed"; hash?: string; reason: string }
  | { phase: "timeout"; hash: string };

export interface RunTransactionParams {
  agreementId: string;
  operation: PrepareTransactionOperation;
  sourcePublicKey: string;
  attestationCommitment?: string;
  resolution?: "Resume" | "Settle" | "Refund";
}

const CONFIRMATION_ATTEMPTS = 15;
const CONFIRMATION_INTERVAL_MS = 3000;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function useTransactionFlow(getSigner: () => WalletSigner) {
  const [state, setState] = useState<TransactionFlowState>({ phase: "idle" });
  const cancelled = useRef(false);

  const reset = useCallback(() => {
    cancelled.current = false;
    setState({ phase: "idle" });
  }, []);

  const run = useCallback(
    async (params: RunTransactionParams) => {
      cancelled.current = false;
      const config = getStellarConfig();

      try {
        setState({ phase: "preparing" });
        setState({ phase: "simulating" });
        const prepared = await prepareAgreementTransaction(params.agreementId, {
          operation: params.operation,
          sourcePublicKey: params.sourcePublicKey,
          attestationCommitment: params.attestationCommitment,
          resolution: params.resolution,
        });
        if (cancelled.current) return;

        setState({ phase: "awaiting_signature" });
        const unsignedEnvelope = TransactionBuilder.fromXDR(
          prepared.unsignedTransactionXdr,
          prepared.networkPassphrase,
        );
        if (!("signatures" in unsignedEnvelope) || typeof unsignedEnvelope.toXDR !== "function") {
          throw new Error("Prepared transaction is not a signable envelope");
        }
        const signed = await awaitWalletSignature(
          getSigner(),
          unsignedEnvelope as Transaction,
          prepared.networkPassphrase,
        );
        if (cancelled.current) return;

        setState({ phase: "submitting" });
        const rpc = createRpcServer(config);
        const { hash } = await submitToRpc(rpc, signed);
        if (cancelled.current) return;
        setState({ phase: "submitted", hash });

        for (let attempt = 1; attempt <= CONFIRMATION_ATTEMPTS; attempt++) {
          if (cancelled.current) return;
          setState({ phase: "confirming", hash, attempt });
          const lookup = await lookupTransaction(hash);
          if (cancelled.current) return;

          if (lookup.status === "success") {
            setState({ phase: "confirmed", hash, ledger: lookup.ledger });
            return;
          }
          if (lookup.status === "failed") {
            setState({ phase: "failed", hash, reason: "Transaction failed on-chain" });
            return;
          }
          if (attempt < CONFIRMATION_ATTEMPTS) {
            await sleep(CONFIRMATION_INTERVAL_MS);
          }
        }
        setState({ phase: "timeout", hash });
      } catch (err) {
        if (cancelled.current) return;
        setState({ phase: "failed", reason: describeError(err) });
      }
    },
    [getSigner],
  );

  const cancel = useCallback(() => {
    cancelled.current = true;
  }, []);

  return { state, run, reset, cancel };
}

function describeError(err: unknown): string {
  if (err instanceof WalletSigningRejectedError) {
    return "Wallet declined to sign the transaction.";
  }
  if (err instanceof SubmissionRejectedError) {
    return `Submission was rejected: ${err.status}`;
  }
  if (err instanceof ApiError) {
    return err.body?.message ?? "The request was rejected.";
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "An unexpected error occurred.";
}
