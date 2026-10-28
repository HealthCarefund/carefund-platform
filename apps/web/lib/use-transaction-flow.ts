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
  getAgreement,
  validateAgreementLifecycleTransaction,
  validateCreateAgreementTransaction,
  WalletSigningRejectedError,
  SubmissionRejectedError,
  SimulationFailedError,
  TransactionValidationError,
  type WalletSigner,
  type CreateAgreementParams,
} from "@carefund/sdk";
import { toAgreementId } from "@carefund/types";
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
 *
 * The backend's response is never treated as authoritative: before
 * returning, this independently re-derives and checks the network,
 * source account, operation, contract, function, arguments, transaction
 * constraints and Soroban authorization against exactly what the caller
 * requested (see `@carefund/sdk`'s `validateAgreementLifecycleTransaction`,
 * packages/sdk/src/transaction/validate.ts). A mismatch throws
 * `TransactionValidationError` before any wallet-signing call happens.
 */
export function prepareViaBackend(params: RunTransactionParams): TransactionPreparer {
  return async () => {
    const config = getStellarConfig();
    const prepared = await prepareAgreementTransaction(params.agreementId, {
      operation: params.operation,
      sourcePublicKey: params.sourcePublicKey,
      attestationCommitment: params.attestationCommitment,
      resolution: params.resolution,
    });

    // Never parse (let alone sign for) a network other than the one this
    // app is independently configured for, even if the API returned a
    // passphrase alongside the XDR — that value is not trusted merely
    // because it arrived with the transaction.
    if (prepared.networkPassphrase !== config.networkPassphrase) {
      throw new TransactionValidationError([
        `network passphrase mismatch: expected "${config.networkPassphrase}", got "${prepared.networkPassphrase}"`,
      ]);
    }

    let envelope: ReturnType<typeof TransactionBuilder.fromXDR>;
    try {
      envelope = TransactionBuilder.fromXDR(prepared.unsignedTransactionXdr, prepared.networkPassphrase);
    } catch (err) {
      throw new TransactionValidationError([
        `malformed transaction XDR: ${err instanceof Error ? err.message : String(err)}`,
      ]);
    }
    if (!("signatures" in envelope)) {
      throw new TransactionValidationError(["prepared transaction is a fee-bump envelope, not a signable transaction"]);
    }
    const transaction = envelope as Transaction;

    // `fund`'s escrow-transfer amount is not one of its own call arguments
    // — the contract reads it from stored agreement state — so there is no
    // way to verify that nested authorization without an independent,
    // trustworthy source for it. Read it fresh from the contract itself,
    // never from this app's own off-chain cache.
    const expectedFundingAmount =
      params.operation === "fund"
        ? (await getAgreement(config, toAgreementId(params.agreementId))).fundingAmount
        : undefined;

    validateAgreementLifecycleTransaction(config, transaction, prepared.networkPassphrase, {
      operation: params.operation,
      agreementId: params.agreementId,
      sourcePublicKey: params.sourcePublicKey,
      attestationCommitment: params.attestationCommitment,
      resolution: params.resolution,
      expectedFundingAmount,
    });

    return { transaction, networkPassphrase: prepared.networkPassphrase };
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

    // Built and simulated client-side rather than returned by the API, but
    // validated the same way for consistency: this is the one path where a
    // compromised RPC endpoint (not a compromised backend) would have to be
    // the attacker, and the same checks catch it just as well.
    validateCreateAgreementTransaction(config, prepared, config.networkPassphrase, {
      sourcePublicKey: params.sourcePublicKey,
      provider: params.provider,
      sponsor: params.sponsor,
      attester: params.attester,
      patientRefCommitment: params.patientRefCommitment,
      serviceCommitment: params.serviceCommitment,
      fundingAmount: params.fundingAmount,
      settlementAmount: params.settlementAmount,
      fundingDeadline: params.fundingDeadline,
      careDeadline: params.careDeadline,
      disputeWindowSecs: params.disputeWindowSecs,
    });

    return { transaction: prepared, networkPassphrase: config.networkPassphrase };
  };
}

const CONFIRMATION_ATTEMPTS = 15;
const CONFIRMATION_INTERVAL_MS = 3000;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * The actual preparation-to-confirmation control flow: prepare (which
 * independently validates, per `prepareViaBackend`/`prepareCreateAgreement`
 * above) → wallet signing → RPC submission → confirmation polling. Pulled
 * out of `useTransactionFlow` as a plain async function — not a hook — so
 * it can be exercised directly in tests against the real implementation,
 * without rendering a React component or reimplementing this sequence
 * inside a test. `useTransactionFlow.run` below is a thin wrapper that
 * supplies `onState`/`isCancelled` from its own `useState`/`useRef`.
 *
 * Preparation strictly precedes signing: if `prepare()` rejects (as it
 * does for every `TransactionValidationError` case), `awaitWalletSignature`
 * is never reached — there is no path from a thrown `prepare()` to the
 * signing/submission steps below. The "failed" state is terminal here; it
 * never carries a hash because `prepare()` rejecting happens before any
 * hash could exist.
 */
export async function runTransactionPipeline(
  prepare: TransactionPreparer,
  getSigner: () => WalletSigner,
  onState: (state: TransactionFlowState) => void,
  isCancelled: () => boolean,
): Promise<void> {
  try {
    onState({ phase: "preparing" });
    onState({ phase: "simulating" });
    const { transaction, networkPassphrase } = await prepare();
    if (isCancelled()) return;

    onState({ phase: "awaiting_signature" });
    const signed = await awaitWalletSignature(getSigner(), transaction, networkPassphrase);
    if (isCancelled()) return;

    onState({ phase: "submitting" });
    const rpc = createRpcServer(getStellarConfig());
    const { hash } = await submitToRpc(rpc, signed);
    if (isCancelled()) return;
    onState({ phase: "submitted", hash });

    for (let attempt = 1; attempt <= CONFIRMATION_ATTEMPTS; attempt++) {
      if (isCancelled()) return;
      onState({ phase: "confirming", hash, attempt });
      const lookup = await lookupTransaction(hash);
      if (isCancelled()) return;

      if (lookup.status === "success") {
        onState({ phase: "confirmed", hash, ledger: lookup.ledger });
        return;
      }
      if (lookup.status === "failed") {
        onState({ phase: "failed", hash, reason: "Transaction failed on-chain" });
        return;
      }
      if (attempt < CONFIRMATION_ATTEMPTS) {
        await sleep(CONFIRMATION_INTERVAL_MS);
      }
    }
    onState({ phase: "timeout", hash });
  } catch (err) {
    if (isCancelled()) return;
    onState({ phase: "failed", reason: describeError(err) });
  }
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
      await runTransactionPipeline(prepare, getSigner, setState, () => cancelled.current);
    },
    [getSigner],
  );

  const cancel = useCallback(() => {
    cancelled.current = true;
  }, []);

  return { state, run, reset, cancel };
}

export function describeError(err: unknown): string {
  if (err instanceof WalletSigningRejectedError) {
    return "Wallet declined to sign the transaction.";
  }
  if (err instanceof SubmissionRejectedError) {
    return `Submission was rejected: ${err.status}`;
  }
  if (err instanceof SimulationFailedError) {
    return err.message;
  }
  if (err instanceof TransactionValidationError) {
    return "The prepared transaction did not match what was requested and was not sent to your wallet.";
  }
  if (err instanceof ApiError) {
    return err.body?.message ?? "The request was rejected.";
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "An unexpected error occurred.";
}
