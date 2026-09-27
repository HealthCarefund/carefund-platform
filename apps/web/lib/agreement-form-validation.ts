import { isStellarAddress, isCommitmentHash, isStellarAmount } from "@carefund/types";

// Shared by the provider "propose a new agreement" form (creates
// create_agreement directly) and the sponsor "request a new agreement"
// form (creates an off-chain agreement intent) — both collect the same
// fields with the same constraints, differing only in which wallet field
// is the caller's own (provider vs sponsor) and what they do with the
// result.
export interface AgreementFormValues {
  counterpartyWallet: string;
  attester: string;
  patientRefCommitment: string;
  serviceCommitment: string;
  fundingAmount: string;
  settlementAmount: string;
  fundingDeadline: string;
  careDeadline: string;
  disputeWindowHours: string;
}

export type AgreementFormErrors = Partial<Record<keyof AgreementFormValues, string>>;

export function validateAgreementFormValues(
  values: AgreementFormValues,
  counterpartyLabel: string,
): AgreementFormErrors {
  const errors: AgreementFormErrors = {};

  if (!isStellarAddress(values.counterpartyWallet)) {
    errors.counterpartyWallet = `Enter a valid ${counterpartyLabel} Stellar address (G...).`;
  }
  if (!isStellarAddress(values.attester)) {
    errors.attester = "Enter a valid Stellar address (G...).";
  }
  if (!isCommitmentHash(values.patientRefCommitment)) {
    errors.patientRefCommitment = "Enter a 64-character lowercase hex commitment hash, not patient data.";
  }
  if (!isCommitmentHash(values.serviceCommitment)) {
    errors.serviceCommitment = "Enter a 64-character lowercase hex commitment hash.";
  }
  if (!isStellarAmount(values.fundingAmount) || BigInt(values.fundingAmount || "0") <= BigInt(0)) {
    errors.fundingAmount = "Enter a positive whole-number amount in the settlement asset's smallest unit.";
  }
  if (!isStellarAmount(values.settlementAmount) || BigInt(values.settlementAmount || "0") <= BigInt(0)) {
    errors.settlementAmount = "Enter a positive whole-number amount.";
  }

  const fundingDeadlineMs = Date.parse(values.fundingDeadline);
  if (Number.isNaN(fundingDeadlineMs) || fundingDeadlineMs <= Date.now()) {
    errors.fundingDeadline = "Choose a funding deadline in the future.";
  }

  const careDeadlineMs = Date.parse(values.careDeadline);
  if (Number.isNaN(careDeadlineMs) || careDeadlineMs <= fundingDeadlineMs) {
    errors.careDeadline = "Choose a care deadline after the funding deadline.";
  }

  const disputeHours = Number(values.disputeWindowHours);
  if (!Number.isFinite(disputeHours) || disputeHours <= 0) {
    errors.disputeWindowHours = "Enter a positive number of hours.";
  }

  return errors;
}
