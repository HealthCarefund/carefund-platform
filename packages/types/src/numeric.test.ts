import { describe, expect, it } from "vitest";
import {
  I128_MAX,
  I128_MIN,
  U64_MAX,
  isAgreementId,
  isStellarAmount,
  toAgreementId,
  toStellarAmount,
} from "./numeric.js";

describe("isAgreementId", () => {
  it("accepts zero and ordinary values", () => {
    expect(isAgreementId("0")).toBe(true);
    expect(isAgreementId("1")).toBe(true);
    expect(isAgreementId("42")).toBe(true);
  });

  it("accepts the exact u64 maximum", () => {
    expect(isAgreementId(U64_MAX.toString())).toBe(true);
  });

  it("rejects one past the u64 maximum", () => {
    expect(isAgreementId((U64_MAX + 1n).toString())).toBe(false);
  });

  it("rejects negative values", () => {
    expect(isAgreementId("-1")).toBe(false);
  });

  it("rejects non-integer and non-canonical forms", () => {
    expect(isAgreementId("1.5")).toBe(false);
    expect(isAgreementId("01")).toBe(false);
    expect(isAgreementId("")).toBe(false);
    expect(isAgreementId("abc")).toBe(false);
  });
});

describe("toAgreementId", () => {
  it("returns the value unchanged when valid", () => {
    expect(toAgreementId("42")).toBe("42");
  });

  it("throws on an invalid id", () => {
    expect(() => toAgreementId("-1")).toThrow(TypeError);
  });
});

describe("isStellarAmount", () => {
  it("accepts zero and ordinary positive/negative values", () => {
    expect(isStellarAmount("0")).toBe(true);
    expect(isStellarAmount("100")).toBe(true);
    expect(isStellarAmount("-100")).toBe(true);
  });

  it("accepts the exact i128 bounds", () => {
    expect(isStellarAmount(I128_MAX.toString())).toBe(true);
    expect(isStellarAmount(I128_MIN.toString())).toBe(true);
  });

  it("rejects one past the i128 bounds", () => {
    expect(isStellarAmount((I128_MAX + 1n).toString())).toBe(false);
    expect(isStellarAmount((I128_MIN - 1n).toString())).toBe(false);
  });

  it("rejects -0 and non-canonical forms", () => {
    expect(isStellarAmount("-0")).toBe(false);
    expect(isStellarAmount("007")).toBe(false);
    expect(isStellarAmount("1.5")).toBe(false);
  });
});

describe("toStellarAmount", () => {
  it("returns the value unchanged when valid", () => {
    expect(toStellarAmount("-100")).toBe("-100");
  });

  it("throws on an invalid amount", () => {
    expect(() => toStellarAmount("not-a-number")).toThrow(TypeError);
  });
});
