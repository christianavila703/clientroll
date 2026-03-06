import { APIRequestContext, Page } from "@playwright/test";

export interface E2EContext {
  email: string;
  password: string;
  tenantSlug: string;
  accessToken: string;
  refreshToken: string;
  goldMembershipSlug: string;
}

let cachedContext: E2EContext | null = null;

async function registerTenant(request: APIRequestContext): Promise<E2EContext> {
  const now = Date.now();
  const tenantSlug = `e2e-suite-${now}`;
  const email = `e2e-suite-${now}@example.com`;
  const password = "JourneyTest1!";

  const regRes = await request.post("/api/register", {
    data: {
      company_name: `E2E Suite ${now}`,
      slug: tenantSlug,
      first_name: "E2E",
      last_name: "Suite",
      email,
      password,
    },
  });
  if (!regRes.ok()) {
    const body = await regRes.text();
    throw new Error(`register failed: ${regRes.status()} ${body}`);
  }
  const regBody = (await regRes.json()) as {
    tokens: { access_token: string; refresh_token: string };
  };
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
    if (!res.ok()) throw new Error(`create membership type failed (${title})`);
    return (await res.json()) as { id: string; slug: string };
  };

  const gold = await createMt("Gold Membership", 1200, 1);
  const silver = await createMt("Silver Membership", 800, 2);
  const platinum = await createMt("Platinum Membership", 1600, 3);

  for (const mt of [gold, silver, platinum]) {
    const activateRes = await request.post(`/api/tenant/membership-types/${mt.id}/activate`, {
      headers: authHeaders,
    });
    if (!activateRes.ok()) throw new Error(`activate membership type failed: ${mt.id}`);
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
    if (!res.ok()) throw new Error(`create term failed for ${mtId}`);
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
    if (!res.ok()) throw new Error(`create bonus failed: ${bonusTitles[i]}`);
    const bonus = (await res.json()) as { id: string };
    bonusIds.push(bonus.id);
  }

  const assignGold6 = await request.put(`/api/tenant/term-options/${gold6.id}/bonuses`, {
    headers: authHeaders,
    data: { bonus_ids: [bonusIds[0], bonusIds[1]] },
  });
  if (!assignGold6.ok()) throw new Error("assign bonuses gold6 failed");

  const assignGold12 = await request.put(`/api/tenant/term-options/${gold12.id}/bonuses`, {
    headers: authHeaders,
    data: { bonus_ids: [bonusIds[0], bonusIds[1], bonusIds[2]] },
  });
  if (!assignGold12.ok()) throw new Error("assign bonuses gold12 failed");

  const createTerms = async (title: string, content: string) => {
    const res = await request.post("/api/tenant/terms", {
      headers: authHeaders,
      data: { title, content },
    });
    if (!res.ok()) throw new Error(`create terms failed: ${title}`);
    return (await res.json()) as { id: string };
  };
  const t1 = await createTerms("Standard Membership Agreement", "Standard terms for e2e.");
  const t2 = await createTerms("Direct Debit Agreement", "DD terms for e2e.");

  const attach1 = await request.post(`/api/tenant/membership-types/${gold.id}/terms/${t1.id}`, { headers: authHeaders });
  const attach2 = await request.post(`/api/tenant/membership-types/${gold.id}/terms/${t2.id}`, { headers: authHeaders });
  if (!attach1.ok() || !attach2.ok()) throw new Error("attach terms failed");

  const serviceRes = await request.post("/api/tenant/services", {
    headers: authHeaders,
    data: { name: "Starter Service", description: "e2e service", price: 99, tax_rate: 10, sort_order: 1 },
  });
  if (!serviceRes.ok()) throw new Error("create service failed");

  const clientRes = await request.post("/api/tenant/clients", {
    headers: authHeaders,
    data: { name: "Ben", email: `ben-${now}@example.com`, phone: "0400000000" },
  });
  if (!clientRes.ok()) throw new Error("create client failed");
  const client = (await clientRes.json()) as { id: string };

  const membershipRes = await request.post("/api/tenant/memberships", {
    headers: authHeaders,
    data: {
      client_id: client.id,
      membership_type_id: gold.id,
      term_option_id: gold6.id,
      start_date: new Date().toISOString().slice(0, 10),
      payment_method: "card",
      payment_frequency: "monthly",
      payment_amount: 100,
    },
  });
  if (!membershipRes.ok()) throw new Error("create baseline membership failed");

  return {
    email,
    password,
    tenantSlug,
    accessToken,
    refreshToken,
    goldMembershipSlug: gold.slug,
  };
}

export async function getE2EContext(request: APIRequestContext): Promise<E2EContext> {
  if (cachedContext) return cachedContext;
  cachedContext = await registerTenant(request);
  return cachedContext;
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
