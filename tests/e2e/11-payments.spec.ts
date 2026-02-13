import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Payments", () => {
  test("payments page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Payments");
    await expect(page.locator("h1:has-text('Payments')")).toBeVisible({ timeout: 10000 });
    await expect(page.locator("button:has-text('All')")).toBeVisible();
    await expect(page.locator("button:has-text('succeeded')")).toBeVisible();
    await expect(page.locator("button:has-text('failed')")).toBeVisible();
  });

  test("payments API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/payments", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Number.isInteger(body.count)).toBeTruthy();
  });

  test("failed payments API works", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/payments/failed", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Number.isInteger(body.count)).toBeTruthy();
  });
});
