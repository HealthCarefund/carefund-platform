import type { IsoTimestamp } from "./timestamps.js";

/**
 * Response metadata for an idempotent write endpoint (e.g. submitting a
 * transaction). `replayed: true` means the client's `idempotencyKey` had
 * already been seen and the original response is being returned, not a
 * new attempt.
 */
export interface IdempotencyMetadata {
  readonly idempotencyKey: string;
  readonly replayed: boolean;
  readonly recordedAt: IsoTimestamp;
}
