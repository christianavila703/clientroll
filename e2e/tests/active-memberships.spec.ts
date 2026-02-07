import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Active Memberships", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/active-memberships`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Memberships')")).toBeVisible();
  });

  test("tabs exist", async ({ page }) => {
    await expect(page.locator("button:has-text('Active')")).toBeVisible();
    await expect(page.locator("button:has-text('Expired')")).toBeVisible();
    await expect(page.locator("button:has-text('Cancelled')")).toBeVisible();
    await expect(page.locator("button:has-text('All')")).toBeVisible();
  });

  test("shows active memberships with demo data", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible();
    await expect(page.locator("text=Gold Membership").first()).toBeVisible();
  });

  test("shows payment columns", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("th:has-text('Payment')")).toBeVisible();
    await expect(page.locator("th:has-text('Last Payment')")).toBeVisible();
    await expect(page.locator("th:has-text('Ends')")).toBeVisible();
  });

  test("shows progress bar for paid/expected", async ({ page }) => {
    await page.waitForTimeout(2000);
    // Progress bars are rendered as divs with dynamic width
    const progressBars = page.locator(".rounded-full.overflow-hidden .h-full");
    await expect(progressBars.first()).toBeVisible();
  });

  test("expired tab shows expired memberships", async ({ page }) => {
    await page.click("button:has-text('Expired')");
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Grace Patel")).toBeVisible();
  });

  test("shows failed payment badges", async ({ page }) => {
    await page.waitForTimeout(2000);
    const failedBadge = page.locator("text=failed").first();
    await expect(failedBadge).toBeVisible();
  });

  test("search filters by client name", async ({ page }) => {
    await page.waitForTimeout(1000);
    await page.fill('input[placeholder*="Search"]', "Jessica");
    await expect(page.locator("text=Jessica Liu")).toBeVisible();
  });
});
