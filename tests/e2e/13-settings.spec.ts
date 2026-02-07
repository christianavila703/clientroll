import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Settings", () => {
  test("settings page loads with tabs", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await expect(page.locator("text=Agreement Details")).toBeVisible();
    await expect(page.locator("text=Organisation")).toBeVisible();
    await expect(page.locator("text=Integrations")).toBeVisible();
  });

  test("agreement details show Studio Figura data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.waitForTimeout(1000);
    const companyInput = page.locator('input').first();
    await expect(companyInput).toHaveValue("Studio Figura AA PTY LTD");
  });

  test("organisation tab loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.click("button:has-text('Organisation')");
    await expect(page.locator("text=General")).toBeVisible();
    await expect(page.locator("text=Default Thank You Page")).toBeVisible();
    await expect(page.locator("text=Brand colour")).toBeVisible();
  });

  test("integrations tab shows GoCardless and MailerSend", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.click("button:has-text('Integrations')");
    await expect(page.locator("text=GoCardless")).toBeVisible();
    await expect(page.locator("text=MailerSend")).toBeVisible();
  });

  test("agreement settings API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/settings/agreement", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.company_name).toBe("Studio Figura AA PTY LTD");
    expect(body.abn_acn).toBe("31 642 633 234");
  });

  test("integrations API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/integrations", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
  });
});
