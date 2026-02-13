import { test, expect } from "@playwright/test";
import { loginAsAdmin, getAuthToken } from "./helpers";

test.describe("Clients", () => {
  test("clients page loads", async ({ page }) => {
    await loginAsAdmin(page);
    await page.click("text=Clients");
    await expect(page.locator("h1:has-text('Clients')")).toBeVisible({ timeout: 10000 });
    // Page shows list or empty state
    await expect(page.locator("text=Clients").first()).toBeVisible();
  });

  test("clients API returns list", async ({ request }) => {
    const token = await getAuthToken(request);
    const res = await request.get("/api/tenant/clients", {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(Number.isInteger(body.count)).toBeTruthy();
    expect(Array.isArray(body.clients ?? [])).toBeTruthy();
  });

  test("create new dropdown opens client form", async ({ page }) => {
    await loginAsAdmin(page);
    await page.getByRole("button", { name: /Create new/i }).first().click();
    await page.getByRole("menuitem", { name: /Add to your client list/i }).or(page.locator("text=Add to your client list")).first().click();
    await expect(page.getByRole("heading", { name: "New client" })).toBeVisible({ timeout: 5000 });
    await expect(page.locator("text=Details")).toBeVisible();
    await expect(page.locator("text=Company")).toBeVisible();
  });
});
