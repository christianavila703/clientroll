import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import api, { ApiError } from "../../api/client";

type BillingConfig = {
  per_member_price_usd: number;
  currency: string;
  has_publishable_key: boolean;
  has_secret_key: boolean;
  has_webhook_secret: boolean;
  stripe_product_id?: string | null;
  stripe_price_id?: string | null;
  is_active: boolean;
};

type BillingConfigResponse = {
  config: BillingConfig;
  webhook_url: string;
  webhook_events: string[];
};

type Tenant = {
  id: string;
  name: string;
  slug: string;
  timezone: string;
  is_active: boolean;
  created_at: string;
};

type Subscription = {
  tenant_id: string;
  status: string;
  stripe_customer_id?: string | null;
  stripe_subscription_id?: string | null;
  active_member_count: number;
  current_monthly_amount_usd: number;
  current_period_start?: string | null;
  current_period_end?: string | null;
  has_valid_payment_method: boolean;
  card_brand?: string | null;
  card_last4?: string | null;
  card_exp_month?: number | null;
  card_exp_year?: number | null;
  billing_grace_until?: string | null;
};

type TenantBillingRow = {
  tenant: Tenant;
  subscription?: Subscription | null;
};

export default function PlatformBillingPage() {
  const [cfg, setCfg] = useState<BillingConfigResponse | null>(null);
  const [tenants, setTenants] = useState<TenantBillingRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [syncingProduct, setSyncingProduct] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const [form, setForm] = useState({
    per_member_price_usd: 1,
    currency: "USD",
    is_active: false,
    stripe_product_id: "",
    stripe_price_id: "",
    stripe_publishable_key: "",
    stripe_secret_key: "",
    stripe_webhook_secret: "",
  });

  const fetchAll = () => {
    setLoading(true);
    setError("");
    Promise.all([api.getPlatformBillingConfig(), api.listPlatformBillingTenants()])
      .then(([cfgRes, tenantsRes]) => {
        const c = cfgRes as BillingConfigResponse;
        setCfg(c);
        setForm({
          per_member_price_usd: c.config.per_member_price_usd ?? 1,
          currency: c.config.currency ?? "USD",
          is_active: !!c.config.is_active,
          stripe_product_id: c.config.stripe_product_id || "",
          stripe_price_id: c.config.stripe_price_id || "",
          stripe_publishable_key: "",
          stripe_secret_key: "",
          stripe_webhook_secret: "",
        });

        const t = tenantsRes as { tenants: TenantBillingRow[] };
        setTenants(t.tenants || []);
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : "Unable to load billing data."))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchAll();
  }, []);

  const keyStatus = useMemo(() => {
    const c = cfg?.config;
    if (!c) return null;
    return {
      publishable: c.has_publishable_key,
      secret: c.has_secret_key,
      webhook: c.has_webhook_secret,
    };
  }, [cfg]);

  const canSyncStripeProduct = useMemo(() => {
    const hasStoredKeys = !!(keyStatus?.publishable && keyStatus?.secret);
    const hasFormKeys = !!(form.stripe_publishable_key.trim() && form.stripe_secret_key.trim());
    return hasStoredKeys || hasFormKeys;
  }, [keyStatus, form.stripe_publishable_key, form.stripe_secret_key]);

  const saveConfig = async () => {
    setSaving(true);
    setMessage("");
    setError("");
    try {
      await api.updatePlatformBillingConfig({
        per_member_price_usd: Number(form.per_member_price_usd),
        currency: form.currency,
        is_active: form.is_active,
        stripe_product_id: form.stripe_product_id || null,
        stripe_price_id: form.stripe_price_id || null,
        stripe_publishable_key: form.stripe_publishable_key || "",
        stripe_secret_key: form.stripe_secret_key || "",
        stripe_webhook_secret: form.stripe_webhook_secret || "",
      });
      setMessage("Billing config saved");
      fetchAll();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unable to save billing configuration.");
    } finally {
      setSaving(false);
    }
  };

  const syncStripeProduct = async () => {
    setSyncingProduct(true);
    setMessage("");
    setError("");
    try {
      const res = await api.syncPlatformBillingStripeProduct({
        per_member_price_usd: Number(form.per_member_price_usd),
        currency: form.currency,
        stripe_publishable_key: form.stripe_publishable_key || "",
        stripe_secret_key: form.stripe_secret_key || "",
      }) as {
        config: BillingConfig;
        sync?: { created_product?: boolean; created_price?: boolean; product_id?: string; price_id?: string };
        migration?: { total?: number; migrated?: number; skipped?: number; failed?: number; errors?: string[] };
      };

      const sync = res.sync;
      const parts: string[] = [];
      if (sync?.created_product) parts.push("product created");
      else if (form.stripe_product_id || res.config.stripe_product_id) parts.push("product updated");
      if (sync?.created_price) parts.push("new price created");
      else parts.push("price unchanged");

      const mig = res.migration;
      if (mig && (mig.total ?? 0) > 0) {
        parts.push(`${mig.migrated ?? 0} subscription(s) migrated`);
        if ((mig.skipped ?? 0) > 0) parts.push(`${mig.skipped} skipped`);
        if ((mig.failed ?? 0) > 0) parts.push(`${mig.failed} failed`);
      }
      setMessage(parts.length ? `Stripe sync complete: ${parts.join(", ")}` : "Stripe product synced");

      if (mig?.errors?.length) {
        setError(mig.errors.slice(0, 3).join(" · "));
      }

      setForm((prev) => ({
        ...prev,
        stripe_product_id: res.config.stripe_product_id || "",
        stripe_price_id: res.config.stripe_price_id || "",
        stripe_publishable_key: "",
        stripe_secret_key: "",
      }));
      fetchAll();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unable to sync Stripe product.");
    } finally {
      setSyncingProduct(false);
    }
  };

  const openPortal = async (tenantId: string) => {
    setError("");
    try {
      const res = await api.createPlatformBillingPortalSession(tenantId, { return_url: window.location.href });
      const url = (res as any)?.url;
      if (url) window.open(url, "_blank", "noopener,noreferrer");
      else setError("Portal session did not return a URL");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unable to open billing portal.");
    }
  };

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Loading billing...</div>;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="page-title">Platform Billing</h1>
          <p className="mt-1 text-sm text-gray-500">Configure Stripe billing and review tenant billing status.</p>
        </div>
        <button className="btn-secondary" onClick={fetchAll}>Refresh</button>
      </div>

      {(error || message) && (
        <div className={`mb-4 rounded-lg px-4 py-3 text-sm ${error ? "border border-red-200 bg-red-50 text-red-700" : "border border-green-200 bg-green-50 text-green-700"}`}>
          {error || message}
        </div>
      )}

      {/* Config */}
      <div className="card px-5 py-4 mb-6">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold text-gray-900">Billing configuration</h2>
            <p className="text-[13px] text-gray-500 mt-0.5">
              Webhook URL: <span className="font-mono text-[12px]">{cfg?.webhook_url || "—"}</span>
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span className={`badge ${form.is_active ? "badge-green" : "badge-gray"}`}>
              {form.is_active ? "Active" : "Inactive"}
            </span>
          </div>
        </div>

        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Per member price (USD)</span>
            <input className="input-field font-mono text-[13px]" type="number" step="0.01" value={form.per_member_price_usd}
              onChange={(e) => setForm({ ...form, per_member_price_usd: Number(e.target.value) })} />
          </label>
          <label className="block">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Currency</span>
            <input className="input-field font-mono text-[13px]" value={form.currency} onChange={(e) => setForm({ ...form, currency: e.target.value })} />
          </label>

          <div className="sm:col-span-2 grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
              <p className="text-[11px] font-semibold text-gray-600">Keys status</p>
              <p className="mt-1 text-[12px] text-gray-600">
                Publishable: <b>{keyStatus?.publishable ? "set" : "missing"}</b><br />
                Secret: <b>{keyStatus?.secret ? "set" : "missing"}</b><br />
                Webhook: <b>{keyStatus?.webhook ? "set" : "missing"}</b>
              </p>
            </div>
            <label className="block sm:col-span-2">
              <span className="mb-1 block text-[13px] font-medium text-gray-700">Enable platform billing</span>
              <div className="flex items-center gap-2">
                <input type="checkbox" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} />
                <span className="text-[13px] text-gray-600">When enabled, billing jobs and enforcement can be applied.</span>
              </div>
            </label>
          </div>

          <label className="block sm:col-span-2">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Stripe publishable key</span>
            <input className="input-field font-mono text-[13px]" value={form.stripe_publishable_key} onChange={(e) => setForm({ ...form, stripe_publishable_key: e.target.value })} placeholder={keyStatus?.publishable ? "Leave blank to keep existing" : "pk_live_..."} />
          </label>
          <label className="block sm:col-span-2">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Stripe secret key</span>
            <input className="input-field font-mono text-[13px]" value={form.stripe_secret_key} onChange={(e) => setForm({ ...form, stripe_secret_key: e.target.value })} placeholder={keyStatus?.secret ? "Leave blank to keep existing" : "sk_live_..."} />
          </label>
          <label className="block sm:col-span-2">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Stripe webhook signing secret</span>
            <input className="input-field font-mono text-[13px]" value={form.stripe_webhook_secret} onChange={(e) => setForm({ ...form, stripe_webhook_secret: e.target.value })} placeholder={keyStatus?.webhook ? "Leave blank to keep existing" : "whsec_..."} />
          </label>

          <label className="block sm:col-span-2">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Stripe product ID</span>
            <input className="input-field font-mono text-[13px]" value={form.stripe_product_id} onChange={(e) => setForm({ ...form, stripe_product_id: e.target.value })} placeholder="prod_... (filled by sync or enter manually)" />
          </label>
          <label className="block sm:col-span-2">
            <span className="mb-1 block text-[13px] font-medium text-gray-700">Stripe price ID (per member / month)</span>
            <input className="input-field font-mono text-[13px]" value={form.stripe_price_id} onChange={(e) => setForm({ ...form, stripe_price_id: e.target.value })} placeholder="price_... (filled by sync or enter manually)" />
          </label>

          <div className="sm:col-span-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between rounded-lg border border-indigo-100 bg-indigo-50/50 px-4 py-3">
            <div>
              <p className="text-[13px] font-medium text-gray-900">Stripe product &amp; price</p>
              <p className="text-[12px] text-gray-600 mt-0.5">
                Enter publishable and secret keys above, then create or update the recurring per-member product in Stripe.
                Changing the price creates a new Stripe price, archives the previous one, and migrates existing tenant subscriptions.
              </p>
            </div>
            <button
              type="button"
              className="btn-secondary shrink-0"
              onClick={syncStripeProduct}
              disabled={!canSyncStripeProduct || syncingProduct}
            >
              {syncingProduct
                ? "Syncing..."
                : form.stripe_product_id
                  ? "Update Stripe product"
                  : "Create Stripe product"}
            </button>
          </div>
        </div>

        <div className="mt-5 flex items-center justify-between gap-3">
          <details className="text-[13px] text-gray-600">
            <summary className="cursor-pointer font-medium">Webhook events to enable</summary>
            <div className="mt-2 rounded-md border border-gray-200 bg-white p-3">
              <pre className="whitespace-pre-wrap text-[12px] text-gray-600">{(cfg?.webhook_events || []).join("\n")}</pre>
            </div>
          </details>

          <button className="btn-primary" onClick={saveConfig} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      {/* Tenant billing list */}
      <div className="card overflow-hidden p-0">
        <div className="px-4 py-3 border-b border-gray-200 flex items-center justify-between">
          <p className="text-[13px] font-semibold text-gray-900">Tenants</p>
          <Link to="/platform/tenants/new" className="btn-secondary text-[12px]">+ Create tenant</Link>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-gray-500">Tenant</th>
                <th className="px-4 py-3 text-left font-medium text-gray-500">Status</th>
                <th className="px-4 py-3 text-left font-medium text-gray-500">Members</th>
                <th className="px-4 py-3 text-left font-medium text-gray-500">Monthly (USD)</th>
                <th className="px-4 py-3 text-left font-medium text-gray-500">Card</th>
                <th className="px-4 py-3 text-right font-medium text-gray-500">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {tenants.map((row) => {
                const s = row.subscription;
                const card = s?.has_valid_payment_method
                  ? `${s.card_brand || "card"} •••• ${s.card_last4 || "—"} exp ${s.card_exp_month || "—"}/${s.card_exp_year || "—"}`
                  : "No valid card";
                return (
                  <tr key={row.tenant.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3">
                      <p className="font-medium text-gray-900">{row.tenant.name}</p>
                      <p className="text-[11px] text-gray-500 font-mono">{row.tenant.slug}</p>
                    </td>
                    <td className="px-4 py-3 text-gray-700">
                      {s?.status || "—"}
                      {s?.status === "past_due" && s.billing_grace_until && (
                        <div className="text-[11px] text-amber-600">Grace until {new Date(s.billing_grace_until).toLocaleDateString()}</div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-700 tabular-nums">{s ? s.active_member_count : "—"}</td>
                    <td className="px-4 py-3 text-gray-700 tabular-nums">{s ? `$${Number(s.current_monthly_amount_usd || 0).toFixed(2)}` : "—"}</td>
                    <td className="px-4 py-3 text-[12px] text-gray-600">{card}</td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end gap-2">
                        <Link to={`/platform/billing/tenants/${row.tenant.id}`} className="btn-secondary text-[12px]">Details</Link>
                        <button className="btn-secondary text-[12px]" onClick={() => openPortal(row.tenant.id)} disabled={!s?.stripe_customer_id}>Portal</button>
                      </div>
                    </td>
                  </tr>
                );
              })}
              {tenants.length === 0 && (
                <tr>
                  <td className="px-4 py-10 text-center text-gray-500" colSpan={6}>No tenants found.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

