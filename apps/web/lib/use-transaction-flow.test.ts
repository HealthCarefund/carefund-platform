import { Account, Address, Keypair, Operation, TransactionBuilder, nativeToScVal } from "@stellar/stellar-sdk";
import { TransactionValidationError } from "@carefund/sdk";
import { describe, expect, it, vi } from "vitest";

// This exercises the real `prepareViaBackend`/`prepareCreateAgreement`
// integration — the actual functions `useTransactionFlow` calls, not a
// reimplementation of their logic — with only the network boundary
// (`./api-client`'s `fetch` call, and the RPC client for the
// `create_agreement` path) mocked. Every transaction handed back by those
// mocks is a real, structurally valid envelope built with the pinned
// `@stellar/stellar-sdk` classes.

const NETWORK_PASSPHRASE = "Standalone Network ; February 2017";
const CARE_AGREEMENT_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const SETTLEMENT_ASSET_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const PROVIDER_REGISTRY_CONTRACT_ID = Address.contract(Keypair.random().rawPublicKey()).toString();
const SPONSOR = Keypair.random().publicKey();

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
vi.mock("@carefund/sdk", async () => {
  const actual = await vi.importActual<typeof import("@carefund/sdk")>("@carefund/sdk");
  return {
    ...actual,
    awaitWalletSignature: (...args: unknown[]) => awaitWalletSignatureMock(...args),
    submitTransaction: (...args: unknown[]) => submitTransactionMock(...args),
  };
});

/** Encodes `fund`'s two arguments exactly as the real contract spec would (u64, address), without pulling in the generated bindings package. */
function fundArgs(agreementId: bigint, sponsor: string) {
  return [nativeToScVal(agreementId, { type: "u64" }), nativeToScVal(sponsor, { type: "address" })];
}

function realFundXdr(overrides: { contractId?: string; sponsor?: string; agreementId?: bigint } = {}): string {
  const sponsor = overrides.sponsor ?? SPONSOR;
  const account = new Account(sponsor, "100");
  const op = Operation.invokeContractFunction({
    contract: overrides.contractId ?? CARE_AGREEMENT_CONTRACT_ID,
    function: "fund",
    args: fundArgs(overrides.agreementId ?? BigInt(7), sponsor),
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
      }),
    )
    .setTimeout(30)
    .build();
  const feeBump = TransactionBuilder.buildFeeBumpTransaction(Keypair.random(), "1000000", inner, NETWORK_PASSPHRASE);
  return feeBump.toXDR();
}

describe("prepareViaBackend — independent validation before wallet signing", () => {
  it("accepts a correct, backend-prepared fund transaction", async () => {
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
});
