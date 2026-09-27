import { Buffer } from "buffer";
import { Address } from "@stellar/stellar-sdk";
import {
  AssembledTransaction,
  Client as ContractClient,
  ClientOptions as ContractClientOptions,
  MethodOptions,
  Result,
  Spec as ContractSpec,
} from "@stellar/stellar-sdk/contract";
import type {
  u32,
  i32,
  u64,
  i64,
  u128,
  i128,
  u256,
  i256,
  Option,
  Timepoint,
  Duration,
} from "@stellar/stellar-sdk/contract";
export * from "@stellar/stellar-sdk";
export * as contract from "@stellar/stellar-sdk/contract";
export * as rpc from "@stellar/stellar-sdk/rpc";

if (typeof window !== "undefined") {
  //@ts-ignore Buffer exists
  window.Buffer = window.Buffer || Buffer;
}




/**
 * Namespaced keys for contract storage.
 */
export type DataKey = {tag: "Admin", values: void} | {tag: "ProviderRegistry", values: void} | {tag: "SettlementAsset", values: void} | {tag: "NextAgreementId", values: void} | {tag: "Agreement", values: readonly [u64]};


/**
 * On-chain record for a care funding agreement.
 */
export interface Agreement {
  attestation_commitment: Option<Buffer>;
  attested_at: Option<u64>;
  attested_by: Option<string>;
  attester: string;
  care_deadline: u64;
  created_at: u64;
  dispute_opened_at: Option<u64>;
  dispute_opened_by: Option<string>;
  dispute_origin: MaybeDisputeOrigin;
  dispute_window_secs: u64;
  funding_amount: i128;
  funding_deadline: u64;
  /**
 * Opaque commitment — not raw patient data.
 */
patient_ref_commitment: Buffer;
  provider: string;
  /**
 * Opaque commitment — not raw service details.
 */
service_commitment: Buffer;
  settlement_amount: i128;
  sponsor: string;
  state: AgreementState;
}

/**
 * Origin state when a dispute is opened.
 */
export type DisputeOrigin = {tag: "Funded", values: void} | {tag: "CareConfirmed", values: void};

export const AgreementError = {
  1: {message:"AlreadyInitialized"},
  2: {message:"Unauthorized"},
  3: {message:"AgreementNotFound"},
  4: {message:"InvalidState"},
  5: {message:"InvalidAmount"},
  6: {message:"InvalidDeadline"},
  7: {message:"ProviderNotActive"},
  8: {message:"AttesterNotAuthorized"},
  9: {message:"FundingDeadlineNotReached"},
  10: {message:"FundingDeadlineNotPassed"},
  11: {message:"CareDeadlineNotPassed"},
  12: {message:"DisputeWindowActive"},
  13: {message:"DisputeWindowClosed"},
  14: {message:"InvalidResolution"},
  15: {message:"TransferFailed"},
  16: {message:"ArithmeticOverflow"}
}

/**
 * Lifecycle state for care agreements.
 */
export type AgreementState = {tag: "Requested", values: void} | {tag: "Funded", values: void} | {tag: "CareConfirmed", values: void} | {tag: "Disputed", values: void} | {tag: "Cancelled", values: void} | {tag: "Expired", values: void} | {tag: "Refunded", values: void} | {tag: "Settled", values: void};

/**
 * Resolution action for a disputed agreement.
 */
export type DisputeResolution = {tag: "Resume", values: void} | {tag: "Settle", values: void} | {tag: "Refund", values: void};

/**
 * Wrapper for optional dispute origin to work around SDK serialization.
 */
export type MaybeDisputeOrigin = {tag: "None", values: void} | {tag: "Some", values: readonly [DisputeOrigin]};

export interface Client {
  /**
   * Construct and simulate a fund transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Fund an agreement by transferring from sponsor to contract.
   * Requires sponsor authorization.
   * Transitions state from Requested to Funded.
   */
  fund: ({agreement_id, sponsor}: {agreement_id: u64, sponsor: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a cancel transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Cancel an agreement in Requested state.
   * Requires provider authorization.
   * Transitions state from Requested to Cancelled.
   */
  cancel: ({agreement_id, provider}: {agreement_id: u64, provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a expire transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Expire an agreement when care deadline has passed.
   * Callable by anyone after care deadline.
   * Transitions state from Funded or CareConfirmed to Expired.
   */
  expire: ({agreement_id}: {agreement_id: u64}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a settle transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Settle an agreement by transferring settlement amount to provider.
   * Requires sponsor authorization.
   * Transitions state from CareConfirmed to Settled.
   * Transfers settlement_amount from contract to provider.
   */
  settle: ({agreement_id, sponsor}: {agreement_id: u64, sponsor: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a get_admin transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read the admin address.
   */
  get_admin: (options?: MethodOptions) => Promise<AssembledTransaction<Result<string>>>

  /**
   * Construct and simulate a initialize transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Initialize the contract with admin, provider registry, and settlement asset.
   * Callable only once.
   */
  initialize: ({admin, provider_registry, settlement_asset}: {admin: string, provider_registry: string, settlement_asset: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a attest_care transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Attest that care has been provided.
   * Requires attester authorization.
   * Transitions state from Funded to CareConfirmed.
   * Records attestation commitment, attester address, and attestation timestamp.
   */
  attest_care: ({agreement_id, attester, attestation_commitment}: {agreement_id: u64, attester: string, attestation_commitment: Buffer}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a open_dispute transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Open a dispute on an agreement.
   * Requires authorization from sponsor or provider.
   * Transitions state from Funded or CareConfirmed to Disputed.
   * Records dispute origin state, opener address, and dispute timestamp.
   */
  open_dispute: ({agreement_id, opener}: {agreement_id: u64, opener: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a get_agreement transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read an agreement record.
   */
  get_agreement: ({agreement_id}: {agreement_id: u64}, options?: MethodOptions) => Promise<AssembledTransaction<Result<Agreement>>>

  /**
   * Construct and simulate a resolve_dispute transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Resolve a dispute on an agreement.
   * Requires admin authorization.
   * Transitions state from Disputed to appropriate final state based on resolution.
   * Performs necessary token transfers based on resolution action.
   */
  resolve_dispute: ({agreement_id, resolution}: {agreement_id: u64, resolution: DisputeResolution}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a create_agreement transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Create a new care funding agreement.
   * Requires provider authorization.
   */
  create_agreement: ({provider, sponsor, attester, patient_ref_commitment, service_commitment, funding_amount, settlement_amount, funding_deadline, care_deadline, dispute_window_secs}: {provider: string, sponsor: string, attester: string, patient_ref_commitment: Buffer, service_commitment: Buffer, funding_amount: i128, settlement_amount: i128, funding_deadline: u64, care_deadline: u64, dispute_window_secs: u64}, options?: MethodOptions) => Promise<AssembledTransaction<Result<u64>>>

  /**
   * Construct and simulate a get_settlement_asset transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read the settlement asset address.
   */
  get_settlement_asset: (options?: MethodOptions) => Promise<AssembledTransaction<Result<string>>>

  /**
   * Construct and simulate a get_provider_registry transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read the provider registry address.
   */
  get_provider_registry: (options?: MethodOptions) => Promise<AssembledTransaction<Result<string>>>

}
export class Client extends ContractClient {
  static async deploy<T = Client>(
    /** Options for initializing a Client as well as for calling a method, with extras specific to deploying. */
    options: MethodOptions &
      Omit<ContractClientOptions, "contractId"> & {
        /** The hash of the Wasm blob, which must already be installed on-chain. */
        wasmHash: Buffer | string;
        /** Salt used to generate the contract's ID. Passed through to {@link Operation.createCustomContract}. Default: random. */
        salt?: Buffer | Uint8Array;
        /** The format used to decode `wasmHash`, if it's provided as a string. */
        format?: "hex" | "base64";
      }
  ): Promise<AssembledTransaction<T>> {
    return ContractClient.deploy(null, options)
  }
  constructor(public readonly options: ContractClientOptions) {
    super(
      new ContractSpec([ "AAAAAgAAACVOYW1lc3BhY2VkIGtleXMgZm9yIGNvbnRyYWN0IHN0b3JhZ2UuAAAAAAAAAAAAAAdEYXRhS2V5AAAAAAUAAAAAAAAAAAAAAAVBZG1pbgAAAAAAAAAAAAAAAAAAEFByb3ZpZGVyUmVnaXN0cnkAAAAAAAAAAAAAAA9TZXR0bGVtZW50QXNzZXQAAAAAAAAAAAAAAAAPTmV4dEFncmVlbWVudElkAAAAAAEAAAAAAAAACUFncmVlbWVudAAAAAAAAAEAAAAG",
        "AAAAAQAAAC1Pbi1jaGFpbiByZWNvcmQgZm9yIGEgY2FyZSBmdW5kaW5nIGFncmVlbWVudC4AAAAAAAAAAAAACUFncmVlbWVudAAAAAAAABIAAAAAAAAAFmF0dGVzdGF0aW9uX2NvbW1pdG1lbnQAAAAAA+gAAAPuAAAAIAAAAAAAAAALYXR0ZXN0ZWRfYXQAAAAD6AAAAAYAAAAAAAAAC2F0dGVzdGVkX2J5AAAAA+gAAAATAAAAAAAAAAhhdHRlc3RlcgAAABMAAAAAAAAADWNhcmVfZGVhZGxpbmUAAAAAAAAGAAAAAAAAAApjcmVhdGVkX2F0AAAAAAAGAAAAAAAAABFkaXNwdXRlX29wZW5lZF9hdAAAAAAAA+gAAAAGAAAAAAAAABFkaXNwdXRlX29wZW5lZF9ieQAAAAAAA+gAAAATAAAAAAAAAA5kaXNwdXRlX29yaWdpbgAAAAAH0AAAABJNYXliZURpc3B1dGVPcmlnaW4AAAAAAAAAAAATZGlzcHV0ZV93aW5kb3dfc2VjcwAAAAAGAAAAAAAAAA5mdW5kaW5nX2Ftb3VudAAAAAAACwAAAAAAAAAQZnVuZGluZ19kZWFkbGluZQAAAAYAAAArT3BhcXVlIGNvbW1pdG1lbnQg4oCUIG5vdCByYXcgcGF0aWVudCBkYXRhLgAAAAAWcGF0aWVudF9yZWZfY29tbWl0bWVudAAAAAAD7gAAACAAAAAAAAAACHByb3ZpZGVyAAAAEwAAAC5PcGFxdWUgY29tbWl0bWVudCDigJQgbm90IHJhdyBzZXJ2aWNlIGRldGFpbHMuAAAAAAASc2VydmljZV9jb21taXRtZW50AAAAAAPuAAAAIAAAAAAAAAARc2V0dGxlbWVudF9hbW91bnQAAAAAAAALAAAAAAAAAAdzcG9uc29yAAAAABMAAAAAAAAABXN0YXRlAAAAAAAH0AAAAA5BZ3JlZW1lbnRTdGF0ZQAA",
        "AAAAAgAAACZPcmlnaW4gc3RhdGUgd2hlbiBhIGRpc3B1dGUgaXMgb3BlbmVkLgAAAAAAAAAAAA1EaXNwdXRlT3JpZ2luAAAAAAAAAgAAAAAAAAAAAAAABkZ1bmRlZAAAAAAAAAAAAAAAAAANQ2FyZUNvbmZpcm1lZAAAAA==",
        "AAAABAAAAAAAAAAAAAAADkFncmVlbWVudEVycm9yAAAAAAAQAAAAAAAAABJBbHJlYWR5SW5pdGlhbGl6ZWQAAAAAAAEAAAAAAAAADFVuYXV0aG9yaXplZAAAAAIAAAAAAAAAEUFncmVlbWVudE5vdEZvdW5kAAAAAAAAAwAAAAAAAAAMSW52YWxpZFN0YXRlAAAABAAAAAAAAAANSW52YWxpZEFtb3VudAAAAAAAAAUAAAAAAAAAD0ludmFsaWREZWFkbGluZQAAAAAGAAAAAAAAABFQcm92aWRlck5vdEFjdGl2ZQAAAAAAAAcAAAAAAAAAFUF0dGVzdGVyTm90QXV0aG9yaXplZAAAAAAAAAgAAAAAAAAAGUZ1bmRpbmdEZWFkbGluZU5vdFJlYWNoZWQAAAAAAAAJAAAAAAAAABhGdW5kaW5nRGVhZGxpbmVOb3RQYXNzZWQAAAAKAAAAAAAAABVDYXJlRGVhZGxpbmVOb3RQYXNzZWQAAAAAAAALAAAAAAAAABNEaXNwdXRlV2luZG93QWN0aXZlAAAAAAwAAAAAAAAAE0Rpc3B1dGVXaW5kb3dDbG9zZWQAAAAADQAAAAAAAAARSW52YWxpZFJlc29sdXRpb24AAAAAAAAOAAAAAAAAAA5UcmFuc2ZlckZhaWxlZAAAAAAADwAAAAAAAAASQXJpdGhtZXRpY092ZXJmbG93AAAAAAAQ",
        "AAAAAgAAACRMaWZlY3ljbGUgc3RhdGUgZm9yIGNhcmUgYWdyZWVtZW50cy4AAAAAAAAADkFncmVlbWVudFN0YXRlAAAAAAAIAAAAAAAAAAAAAAAJUmVxdWVzdGVkAAAAAAAAAAAAAAAAAAAGRnVuZGVkAAAAAAAAAAAAAAAAAA1DYXJlQ29uZmlybWVkAAAAAAAAAAAAAAAAAAAIRGlzcHV0ZWQAAAAAAAAAAAAAAAlDYW5jZWxsZWQAAAAAAAAAAAAAAAAAAAdFeHBpcmVkAAAAAAAAAAAAAAAACFJlZnVuZGVkAAAAAAAAAAAAAAAHU2V0dGxlZAA=",
        "AAAAAgAAACtSZXNvbHV0aW9uIGFjdGlvbiBmb3IgYSBkaXNwdXRlZCBhZ3JlZW1lbnQuAAAAAAAAAAARRGlzcHV0ZVJlc29sdXRpb24AAAAAAAADAAAAAAAAAAAAAAAGUmVzdW1lAAAAAAAAAAAAAAAAAAZTZXR0bGUAAAAAAAAAAAAAAAAABlJlZnVuZAAA",
        "AAAAAgAAAEVXcmFwcGVyIGZvciBvcHRpb25hbCBkaXNwdXRlIG9yaWdpbiB0byB3b3JrIGFyb3VuZCBTREsgc2VyaWFsaXphdGlvbi4AAAAAAAAAAAAAEk1heWJlRGlzcHV0ZU9yaWdpbgAAAAAAAgAAAAAAAAAAAAAABE5vbmUAAAABAAAAAAAAAARTb21lAAAAAQAAB9AAAAANRGlzcHV0ZU9yaWdpbgAAAA==",
        "AAAAAAAAAIdGdW5kIGFuIGFncmVlbWVudCBieSB0cmFuc2ZlcnJpbmcgZnJvbSBzcG9uc29yIHRvIGNvbnRyYWN0LgpSZXF1aXJlcyBzcG9uc29yIGF1dGhvcml6YXRpb24uClRyYW5zaXRpb25zIHN0YXRlIGZyb20gUmVxdWVzdGVkIHRvIEZ1bmRlZC4AAAAABGZ1bmQAAAACAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAAAAAAdzcG9uc29yAAAAABMAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAAHdDYW5jZWwgYW4gYWdyZWVtZW50IGluIFJlcXVlc3RlZCBzdGF0ZS4KUmVxdWlyZXMgcHJvdmlkZXIgYXV0aG9yaXphdGlvbi4KVHJhbnNpdGlvbnMgc3RhdGUgZnJvbSBSZXF1ZXN0ZWQgdG8gQ2FuY2VsbGVkLgAAAAAGY2FuY2VsAAAAAAACAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAAAAAAhwcm92aWRlcgAAABMAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAAJVFeHBpcmUgYW4gYWdyZWVtZW50IHdoZW4gY2FyZSBkZWFkbGluZSBoYXMgcGFzc2VkLgpDYWxsYWJsZSBieSBhbnlvbmUgYWZ0ZXIgY2FyZSBkZWFkbGluZS4KVHJhbnNpdGlvbnMgc3RhdGUgZnJvbSBGdW5kZWQgb3IgQ2FyZUNvbmZpcm1lZCB0byBFeHBpcmVkLgAAAAAAAAZleHBpcmUAAAAAAAEAAAAAAAAADGFncmVlbWVudF9pZAAAAAYAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAAMpTZXR0bGUgYW4gYWdyZWVtZW50IGJ5IHRyYW5zZmVycmluZyBzZXR0bGVtZW50IGFtb3VudCB0byBwcm92aWRlci4KUmVxdWlyZXMgc3BvbnNvciBhdXRob3JpemF0aW9uLgpUcmFuc2l0aW9ucyBzdGF0ZSBmcm9tIENhcmVDb25maXJtZWQgdG8gU2V0dGxlZC4KVHJhbnNmZXJzIHNldHRsZW1lbnRfYW1vdW50IGZyb20gY29udHJhY3QgdG8gcHJvdmlkZXIuAAAAAAAGc2V0dGxlAAAAAAACAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAAAAAAdzcG9uc29yAAAAABMAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAABdSZWFkIHRoZSBhZG1pbiBhZGRyZXNzLgAAAAAJZ2V0X2FkbWluAAAAAAAAAAAAAAEAAAPpAAAAEwAAB9AAAAAOQWdyZWVtZW50RXJyb3IAAA==",
        "AAAAAAAAAGBJbml0aWFsaXplIHRoZSBjb250cmFjdCB3aXRoIGFkbWluLCBwcm92aWRlciByZWdpc3RyeSwgYW5kIHNldHRsZW1lbnQgYXNzZXQuCkNhbGxhYmxlIG9ubHkgb25jZS4AAAAKaW5pdGlhbGl6ZQAAAAAAAwAAAAAAAAAFYWRtaW4AAAAAAAATAAAAAAAAABFwcm92aWRlcl9yZWdpc3RyeQAAAAAAABMAAAAAAAAAEHNldHRsZW1lbnRfYXNzZXQAAAATAAAAAQAAA+kAAAACAAAH0AAAAA5BZ3JlZW1lbnRFcnJvcgAA",
        "AAAAAAAAAMFBdHRlc3QgdGhhdCBjYXJlIGhhcyBiZWVuIHByb3ZpZGVkLgpSZXF1aXJlcyBhdHRlc3RlciBhdXRob3JpemF0aW9uLgpUcmFuc2l0aW9ucyBzdGF0ZSBmcm9tIEZ1bmRlZCB0byBDYXJlQ29uZmlybWVkLgpSZWNvcmRzIGF0dGVzdGF0aW9uIGNvbW1pdG1lbnQsIGF0dGVzdGVyIGFkZHJlc3MsIGFuZCBhdHRlc3RhdGlvbiB0aW1lc3RhbXAuAAAAAAAAC2F0dGVzdF9jYXJlAAAAAAMAAAAAAAAADGFncmVlbWVudF9pZAAAAAYAAAAAAAAACGF0dGVzdGVyAAAAEwAAAAAAAAAWYXR0ZXN0YXRpb25fY29tbWl0bWVudAAAAAAD7gAAACAAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAANFPcGVuIGEgZGlzcHV0ZSBvbiBhbiBhZ3JlZW1lbnQuClJlcXVpcmVzIGF1dGhvcml6YXRpb24gZnJvbSBzcG9uc29yIG9yIHByb3ZpZGVyLgpUcmFuc2l0aW9ucyBzdGF0ZSBmcm9tIEZ1bmRlZCBvciBDYXJlQ29uZmlybWVkIHRvIERpc3B1dGVkLgpSZWNvcmRzIGRpc3B1dGUgb3JpZ2luIHN0YXRlLCBvcGVuZXIgYWRkcmVzcywgYW5kIGRpc3B1dGUgdGltZXN0YW1wLgAAAAAAAAxvcGVuX2Rpc3B1dGUAAAACAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAAAAAAZvcGVuZXIAAAAAABMAAAABAAAD6QAAAAIAAAfQAAAADkFncmVlbWVudEVycm9yAAA=",
        "AAAAAAAAABlSZWFkIGFuIGFncmVlbWVudCByZWNvcmQuAAAAAAAADWdldF9hZ3JlZW1lbnQAAAAAAAABAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAQAAA+kAAAfQAAAACUFncmVlbWVudAAAAAAAB9AAAAAOQWdyZWVtZW50RXJyb3IAAA==",
        "AAAAAAAAAM9SZXNvbHZlIGEgZGlzcHV0ZSBvbiBhbiBhZ3JlZW1lbnQuClJlcXVpcmVzIGFkbWluIGF1dGhvcml6YXRpb24uClRyYW5zaXRpb25zIHN0YXRlIGZyb20gRGlzcHV0ZWQgdG8gYXBwcm9wcmlhdGUgZmluYWwgc3RhdGUgYmFzZWQgb24gcmVzb2x1dGlvbi4KUGVyZm9ybXMgbmVjZXNzYXJ5IHRva2VuIHRyYW5zZmVycyBiYXNlZCBvbiByZXNvbHV0aW9uIGFjdGlvbi4AAAAAD3Jlc29sdmVfZGlzcHV0ZQAAAAACAAAAAAAAAAxhZ3JlZW1lbnRfaWQAAAAGAAAAAAAAAApyZXNvbHV0aW9uAAAAAAfQAAAAEURpc3B1dGVSZXNvbHV0aW9uAAAAAAAAAQAAA+kAAAACAAAH0AAAAA5BZ3JlZW1lbnRFcnJvcgAA",
        "AAAAAAAAAEVDcmVhdGUgYSBuZXcgY2FyZSBmdW5kaW5nIGFncmVlbWVudC4KUmVxdWlyZXMgcHJvdmlkZXIgYXV0aG9yaXphdGlvbi4AAAAAAAAQY3JlYXRlX2FncmVlbWVudAAAAAoAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAAAAAAHc3BvbnNvcgAAAAATAAAAAAAAAAhhdHRlc3RlcgAAABMAAAAAAAAAFnBhdGllbnRfcmVmX2NvbW1pdG1lbnQAAAAAA+4AAAAgAAAAAAAAABJzZXJ2aWNlX2NvbW1pdG1lbnQAAAAAA+4AAAAgAAAAAAAAAA5mdW5kaW5nX2Ftb3VudAAAAAAACwAAAAAAAAARc2V0dGxlbWVudF9hbW91bnQAAAAAAAALAAAAAAAAABBmdW5kaW5nX2RlYWRsaW5lAAAABgAAAAAAAAANY2FyZV9kZWFkbGluZQAAAAAAAAYAAAAAAAAAE2Rpc3B1dGVfd2luZG93X3NlY3MAAAAABgAAAAEAAAPpAAAABgAAB9AAAAAOQWdyZWVtZW50RXJyb3IAAA==",
        "AAAAAAAAACJSZWFkIHRoZSBzZXR0bGVtZW50IGFzc2V0IGFkZHJlc3MuAAAAAAAUZ2V0X3NldHRsZW1lbnRfYXNzZXQAAAAAAAAAAQAAA+kAAAATAAAH0AAAAA5BZ3JlZW1lbnRFcnJvcgAA",
        "AAAAAAAAACNSZWFkIHRoZSBwcm92aWRlciByZWdpc3RyeSBhZGRyZXNzLgAAAAAVZ2V0X3Byb3ZpZGVyX3JlZ2lzdHJ5AAAAAAAAAAAAAAEAAAPpAAAAEwAAB9AAAAAOQWdyZWVtZW50RXJyb3IAAA==" ]),
      options
    )
  }
  public readonly fromJSON = {
    fund: this.txFromJSON<Result<void>>,
        cancel: this.txFromJSON<Result<void>>,
        expire: this.txFromJSON<Result<void>>,
        settle: this.txFromJSON<Result<void>>,
        get_admin: this.txFromJSON<Result<string>>,
        initialize: this.txFromJSON<Result<void>>,
        attest_care: this.txFromJSON<Result<void>>,
        open_dispute: this.txFromJSON<Result<void>>,
        get_agreement: this.txFromJSON<Result<Agreement>>,
        resolve_dispute: this.txFromJSON<Result<void>>,
        create_agreement: this.txFromJSON<Result<u64>>,
        get_settlement_asset: this.txFromJSON<Result<string>>,
        get_provider_registry: this.txFromJSON<Result<string>>
  }
}