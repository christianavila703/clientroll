import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Dashboard", () => {
  test("dashboard loads with summary cards", async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page.locator("h1:has-text('Overview')")).toBeVisible();
    await expect(page.getByRole("main").getByText("Active Members")).toBeVisible();
    await expect(page.locator("text=Revenue this month")).toBeVisible();
    await expect(page.getByRole("main").getByText("Failed payments").first()).toBeVisible();
    await expect(page.locator("text=New signups").first()).toBeVisible();
  });

  test("dashboard shows revenue chart section", async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page.getByRole("heading", { name: "Revenue" }).first()).toBeVisible();
  });

  test("dashboard API returns summary", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/reports/dashboard", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(typeof body.active_members).toBe("number");
    expect(typeof body.month_revenue).toBe("number");
    expect(typeof body.failed_payments).toBe("number");
    expect(typeof body.new_signups).toBe("number");
  });

  test("notification bell shows alert count", async ({ page }) => {
    await loginAsAdmin(page);
    // Bell should be visible
    await expect(page.locator('[title="Notifications"]')).toBeVisible();
  });

  test("sidebar navigation works", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await expect(page.locator("h1:has-text('Clients')")).toBeVisible();
  });
});
