import type { Brand } from "./brand.js";

/**
 * A Stellar strkey address: either a classic account (G...) or a Soroban
 * contract (C...). 56 characters, base32 (RFC 4648, no padding).
 */
export type StellarAddress = Brand<string, "StellarAddress">;

const STELLAR_ADDRESS_PATTERN = /^[GC][A-Z2-7]{55}$/;

export function isStellarAddress(value: string): value is StellarAddress {
  return STELLAR_ADDRESS_PATTERN.test(value);
}

export function toStellarAddress(value: string): StellarAddress {
  if (!isStellarAddress(value)) {
    throw new TypeError(`Not a valid Stellar address: ${value}`);
  }
  return value;
}

/**
 * A 32-byte opaque commitment (e.g. `patient_ref_commitment`,
 * `service_commitment`, `attestation_commitment`, provider/attester
 * `*_ref`). These are hashes recorded on-chain, never raw clinical or
 * identifying data — see the provider-registry and care-agreement
 * contract source.
 */
export type CommitmentHash = Brand<string, "CommitmentHash">;

const HEX_32_BYTES_PATTERN = /^[0-9a-f]{64}$/;

export function isCommitmentHash(value: string): value is CommitmentHash {
  return HEX_32_BYTES_PATTERN.test(value);
}

export function toCommitmentHash(value: string): CommitmentHash {
  if (!isCommitmentHash(value)) {
    throw new TypeError(`Not a valid 32-byte commitment hash: ${value}`);
  }
  return value;
}

/** A Stellar transaction hash: 32 bytes, lowercase hex. */
export type TransactionHash = Brand<string, "TransactionHash">;

export function isTransactionHash(value: string): value is TransactionHash {
  return HEX_32_BYTES_PATTERN.test(value);
}

export function toTransactionHash(value: string): TransactionHash {
  if (!isTransactionHash(value)) {
    throw new TypeError(`Not a valid transaction hash: ${value}`);
  }
  return value;
}
