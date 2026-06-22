// End-to-end test: create a membership type with 3 different term option
// configurations (monthly, pay upfront, custom fixed-count instalments), then
// open the public signup page and verify it renders correctly.
//
// Usage:
//   CR_EMAIL=... CR_PASSWORD=... node test-term-options.js
//
// Optional: HEADED=1 to run with a visible browser.

const { chromium } = require("playwright");
const fs = require("fs");
const path = require("path");

const BASE = process.env.BASE_URL || "https://app.clientroll.com";
const EMAIL = process.env.CR_EMAIL;
const PASSWORD = process.env.CR_PASSWORD;
const HEADED = !!process.env.HEADED;

if (!EMAIL || !PASSWORD) {
  console.error("Missing CR_EMAIL / CR_PASSWORD env vars");
  process.exit(2);
}

const SCREENSHOTS = path.join(__dirname, "artifacts");
fs.mkdirSync(SCREENSHOTS, { recursive: true });

const stamp = new Date().toISOString().replace(/[-:T.]/g, "").slice(0, 14);
const TEST_NAME = `Playwright Test ${stamp}`;
const TEST_LEVEL = "QA";

const log = (...args) => console.log("[e2e]", ...args);

async function shot(page, name) {
  const file = path.join(SCREENSHOTS, `${stamp}-${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  log("screenshot:", file);
}

async function fillSimplePricing(page, opts) {
  // opts: { durationValue, durationUnit, label, totalPrice, method, custom? }
  await page
    .locator('label:has-text("Term length")')
    .first()
    .locator("xpath=..")
    .locator('input[type="number"]')
    .first()
    .fill(String(opts.durationValue));
  if (opts.durationUnit) {
    await page
      .locator('label:has-text("Term length")')
      .first()
      .locator("xpath=..")
      .locator("select")
      .selectOption(opts.durationUnit);
  }
  if (opts.label) {
    await page
      .locator('label:has-text("Display label")')
      .first()
      .locator("xpath=..")
      .locator('input[type="text"]')
      .fill(opts.label);
  }
  // Total price field — first number input whose row contains "$"
  await page
    .locator('label:has-text("Total price")')
    .first()
    .locator("xpath=..")
    .locator('input[type="number"]')
    .first()
    .fill(String(opts.totalPrice));

  const radioLabel = {
    upfront: "Pay upfront",
    weekly: "Pay weekly",
    fortnightly: "Pay fortnightly",
    monthly: "Pay monthly",
    custom: "Custom plan",
  }[opts.method];
  if (!radioLabel) throw new Error("unknown method: " + opts.method);
  await page.locator(`label:has-text("${radioLabel}")`).first().click();

  if (opts.method === "custom" && opts.custom) {
    if (opts.custom.label) {
      await page.locator('input[placeholder^="Plan label"]').fill(opts.custom.label);
    }
    // Use exact label match — "every 6 months" in the Custom plan radio
    // helper text would otherwise match :has-text("Every") case-insensitively.
    const customBox = page.locator('input[placeholder^="Plan label"]').locator('xpath=ancestor::div[contains(@class,"rounded-md")][1]');
    if (opts.custom.intervalCount != null) {
      await customBox
        .locator('label', { hasText: /^Every$/ })
        .locator("xpath=..")
        .locator('input[type="number"]')
        .fill(String(opts.custom.intervalCount));
    }
    if (opts.custom.intervalUnit) {
      await customBox
        .locator('label', { hasText: /^Unit$/ })
        .locator("xpath=..")
        .locator("select")
        .selectOption(opts.custom.intervalUnit);
    }
    if (opts.custom.numPayments != null) {
      await customBox
        .locator('label', { hasText: /^Total payments$/ })
        .locator("xpath=..")
        .locator('input[type="number"]')
        .fill(String(opts.custom.numPayments));
    }
  }
}

(async () => {
  const browser = await chromium.launch({ headless: !HEADED });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 1000 } });
  const page = await ctx.newPage();
  page.on("console", (msg) => {
    if (msg.type() === "error") log("page-console-error:", msg.text());
  });
  page.on("pageerror", (e) => log("page-error:", e.message));

  try {
    // 1. Login
    log("logging in as", EMAIL);
    await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
    await shot(page, "01-login");
    await page.locator('input#email').fill(EMAIL);
    await page.locator('input#password').fill(PASSWORD);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/dashboard|\/memberships|\/active-memberships/, { timeout: 20000 });
    log("logged in, url:", page.url());
    await shot(page, "02-after-login");

    // 2. Go to memberships
    await page.goto(`${BASE}/memberships`, { waitUntil: "networkidle" });
    await shot(page, "03-memberships-list");

    // 3. Click create button
    const createBtn = page
      .locator("button", { hasText: /New membership type/i })
      .first();
    await createBtn.waitFor({ state: "visible", timeout: 10000 });
    await createBtn.click();
    await page.locator('h3:has-text("New Membership Type")').waitFor({ timeout: 10000 });
    await shot(page, "04-create-form");

    // 4. Fill the form
    await page
      .locator('label:has-text("Membership Name")')
      .first()
      .locator("xpath=..")
      .locator("input")
      .fill(TEST_NAME);
    await page
      .locator('label:has-text("Level")')
      .first()
      .locator("xpath=..")
      .locator("input")
      .fill(TEST_LEVEL);
    await page
      .locator('label:has-text("Description")')
      .first()
      .locator("xpath=..")
      .locator("textarea")
      .fill("Automated Playwright test — safe to delete.");
    await page
      .locator('label:has-text("Inclusions")')
      .first()
      .locator("xpath=..")
      .locator("textarea")
      .fill("• QA inclusion 1\n• QA inclusion 2");
    await shot(page, "05-create-form-filled");

    // Submit
    await page.locator('button:has-text("Save")').first().click();
    // Wait for the new MT to appear in the list and click it.
    const newRow = page.getByText(TEST_NAME, { exact: false }).first();
    await newRow.waitFor({ timeout: 20000 });
    log("created MT row in list, clicking through");
    await newRow.click();
    await page.waitForURL(/\/memberships\/[0-9a-f-]+/, { timeout: 15000 });
    await page.waitForLoadState("networkidle");
    await shot(page, "06-mt-detail-empty");

    const mtUrl = page.url();
    const mtId = mtUrl.match(/\/memberships\/([0-9a-f-]+)/)[1];
    log("MT id:", mtId);

    // 5. Add 3 term options.
    const terms = [
      {
        name: "12-month monthly",
        durationValue: 12,
        durationUnit: "month",
        label: "12 Months — Monthly",
        totalPrice: 1200,
        method: "monthly",
        expect: { amount: "100", suffix: /per month/i, total: "1,200" },
      },
      {
        name: "6-month upfront",
        durationValue: 6,
        durationUnit: "month",
        label: "6 Months — Upfront",
        totalPrice: 1500,
        method: "upfront",
        expect: { amount: "1,500", suffix: /Pay in full/i, total: null },
      },
      {
        name: "12-month custom 4x quarterly",
        durationValue: 12,
        durationUnit: "month",
        label: "12 Months — 4 quarterly payments",
        totalPrice: 2000,
        method: "custom",
        custom: { intervalCount: 3, intervalUnit: "month", numPayments: 4 },
        expect: { amount: "500", suffix: /4 payments total/i, total: "2,000" },
      },
    ];

    for (const t of terms) {
      log("adding term:", t.name);
      // Click "Add term option"
      const addBtn = page
        .locator("button", { hasText: /Add term option|\+ Add term|New term option/i })
        .first();
      await addBtn.click();
      await page.locator('h3:has-text("New Term Option")').waitFor({ timeout: 5000 });
      await fillSimplePricing(page, t);
      await shot(page, `07-add-term-${t.method}`);
      await page.locator('button:has-text("Add term option")').last().click();
      // Wait for the form to disappear (success)
      await page
        .locator('h3:has-text("New Term Option")')
        .waitFor({ state: "detached", timeout: 15000 });
      await page.waitForLoadState("networkidle");
    }
    await shot(page, "08-mt-detail-with-terms");

    // 6. Activate the MT (created as draft by default), then fetch the signup link.
    log("activating MT");
    const activateStatus = await page.evaluate(async (id) => {
      const token = localStorage.getItem("access_token");
      const csrf = document.cookie
        .split(";").map(s => s.trim()).find(s => s.startsWith("csrf_token="));
      const headers = { "Content-Type": "application/json" };
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (csrf) headers["X-CSRF-Token"] = csrf.split("=")[1];
      const r = await fetch(`/api/tenant/membership-types/${id}/activate`, {
        method: "POST", credentials: "include", headers,
      });
      return r.status;
    }, mtId);
    log("activate status:", activateStatus);

    log("fetching signup links");
    const linksResp = await page.evaluate(async () => {
      const token = localStorage.getItem("access_token");
      const csrf = document.cookie
        .split(";").map(s => s.trim()).find(s => s.startsWith("csrf_token="));
      const headers = { "Content-Type": "application/json" };
      if (token) headers["Authorization"] = `Bearer ${token}`;
      if (csrf) headers["X-CSRF-Token"] = csrf.split("=")[1];
      const r = await fetch("/api/tenant/signup-links", { credentials: "include", headers });
      if (!r.ok) throw new Error("signup-links fetch failed: " + r.status);
      return await r.json();
    });
    log("got", linksResp.links.length, "links");
    const newLink = linksResp.links.find((l) => l.membership_id === mtId);
    if (!newLink) {
      // Slug may be slightly different — try to find by slug prefix
      log("no exact match by id, dumping links to find newly created:");
      log(JSON.stringify(linksResp.links.slice(-5), null, 2));
      throw new Error("Could not find signup link for new MT");
    }
    const signupUrl = `${BASE}${newLink.path}`;
    log("signup url:", signupUrl);

    // 7. Open the signup page (use a fresh context to mimic a public visitor)
    const publicCtx = await browser.newContext({ viewport: { width: 1400, height: 1200 } });
    const publicPage = await publicCtx.newPage();
    await publicPage.goto(signupUrl, { waitUntil: "networkidle" });
    await publicPage.screenshot({
      path: path.join(SCREENSHOTS, `${stamp}-09-signup.png`),
      fullPage: true,
    });
    const body = await publicPage.locator("body").innerText();
    log("--- signup page body (first 3000 chars) ---");
    log(body.slice(0, 3000));
    log("--- end body ---");

    // 8. Verify rendered text
    const failures = [];
    for (const t of terms) {
      const blob = body;
      if (!blob.includes(t.expect.amount)) {
        failures.push(`expected amount "${t.expect.amount}" for term "${t.name}"`);
      }
      if (!t.expect.suffix.test(blob)) {
        failures.push(`expected suffix /${t.expect.suffix}/ for term "${t.name}"`);
      }
      if (t.expect.total && !blob.includes(t.expect.total)) {
        failures.push(`expected total "${t.expect.total}" for term "${t.name}"`);
      }
    }
    if (failures.length) {
      log("FAILURES:");
      failures.forEach((f) => log("  -", f));
      process.exitCode = 1;
    } else {
      log("ALL ASSERTIONS PASSED");
    }

    // 9. Cleanup: archive the test MT so it doesn't pollute the listing
    log("archiving test MT to clean up");
    await page
      .evaluate(async (id) => {
        const token = localStorage.getItem("access_token");
        const csrf = document.cookie
          .split(";").map(s => s.trim()).find(s => s.startsWith("csrf_token="));
        const headers = { "Content-Type": "application/json" };
        if (token) headers["Authorization"] = `Bearer ${token}`;
        if (csrf) headers["X-CSRF-Token"] = csrf.split("=")[1];
        const r = await fetch(`/api/tenant/membership-types/${id}/archive`, {
          method: "POST", credentials: "include", headers,
        });
        return r.status;
      }, mtId)
      .then((s) => log("archive status:", s))
      .catch((e) => log("archive failed (non-fatal):", e.message));

    await publicCtx.close();
  } catch (e) {
    log("FATAL:", e.message);
    log(e.stack);
    try {
      await shot(page, "99-error");
    } catch {}
    process.exitCode = 1;
  } finally {
    await ctx.close();
    await browser.close();
  }
})();
