import { describe, expect, it } from "vitest";
import {
  isCommitmentHash,
  isStellarAddress,
  isTransactionHash,
  toCommitmentHash,
  toStellarAddress,
  toTransactionHash,
} from "./stellar.js";

const VALID_ACCOUNT_ADDRESS = "G" + "A".repeat(55);
const VALID_CONTRACT_ADDRESS = "C" + "B".repeat(55);
const VALID_HASH = "a".repeat(64);

describe("isStellarAddress", () => {
  it("accepts a well-formed account address", () => {
    expect(isStellarAddress(VALID_ACCOUNT_ADDRESS)).toBe(true);
  });

  it("accepts a well-formed contract address", () => {
    expect(isStellarAddress(VALID_CONTRACT_ADDRESS)).toBe(true);
  });

  it("rejects an address with an unknown prefix", () => {
    expect(isStellarAddress("A" + "A".repeat(55))).toBe(false);
  });

  it("rejects an address that is too short", () => {
    expect(isStellarAddress("G" + "A".repeat(54))).toBe(false);
  });

  it("rejects an address that is too long", () => {
    expect(isStellarAddress("G" + "A".repeat(56))).toBe(false);
  });

  it("rejects lowercase characters", () => {
    expect(isStellarAddress("G" + "a".repeat(55))).toBe(false);
  });

  it("rejects characters outside the base32 alphabet (0, 1, 8, 9)", () => {
    expect(isStellarAddress("G" + "1".repeat(55))).toBe(false);
  });
});

describe("toStellarAddress", () => {
  it("returns the value unchanged when valid", () => {
    expect(toStellarAddress(VALID_ACCOUNT_ADDRESS)).toBe(VALID_ACCOUNT_ADDRESS);
  });

  it("throws on an invalid address", () => {
    expect(() => toStellarAddress("not-an-address")).toThrow(TypeError);
  });
});

describe("isCommitmentHash / isTransactionHash", () => {
  it("accepts a 64-character lowercase hex string", () => {
    expect(isCommitmentHash(VALID_HASH)).toBe(true);
    expect(isTransactionHash(VALID_HASH)).toBe(true);
  });

  it("rejects uppercase hex", () => {
    expect(isCommitmentHash(VALID_HASH.toUpperCase())).toBe(false);
  });

  it("rejects the wrong length", () => {
    expect(isCommitmentHash(VALID_HASH.slice(0, 63))).toBe(false);
    expect(isCommitmentHash(VALID_HASH + "a")).toBe(false);
  });

  it("rejects non-hex characters", () => {
    expect(isCommitmentHash("g".repeat(64))).toBe(false);
  });
});

describe("toCommitmentHash / toTransactionHash", () => {
  it("returns the value unchanged when valid", () => {
    expect(toCommitmentHash(VALID_HASH)).toBe(VALID_HASH);
    expect(toTransactionHash(VALID_HASH)).toBe(VALID_HASH);
  });

  it("throws on an invalid hash", () => {
    expect(() => toCommitmentHash("nope")).toThrow(TypeError);
    expect(() => toTransactionHash("nope")).toThrow(TypeError);
  });
});
