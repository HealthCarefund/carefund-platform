import { describe, expect, it } from "vitest";
import {
  InvalidLifecycleTransitionError,
  assertValidLifecycleTransition,
  isValidLifecycleTransition,
  type TransactionLifecycleState,
} from "./lifecycle.js";

describe("isValidLifecycleTransition", () => {
  it("allows the full happy path from building to success", () => {
    expect(isValidLifecycleTransition("building", "simulating")).toBe(true);
    expect(isValidLifecycleTransition("simulating", "awaiting_signature")).toBe(true);
    expect(isValidLifecycleTransition("awaiting_signature", "submitting")).toBe(true);
    expect(isValidLifecycleTransition("submitting", "submitted")).toBe(true);
    expect(isValidLifecycleTransition("submitted", "confirming")).toBe(true);
    expect(isValidLifecycleTransition("confirming", "success")).toBe(true);
  });

  it("allows confirming to poll itself again", () => {
    expect(isValidLifecycleTransition("confirming", "confirming")).toBe(true);
  });

  it("allows confirming to end in failed or timeout", () => {
    expect(isValidLifecycleTransition("confirming", "failed")).toBe(true);
    expect(isValidLifecycleTransition("confirming", "timeout")).toBe(true);
  });

  it("allows simulation and submission to fail", () => {
    expect(isValidLifecycleTransition("simulating", "failed")).toBe(true);
    expect(isValidLifecycleTransition("submitting", "failed")).toBe(true);
  });

  it("rejects skipping phases", () => {
    expect(isValidLifecycleTransition("building", "success")).toBe(false);
    expect(isValidLifecycleTransition("building", "submitted")).toBe(false);
  });

  it("rejects moving backwards", () => {
    expect(isValidLifecycleTransition("submitted", "awaiting_signature")).toBe(false);
  });

  it("treats success, failed, and timeout as terminal", () => {
    for (const terminal of ["success", "failed", "timeout"] as const) {
      for (const target of [
        "building",
        "simulating",
        "awaiting_signature",
        "submitting",
        "submitted",
        "confirming",
        "success",
        "failed",
        "timeout",
      ] as const) {
        expect(isValidLifecycleTransition(terminal, target)).toBe(false);
      }
    }
  });
});

describe("assertValidLifecycleTransition", () => {
  it("does not throw for a valid transition", () => {
    const from: TransactionLifecycleState = { phase: "building" };
    const to: TransactionLifecycleState = { phase: "simulating" };
    expect(() => assertValidLifecycleTransition(from, to)).not.toThrow();
  });

  it("throws InvalidLifecycleTransitionError for an invalid transition", () => {
    const from: TransactionLifecycleState = { phase: "submitted", hash: "abc" };
    const to: TransactionLifecycleState = { phase: "building" };
    expect(() => assertValidLifecycleTransition(from, to)).toThrow(InvalidLifecycleTransitionError);
  });
});
