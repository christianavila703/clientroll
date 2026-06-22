// Archive every membership type starting with "Playwright Test"
const { chromium } = require("playwright");
const EMAIL = process.env.CR_EMAIL;
const PASSWORD = process.env.CR_PASSWORD;
const BASE = process.env.BASE_URL || "https://app.clientroll.com";
(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(`${BASE}/login`);
  await page.locator('input#email').fill(EMAIL);
  await page.locator('input#password').fill(PASSWORD);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL(/\/dashboard|\/memberships/, { timeout: 20000 });
  const result = await page.evaluate(async () => {
    const token = localStorage.getItem("access_token");
    const csrf = document.cookie.split(";").map(s => s.trim()).find(s => s.startsWith("csrf_token="));
    const headers = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;
    if (csrf) headers["X-CSRF-Token"] = csrf.split("=")[1];
    const list = await fetch("/api/tenant/membership-types", { credentials: "include", headers }).then(r => r.json());
    const types = Array.isArray(list) ? list : (list.membership_types || list.data || []);
    const targets = types.filter(t => /Playwright Test/i.test(t.title || t.membership_name || ""));
    const out = [];
    for (const t of targets) {
      // Try DELETE first (only works if no memberships use it), fall back to archive.
      let r = await fetch(`/api/tenant/membership-types/${t.id}`, { method: "DELETE", credentials: "include", headers });
      if (!r.ok) {
        r = await fetch(`/api/tenant/membership-types/${t.id}/archive`, { method: "POST", credentials: "include", headers });
      }
      out.push({ id: t.id, name: t.title || t.membership_name, status: r.status });
    }
    return out;
  });
  console.log(JSON.stringify(result, null, 2));
  await browser.close();
})();
