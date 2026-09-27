import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

// Automated axe-core scans catch a meaningful subset of WCAG 2.2 AA
// issues (contrast, labeling, landmarks, ARIA misuse) but not everything
// — they do not replace manual keyboard/screen-reader review. These run
// against pages that render without a connected wallet or backend data,
// which is every page reachable without one.
test.describe("Accessibility (automated axe scan)", () => {
  for (const path of ["/", "/connect", "/provider", "/sponsor"]) {
    test(`${path} has no detectable WCAG 2.0/2.1 A/AA violations`, async ({ page }) => {
      await page.goto(path);
      const results = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
        .analyze();
      expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
    });
  }
});
