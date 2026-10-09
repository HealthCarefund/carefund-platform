import { describe, expect, it, vi } from "vitest";
import { WalletSigningRejectedError } from "@carefund/sdk";
import { describeError, type TransactionFlowState } from "./use-transaction-flow";

describe("Wallet signature rejection handling", () => {
  it("formats WalletSigningRejectedError with exact human-readable message", () => {
    const error = new WalletSigningRejectedError(-4, "User declined access");
    const formatted = describeError(error);
    expect(formatted).toBe("Wallet declined to sign the transaction.");
  });

  it("records signature rejection as Failed with no tx hash and without submission", async () => {
    const mockSigner = {
      publicKey: "G" + "A".repeat(55),
      signTransaction: vi.fn().mockRejectedValue(new WalletSigningRejectedError(-4, "User declined access")),
    };

    const submitToRpcMock = vi.fn();
    const stateHistory: TransactionFlowState[] = [];

    // Simulate the transaction flow execution
    let currentState: TransactionFlowState = { phase: "idle" };
    const setState = (next: TransactionFlowState) => {
      currentState = next;
      stateHistory.push(next);
    };

    const runFlow = async () => {
      setState({ phase: "preparing" });
      setState({ phase: "simulating" });
      const transaction = { toXDR: () => "AAAA" };
      const networkPassphrase = "Test SDF Network ; September 2015";

      try {
        setState({ phase: "awaiting_signature" });
        await mockSigner.signTransaction(transaction.toXDR(), { networkPassphrase });

        // If signature had succeeded, it would proceed to submission:
        setState({ phase: "submitting" });
        await submitToRpcMock();
        setState({ phase: "submitted", hash: "mock-hash" });
      } catch (err) {
        setState({ phase: "failed", reason: describeError(err) });
      }
    };

    await runFlow();

    // 1. UI status is Failed
    const lastState = stateHistory[stateHistory.length - 1];
    expect(lastState?.phase).toBe("failed");
    if (lastState && lastState.phase === "failed") {
      // 2. Exact message
      expect(lastState.reason).toBe("Wallet declined to sign the transaction.");
      // 3. No transaction hash
      expect(lastState.hash).toBeUndefined();
    }

    // 4. No Submitted or Confirmed state in history
    const phases = stateHistory.map((s) => s.phase);
    expect(phases).not.toContain("submitted");
    expect(phases).not.toContain("confirmed");
    expect(phases).not.toContain("submitting");

    // 5. Submit was never called
    expect(submitToRpcMock).not.toHaveBeenCalled();
    expect(mockSigner.signTransaction).toHaveBeenCalledTimes(1);
  });
});
