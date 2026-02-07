import { test, expect } from "@playwright/test";

test.describe("Public Signup Flow", () => {
  test("tenant memberships API returns active types", async ({ request }) => {
    const res = await request.get("/api/signup/studio-figura");
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.tenant.name).toBe("Studio Figura");
    expect(body.memberships.length).toBeGreaterThan(0);
  });

  test("membership detail API returns full data with bonuses", async ({ request }) => {
    const res = await request.get("/api/signup/studio-figura/gold-membership");
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.membership.title).toBe("Gold Membership");
    expect(body.membership.term_options.length).toBe(2);
    expect(body.terms.length).toBe(2);
    // 12-month term should have bonuses
    const term12 = body.membership.term_options.find((t: { duration_months: number }) => t.duration_months === 12);
    expect(term12.bonuses.length).toBe(3);
  });

  test("term bonus API returns bonuses for specific term", async ({ request }) => {
    // Get the term option ID
    const mtRes = await request.get("/api/signup/studio-figura/gold-membership");
    const { membership } = await mtRes.json();
    const term12 = membership.term_options.find((t: { duration_months: number }) => t.duration_months === 12);

    const bonusRes = await request.get(`/api/signup/studio-figura/gold-membership/term/${term12.id}/bonuses`);
    expect(bonusRes.ok()).toBeTruthy();
    const { bonuses } = await bonusRes.json();
    expect(bonuses.length).toBe(3);
  });

  test("signup page loads for tenant", async ({ page }) => {
    await page.goto("/studio-figura/gold-membership");
    await expect(page.locator("header >> text=Studio Figura")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Gold Membership")).toBeVisible({ timeout: 5000 });
  });

  test("signup page shows term options", async ({ page }) => {
    await page.goto("/studio-figura/gold-membership");
    await expect(page.locator("text=6 Months")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=12 Months")).toBeVisible();
  });

  test("invalid tenant returns 404", async ({ request }) => {
    const res = await request.get("/api/signup/nonexistent-tenant");
    expect(res.status()).toBe(404);
  });

  test("invalid membership returns 404", async ({ request }) => {
    const res = await request.get("/api/signup/studio-figura/nonexistent-membership");
    expect(res.status()).toBe(404);
  });
});
