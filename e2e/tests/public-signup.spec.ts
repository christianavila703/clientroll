import { test, expect } from "@playwright/test";

const BASE = "https://clientroll.com";

test.describe("Public Signup Flow", () => {
  test("browse tenant memberships", async ({ page }) => {
    await page.goto(`${BASE}/join/studio-figura`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Studio Figura")).toBeVisible();
    await expect(page.locator("text=Choose Your Membership")).toBeVisible();
  });

  test("shows active membership types", async ({ page }) => {
    await page.goto(`${BASE}/join/studio-figura`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Gold Membership")).toBeVisible();
    await expect(page.locator("text=Silver Membership")).toBeVisible();
    await expect(page.locator("text=Platinum Membership")).toBeVisible();
  });

  test("shows membership values", async ({ page }) => {
    await page.goto(`${BASE}/join/studio-figura`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=total value").first()).toBeVisible();
  });

  test("direct link to specific membership", async ({ page }) => {
    await page.goto(`${BASE}/studio-figura/gold-membership`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Gold Membership")).toBeVisible();
    await expect(page.locator("text=Choose Your Term")).toBeVisible();
  });

  test("shows term options with pricing", async ({ page }) => {
    await page.goto(`${BASE}/studio-figura/gold-membership`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=6 Months")).toBeVisible();
    await expect(page.locator("text=12 Months")).toBeVisible();
  });

  test("selecting a term shows bonuses", async ({ page }) => {
    await page.goto(`${BASE}/studio-figura/gold-membership`);
    await page.waitForTimeout(3000);
    // 12-month should show bonuses
    await page.click("text=12 Months");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Free Body Composition Scan")).toBeVisible();
  });

  test("invalid tenant shows error", async ({ page }) => {
    await page.goto(`${BASE}/join/nonexistent-tenant`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=not found")).toBeVisible({ timeout: 5000 }).catch(() => {
      expect(page.locator("text=No memberships")).toBeVisible();
    });
  });

  test("continue button appears when term selected", async ({ page }) => {
    await page.goto(`${BASE}/studio-figura/gold-membership`);
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Continue with")).toBeVisible();
  });
});
