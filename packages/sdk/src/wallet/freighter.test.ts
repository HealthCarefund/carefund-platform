import { beforeEach, describe, expect, it, vi } from "vitest";
import { validateStellarClientConfig } from "../config.js";
import {
  WalletConnectionRejectedError,
  WalletNotInstalledError,
  WalletSigningRejectedError,
  WrongNetworkError,
} from "../errors.js";

// These tests mock the Freighter *browser extension* boundary — not any
// blockchain or RPC behavior — so this SDK's own connect/network-guard/
// error-mapping logic can be verified deterministically without a real
// browser or a real Freighter install. No RPC or chain call happens here.

const freighterMock = vi.hoisted(() => ({
  isBrowser: true,
  isConnected: vi.fn(),
  requestAccess: vi.fn(),
  getAddress: vi.fn(),
  getNetworkDetails: vi.fn(),
  signTransaction: vi.fn(),
}));

vi.mock("@stellar/freighter-api", () => freighterMock);

const { isFreighterInstalled, connectFreighterWallet, getConnectedFreighterAddress, verifyFreighterNetwork, createFreighterSigner } =
  await import("./freighter.js");
const { toStellarAddress } = await import("@carefund/types");

const ADDRESS = toStellarAddress("G" + "A".repeat(55));

const config = validateStellarClientConfig({
  network: "TESTNET",
  rpcUrl: "https://soroban-testnet.stellar.org",
  networkPassphrase: "Test SDF Network ; September 2015",
  providerRegistryContractId: "C" + "A".repeat(55),
  careAgreementContractId: "C" + "B".repeat(55),
  settlementAssetContractId: "C" + "D".repeat(55),
});

beforeEach(() => {
  freighterMock.isBrowser = true;
  vi.clearAllMocks();
});

describe("isFreighterInstalled", () => {
  it("is false outside a browser regardless of what isConnected would say", async () => {
    freighterMock.isBrowser = false;
    expect(await isFreighterInstalled()).toBe(false);
    expect(freighterMock.isConnected).not.toHaveBeenCalled();
  });

  it("is true when isConnected reports true with no error", async () => {
    freighterMock.isConnected.mockResolvedValue({ isConnected: true });
    expect(await isFreighterInstalled()).toBe(true);
  });

  it("is false when isConnected reports an error", async () => {
    freighterMock.isConnected.mockResolvedValue({ isConnected: true, error: { code: 1, message: "no extension" } });
    expect(await isFreighterInstalled()).toBe(false);
  });
});

describe("connectFreighterWallet", () => {
  it("throws WalletNotInstalledError when the extension is not present", async () => {
    freighterMock.isConnected.mockResolvedValue({ isConnected: false });
    await expect(connectFreighterWallet()).rejects.toThrow(WalletNotInstalledError);
    expect(freighterMock.requestAccess).not.toHaveBeenCalled();
  });

  it("returns the address on success", async () => {
    freighterMock.isConnected.mockResolvedValue({ isConnected: true });
    freighterMock.requestAccess.mockResolvedValue({ address: ADDRESS });
    expect(await connectFreighterWallet()).toBe(ADDRESS);
  });

  it("throws WalletConnectionRejectedError, carrying Freighter's code/message, on user decline", async () => {
    freighterMock.isConnected.mockResolvedValue({ isConnected: true });
    freighterMock.requestAccess.mockResolvedValue({ address: "", error: { code: -4, message: "User declined access" } });

    try {
      await connectFreighterWallet();
      expect.unreachable("expected connectFreighterWallet to throw");
    } catch (error) {
      expect(error).toBeInstanceOf(WalletConnectionRejectedError);
      expect((error as WalletConnectionRejectedError).freighterCode).toBe(-4);
      expect((error as WalletConnectionRejectedError).freighterMessage).toBe("User declined access");
    }
  });
});

describe("getConnectedFreighterAddress", () => {
  it("returns undefined outside a browser", async () => {
    freighterMock.isBrowser = false;
    expect(await getConnectedFreighterAddress()).toBeUndefined();
  });

  it("returns undefined when Freighter reports an error", async () => {
    freighterMock.getAddress.mockResolvedValue({ address: "", error: { code: 1, message: "not allowed" } });
    expect(await getConnectedFreighterAddress()).toBeUndefined();
  });

  it("returns the address when present", async () => {
    freighterMock.getAddress.mockResolvedValue({ address: ADDRESS });
    expect(await getConnectedFreighterAddress()).toBe(ADDRESS);
  });
});

describe("verifyFreighterNetwork", () => {
  it("resolves when the wallet's passphrase matches the config", async () => {
    freighterMock.getNetworkDetails.mockResolvedValue({
      network: "TESTNET",
      networkUrl: "https://horizon-testnet.stellar.org",
      networkPassphrase: config.networkPassphrase,
    });
    await expect(verifyFreighterNetwork(config)).resolves.toBeUndefined();
  });

  it("throws WrongNetworkError when the wallet is on a different network", async () => {
    freighterMock.getNetworkDetails.mockResolvedValue({
      network: "PUBLIC",
      networkUrl: "https://horizon.stellar.org",
      networkPassphrase: "Public Global Stellar Network ; September 2015",
    });

    try {
      await verifyFreighterNetwork(config);
      expect.unreachable("expected verifyFreighterNetwork to throw");
    } catch (error) {
      expect(error).toBeInstanceOf(WrongNetworkError);
      expect((error as WrongNetworkError).expectedPassphrase).toBe(config.networkPassphrase);
      expect((error as WrongNetworkError).actualPassphrase).toBe("Public Global Stellar Network ; September 2015");
    }
  });
});

describe("createFreighterSigner", () => {
  it("rejects immediately on a networkPassphrase mismatch, without ever calling Freighter", async () => {
    const signer = createFreighterSigner(config, ADDRESS);

    await expect(
      signer.signTransaction("AAAA", { networkPassphrase: "Public Global Stellar Network ; September 2015" }),
    ).rejects.toThrow(WrongNetworkError);
    expect(freighterMock.getNetworkDetails).not.toHaveBeenCalled();
    expect(freighterMock.signTransaction).not.toHaveBeenCalled();
  });

  it("blocks signing when the wallet itself reports the wrong network, even if the caller's passphrase matched", async () => {
    freighterMock.getNetworkDetails.mockResolvedValue({
      network: "PUBLIC",
      networkUrl: "https://horizon.stellar.org",
      networkPassphrase: "Public Global Stellar Network ; September 2015",
    });
    const signer = createFreighterSigner(config, ADDRESS);

    await expect(signer.signTransaction("AAAA", { networkPassphrase: config.networkPassphrase })).rejects.toThrow(
      WrongNetworkError,
    );
    expect(freighterMock.signTransaction).not.toHaveBeenCalled();
  });

  it("returns the signed XDR on success", async () => {
    freighterMock.getNetworkDetails.mockResolvedValue({
      network: "TESTNET",
      networkUrl: "https://horizon-testnet.stellar.org",
      networkPassphrase: config.networkPassphrase,
    });
    freighterMock.signTransaction.mockResolvedValue({ signedTxXdr: "SIGNED_XDR", signerAddress: ADDRESS });
    const signer = createFreighterSigner(config, ADDRESS);

    const result = await signer.signTransaction("AAAA", { networkPassphrase: config.networkPassphrase });

    expect(result).toEqual({ signedTransactionXdr: "SIGNED_XDR" });
    expect(freighterMock.signTransaction).toHaveBeenCalledWith("AAAA", {
      networkPassphrase: config.networkPassphrase,
      address: ADDRESS,
    });
  });

  it("throws WalletSigningRejectedError on user decline, carrying Freighter's code/message", async () => {
    freighterMock.getNetworkDetails.mockResolvedValue({
      network: "TESTNET",
      networkUrl: "https://horizon-testnet.stellar.org",
      networkPassphrase: config.networkPassphrase,
    });
    freighterMock.signTransaction.mockResolvedValue({
      signedTxXdr: "",
      signerAddress: ADDRESS,
      error: { code: -4, message: "User declined access" },
    });
    const signer = createFreighterSigner(config, ADDRESS);

    try {
      await signer.signTransaction("AAAA", { networkPassphrase: config.networkPassphrase });
      expect.unreachable("expected signTransaction to throw");
    } catch (error) {
      expect(error).toBeInstanceOf(WalletSigningRejectedError);
      expect((error as WalletSigningRejectedError).freighterCode).toBe(-4);
    }
  });
});
