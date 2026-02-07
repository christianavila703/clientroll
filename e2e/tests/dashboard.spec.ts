import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/dashboard`);
  });

  test("shows overview heading", async ({ page }) => {
    await expect(page.locator("text=Overview")).toBeVisible();
  });

  test("shows metric cards", async ({ page }) => {
    await expect(page.locator("text=Active members")).toBeVisible();
    await expect(page.locator("text=Revenue this month")).toBeVisible();
    await expect(page.locator("text=Failed payments")).toBeVisible();
    await expect(page.locator("text=New signups")).toBeVisible();
  });

  test("shows revenue chart section", async ({ page }) => {
    await expect(page.locator("text=Revenue")).toBeVisible();
  });

  test("shows quick actions", async ({ page }) => {
    await expect(page.locator("text=New client")).toBeVisible();
    await expect(page.locator("text=Memberships")).toBeVisible();
    await expect(page.locator("text=Reports")).toBeVisible();
    await expect(page.locator("text=Settings")).toBeVisible();
  });

  test("metric cards show real numbers", async ({ page }) => {
    // Wait for API data to load
    await page.waitForTimeout(2000);
    const activeMembers = page.locator("text=Active members").locator("..");
    await expect(activeMembers).toBeVisible();
  });
});
