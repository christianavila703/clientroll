const API_BASE = "/api";

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
}

class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

export interface EmailTemplateConfig {
  id: string;
  name: string;
  subject: string;
  body: string;
  category: "client" | "payments" | "internal" | "reminders";
  description: string;
  variables: string[];
  enabled: boolean;
  updated_at?: string | null;
}

async function request<T>(
  endpoint: string,
  options: RequestOptions = {}
): Promise<T> {
  const { method = "GET", body, headers = {} } = options;

  const token = localStorage.getItem("access_token");
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const csrfToken = getCookie("csrf_token");
  if (csrfToken && method !== "GET") {
    headers["X-CSRF-Token"] = csrfToken;
  }

  const config: RequestInit = {
    method,
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    credentials: "include",
  };

  if (body && method !== "GET") {
    config.body = JSON.stringify(body);
  }

  const response = await fetch(`${API_BASE}${endpoint}`, config);

  if (response.status === 401) {
    // Don't auto-redirect for auth endpoints — let the UI show the error
    const isAuthEndpoint = endpoint.startsWith("/auth/") || endpoint === "/register";
    if (!isAuthEndpoint) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        return request<T>(endpoint, options);
      }
      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/login";
    }
    const err = await response.json().catch(() => ({ error: "Invalid credentials" }));
    throw new ApiError(err.error || "Unauthorised", 401);
  }

  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: "Request failed" }));
    throw new ApiError(err.error || "Request failed", response.status);
  }

  return response.json();
}

// Prevent multiple tabs from refreshing simultaneously
let refreshPromise: Promise<boolean> | null = null;

async function tryRefreshToken(): Promise<boolean> {
  // If a refresh is already in progress, wait for it
  if (refreshPromise) return refreshPromise;

  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return false;

  refreshPromise = (async () => {
    try {
      const response = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });

      if (!response.ok) return false;

      const data = await response.json();
      localStorage.setItem("access_token", data.access_token);
      localStorage.setItem("refresh_token", data.refresh_token);
      return true;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? match[2] : null;
}

async function uploadFile(file: File, entityType = "general"): Promise<{ id: string; url: string; public_url: string }> {
  const form = new FormData();
  form.append("file", file);
  form.append("entity_type", entityType);
  const headers: Record<string, string> = {};
  const token = localStorage.getItem("access_token");
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const csrf = getCookie("csrf_token");
  if (csrf) headers["X-CSRF-Token"] = csrf;
  const res = await fetch(`${API_BASE}/tenant/upload`, { method: "POST", headers, body: form, credentials: "include" });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Upload failed" }));
    throw new ApiError(err.error || "Upload failed", res.status);
  }
  return res.json();
}

// API methods
export const api = {
  // Auth
  login: (email: string, password: string) =>
    request("/auth/login", { method: "POST", body: { email, password } }),

  platformLogin: (email: string, password: string) =>
    request("/auth/platform-login", {
      method: "POST",
      body: { email, password },
    }),

  logout: () => request("/auth/logout", { method: "POST" }),

  // Current user
  me: () => request("/me"),

  // Settings
  getAgreementSettings: () => request("/tenant/settings/agreement"),
  updateAgreementSettings: (data: unknown) =>
    request("/tenant/settings/agreement", { method: "PUT", body: data }),
  lookupBusinessByName: (name: string) =>
    request(`/tenant/settings/business-lookup?name=${encodeURIComponent(name)}`),
  getTenantSettings: () => request("/tenant/settings/tenant"),
  updateTenantSettings: (data: unknown) =>
    request("/tenant/settings/tenant", { method: "PUT", body: data }),
  getEmailTemplates: () =>
    request<{ templates: EmailTemplateConfig[] }>("/tenant/settings/email-templates"),
  updateEmailTemplate: (
    templateKey: string,
    data: { enabled?: boolean; subject?: string; body?: string }
  ) =>
    request<EmailTemplateConfig>(`/tenant/settings/email-templates/${encodeURIComponent(templateKey)}`, {
      method: "PUT",
      body: data,
    }),
  toggleEmailTemplatesCategory: (category: string, enabled: boolean) =>
    request<{ category: string; enabled: boolean; updated: number }>(
      `/tenant/settings/email-templates/category/${encodeURIComponent(category)}/toggle-all`,
      { method: "PUT", body: { enabled } }
    ),
  completeSetup: () =>
    request("/tenant/complete-setup", { method: "POST" }),

  // Staff
  listStaff: () => request("/tenant/staff"),
  createStaff: (data: unknown) =>
    request("/tenant/staff", { method: "POST", body: data }),
  getStaff: (id: string) => request(`/tenant/staff/${id}`),

  // Clients
  listClients: (search?: string, status?: string) => {
    const params = new URLSearchParams();
    if (search) params.set("search", search);
    if (status && status !== "All") params.set("status", status.toLowerCase());
    const qs = params.toString();
    return request(`/tenant/clients${qs ? `?${qs}` : ""}`);
  },
  createClient: (data: unknown) =>
    request("/tenant/clients", { method: "POST", body: data }),
  getClient: (id: string) => request(`/tenant/clients/${id}`),
  updateClient: (id: string, data: unknown) =>
    request(`/tenant/clients/${id}`, { method: "PUT", body: data }),
  deleteClient: (id: string) =>
    request(`/tenant/clients/${id}`, { method: "DELETE" }),
  bulkDeleteClients: (clientIds: string[]) =>
    request(`/tenant/clients/bulk-delete`, { method: "POST", body: { client_ids: clientIds } }),
  getClientMemberships: (id: string) =>
    request(`/tenant/clients/${id}/memberships`),
  getClientMandateStatus: (id: string) =>
    request(`/tenant/clients/${id}/mandate-status`),
  sendClientMandateLink: (id: string) =>
    request(`/tenant/clients/${id}/send-mandate-link`, { method: "POST", body: {} }),
  createClientStripeSetupSession: (id: string) =>
    request(`/tenant/clients/${id}/stripe-setup-session`, { method: "POST", body: {} }),

  // Memberships
  createMembership: (data: unknown) =>
    request("/tenant/memberships", { method: "POST", body: data }),
  createMembershipInvite: (data: unknown) =>
    request("/tenant/signup-invites", { method: "POST", body: data }),
  getActiveMembershipInvite: (clientId: string) =>
    request(`/tenant/signup-invites/clients/${clientId}/active`),
  revokeMembershipInvite: (inviteId: string) =>
    request(`/tenant/signup-invites/${inviteId}/revoke`, { method: "POST", body: {} }),
  updateMembershipContinueAfterTerm: (membershipId: string, continueAfterTerm: boolean) =>
    request(`/tenant/memberships/${membershipId}/continue-after-term`, {
      method: "PUT",
      body: { continue_after_term: continueAfterTerm },
    }),
  getSignupLinks: () => request("/tenant/signup-links"),

  // Membership Types
  listMembershipTypes: () => request("/tenant/membership-types"),
  createMembershipType: (data: unknown) =>
    request("/tenant/membership-types", { method: "POST", body: data }),
  updateMembershipType: (id: string, data: unknown) =>
    request(`/tenant/membership-types/${id}`, { method: "PUT", body: data }),
  getMembershipType: (id: string) => request(`/tenant/membership-types/${id}`),
  duplicateMembershipType: (id: string) =>
    request(`/tenant/membership-types/${id}/duplicate`, { method: "POST" }),
  createTermOption: (mtId: string, data: unknown) =>
    request(`/tenant/membership-types/${mtId}/term-options`, { method: "POST", body: data }),
  updateTermOption: (mtId: string, toId: string, data: unknown) =>
    request(`/tenant/membership-types/${mtId}/term-options/${toId}`, { method: "PUT", body: data }),
  deleteTermOption: (id: string) =>
    request(`/tenant/term-options/${id}`, { method: "DELETE" }),

  // Bonuses
  listBonuses: () => request("/tenant/bonuses"),
  createBonus: (data: unknown) =>
    request("/tenant/bonuses", { method: "POST", body: data }),
  updateBonus: (id: string, data: unknown) =>
    request(`/tenant/bonuses/${id}`, { method: "PUT", body: data }),
  deleteBonus: (id: string) =>
    request(`/tenant/bonuses/${id}`, { method: "DELETE" }),
  setTermOptionBonuses: (termOptionId: string, bonusIds: string[]) =>
    request(`/tenant/term-options/${termOptionId}/bonuses`, {
      method: "PUT",
      body: { bonus_ids: bonusIds },
    }),
  getTermOptionBonuses: (termOptionId: string) =>
    request(`/tenant/term-options/${termOptionId}/bonuses`),

  // Services
  listServices: () => request("/tenant/services"),
  createService: (data: unknown) =>
    request("/tenant/services", { method: "POST", body: data }),
  updateService: (id: string, data: unknown) =>
    request(`/tenant/services/${id}`, { method: "PUT", body: data }),
  deleteService: (id: string) =>
    request(`/tenant/services/${id}`, { method: "DELETE" }),
  chargeService: (data: unknown) =>
    request("/tenant/services/charge", { method: "POST", body: data }),
  chargeServiceWithBankDetails: (data: unknown) =>
    request("/tenant/services/charge-with-bank-details", {
      method: "POST",
      body: data,
    }),
  chargeServiceStripe: (data: unknown) =>
    request("/tenant/services/charge-stripe", {
      method: "POST",
      body: data,
    }),
  listClientStripePaymentMethods: (clientId: string) =>
    request(`/tenant/services/stripe-payment-methods/${clientId}`),
  listClientCharges: (clientId: string) =>
    request(`/tenant/services/charges/${clientId}`),

  // Reports
  getDashboardReport: () => request("/tenant/reports/dashboard"),
  getMonthlyRevenueReport: () => request("/tenant/reports/revenue/monthly"),
  getMembershipBreakdownReport: () => request("/tenant/reports/memberships/breakdown"),
  getPaymentStatusReport: () => request("/tenant/reports/payments/status"),
  getComprehensiveReport: () => request("/tenant/reports/comprehensive"),

  // Payment Monitoring (platform admin)
  getAlertConfig: (tenantId?: string) =>
    request(`/platform/monitoring/alert-config${tenantId ? `?tenant_id=${tenantId}` : ""}`),
  updateAlertConfig: (tenantId: string, data: unknown) =>
    request(`/platform/monitoring/alert-config?tenant_id=${tenantId}`, { method: "PUT", body: data }),
  getExpectedVsReceived: (tenantId: string) =>
    request(`/platform/monitoring/expected-vs-received?tenant_id=${tenantId}`),
  getWebhookHealth: (tenantId: string) =>
    request(`/platform/monitoring/webhook-health?tenant_id=${tenantId}`),
  getPaymentAging: (tenantId: string) =>
    request(`/platform/monitoring/payment-aging?tenant_id=${tenantId}`),
  getReconciliationReport: (tenantId: string) =>
    request(`/platform/monitoring/payment-reconciliation?tenant_id=${tenantId}`),
  runReconciliationNow: (tenantId: string) =>
    request(`/platform/monitoring/run-reconciliation?tenant_id=${tenantId}`, { method: "POST" }),
  runGCReconciliationNow: (tenantId: string) =>
    request(`/platform/monitoring/run-gc-reconciliation?tenant_id=${tenantId}`, { method: "POST" }),
  getWebhookAudit: (tenantId: string) =>
    request(`/platform/monitoring/webhook-audit?tenant_id=${tenantId}`),

  // Terms & Conditions
  listTerms: () => request("/tenant/terms"),
  createTerms: (data: unknown) =>
    request("/tenant/terms", { method: "POST", body: data }),
  updateTerms: (id: string, data: unknown) =>
    request(`/tenant/terms/${id}`, { method: "PUT", body: data }),
  deleteTerms: (id: string) =>
    request(`/tenant/terms/${id}`, { method: "DELETE" }),
  attachTerms: (mtId: string, termsId: string) =>
    request(`/tenant/membership-types/${mtId}/terms/${termsId}`, { method: "POST" }),
  detachTerms: (mtId: string, termsId: string) =>
    request(`/tenant/membership-types/${mtId}/terms/${termsId}`, { method: "DELETE" }),

  // Integrations
  listIntegrations: () => request("/tenant/integrations"),
  saveIntegration: (provider: string, data: unknown) =>
    request(`/tenant/integrations/${provider}`, { method: "PUT", body: data }),
  deleteIntegration: (provider: string) =>
    request(`/tenant/integrations/${provider}`, { method: "DELETE" }),
  getGoCardlessConnectUrl: (environment: "sandbox" | "live") =>
    request(`/tenant/gocardless/connect?environment=${environment}`) as Promise<{ redirect_url: string }>,
  getXeroConnectUrl: () =>
    request("/tenant/xero/connect") as Promise<{ redirect_url: string }>,
  getXeroStatus: () =>
    request("/tenant/xero/status") as Promise<{
      connected: boolean;
      is_active?: boolean;
      feature_enabled?: boolean;
      oauth_configured?: boolean;
      credit_notes_enabled?: boolean;
      needs_reauth?: boolean;
      xero_tenant_id?: string;
      xero_tenant_name?: string;
      default_revenue_account_code?: string;
      default_payment_account_code?: string;
      token_expiry?: string;
    }>,
  getXeroPendingSelection: (token: string) =>
    request(`/tenant/xero/pending-selection?token=${encodeURIComponent(token)}`) as Promise<{
      items: Array<{ tenant_id: string; tenant_name: string }>;
      expires_at: string;
    }>,
  updateXeroConfig: (data: { default_revenue_account_code: string; default_payment_account_code: string }) =>
    request("/tenant/xero/config", { method: "PUT", body: data }),
  disconnectXero: () =>
    request("/tenant/xero/disconnect", { method: "POST", body: {} }),
  selectXeroOrg: (data: { selection_token: string; xero_tenant_id: string }) =>
    request("/tenant/xero/select-org", { method: "POST", body: data }),
  listXeroSyncFailures: () =>
    request("/tenant/xero/sync-failures") as Promise<{ items: unknown[] }>,
  retryXeroSyncEvent: (eventId: string) =>
    request("/tenant/xero/sync/retry", { method: "POST", body: { event_id: eventId } }),

  // Platform admin
  listTenants: () => request("/platform/tenants"),
  createTenant: (data: unknown) =>
    request("/platform/tenants", { method: "POST", body: data }),
  getTenant: (id: string) => request(`/platform/tenants/${id}`),
  updateTenant: (id: string, data: unknown) =>
    request(`/platform/tenants/${id}`, { method: "PUT", body: data }),

  // Platform billing (platform admin)
  getPlatformBillingConfig: () => request("/platform/billing/config"),
  updatePlatformBillingConfig: (data: unknown) =>
    request("/platform/billing/config", { method: "PUT", body: data }),
  listPlatformBillingTenants: () => request("/platform/billing/tenants"),
  getPlatformBillingTenant: (tenantId: string) =>
    request(`/platform/billing/tenants/${tenantId}`),
  createPlatformBillingPortalSession: (tenantId: string, data?: unknown) =>
    request(`/platform/billing/tenants/${tenantId}/portal`, { method: "POST", body: data || {} }),

  // File uploads
  uploadImage: (file: File, entityType?: string) => uploadFile(file, entityType),
};

export { ApiError };
export default api;
