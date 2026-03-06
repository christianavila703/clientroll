import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Bonuses", () => {
  test("bonuses page loads with data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Bonuses");
    await expect(page.getByText("Free Body Composition Scan", { exact: true }).first()).toBeVisible({ timeout: 10000 });
    await expect(page.locator("text=results")).toBeVisible();
  });

  test("click bonus opens edit form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Bonuses");
    await page.click("text=Free Body Composition Scan");
    await expect(page.locator("text=Edit: Free Body Composition Scan")).toBeVisible();
  });

  test("bonuses API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/bonuses", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBeGreaterThanOrEqual(1);
  });

  test("bonus term option assignment API works", async ({ request }) => {
    const token = await getAuthToken(request);
    const headers = { Authorization: `Bearer ${token}` };

    // Get a term option
    const mtRes = await request.get("/api/tenant/membership-types", { headers });
    const { membership_types } = await mtRes.json();
    const gold = membership_types.find((mt: { title: string }) => mt.title === "Gold Membership");
    const mtDetail = await (await request.get(`/api/tenant/membership-types/${gold.id}`, { headers })).json();
    const termOption12 = mtDetail.term_options.find((to: { duration_months: number }) => to.duration_months === 12);

    // Get bonuses for that term option
    const bonusRes = await request.get(`/api/tenant/term-options/${termOption12.id}/bonuses`, { headers });
    expect(bonusRes.ok()).toBeTruthy();
    const { bonuses } = await bonusRes.json();
    expect(bonuses.length).toBeGreaterThanOrEqual(1);
  });
});
