import { Buffer } from "buffer";
import type { Transaction } from "@stellar/stellar-sdk";
import { Client as CareAgreementBindingsClient } from "@carefund/contract-bindings-care-agreement";
import type {
  AgreementId,
  CommitmentHash,
  DisputeResolution,
  StellarAddress,
  StellarAmount,
  LedgerTimestamp,
  DurationSeconds,
} from "@carefund/types";
import type { StellarClientConfig } from "../config.js";
import { createCareFundContracts } from "../contracts.js";
import type { AccountLookupClient } from "../rpc.js";
import { buildContractCallTransaction } from "./build.js";

/**
 * The generated `Client`'s only job here is exposing `.spec` so contract
 * arguments can be encoded exactly as the contract declares them
 * (`funcArgsToScVals`) — no network call happens building this client, and
 * it is never used to sign or submit anything. Actual building, signing,
 * and submission go through this SDK's own transaction pipeline
 * (`build.ts`/`simulate.ts`/`sign.ts`/`submit.ts`/`confirm.ts`).
 */
function careAgreementSpecOnly(config: StellarClientConfig): CareAgreementBindingsClient {
  return new CareAgreementBindingsClient({
    contractId: config.careAgreementContractId,
    networkPassphrase: config.networkPassphrase,
    rpcUrl: config.rpcUrl,
  });
}

function commitmentToBuffer(hash: CommitmentHash): Buffer {
  return Buffer.from(hash, "hex");
}

interface BaseCallOptions {
  /** The account that will pay the fee and, ordinarily, that must authorize the call. */
  readonly sourcePublicKey: StellarAddress;
  readonly timeoutSeconds: number;
}

async function buildCall(
  config: StellarClientConfig,
  server: AccountLookupClient,
  method: string,
  args: Record<string, unknown>,
  options: BaseCallOptions,
): Promise<Transaction> {
  const client = careAgreementSpecOnly(config);
  const scArgs = client.spec.funcArgsToScVals(method, args);
  const sourceAccount = await server.getAccount(options.sourcePublicKey);
  const { careAgreement } = createCareFundContracts(config);

  return buildContractCallTransaction(config, {
    contract: careAgreement,
    method,
    args: scArgs,
    sourceAccount,
    timeoutSeconds: options.timeoutSeconds,
  });
}

export interface CreateAgreementParams extends BaseCallOptions {
  readonly provider: StellarAddress;
  readonly sponsor: StellarAddress;
  readonly attester: StellarAddress;
  readonly patientRefCommitment: CommitmentHash;
  readonly serviceCommitment: CommitmentHash;
  readonly fundingAmount: StellarAmount;
  readonly settlementAmount: StellarAmount;
  readonly fundingDeadline: LedgerTimestamp;
  readonly careDeadline: LedgerTimestamp;
  readonly disputeWindowSecs: DurationSeconds;
}

/** Provider-authorized: `sourcePublicKey` must be `provider`. */
export function createAgreementTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: CreateAgreementParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "create_agreement",
    {
      provider: params.provider,
      sponsor: params.sponsor,
      attester: params.attester,
      patient_ref_commitment: commitmentToBuffer(params.patientRefCommitment),
      service_commitment: commitmentToBuffer(params.serviceCommitment),
      funding_amount: BigInt(params.fundingAmount),
      settlement_amount: BigInt(params.settlementAmount),
      funding_deadline: BigInt(params.fundingDeadline),
      care_deadline: BigInt(params.careDeadline),
      dispute_window_secs: BigInt(params.disputeWindowSecs),
    },
    params,
  );
}

export interface FundAgreementParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
  readonly sponsor: StellarAddress;
}

/** Sponsor-authorized: `sourcePublicKey` must be `sponsor`. */
export function fundAgreementTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: FundAgreementParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "fund",
    { agreement_id: BigInt(params.agreementId), sponsor: params.sponsor },
    params,
  );
}

export interface CancelAgreementParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
  readonly provider: StellarAddress;
}

/** Provider-authorized: `sourcePublicKey` must be `provider`. */
export function cancelAgreementTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: CancelAgreementParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "cancel",
    { agreement_id: BigInt(params.agreementId), provider: params.provider },
    params,
  );
}

export interface AttestCareParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
  readonly attester: StellarAddress;
  readonly attestationCommitment: CommitmentHash;
}

/** Attester-authorized: `sourcePublicKey` must be `attester`. */
export function attestCareTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: AttestCareParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "attest_care",
    {
      agreement_id: BigInt(params.agreementId),
      attester: params.attester,
      attestation_commitment: commitmentToBuffer(params.attestationCommitment),
    },
    params,
  );
}

export interface OpenDisputeParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
  /** The sponsor or provider opening the dispute. */
  readonly opener: StellarAddress;
}

/** Sponsor- or provider-authorized: `sourcePublicKey` must be `opener`. */
export function openDisputeTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: OpenDisputeParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "open_dispute",
    { agreement_id: BigInt(params.agreementId), opener: params.opener },
    params,
  );
}

export interface ExpireAgreementParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
}

/** Permissionless on-chain (no `require_auth`), but still needs a fee-paying source account. */
export function expireAgreementTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: ExpireAgreementParams,
): Promise<Transaction> {
  return buildCall(config, server, "expire", { agreement_id: BigInt(params.agreementId) }, params);
}

export interface SettleAgreementParams extends BaseCallOptions {
  readonly agreementId: AgreementId;
}

/** Permissionless on-chain (no `require_auth`), but still needs a fee-paying source account. */
export function settleAgreementTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: SettleAgreementParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "settle",
    { agreement_id: BigInt(params.agreementId) },
    params,
  );
}

export interface ResolveDisputeParams {
  readonly agreementId: AgreementId;
  readonly resolution: DisputeResolution;
  /** The contract admin — the only account authorized to resolve a dispute; also the transaction source. */
  readonly adminPublicKey: StellarAddress;
  readonly timeoutSeconds: number;
}

/** Admin-authorized: the transaction source is always `adminPublicKey`. */
export function resolveDisputeTransaction(
  config: StellarClientConfig,
  server: AccountLookupClient,
  params: ResolveDisputeParams,
): Promise<Transaction> {
  return buildCall(
    config,
    server,
    "resolve_dispute",
    {
      agreement_id: BigInt(params.agreementId),
      resolution: { tag: params.resolution, values: undefined },
    },
    { sourcePublicKey: params.adminPublicKey, timeoutSeconds: params.timeoutSeconds },
  );
}
