import { Buffer } from "buffer";
import { Account, Address, Keypair } from "@stellar/stellar-sdk";
import { Client as CareAgreementBindingsClient } from "@carefund/contract-bindings-care-agreement";
import {
  toAgreementId,
  toCommitmentHash,
  toDurationSeconds,
  toLedgerTimestamp,
  toStellarAddress,
  toStellarAmount,
} from "@carefund/types";
import { describe, expect, it, vi } from "vitest";
import { validateStellarClientConfig } from "../config.js";
import type { AccountLookupClient } from "../rpc.js";
import {
  attestCareTransaction,
  cancelAgreementTransaction,
  createAgreementTransaction,
  expireAgreementTransaction,
  fundAgreementTransaction,
  openDisputeTransaction,
  resolveDisputeTransaction,
  settleAgreementTransaction,
} from "./builders.js";

// These tests exercise this SDK's own transaction-building/arg-encoding
// logic against the real generated contract spec (a pure, local
// computation) — no RPC call, network, or blockchain interaction happens
// anywhere in this file. `AccountLookupClient` is a controlled in-memory
// fixture, not a stand-in for live chain state.

function randomContractId(): string {
  return Address.contract(Keypair.random().rawPublicKey()).toString();
}

const CONTRACT_ID = randomContractId();
const SPONSOR = toStellarAddress(Keypair.random().publicKey());
const PROVIDER = toStellarAddress(Keypair.random().publicKey());
const ATTESTER = toStellarAddress(Keypair.random().publicKey());
const ADMIN = toStellarAddress(Keypair.random().publicKey());
const OPENER = SPONSOR;

const config = validateStellarClientConfig({
  network: "CUSTOM",
  rpcUrl: "https://rpc.example.com",
  networkPassphrase: "Standalone Network ; February 2017",
  providerRegistryContractId: randomContractId(),
  careAgreementContractId: CONTRACT_ID,
  settlementAssetContractId: randomContractId(),
});

function fakeAccountLookup(sequence = "100"): AccountLookupClient {
  return { getAccount: vi.fn(async (address: string) => new Account(address, sequence)) };
}

const spec = new CareAgreementBindingsClient({
  contractId: CONTRACT_ID,
  networkPassphrase: config.networkPassphrase,
  rpcUrl: config.rpcUrl,
}).spec;

function invokedCall(tx: Awaited<ReturnType<typeof fundAgreementTransaction>>) {
  const op = tx.operations[0] as unknown as {
    func: { type: string; invokeContract: { functionName: { toString(): string }; args: unknown[] } };
  };
  expect(op.func.type).toBe("hostFunctionTypeInvokeContract");
  const invocation = op.func.invokeContract;
  return { functionName: invocation.functionName.toString(), args: invocation.args };
}

function argsXdr(scVals: unknown[]): string[] {
  return (scVals as { toXDR(format: "base64"): string }[]).map((v) => v.toXDR("base64"));
}

describe("transaction builders", () => {
  it("createAgreementTransaction invokes create_agreement with exactly the encoded args", async () => {
    const server = fakeAccountLookup();
    const tx = await createAgreementTransaction(config, server, {
      provider: PROVIDER,
      sponsor: SPONSOR,
      attester: ATTESTER,
      patientRefCommitment: toCommitmentHash("a".repeat(64)),
      serviceCommitment: toCommitmentHash("b".repeat(64)),
      fundingAmount: toStellarAmount("1000000"),
      settlementAmount: toStellarAmount("900000"),
      fundingDeadline: toLedgerTimestamp("1700000000"),
      careDeadline: toLedgerTimestamp("1700100000"),
      disputeWindowSecs: toDurationSeconds("86400"),
      sourcePublicKey: PROVIDER,
      timeoutSeconds: 30,
    });

    const expected = spec.funcArgsToScVals("create_agreement", {
      provider: PROVIDER,
      sponsor: SPONSOR,
      attester: ATTESTER,
      patient_ref_commitment: Buffer.from("a".repeat(64), "hex"),
      service_commitment: Buffer.from("b".repeat(64), "hex"),
      funding_amount: 1_000_000n,
      settlement_amount: 900_000n,
      funding_deadline: 1_700_000_000n,
      care_deadline: 1_700_100_000n,
      dispute_window_secs: 86_400n,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("create_agreement");
    expect(argsXdr(args)).toEqual(argsXdr(expected));
    expect(server.getAccount).toHaveBeenCalledWith(PROVIDER);
    expect(tx.source).toBe(PROVIDER);
  });

  it("fundAgreementTransaction invokes fund with agreement_id and sponsor sourced from sponsor's account", async () => {
    const server = fakeAccountLookup();
    const tx = await fundAgreementTransaction(config, server, {
      agreementId: toAgreementId("42"),
      sponsor: SPONSOR,
      sourcePublicKey: SPONSOR,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("fund");
    expect(argsXdr(args)).toEqual(argsXdr(spec.funcArgsToScVals("fund", { agreement_id: 42n, sponsor: SPONSOR })));
    expect(server.getAccount).toHaveBeenCalledWith(SPONSOR);
  });

  it("cancelAgreementTransaction invokes cancel authorized by provider", async () => {
    const server = fakeAccountLookup();
    const tx = await cancelAgreementTransaction(config, server, {
      agreementId: toAgreementId("7"),
      provider: PROVIDER,
      sourcePublicKey: PROVIDER,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("cancel");
    expect(argsXdr(args)).toEqual(argsXdr(spec.funcArgsToScVals("cancel", { agreement_id: 7n, provider: PROVIDER })));
  });

  it("attestCareTransaction invokes attest_care with the commitment as bytes", async () => {
    const server = fakeAccountLookup();
    const commitment = toCommitmentHash("c".repeat(64));
    const tx = await attestCareTransaction(config, server, {
      agreementId: toAgreementId("7"),
      attester: ATTESTER,
      attestationCommitment: commitment,
      sourcePublicKey: ATTESTER,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("attest_care");
    expect(argsXdr(args)).toEqual(
      argsXdr(
        spec.funcArgsToScVals("attest_care", {
          agreement_id: 7n,
          attester: ATTESTER,
          attestation_commitment: Buffer.from("c".repeat(64), "hex"),
        }),
      ),
    );
  });

  it("openDisputeTransaction invokes open_dispute with the opener", async () => {
    const server = fakeAccountLookup();
    const tx = await openDisputeTransaction(config, server, {
      agreementId: toAgreementId("7"),
      opener: OPENER,
      sourcePublicKey: OPENER,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("open_dispute");
    expect(argsXdr(args)).toEqual(
      argsXdr(spec.funcArgsToScVals("open_dispute", { agreement_id: 7n, opener: OPENER })),
    );
  });

  it("expireAgreementTransaction invokes expire with only the agreement id, from any funded source account", async () => {
    const server = fakeAccountLookup();
    const caller = toStellarAddress(Keypair.random().publicKey());
    const tx = await expireAgreementTransaction(config, server, {
      agreementId: toAgreementId("7"),
      sourcePublicKey: caller,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("expire");
    expect(argsXdr(args)).toEqual(argsXdr(spec.funcArgsToScVals("expire", { agreement_id: 7n })));
    expect(tx.source).toBe(caller);
  });

  it("settleAgreementTransaction invokes settle authorized by sponsor", async () => {
    const server = fakeAccountLookup();
    const tx = await settleAgreementTransaction(config, server, {
      agreementId: toAgreementId("7"),
      sponsor: SPONSOR,
      sourcePublicKey: SPONSOR,
      timeoutSeconds: 30,
    });

    const { functionName, args } = invokedCall(tx);
    expect(functionName).toBe("settle");
    expect(argsXdr(args)).toEqual(argsXdr(spec.funcArgsToScVals("settle", { agreement_id: 7n, sponsor: SPONSOR })));
  });

  it.each(["Resume", "Settle", "Refund"] as const)(
    "resolveDisputeTransaction invokes resolve_dispute with resolution=%s, authorized by admin",
    async (resolution) => {
      const server = fakeAccountLookup();
      const tx = await resolveDisputeTransaction(config, server, {
        agreementId: toAgreementId("7"),
        resolution,
        adminPublicKey: ADMIN,
        timeoutSeconds: 30,
      });

      const { functionName, args } = invokedCall(tx);
      expect(functionName).toBe("resolve_dispute");
      expect(argsXdr(args)).toEqual(
        argsXdr(
          spec.funcArgsToScVals("resolve_dispute", {
            agreement_id: 7n,
            resolution: { tag: resolution, values: undefined },
          }),
        ),
      );
      expect(server.getAccount).toHaveBeenCalledWith(ADMIN);
      expect(tx.source).toBe(ADMIN);
    },
  );
});
