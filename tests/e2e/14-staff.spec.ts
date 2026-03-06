import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Staff", () => {
  test("staff page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Team");
    await expect(page).toHaveURL(/\/staff/);
    await expect(page.getByRole("button", { name: /Add Staff/i })).toBeVisible();
  });

  test("add staff button shows form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Team");
    await page.click("button:has-text('Add Staff')");
    await expect(page.locator("text=Add Staff Member")).toBeVisible();
  });

  test("staff API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/staff", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBeGreaterThan(0);
  });
});
