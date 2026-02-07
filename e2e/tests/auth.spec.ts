import { test, expect } from "@playwright/test";

const BASE = "https://clientroll.com";
const EMAIL = "ben@businessnet.com.au";
const PASSWORD = "TestPassword1!";

test.describe("Authentication", () => {
  test("login page loads", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await expect(page.locator("text=Sign in")).toBeVisible();
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator("text=Sign in")).toBeVisible();
  });

  test("shows error on invalid credentials", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.fill('input[type="email"]', "wrong@email.com");
    await page.fill('input[type="password"]', "wrongpassword");
    await page.click('button:has-text("Sign in")');
    await expect(page.locator("text=Invalid email or password")).toBeVisible({ timeout: 5000 });
  });

  test("forgot password page loads", async ({ page }) => {
    await page.goto(`${BASE}/forgot-password`);
    await expect(page.locator("text=Reset your password")).toBeVisible();
    await expect(page.locator('input[type="email"]')).toBeVisible();
  });

  test("forgot password submits", async ({ page }) => {
    await page.goto(`${BASE}/forgot-password`);
    await page.fill('input[type="email"]', EMAIL);
    await page.click('button:has-text("Send reset link")');
    await expect(page.locator("text=Check your email")).toBeVisible({ timeout: 5000 });
  });

  test("register page loads with two steps", async ({ page }) => {
    await page.goto(`${BASE}/register`);
    await expect(page.locator("text=Create your account")).toBeVisible();
    await expect(page.locator("text=Company name")).toBeVisible();
    await expect(page.locator("text=URL")).toBeVisible();
  });

  test("register slug check shows availability", async ({ page }) => {
    await page.goto(`${BASE}/register`);
    await page.fill('input[placeholder="Acme Wellness"]', "Test Company Unique 12345");
    await expect(page.locator("text=Available")).toBeVisible({ timeout: 5000 });
  });

  test("register slug check shows taken", async ({ page }) => {
    await page.goto(`${BASE}/register`);
    await page.fill('input[placeholder="Acme Wellness"]', "Studio Figura");
    await expect(page.locator("text=Already taken")).toBeVisible({ timeout: 5000 });
  });

  test("platform admin login link exists", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await expect(page.locator("text=Platform admin")).toBeVisible();
  });

  test("login page links to register", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await expect(page.locator("text=Create one")).toBeVisible();
  });

  test("reset password page rejects missing token", async ({ page }) => {
    await page.goto(`${BASE}/reset-password`);
    await expect(page.locator("text=Invalid reset link")).toBeVisible();
  });
});
