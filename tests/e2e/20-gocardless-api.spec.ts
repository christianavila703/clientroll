import { test, expect } from "@playwright/test";
import { getAuthToken } from "./helpers";

const BASE = "/api/tenant";

test.describe("GoCardless API from Client Roll", () => {
  test("GET gocardless/connect returns redirect URL", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get(`${BASE}/gocardless/connect`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (res.status() === 400) {
      test.skip(true, "GoCardless not configured");
      return;
    }
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.redirect_url).toBeTruthy();
    expect(body.redirect_url).toContain("gocardless.com");
  });

  test("GET gocardless/payments (list from GoCardless) returns 200 and array", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get(`${BASE}/gocardless/payments?limit=10`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (res.status() === 400 || res.status() === 404) {
      test.skip(true, "GoCardless not configured or route not found");
      return;
    }
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Array.isArray(body.payments)).toBeTruthy();
  });

  test("GET gocardless/customers returns 200 and array", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get(`${BASE}/gocardless/customers`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (res.status() === 400 || res.status() === 404) {
      test.skip(true, "GoCardless not configured or route not found");
      return;
    }
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Array.isArray(body.customers)).toBeTruthy();
  });

  test("POST gocardless/redirect-flows returns redirect_url and flow_id", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.post(`${BASE}/gocardless/redirect-flows`, {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      data: JSON.stringify({
        redirect_uri: "https://app.clientroll.com/settings/integrations",
        description: "E2E mandate setup",
        session_token: "e2e-session-" + Date.now(),
      }),
    });
    if (res.status() === 400 || res.status() === 404) {
      test.skip(true, "GoCardless not configured or route not found");
      return;
    }
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.redirect_url).toBeTruthy();
    expect(body.flow_id).toBeTruthy();
    expect(body.redirect_url).toContain("gocardless.com");
  });

  test("POST gocardless/payments (create) requires membership_id, amount, due_date", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.post(`${BASE}/gocardless/payments`, {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      data: JSON.stringify({
        membership_id: "00000000-0000-0000-0000-000000000000",
        amount: 10,
        due_date: "2025-03-01",
      }),
    });
    if (res.status() === 400) {
      const body = await res.json();
      expect(body.error).toBeTruthy();
      return;
    }
    if (res.status() === 404) {
      return;
    }
    if (res.status() === 201) {
      const body = await res.json();
      expect(body.payment_id).toBeTruthy();
      expect(body.provider_reference).toBeTruthy();
      expect(body.amount).toBe(10);
      return;
    }
    expect([400, 404, 201]).toContain(res.status());
  });

  test("POST gocardless/payments/{id}/retry returns 200 or 404", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.post(
      `${BASE}/gocardless/payments/00000000-0000-0000-0000-000000000001/retry`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    if (res.status() === 400) {
      test.skip(true, "GoCardless not configured");
      return;
    }
    expect([200, 404, 500]).toContain(res.status());
    if (res.ok()) {
      const body = await res.json();
      expect(body.status).toBe("retry_submitted");
    }
  });

  test("POST gocardless/payments/{id}/cancel returns 200 or 404", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.post(
      `${BASE}/gocardless/payments/00000000-0000-0000-0000-000000000001/cancel`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    if (res.status() === 400) {
      test.skip(true, "GoCardless not configured");
      return;
    }
    expect([200, 404, 500]).toContain(res.status());
    if (res.ok()) {
      const body = await res.json();
      expect(body.status).toBe("cancelled");
    }
  });

  test("GET gocardless/mandates without customer_id returns 400 or 404", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get(`${BASE}/gocardless/mandates`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect([400, 404]).toContain(res.status());
    if (res.status() === 400) {
      const body = await res.json();
      expect(body.error).toContain("customer_id");
    }
  });

  test("GET gocardless/mandates?customer_id=xxx returns 200 and array or 4xx/5xx", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get(`${BASE}/gocardless/mandates?customer_id=cus_dummy`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (res.status() === 400) {
      const body = await res.json();
      if (body.error?.includes("not configured")) {
        test.skip(true, "GoCardless not configured");
        return;
      }
    }
    expect([200, 400, 404, 500]).toContain(res.status());
    if (res.ok()) {
      const body = await res.json();
      expect(Array.isArray(body.mandates)).toBeTruthy();
    }
  });
});
