import { defineConfig, devices } from "@playwright/test";

// These E2E tests run against a plain Chromium/Firefox/WebKit instance with
// no Freighter extension installed and no real backend or Testnet RPC
// reachable at the configured URLs. They verify this app's own UI behavior
// (rendering, navigation, accessibility, form validation, and — where a
// test mocks the API — response handling) using Playwright's route
// interception. None of this is, or is claimed to be, live Testnet
// verification, real wallet signing, or real settlement: see the root
// README's "Testing" section for what these tests do and do not cover,
// and Phase 10 for actual live-network verification.
const PORT = 4173;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `pnpm run build && pnpm exec next start -p ${PORT}`,
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: {
      // Deliberately unreachable/placeholder values — no test in this
      // suite depends on a real backend or Testnet RPC responding; any
      // test that needs a response mocks it with page.route.
      NEXT_PUBLIC_STELLAR_NETWORK: "TESTNET",
      NEXT_PUBLIC_STELLAR_RPC_URL: "https://soroban-testnet.stellar.org",
      NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE: "Test SDF Network ; September 2015",
      NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID: "C" + "A".repeat(55),
      NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID: "C" + "B".repeat(55),
      NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID: "C" + "D".repeat(55),
      NEXT_PUBLIC_API_BASE_URL: "http://127.0.0.1:4010",
    },
  },
});
