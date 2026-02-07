import { test, expect } from "@playwright/test";
import { getAuthToken } from "./helpers";

test.describe("Webhooks API", () => {
  test("event types endpoint returns all types", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/webhooks/event-types", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.event_types).toContain("client.created");
    expect(body.event_types).toContain("membership.created");
    expect(body.event_types).toContain("payment.succeeded");
    expect(body.event_types).toContain("payment.failed");
    expect(body.event_types.length).toBe(12);
  });

  test("webhook endpoints list returns empty initially", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/webhooks", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBe(0);
  });

  test("create webhook endpoint returns signing secret", async ({ request }) => {
    const token = await getAuthToken(request);
    const headers = { Authorization: `Bearer ${token}` };
    const res = await request.post("/api/tenant/webhooks", {
      headers,
      data: {
        url: "https://example.com/webhook",
        event_types: ["client.created", "membership.created"],
      },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.signing_secret).toBeTruthy();
    expect(body.signing_secret).toMatch(/^whsec_/);
    expect(body.endpoint.url).toBe("https://example.com/webhook");

    // Clean up
    await request.delete(`/api/tenant/webhooks/${body.endpoint.id}`, { headers });
  });
});
