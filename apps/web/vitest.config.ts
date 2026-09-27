import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Playwright owns e2e/**/*.spec.ts — vitest's default include glob
    // would otherwise also pick those up and fail importing
    // @playwright/test outside its own runner.
    include: ["**/*.test.ts"],
    exclude: ["e2e/**", "node_modules/**", ".next/**"],
  },
});
