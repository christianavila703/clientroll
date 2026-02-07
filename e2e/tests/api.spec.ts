import { test, expect } from "@playwright/test";

const BASE = "https://clientroll.com/api";

test.describe("API Health & Public Endpoints", () => {
  test("health check returns ok", async ({ request }) => {
    const res = await request.get(`${BASE}/health`);
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    expect(data.status).toBe("ok");
    expect(data.service).toBe("clientroll");
  });

  test("login rejects invalid credentials", async ({ request }) => {
    const res = await request.post(`${BASE}/auth/login`, {
      data: { email: "wrong@email.com", password: "wrongpassword" },
    });
    expect([401, 429]).toContain(res.status());
  });

  test("protected endpoints require auth", async ({ request }) => {
    const res = await request.get(`${BASE}/tenant/me`);
    expect(res.status()).toBe(401);
  });

  test("slug check returns availability", async ({ request }) => {
    const res = await request.get(`${BASE}/register/check-slug/test-unique-slug-xyz`);
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    expect(data).toHaveProperty("available");
    expect(data).toHaveProperty("slug");
  });

  test("slug check returns taken for existing", async ({ request }) => {
    const res = await request.get(`${BASE}/register/check-slug/studio-figura`);
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    expect(data.available).toBe(false);
  });

  test("public signup endpoint returns tenant memberships", async ({ request }) => {
    const res = await request.get(`${BASE}/signup/studio-figura`);
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    expect(data).toHaveProperty("tenant");
    expect(data).toHaveProperty("memberships");
    expect(data.tenant.name).toBe("Studio Figura");
  });

  test("public signup detail returns membership with terms", async ({ request }) => {
    const res = await request.get(`${BASE}/signup/studio-figura/gold-membership`);
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    expect(data.membership.title).toBe("Gold Membership");
    expect(data.membership.term_options.length).toBeGreaterThan(0);
  });

  test("forgot password returns success or rate limit", async ({ request }) => {
    const res = await request.post(`${BASE}/auth/forgot-password`, {
      data: { email: "doesntexist@email.com" },
    });
    expect([200, 429]).toContain(res.status());
  });

  test("reset password rejects invalid token", async ({ request }) => {
    const res = await request.post(`${BASE}/auth/reset-password`, {
      data: { token: "invalidtoken", password: "NewPassword123!" },
    });
    expect([400, 429]).toContain(res.status());
  });

  test("webhook event types endpoint exists", async ({ request }) => {
    // This is behind auth so should return 401
    const res = await request.get(`${BASE}/tenant/webhooks/event-types`);
    expect(res.status()).toBe(401);
  });
});
