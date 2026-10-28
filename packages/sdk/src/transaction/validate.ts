import { Buffer } from "buffer";
import { Address, Transaction, xdr } from "@stellar/stellar-sdk";
import { Client as CareAgreementBindingsClient } from "@carefund/contract-bindings-care-agreement";
import type { StellarClientConfig } from "../config.js";
import { TransactionValidationError } from "../errors.js";

/**
 * Independently validates a transaction the API (or this SDK's own
 * client-side builder) prepared, before it is ever handed to a wallet for
 * signing. The browser must not treat a prepared transaction as
 * authoritative just because its XDR parsed successfully: a compromised or
 * misconfigured backend could return a transaction invoking a different
 * contract, function, argument, or source account than the one the user
 * approved, and ask the wallet to sign it anyway.
 *
 * This checks, using the real pinned `@stellar/stellar-sdk` types and the
 * real generated care-agreement contract spec (never raw XDR string
 * comparison for anything structured):
 *
 *   1. Network passphrase matches this app's own configuration — never
 *      trusted merely because it was returned alongside the XDR.
 *   2. Transaction source account matches the expected signer.
 *   3. Exactly one operation.
 *   4. The operation invokes the configured care-agreement contract.
 *   5. The invoked function name matches the requested operation.
 *   6-9. The invocation arguments — agreement ID, role addresses,
 *      attestation commitment, dispute resolution choice — match exactly,
 *      verified by re-encoding the expected arguments through the same
 *      contract spec and comparing the resulting `ScVal`s, not by
 *      comparing arbitrary strings.
 *   10. Time bounds are present, not expired, and not unreasonably far in
 *      the future; no memo, pre-existing signatures, or classic-account
 *      preconditions (ledger bounds, minimum sequence/age/gap, extra
 *      signers) that none of this app's builders ever set; fee and
 *      sequence are sane.
 *   11. Every Soroban authorization entry demands a signature only from the
 *      expected signer (or none at all, for the permissionless
 *      `settle`/`expire` operations), and every contract any entry's
 *      authorized-invocation tree touches is one this app already trusts
 *      for this flow (the care-agreement contract or its configured
 *      settlement asset) — never an unrelated or unexpected contract.
 *
 * A failed check throws `TransactionValidationError` listing every issue
 * found, before any wallet-signing call happens.
 *
 * Scope: this validates the prepared transaction against the app's own
 * trusted configuration and the request the user approved. It cannot
 * protect against a fully compromised browser, wallet extension, or
 * signing device — those already hold the keys.
 */
export type AgreementLifecycleOperation =
  | "fund"
  | "cancel"
  | "attest_care"
  | "open_dispute"
  | "expire"
  | "settle"
  | "resolve_dispute";

export interface AgreementLifecycleCall {
  readonly operation: AgreementLifecycleOperation;
  readonly agreementId: string;
  readonly sourcePublicKey: string;
  readonly attestationCommitment?: string;
  readonly resolution?: "Resume" | "Settle" | "Refund";
}

export interface CreateAgreementCall {
  /** Must equal `provider` — `create_agreement` is provider-authorized. */
  readonly sourcePublicKey: string;
  readonly provider: string;
  readonly sponsor: string;
  readonly attester: string;
  readonly patientRefCommitment: string;
  readonly serviceCommitment: string;
  readonly fundingAmount: string;
  readonly settlementAmount: string;
  readonly fundingDeadline: string;
  readonly careDeadline: string;
  readonly disputeWindowSecs: string;
}

/** Operations whose on-chain functions take no admin/role signature at all. */
const PERMISSIONLESS_OPERATIONS: ReadonlySet<string> = new Set(["settle", "expire"]);

/** A generous circuit breaker, not a tight bound: current builders charge a tiny resource-padded fee. */
const MAX_FEE_STROOPS = 500_000_000n; // 50 XLM

/** Current builders use timeouts of <=120s; this leaves ample headroom without accepting a near-infinite window. */
const MAX_TIME_BOUND_SECONDS = 600n;

interface ExpectedCall {
  readonly method: string;
  readonly sourcePublicKey: string;
  readonly args: Record<string, unknown>;
  readonly permissionless: boolean;
}

function careAgreementSpec(config: StellarClientConfig) {
  return new CareAgreementBindingsClient({
    contractId: config.careAgreementContractId,
    networkPassphrase: config.networkPassphrase,
    rpcUrl: config.rpcUrl,
  }).spec;
}

function argsForLifecycleCall(call: AgreementLifecycleCall): Record<string, unknown> {
  const agreementId = BigInt(call.agreementId);
  switch (call.operation) {
    case "fund":
      return { agreement_id: agreementId, sponsor: call.sourcePublicKey };
    case "cancel":
      return { agreement_id: agreementId, provider: call.sourcePublicKey };
    case "attest_care":
      return {
        agreement_id: agreementId,
        attester: call.sourcePublicKey,
        attestation_commitment: Buffer.from(call.attestationCommitment ?? "", "hex"),
      };
    case "open_dispute":
      return { agreement_id: agreementId, opener: call.sourcePublicKey };
    case "expire":
      return { agreement_id: agreementId };
    case "settle":
      return { agreement_id: agreementId };
    case "resolve_dispute":
      return { agreement_id: agreementId, resolution: { tag: call.resolution, values: undefined } };
  }
}

export function validateAgreementLifecycleTransaction(
  config: StellarClientConfig,
  transaction: Transaction,
  returnedNetworkPassphrase: string,
  call: AgreementLifecycleCall,
): void {
  validateCore(config, transaction, returnedNetworkPassphrase, {
    method: call.operation,
    sourcePublicKey: call.sourcePublicKey,
    args: argsForLifecycleCall(call),
    permissionless: PERMISSIONLESS_OPERATIONS.has(call.operation),
  });
}

export function validateCreateAgreementTransaction(
  config: StellarClientConfig,
  transaction: Transaction,
  returnedNetworkPassphrase: string,
  call: CreateAgreementCall,
): void {
  validateCore(config, transaction, returnedNetworkPassphrase, {
    method: "create_agreement",
    sourcePublicKey: call.sourcePublicKey,
    args: {
      provider: call.provider,
      sponsor: call.sponsor,
      attester: call.attester,
      patient_ref_commitment: Buffer.from(call.patientRefCommitment, "hex"),
      service_commitment: Buffer.from(call.serviceCommitment, "hex"),
      funding_amount: BigInt(call.fundingAmount),
      settlement_amount: BigInt(call.settlementAmount),
      funding_deadline: BigInt(call.fundingDeadline),
      care_deadline: BigInt(call.careDeadline),
      dispute_window_secs: BigInt(call.disputeWindowSecs),
    },
    permissionless: false,
  });
}

function validateCore(
  config: StellarClientConfig,
  transaction: Transaction,
  returnedNetworkPassphrase: string,
  expected: ExpectedCall,
): void {
  const issues: string[] = [];

  // 1. Network.
  if (returnedNetworkPassphrase !== config.networkPassphrase) {
    issues.push(
      `network passphrase mismatch: expected "${config.networkPassphrase}", got "${returnedNetworkPassphrase}"`,
    );
  }

  // 2. Source account.
  if (transaction.source !== expected.sourcePublicKey) {
    issues.push(
      `transaction source "${transaction.source}" does not match the expected signer "${expected.sourcePublicKey}"`,
    );
  }

  // 3. Operation count.
  if (transaction.operations.length !== 1) {
    issues.push(`expected exactly 1 operation, found ${transaction.operations.length}`);
  }

  // Structural problems above make the operation-shape checks below
  // meaningless (there may be no operation to inspect at all) — fail fast
  // rather than let a missing-index access throw a confusing TypeError.
  if (issues.length > 0) {
    throw new TransactionValidationError(issues);
  }

  const op = transaction.operations[0];
  if (op.type !== "invokeHostFunction") {
    throw new TransactionValidationError([`expected an invokeHostFunction operation, found "${op.type}"`]);
  }
  if (op.source !== undefined) {
    issues.push("operation must not override the transaction source");
  }

  const func = op.func;
  if (func.type !== "hostFunctionTypeInvokeContract") {
    throw new TransactionValidationError([
      `expected a contract invocation, found host function type "${func.type}"`,
    ]);
  }

  // 4. Contract address.
  const invokedContract = Address.fromScAddress(func.invokeContract.contractAddress).toString();
  if (invokedContract !== config.careAgreementContractId) {
    issues.push(
      `invoked contract "${invokedContract}" does not match the configured care-agreement contract "${config.careAgreementContractId}"`,
    );
  }

  // 5. Function name.
  const invokedFunction = func.invokeContract.functionName.toString();
  if (invokedFunction !== expected.method) {
    issues.push(`invoked function "${invokedFunction}" does not match the requested operation "${expected.method}"`);
  }

  // 6-9. Arguments: agreement ID, role addresses, attestation commitment,
  // dispute resolution — all encoded as this call's arguments, so a single
  // structured, typed comparison against the real contract spec covers all
  // four at once.
  try {
    const spec = careAgreementSpec(config);
    const expectedArgs = spec.funcArgsToScVals(expected.method, expected.args);
    const actualArgs = func.invokeContract.args;
    const argsMatch =
      actualArgs.length === expectedArgs.length && actualArgs.every((arg, i) => arg.equals(expectedArgs[i]));
    if (!argsMatch) {
      issues.push(`invocation arguments do not match the requested "${expected.method}" call`);
    }
  } catch (err) {
    issues.push(
      `could not encode the expected arguments for "${expected.method}": ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  // 10. Transaction-level constraints.
  if (transaction.memo.type !== "none") {
    issues.push(`unexpected memo of type "${transaction.memo.type}"`);
  }
  if (transaction.signatures.length !== 0) {
    issues.push("transaction already carries signatures before wallet signing");
  }
  if (transaction.ledgerBounds !== undefined) {
    issues.push("unexpected ledger-bounds precondition");
  }
  if (transaction.minAccountSequence !== undefined) {
    issues.push("unexpected minimum-account-sequence precondition");
  }
  if (transaction.minAccountSequenceAge !== undefined) {
    issues.push("unexpected minimum-account-sequence-age precondition");
  }
  if (transaction.minAccountSequenceLedgerGap !== undefined) {
    issues.push("unexpected minimum-account-sequence-ledger-gap precondition");
  }
  if (transaction.extraSigners !== undefined && transaction.extraSigners.length > 0) {
    issues.push("unexpected extra-signers precondition");
  }

  const timeBounds = transaction.timeBounds;
  if (!timeBounds) {
    issues.push("transaction has no time bounds");
  } else {
    const minTime = BigInt(timeBounds.minTime);
    const maxTime = BigInt(timeBounds.maxTime);
    if (minTime !== 0n) {
      issues.push(`unexpected minimum time bound ${minTime}`);
    }
    if (maxTime === 0n) {
      issues.push("transaction has no maximum time bound (never expires)");
    } else {
      const nowSeconds = BigInt(Math.floor(Date.now() / 1000));
      if (maxTime < nowSeconds) {
        issues.push("transaction's time bound has already expired");
      } else if (maxTime > nowSeconds + MAX_TIME_BOUND_SECONDS) {
        issues.push(`transaction's time bound is further in the future than expected (>${MAX_TIME_BOUND_SECONDS}s)`);
      }
    }
  }

  const fee = BigInt(transaction.fee);
  if (fee < 0n) {
    issues.push("transaction fee is negative");
  } else if (fee > MAX_FEE_STROOPS) {
    issues.push(`transaction fee ${fee} stroops exceeds the expected maximum`);
  }

  if (BigInt(transaction.sequence) < 0n) {
    issues.push("transaction sequence number is negative");
  }

  // 11. Soroban authorization entries.
  const authEntries = op.auth ?? [];
  if (expected.permissionless) {
    if (authEntries.length > 0) {
      issues.push(
        `"${expected.method}" is expected to be permissionless but the transaction carries ${authEntries.length} authorization entr${authEntries.length === 1 ? "y" : "ies"}`,
      );
    }
  } else {
    for (const entry of authEntries) {
      validateAuthEntry(config, entry, expected.sourcePublicKey, issues);
    }
  }

  if (issues.length > 0) {
    throw new TransactionValidationError(issues);
  }
}

function validateAuthEntry(
  config: StellarClientConfig,
  entry: xdr.SorobanAuthorizationEntry,
  expectedSigner: string,
  issues: string[],
): void {
  const credentials = entry.credentials;
  if (credentials.type === "sorobanCredentialsAddress" || credentials.type === "sorobanCredentialsAddressV2") {
    const signer = Address.fromScAddress(credentials.value.address).toString();
    if (signer !== expectedSigner) {
      issues.push(`authorization entry demands a signature from "${signer}", not the expected signer "${expectedSigner}"`);
    }
  } else if (credentials.type === "sorobanCredentialsAddressWithDelegates") {
    issues.push("unexpected delegated authorization credentials");
  }
  // "sorobanCredentialsSourceAccount" needs no further check: it is
  // authenticated as the transaction source, already validated above.

  validateInvocationTree(config, entry.rootInvocation, issues);
}

function validateInvocationTree(
  config: StellarClientConfig,
  node: xdr.SorobanAuthorizedInvocation,
  issues: string[],
): void {
  const fn = node.function;
  if (fn.type !== "sorobanAuthorizedFunctionTypeContractFn") {
    issues.push(`unexpected authorized function type "${fn.type}"`);
  } else {
    const contractId = Address.fromScAddress(fn.contractFn.contractAddress).toString();
    if (contractId !== config.careAgreementContractId && contractId !== config.settlementAssetContractId) {
      issues.push(`authorization tree invokes an unexpected contract "${contractId}"`);
    }
  }
  for (const sub of node.subInvocations) {
    validateInvocationTree(config, sub, issues);
  }
}
