import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Active Memberships", () => {
  test("active memberships page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Active Members");
    await expect(page.locator("h1:has-text('Memberships')")).toBeVisible({ timeout: 10000 });
  });

  test("tabs exist for status filter", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Active Members");
    await expect(page.locator("button:has-text('Active')")).toBeVisible();
    await expect(page.locator("button:has-text('All')")).toBeVisible();
  });

  test("API returns active memberships with payment data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/active-memberships?status=active", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Number.isInteger(body.count)).toBeTruthy();
    const list = body.memberships ?? body.data ?? [];
    expect(Array.isArray(list)).toBeTruthy();
    if (list.length > 0) {
      const first = list[0];
      expect(first.client_name).toBeTruthy();
      expect(first.membership_type).toBeTruthy();
      expect(typeof first.total_paid).toBe("number");
      expect(typeof first.total_expected).toBe("number");
      expect(first.end_date).toBeTruthy();
    }
  });
});
