import { Client as ProviderRegistryClient } from "@carefund/contract-bindings-provider-registry";
import type {
  ActorStatus as GeneratedActorStatus,
  AttesterRecord as GeneratedAttesterRecord,
  ProviderRecord as GeneratedProviderRecord,
} from "@carefund/contract-bindings-provider-registry";
import { Client as CareAgreementClient } from "@carefund/contract-bindings-care-agreement";
import type {
  Agreement as GeneratedAgreement,
  AgreementState as GeneratedAgreementState,
  MaybeDisputeOrigin as GeneratedMaybeDisputeOrigin,
} from "@carefund/contract-bindings-care-agreement";
import type {
  ActorStatus,
  AgreementErrorCode,
  AgreementId,
  AgreementState,
  DisputeOrigin,
  OnChainAgreement,
  OnChainAttesterRecord,
  OnChainProviderRecord,
  StellarAddress,
} from "@carefund/types";
import {
  toAgreementId,
  toCommitmentHash,
  toDurationSeconds,
  toLedgerTimestamp,
  toStellarAddress,
  toStellarAmount,
} from "@carefund/types";
import type { StellarClientConfig } from "./config.js";
import { ContractCallError } from "./errors.js";

function isCustomHttp(config: StellarClientConfig): boolean {
  return config.network === "CUSTOM" && new URL(config.rpcUrl).protocol === "http:";
}

/**
 * A read-only generated client: no `publicKey`/`signTransaction` is
 * supplied, so this can only ever simulate — it can never build a
 * transaction that would need a signature.
 */
export function createProviderRegistryReader(config: StellarClientConfig): ProviderRegistryClient {
  return new ProviderRegistryClient({
    contractId: config.providerRegistryContractId,
    networkPassphrase: config.networkPassphrase,
    rpcUrl: config.rpcUrl,
    allowHttp: isCustomHttp(config),
  });
}

export function createCareAgreementReader(config: StellarClientConfig): CareAgreementClient {
  return new CareAgreementClient({
    contractId: config.careAgreementContractId,
    networkPassphrase: config.networkPassphrase,
    rpcUrl: config.rpcUrl,
    allowHttp: isCustomHttp(config),
  });
}

/**
 * The mapping functions below (`map*`) are pure and exported specifically
 * so they can be unit-tested against constructed fixtures without any RPC
 * access — this is where the risk of a wrong on-chain <-> domain-type
 * conversion actually lives, not in the thin network-calling functions
 * that follow them.
 */

export function mapActorStatus(status: GeneratedActorStatus): ActorStatus {
  return status.tag;
}

export function mapProviderRecord(record: GeneratedProviderRecord): OnChainProviderRecord {
  return {
    status: mapActorStatus(record.status),
    providerRef: toCommitmentHash(record.provider_ref.toString("hex")),
  };
}

export function mapAttesterRecord(record: GeneratedAttesterRecord): OnChainAttesterRecord {
  return {
    provider: toStellarAddress(record.provider),
    status: mapActorStatus(record.status),
    credentialRef: toCommitmentHash(record.credential_ref.toString("hex")),
  };
}

export function mapAgreementState(state: GeneratedAgreementState): AgreementState {
  return state.tag;
}

export function mapDisputeOrigin(maybe: GeneratedMaybeDisputeOrigin): DisputeOrigin | undefined {
  return maybe.tag === "Some" ? maybe.values[0].tag : undefined;
}

export function mapAgreement(agreementId: AgreementId, agreement: GeneratedAgreement): OnChainAgreement {
  return {
    agreementId,
    sponsor: toStellarAddress(agreement.sponsor),
    provider: toStellarAddress(agreement.provider),
    attester: toStellarAddress(agreement.attester),
    patientRefCommitment: toCommitmentHash(agreement.patient_ref_commitment.toString("hex")),
    serviceCommitment: toCommitmentHash(agreement.service_commitment.toString("hex")),
    fundingAmount: toStellarAmount(agreement.funding_amount.toString()),
    settlementAmount: toStellarAmount(agreement.settlement_amount.toString()),
    fundingDeadline: toLedgerTimestamp(agreement.funding_deadline.toString()),
    careDeadline: toLedgerTimestamp(agreement.care_deadline.toString()),
    disputeWindowSecs: toDurationSeconds(agreement.dispute_window_secs.toString()),
    state: mapAgreementState(agreement.state),
    attestationCommitment:
      agreement.attestation_commitment !== undefined
        ? toCommitmentHash(agreement.attestation_commitment.toString("hex"))
        : undefined,
    attestedBy: agreement.attested_by !== undefined ? toStellarAddress(agreement.attested_by) : undefined,
    attestedAt: agreement.attested_at !== undefined ? toLedgerTimestamp(agreement.attested_at.toString()) : undefined,
    disputeOrigin: mapDisputeOrigin(agreement.dispute_origin),
    disputeOpenedBy:
      agreement.dispute_opened_by !== undefined ? toStellarAddress(agreement.dispute_opened_by) : undefined,
    disputeOpenedAt:
      agreement.dispute_opened_at !== undefined ? toLedgerTimestamp(agreement.dispute_opened_at.toString()) : undefined,
    createdAt: toLedgerTimestamp(agreement.created_at.toString()),
  };
}

export async function getProvider(
  config: StellarClientConfig,
  provider: StellarAddress,
): Promise<OnChainProviderRecord | undefined> {
  const client = createProviderRegistryReader(config);
  const tx = await client.get_provider({ provider });
  return tx.result !== undefined ? mapProviderRecord(tx.result) : undefined;
}

export async function getAttester(
  config: StellarClientConfig,
  attester: StellarAddress,
): Promise<OnChainAttesterRecord | undefined> {
  const client = createProviderRegistryReader(config);
  const tx = await client.get_attester({ attester });
  return tx.result !== undefined ? mapAttesterRecord(tx.result) : undefined;
}

export async function isProviderActive(config: StellarClientConfig, provider: StellarAddress): Promise<boolean> {
  const client = createProviderRegistryReader(config);
  const tx = await client.is_provider_active({ provider });
  return tx.result;
}

/** Mirrors the contract's `check_attester`: attester Active, provider Active, and bound to it. */
export async function isAttesterAuthorizedForProvider(
  config: StellarClientConfig,
  attester: StellarAddress,
  provider: StellarAddress,
): Promise<boolean> {
  const client = createProviderRegistryReader(config);
  const tx = await client.check_attester({ attester, provider });
  return tx.result;
}

export async function getAgreement(config: StellarClientConfig, agreementId: AgreementId): Promise<OnChainAgreement> {
  const client = createCareAgreementReader(config);
  const tx = await client.get_agreement({ agreement_id: BigInt(agreementId) });
  if (tx.result.isErr()) {
    throw new ContractCallError("care-agreement", tx.result.unwrapErr().message as AgreementErrorCode);
  }
  return mapAgreement(toAgreementId(agreementId), tx.result.unwrap());
}
