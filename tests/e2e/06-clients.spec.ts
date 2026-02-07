import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Clients", () => {
  test("clients page loads with data", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await expect(page.locator("h1:has-text('Clients')")).toBeVisible();
    // Should show demo clients
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible({ timeout: 5000 });
  });

  test("clients search works", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await page.fill('input[placeholder="Search..."]', "Sarah");
    await page.keyboard.press("Enter");
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible();
  });

  test("clients API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/clients", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.count).toBeGreaterThan(0);
    expect(body.clients.length).toBeGreaterThan(0);
  });

  test("clients API search works", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/clients?search=Sarah", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const body = await res.json();
    expect(body.clients.some((c: { name: string }) => c.name.includes("Sarah"))).toBeTruthy();
  });

  test("click client row opens edit form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await page.waitForSelector("text=Sarah Mitchell");
    await page.click("text=Sarah Mitchell");
    await expect(page.locator("text=Edit: Sarah Mitchell")).toBeVisible();
  });

  test("create new dropdown opens client form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Create new");
    await page.click("text=Add to your client list");
    await expect(page.locator("text=New client")).toBeVisible();
    await expect(page.locator("text=Details")).toBeVisible();
    await expect(page.locator("text=Company")).toBeVisible();
  });

  test("three-dot menu shows options", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await page.waitForSelector("text=Sarah Mitchell");
    // Click the three-dot button on first row
    const menuBtn = page.locator("table tbody tr").first().locator('svg[fill="currentColor"]').first();
    await menuBtn.click();
    await expect(page.locator("text=Edit")).toBeVisible();
    await expect(page.locator("text=View profile")).toBeVisible();
    await expect(page.locator("text=Delete")).toBeVisible();
  });
});
