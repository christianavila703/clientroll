import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Membership Types", () => {
  test("membership types page loads with data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await expect(page.locator("text=Gold Membership")).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Silver Membership")).toBeVisible();
    await expect(page.locator("text=Platinum Membership")).toBeVisible();
  });

  test("tabs filter correctly", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.waitForSelector("text=Gold Membership");
    // All tab should show all
    await expect(page.locator("text=3 results")).toBeVisible();
    // Active tab
    await page.click("button:has-text('Active')");
    await expect(page.locator("text=3 results")).toBeVisible();
    // Draft tab
    await page.click("button:has-text('Draft')");
    await expect(page.locator("text=0 results")).toBeVisible();
  });

  test("search filters membership types", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.fill('input[placeholder="Search..."]', "Gold");
    await expect(page.locator("text=1 results")).toBeVisible();
  });

  test("clicking a membership type opens detail page", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.waitForSelector("text=Gold Membership");
    await page.click("text=Gold Membership");
    await page.waitForURL("**/memberships/**");
    await expect(page.locator("text=Term Options & Pricing")).toBeVisible();
    await expect(page.locator("text=Bonuses")).toBeVisible();
    await expect(page.locator("text=Terms & Conditions")).toBeVisible();
  });

  test("membership type detail shows term options", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.click("text=Gold Membership");
    await page.waitForURL("**/memberships/**");
    await expect(page.locator("text=6 months")).toBeVisible();
    await expect(page.locator("text=12 months")).toBeVisible();
  });

  test("bonuses tab shows assignments", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.click("text=Gold Membership");
    await page.waitForURL("**/memberships/**");
    await page.click("button:has-text('Bonuses')");
    await expect(page.locator("text=Free Body Composition Scan")).toBeVisible();
    await expect(page.locator("text=Personal Training Session")).toBeVisible();
  });

  test("T&C tab shows attached terms", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Membership Types");
    await page.click("text=Gold Membership");
    await page.waitForURL("**/memberships/**");
    await page.click("button:has-text('Terms & Conditions')");
    await expect(page.locator("text=Standard Membership Agreement")).toBeVisible();
    await expect(page.locator("text=Direct Debit Agreement")).toBeVisible();
  });

  test("membership types API returns data", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/membership-types", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBe(3);
  });

  test("duplicate membership type works via API", async ({ request }) => {
    const token = await getAuthToken(request);
    const headers = { Authorization: `Bearer ${token}` };
    const listRes = await request.get("/api/tenant/membership-types", { headers });
    const { membership_types } = await listRes.json();
    const silver = membership_types.find((mt: { title: string }) => mt.title === "Silver Membership");

    const dupRes = await request.post(`/api/tenant/membership-types/${silver.id}/duplicate`, { headers });
    expect(dupRes.ok()).toBeTruthy();
    const dup = await dupRes.json();
    expect(dup.title).toContain("(Copy)");
    expect(dup.is_active).toBe(false);
  });
});
