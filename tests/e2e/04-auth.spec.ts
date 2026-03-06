import { test, expect } from "@playwright/test";
import { getE2EContext, loginViaForm } from "./helpers";

test.describe("Authentication", () => {
  test("login with invalid credentials shows error", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[type="email"]', "wrong@email.com");
    await page.fill('input[type="password"]', "wrongpassword1A");
    await page.click('button:has-text("Log in")');
    await expect(page.locator("text=Invalid email or password")).toBeVisible({ timeout: 10000 });
  });

  test("login with valid credentials redirects to dashboard", async ({ page }) => {
    await loginViaForm(page);
    await expect(page.locator("text=Overview")).toBeVisible();
  });

  test("login API returns tokens", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const res = await request.post("/api/auth/login", {
      headers: { "Content-Type": "application/json" },
      data: JSON.stringify({ email: ctx.email, password: ctx.password }),
    });
    const body = await res.json();
    if (!res.ok()) {
      throw new Error(`Login API failed: ${res.status()} ${JSON.stringify(body)}`);
    }
    expect(body.tokens.access_token).toBeTruthy();
    expect(body.tokens.refresh_token).toBeTruthy();
    expect(body.user.email).toBe(ctx.email);
    expect(body.user.role).toBe("tenant_admin");
  });

  test("refresh token rotation works", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const loginRes = await request.post("/api/auth/login", {
      headers: { "Content-Type": "application/json" },
      data: JSON.stringify({ email: ctx.email, password: ctx.password }),
    });
    const body = await loginRes.json();
    const tokens = body.tokens;
    if (!tokens?.refresh_token) {
      throw new Error(`Login failed: ${JSON.stringify(body)}`);
    }

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
