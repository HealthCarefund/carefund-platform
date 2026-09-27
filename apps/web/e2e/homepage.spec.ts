import { test, expect } from "@playwright/test";

test.describe("Homepage", () => {
  test("renders the headline and both primary calls to action", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Fund care agreements");
    await expect(page.getByRole("link", { name: "Sponsor a care agreement" })).toBeVisible();
    await expect(page.getByRole("link", { name: "I’m a provider" })).toBeVisible();
  });

  test("has a working skip-to-content link", async ({ page }) => {
    await page.goto("/");
    const skipLink = page.getByRole("link", { name: "Skip to main content" });
    await skipLink.focus();
    await expect(skipLink).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/#main-content$/);
  });

  test("desktop primary navigation links to provider, sponsor, and directory pages", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/");
    const nav = page.getByRole("navigation", { name: "Primary" });
    await expect(nav.getByRole("link", { name: "For providers" })).toHaveAttribute("href", "/provider");
    await expect(nav.getByRole("link", { name: "For sponsors" })).toHaveAttribute("href", "/sponsor");
    await expect(nav.getByRole("link", { name: "Provider directory" })).toHaveAttribute(
      "href",
      "/admin/providers",
    );
  });
});
