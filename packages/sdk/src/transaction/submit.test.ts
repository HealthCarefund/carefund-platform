import { rpc } from "@stellar/stellar-sdk";
import type { Transaction } from "@stellar/stellar-sdk";
import { describe, expect, it, vi } from "vitest";
import { SubmissionRejectedError } from "../errors.js";
import { submitTransaction, type TransactionSubmitClient } from "./submit.js";

// Controlled fixtures for this SDK's own status-mapping logic — no live
// submission to any network happens in this file.

const HASH = "b".repeat(64);
const fakeTransaction = {} as Transaction;

function pendingResponse(): rpc.Api.SendTransactionResponse {
  return {
    status: "PENDING",
    hash: HASH,
    latestLedger: 100,
    latestLedgerCloseTime: 0,
  };
}

describe("submitTransaction", () => {
  it("returns the hash when the network accepts the transaction as PENDING", async () => {
    const sendTransaction = vi.fn().mockResolvedValue(pendingResponse());
    const client: TransactionSubmitClient = { sendTransaction };

    const result = await submitTransaction(client, fakeTransaction);

    expect(result).toEqual({ hash: HASH, status: "PENDING" });
  });

  it.each(["DUPLICATE", "TRY_AGAIN_LATER", "ERROR"] as const)(
    "throws SubmissionRejectedError with the exact status for %s",
    async (status) => {
      const sendTransaction = vi.fn().mockResolvedValue({
        status,
        hash: HASH,
        latestLedger: 100,
        latestLedgerCloseTime: 0,
      });
      const client: TransactionSubmitClient = { sendTransaction };

      try {
        await submitTransaction(client, fakeTransaction);
        expect.unreachable("expected submitTransaction to throw");
      } catch (error) {
        expect(error).toBeInstanceOf(SubmissionRejectedError);
        expect((error as SubmissionRejectedError).status).toBe(status);
      }
    },
  );

  it("does not resubmit or retry on rejection — it throws immediately", async () => {
    const sendTransaction = vi.fn().mockResolvedValue({
      status: "TRY_AGAIN_LATER",
      hash: HASH,
      latestLedger: 100,
      latestLedgerCloseTime: 0,
    });
    const client: TransactionSubmitClient = { sendTransaction };

    await expect(submitTransaction(client, fakeTransaction)).rejects.toThrow(
      SubmissionRejectedError,
    );
    expect(sendTransaction).toHaveBeenCalledTimes(1);
  });
});
