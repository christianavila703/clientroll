import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Terms & Conditions", () => {
  test("terms page loads with data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Terms");
    await expect(page.locator("text=Standard Membership Agreement")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Direct Debit Agreement")).toBeVisible();
  });

  test("click term opens edit form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Terms");
    await page.click("text=Standard Membership Agreement");
    await expect(page.locator("text=Edit: Standard Membership Agreement")).toBeVisible();
  });

  test("terms API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/terms", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBeGreaterThanOrEqual(1);
  });

  test("terms attached to membership types via API", async ({ request }) => {
    const token = await getAuthToken(request);
    const headers = { Authorization: `Bearer ${token}` };
    const mtRes = await request.get("/api/tenant/membership-types", { headers });
    const { membership_types } = await mtRes.json();
    const gold = membership_types.find((mt: { title: string }) => mt.title === "Gold Membership");

    const termsRes = await request.get(`/api/tenant/membership-types/${gold.id}/terms`, { headers });
    const { terms } = await termsRes.json();
    expect(terms.length).toBeGreaterThanOrEqual(1);
  });
});
