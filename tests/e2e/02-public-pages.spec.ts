import { test, expect } from "@playwright/test";

test.describe("Public Pages", () => {
  test("login page loads", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("text=Sign in to ClientRoll")).toBeVisible();
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button:has-text("Sign in")')).toBeVisible();
  });

  test("login page has registration link", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("text=Create one")).toBeVisible();
  });

  test("login page has forgot password link", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("text=Forgot password?")).toBeVisible();
  });

  test("register page loads with two steps", async ({ page }) => {
    await page.goto("/register");
    await expect(page.locator("text=Create your account")).toBeVisible();
    await expect(page.locator("text=Company name")).toBeVisible();
    await expect(page.locator("text=URL")).toBeVisible();
    await expect(page.locator("text=Timezone")).toBeVisible();
  });

  test("forgot password page loads", async ({ page }) => {
    await page.goto("/forgot-password");
    await expect(page.locator("text=Reset your password")).toBeVisible({ timeout: 5000 });
    await expect(page.locator('input[type="email"]')).toBeVisible();
  });

  test("unauthenticated routes redirect to login", async ({ page }) => {
    await page.goto("/dashboard");
    await page.waitForURL("**/login");
    await expect(page.locator("text=Sign in to ClientRoll")).toBeVisible();
  });
});
