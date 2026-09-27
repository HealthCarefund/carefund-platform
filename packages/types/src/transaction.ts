import type { AgreementId } from "./numeric.js";
import type { TransactionHash } from "./stellar.js";
import type { IsoTimestamp } from "./timestamps.js";

/**
 * Off-chain lifecycle of a Stellar transaction the application has
 * submitted, as tracked by CareFund's API — distinct from, and unrelated
 * to, `AgreementState`. A transaction moves through this lifecycle once
 * per submission; it does not represent contract state.
 */
export type TransactionLifecycleStatus = "pending" | "submitted" | "confirmed" | "failed";

export type TransactionRecord =
  | {
      readonly status: "pending";
      readonly hash?: undefined;
    }
  | {
      readonly status: "submitted" | "confirmed";
      readonly hash: TransactionHash;
      readonly submittedAt: IsoTimestamp;
    }
  | {
      readonly status: "failed";
      readonly hash?: TransactionHash;
      readonly submittedAt: IsoTimestamp;
      readonly errorMessage: string;
    };

/**
 * Off-chain record of an `attest_care` submission. The commitment and
 * `attestedAt` ledger timestamp are on-chain-authoritative (see
 * `OnChainAgreement`); this wraps them with application bookkeeping about
 * the transaction that carried the attestation.
 */
export interface AttestationMetadata {
  readonly agreementId: AgreementId;
  readonly transaction: TransactionRecord;
  readonly recordedAt: IsoTimestamp;
}

/**
 * Off-chain record of a settlement-affecting submission (`settle` or a
 * `resolve_dispute` that pays out). `settlementAmount` on `OnChainAgreement`
 * remains the authoritative amount; this wraps the transaction that
 * executed it.
 */
export interface SettlementMetadata {
  readonly agreementId: AgreementId;
  readonly transaction: TransactionRecord;
  readonly recordedAt: IsoTimestamp;
}
