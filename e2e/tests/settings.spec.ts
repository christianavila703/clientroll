import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Settings", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/settings`);
  });

  test("page loads with tabs", async ({ page }) => {
    await expect(page.locator("h1:has-text('Settings')")).toBeVisible();
    await expect(page.locator("button:has-text('Agreement Details')")).toBeVisible();
    await expect(page.locator("button:has-text('Organisation')")).toBeVisible();
    await expect(page.locator("button:has-text('Integrations')")).toBeVisible();
  });

  test("agreement details shows company fields", async ({ page }) => {
    await expect(page.locator("text=Company Name")).toBeVisible();
    await expect(page.locator("text=ABN/ACN")).toBeVisible();
    await expect(page.locator("text=Trading Name")).toBeVisible();
  });

  test("agreement details has demo data", async ({ page }) => {
    await page.waitForTimeout(2000);
    const companyInput = page.locator('input').first();
    await expect(companyInput).toBeVisible();
  });

  test("organisation tab shows timezone and branding", async ({ page }) => {
    await page.click("button:has-text('Organisation')");
    await expect(page.locator("text=Timezone")).toBeVisible();
    await expect(page.locator("text=Brand colour")).toBeVisible();
  });

  test("organisation tab shows thank you page settings", async ({ page }) => {
    await page.click("button:has-text('Organisation')");
    await expect(page.locator("text=Default Thank You Page")).toBeVisible();
    await expect(page.locator("text=Heading")).toBeVisible();
    await expect(page.locator("text=Body text")).toBeVisible();
    await expect(page.locator("text=Redirect URL")).toBeVisible();
  });

  test("integrations tab shows GoCardless and MailerSend", async ({ page }) => {
    await page.click("button:has-text('Integrations')");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=GoCardless")).toBeVisible();
    await expect(page.locator("text=MailerSend")).toBeVisible();
  });

  test("integrations show connected/not configured badges", async ({ page }) => {
    await page.click("button:has-text('Integrations')");
    await page.waitForTimeout(1000);
    // At least one should show "Not configured"
    await expect(page.locator("text=Not configured").first()).toBeVisible();
  });

  test("integration configure button expands form", async ({ page }) => {
    await page.click("button:has-text('Integrations')");
    await page.waitForTimeout(1000);
    await page.click("text=Configure");
    await expect(page.locator("text=Environment")).toBeVisible();
  });
});
