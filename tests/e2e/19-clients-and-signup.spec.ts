import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

const TENANT_SLUG = "studio-figura";

test.describe("Clients and signup", () => {
  test("create client via API then list includes them", async ({ request }) => {
    const token = await getAuthToken(request);
    const name = `E2E Client ${Date.now()}`;
    const email = `e2e-${Date.now()}@example.com`;

    const createRes = await request.post("/api/tenant/clients", {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      data: JSON.stringify({ name, email }),
    });
    expect(createRes.ok()).toBeTruthy();
    const client = await createRes.json();
    expect(client.id).toBeTruthy();
    expect(client.name).toBe(name);
    expect(client.email).toBe(email);

    const listRes = await request.get("/api/tenant/clients", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(listRes.ok()).toBeTruthy();
    const { clients } = await listRes.json();
    const found = clients.find((c: { id: string }) => c.id === client.id);
    expect(found).toBeTruthy();
    expect(found.name).toBe(name);
  });

  test("create client and membership via API (staff signup)", async ({ request }) => {
    const token = await getAuthToken(request);

    // Create client
    const name = `E2E Member ${Date.now()}`;
    const createClientRes = await request.post("/api/tenant/clients", {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      data: JSON.stringify({ name, email: `member-${Date.now()}@example.com` }),
    });
    expect(createClientRes.ok()).toBeTruthy();
    const client = await createClientRes.json();

    // Get first membership type with term option
    const mtListRes = await request.get("/api/tenant/membership-types", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(mtListRes.ok()).toBeTruthy();
    const { membership_types: types } = await mtListRes.json();
    if (types.length === 0) {
      test.skip(true, "No membership types in tenant");
      return;
    }
    const mtId = types[0].id;

    const mtDetailRes = await request.get(`/api/tenant/membership-types/${mtId}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(mtDetailRes.ok()).toBeTruthy();
    const mt = await mtDetailRes.json();
    const termOptions = mt.term_options || [];
    if (termOptions.length === 0) {
      test.skip(true, "No term options on membership type");
      return;
    }
    const termOptionId = termOptions[0].id;

    const startDate = new Date();
    startDate.setDate(1);
    const startDateStr = startDate.toISOString().slice(0, 10);

    const createMemberRes = await request.post("/api/tenant/memberships", {
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      data: JSON.stringify({
        client_id: client.id,
        membership_type_id: mtId,
        term_option_id: termOptionId,
        start_date: startDateStr,
        payment_method: "direct_debit",
        payment_frequency: "monthly",
        payment_amount: 50,
      }),
    });
    expect(createMemberRes.ok()).toBeTruthy();
    const membership = await createMemberRes.json();
    expect(membership.id).toBeTruthy();
    expect(membership.client_id).toBe(client.id);
    expect(membership.status).toBeTruthy();

    // Active memberships list should include this
    const activeRes = await request.get("/api/tenant/active-memberships?status=active", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(activeRes.ok()).toBeTruthy();
    const { memberships: list } = await activeRes.json();
    const found = list.find((m: { membership_id: string }) => m.membership_id === membership.id);
    expect(found).toBeTruthy();
    expect(found.client_name).toBe(name);
  });

  test("public signup API returns tenant and memberships", async ({ request }) => {
    const res = await request.get(`/api/signup/${TENANT_SLUG}`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.tenant).toBeTruthy();
    expect(body.tenant.slug).toBe(TENANT_SLUG);
    expect(Array.isArray(body.memberships)).toBeTruthy();
  });

  test("public signup submit creates client and membership", async ({ request }) => {
    const name = `Public Signup ${Date.now()}`;
    const email = `signup-${Date.now()}@example.com`;

    // List returns types without term_options; get detail for one (includes term_options)
    const listRes = await request.get(`/api/signup/${TENANT_SLUG}`);
    expect(listRes.ok()).toBeTruthy();
    const { memberships } = await listRes.json();
    if (!memberships || memberships.length === 0) {
      test.skip(true, "No membership types for tenant");
      return;
    }
    const firstSlug = memberships[0].slug;
    const detailRes = await request.get(`/api/signup/${TENANT_SLUG}/${firstSlug}`);
    expect(detailRes.ok()).toBeTruthy();
    const { membership: mt } = await detailRes.json();
    const termOptions = mt.term_options || [];
    if (termOptions.length === 0) {
      test.skip(true, "No term options");
      return;
    }
    const termOptionId = termOptions[0].id;
    const startDate = new Date();
    startDate.setDate(1);
    const startDateStr = startDate.toISOString().slice(0, 10); // YYYY-MM-DD for staff API
    const startDateISO = startDate.toISOString(); // RFC3339 for signup submit

    const submitRes = await request.post(`/api/signup/${TENANT_SLUG}/submit`, {
      headers: { "Content-Type": "application/json" },
      data: JSON.stringify({
        client: { name, email },
        membership_type_id: mt.id,
        term_option_id: termOptionId,
        start_date: startDateISO,
        payment_method: "direct_debit",
        payment_frequency: "monthly",
        payment_amount: 50,
        member_signature: "e2e-test-signature",
      }),
    });
    expect(submitRes.ok()).toBeTruthy();
    const submitBody = await submitRes.json();
    expect(submitBody.membership).toBeTruthy();
    expect(submitBody.client).toBeTruthy();
    expect(submitBody.client.name).toBe(name);
    expect(submitBody.client.email).toBe(email);
    expect(submitBody.membership.id).toBeTruthy();
    expect(submitBody.thankyou).toBeTruthy();
  });
});
