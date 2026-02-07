import { test, expect } from "@playwright/test";

test.describe("Authentication", () => {
  test("login with invalid credentials shows error", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[type="email"]', "wrong@email.com");
    await page.fill('input[type="password"]', "wrongpassword1A");
    await page.click('button:has-text("Sign in")');
    await expect(page.locator("text=Invalid email or password")).toBeVisible();
  });

  test("login with valid credentials redirects to dashboard", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[type="email"]', "ben@businessnet.com.au");
    await page.fill('input[type="password"]', "Clientroll1Test");
    await page.click('button:has-text("Sign in")');
    await page.waitForURL("**/dashboard", { timeout: 15000 });
    await expect(page.locator("text=Overview")).toBeVisible();
  });

  test("login API returns tokens", async ({ request }) => {
    const res = await request.post("/api/auth/login", {
      data: { email: "ben@businessnet.com.au", password: "Clientroll1Test" },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.tokens.access_token).toBeTruthy();
    expect(body.tokens.refresh_token).toBeTruthy();
    expect(body.user.email).toBe("ben@businessnet.com.au");
    expect(body.user.role).toBe("tenant_admin");
  });

  test("refresh token rotation works", async ({ request }) => {
    const loginRes = await request.post("/api/auth/login", {
      data: { email: "ben@businessnet.com.au", password: "Clientroll1Test" },
    });
    const { tokens } = await loginRes.json();

    const refreshRes = await request.post("/api/auth/refresh", {
      data: { refresh_token: tokens.refresh_token },
    });
    expect(refreshRes.ok()).toBeTruthy();
    const newTokens = await refreshRes.json();
    expect(newTokens.access_token).toBeTruthy();
    expect(newTokens.access_token).not.toBe(tokens.access_token);
  });

  test("forgot password returns success (no enumeration)", async ({ request }) => {
    const res = await request.post("/api/auth/forgot-password", {
      data: { email: "nonexistent@email.com" },
    });
    expect(res.ok()).toBeTruthy();
  });
});
