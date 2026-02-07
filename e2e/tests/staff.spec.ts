import { test, expect } from "@playwright/test";
import { login, BASE } from "./helpers";

test.describe("Staff / Team", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await page.goto(`${BASE}/staff`);
  });

  test("page loads", async ({ page }) => {
    await expect(page.locator("h1:has-text('Staff')")).toBeVisible();
  });

  test("shows current user (Ben)", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Ben")).toBeVisible();
    await expect(page.locator("text=ben@businessnet.com.au")).toBeVisible();
  });

  test("add staff button exists for admin", async ({ page }) => {
    await expect(page.locator("text=Add Staff")).toBeVisible();
  });

  test("add staff form opens", async ({ page }) => {
    await page.click("text=Add Staff");
    await expect(page.locator("text=Add Staff Member")).toBeVisible();
    await expect(page.locator("text=First Name")).toBeVisible();
    await expect(page.locator("text=Email")).toBeVisible();
    await expect(page.locator("text=Role")).toBeVisible();
  });

  test("shows role badges", async ({ page }) => {
    await page.waitForTimeout(2000);
    await expect(page.locator("text=Admin").first()).toBeVisible();
  });
});
