import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Bonuses", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/bonuses`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Bonuses')")).toBeVisible();
  });

  test("shows demo bonuses", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Free Body Composition Scan")).toBeVisible();
    await expect(page.locator("text=Personal Training Session")).toBeVisible();
    await expect(page.locator("text=Nutrition Consultation")).toBeVisible();
    await expect(page.locator("text=Wellness Starter Pack")).toBeVisible();
    await expect(page.locator("text=Infrared Sauna Sessions")).toBeVisible();
  });

  test("shows table columns", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("th:has-text('Bonus')")).toBeVisible();
    await expect(page.locator("th:has-text('Value')")).toBeVisible();
    await expect(page.locator("th:has-text('Status')")).toBeVisible();
  });

  test("new bonus button opens form", async ({ page }) => {
    await page.click("text=New bonus");
    await expect(page.locator("text=New Bonus")).toBeVisible();
    await expect(page.locator("text=Title")).toBeVisible();
    await expect(page.locator("text=Description")).toBeVisible();
    await expect(page.locator("text=Image")).toBeVisible();
  });

  test("clicking a row opens edit form", async ({ page }) => {
    await page.waitForTimeout(2000);
    await page.locator("table tbody tr").first().click();
    await expect(page.locator("text=Edit:")).toBeVisible();
  });

  test("search filters bonuses", async ({ page }) => {
    await page.waitForTimeout(1000);
    await page.fill('input[placeholder*="Search"]', "Nutrition");
    await expect(page.locator("text=Nutrition Consultation")).toBeVisible();
  });

  test("?new=1 auto-opens form", async ({ page }) => {
    await page.goto(`${BASE}/bonuses?new=1`);
    await expect(page.locator("text=New Bonus")).toBeVisible({ timeout: 5000 });
  });

  test("image upload component shows in form", async ({ page }) => {
    await page.click("text=New bonus");
    await expect(page.locator("text=Click to upload image")).toBeVisible();
  });
});
