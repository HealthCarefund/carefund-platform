import type { Brand } from "./brand.js";

/**
 * Contract integers (u64, i128) exceed `Number.MAX_SAFE_INTEGER` or don't
 * exist as a JS numeric type at all, so they are represented as decimal
 * strings at this boundary rather than `number`. Callers convert to
 * `bigint` when doing arithmetic.
 */

export const U64_MAX = (1n << 64n) - 1n;
export const I128_MIN = -(1n << 127n);
export const I128_MAX = (1n << 127n) - 1n;

function isDecimalIntegerString(value: string, allowNegative: boolean): boolean {
  const pattern = allowNegative ? /^-?\d+$/ : /^\d+$/;
  if (!pattern.test(value)) {
    return false;
  }
  // Reject forms bigint would otherwise happily parse but that aren't
  // canonical decimal integers (leading zeros, "-0").
  if (value === "-0") {
    return false;
  }
  const digits = value.startsWith("-") ? value.slice(1) : value;
  if (digits.length > 1 && digits.startsWith("0")) {
    return false;
  }
  return true;
}

/** The u64 `agreement_id` used as the on-chain `Agreement` storage key. */
export type AgreementId = Brand<string, "AgreementId">;

export function isAgreementId(value: string): value is AgreementId {
  if (!isDecimalIntegerString(value, false)) {
    return false;
  }
  return BigInt(value) <= U64_MAX;
}

export function toAgreementId(value: string): AgreementId {
  if (!isAgreementId(value)) {
    throw new TypeError(`Not a valid u64 agreement id: ${value}`);
  }
  return value;
}

/**
 * An i128 monetary amount (`funding_amount`, `settlement_amount`) in the
 * settlement asset's smallest unit, as a decimal string.
 */
export type StellarAmount = Brand<string, "StellarAmount">;

export function isStellarAmount(value: string): value is StellarAmount {
  if (!isDecimalIntegerString(value, true)) {
    return false;
  }
  const parsed = BigInt(value);
  return parsed >= I128_MIN && parsed <= I128_MAX;
}

export function toStellarAmount(value: string): StellarAmount {
  if (!isStellarAmount(value)) {
    throw new TypeError(`Not a valid i128 amount: ${value}`);
  }
  return value;
}
