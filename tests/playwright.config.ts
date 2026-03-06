import { defineConfig } from "@playwright/test";

const browserbaseConnectUrl = process.env.BROWSERBASE_CONNECT_URL;
const reportOutputPath = process.env.PW_JSON_REPORT_PATH || "test-results/raw-report.json";
const browserbaseConnectTimeout = Number(process.env.BROWSERBASE_CONNECT_TIMEOUT_MS || 120000);

export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  expect: { timeout: 10000 },
  fullyParallel: false,
  retries: 1,
  workers: 1,
  reporter: [["list"], ["json", { outputFile: reportOutputPath }]],
  use: {
    baseURL: "https://app.clientroll.com",
    headless: true,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
    ...(browserbaseConnectUrl
      ? {
          connectOptions: {
            wsEndpoint: browserbaseConnectUrl,
            timeout: Number.isFinite(browserbaseConnectTimeout) ? browserbaseConnectTimeout : 120000,
          },
        }
      : {}),
  },
  projects: [
    { name: "chromium", use: { browserName: "chromium" } },
  ],
});
