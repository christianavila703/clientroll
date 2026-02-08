import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe.serial("Comprehensive CRUD Tests", () => {
  let token: string;

  test.beforeAll(async ({ request }) => {
    token = await getAuthToken(request);
  });

  // ─── Auth ───
  test("login with correct credentials", async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page).toHaveURL(/dashboard/);
  });

  test("login with wrong password shows error", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[type="email"]', "ben@businessnet.com.au");
    await page.fill('input[type="password"]', "wrongpassword");
    await page.click('button:has-text("Log in")');
    await expect(page.locator("text=Invalid")).toBeVisible({ timeout: 5000 });
  });

  // ─── Dashboard ───
  test("dashboard loads with KPI cards and charts", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/dashboard");
    await expect(page.locator("text=Overview")).toBeVisible();
    await expect(page.locator("main").getByText("Active Members")).toBeVisible();
    await expect(page.locator("main").getByText("Revenue This Month")).toBeVisible();
  });

  // ─── Services CRUD ───
  test("create a service", async ({ request }) => {
    const res = await request.post("/api/tenant/services", {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: "E2E Test Service", description: "Test service", price: 99.99, tax_rate: 10, sort_order: 0 },
    });
    expect(res.status()).toBe(201);
    const body = await res.json();
    expect(body.name).toBe("E2E Test Service");
    expect(body.price).toBe(99.99);
  });

  test("list services includes test service", async ({ request }) => {
    const res = await request.get("/api/tenant/services", { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.services.some((s: any) => s.name === "E2E Test Service")).toBeTruthy();
  });

  test("update a service", async ({ request }) => {
    const list = await request.get("/api/tenant/services", { headers: { Authorization: `Bearer ${token}` } });
    const svc = (await list.json()).services.find((s: any) => s.name === "E2E Test Service");
    const res = await request.put(`/api/tenant/services/${svc.id}`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: "E2E Updated Service", description: "Updated", price: 149.99, tax_rate: 10, is_active: true, sort_order: 0 },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.name).toBe("E2E Updated Service");
    expect(body.price).toBe(149.99);
  });

  test("delete a service", async ({ request }) => {
    const list = await request.get("/api/tenant/services", { headers: { Authorization: `Bearer ${token}` } });
    const svc = (await list.json()).services.find((s: any) => s.name === "E2E Updated Service");
    const res = await request.delete(`/api/tenant/services/${svc.id}`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
  });

  // ─── Clients CRUD ───
  test("create a client", async ({ request }) => {
    const res = await request.post("/api/tenant/clients", {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: "E2E Test Client", email: "e2e@test.com", phone: "0400000000" },
    });
    expect(res.status()).toBe(201);
    const body = await res.json();
    expect(body.name).toBe("E2E Test Client");
  });

  test("list clients includes test client", async ({ request }) => {
    const res = await request.get("/api/tenant/clients?search=E2E", { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.clients.some((c: any) => c.name === "E2E Test Client")).toBeTruthy();
  });

  test("get single client", async ({ request }) => {
    const list = await request.get("/api/tenant/clients?search=E2E", { headers: { Authorization: `Bearer ${token}` } });
    const client = (await list.json()).clients.find((c: any) => c.name === "E2E Test Client");
    const res = await request.get(`/api/tenant/clients/${client.id}`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.name).toBe("E2E Test Client");
  });

  test("update a client", async ({ request }) => {
    const list = await request.get("/api/tenant/clients?search=E2E", { headers: { Authorization: `Bearer ${token}` } });
    const client = (await list.json()).clients.find((c: any) => c.name === "E2E Test Client");
    const res = await request.put(`/api/tenant/clients/${client.id}`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { name: "E2E Updated Client", email: "e2e-updated@test.com", phone: "0411111111" },
    });
    expect(res.status()).toBe(200);
    expect((await res.json()).name).toBe("E2E Updated Client");
  });

  // ─── Bonuses CRUD ───
  test("create a bonus", async ({ request }) => {
    const res = await request.post("/api/tenant/bonuses", {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: "E2E Test Bonus", description: "Test", value: 250, sort_order: 0 },
    });
    expect(res.status()).toBe(201);
    expect((await res.json()).title).toBe("E2E Test Bonus");
  });

  test("list bonuses includes test bonus", async ({ request }) => {
    const res = await request.get("/api/tenant/bonuses", { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.bonuses.some((b: any) => b.title === "E2E Test Bonus")).toBeTruthy();
  });

  test("update a bonus", async ({ request }) => {
    const list = await request.get("/api/tenant/bonuses", { headers: { Authorization: `Bearer ${token}` } });
    const bonus = (await list.json()).bonuses.find((b: any) => b.title === "E2E Test Bonus");
    const res = await request.put(`/api/tenant/bonuses/${bonus.id}`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: "E2E Updated Bonus", description: "Updated", value: 300, is_active: true, sort_order: 0 },
    });
    expect(res.status()).toBe(200);
    expect((await res.json()).title).toBe("E2E Updated Bonus");
  });

  test("delete a bonus", async ({ request }) => {
    const list = await request.get("/api/tenant/bonuses", { headers: { Authorization: `Bearer ${token}` } });
    const bonus = (await list.json()).bonuses.find((b: any) => b.title === "E2E Updated Bonus");
    const res = await request.delete(`/api/tenant/bonuses/${bonus.id}`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
  });

  // ─── Terms CRUD ───
  test("create terms", async ({ request }) => {
    const res = await request.post("/api/tenant/terms", {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: "E2E Test Terms", content: "<p>Test terms content with {{client_name}} variable</p>" },
    });
    expect(res.status()).toBe(201);
    expect((await res.json()).title).toBe("E2E Test Terms");
  });

  test("list terms includes test terms", async ({ request }) => {
    const res = await request.get("/api/tenant/terms", { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.terms.some((t: any) => t.title === "E2E Test Terms")).toBeTruthy();
  });

  test("update terms", async ({ request }) => {
    const list = await request.get("/api/tenant/terms", { headers: { Authorization: `Bearer ${token}` } });
    const terms = (await list.json()).terms.find((t: any) => t.title === "E2E Test Terms");
    const res = await request.put(`/api/tenant/terms/${terms.id}`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: "E2E Updated Terms", content: "<p>Updated content</p>", is_active: true },
    });
    expect(res.status()).toBe(200);
    expect((await res.json()).title).toBe("E2E Updated Terms");
  });

  // ─── Membership Types CRUD ───
  test("create membership type", async ({ request }) => {
    const res = await request.post("/api/tenant/membership-types", {
      headers: { Authorization: `Bearer ${token}` },
      data: { title: "E2E Test Membership", description: "Test", total_value: 1000, sort_order: 99 },
    });
    expect(res.status()).toBe(201);
    expect((await res.json()).title).toBe("E2E Test Membership");
  });

  test("list membership types includes test type", async ({ request }) => {
    const res = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.membership_types.some((mt: any) => mt.title === "E2E Test Membership")).toBeTruthy();
  });

  test("add term option to membership type", async ({ request }) => {
    const list = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const mt = (await list.json()).membership_types.find((m: any) => m.title === "E2E Test Membership");
    const res = await request.post(`/api/tenant/membership-types/${mt.id}/term-options`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { duration_months: 6, fee_total: 1000, fee_weekly: 38.46, fee_monthly: 166.67, allow_upfront: true },
    });
    expect(res.status()).toBe(201);
    expect((await res.json()).duration_months).toBe(6);
  });

  test("get membership type with term options", async ({ request }) => {
    const list = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const mt = (await list.json()).membership_types.find((m: any) => m.title === "E2E Test Membership");
    const res = await request.get(`/api/tenant/membership-types/${mt.id}`, { headers: { Authorization: `Bearer ${token}` } });
    const body = await res.json();
    expect(body.term_options.length).toBeGreaterThan(0);
    expect(body.term_options[0].fee_total).toBe(1000);
  });

  test("archive membership type", async ({ request }) => {
    const list = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const mt = (await list.json()).membership_types.find((m: any) => m.title === "E2E Test Membership");
    const res = await request.post(`/api/tenant/membership-types/${mt.id}/archive`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
  });

  test("activate membership type", async ({ request }) => {
    const list = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const mt = (await list.json()).membership_types.find((m: any) => m.title === "E2E Test Membership");
    const res = await request.post(`/api/tenant/membership-types/${mt.id}/activate`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
  });

  test("soft delete membership type", async ({ request }) => {
    const list = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const mt = (await list.json()).membership_types.find((m: any) => m.title === "E2E Test Membership");
    const res = await request.delete(`/api/tenant/membership-types/${mt.id}`, { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    // Verify it's gone from the list
    const list2 = await request.get("/api/tenant/membership-types", { headers: { Authorization: `Bearer ${token}` } });
    const body = await list2.json();
    expect(body.membership_types.some((m: any) => m.title === "E2E Test Membership")).toBeFalsy();
  });

  // ─── Payments ───
  test("list payments returns enriched data", async ({ request }) => {
    const res = await request.get("/api/tenant/payments", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    if (body.payments && body.payments.length > 0) {
      const p = body.payments[0];
      expect(p).toHaveProperty("client_name");
      expect(p).toHaveProperty("membership_type");
      expect(p).toHaveProperty("total_paid");
      expect(p).toHaveProperty("total_expected");
    }
  });

  test("filter payments by status", async ({ request }) => {
    const res = await request.get("/api/tenant/payments?status=succeeded", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    if (body.payments) {
      body.payments.forEach((p: any) => expect(p.status).toBe("succeeded"));
    }
  });

  // ─── Reports ───
  test("dashboard summary returns all fields", async ({ request }) => {
    const res = await request.get("/api/tenant/reports/dashboard", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("active_members");
    expect(body).toHaveProperty("month_revenue");
    expect(body).toHaveProperty("failed_payments");
    expect(body).toHaveProperty("new_signups");
  });

  test("comprehensive report returns all sections", async ({ request }) => {
    const res = await request.get("/api/tenant/reports/comprehensive", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("mrr");
    expect(body).toHaveProperty("arr");
    expect(body).toHaveProperty("collection_rate");
    expect(body).toHaveProperty("monthly_revenue");
    expect(body).toHaveProperty("membership_breakdown");
    expect(body).toHaveProperty("top_clients");
    expect(body).toHaveProperty("upcoming_expirations");
  });

  // ─── Active Memberships ───
  test("active memberships returns enriched data", async ({ request }) => {
    const res = await request.get("/api/tenant/active-memberships?status=active", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    if (body.memberships && body.memberships.length > 0) {
      expect(body.memberships[0]).toHaveProperty("client_id");
      expect(body.memberships[0]).toHaveProperty("client_name");
      expect(body.memberships[0]).toHaveProperty("membership_type");
    }
  });

  // ─── Service Charges (Take Payment) ───
  test("take payment for a client", async ({ request }) => {
    // Get a client
    const clientsRes = await request.get("/api/tenant/clients", { headers: { Authorization: `Bearer ${token}` } });
    const clients = (await clientsRes.json()).clients;
    if (clients && clients.length > 0) {
      const res = await request.post("/api/tenant/services/charge", {
        headers: { Authorization: `Bearer ${token}` },
        data: { client_id: clients[0].id, description: "E2E Test Charge", amount: 50, tax_amount: 5 },
      });
      expect(res.status()).toBe(201);
      expect((await res.json()).amount).toBe(50);
    }
  });

  // ─── Settings ───
  test("get tenant settings", async ({ request }) => {
    const res = await request.get("/api/tenant/settings/tenant", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body).toHaveProperty("name");
    expect(body).toHaveProperty("slug");
  });

  // ─── Staff ───
  test("list staff returns current user", async ({ request }) => {
    const res = await request.get("/api/tenant/staff", { headers: { Authorization: `Bearer ${token}` } });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.staff.some((s: any) => s.email === "ben@businessnet.com.au")).toBeTruthy();
  });

  // ─── Public Signup API ───
  test("public signup browse memberships", async ({ request }) => {
    const res = await request.get("/api/signup/studio-figura");
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.tenant.name).toBe("Studio Figura");
    expect(body.memberships.length).toBeGreaterThan(0);
  });

  // ─── UI Navigation ───
  test("all sidebar pages load without error", async ({ page }) => {
    await loginAsAdmin(page);
    const pages = [
      "/dashboard", "/clients", "/active-memberships", "/memberships",
      "/bonuses", "/services", "/terms", "/payments", "/reports",
      "/staff", "/email-templates", "/settings", "/signup",
    ];
    for (const path of pages) {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      // Should not show error page
      const body = await page.textContent("body");
      expect(body).not.toContain("Cannot GET");
      expect(body).not.toContain("Application error");
    }
  });

  test("client profile page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/clients");
    await page.waitForLoadState("networkidle");
    // Click first client row
    const firstRow = page.locator("table tbody tr").first();
    if (await firstRow.isVisible()) {
      await firstRow.click();
      await page.waitForURL(/\/clients\//);
      await expect(page.getByRole("button", { name: "Summary" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Memberships" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Billing schedule" })).toBeVisible();
    }
  });

  test("global search returns results", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/dashboard");
    const searchInput = page.locator('input[placeholder*="Search"]');
    if (await searchInput.isVisible()) {
      await searchInput.fill("Mia");
      await page.waitForTimeout(500);
      await expect(page.locator("text=Mia Chen").first()).toBeVisible({ timeout: 5000 });
    }
  });

  // ─── Cleanup: delete E2E test client ───
  test("cleanup test data", async ({ request }) => {
    // Delete test client
    const list = await request.get("/api/tenant/clients?search=E2E", { headers: { Authorization: `Bearer ${token}` } });
    const clients = (await list.json()).clients || [];
    for (const c of clients) {
      if (c.name.includes("E2E")) {
        // Can't delete clients via API yet, that's fine
      }
    }
    // Delete test terms
    const termsList = await request.get("/api/tenant/terms", { headers: { Authorization: `Bearer ${token}` } });
    const terms = (await termsList.json()).terms || [];
    for (const t of terms) {
      if (t.title.includes("E2E")) {
        await request.delete(`/api/tenant/terms/${t.id}`, { headers: { Authorization: `Bearer ${token}` } });
      }
    }
  });
});
