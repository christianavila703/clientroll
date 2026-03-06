import { test, expect } from "@playwright/test";
import { getE2EContext } from "./helpers";

test.describe("Public Signup Flow", () => {
  test("tenant memberships API returns active types", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const res = await request.get(`/api/signup/${ctx.tenantSlug}`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.tenant.slug).toBe(ctx.tenantSlug);
    expect(Array.isArray(body.memberships)).toBeTruthy();
    expect((body.memberships ?? []).length).toBeGreaterThan(0);
  });

  test("membership detail API returns full data with bonuses", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const res = await request.get(`/api/signup/${ctx.tenantSlug}/${ctx.goldMembershipSlug}`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.membership.title).toBe("Gold Membership");
    expect(body.membership.term_options.length).toBeGreaterThanOrEqual(2);
    expect(body.terms.length).toBeGreaterThanOrEqual(1);
    const term12 = body.membership.term_options.find((t: { duration_months: number }) => t.duration_months === 12);
    expect(term12).toBeTruthy();
    expect((term12?.bonuses ?? []).length).toBeGreaterThanOrEqual(1);
  });

  test("term bonus API returns bonuses for specific term", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const mtRes = await request.get(`/api/signup/${ctx.tenantSlug}/${ctx.goldMembershipSlug}`);
    const { membership } = await mtRes.json();
    const term12 = membership.term_options.find((t: { duration_months: number }) => t.duration_months === 12);
    expect(term12).toBeTruthy();

    const bonusRes = await request.get(`/api/signup/${ctx.tenantSlug}/${ctx.goldMembershipSlug}/term/${term12?.id}/bonuses`);
    expect(bonusRes.ok()).toBeTruthy();
    const { bonuses } = await bonusRes.json();
    expect(bonuses.length).toBeGreaterThanOrEqual(1);
  });

  test("signup page loads for tenant", async ({ page }) => {
    const ctx = await getE2EContext(page.request);
    await page.goto(`/${ctx.tenantSlug}/${ctx.goldMembershipSlug}`);
    await expect(page.getByRole("heading", { name: "Gold Membership" })).toBeVisible({ timeout: 10000 });
  });

  test("signup page shows term options", async ({ page }) => {
    const ctx = await getE2EContext(page.request);
    await page.goto(`/${ctx.tenantSlug}/${ctx.goldMembershipSlug}`);
    await expect(page.getByText("6 Months", { exact: true }).first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("12 Months", { exact: true }).first()).toBeVisible();
  });

  test("invalid tenant returns 404", async ({ request }) => {
    const res = await request.get("/api/signup/nonexistent-tenant");
    expect(res.status()).toBe(404);
  });

  test("invalid membership returns 404", async ({ request }) => {
    const ctx = await getE2EContext(request);
    const res = await request.get(`/api/signup/${ctx.tenantSlug}/nonexistent-membership`);
    expect(res.status()).toBe(404);
  });
});
