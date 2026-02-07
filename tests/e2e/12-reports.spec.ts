import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Reports", () => {
  test("reports page loads with charts", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Reports");
    await expect(page.locator("text=Monthly Revenue")).toBeVisible();
    await expect(page.locator("text=Revenue by Membership Type")).toBeVisible();
    await expect(page.locator("text=Payment Status")).toBeVisible();
  });

  test("monthly revenue API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/reports/revenue/monthly", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.months).toBeDefined();
  });

  test("membership breakdown API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/reports/memberships/breakdown", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.breakdown).toBeDefined();
    expect(body.breakdown.length).toBeGreaterThan(0);
  });

  test("payment status API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/reports/payments/status", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.statuses).toBeDefined();
    expect(body.statuses.succeeded).toBeGreaterThan(0);
  });
});
