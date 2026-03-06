import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Settings", () => {
  test("settings page loads with tabs", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await expect(page.getByRole("main")).toContainText("Agreement");
    await expect(page.getByRole("main")).toContainText("Organisation");
    await expect(page.getByRole("main")).toContainText("Integrations");
  });

  test("agreement details show company fields", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.waitForTimeout(1000);
    await expect(page.locator("text=Company Name")).toBeVisible();
    await expect(page.getByRole("main")).toContainText("Company");
  });

  test("organisation tab loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.click("button:has-text('Organisation')");
    await expect(page.locator("text=General")).toBeVisible();
    await expect(page.locator("text=Default Thank You Page")).toBeVisible();
    await expect(page.locator("text=Brand colour")).toBeVisible();
  });

  test("integrations tab shows GoCardless", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Settings");
    await page.click("button:has-text('Integrations')");
    await expect(page.getByRole("heading", { name: "GoCardless" })).toBeVisible();
    await expect(page.locator("text=Direct debit").first()).toBeVisible();
  });

  test("agreement settings API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/settings/agreement", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(typeof body).toBe("object");
  });

  test("integrations API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/integrations", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
  });
});
