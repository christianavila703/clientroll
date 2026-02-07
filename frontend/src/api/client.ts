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
    // Try token refresh
    const refreshed = await tryRefreshToken();
    if (refreshed) {
      return request<T>(endpoint, options);
    }
    // Refresh failed — clear auth and redirect
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    window.location.href = "/login";
    throw new ApiError("Unauthorised", 401);
  }

  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: "Request failed" }));
    throw new ApiError(err.error || "Request failed", response.status);
  }

  return response.json();
}

async function tryRefreshToken(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) return false;

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
  }
}

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? match[2] : null;
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
  me: () => request("/tenant/me"),

  // Settings
  getAgreementSettings: () => request("/tenant/settings/agreement"),
  updateAgreementSettings: (data: unknown) =>
    request("/tenant/settings/agreement", { method: "PUT", body: data }),
  getTenantSettings: () => request("/tenant/settings/tenant"),
  updateTenantSettings: (data: unknown) =>
    request("/tenant/settings/tenant", { method: "PUT", body: data }),

  // Staff
  listStaff: () => request("/tenant/staff"),
  createStaff: (data: unknown) =>
    request("/tenant/staff", { method: "POST", body: data }),
  getStaff: (id: string) => request(`/tenant/staff/${id}`),

  // Clients
  listClients: (search?: string) =>
    request(`/tenant/clients${search ? `?search=${encodeURIComponent(search)}` : ""}`),
  createClient: (data: unknown) =>
    request("/tenant/clients", { method: "POST", body: data }),
  getClient: (id: string) => request(`/tenant/clients/${id}`),
  updateClient: (id: string, data: unknown) =>
    request(`/tenant/clients/${id}`, { method: "PUT", body: data }),
  getClientMemberships: (id: string) =>
    request(`/tenant/clients/${id}/memberships`),

  // Membership Types
  listMembershipTypes: () => request("/tenant/membership-types"),
  createMembershipType: (data: unknown) =>
    request("/tenant/membership-types", { method: "POST", body: data }),
  getMembershipType: (id: string) => request(`/tenant/membership-types/${id}`),
  duplicateMembershipType: (id: string) =>
    request(`/tenant/membership-types/${id}/duplicate`, { method: "POST" }),
  createTermOption: (mtId: string, data: unknown) =>
    request(`/tenant/membership-types/${mtId}/term-options`, { method: "POST", body: data }),
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

  // Terms & Conditions
  listTerms: () => request("/tenant/terms"),
  createTerms: (data: unknown) =>
    request("/tenant/terms", { method: "POST", body: data }),
  updateTerms: (id: string, data: unknown) =>
    request(`/tenant/terms/${id}`, { method: "PUT", body: data }),
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

  // Platform admin
  listTenants: () => request("/platform/tenants"),
  createTenant: (data: unknown) =>
    request("/platform/tenants", { method: "POST", body: data }),
  getTenant: (id: string) => request(`/platform/tenants/${id}`),
  updateTenant: (id: string, data: unknown) =>
    request(`/platform/tenants/${id}`, { method: "PUT", body: data }),
};

export { ApiError };
export default api;
