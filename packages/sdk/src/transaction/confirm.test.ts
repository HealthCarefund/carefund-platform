import { rpc } from "@stellar/stellar-sdk";
import { describe, expect, it, vi } from "vitest";
import { TransactionFailedError, TransactionTimeoutError } from "../errors.js";
import { lookupTransaction, waitForTransaction, type TransactionLookupClient } from "./confirm.js";

// These are controlled fixtures exercising this SDK's own polling/error-mapping
// logic — no live RPC call is made anywhere in this file.

const HASH = "a".repeat(64);

function notFoundResponse(): rpc.Api.GetMissingTransactionResponse {
  return {
    status: rpc.Api.GetTransactionStatus.NOT_FOUND,
    txHash: HASH,
    latestLedger: 100,
    latestLedgerCloseTime: 0,
    oldestLedger: 1,
    oldestLedgerCloseTime: 0,
  };
}

function successResponse(): rpc.Api.GetSuccessfulTransactionResponse {
  return {
    status: rpc.Api.GetTransactionStatus.SUCCESS,
    txHash: HASH,
    latestLedger: 101,
    latestLedgerCloseTime: 0,
    oldestLedger: 1,
    oldestLedgerCloseTime: 0,
    ledger: 100,
    createdAt: 0,
    applicationOrder: 1,
    feeBump: false,
    events: { diagnosticEventsXdr: [], transactionEventsXdr: [], contractEventsXdr: [] },
  } as unknown as rpc.Api.GetSuccessfulTransactionResponse;
}

function failedResponse(): rpc.Api.GetFailedTransactionResponse {
  return {
    status: rpc.Api.GetTransactionStatus.FAILED,
    txHash: HASH,
    latestLedger: 101,
    latestLedgerCloseTime: 0,
    oldestLedger: 1,
    oldestLedgerCloseTime: 0,
    ledger: 100,
    createdAt: 0,
    applicationOrder: 1,
    feeBump: false,
    events: { diagnosticEventsXdr: [], transactionEventsXdr: [], contractEventsXdr: [] },
  } as unknown as rpc.Api.GetFailedTransactionResponse;
}

describe("lookupTransaction", () => {
  it("calls getTransaction exactly once and returns its response verbatim", async () => {
    const response = notFoundResponse();
    const getTransaction = vi.fn().mockResolvedValue(response);
    const client: TransactionLookupClient = { getTransaction };

    const result = await lookupTransaction(client, HASH);

    expect(result).toBe(response);
    expect(getTransaction).toHaveBeenCalledTimes(1);
    expect(getTransaction).toHaveBeenCalledWith(HASH);
  });
});

describe("waitForTransaction", () => {
  it("resolves as soon as the status is SUCCESS", async () => {
    const success = successResponse();
    const getTransaction = vi.fn().mockResolvedValue(success);
    const client: TransactionLookupClient = { getTransaction };

    const result = await waitForTransaction(client, HASH, {
      attempts: 5,
      intervalMs: 0,
      sleep: vi.fn().mockResolvedValue(undefined),
    });

    expect(result).toBe(success);
    expect(getTransaction).toHaveBeenCalledTimes(1);
  });

  it("polls through NOT_FOUND responses until SUCCESS", async () => {
    const success = successResponse();
    const getTransaction = vi
      .fn()
      .mockResolvedValueOnce(notFoundResponse())
      .mockResolvedValueOnce(notFoundResponse())
      .mockResolvedValueOnce(success);
    const client: TransactionLookupClient = { getTransaction };
    const sleep = vi.fn().mockResolvedValue(undefined);

    const result = await waitForTransaction(client, HASH, { attempts: 5, intervalMs: 10, sleep });

    expect(result).toBe(success);
    expect(getTransaction).toHaveBeenCalledTimes(3);
    expect(sleep).toHaveBeenCalledTimes(2);
    expect(sleep).toHaveBeenCalledWith(10);
  });

  it("throws TransactionFailedError as soon as the status is FAILED, without further polling", async () => {
    const getTransaction = vi
      .fn()
      .mockResolvedValueOnce(notFoundResponse())
      .mockResolvedValueOnce(failedResponse());
    const client: TransactionLookupClient = { getTransaction };

    await expect(
      waitForTransaction(client, HASH, { attempts: 5, intervalMs: 0, sleep: vi.fn() }),
    ).rejects.toThrow(TransactionFailedError);
    expect(getTransaction).toHaveBeenCalledTimes(2);
  });

  it("throws TransactionTimeoutError after exhausting attempts, and never resubmits", async () => {
    const getTransaction = vi.fn().mockResolvedValue(notFoundResponse());
    const client: TransactionLookupClient = { getTransaction };

    await expect(
      waitForTransaction(client, HASH, { attempts: 3, intervalMs: 0, sleep: vi.fn() }),
    ).rejects.toThrow(TransactionTimeoutError);
    // Exactly `attempts` lookups — this function has no way to submit anything,
    // so "never resubmits" here means it never calls getTransaction more than
    // the bound allows either.
    expect(getTransaction).toHaveBeenCalledTimes(3);
  });
});
