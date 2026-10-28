import { Buffer } from "buffer";
import { Address, Transaction, nativeToScVal, xdr } from "@stellar/stellar-sdk";
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
 *   11. The *exact* Soroban authorization tree, not merely a contract
 *      allowlist. Every method has a known, fixed authorization shape —
 *      established by reading the actual contract source, not assumed —
 *      and anything that deviates from it is rejected:
 *        - `settle`/`expire` are genuinely permissionless: both transfer
 *          funds with the care-agreement contract itself as the token
 *          `from` (self-authorizing, no `SorobanAuthorizationEntry`
 *          needed), so they must carry zero authorization entries.
 *        - Every other operation requires exactly one authorization
 *          entry, from exactly the expected signer, whose root invocation
 *          is *exactly* this same contract/method/arguments (not merely
 *          "some call to the care-agreement contract").
 *        - `fund` is the one operation that moves tokens from a real user
 *          (the sponsor) rather than from the contract itself, so its
 *          authorization root has exactly one nested sub-invocation: the
 *          settlement asset's `transfer(sponsor, careAgreementContract,
 *          fundingAmount)` — contract, function, and all three arguments
 *          checked exactly, including the amount, which is read fresh
 *          from on-chain contract state (see `expectedFundingAmount`
 *          below) rather than trusted from any off-chain cache. Every
 *          other operation's authorization root has zero nested
 *          sub-invocations, because every other fund movement those
 *          methods perform uses the contract's own address as the `from`.
 *      An authorization shape this validator does not recognize (an
 *      unexpected function, an unexpected argument, an unexpected nested
 *      call, a different signer, more or fewer authorization entries than
 *      expected) is rejected — fail closed, never "trusted because the
 *      contract address matched."
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
  /**
   * Required for, and only for, `fund`: the authoritative on-chain funding
   * amount (stroops) for this agreement. `fund`'s contract-side escrow
   * transfer amount is not an argument of the `fund` call itself — the
   * contract reads it from stored agreement state — so there is no way to
   * verify the nested transfer this validator must check without an
   * independent, trustworthy source for that number. Read it fresh via
   * this SDK's `getAgreement` (a live contract read) immediately before
   * calling this function; never pass through an off-chain cache or a
   * value the backend merely asserts.
   */
  readonly expectedFundingAmount?: string;
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

/** A single node this validator expects to find in a Soroban authorization tree: an exact contract, function, argument list, and set of expected children — never just a contract allowlist. */
interface ExpectedInvocation {
  readonly contractId: string;
  readonly method: string;
  readonly argScVals: readonly xdr.ScVal[];
  readonly subInvocations: readonly ExpectedInvocation[];
}

interface ExpectedCall {
  readonly method: string;
  readonly sourcePublicKey: string;
  readonly args: Record<string, unknown>;
  readonly permissionless: boolean;
  /** Nested sub-invocations expected directly under this call's own authorization root — see the module doc comment. Empty for every operation except `fund`. */
  readonly subInvocations: readonly ExpectedInvocation[];
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

/**
 * `fund`'s one nested sub-invocation: the settlement asset's
 * `transfer(sponsor, careAgreementContract, fundingAmount)`, which the
 * care-agreement contract calls internally and which therefore needs the
 * sponsor's authorization nested under their `fund` authorization root.
 * The amount comes from `call.expectedFundingAmount` — the caller's
 * responsibility to have read from live contract state — never invented
 * here.
 */
function fundTransferSubInvocation(config: StellarClientConfig, call: AgreementLifecycleCall): ExpectedInvocation {
  if (call.expectedFundingAmount === undefined) {
    throw new TransactionValidationError([
      "fund requires expectedFundingAmount (the authoritative on-chain funding amount, read via getAgreement) to validate the escrow transfer — refusing to validate without it rather than trust an unverified amount",
    ]);
  }
  return {
    contractId: config.settlementAssetContractId,
    method: "transfer",
    argScVals: [
      nativeToScVal(call.sourcePublicKey, { type: "address" }),
      nativeToScVal(config.careAgreementContractId, { type: "address" }),
      nativeToScVal(BigInt(call.expectedFundingAmount), { type: "i128" }),
    ],
    subInvocations: [],
  };
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
    subInvocations: call.operation === "fund" ? [fundTransferSubInvocation(config, call)] : [],
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
    subInvocations: [],
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
  // four at once. The same encoded ScVals are reused below (section 11) as
  // the expected root arguments of the authorization tree, so both checks
  // agree on exactly one definition of "the right call".
  let expectedArgs: xdr.ScVal[] | undefined;
  try {
    const spec = careAgreementSpec(config);
    expectedArgs = spec.funcArgsToScVals(expected.method, expected.args);
    const actualArgs = func.invokeContract.args;
    const argsMatch =
      actualArgs.length === expectedArgs.length && actualArgs.every((arg, i) => arg.equals(expectedArgs![i]));
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

  // 11. The exact Soroban authorization tree (see the module doc comment
  // for why each operation's expected shape is what it is).
  const authEntries = op.auth ?? [];
  if (expected.permissionless) {
    if (authEntries.length > 0) {
      issues.push(
        `"${expected.method}" is expected to be permissionless but the transaction carries ${authEntries.length} authorization entr${authEntries.length === 1 ? "y" : "ies"}`,
      );
    }
  } else if (authEntries.length !== 1) {
    issues.push(
      `expected exactly 1 authorization entry for "${expected.method}", found ${authEntries.length}`,
    );
  } else if (expectedArgs !== undefined) {
    const entry = authEntries[0];
    const credentials = entry.credentials;
    if (credentials.type === "sorobanCredentialsAddress" || credentials.type === "sorobanCredentialsAddressV2") {
      const signer = Address.fromScAddress(credentials.value.address).toString();
      if (signer !== expected.sourcePublicKey) {
        issues.push(
          `authorization entry demands a signature from "${signer}", not the expected signer "${expected.sourcePublicKey}"`,
        );
      }
    } else if (credentials.type === "sorobanCredentialsAddressWithDelegates") {
      issues.push("unexpected delegated authorization credentials");
    }
    // "sorobanCredentialsSourceAccount" needs no further check: it is
    // authenticated as the transaction source, already validated above.

    const expectedRoot: ExpectedInvocation = {
      contractId: config.careAgreementContractId,
      method: expected.method,
      argScVals: expectedArgs,
      subInvocations: expected.subInvocations,
    };
    validateInvocationTree(entry.rootInvocation, expectedRoot, issues);
  }

  if (issues.length > 0) {
    throw new TransactionValidationError(issues);
  }
}

/**
 * Recursively checks that `node` is *exactly* `expected`: same contract,
 * same function, same arguments (typed, structured comparison against the
 * real ScVal encoding — never a string/XDR-blob comparison), and exactly
 * the expected set of nested sub-invocations in the expected order. Any
 * authorized-function shape this validator does not recognize (a
 * create-contract authorization, anywhere in the tree) is rejected.
 */
function validateInvocationTree(
  node: xdr.SorobanAuthorizedInvocation,
  expected: ExpectedInvocation,
  issues: string[],
): void {
  const fn = node.function;
  if (fn.type !== "sorobanAuthorizedFunctionTypeContractFn") {
    issues.push(
      `unexpected authorized function type "${fn.type}" (expected a call to "${expected.method}" on "${expected.contractId}")`,
    );
    return;
  }

  const contractId = Address.fromScAddress(fn.contractFn.contractAddress).toString();
  if (contractId !== expected.contractId) {
    issues.push(
      `authorization invokes unexpected contract "${contractId}" (expected "${expected.contractId}" for "${expected.method}")`,
    );
  }

  const functionName = fn.contractFn.functionName.toString();
  if (functionName !== expected.method) {
    issues.push(`authorization invokes unexpected function "${functionName}" (expected "${expected.method}")`);
  }

  const actualArgs = fn.contractFn.args;
  const argsMatch =
    actualArgs.length === expected.argScVals.length &&
    actualArgs.every((arg, i) => arg.equals(expected.argScVals[i]));
  if (!argsMatch) {
    issues.push(`authorization arguments for "${expected.method}" do not match the expected call`);
  }

  if (node.subInvocations.length !== expected.subInvocations.length) {
    issues.push(
      `authorization for "${expected.method}" has ${node.subInvocations.length} nested invocation(s), expected exactly ${expected.subInvocations.length}`,
    );
    return;
  }

  node.subInvocations.forEach((sub, i) => validateInvocationTree(sub, expected.subInvocations[i], issues));
}
