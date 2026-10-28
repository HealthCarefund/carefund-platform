import { Buffer } from "buffer";
import {
  Account,
  Address,
  Asset,
  Keypair,
  nativeToScVal,
  Operation,
  TransactionBuilder,
  xdr,
  type Transaction,
} from "@stellar/stellar-sdk";
import { Client as CareAgreementBindingsClient } from "@carefund/contract-bindings-care-agreement";
import { describe, expect, it } from "vitest";
import { validateStellarClientConfig } from "../config.js";
import { TransactionValidationError } from "../errors.js";
import {
  validateAgreementLifecycleTransaction,
  validateCreateAgreementTransaction,
  type AgreementLifecycleCall,
} from "./validate.js";

// Every transaction built here goes through the real `TransactionBuilder` /
// `Operation` / `xdr` classes from the pinned `@stellar/stellar-sdk` — the
// same ones production code uses — so these are genuine, structurally
// valid envelopes and Soroban authorization entries, not loosely-typed
// stand-ins. No network call, RPC server, or real signature happens
// anywhere in this file; "signatures" on authorization entries are
// placeholder `ScVal`s, which is enough to exercise this validator (it
// checks structure and addressing, not cryptographic validity — that is
// the network's job once submitted).

function randomContractId(): string {
  return Address.contract(Keypair.random().rawPublicKey()).toString();
}

const NETWORK_PASSPHRASE = "Standalone Network ; February 2017";
const CARE_AGREEMENT_CONTRACT_ID = randomContractId();
const SETTLEMENT_ASSET_CONTRACT_ID = randomContractId();
const OTHER_CONTRACT_ID = randomContractId();

const config = validateStellarClientConfig({
  network: "CUSTOM",
  rpcUrl: "https://rpc.example.com",
  networkPassphrase: NETWORK_PASSPHRASE,
  providerRegistryContractId: randomContractId(),
  careAgreementContractId: CARE_AGREEMENT_CONTRACT_ID,
  settlementAssetContractId: SETTLEMENT_ASSET_CONTRACT_ID,
});

const spec = new CareAgreementBindingsClient({
  contractId: CARE_AGREEMENT_CONTRACT_ID,
  networkPassphrase: NETWORK_PASSPHRASE,
  rpcUrl: config.rpcUrl,
}).spec;

const SPONSOR = Keypair.random().publicKey();
const PROVIDER = Keypair.random().publicKey();
const ATTESTER = Keypair.random().publicKey();
const ADMIN = Keypair.random().publicKey();
const AGREEMENT_ID = "7";
const COMMITMENT = "c".repeat(64);

interface BuildOpts {
  readonly source: string;
  readonly contractId?: string;
  readonly method: string;
  readonly args: unknown[]; // positional ScVal-encodable record values, encoded via the real spec below
  readonly argsRecord: Record<string, unknown>;
  readonly auth?: xdr.SorobanAuthorizationEntry[];
  readonly extraOperations?: xdr.Operation[];
  readonly timeoutSeconds?: number;
  readonly fee?: string;
}

function buildTransaction(opts: BuildOpts): Transaction {
  const scArgs = spec.funcArgsToScVals(opts.method, opts.argsRecord);
  const op = Operation.invokeContractFunction({
    contract: opts.contractId ?? CARE_AGREEMENT_CONTRACT_ID,
    function: opts.method,
    args: scArgs,
    auth: opts.auth,
  });
  const account = new Account(opts.source, "100");
  const builder = new TransactionBuilder(account, {
    fee: opts.fee ?? "10000",
    networkPassphrase: NETWORK_PASSPHRASE,
  }).addOperation(op);
  for (const extra of opts.extraOperations ?? []) {
    builder.addOperation(extra);
  }
  return builder.setTimeout(opts.timeoutSeconds ?? 30).build();
}

/** A real, structurally valid Soroban authorization entry naming `signer`, authorizing exactly the given invocation. Signature is a placeholder — structure only, as this validator checks. */
function authEntry(signer: string, contractId: string, method: string, argsRecord: Record<string, unknown>): xdr.SorobanAuthorizationEntry {
  const scArgs = spec.funcArgsToScVals(method, argsRecord);
  return new xdr.SorobanAuthorizationEntry({
    credentials: xdr.SorobanCredentials.sorobanCredentialsAddress(
      new xdr.SorobanAddressCredentials({
        address: Address.fromString(signer).toScAddress(),
        nonce: 123n,
        signatureExpirationLedger: 1000,
        signature: xdr.ScVal.scvVoid(),
      }),
    ),
    rootInvocation: new xdr.SorobanAuthorizedInvocation({
      function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
        new xdr.InvokeContractArgs({
          contractAddress: Address.fromString(contractId).toScAddress(),
          functionName: method,
          args: scArgs,
        }),
      ),
      subInvocations: [],
    }),
  });
}

const FUNDING_AMOUNT = "1000000";

function fundCall(overrides: Partial<AgreementLifecycleCall> = {}): AgreementLifecycleCall {
  return {
    operation: "fund",
    agreementId: AGREEMENT_ID,
    sourcePublicKey: SPONSOR,
    expectedFundingAmount: FUNDING_AMOUNT,
    ...overrides,
  };
}

/**
 * `fund`'s real, complete authorization shape: the sponsor's root
 * authorization for `fund(agreementId, sponsor)`, with exactly one nested
 * sub-invocation — the settlement asset's own `transfer(sponsor,
 * careAgreementContract, amount)` — matching exactly what
 * `contracts/care-agreement/src/lib.rs::fund` actually calls internally.
 */
function fundAuthEntry(signer: string, agreementId: bigint, amount: bigint): xdr.SorobanAuthorizationEntry {
  const rootArgs = spec.funcArgsToScVals("fund", { agreement_id: agreementId, sponsor: signer });
  const transferArgs = [
    nativeToScVal(signer, { type: "address" }),
    nativeToScVal(CARE_AGREEMENT_CONTRACT_ID, { type: "address" }),
    nativeToScVal(amount, { type: "i128" }),
  ];
  return new xdr.SorobanAuthorizationEntry({
    credentials: xdr.SorobanCredentials.sorobanCredentialsAddress(
      new xdr.SorobanAddressCredentials({
        address: Address.fromString(signer).toScAddress(),
        nonce: 1n,
        signatureExpirationLedger: 1000,
        signature: xdr.ScVal.scvVoid(),
      }),
    ),
    rootInvocation: new xdr.SorobanAuthorizedInvocation({
      function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
        new xdr.InvokeContractArgs({
          contractAddress: Address.fromString(CARE_AGREEMENT_CONTRACT_ID).toScAddress(),
          functionName: "fund",
          args: rootArgs,
        }),
      ),
      subInvocations: [
        new xdr.SorobanAuthorizedInvocation({
          function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
            new xdr.InvokeContractArgs({
              contractAddress: Address.fromString(SETTLEMENT_ASSET_CONTRACT_ID).toScAddress(),
              functionName: "transfer",
              args: transferArgs,
            }),
          ),
          subInvocations: [],
        }),
      ],
    }),
  });
}

describe("validateAgreementLifecycleTransaction — legitimate calls accepted", () => {
  it("accepts a correct fund transaction with its real nested escrow-transfer authorization", () => {
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
      auth: [fundAuthEntry(SPONSOR, 7n, BigInt(FUNDING_AMOUNT))],
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).not.toThrow();
  });

  it("accepts a correct cancel transaction", () => {
    const tx = buildTransaction({
      source: PROVIDER,
      method: "cancel",
      args: [],
      argsRecord: { agreement_id: 7n, provider: PROVIDER },
      auth: [authEntry(PROVIDER, CARE_AGREEMENT_CONTRACT_ID, "cancel", { agreement_id: 7n, provider: PROVIDER })],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "cancel", sourcePublicKey: PROVIDER })),
    ).not.toThrow();
  });

  it("accepts a correct attest_care transaction", () => {
    const argsRecord = { agreement_id: 7n, attester: ATTESTER, attestation_commitment: Buffer.from(COMMITMENT, "hex") };
    const tx = buildTransaction({
      source: ATTESTER,
      method: "attest_care",
      args: [],
      argsRecord,
      auth: [authEntry(ATTESTER, CARE_AGREEMENT_CONTRACT_ID, "attest_care", argsRecord)],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(
        config,
        tx,
        NETWORK_PASSPHRASE,
        fundCall({ operation: "attest_care", sourcePublicKey: ATTESTER, attestationCommitment: COMMITMENT }),
      ),
    ).not.toThrow();
  });

  it("accepts a correct open_dispute transaction", () => {
    const tx = buildTransaction({
      source: SPONSOR,
      method: "open_dispute",
      args: [],
      argsRecord: { agreement_id: 7n, opener: SPONSOR },
      auth: [authEntry(SPONSOR, CARE_AGREEMENT_CONTRACT_ID, "open_dispute", { agreement_id: 7n, opener: SPONSOR })],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "open_dispute" })),
    ).not.toThrow();
  });

  it("accepts a correct expire transaction with no authorization entries", () => {
    const caller = Keypair.random().publicKey();
    const tx = buildTransaction({ source: caller, method: "expire", args: [], argsRecord: { agreement_id: 7n } });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "expire", sourcePublicKey: caller })),
    ).not.toThrow();
  });

  it("accepts a correct settle transaction with no authorization entries", () => {
    const caller = Keypair.random().publicKey();
    const tx = buildTransaction({ source: caller, method: "settle", args: [], argsRecord: { agreement_id: 7n } });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "settle", sourcePublicKey: caller })),
    ).not.toThrow();
  });

  it.each(["Resume", "Settle", "Refund"] as const)("accepts a correct resolve_dispute transaction (%s)", (resolution) => {
    const argsRecord = { agreement_id: 7n, resolution: { tag: resolution, values: undefined } };
    const tx = buildTransaction({
      source: ADMIN,
      method: "resolve_dispute",
      args: [],
      argsRecord,
      auth: [authEntry(ADMIN, CARE_AGREEMENT_CONTRACT_ID, "resolve_dispute", argsRecord)],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(
        config,
        tx,
        NETWORK_PASSPHRASE,
        fundCall({ operation: "resolve_dispute", sourcePublicKey: ADMIN, resolution }),
      ),
    ).not.toThrow();
  });

  it("accepts a legitimate fund transaction carrying a simulation-style resource-padded fee, with its real nested escrow-transfer authorization", () => {
    const argsRecord = { agreement_id: 7n, sponsor: SPONSOR };
    // A resource-fee-padded fee, well above the trivial base fee but still
    // far under the circuit-breaker — representative of real
    // `prepareTransaction` output.
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord,
      auth: [fundAuthEntry(SPONSOR, 7n, BigInt(FUNDING_AMOUNT))],
      fee: "1234567",
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).not.toThrow();
  });
});

describe("validateAgreementLifecycleTransaction — rejections", () => {
  it("rejects a wrong contract", () => {
    const tx = buildTransaction({
      source: SPONSOR,
      contractId: OTHER_CONTRACT_ID,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a wrong network passphrase", () => {
    const tx = buildTransaction({ source: SPONSOR, method: "fund", args: [], argsRecord: { agreement_id: 7n, sponsor: SPONSOR } });
    expect(() => validateAgreementLifecycleTransaction(config, tx, "Public Global Stellar Network ; September 2015", fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a wrong source account", () => {
    const tx = buildTransaction({ source: ATTESTER, method: "fund", args: [], argsRecord: { agreement_id: 7n, sponsor: SPONSOR } });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a wrong agreement ID", () => {
    const tx = buildTransaction({ source: SPONSOR, method: "fund", args: [], argsRecord: { agreement_id: 999n, sponsor: SPONSOR } });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a wrong method", () => {
    const tx = buildTransaction({ source: SPONSOR, method: "cancel", args: [], argsRecord: { agreement_id: 7n, provider: SPONSOR } });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ sourcePublicKey: SPONSOR })),
    ).toThrow(TransactionValidationError);
  });

  it("rejects a wrong role address (sponsor replaced with an outsider)", () => {
    const outsider = Keypair.random().publicKey();
    const tx = buildTransaction({ source: SPONSOR, method: "fund", args: [], argsRecord: { agreement_id: 7n, sponsor: outsider } });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a wrong attestation commitment", () => {
    const argsRecord = {
      agreement_id: 7n,
      attester: ATTESTER,
      attestation_commitment: Buffer.from("d".repeat(64), "hex"),
    };
    const tx = buildTransaction({ source: ATTESTER, method: "attest_care", args: [], argsRecord });
    expect(() =>
      validateAgreementLifecycleTransaction(
        config,
        tx,
        NETWORK_PASSPHRASE,
        fundCall({ operation: "attest_care", sourcePublicKey: ATTESTER, attestationCommitment: COMMITMENT }),
      ),
    ).toThrow(TransactionValidationError);
  });

  it("rejects a wrong resolution choice", () => {
    const tx = buildTransaction({
      source: ADMIN,
      method: "resolve_dispute",
      args: [],
      argsRecord: { agreement_id: 7n, resolution: { tag: "Refund", values: undefined } },
    });
    expect(() =>
      validateAgreementLifecycleTransaction(
        config,
        tx,
        NETWORK_PASSPHRASE,
        fundCall({ operation: "resolve_dispute", sourcePublicKey: ADMIN, resolution: "Settle" }),
      ),
    ).toThrow(TransactionValidationError);
  });

  it("rejects an unexpected second operation", () => {
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
      extraOperations: [Operation.bumpSequence({ bumpTo: "200" })],
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects an unexpected payment operation smuggled alongside the real call", () => {
    const attacker = Keypair.random().publicKey();
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
      extraOperations: [Operation.payment({ destination: attacker, asset: Asset.native(), amount: "1000" })],
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects settle carrying an authorization entry when none is expected", () => {
    const argsRecord = { agreement_id: 7n };
    const tx = buildTransaction({
      source: SPONSOR,
      method: "settle",
      args: [],
      argsRecord,
      auth: [authEntry(SPONSOR, CARE_AGREEMENT_CONTRACT_ID, "settle", argsRecord)],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "settle", sourcePublicKey: SPONSOR })),
    ).toThrow(TransactionValidationError);
  });

  it("rejects an authorization entry demanding a signature from an unexpected address", () => {
    const outsider = Keypair.random().publicKey();
    const argsRecord = { agreement_id: 7n, sponsor: SPONSOR };
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord,
      auth: [authEntry(outsider, CARE_AGREEMENT_CONTRACT_ID, "fund", argsRecord)],
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects an authorization entry whose invocation tree touches an unrelated contract", () => {
    const argsRecord = { agreement_id: 7n, sponsor: SPONSOR };
    const entry = new xdr.SorobanAuthorizationEntry({
      credentials: xdr.SorobanCredentials.sorobanCredentialsAddress(
        new xdr.SorobanAddressCredentials({
          address: Address.fromString(SPONSOR).toScAddress(),
          nonce: 1n,
          signatureExpirationLedger: 1000,
          signature: xdr.ScVal.scvVoid(),
        }),
      ),
      rootInvocation: new xdr.SorobanAuthorizedInvocation({
        function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
          new xdr.InvokeContractArgs({
            contractAddress: Address.fromString(CARE_AGREEMENT_CONTRACT_ID).toScAddress(),
            functionName: "fund",
            args: spec.funcArgsToScVals("fund", argsRecord),
          }),
        ),
        subInvocations: [
          new xdr.SorobanAuthorizedInvocation({
            function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
              new xdr.InvokeContractArgs({
                contractAddress: Address.fromString(OTHER_CONTRACT_ID).toScAddress(),
                functionName: "revoke_provider",
                args: [],
              }),
            ),
            subInvocations: [],
          }),
        ],
      }),
    });
    const tx = buildTransaction({ source: SPONSOR, method: "fund", args: [], argsRecord, auth: [entry] });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a transaction with no time bounds", () => {
    const built = buildTransaction({ source: SPONSOR, method: "fund", args: [], argsRecord: { agreement_id: 7n, sponsor: SPONSOR } });
    // `TransactionBuilder` requires an explicit timeout (or `TimeoutInfinite`);
    // there is no supported way to build a transaction with no precondition
    // at all. Rebuild the envelope with `cond: precondNone()`, copying every
    // other field from the real built transaction, to exercise the "no time
    // bounds at all" case this validator must still catch.
    const builtEnvelope = built.toEnvelope();
    if (builtEnvelope.type !== "envelopeTypeTx") throw new Error("expected a v1 envelope");
    const innerTx = builtEnvelope.v1.tx;
    const noConditionTx = new xdr.Transaction({
      sourceAccount: innerTx.sourceAccount,
      fee: innerTx.fee,
      seqNum: innerTx.seqNum,
      cond: xdr.Preconditions.precondNone(),
      memo: innerTx.memo,
      operations: innerTx.operations,
      ext: innerTx.ext,
    });
    const envelope = xdr.TransactionEnvelope.envelopeTypeTx(
      new xdr.TransactionV1Envelope({ tx: noConditionTx, signatures: [] }),
    );
    const tx = TransactionBuilder.fromXDR(envelope.toXdr("base64"), NETWORK_PASSPHRASE) as Transaction;
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("rejects a transaction whose fee is absurdly high", () => {
    const tx = buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
      fee: "1000000000", // 100 XLM: under the protocol's uint32 fee ceiling but over this validator's circuit breaker
    });
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });
});

// These 7 scenarios were confirmed, before this validator verified the full
// authorization tree, to be incorrectly ACCEPTED by `validateInvocationTree`
// — it checked only that a node's contract address was on the allowlist
// (care-agreement or settlement asset), never the authorized function or
// its arguments. Every one of the 7 cases below passed validation against
// the pre-fix code; this was confirmed directly against that code (not
// assumed) before the fix was written.
describe("validateAgreementLifecycleTransaction — full authorization-tree verification", () => {
  function fundTx(auth: xdr.SorobanAuthorizationEntry[]) {
    return buildTransaction({
      source: SPONSOR,
      method: "fund",
      args: [],
      argsRecord: { agreement_id: 7n, sponsor: SPONSOR },
      auth,
    });
  }

  function customInvocation(
    contractId: string,
    method: string,
    args: xdr.ScVal[],
    subInvocations: xdr.SorobanAuthorizedInvocation[] = [],
  ): xdr.SorobanAuthorizedInvocation {
    return new xdr.SorobanAuthorizedInvocation({
      function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
        new xdr.InvokeContractArgs({
          contractAddress: Address.fromString(contractId).toScAddress(),
          functionName: method,
          args,
        }),
      ),
      subInvocations,
    });
  }

  function customAuthEntry(signer: string, rootInvocation: xdr.SorobanAuthorizedInvocation): xdr.SorobanAuthorizationEntry {
    return new xdr.SorobanAuthorizationEntry({
      credentials: xdr.SorobanCredentials.sorobanCredentialsAddress(
        new xdr.SorobanAddressCredentials({
          address: Address.fromString(signer).toScAddress(),
          nonce: 1n,
          signatureExpirationLedger: 1000,
          signature: xdr.ScVal.scvVoid(),
        }),
      ),
      rootInvocation,
    });
  }

  it("1: rejects an authorization root for fund(8, sponsor) when the operation itself invokes fund(7, sponsor)", () => {
    const authArgs = spec.funcArgsToScVals("fund", { agreement_id: 8n, sponsor: SPONSOR });
    const tx = fundTx([customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "fund", authArgs))]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("2: rejects an authorization root for a different CareFund method on the same (trusted) contract address", () => {
    const authArgs = spec.funcArgsToScVals("resolve_dispute", {
      agreement_id: 7n,
      resolution: { tag: "Refund", values: undefined },
    });
    const tx = fundTx([
      customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "resolve_dispute", authArgs)),
    ]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("3: rejects a settlement-asset sub-invocation with an unexpected function (approve instead of transfer)", () => {
    const attacker = Keypair.random().publicKey();
    const sub = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "approve", [
      nativeToScVal(SPONSOR, { type: "address" }),
      nativeToScVal(attacker, { type: "address" }),
      nativeToScVal(BigInt(FUNDING_AMOUNT), { type: "i128" }),
    ]);
    const rootArgs = spec.funcArgsToScVals("fund", { agreement_id: 7n, sponsor: SPONSOR });
    const tx = fundTx([
      customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "fund", rootArgs, [sub])),
    ]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("4: rejects a token transfer whose source is not the sponsor", () => {
    const attacker = Keypair.random().publicKey();
    const sub = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "transfer", [
      nativeToScVal(attacker, { type: "address" }),
      nativeToScVal(CARE_AGREEMENT_CONTRACT_ID, { type: "address" }),
      nativeToScVal(BigInt(FUNDING_AMOUNT), { type: "i128" }),
    ]);
    const rootArgs = spec.funcArgsToScVals("fund", { agreement_id: 7n, sponsor: SPONSOR });
    const tx = fundTx([
      customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "fund", rootArgs, [sub])),
    ]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("5: rejects a token transfer whose destination is not the care-agreement contract", () => {
    const attacker = Keypair.random().publicKey();
    const sub = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "transfer", [
      nativeToScVal(SPONSOR, { type: "address" }),
      nativeToScVal(attacker, { type: "address" }),
      nativeToScVal(BigInt(FUNDING_AMOUNT), { type: "i128" }),
    ]);
    const rootArgs = spec.funcArgsToScVals("fund", { agreement_id: 7n, sponsor: SPONSOR });
    const tx = fundTx([
      customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "fund", rootArgs, [sub])),
    ]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("6: rejects a token transfer amount that differs from the independently-established legitimate amount", () => {
    const sub = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "transfer", [
      nativeToScVal(SPONSOR, { type: "address" }),
      nativeToScVal(CARE_AGREEMENT_CONTRACT_ID, { type: "address" }),
      nativeToScVal(BigInt(FUNDING_AMOUNT) * 1000n, { type: "i128" }),
    ]);
    const rootArgs = spec.funcArgsToScVals("fund", { agreement_id: 7n, sponsor: SPONSOR });
    const tx = fundTx([
      customAuthEntry(SPONSOR, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "fund", rootArgs, [sub])),
    ]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("7: rejects an additional, unrelated authorization entry alongside a legitimate fund entry", () => {
    const attacker = Keypair.random().publicKey();
    const legitEntry = fundAuthEntry(SPONSOR, 7n, BigInt(FUNDING_AMOUNT));
    const extraInvocation = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "transfer", [
      nativeToScVal(SPONSOR, { type: "address" }),
      nativeToScVal(attacker, { type: "address" }),
      nativeToScVal(500_000_000n, { type: "i128" }),
    ]);
    const extraEntry = customAuthEntry(SPONSOR, extraInvocation);
    const tx = fundTx([legitEntry, extraEntry]);
    expect(() => validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall())).toThrow(
      TransactionValidationError,
    );
  });

  it("fails closed when fund is validated without an independently-established expectedFundingAmount", () => {
    const tx = fundTx([fundAuthEntry(SPONSOR, 7n, BigInt(FUNDING_AMOUNT))]);
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ expectedFundingAmount: undefined })),
    ).toThrow(TransactionValidationError);
  });

  it("rejects a non-fund operation (cancel) that unexpectedly carries a nested sub-invocation", () => {
    const rootArgs = spec.funcArgsToScVals("cancel", { agreement_id: 7n, provider: PROVIDER });
    const sub = customInvocation(SETTLEMENT_ASSET_CONTRACT_ID, "transfer", [
      nativeToScVal(PROVIDER, { type: "address" }),
      nativeToScVal(CARE_AGREEMENT_CONTRACT_ID, { type: "address" }),
      nativeToScVal(1n, { type: "i128" }),
    ]);
    const tx = buildTransaction({
      source: PROVIDER,
      method: "cancel",
      args: [],
      argsRecord: { agreement_id: 7n, provider: PROVIDER },
      auth: [customAuthEntry(PROVIDER, customInvocation(CARE_AGREEMENT_CONTRACT_ID, "cancel", rootArgs, [sub]))],
    });
    expect(() =>
      validateAgreementLifecycleTransaction(config, tx, NETWORK_PASSPHRASE, fundCall({ operation: "cancel", sourcePublicKey: PROVIDER, expectedFundingAmount: undefined })),
    ).toThrow(TransactionValidationError);
  });
});

describe("validateCreateAgreementTransaction", () => {
  const createArgs = {
    sourcePublicKey: PROVIDER,
    provider: PROVIDER,
    sponsor: SPONSOR,
    attester: ATTESTER,
    patientRefCommitment: "a".repeat(64),
    serviceCommitment: "b".repeat(64),
    fundingAmount: "1000000",
    settlementAmount: "900000",
    fundingDeadline: "1700000000",
    careDeadline: "1700100000",
    disputeWindowSecs: "86400",
  };

  function createArgsRecord() {
    return {
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
    };
  }

  it("accepts a correct create_agreement transaction", () => {
    const tx = buildTransaction({
      source: PROVIDER,
      method: "create_agreement",
      args: [],
      argsRecord: createArgsRecord(),
      auth: [authEntry(PROVIDER, CARE_AGREEMENT_CONTRACT_ID, "create_agreement", createArgsRecord())],
    });
    expect(() => validateCreateAgreementTransaction(config, tx, NETWORK_PASSPHRASE, createArgs)).not.toThrow();
  });

  it("rejects a create_agreement transaction with a tampered funding amount", () => {
    const tamperedArgs = { ...createArgsRecord(), funding_amount: 1n };
    const tx = buildTransaction({ source: PROVIDER, method: "create_agreement", args: [], argsRecord: tamperedArgs });
    expect(() => validateCreateAgreementTransaction(config, tx, NETWORK_PASSPHRASE, createArgs)).toThrow(
      TransactionValidationError,
    );
  });
});
