import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Navigation & Layout", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/dashboard`);
  });

  test("sidebar shows all nav items", async ({ page }) => {
    await expect(page.locator("text=Home")).toBeVisible();
    await expect(page.locator("text=Clients")).toBeVisible();
    await expect(page.locator("text=Active Members")).toBeVisible();
    await expect(page.locator("text=Membership Types")).toBeVisible();
    await expect(page.locator("text=Bonuses")).toBeVisible();
    await expect(page.locator("text=Terms")).toBeVisible();
    await expect(page.locator("text=Payments")).toBeVisible();
    await expect(page.locator("text=Reports")).toBeVisible();
    await expect(page.locator("text=Team")).toBeVisible();
    await expect(page.locator("text=Settings")).toBeVisible();
  });

  test("sidebar shows ClientRoll branding", async ({ page }) => {
    await expect(page.locator("text=ClientRoll")).toBeVisible();
  });

  test("sidebar shows user info", async ({ page }) => {
    await expect(page.locator("text=Ben")).toBeVisible();
    await expect(page.locator("text=Admin")).toBeVisible();
  });

  test("create new dropdown works", async ({ page }) => {
    await page.click("text=Create new");
    await expect(page.locator("text=Add to your client list")).toBeVisible();
    await expect(page.locator("text=Define a new offering")).toBeVisible();
    await expect(page.locator("text=Create a signup bonus")).toBeVisible();
  });

  test("header search bar exists", async ({ page }) => {
    await expect(page.locator('input[placeholder*="Search"]').first()).toBeVisible();
  });

  test("notification bell exists", async ({ page }) => {
    await expect(page.locator('button[title="Notifications"]')).toBeVisible();
  });

  test("notification bell shows alerts", async ({ page }) => {
    await page.click('button[title="Notifications"]');
    await expect(page.locator("text=Notifications")).toBeVisible();
  });

  test("sidebar navigation works", async ({ page }) => {
    await page.click("text=Clients");
    await expect(page.locator("h1:has-text('Clients')")).toBeVisible();

    await page.click("text=Bonuses");
    await expect(page.locator("h1:has-text('Bonuses')")).toBeVisible();

    await page.click("text=Reports");
    await expect(page.locator("h1:has-text('Reports')")).toBeVisible();
  });

  test("unauthenticated redirects to login", async ({ page }) => {
    await page.evaluate(() => {
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
    });
    await page.goto(`${BASE}/dashboard`);
    await expect(page).toHaveURL(/\/login/);
  });
});
