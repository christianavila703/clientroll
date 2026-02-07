import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Clients", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/clients`);
  });

  test("page loads with title", async ({ page }) => {
    await expect(page.locator("text=Clients")).toBeVisible();
  });

  test("shows client list with demo data", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible();
    await expect(page.locator("text=Emma Thompson")).toBeVisible();
  });

  test("shows result count", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=results")).toBeVisible();
  });

  test("search filters clients", async ({ page }) => {
    await page.waitForTimeout(1000);
    await page.fill('input[placeholder*="Search"]', "Sarah");
    await page.keyboard.press("Enter");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible();
  });

  test("status tabs exist", async ({ page }) => {
    await expect(page.locator("button:has-text('All')")).toBeVisible();
    await expect(page.locator("button:has-text('Active')")).toBeVisible();
    await expect(page.locator("button:has-text('Inactive')")).toBeVisible();
  });

  test("new client button opens form", async ({ page }) => {
    await page.click("text=New client");
    await expect(page.locator("text=New client").nth(1)).toBeVisible();
    await expect(page.locator("text=Personal information")).toBeVisible();
  });

  test("new client form has Details and Company tabs", async ({ page }) => {
    await page.click("text=New client");
    await expect(page.locator("button:has-text('Details')")).toBeVisible();
    await expect(page.locator("button:has-text('Company')")).toBeVisible();
  });

  test("three-dot menu opens with options", async ({ page }) => {
    await page.waitForTimeout(2000);
    const menuBtn = page.locator("table tbody tr").first().locator("button").last();
    await menuBtn.click();
    await expect(page.locator("text=Edit")).toBeVisible();
    await expect(page.locator("text=View profile")).toBeVisible();
    await expect(page.locator("text=Delete")).toBeVisible();
  });

  test("clicking client row opens edit form", async ({ page }) => {
    await page.waitForTimeout(2000);
    await page.locator("table tbody tr").first().click();
    await expect(page.locator("text=Edit:")).toBeVisible();
  });

  test("?new=1 auto-opens create form", async ({ page }) => {
    await page.goto(`${BASE}/clients?new=1`);
    await expect(page.locator("text=Personal information")).toBeVisible({ timeout: 5000 });
  });

  test("export button exists", async ({ page }) => {
    await expect(page.locator("text=Export")).toBeVisible();
  });
});
