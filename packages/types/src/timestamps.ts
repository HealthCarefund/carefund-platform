import type { Brand } from "./brand.js";
import { U64_MAX } from "./numeric.js";

/**
 * A u64 unix-seconds value as recorded on-chain (ledger timestamps such as
 * `created_at`, `funding_deadline`, `care_deadline`, `attested_at`,
 * `dispute_opened_at`), represented as a decimal string for the same
 * reason as `AgreementId`/`StellarAmount`.
 */
export type LedgerTimestamp = Brand<string, "LedgerTimestamp">;

const NON_NEGATIVE_INTEGER_PATTERN = /^\d+$/;

export function isLedgerTimestamp(value: string): value is LedgerTimestamp {
  if (!NON_NEGATIVE_INTEGER_PATTERN.test(value)) {
    return false;
  }
  if (value.length > 1 && value.startsWith("0")) {
    return false;
  }
  return BigInt(value) <= U64_MAX;
}

export function toLedgerTimestamp(value: string): LedgerTimestamp {
  if (!isLedgerTimestamp(value)) {
    throw new TypeError(`Not a valid ledger timestamp: ${value}`);
  }
  return value;
}

/**
 * A u64 seconds duration as recorded on-chain (`dispute_window_secs`).
 * Distinct from `LedgerTimestamp`: a duration, not a point in time.
 */
export type DurationSeconds = Brand<string, "DurationSeconds">;

export function isDurationSeconds(value: string): value is DurationSeconds {
  return isLedgerTimestamp(value as LedgerTimestamp);
}

export function toDurationSeconds(value: string): DurationSeconds {
  if (!isDurationSeconds(value)) {
    throw new TypeError(`Not a valid duration in seconds: ${value}`);
  }
  return value;
}

/**
 * An off-chain application timestamp (when the API recorded/observed
 * something), as an ISO-8601 string. Never conflate with a
 * `LedgerTimestamp`: only the ledger value is settlement-authoritative.
 */
export type IsoTimestamp = Brand<string, "IsoTimestamp">;

export function isIsoTimestamp(value: string): value is IsoTimestamp {
  if (Number.isNaN(Date.parse(value))) {
    return false;
  }
  // Date.parse is lenient (accepts non-ISO forms); require the canonical
  // ISO-8601 extended form so round-tripping is exact.
  return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/.test(value);
}

export function toIsoTimestamp(value: string): IsoTimestamp {
  if (!isIsoTimestamp(value)) {
    throw new TypeError(`Not a valid ISO-8601 timestamp: ${value}`);
  }
  return value;
}
