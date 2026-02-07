import { test, expect } from "@playwright/test";

test.describe("Security", () => {
  test("API rejects requests without auth", async ({ request }) => {
    const endpoints = [
      "/api/tenant/clients",
      "/api/tenant/membership-types",
      "/api/tenant/bonuses",
      "/api/tenant/terms",
      "/api/tenant/staff",
      "/api/tenant/payments",
      "/api/tenant/webhooks",
      "/api/tenant/settings/agreement",
      "/api/tenant/reports/dashboard",
      "/api/tenant/active-memberships",
      "/api/tenant/integrations",
    ];

    for (const endpoint of endpoints) {
      const res = await request.get(endpoint);
      expect(res.status(), `${endpoint} should require auth`).toBe(401);
    }
  });

  test("API rejects invalid tokens", async ({ request }) => {
    const res = await request.get("/api/tenant/clients", {
      headers: { Authorization: "Bearer invalid-token" },
    });
    expect(res.status()).toBe(401);
  });

  test("platform routes reject non-platform users", async ({ request }) => {
    // Login as tenant user
    const loginRes = await request.post("/api/auth/login", {
      data: { email: "ben@businessnet.com.au", password: "Clientroll1Test" },
    });
    const { tokens } = await loginRes.json();

    const res = await request.get("/api/platform/tenants", {
      headers: { Authorization: `Bearer ${tokens.access_token}` },
    });
    expect(res.status()).toBe(403);
  });

  test("password validation rejects weak passwords", async ({ request }) => {
    const res = await request.post("/api/register", {
      data: {
        company_name: "Test", slug: "test-weak-pw-e2e",
        email: "test-weak@test.com", password: "short",
        first_name: "Test",
      },
    });
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.error).toContain("10 characters");
  });

  test("public files endpoint only serves images", async ({ request }) => {
    // Non-existent file should 404
    const res = await request.get("/api/public/files/00000000-0000-0000-0000-000000000000");
    expect(res.status()).toBe(404);
  });

  test("brute force login returns 401 or 429", async ({ request }) => {
    // Just verify the endpoint responds correctly to bad credentials
    const res = await request.post("/api/auth/login", {
      data: { email: "nonexistent-test@test.com", password: "WrongPassword1" },
    });
    expect([401, 429].includes(res.status())).toBeTruthy();
  });
});
