import type { CommitmentHash, StellarAddress } from "./stellar.js";

/**
 * Mirrors `ActorStatus` in contracts/provider-registry/src/lib.rs exactly.
 * Shared by both providers and attesters.
 */
export type ActorStatus = "Active" | "Suspended" | "Revoked";

/**
 * On-chain provider record. Mirrors `ProviderRecord` in
 * contracts/provider-registry/src/lib.rs. `providerRef` is an opaque
 * commitment, never a raw license/document.
 */
export interface OnChainProviderRecord {
  readonly status: ActorStatus;
  readonly providerRef: CommitmentHash;
}

/**
 * On-chain attester record. Mirrors `AttesterRecord` in
 * contracts/provider-registry/src/lib.rs. `credentialRef` is an opaque
 * commitment, never a raw credential document.
 */
export interface OnChainAttesterRecord {
  readonly provider: StellarAddress;
  readonly status: ActorStatus;
  readonly credentialRef: CommitmentHash;
}
