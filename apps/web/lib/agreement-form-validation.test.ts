import { describe, expect, it } from "vitest";
import { validateAgreementFormValues, type AgreementFormValues } from "./agreement-form-validation";

const VALID_ADDRESS = "G" + "A".repeat(55);
const OTHER_VALID_ADDRESS = "G" + "B".repeat(55);
const VALID_HASH = "a".repeat(64);

function validValues(): AgreementFormValues {
  const future = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
  const laterFuture = new Date(Date.now() + 48 * 60 * 60 * 1000).toISOString();
  return {
    counterpartyWallet: VALID_ADDRESS,
    attester: OTHER_VALID_ADDRESS,
    patientRefCommitment: VALID_HASH,
    serviceCommitment: VALID_HASH,
    fundingAmount: "1000000",
    settlementAmount: "900000",
    fundingDeadline: future,
    careDeadline: laterFuture,
    disputeWindowHours: "72",
  };
}

describe("validateAgreementFormValues", () => {
  it("accepts a fully valid form", () => {
    expect(validateAgreementFormValues(validValues(), "provider")).toEqual({});
  });

  it("rejects an invalid counterparty wallet", () => {
    const errors = validateAgreementFormValues({ ...validValues(), counterpartyWallet: "not-a-wallet" }, "provider");
    expect(errors.counterpartyWallet).toMatch(/provider/);
  });

  it("rejects an invalid attester wallet", () => {
    const errors = validateAgreementFormValues({ ...validValues(), attester: "not-a-wallet" }, "provider");
    expect(errors.attester).toBeDefined();
  });

  it("rejects a commitment that isn't 64 lowercase hex characters", () => {
    const errors = validateAgreementFormValues({ ...validValues(), patientRefCommitment: "deadbeef" }, "provider");
    expect(errors.patientRefCommitment).toBeDefined();
  });

  it("rejects a zero funding amount", () => {
    const errors = validateAgreementFormValues({ ...validValues(), fundingAmount: "0" }, "provider");
    expect(errors.fundingAmount).toBeDefined();
  });

  it("rejects a negative settlement amount", () => {
    const errors = validateAgreementFormValues({ ...validValues(), settlementAmount: "-5" }, "provider");
    expect(errors.settlementAmount).toBeDefined();
  });

  it("rejects a funding deadline that is not in the future", () => {
    const past = new Date(Date.now() - 1000).toISOString();
    const errors = validateAgreementFormValues({ ...validValues(), fundingDeadline: past }, "provider");
    expect(errors.fundingDeadline).toBeDefined();
  });

  it("rejects a care deadline that is not after the funding deadline", () => {
    const values = validValues();
    const errors = validateAgreementFormValues({ ...values, careDeadline: values.fundingDeadline }, "provider");
    expect(errors.careDeadline).toBeDefined();
  });

  it("rejects a non-positive dispute window", () => {
    const errors = validateAgreementFormValues({ ...validValues(), disputeWindowHours: "0" }, "provider");
    expect(errors.disputeWindowHours).toBeDefined();
  });
});
