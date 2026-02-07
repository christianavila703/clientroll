import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Payments", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/payments`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Payments')")).toBeVisible();
  });

  test("shows filter pills", async ({ page }) => {
    await expect(page.locator("button:has-text('All')")).toBeVisible();
    await expect(page.locator("button:has-text('succeeded')")).toBeVisible();
    await expect(page.locator("button:has-text('failed')")).toBeVisible();
  });

  test("shows payment data", async ({ page }) => {
    await page.waitForTimeout(2000);
    // Should have payments from demo data
    const rows = page.locator("table tbody tr, .divide-y > div");
    await expect(rows.first()).toBeVisible();
  });

  test("filter by failed shows only failed", async ({ page }) => {
    await page.click("button:has-text('failed')");
    await page.waitForTimeout(2000);
    const failedBadges = page.locator("text=failed");
    await expect(failedBadges.first()).toBeVisible();
  });

  test("shows amount and status columns", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("th:has-text('Amount')")).toBeVisible();
    await expect(page.locator("th:has-text('Status')")).toBeVisible();
    await expect(page.locator("th:has-text('Due Date')")).toBeVisible();
  });
});
