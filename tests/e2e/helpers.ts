import { APIRequestContext, Page } from "@playwright/test";

export interface E2EContext {
  email: string;
  password: string;
  tenantSlug: string;
  accessToken: string;
  refreshToken: string;
  goldMembershipSlug: string;
  baselineMembershipCreated: boolean;
}

let cachedContext: E2EContext | null = null;
let cachedContextPromise: Promise<E2EContext> | null = null;

const REGISTER_MAX_RETRIES = 5;
const REGISTER_BACKOFF_MS = 1500;

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function responseBodyPreview(res: { text(): Promise<string> }, limit = 400): Promise<string> {
  const text = await res.text();
  return text.length > limit ? `${text.slice(0, limit)}...` : text;
}

async function assertOk(
  res: { ok(): boolean; status(): number; text(): Promise<string> },
  label: string,
): Promise<void> {
  if (res.ok()) return;
  const body = await responseBodyPreview(res);
  throw new Error(`${label} failed: ${res.status()} ${body}`);
}

async function registerWithRetry(
  request: APIRequestContext,
  data: Record<string, unknown>,
): Promise<{ tokens: { access_token: string; refresh_token: string } }> {
  let attempt = 0;
  let lastError = "unknown";
  while (attempt < REGISTER_MAX_RETRIES) {
    const regRes = await request.post("/api/register", { data });
    if (regRes.ok()) {
      return (await regRes.json()) as { tokens: { access_token: string; refresh_token: string } };
    }
    const body = await responseBodyPreview(regRes);
    lastError = `${regRes.status()} ${body}`;

    // Retry only on throttling/transient errors.
    if (regRes.status() !== 429 && regRes.status() < 500) {
      throw new Error(`register failed: ${lastError}`);
    }
    const jitter = Math.floor(Math.random() * 500);
    await sleep(REGISTER_BACKOFF_MS * (attempt + 1) + jitter);
    attempt += 1;
  }
  throw new Error(`register failed after retries: ${lastError}`);
}

async function registerTenant(request: APIRequestContext): Promise<E2EContext> {
  const now = Date.now();
  const tenantSlug = `e2e-suite-${now}`;
  const email = `e2e-suite-${now}@example.com`;
  const password = "JourneyTest1!";

  const regBody = await registerWithRetry(request, {
    company_name: `E2E Suite ${now}`,
    slug: tenantSlug,
    first_name: "E2E",
    last_name: "Suite",
    email,
    password,
  });
  const accessToken = regBody.tokens?.access_token;
  const refreshToken = regBody.tokens?.refresh_token;
  if (!accessToken || !refreshToken) {
    throw new Error("register did not return auth tokens");
  }

  const authHeaders = { Authorization: `Bearer ${accessToken}` };

  const createMt = async (title: string, total: number, sortOrder: number) => {
    const res = await request.post("/api/tenant/membership-types", {
      headers: authHeaders,
      data: {
        title,
        description: `${title} for e2e`,
        total_value: total,
        sort_order: sortOrder,
      },
    });
    await assertOk(res, `create membership type (${title})`);
    return (await res.json()) as { id: string; slug: string };
  };

  const gold = await createMt("Gold Membership", 1200, 1);
  const silver = await createMt("Silver Membership", 800, 2);
  const platinum = await createMt("Platinum Membership", 1600, 3);

  for (const mt of [gold, silver, platinum]) {
    const activateRes = await request.post(`/api/tenant/membership-types/${mt.id}/activate`, {
      headers: authHeaders,
    });
    await assertOk(activateRes, `activate membership type ${mt.id}`);
  }

  const createTerm = async (mtId: string, duration: number, total: number, monthly: number) => {
    const res = await request.post(`/api/tenant/membership-types/${mtId}/term-options`, {
      headers: authHeaders,
      data: {
        duration_months: duration,
        fee_total: total,
        fee_monthly: monthly,
        allow_upfront: true,
      },
    });
    await assertOk(res, `create term option for membership type ${mtId}`);
    return (await res.json()) as { id: string };
  };

  const gold6 = await createTerm(gold.id, 6, 600, 100);
  const gold12 = await createTerm(gold.id, 12, 1200, 100);
  await createTerm(silver.id, 6, 480, 80);
  await createTerm(platinum.id, 12, 1800, 150);

  const bonusTitles = [
    "Free Body Composition Scan",
    "Personal Training Session",
    "Nutrition Plan",
    "Welcome Pack",
    "Recovery Session",
  ];
  const bonusIds: string[] = [];
  for (let i = 0; i < bonusTitles.length; i += 1) {
    const res = await request.post("/api/tenant/bonuses", {
      headers: authHeaders,
      data: {
        title: bonusTitles[i],
        description: `${bonusTitles[i]} bonus`,
        value: 50 + i * 10,
        sort_order: i,
      },
    });
    await assertOk(res, `create bonus ${bonusTitles[i]}`);
    const bonus = (await res.json()) as { id: string };
    bonusIds.push(bonus.id);
  }

  const assignGold6 = await request.put(`/api/tenant/term-options/${gold6.id}/bonuses`, {
    headers: authHeaders,
    data: { bonus_ids: [bonusIds[0], bonusIds[1]] },
  });
  await assertOk(assignGold6, "assign bonuses to gold 6 month term");

  const assignGold12 = await request.put(`/api/tenant/term-options/${gold12.id}/bonuses`, {
    headers: authHeaders,
    data: { bonus_ids: [bonusIds[0], bonusIds[1], bonusIds[2]] },
  });
  await assertOk(assignGold12, "assign bonuses to gold 12 month term");

  const createTerms = async (title: string, content: string) => {
    const res = await request.post("/api/tenant/terms", {
      headers: authHeaders,
      data: { title, content },
    });
    await assertOk(res, `create terms ${title}`);
    return (await res.json()) as { id: string };
  };
  const t1 = await createTerms("Standard Membership Agreement", "Standard terms for e2e.");
  const t2 = await createTerms("Direct Debit Agreement", "DD terms for e2e.");

  const attach1 = await request.post(`/api/tenant/membership-types/${gold.id}/terms/${t1.id}`, { headers: authHeaders });
  const attach2 = await request.post(`/api/tenant/membership-types/${gold.id}/terms/${t2.id}`, { headers: authHeaders });
  await assertOk(attach1, "attach terms to membership type (first)");
  await assertOk(attach2, "attach terms to membership type (second)");

  const serviceRes = await request.post("/api/tenant/services", {
    headers: authHeaders,
    data: { name: "Starter Service", description: "e2e service", price: 99, tax_rate: 10, sort_order: 1 },
  });
  await assertOk(serviceRes, "create service");

  const clientRes = await request.post("/api/tenant/clients", {
    headers: authHeaders,
    data: { name: "Ben", email: `ben-${now}@example.com`, phone: "0400000000" },
  });
  await assertOk(clientRes, "create client");
  const client = (await clientRes.json()) as { id: string };

  const membershipPayloadBase = {
    client_id: client.id,
    membership_type_id: gold.id,
    term_option_id: gold6.id,
    start_date: new Date().toISOString().slice(0, 10),
    payment_amount: 100,
  };
  let membershipCreated = false;
  let membershipLastError = "";
  const paymentOptions = [
    { payment_method: "card", payment_frequency: "monthly" },
    { payment_method: "card", payment_frequency: "upfront" },
    { payment_method: "direct_debit", payment_frequency: "monthly" },
  ];
  for (const option of paymentOptions) {
    const membershipRes = await request.post("/api/tenant/memberships", {
      headers: authHeaders,
      data: { ...membershipPayloadBase, ...option },
    });
    if (membershipRes.ok()) {
      membershipCreated = true;
      break;
    }
    membershipLastError = `${membershipRes.status()} ${await responseBodyPreview(membershipRes)}`;
  }
  if (!membershipCreated) {
    // Do not fail context bootstrap when membership creation is blocked on live env.
    // This avoids cascading auth/page failures and lets tests classify downstream behavior.
    console.warn(`e2e bootstrap warning: baseline membership not created (${membershipLastError || "unknown error"})`);
  }

  return {
    email,
    password,
    tenantSlug,
    accessToken,
    refreshToken,
    goldMembershipSlug: gold.slug,
    baselineMembershipCreated: membershipCreated,
  };
}

export async function getE2EContext(request: APIRequestContext): Promise<E2EContext> {
  if (cachedContext) return cachedContext;
  if (!cachedContextPromise) {
    cachedContextPromise = registerTenant(request)
      .then((ctx) => {
        cachedContext = ctx;
        return ctx;
      })
      .catch((err) => {
        cachedContextPromise = null;
        throw err;
      });
  }
  return cachedContextPromise;
}

export async function loginAsAdmin(page: Page) {
  const ctx = await getE2EContext(page.request);
  await page.goto("/login");
  await page.evaluate(({ accessToken, refreshToken }) => {
    localStorage.setItem("access_token", accessToken);
    localStorage.setItem("refresh_token", refreshToken);
  }, { accessToken: ctx.accessToken, refreshToken: ctx.refreshToken });
  await page.goto("/dashboard");
  await page.waitForLoadState("networkidle");
}

export async function loginViaForm(page: Page) {
  const ctx = await getE2EContext(page.request);
  await page.goto("/login");
  await page.fill('input[type="email"]', ctx.email);
  await page.fill('input[type="password"]', ctx.password);
  await page.click('button:has-text("Log in")');
  await page.waitForURL("**/dashboard", { timeout: 20000 });
}

export async function getAuthToken(request: APIRequestContext): Promise<string> {
  const ctx = await getE2EContext(request);
  return ctx.accessToken;
}
