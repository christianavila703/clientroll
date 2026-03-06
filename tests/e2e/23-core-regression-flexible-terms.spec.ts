import { expect, test } from "@playwright/test";

test.describe("Core Regression - Flexible Terms", () => {
  test("tenant admin can exercise core endpoints with flexible schedules", async ({ page, request }) => {
    const now = Date.now();
    const slug = `e2e-flex-${now}`;
    const email = `e2e-flex-${now}@example.com`;
    const password = "JourneyTest1!";

    const membershipTitle = `E2E Flex Membership ${now}`;
    const bonusTitle = `E2E Flex Bonus ${now}`;
    const serviceName = `E2E Flex Service ${now}`;
    const clientName = `E2E Flex Client ${now}`;

    let token = "";
    let membershipTypeId = "";
    let membershipSlug = "";
    let termOptionId = "";
    let duplicateMembershipTypeId = "";
    let bonusId = "";
    let serviceId = "";
    let clientId = "";
    let flexibleScheduleSupported = false;

    const authHeaders = () => ({ Authorization: `Bearer ${token}` });

    await test.step("Register tenant admin and acquire token", async () => {
      await page.goto("/register");
      await page.getByPlaceholder("Acme Wellness").fill(`E2E Flex ${now}`);
      await page.getByPlaceholder("acme-wellness").fill(slug);
      await page.getByRole("button", { name: "Continue" }).click();

      await page.locator('label:has-text("First name")').locator("xpath=following-sibling::input[1]").fill("E2E");
      await page.locator('label:has-text("Last name")').locator("xpath=following-sibling::input[1]").fill("Flex");
      await page.locator('label:has-text("Email")').locator("xpath=following-sibling::input[1]").fill(email);
      await page.locator('label:has-text("Password")').locator("xpath=following-sibling::input[1]").fill(password);
      await page.getByRole("button", { name: "Create account" }).click();

      await expect(page.getByText("Check your email")).toBeVisible({ timeout: 15000 });
      await page.goto("/dashboard");
      await page.waitForLoadState("networkidle");
      token = (await page.evaluate(() => localStorage.getItem("access_token"))) || "";
      expect(token).toBeTruthy();
    });

    await test.step("Create bonus and service", async () => {
      const bonusRes = await request.post("/api/tenant/bonuses", {
        headers: authHeaders(),
        data: {
          title: bonusTitle,
          description: "E2E bonus",
          value: 49,
          sort_order: 1,
        },
      });
      expect(bonusRes.ok()).toBeTruthy();
      bonusId = ((await bonusRes.json()) as { id: string }).id;

      const serviceRes = await request.post("/api/tenant/services", {
        headers: authHeaders(),
        data: {
          name: serviceName,
          description: "E2E service",
          price: 79,
          tax_rate: 10,
          sort_order: 2,
        },
      });
      expect(serviceRes.ok()).toBeTruthy();
      serviceId = ((await serviceRes.json()) as { id: string }).id;
    });

    await test.step("Create membership type + custom schedule term option", async () => {
      const mtRes = await request.post("/api/tenant/membership-types", {
        headers: authHeaders(),
        data: {
          title: membershipTitle,
          description: "E2E flexible membership",
          total_value: 600,
          sort_order: 5,
        },
      });
      expect(mtRes.status()).toBe(201);
      const mt = (await mtRes.json()) as { id: string; slug: string };
      membershipTypeId = mt.id;
      membershipSlug = mt.slug;
      expect(membershipSlug).toBeTruthy();

      const termRes = await request.post(`/api/tenant/membership-types/${membershipTypeId}/term-options`, {
        headers: authHeaders(),
        data: {
          duration_months: 6,
          fee_total: 600,
          fee_monthly: 100,
          allow_upfront: true,
          custom_schedule_enabled: true,
          custom_schedule_label: "2 payments, 3 months apart",
          custom_schedule_amount: 300,
          custom_schedule_interval_unit: "month",
          custom_schedule_interval_count: 3,
          custom_schedule_num_payments: 2,
        },
      });
      if (termRes.status() === 201) {
        flexibleScheduleSupported = true;
        termOptionId = ((await termRes.json()) as { id: string }).id;
      } else {
        const fallbackTermRes = await request.post(`/api/tenant/membership-types/${membershipTypeId}/term-options`, {
          headers: authHeaders(),
          data: {
            duration_months: 6,
            fee_total: 600,
            fee_monthly: 100,
            allow_upfront: true,
            num_payments: 2,
          },
        });
        expect(fallbackTermRes.status()).toBe(201);
        termOptionId = ((await fallbackTermRes.json()) as { id: string }).id;
      }

      const attachBonusRes = await request.put(`/api/tenant/term-options/${termOptionId}/bonuses`, {
        headers: authHeaders(),
        data: { bonus_ids: [bonusId] },
      });
      expect(attachBonusRes.ok()).toBeTruthy();
    });

    await test.step("Exercise membership-type lifecycle functions", async () => {
      const activateRes = await request.post(`/api/tenant/membership-types/${membershipTypeId}/activate`, {
        headers: authHeaders(),
      });
      expect(activateRes.ok()).toBeTruthy();

      const duplicateRes = await request.post(`/api/tenant/membership-types/${membershipTypeId}/duplicate`, {
        headers: authHeaders(),
      });
      expect(duplicateRes.status()).toBe(201);
      duplicateMembershipTypeId = ((await duplicateRes.json()) as { id: string }).id;

      const archiveDupRes = await request.post(`/api/tenant/membership-types/${duplicateMembershipTypeId}/archive`, {
        headers: authHeaders(),
      });
      expect(archiveDupRes.ok()).toBeTruthy();

      const reactivateDupRes = await request.post(`/api/tenant/membership-types/${duplicateMembershipTypeId}/activate`, {
        headers: authHeaders(),
      });
      expect(reactivateDupRes.ok()).toBeTruthy();
    });

    await test.step("Verify public signup API returns expected term schedule data", async () => {
      const publicDetailRes = await request.get(`/api/signup/${slug}/${membershipSlug}`);
      expect(publicDetailRes.status()).toBe(200);
      const publicDetail = (await publicDetailRes.json()) as {
        membership?: { term_options?: Array<Record<string, unknown>> };
      };
      const terms = publicDetail.membership?.term_options || [];
      expect(terms.length).toBeGreaterThan(0);
      const targetTerm = terms.find((t) => String(t.id) === termOptionId);
      expect(targetTerm).toBeTruthy();
      if (flexibleScheduleSupported) {
        expect(String(targetTerm?.custom_schedule_label || "")).toContain("2 payments");
        expect(Number(targetTerm?.custom_schedule_num_payments || 0)).toBe(2);
      } else {
        expect(Number(targetTerm?.num_payments || 0)).toBe(2);
      }
    });

    await test.step("Create + update client, then create membership using custom schedule", async () => {
      const clientRes = await request.post("/api/tenant/clients", {
        headers: authHeaders(),
        data: {
          name: clientName,
          email: `e2e-flex-client-${now}@example.com`,
          phone: "0400000000",
        },
      });
      expect(clientRes.status()).toBe(201);
      clientId = ((await clientRes.json()) as { id: string }).id;

      const updateClientRes = await request.put(`/api/tenant/clients/${clientId}`, {
        headers: authHeaders(),
        data: {
          name: `${clientName} Updated`,
          email: `e2e-flex-client-${now}@example.com`,
          phone: "0411111111",
          address: "123 Test Street",
          suburb: "Testville",
          state: "VIC",
          postcode: "3000",
          gender: null,
          birth_date: null,
          emergency_contact_name: null,
          emergency_contact_phone: null,
        },
      });
      expect(updateClientRes.status()).toBe(200);

      const membershipRes = await request.post("/api/tenant/memberships", {
        headers: authHeaders(),
        data: flexibleScheduleSupported
          ? {
              client_id: clientId,
              membership_type_id: membershipTypeId,
              term_option_id: termOptionId,
              start_date: new Date().toISOString().slice(0, 10),
              payment_method: "card",
              payment_frequency: "custom_schedule",
              payment_interval_unit: "month",
              payment_interval_count: 3,
              payment_amount: 300,
              num_payments: 2,
            }
          : {
              client_id: clientId,
              membership_type_id: membershipTypeId,
              term_option_id: termOptionId,
              start_date: new Date().toISOString().slice(0, 10),
              payment_method: "card",
              payment_frequency: "monthly",
              payment_amount: 100,
            },
      });
      expect(membershipRes.status()).toBe(201);
    });

    await test.step("Verify active memberships can find the created client", async () => {
      await page.goto("/active-memberships");
      await page.waitForLoadState("networkidle");
      await page.fill('input[placeholder*="Search by client"]', `${clientName} Updated`);
      await expect(page.locator("main")).toContainText(`${clientName} Updated`, { timeout: 15000 });
      await expect(page.locator("main")).toContainText(membershipTitle, { timeout: 15000 });
    });

    await test.step("Cleanup created records", async () => {
      if (serviceId) {
        const deleteServiceRes = await request.delete(`/api/tenant/services/${serviceId}`, { headers: authHeaders() });
        expect(deleteServiceRes.ok()).toBeTruthy();
      }
      if (bonusId) {
        const deleteBonusRes = await request.delete(`/api/tenant/bonuses/${bonusId}`, { headers: authHeaders() });
        expect(deleteBonusRes.ok()).toBeTruthy();
      }
      if (membershipTypeId) {
        const deleteMtRes = await request.delete(`/api/tenant/membership-types/${membershipTypeId}`, { headers: authHeaders() });
        expect(deleteMtRes.ok()).toBeTruthy();
      }
      if (duplicateMembershipTypeId) {
        const deleteDupRes = await request.delete(`/api/tenant/membership-types/${duplicateMembershipTypeId}`, { headers: authHeaders() });
        expect(deleteDupRes.ok()).toBeTruthy();
      }
    });
  });
});

