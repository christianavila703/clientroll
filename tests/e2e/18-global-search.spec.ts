import { test, expect } from "@playwright/test";
import { loginAsAdmin } from "./helpers";

test.describe("Global Search", () => {
  test("search finds clients", async ({ page }) => {
    await loginAsAdmin(page);
    await page.fill('input[placeholder="Search clients, memberships..."]', "Sarah");
    await page.waitForTimeout(500);
    await expect(page.locator("text=Sarah Mitchell")).toBeVisible();
  });

  test("search finds membership types", async ({ page }) => {
    await loginAsAdmin(page);
    await page.fill('input[placeholder="Search clients, memberships..."]', "Gold");
    await page.waitForTimeout(500);
    await expect(page.locator("text=Gold Membership")).toBeVisible();
  });

  test("search clears on result click", async ({ page }) => {
    await loginAsAdmin(page);
    const searchInput = page.locator('input[placeholder="Search clients, memberships..."]');
    await searchInput.fill("Sarah");
    await page.waitForTimeout(500);
    await page.click("text=Sarah Mitchell");
    await expect(searchInput).toHaveValue("");
  });
});
