import { Account, Address, Keypair, Operation, TransactionBuilder, nativeToScVal, xdr } from "@stellar/stellar-sdk";
import { TransactionValidationError, WalletSigningRejectedError } from "@carefund/sdk";
import { describe, expect, it, vi } from "vitest";

// This exercises the real `prepareViaBackend`/`prepareCreateAgreement`
// integration — the actual functions `useTransactionFlow` calls, not a
// reimplementation of their logic — with only the network boundary
// (`./api-client`'s `fetch` call, `getAgreement`'s RPC read, and the
// wallet/RPC submission calls) mocked. Every transaction handed back by
// those mocks is a real, structurally valid envelope built with the
// pinned `@stellar/stellar-sdk` classes, including real (placeholder-
// signed) Soroban authorization entries matching the real `fund`
// authorization tree — see packages/sdk/src/transaction/validate.ts for
// why that tree has exactly the shape used below.

const NETWORK_PASSPHRASE = "Standalone Network ; February 2017";
const CARE_AGREEMENT_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const SETTLEMENT_ASSET_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const PROVIDER_REGISTRY_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const SPONSOR = Keypair.random().publicKey();
const FUNDING_AMOUNT = BigInt(1_000_000);

process.env.NEXT_PUBLIC_STELLAR_NETWORK = "CUSTOM";
process.env.NEXT_PUBLIC_STELLAR_RPC_URL = "https://rpc.example.com";
process.env.NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE = NETWORK_PASSPHRASE;
process.env.NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID = PROVIDER_REGISTRY_CONTRACT_ID;
process.env.NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID = CARE_AGREEMENT_CONTRACT_ID;
process.env.NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID = SETTLEMENT_ASSET_CONTRACT_ID;
process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.com";

const prepareAgreementTransactionMock = vi.fn();
vi.mock("./api-client", () => ({
  prepareAgreementTransaction: (...args: unknown[]) => prepareAgreementTransactionMock(...args),
  lookupTransaction: vi.fn(),
  ApiError: class ApiError extends Error {},
}));

const awaitWalletSignatureMock = vi.fn();
const submitTransactionMock = vi.fn();
const getAgreementMock = vi.fn();
vi.mock("@carefund/sdk", async () => {
  const actual = await vi.importActual<typeof import("@carefund/sdk")>("@carefund/sdk");
  return {
    ...actual,
    awaitWalletSignature: (...args: unknown[]) => awaitWalletSignatureMock(...args),
    submitTransaction: (...args: unknown[]) => submitTransactionMock(...args),
    getAgreement: (...args: unknown[]) => getAgreementMock(...args),
  };
});

getAgreementMock.mockResolvedValue({ fundingAmount: FUNDING_AMOUNT.toString() });

/** Encodes `fund`'s two arguments exactly as the real contract spec would (u64, address), without pulling in the generated bindings package. */
function fundArgs(agreementId: bigint, sponsor: string) {
  return [nativeToScVal(agreementId, { type: "u64" }), nativeToScVal(sponsor, { type: "address" })];
}

/** The real authorization tree `fund` produces: the sponsor's root authorization nesting the settlement asset's own transfer call — see `contracts/care-agreement/src/lib.rs::fund`. */
function fundAuthEntry(sponsor: string, agreementId: bigint, amount: bigint, contractId = CARE_AGREEMENT_CONTRACT_ID): xdr.SorobanAuthorizationEntry {
  return new xdr.SorobanAuthorizationEntry({
    credentials: xdr.SorobanCredentials.sorobanCredentialsAddress(
      new xdr.SorobanAddressCredentials({
        address: Address.fromString(sponsor).toScAddress(),
        nonce: BigInt(1),
        signatureExpirationLedger: 1000,
        signature: xdr.ScVal.scvVoid(),
      }),
    ),
    rootInvocation: new xdr.SorobanAuthorizedInvocation({
      function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
        new xdr.InvokeContractArgs({
          contractAddress: Address.fromString(contractId).toScAddress(),
          functionName: "fund",
          args: fundArgs(agreementId, sponsor),
        }),
      ),
      subInvocations: [
        new xdr.SorobanAuthorizedInvocation({
          function: xdr.SorobanAuthorizedFunction.sorobanAuthorizedFunctionTypeContractFn(
            new xdr.InvokeContractArgs({
              contractAddress: Address.fromString(SETTLEMENT_ASSET_CONTRACT_ID).toScAddress(),
              functionName: "transfer",
              args: [
                nativeToScVal(sponsor, { type: "address" }),
                nativeToScVal(contractId, { type: "address" }),
                nativeToScVal(amount, { type: "i128" }),
              ],
            }),
          ),
          subInvocations: [],
        }),
      ],
    }),
  });
}

function realFundXdr(
  overrides: { contractId?: string; sponsor?: string; agreementId?: bigint; amount?: bigint } = {},
): string {
  const sponsor = overrides.sponsor ?? SPONSOR;
  const contractId = overrides.contractId ?? CARE_AGREEMENT_CONTRACT_ID;
  const agreementId = overrides.agreementId ?? BigInt(7);
  const amount = overrides.amount ?? FUNDING_AMOUNT;
  const account = new Account(sponsor, "100");
  const op = Operation.invokeContractFunction({
    contract: contractId,
    function: "fund",
    args: fundArgs(agreementId, sponsor),
    auth: [fundAuthEntry(sponsor, agreementId, amount, contractId)],
  });
  const tx = new TransactionBuilder(account, { fee: "10000", networkPassphrase: NETWORK_PASSPHRASE })
    .addOperation(op)
    .setTimeout(30)
    .build();
  return tx.toXDR();
}

function feeBumpXdr(): string {
  const inner = new TransactionBuilder(new Account(SPONSOR, "100"), {
    fee: "100",
    networkPassphrase: NETWORK_PASSPHRASE,
  })
    .addOperation(
      Operation.invokeContractFunction({
        contract: CARE_AGREEMENT_CONTRACT_ID,
        function: "fund",
        args: fundArgs(BigInt(7), SPONSOR),
        auth: [fundAuthEntry(SPONSOR, BigInt(7), FUNDING_AMOUNT)],
      }),
    )
    .setTimeout(30)
    .build();
  const feeBump = TransactionBuilder.buildFeeBumpTransaction(Keypair.random(), "1000000", inner, NETWORK_PASSPHRASE);
  return feeBump.toXDR();
}

describe("prepareViaBackend — independent validation before wallet signing", () => {
  it("accepts a correct, backend-prepared fund transaction with its real escrow-transfer authorization", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr(),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const result = await prepareViaBackend({
      agreementId: "7",
      operation: "fund",
      sourcePublicKey: SPONSOR,
    })();

    expect(result.transaction.source).toBe(SPONSOR);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
  });

  it("rejects malformed XDR before any signing attempt", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: "not-valid-base64-xdr!!!",
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
    expect(submitTransactionMock).not.toHaveBeenCalled();
  });

  it("rejects an unsupported fee-bump envelope before any signing attempt", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: feeBumpXdr(),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
    expect(submitTransactionMock).not.toHaveBeenCalled();
  });

  it("rejects a wrong network passphrase returned alongside otherwise-valid XDR", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr(),
      network: "TESTNET",
      networkPassphrase: "Test SDF Network ; September 2015",
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
  });

  it("rejects a transaction invoking a different contract", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    const otherContract = Address.contract(Keypair.random().rawPublicKey()).toString();
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr({ contractId: otherContract }),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
  });

  it("rejects a transaction for a different agreement ID than requested", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr({ agreementId: BigInt(999) }),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
  });

  it("rejects a transaction from a different source account than the requester", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    const impostor = Keypair.random().publicKey();
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr({ sponsor: impostor }),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
  });

  it("rejects an escrow transfer whose amount does not match the authoritative on-chain funding amount", async () => {
    const { prepareViaBackend } = await import("./use-transaction-flow");
    prepareAgreementTransactionMock.mockResolvedValueOnce({
      unsignedTransactionXdr: realFundXdr({ amount: FUNDING_AMOUNT * BigInt(1000) }),
      network: "CUSTOM",
      networkPassphrase: NETWORK_PASSPHRASE,
    });

    const preparer = prepareViaBackend({ agreementId: "7", operation: "fund", sourcePublicKey: SPONSOR });
    await expect(preparer()).rejects.toThrow(TransactionValidationError);
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
    expect(getAgreementMock).toHaveBeenCalled();
  });
});

describe("runTransactionPipeline — the real signing boundary", () => {
  it("for invalid prepared XDR: rejects, never signs, never submits, generates no hash, and ends in a failed (not submitted/confirmed) state", async () => {
    const { runTransactionPipeline } = await import("./use-transaction-flow");
    const states: unknown[] = [];
    const prepare = vi.fn().mockRejectedValue(new TransactionValidationError(["wrong contract"]));
    const getSigner = vi.fn();

    await runTransactionPipeline(prepare, getSigner, (s) => states.push(s), () => false);

    expect(prepare).toHaveBeenCalledTimes(1);
    expect(getSigner).not.toHaveBeenCalled();
    expect(awaitWalletSignatureMock).not.toHaveBeenCalled();
    expect(submitTransactionMock).not.toHaveBeenCalled();

    const phases = (states as { phase: string }[]).map((s) => s.phase);
    expect(phases).not.toContain("awaiting_signature");
    expect(phases).not.toContain("submitting");
    expect(phases).not.toContain("submitted");
    expect(phases).not.toContain("confirmed");

    const last = states[states.length - 1] as { phase: string; hash?: string };
    expect(last.phase).toBe("failed");
    expect(last.hash).toBeUndefined();
  });

  it("for a valid prepared transaction: reaches the wallet-signing stage with the prepared transaction and passphrase", async () => {
    const { runTransactionPipeline } = await import("./use-transaction-flow");
    const states: unknown[] = [];
    const fakeTransaction = { source: SPONSOR } as never;
    const prepare = vi.fn().mockResolvedValue({ transaction: fakeTransaction, networkPassphrase: NETWORK_PASSPHRASE });
    const signer = { publicKey: SPONSOR, signTransaction: vi.fn() };
    const getSigner = vi.fn().mockReturnValue(signer);

    // Let the pipeline reach signing, then stop it there deliberately by
    // having the (real) awaitWalletSignature reject — this test is about
    // whether the real code *reaches* the signing call with the right
    // arguments, not about what happens after a successful signature.
    awaitWalletSignatureMock.mockRejectedValueOnce(new WalletSigningRejectedError(-4, "stop here"));

    await runTransactionPipeline(prepare, getSigner, (s) => states.push(s), () => false);

    expect(prepare).toHaveBeenCalledTimes(1);
    expect(awaitWalletSignatureMock).toHaveBeenCalledWith(signer, fakeTransaction, NETWORK_PASSPHRASE);

    const phases = (states as { phase: string }[]).map((s) => s.phase);
    expect(phases).toContain("awaiting_signature");
  });
});
