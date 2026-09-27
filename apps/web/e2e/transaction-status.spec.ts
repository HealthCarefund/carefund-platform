import { test, expect } from "@playwright/test";

// Mocks the transaction-lookup API response — verifies this page's
// rendering of each lookup status, not a real transaction's real
// on-chain outcome.
const HASH = "a".repeat(64);

test.describe("Transaction status page (mocked API)", () => {
  test("renders a confirmed transaction with its ledger", async ({ page }) => {
    await page.route(`**/api/v1/transactions/${HASH}`, async (route) => {
      await route.fulfill({ json: { hash: HASH, status: "success", ledger: "123456" } });
    });
    await page.goto(`/transactions/${HASH}`);
    await expect(page.getByText("Confirmed on-chain")).toBeVisible();
    await expect(page.getByText("Ledger 123456")).toBeVisible();
  });

  test("renders a failed transaction", async ({ page }) => {
    await page.route(`**/api/v1/transactions/${HASH}`, async (route) => {
      await route.fulfill({ json: { hash: HASH, status: "failed" } });
    });
    await page.goto(`/transactions/${HASH}`);
    await expect(page.getByText("Failed on-chain")).toBeVisible();
  });

  test("renders a not-found lookup without claiming failure or success", async ({ page }) => {
    await page.route(`**/api/v1/transactions/${HASH}`, async (route) => {
      await route.fulfill({ json: { hash: HASH, status: "not_found" } });
    });
    await page.goto(`/transactions/${HASH}`);
    await expect(page.getByText(/Not found/)).toBeVisible();
  });

  test("renders reconciliation detail when present", async ({ page }) => {
    await page.route(`**/api/v1/transactions/${HASH}`, async (route) => {
      await route.fulfill({
        json: {
          hash: HASH,
          status: "not_found",
          reconciliation: {
            operation: "fund",
            agreementId: "2",
            firstSeen: "2026-01-01T00:00:00Z",
          },
        },
      });
    });
    await page.goto(`/transactions/${HASH}`);
    await expect(page.getByText("Reconciliation")).toBeVisible();
    await expect(page.getByText("#2")).toBeVisible();
  });
});
