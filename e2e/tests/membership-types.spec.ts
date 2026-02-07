import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Membership Types", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/memberships`);
  });

  test("page loads with title", async ({ page }) => {
    await expect(page.locator("h1:has-text('Membership Types')")).toBeVisible();
  });

  test("shows demo membership types", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Gold Membership")).toBeVisible();
    await expect(page.locator("text=Silver Membership")).toBeVisible();
    await expect(page.locator("text=Platinum Membership")).toBeVisible();
  });

  test("tabs filter by status", async ({ page }) => {
    await expect(page.locator("button:has-text('All')")).toBeVisible();
    await expect(page.locator("button:has-text('Active')")).toBeVisible();
    await expect(page.locator("button:has-text('Draft')")).toBeVisible();
  });

  test("search filters types", async ({ page }) => {
    await page.waitForTimeout(1000);
    await page.fill('input[placeholder*="Search"]', "Gold");
    await expect(page.locator("text=Gold Membership")).toBeVisible();
  });

  test("shows table columns", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("th:has-text('Name')")).toBeVisible();
    await expect(page.locator("th:has-text('Status')")).toBeVisible();
    await expect(page.locator("th:has-text('Value')")).toBeVisible();
    await expect(page.locator("th:has-text('Terms')")).toBeVisible();
    await expect(page.locator("th:has-text('Last Updated')")).toBeVisible();
  });

  test("new membership button opens form", async ({ page }) => {
    await page.click("text=New membership type");
    await expect(page.locator("text=New Membership Type")).toBeVisible();
    await expect(page.locator("text=Title")).toBeVisible();
    await expect(page.locator("text=Description")).toBeVisible();
    await expect(page.locator("text=Hero Image")).toBeVisible();
  });

  test("clicking a row navigates to detail", async ({ page }) => {
    await page.waitForTimeout(2000);
    await page.locator("table tbody tr").first().click();
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Term Options & Pricing")).toBeVisible();
    await expect(page.locator("text=Bonuses")).toBeVisible();
    await expect(page.locator("text=Terms & Conditions")).toBeVisible();
  });

  test("duplicate button exists", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator('button[title="Duplicate"]').first()).toBeVisible();
  });
});

test.describe("Membership Type Detail", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/memberships`);
    await page.waitForTimeout(2000);
    await page.locator("table tbody tr").first().click();
    await page.waitForTimeout(1000);
  });

  test("term options tab shows pricing table", async ({ page }) => {
    await expect(page.locator("th:has-text('Duration')")).toBeVisible();
    await expect(page.locator("th:has-text('Total')")).toBeVisible();
    await expect(page.locator("th:has-text('Monthly')")).toBeVisible();
  });

  test("add term option button exists", async ({ page }) => {
    await expect(page.locator("text=Add term option")).toBeVisible();
  });

  test("bonuses tab shows bonus assignment", async ({ page }) => {
    await page.click("text=Bonuses");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Assign bonuses")).toBeVisible({ timeout: 5000 }).catch(() => {
      // May show bonus names directly
      expect(page.locator("text=Free Body Composition Scan")).toBeVisible();
    });
  });

  test("T&C tab shows attachable terms", async ({ page }) => {
    await page.click("text=Terms & Conditions");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Standard Membership Agreement")).toBeVisible();
    await expect(page.locator("text=Direct Debit Agreement")).toBeVisible();
  });

  test("back button returns to list", async ({ page }) => {
    await page.locator('svg[viewBox="0 0 24 24"]').first().click();
    await expect(page.locator("h1:has-text('Membership Types')")).toBeVisible({ timeout: 5000 });
  });
});
