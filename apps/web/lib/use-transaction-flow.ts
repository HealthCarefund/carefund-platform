"use client";

import { useCallback, useRef, useState } from "react";
import { TransactionBuilder, type Transaction } from "@stellar/stellar-sdk";
import {
  createRpcServer,
  submitTransaction as submitToRpc,
  awaitWalletSignature,
  simulateContractCall,
  prepareContractCallTransaction,
  createAgreementTransaction,
  WalletSigningRejectedError,
  SubmissionRejectedError,
  SimulationFailedError,
  type WalletSigner,
  type CreateAgreementParams,
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

/** Resolves to an unsigned, ready-to-sign transaction plus the network passphrase it was built for. */
export type TransactionPreparer = () => Promise<{ transaction: Transaction; networkPassphrase: string }>;

export interface RunTransactionParams {
  agreementId: string;
  operation: PrepareTransactionOperation;
  sourcePublicKey: string;
  attestationCommitment?: string;
  resolution?: "Resume" | "Settle" | "Refund";
}

/**
 * Prepares one of the seven agreement-lifecycle operations via the Go
 * backend (build + simulate + assemble happen server-side; see
 * apps/api/internal/api/transactions.go, Commit 13). Use this for
 * fund/cancel/attest_care/open_dispute/expire/settle/resolve_dispute.
 */
export function prepareViaBackend(params: RunTransactionParams): TransactionPreparer {
  return async () => {
    const prepared = await prepareAgreementTransaction(params.agreementId, {
      operation: params.operation,
      sourcePublicKey: params.sourcePublicKey,
      attestationCommitment: params.attestationCommitment,
      resolution: params.resolution,
    });
    const envelope = TransactionBuilder.fromXDR(prepared.unsignedTransactionXdr, prepared.networkPassphrase);
    if (!("signatures" in envelope)) {
      throw new Error("Prepared transaction is not a signable envelope");
    }
    return { transaction: envelope as Transaction, networkPassphrase: prepared.networkPassphrase };
  };
}

/**
 * Prepares a create_agreement call directly against RPC, client-side.
 * There is deliberately no backend endpoint for this one operation (see
 * apps/api/internal/api/transactions.go's comment): create_agreement
 * belongs to an off-chain agreement_intent, not an existing agreementId,
 * so it doesn't fit that endpoint's URL shape. packages/sdk already
 * builds exactly this (Commit 5); this just adds the same
 * simulate-then-assemble step the backend performs for the other seven.
 */
export function prepareCreateAgreement(params: CreateAgreementParams): TransactionPreparer {
  return async () => {
    const config = getStellarConfig();
    const rpc = createRpcServer(config);
    const built = await createAgreementTransaction(config, rpc, params);
    // Throws SimulationFailedError itself if the simulation reports the
    // call would fail — never silently proceeds to signing on a bad call.
    await simulateContractCall(rpc, built);
    const prepared = await prepareContractCallTransaction(rpc, built);
    return { transaction: prepared, networkPassphrase: config.networkPassphrase };
  };
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
    async (prepare: TransactionPreparer) => {
      cancelled.current = false;

      try {
        setState({ phase: "preparing" });
        setState({ phase: "simulating" });
        const { transaction, networkPassphrase } = await prepare();
        if (cancelled.current) return;

        setState({ phase: "awaiting_signature" });
        const signed = await awaitWalletSignature(getSigner(), transaction, networkPassphrase);
        if (cancelled.current) return;

        setState({ phase: "submitting" });
        const rpc = createRpcServer(getStellarConfig());
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
  if (err instanceof SimulationFailedError) {
    return err.message;
  }
  if (err instanceof ApiError) {
    return err.body?.message ?? "The request was rejected.";
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "An unexpected error occurred.";
}
