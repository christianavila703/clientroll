import { test, expect } from "@playwright/test";

test.describe("Tenant Registration", () => {
  test("slug check API works", async ({ request }) => {
    const res = await request.get("/api/register/check-slug/studio-figura");
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.slug).toBe("studio-figura");
    expect(typeof body.available).toBe("boolean");
  });

  test("slug check rejects invalid slugs", async ({ request }) => {
    const res = await request.get("/api/register/check-slug/a");
    const body = await res.json();
    expect(body.available).toBe(false);
  });

  test("continue button is disabled without company name", async ({ page }) => {
    await page.goto("/register");
    await page.waitForSelector("text=Company name");
    const btn = page.locator('button:has-text("Continue")');
    await expect(btn).toBeDisabled();
  });

  test("registration form auto-generates slug from company name", async ({ page }) => {
    await page.goto("/register");
    await page.fill('input >> nth=0', "Test Company XYZ");
    // Slug field should auto-populate
    const slugInput = page.locator('input >> nth=1');
    await expect(slugInput).toHaveValue("test-company-xyz");
  });
});
