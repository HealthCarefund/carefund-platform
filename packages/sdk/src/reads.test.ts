import { Buffer } from "buffer";
import { describe, expect, it } from "vitest";
import {
  mapActorStatus,
  mapAgreement,
  mapAgreementState,
  mapAttesterRecord,
  mapDisputeOrigin,
  mapProviderRecord,
} from "./reads.js";
import { toAgreementId, toStellarAddress } from "@carefund/types";

const ADDRESS_A = toStellarAddress("G" + "A".repeat(55));
const ADDRESS_B = toStellarAddress("G" + "B".repeat(55));
const ADDRESS_C = toStellarAddress("G" + "C".repeat(55));
const HASH_A = Buffer.from("a".repeat(64), "hex");
const HASH_B = Buffer.from("b".repeat(64), "hex");

describe("mapActorStatus", () => {
  it.each(["Active", "Suspended", "Revoked"] as const)("passes %s through unchanged", (tag) => {
    expect(mapActorStatus({ tag, values: undefined })).toBe(tag);
  });
});

describe("mapProviderRecord", () => {
  it("converts the provider_ref Buffer to a lowercase hex CommitmentHash", () => {
    const result = mapProviderRecord({ status: { tag: "Active", values: undefined }, provider_ref: HASH_A });
    expect(result).toEqual({ status: "Active", providerRef: "a".repeat(64) });
  });
});

describe("mapAttesterRecord", () => {
  it("maps provider address, status, and credential_ref together", () => {
    const result = mapAttesterRecord({
      provider: ADDRESS_A,
      status: { tag: "Suspended", values: undefined },
      credential_ref: HASH_B,
    });
    expect(result).toEqual({ provider: ADDRESS_A, status: "Suspended", credentialRef: "b".repeat(64) });
  });
});

describe("mapAgreementState", () => {
  it.each([
    "Requested",
    "Funded",
    "CareConfirmed",
    "Disputed",
    "Cancelled",
    "Expired",
    "Refunded",
    "Settled",
  ] as const)("passes %s through unchanged", (tag) => {
    expect(mapAgreementState({ tag, values: undefined })).toBe(tag);
  });
});

describe("mapDisputeOrigin", () => {
  it("returns undefined for None", () => {
    expect(mapDisputeOrigin({ tag: "None", values: undefined })).toBeUndefined();
  });

  it("unwraps Some to the inner DisputeOrigin tag", () => {
    expect(mapDisputeOrigin({ tag: "Some", values: [{ tag: "Funded", values: undefined }] })).toBe("Funded");
    expect(mapDisputeOrigin({ tag: "Some", values: [{ tag: "CareConfirmed", values: undefined }] })).toBe(
      "CareConfirmed",
    );
  });
});

describe("mapAgreement", () => {
  const agreementId = toAgreementId("42");

  function baseAgreement() {
    return {
      sponsor: ADDRESS_A,
      provider: ADDRESS_B,
      attester: ADDRESS_C,
      patient_ref_commitment: HASH_A,
      service_commitment: HASH_B,
      funding_amount: 1_000_000n,
      settlement_amount: 900_000n,
      funding_deadline: 1_700_000_000n,
      care_deadline: 1_700_100_000n,
      dispute_window_secs: 86_400n,
      state: { tag: "Requested" as const, values: undefined },
      attestation_commitment: undefined,
      attested_by: undefined,
      attested_at: undefined,
      dispute_origin: { tag: "None" as const, values: undefined },
      dispute_opened_by: undefined,
      dispute_opened_at: undefined,
      created_at: 1_699_000_000n,
    };
  }

  it("maps a freshly-requested agreement with no optional fields set", () => {
    const result = mapAgreement(agreementId, baseAgreement());

    expect(result).toEqual({
      agreementId: "42",
      sponsor: ADDRESS_A,
      provider: ADDRESS_B,
      attester: ADDRESS_C,
      patientRefCommitment: "a".repeat(64),
      serviceCommitment: "b".repeat(64),
      fundingAmount: "1000000",
      settlementAmount: "900000",
      fundingDeadline: "1700000000",
      careDeadline: "1700100000",
      disputeWindowSecs: "86400",
      state: "Requested",
      attestationCommitment: undefined,
      attestedBy: undefined,
      attestedAt: undefined,
      disputeOrigin: undefined,
      disputeOpenedBy: undefined,
      disputeOpenedAt: undefined,
      createdAt: "1699000000",
    });
  });

  it("maps a disputed agreement with attestation and dispute fields populated", () => {
    const result = mapAgreement(agreementId, {
      ...baseAgreement(),
      state: { tag: "Disputed", values: undefined },
      attestation_commitment: HASH_A,
      attested_by: ADDRESS_C,
      attested_at: 1_700_050_000n,
      dispute_origin: { tag: "Some", values: [{ tag: "CareConfirmed", values: undefined }] },
      dispute_opened_by: ADDRESS_A,
      dispute_opened_at: 1_700_060_000n,
    });

    expect(result.state).toBe("Disputed");
    expect(result.attestationCommitment).toBe("a".repeat(64));
    expect(result.attestedBy).toBe(ADDRESS_C);
    expect(result.attestedAt).toBe("1700050000");
    expect(result.disputeOrigin).toBe("CareConfirmed");
    expect(result.disputeOpenedBy).toBe(ADDRESS_A);
    expect(result.disputeOpenedAt).toBe("1700060000");
  });

  it("keeps dispute window as a duration, not a timestamp with the same numeric value as a deadline", () => {
    const result = mapAgreement(agreementId, baseAgreement());
    // Regression guard: an earlier draft of this mapping used
    // toLedgerTimestamp for dispute_window_secs (a duration), which
    // TypeScript caught at compile time. This asserts the runtime value
    // is still exactly the decimal string form of the u64, whichever
    // branded type it carries.
    expect(result.disputeWindowSecs).toBe("86400");
  });
});
