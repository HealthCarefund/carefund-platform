import type { AgreementId, StellarAmount } from "./numeric.js";
import type { CommitmentHash, StellarAddress } from "./stellar.js";
import type { DurationSeconds, LedgerTimestamp } from "./timestamps.js";

/** Mirrors `AgreementState` in contracts/care-agreement/src/lib.rs exactly. */
export type AgreementState =
  | "Requested"
  | "Funded"
  | "CareConfirmed"
  | "Disputed"
  | "Cancelled"
  | "Expired"
  | "Refunded"
  | "Settled";

/**
 * Mirrors `DisputeOrigin` in contracts/care-agreement/src/lib.rs: the state
 * an agreement was in when a dispute was opened against it.
 */
export type DisputeOrigin = "Funded" | "CareConfirmed";

/** Mirrors `DisputeResolution` in contracts/care-agreement/src/lib.rs. */
export type DisputeResolution = "Resume" | "Settle" | "Refund";

/**
 * On-chain care agreement record. Field-for-field mirror of `Agreement` in
 * contracts/care-agreement/src/lib.rs (camelCase). This is the
 * settlement-authoritative record; anything not present here does not
 * exist on-chain. `patientRefCommitment` / `serviceCommitment` /
 * `attestationCommitment` are opaque hashes, never raw clinical data.
 */
export interface OnChainAgreement {
  readonly agreementId: AgreementId;

  readonly sponsor: StellarAddress;
  readonly provider: StellarAddress;
  readonly attester: StellarAddress;

  readonly patientRefCommitment: CommitmentHash;
  readonly serviceCommitment: CommitmentHash;

  readonly fundingAmount: StellarAmount;
  readonly settlementAmount: StellarAmount;

  readonly fundingDeadline: LedgerTimestamp;
  readonly careDeadline: LedgerTimestamp;
  readonly disputeWindowSecs: DurationSeconds;

  readonly state: AgreementState;

  readonly attestationCommitment?: CommitmentHash;
  readonly attestedBy?: StellarAddress;
  readonly attestedAt?: LedgerTimestamp;

  readonly disputeOrigin?: DisputeOrigin;
  readonly disputeOpenedBy?: StellarAddress;
  readonly disputeOpenedAt?: LedgerTimestamp;

  readonly createdAt: LedgerTimestamp;
}
