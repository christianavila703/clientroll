import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Active Memberships", () => {
  test("active memberships page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Active Members");
    await expect(page.locator("h1:has-text('Memberships')")).toBeVisible();
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible({ timeout: 5000 });
  });

  test("tabs filter by status", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Active Members");
    await page.waitForSelector("text=Sarah Mitchell");
    // Switch to expired tab
    await page.click("button:has-text('Expired')");
    await expect(page.locator("text=Grace Patel")).toBeVisible({ timeout: 5000 });
  });

  test("shows payment progress bars", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Active Members");
    await page.waitForSelector("text=Sarah Mitchell");
    // Progress bars should be visible (they're divs with bg-* classes)
    await expect(page.locator(".bg-emerald-500, .bg-indigo-500, .bg-amber-500").first()).toBeVisible();
  });

  test("API returns active memberships with payment data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/active-memberships?status=active", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBeGreaterThan(0);
    const first = body.memberships[0];
    expect(first.client_name).toBeTruthy();
    expect(first.membership_type).toBeTruthy();
    expect(typeof first.total_paid).toBe("number");
    expect(typeof first.total_expected).toBe("number");
    expect(first.end_date).toBeTruthy();
  });
});
