import { Page, APIRequestContext } from "@playwright/test";

const TEST_EMAIL = "ben@businessnet.com.au";
const TEST_PASS = "Clientroll1Test!";

export async function loginAsAdmin(page: Page) {
  await page.goto("/login");
  await page.fill('input[type="email"]', TEST_EMAIL);
  await page.fill('input[type="password"]', TEST_PASS);
  await page.click('button:has-text("Log in")');
  await page.waitForURL("**/dashboard", { timeout: 15000 });
}

export async function getAuthToken(request: APIRequestContext): Promise<string> {
  const res = await request.post("/api/auth/login", {
    data: { email: TEST_EMAIL, password: TEST_PASS },
  });
  const body = await res.json();
  if (!body.tokens) throw new Error(`Login failed: ${JSON.stringify(body)}`);
  return body.tokens.access_token;
}
