import { test, expect } from "@playwright/test";

test.describe("Mobile navigation", () => {
  test.use({ viewport: { width: 375, height: 700 } });

  test("hamburger toggle opens and closes the mobile menu with correct aria state", async ({ page }) => {
    await page.goto("/");
    const toggle = page.getByRole("button", { name: "Open menu" });
    await expect(toggle).toHaveAttribute("aria-expanded", "false");

    await toggle.click();
    await expect(page.getByRole("button", { name: "Close menu" })).toHaveAttribute("aria-expanded", "true");

    const mobileNav = page.locator("#primary-nav-mobile");
    await expect(mobileNav.getByRole("link", { name: "For providers" })).toBeVisible();
    await expect(mobileNav.getByRole("link", { name: "For sponsors" })).toBeVisible();
    await expect(mobileNav.getByRole("link", { name: "Provider directory" })).toBeVisible();

    await page.getByRole("button", { name: "Close menu" }).click();
    await expect(page.locator("#primary-nav-mobile")).toHaveCount(0);
  });

  test("choosing a mobile nav link navigates and closes the menu", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Open menu" }).click();
    await page.locator("#primary-nav-mobile").getByRole("link", { name: "For providers" }).click();
    await expect(page).toHaveURL(/\/provider$/);
    await expect(page.locator("#primary-nav-mobile")).toHaveCount(0);
  });

  test("is keyboard-operable without a pointer", async ({ page }) => {
    await page.goto("/");
    const toggle = page.getByRole("button", { name: "Open menu" });
    await toggle.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("button", { name: "Close menu" })).toHaveAttribute("aria-expanded", "true");
  });
});
