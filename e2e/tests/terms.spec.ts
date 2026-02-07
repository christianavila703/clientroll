import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Terms & Conditions", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/terms`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Terms & Conditions')")).toBeVisible();
  });

  test("shows demo T&Cs", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Standard Membership Agreement")).toBeVisible();
    await expect(page.locator("text=Direct Debit Agreement")).toBeVisible();
  });

  test("shows table with Name and Last Updated", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("th:has-text('Name')")).toBeVisible();
    await expect(page.locator("th:has-text('Last Updated')")).toBeVisible();
  });

  test("active badge shown", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Active").first()).toBeVisible();
  });

  test("new terms button opens form", async ({ page }) => {
    await page.click("text=New terms");
    await expect(page.locator("text=New Terms & Conditions")).toBeVisible();
    await expect(page.locator("text=Content")).toBeVisible();
  });

  test("clicking row opens edit form", async ({ page }) => {
    await page.waitForTimeout(2000);
    await page.locator("table tbody tr").first().click();
    await expect(page.locator("text=Edit:")).toBeVisible();
  });

  test("search works", async ({ page }) => {
    await page.fill('input[placeholder*="Search"]', "Direct");
    await expect(page.locator("text=Direct Debit Agreement")).toBeVisible();
  });
});
