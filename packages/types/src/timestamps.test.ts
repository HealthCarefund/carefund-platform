import { describe, expect, it } from "vitest";
import {
  isDurationSeconds,
  isIsoTimestamp,
  isLedgerTimestamp,
  toIsoTimestamp,
  toLedgerTimestamp,
} from "./timestamps.js";
import { U64_MAX } from "./numeric.js";

describe("isLedgerTimestamp", () => {
  it("accepts zero and the u64 maximum", () => {
    expect(isLedgerTimestamp("0")).toBe(true);
    expect(isLedgerTimestamp(U64_MAX.toString())).toBe(true);
  });

  it("rejects negative values and one past the u64 maximum", () => {
    expect(isLedgerTimestamp("-1")).toBe(false);
    expect(isLedgerTimestamp((U64_MAX + 1n).toString())).toBe(false);
  });

  it("rejects non-canonical forms", () => {
    expect(isLedgerTimestamp("01")).toBe(false);
    expect(isLedgerTimestamp("")).toBe(false);
  });
});

describe("isDurationSeconds", () => {
  it("shares LedgerTimestamp's validation rules", () => {
    expect(isDurationSeconds("3600")).toBe(true);
    expect(isDurationSeconds("-1")).toBe(false);
  });
});

describe("isIsoTimestamp", () => {
  it("accepts canonical ISO-8601 with a Z offset", () => {
    expect(isIsoTimestamp("2026-09-27T00:00:00Z")).toBe(true);
  });

  it("accepts fractional seconds and numeric offsets", () => {
    expect(isIsoTimestamp("2026-09-27T00:00:00.123Z")).toBe(true);
    expect(isIsoTimestamp("2026-09-27T00:00:00+05:30")).toBe(true);
  });

  it("rejects a date with no time component", () => {
    expect(isIsoTimestamp("2026-09-27")).toBe(false);
  });

  it("rejects a non-ISO, human-readable date string", () => {
    expect(isIsoTimestamp("September 27, 2026")).toBe(false);
  });

  it("rejects garbage", () => {
    expect(isIsoTimestamp("not-a-date")).toBe(false);
  });
});

describe("toLedgerTimestamp / toIsoTimestamp", () => {
  it("return the value unchanged when valid", () => {
    expect(toLedgerTimestamp("100")).toBe("100");
    expect(toIsoTimestamp("2026-09-27T00:00:00Z")).toBe("2026-09-27T00:00:00Z");
  });

  it("throw on invalid input", () => {
    expect(() => toLedgerTimestamp("-1")).toThrow(TypeError);
    expect(() => toIsoTimestamp("not-a-date")).toThrow(TypeError);
  });
});
