import { Page } from "@playwright/test";

const BASE = "https://clientroll.com";
const EMAIL = "ben@businessnet.com.au";

// Login and store tokens — reusable across tests
export async function login(page: Page, password = "TestPassword1!") {
  // Try API login first for speed
  const res = await page.request.post(`${BASE}/api/auth/login`, {
    data: { email: EMAIL, password },
  });

  if (res.ok()) {
    const data = await res.json();
    await page.goto(BASE);
    await page.evaluate((tokens) => {
      localStorage.setItem("access_token", tokens.access_token);
      localStorage.setItem("refresh_token", tokens.refresh_token);
    }, data.tokens);
    return data;
  }

  // Fallback: UI login
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Sign in")');
  await page.waitForURL("**/dashboard", { timeout: 10000 });
}

export { BASE, EMAIL };
