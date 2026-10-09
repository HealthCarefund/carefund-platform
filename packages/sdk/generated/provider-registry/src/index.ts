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
 * Lifecycle status for providers and attesters.
 */
export type ActorStatus = {tag: "Active", values: void} | {tag: "Suspended", values: void} | {tag: "Revoked", values: void};

export const RegistryError = {
  1: {message:"AlreadyInitialized"},
  2: {message:"Unauthorized"},
  3: {message:"ProviderNotFound"},
  4: {message:"ProviderAlreadyExists"},
  5: {message:"ProviderAlreadySuspended"},
  6: {message:"ProviderAlreadyActive"},
  7: {message:"ProviderAlreadyRevoked"},
  8: {message:"AttesterNotFound"},
  9: {message:"AttesterAlreadyExists"},
  10: {message:"AttesterAlreadySuspended"},
  11: {message:"AttesterAlreadyActive"},
  12: {message:"AttesterAlreadyRevoked"},
  13: {message:"ProviderNotActive"},
  14: {message:"InvalidState"}
}


/**
 * On-chain record for a registered attester.
 */
export interface AttesterRecord {
  /**
 * Opaque commitment — not a raw credential document.
 */
credential_ref: Buffer;
  provider: string;
  status: ActorStatus;
}


/**
 * On-chain record for a registered provider.
 */
export interface ProviderRecord {
  /**
 * Opaque commitment — not a raw document or license.
 */
provider_ref: Buffer;
  status: ActorStatus;
}

export interface Client {
  /**
   * Construct and simulate a get_admin transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read the admin address.
   */
  get_admin: (options?: MethodOptions) => Promise<AssembledTransaction<Result<string>>>

  /**
   * Construct and simulate a initialize transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Initialize the contract with an admin address. Callable only once.
   */
  initialize: ({admin}: {admin: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a get_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read an attester's full record.
   */
  get_attester: ({attester}: {attester: string}, options?: MethodOptions) => Promise<AssembledTransaction<Option<AttesterRecord>>>

  /**
   * Construct and simulate a get_provider transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Read a provider's full record.
   */
  get_provider: ({provider}: {provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<Option<ProviderRecord>>>

  /**
   * Construct and simulate a check_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Check if an attester is Active and bound to an Active provider.
   * Returns true only when: attester exists, is Active, provider exists,
   * is Active, and attester.provider == provider.
   */
  check_attester: ({attester, provider}: {attester: string, provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<boolean>>

  /**
   * Construct and simulate a revoke_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Revoke an attester (terminal operation). Requires admin authorization.
   */
  revoke_attester: ({attester}: {attester: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a revoke_provider transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Revoke a provider (terminal operation). Requires admin authorization.
   */
  revoke_provider: ({provider}: {provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a suspend_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Suspend an active attester. Requires admin authorization.
   */
  suspend_attester: ({attester}: {attester: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a suspend_provider transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Suspend an active provider. Requires admin authorization.
   */
  suspend_provider: ({provider}: {provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a register_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Register a new attester bound to a specific provider. Requires admin authorization.
   */
  register_attester: ({attester, provider, credential_ref}: {attester: string, provider: string, credential_ref: Buffer}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a register_provider transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Register a new provider. Requires admin authorization.
   */
  register_provider: ({provider, provider_ref}: {provider: string, provider_ref: Buffer}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a is_provider_active transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Check if a provider exists and is currently Active.
   */
  is_provider_active: ({provider}: {provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<boolean>>

  /**
   * Construct and simulate a reinstate_attester transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Reinstate a suspended attester. Requires admin authorization.
   */
  reinstate_attester: ({attester}: {attester: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

  /**
   * Construct and simulate a reinstate_provider transaction. Returns an `AssembledTransaction` object which will have a `result` field containing the result of the simulation. If this transaction changes contract state, you will need to call `signAndSend()` on the returned object.
   * Reinstate a suspended provider. Requires admin authorization.
   */
  reinstate_provider: ({provider}: {provider: string}, options?: MethodOptions) => Promise<AssembledTransaction<Result<void>>>

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
      new ContractSpec([ "AAAAAgAAAC1MaWZlY3ljbGUgc3RhdHVzIGZvciBwcm92aWRlcnMgYW5kIGF0dGVzdGVycy4AAAAAAAAAAAAAC0FjdG9yU3RhdHVzAAAAAAMAAAAAAAAAAAAAAAZBY3RpdmUAAAAAAAAAAAAAAAAACVN1c3BlbmRlZAAAAAAAAAAAAAAAAAAAB1Jldm9rZWQA",
        "AAAABAAAAAAAAAAAAAAADVJlZ2lzdHJ5RXJyb3IAAAAAAAAOAAAAAAAAABJBbHJlYWR5SW5pdGlhbGl6ZWQAAAAAAAEAAAAAAAAADFVuYXV0aG9yaXplZAAAAAIAAAAAAAAAEFByb3ZpZGVyTm90Rm91bmQAAAADAAAAAAAAABVQcm92aWRlckFscmVhZHlFeGlzdHMAAAAAAAAEAAAAAAAAABhQcm92aWRlckFscmVhZHlTdXNwZW5kZWQAAAAFAAAAAAAAABVQcm92aWRlckFscmVhZHlBY3RpdmUAAAAAAAAGAAAAAAAAABZQcm92aWRlckFscmVhZHlSZXZva2VkAAAAAAAHAAAAAAAAABBBdHRlc3Rlck5vdEZvdW5kAAAACAAAAAAAAAAVQXR0ZXN0ZXJBbHJlYWR5RXhpc3RzAAAAAAAACQAAAAAAAAAYQXR0ZXN0ZXJBbHJlYWR5U3VzcGVuZGVkAAAACgAAAAAAAAAVQXR0ZXN0ZXJBbHJlYWR5QWN0aXZlAAAAAAAACwAAAAAAAAAWQXR0ZXN0ZXJBbHJlYWR5UmV2b2tlZAAAAAAADAAAAAAAAAARUHJvdmlkZXJOb3RBY3RpdmUAAAAAAAANAAAAAAAAAAxJbnZhbGlkU3RhdGUAAAAO",
        "AAAAAQAAACpPbi1jaGFpbiByZWNvcmQgZm9yIGEgcmVnaXN0ZXJlZCBhdHRlc3Rlci4AAAAAAAAAAAAOQXR0ZXN0ZXJSZWNvcmQAAAAAAAMAAAA0T3BhcXVlIGNvbW1pdG1lbnQg4oCUIG5vdCBhIHJhdyBjcmVkZW50aWFsIGRvY3VtZW50LgAAAA5jcmVkZW50aWFsX3JlZgAAAAAD7gAAACAAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAAAAAAGc3RhdHVzAAAAAAfQAAAAC0FjdG9yU3RhdHVzAA==",
        "AAAAAQAAACpPbi1jaGFpbiByZWNvcmQgZm9yIGEgcmVnaXN0ZXJlZCBwcm92aWRlci4AAAAAAAAAAAAOUHJvdmlkZXJSZWNvcmQAAAAAAAIAAAA0T3BhcXVlIGNvbW1pdG1lbnQg4oCUIG5vdCBhIHJhdyBkb2N1bWVudCBvciBsaWNlbnNlLgAAAAxwcm92aWRlcl9yZWYAAAPuAAAAIAAAAAAAAAAGc3RhdHVzAAAAAAfQAAAAC0FjdG9yU3RhdHVzAA==",
        "AAAAAAAAABdSZWFkIHRoZSBhZG1pbiBhZGRyZXNzLgAAAAAJZ2V0X2FkbWluAAAAAAAAAAAAAAEAAAPpAAAAEwAAB9AAAAANUmVnaXN0cnlFcnJvcgAAAA==",
        "AAAAAAAAAEJJbml0aWFsaXplIHRoZSBjb250cmFjdCB3aXRoIGFuIGFkbWluIGFkZHJlc3MuIENhbGxhYmxlIG9ubHkgb25jZS4AAAAAAAppbml0aWFsaXplAAAAAAABAAAAAAAAAAVhZG1pbgAAAAAAABMAAAABAAAD6QAAAAIAAAfQAAAADVJlZ2lzdHJ5RXJyb3IAAAA=",
        "AAAAAAAAAB9SZWFkIGFuIGF0dGVzdGVyJ3MgZnVsbCByZWNvcmQuAAAAAAxnZXRfYXR0ZXN0ZXIAAAABAAAAAAAAAAhhdHRlc3RlcgAAABMAAAABAAAD6AAAB9AAAAAOQXR0ZXN0ZXJSZWNvcmQAAA==",
        "AAAAAAAAAB5SZWFkIGEgcHJvdmlkZXIncyBmdWxsIHJlY29yZC4AAAAAAAxnZXRfcHJvdmlkZXIAAAABAAAAAAAAAAhwcm92aWRlcgAAABMAAAABAAAD6AAAB9AAAAAOUHJvdmlkZXJSZWNvcmQAAA==",
        "AAAAAAAAALJDaGVjayBpZiBhbiBhdHRlc3RlciBpcyBBY3RpdmUgYW5kIGJvdW5kIHRvIGFuIEFjdGl2ZSBwcm92aWRlci4KUmV0dXJucyB0cnVlIG9ubHkgd2hlbjogYXR0ZXN0ZXIgZXhpc3RzLCBpcyBBY3RpdmUsIHByb3ZpZGVyIGV4aXN0cywKaXMgQWN0aXZlLCBhbmQgYXR0ZXN0ZXIucHJvdmlkZXIgPT0gcHJvdmlkZXIuAAAAAAAOY2hlY2tfYXR0ZXN0ZXIAAAAAAAIAAAAAAAAACGF0dGVzdGVyAAAAEwAAAAAAAAAIcHJvdmlkZXIAAAATAAAAAQAAAAE=",
        "AAAAAAAAAEZSZXZva2UgYW4gYXR0ZXN0ZXIgKHRlcm1pbmFsIG9wZXJhdGlvbikuIFJlcXVpcmVzIGFkbWluIGF1dGhvcml6YXRpb24uAAAAAAAPcmV2b2tlX2F0dGVzdGVyAAAAAAEAAAAAAAAACGF0dGVzdGVyAAAAEwAAAAEAAAPpAAAAAgAAB9AAAAANUmVnaXN0cnlFcnJvcgAAAA==",
        "AAAAAAAAAEVSZXZva2UgYSBwcm92aWRlciAodGVybWluYWwgb3BlcmF0aW9uKS4gUmVxdWlyZXMgYWRtaW4gYXV0aG9yaXphdGlvbi4AAAAAAAAPcmV2b2tlX3Byb3ZpZGVyAAAAAAEAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAEAAAPpAAAAAgAAB9AAAAANUmVnaXN0cnlFcnJvcgAAAA==",
        "AAAAAAAAADlTdXNwZW5kIGFuIGFjdGl2ZSBhdHRlc3Rlci4gUmVxdWlyZXMgYWRtaW4gYXV0aG9yaXphdGlvbi4AAAAAAAAQc3VzcGVuZF9hdHRlc3RlcgAAAAEAAAAAAAAACGF0dGVzdGVyAAAAEwAAAAEAAAPpAAAAAgAAB9AAAAANUmVnaXN0cnlFcnJvcgAAAA==",
        "AAAAAAAAADlTdXNwZW5kIGFuIGFjdGl2ZSBwcm92aWRlci4gUmVxdWlyZXMgYWRtaW4gYXV0aG9yaXphdGlvbi4AAAAAAAAQc3VzcGVuZF9wcm92aWRlcgAAAAEAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAEAAAPpAAAAAgAAB9AAAAANUmVnaXN0cnlFcnJvcgAAAA==",
        "AAAAAAAAAFNSZWdpc3RlciBhIG5ldyBhdHRlc3RlciBib3VuZCB0byBhIHNwZWNpZmljIHByb3ZpZGVyLiBSZXF1aXJlcyBhZG1pbiBhdXRob3JpemF0aW9uLgAAAAARcmVnaXN0ZXJfYXR0ZXN0ZXIAAAAAAAADAAAAAAAAAAhhdHRlc3RlcgAAABMAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAAAAAAOY3JlZGVudGlhbF9yZWYAAAAAA+4AAAAgAAAAAQAAA+kAAAACAAAH0AAAAA1SZWdpc3RyeUVycm9yAAAA",
        "AAAAAAAAADZSZWdpc3RlciBhIG5ldyBwcm92aWRlci4gUmVxdWlyZXMgYWRtaW4gYXV0aG9yaXphdGlvbi4AAAAAABFyZWdpc3Rlcl9wcm92aWRlcgAAAAAAAAIAAAAAAAAACHByb3ZpZGVyAAAAEwAAAAAAAAAMcHJvdmlkZXJfcmVmAAAD7gAAACAAAAABAAAD6QAAAAIAAAfQAAAADVJlZ2lzdHJ5RXJyb3IAAAA=",
        "AAAAAAAAADNDaGVjayBpZiBhIHByb3ZpZGVyIGV4aXN0cyBhbmQgaXMgY3VycmVudGx5IEFjdGl2ZS4AAAAAEmlzX3Byb3ZpZGVyX2FjdGl2ZQAAAAAAAQAAAAAAAAAIcHJvdmlkZXIAAAATAAAAAQAAAAE=",
        "AAAAAAAAAD1SZWluc3RhdGUgYSBzdXNwZW5kZWQgYXR0ZXN0ZXIuIFJlcXVpcmVzIGFkbWluIGF1dGhvcml6YXRpb24uAAAAAAAAEnJlaW5zdGF0ZV9hdHRlc3RlcgAAAAAAAQAAAAAAAAAIYXR0ZXN0ZXIAAAATAAAAAQAAA+kAAAACAAAH0AAAAA1SZWdpc3RyeUVycm9yAAAA",
        "AAAAAAAAAD1SZWluc3RhdGUgYSBzdXNwZW5kZWQgcHJvdmlkZXIuIFJlcXVpcmVzIGFkbWluIGF1dGhvcml6YXRpb24uAAAAAAAAEnJlaW5zdGF0ZV9wcm92aWRlcgAAAAAAAQAAAAAAAAAIcHJvdmlkZXIAAAATAAAAAQAAA+kAAAACAAAH0AAAAA1SZWdpc3RyeUVycm9yAAAA" ]),
      options
    )
  }
  public readonly fromJSON = {
    get_admin: this.txFromJSON<Result<string>>,
        initialize: this.txFromJSON<Result<void>>,
        get_attester: this.txFromJSON<Option<AttesterRecord>>,
        get_provider: this.txFromJSON<Option<ProviderRecord>>,
        check_attester: this.txFromJSON<boolean>,
        revoke_attester: this.txFromJSON<Result<void>>,
        revoke_provider: this.txFromJSON<Result<void>>,
        suspend_attester: this.txFromJSON<Result<void>>,
        suspend_provider: this.txFromJSON<Result<void>>,
        register_attester: this.txFromJSON<Result<void>>,
        register_provider: this.txFromJSON<Result<void>>,
        is_provider_active: this.txFromJSON<boolean>,
        reinstate_attester: this.txFromJSON<Result<void>>,
        reinstate_provider: this.txFromJSON<Result<void>>
  }
}