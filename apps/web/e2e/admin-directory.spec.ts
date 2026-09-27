import { test, expect } from "@playwright/test";

// These tests mock the backend API with Playwright route interception —
// they verify this app's own rendering and pagination logic against a
// controlled response shape, not real off-chain mirror data and not any
// live chain state. No assertion here claims otherwise.

test.describe("Admin provider directory (mocked API)", () => {
  test("renders a page of providers and paginates via Load more", async ({ page }) => {
    await page.route("**/api/v1/providers*", async (route) => {
      const url = new URL(route.request().url());
      const cursor = url.searchParams.get("cursor");
      if (!cursor) {
        await route.fulfill({
          json: {
            providers: [
              { walletAddress: "G" + "A".repeat(55), providerRef: "aa".repeat(32), status: "Active", updatedAt: "2026-01-01T00:00:00Z" },
            ],
            nextCursor: "1",
          },
        });
        return;
      }
      await route.fulfill({
        json: {
          providers: [
            { walletAddress: "G" + "B".repeat(55), providerRef: "bb".repeat(32), status: "Active", updatedAt: "2026-01-01T00:00:00Z" },
          ],
        },
      });
    });

    await page.goto("/admin/providers");
    await expect(page.getByText("G" + "A".repeat(55))).toBeVisible();
    await expect(page.getByRole("button", { name: "Load more" })).toBeVisible();

    await page.getByRole("button", { name: "Load more" }).click();
    await expect(page.getByText("G" + "B".repeat(55))).toBeVisible();
    await expect(page.getByRole("button", { name: "Load more" })).toHaveCount(0);
  });

  test("shows an empty state with no providers", async ({ page }) => {
    await page.route("**/api/v1/providers*", async (route) => {
      await route.fulfill({ json: { providers: [] } });
    });
    await page.goto("/admin/providers");
    await expect(page.getByText("No providers registered yet.")).toBeVisible();
  });

  test("surfaces an error state when the API call fails", async ({ page }) => {
    await page.route("**/api/v1/providers*", async (route) => {
      await route.fulfill({ status: 500, json: { kind: "internal_error", message: "internal error" } });
    });
    await page.goto("/admin/providers");
    await expect(page.getByRole("alert")).toBeVisible();
  });
});
