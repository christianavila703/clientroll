import { expect, test } from "@playwright/test";

test("edit client modal saves without invalid request body", async ({ page, request }) => {
  const now = Date.now();
  const slug = `e2e-edit-client-${now}`;
  const email = `e2e-edit-client-${now}@example.com`;
  const password = "JourneyTest1!";
  const clientName = `E2E Edit Client ${now}`;

  await page.goto("/register");
  await page.getByPlaceholder("Acme Wellness").fill(`E2E Edit Client ${now}`);
  await page.getByPlaceholder("acme-wellness").fill(slug);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.locator('label:has-text("First name")').locator("xpath=following-sibling::input[1]").fill("E2E");
  await page.locator('label:has-text("Last name")').locator("xpath=following-sibling::input[1]").fill("Editor");
  await page.locator('label:has-text("Email")').locator("xpath=following-sibling::input[1]").fill(email);
  await page.locator('label:has-text("Password")').locator("xpath=following-sibling::input[1]").fill(password);
  await page.getByRole("button", { name: "Create account" }).click();
  const checkEmailNotice = page.getByText("Check your email");
  await Promise.race([
    page.waitForURL("**/dashboard", { timeout: 15000 }),
    checkEmailNotice.waitFor({ state: "visible", timeout: 15000 }),
  ]);

  await page.goto("/dashboard");
  await page.waitForLoadState("networkidle");
  const token = await page.evaluate(() => localStorage.getItem("access_token"));
  expect(token).toBeTruthy();
  const headers = { Authorization: `Bearer ${token}` };

  const createClientRes = await request.post("/api/tenant/clients", {
    headers,
    data: {
      name: clientName,
      email: `e2e-client-${now}@example.com`,
      phone: "0400000000",
    },
  });
  expect(createClientRes.status()).toBe(201);
  const client = (await createClientRes.json()) as { id: string };

  await page.goto(`/clients/${client.id}`);
  await page.waitForLoadState("networkidle");

  await page.getByRole("button", { name: "Edit client" }).click();
  const modal = page.locator("text=Edit:").first();
  await expect(modal).toBeVisible();

  await page.locator('input[type="tel"]').first().fill("0411111111");
  await page.getByRole("button", { name: "Save" }).click();

  await expect(page.locator("text=Invalid request body")).toHaveCount(0);
  await expect(page.locator("text=Failed")).toHaveCount(0);
  await expect(modal).toBeHidden({ timeout: 20000 });
  await expect(page.locator("main")).toContainText("0411111111");
});

