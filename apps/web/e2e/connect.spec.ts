import { test, expect } from "@playwright/test";

// No Freighter extension is installed in this test browser, so
// isFreighterInstalled() genuinely resolves false here — this exercises
// the real "wallet not installed" code path, not a mock of it.
test.describe("Wallet connection (no wallet extension installed)", () => {
  test("prompts to install Freighter and never claims a connection", async ({ page }) => {
    await page.goto("/connect");
    const main = page.getByRole("main");
    await expect(page.getByRole("heading", { name: "Connect your wallet" })).toBeVisible();
    await expect(main.getByRole("link", { name: "Install Freighter" })).toBeVisible();
    await expect(main.getByRole("button", { name: "Connect wallet" })).toHaveCount(0);
  });

  test("the header wallet button also reflects not-installed state", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("link", { name: "Install Freighter" })).toBeVisible();
  });
});
