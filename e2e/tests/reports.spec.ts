import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Reports", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/reports`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Reports')")).toBeVisible();
    await expect(page.locator("text=Financial analytics")).toBeVisible();
  });

  test("shows monthly revenue chart card", async ({ page }) => {
    await expect(page.locator("text=Monthly Revenue")).toBeVisible();
  });

  test("shows membership breakdown card", async ({ page }) => {
    await expect(page.locator("text=Revenue by Membership Type")).toBeVisible();
  });

  test("shows payment status card", async ({ page }) => {
    await expect(page.locator("text=Payment Status")).toBeVisible();
  });
});
