import { useState, useEffect } from "react";
import api, { ApiError } from "../../api/client";
import ImageUpload from "../common/ImageUpload";

interface AgreementData {
  company_name?: string;
  business_name?: string;
  abn_acn?: string;
  trading_name?: string;
  address?: string;
  state?: string;
  postcode?: string;
  email?: string;
  phone?: string;
}

export default function SettingsPage() {
  const [tab, setTab] = useState<"agreement" | "tenant" | "integrations">("agreement");

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Settings</h1>
          <p className="text-[13px] text-gray-500 mt-0.5">Manage your organisation settings</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {([
          { key: "agreement" as const, label: "Agreement Details" },
          { key: "tenant" as const, label: "Organisation" },
          { key: "integrations" as const, label: "Integrations" },
        ]).map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 transition-colors -mb-px ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "agreement" && <AgreementSettings />}
      {tab === "tenant" && <TenantSettings />}
      {tab === "integrations" && <IntegrationsSettings />}
    </div>
  );
}

function AgreementSettings() {
  const [data, setData] = useState<AgreementData>({});
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    api
      .getAgreementSettings()
      .then((res) => setData(res as AgreementData))
      .catch((e) => { console.error(e); setMessage("Failed to load agreement settings"); });
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setMessage("");
    try {
      await api.updateAgreementSettings(data);
      setMessage("Settings saved successfully");
    } catch {
      setMessage("Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  const updateField = (field: keyof AgreementData, value: string) => {
    setData((prev) => ({ ...prev, [field]: value }));
  };

  return (
    <div className="card px-5 py-4">
      <h2 className="text-base font-semibold text-gray-900 mb-4">
        Agreement Details (Section A)
      </h2>
      <p className="text-sm text-gray-500 mb-6">
        These details appear on all membership agreements.
      </p>

      {message && (
        <div
          className={`mb-4 rounded-lg px-4 py-3 text-sm ${
            message.includes("success")
              ? "bg-green-50 text-green-700"
              : "bg-red-50 text-red-700"
          }`}
        >
          {message}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Company Name" value={data.company_name} onChange={(v) => updateField("company_name", v)} />
        <Field label="Business Name" value={data.business_name} onChange={(v) => updateField("business_name", v)} />
        <Field label="ABN/ACN" value={data.abn_acn} onChange={(v) => updateField("abn_acn", v)} />
        <Field label="Trading Name" value={data.trading_name} onChange={(v) => updateField("trading_name", v)} />
        <Field label="Address" value={data.address} onChange={(v) => updateField("address", v)} className="sm:col-span-2" />
        <Field label="State" value={data.state} onChange={(v) => updateField("state", v)} />
        <Field label="Postcode" value={data.postcode} onChange={(v) => updateField("postcode", v)} />
        <Field label="Email" value={data.email} onChange={(v) => updateField("email", v)} type="email" />
        <Field label="Phone" value={data.phone} onChange={(v) => updateField("phone", v)} type="tel" />
      </div>

      <div className="mt-6 flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary">
          {saving ? "Saving..." : "Save Settings"}
        </button>
      </div>
    </div>
  );
}

function TenantSettings() {
  const [data, setData] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    api.getTenantSettings().then((res) => {
      const t = res as Record<string, string>;
      setData({
        slug: t.slug || "",
        timezone: t.timezone || "Australia/Brisbane",
        primary_colour: t.primary_colour || "#6366f1",
        secondary_colour: t.secondary_colour || "",
        logo_url: t.logo_url || "",
        default_thankyou_heading: t.default_thankyou_heading || "",
        default_thankyou_body: t.default_thankyou_body || "",
        default_thankyou_image_url: t.default_thankyou_image_url || "",
        default_thankyou_redirect_url: t.default_thankyou_redirect_url || "",
      });
    }).catch((e) => { console.error(e); setMessage("Failed to load organisation settings"); });
  }, []);

  const handleSave = async () => {
    setSaving(true); setMessage("");
    try {
      await api.updateTenantSettings(data);
      setMessage("Settings saved successfully");
    } catch { setMessage("Failed to save"); }
    finally { setSaving(false); }
  };

  const joinUrl = data.slug ? `${window.location.origin}/join/${data.slug}` : "";

  const timezones = [
    "Australia/Brisbane", "Australia/Sydney", "Australia/Melbourne", "Australia/Perth",
    "Australia/Adelaide", "Australia/Darwin", "Australia/Hobart",
    "Pacific/Auckland", "America/New_York", "America/Chicago", "America/Los_Angeles",
    "Europe/London", "Europe/Berlin", "Asia/Singapore", "Asia/Tokyo",
  ];

  return (
    <div className="space-y-5">
      {message && (
        <div className={`rounded-md px-3 py-2 text-[13px] ${message.includes("success") ? "bg-emerald-50 border border-emerald-200 text-emerald-700" : "bg-red-50 border border-red-200 text-red-600"}`}>{message}</div>
      )}

      {/* General */}
      <div className="card">
        <div className="card-header"><h3>General</h3></div>
        <div className="card-body space-y-4">
          <ImageUpload
            value={data.logo_url || ""}
            entityType="tenant"
            onChange={(url) => setData({ ...data, logo_url: url })}
            label="Logo"
            hint="Shown on public signup pages"
          />
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Join page URL slug</label>
            <p className="text-[11px] text-gray-400 mb-1.5">This controls your public join URL.</p>
            <input
              type="text"
              value={data.slug || ""}
              onChange={(e) => setData({ ...data, slug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "") })}
              className="input-field max-w-sm font-mono text-[13px]"
              placeholder="your-business-slug"
            />
            <div className="mt-2 flex items-center gap-2">
              <code className="rounded bg-gray-50 px-2 py-1 text-[11px] text-gray-600 border border-gray-200">
                {joinUrl || "https://your-domain/join/your-slug"}
              </code>
              <button
                type="button"
                disabled={!joinUrl}
                onClick={() => navigator.clipboard.writeText(joinUrl)}
                className="btn-secondary text-[11px] px-2 py-1 disabled:opacity-50"
              >
                Copy
              </button>
            </div>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Timezone</label>
            <select value={data.timezone || ""} onChange={(e) => setData({ ...data, timezone: e.target.value })} className="input-field max-w-xs">
              {timezones.map((tz) => <option key={tz} value={tz}>{tz.replace("_", " ")}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Brand colour</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Used on public signup pages and emails</p>
            <div className="flex items-center gap-3">
              <input type="color" value={data.primary_colour || "#6366f1"} onChange={(e) => setData({ ...data, primary_colour: e.target.value })} className="h-9 w-9 rounded border border-gray-300 cursor-pointer p-0.5" />
              <input type="text" value={data.primary_colour || ""} onChange={(e) => setData({ ...data, primary_colour: e.target.value })} className="input-field max-w-[120px] font-mono text-[13px]" placeholder="#6366f1" />
            </div>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Secondary / accent colour</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Used for accents on public signup pages</p>
            <div className="flex items-center gap-3">
              <input type="color" value={data.secondary_colour || "#4f46e5"} onChange={(e) => setData({ ...data, secondary_colour: e.target.value })} className="h-9 w-9 rounded border border-gray-300 cursor-pointer p-0.5" />
              <input type="text" value={data.secondary_colour || ""} onChange={(e) => setData({ ...data, secondary_colour: e.target.value })} className="input-field max-w-[120px] font-mono text-[13px]" placeholder="#4f46e5" />
              <button type="button" className="btn-secondary text-[12px]" onClick={() => setData({ ...data, secondary_colour: "" })}>
                Reset
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Thank you page defaults */}
      <div className="card">
        <div className="card-header"><h3>Default Thank You Page</h3></div>
        <div className="card-body space-y-4">
          <p className="text-[11px] text-gray-400 -mt-1">Shown after a client completes a membership signup. Can be overridden per membership type.</p>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Heading</label>
            <input value={data.default_thankyou_heading || ""} onChange={(e) => setData({ ...data, default_thankyou_heading: e.target.value })} className="input-field" placeholder="Thank You!" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Body text</label>
            <textarea
              value={data.default_thankyou_body || ""}
              onChange={(e) => setData({ ...data, default_thankyou_body: e.target.value })}
              className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
              rows={3}
              placeholder="Your membership has been successfully created."
            />
          </div>
          <ImageUpload
            value={data.default_thankyou_image_url || ""}
            entityType="thankyou"
            onChange={(url) => setData({ ...data, default_thankyou_image_url: url })}
            label="Image"
            hint="Shown on the thank you page after signup"
          />
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Redirect URL</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Optional — auto-redirect after 5 seconds</p>
            <input value={data.default_thankyou_redirect_url || ""} onChange={(e) => setData({ ...data, default_thankyou_redirect_url: e.target.value })} className="input-field" placeholder="https://yourwebsite.com" />
          </div>
        </div>
      </div>

      <div className="flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary">{saving ? "Saving..." : "Save Settings"}</button>
      </div>
    </div>
  );
}

function IntegrationsSettings() {
  const [integrations, setIntegrations] = useState<{ provider: string; environment: string; is_active: boolean; has_key: boolean; has_secret: boolean; key_hint: string }[]>([]);
  const [saving, setSaving] = useState<string | null>(null);
  const [disconnecting, setDisconnecting] = useState(false);
  const [message, setMessage] = useState("");
  const [tenantSlug, setTenantSlug] = useState("");

  const refreshIntegrations = () => {
    api.listIntegrations().then((data) => {
      setIntegrations((data as { integrations: typeof integrations }).integrations || []);
    }).catch((e) => { console.error(e); setMessage("Failed to load integrations"); });
  };

  useEffect(() => {
    refreshIntegrations();
    api.getTenantSettings().then((res) => {
      const t = res as { slug?: string };
      setTenantSlug(t.slug || "");
    }).catch((e) => { console.error(e); setMessage("Failed to load tenant settings"); });
  }, []);

  const providers = [
    { id: "gocardless", name: "GoCardless", desc: "Direct debit payments for your members. Add your API key from the GoCardless developer dashboard.", hasSecret: false,
      fields: { key: "API key", secret: "" } },
    { id: "stripe", name: "Stripe", desc: "Card payments and billing. Add your Stripe secret key and webhook signing secret per tenant.", hasSecret: true,
      fields: { key: "Secret key", secret: "Webhook signing secret" } },
  ];

  const handleSave = async (providerId: string, data: { api_key: string; api_secret: string; environment: string; is_active: boolean }) => {
    setSaving(providerId); setMessage("");
    try {
      await api.saveIntegration(providerId, data);
      setMessage(`${providerId} saved`);
      refreshIntegrations();
    } catch { setMessage("Failed to save"); }
    finally { setSaving(null); }
  };

  const handleDisconnectGoCardless = async () => {
    if (!window.confirm("Remove GoCardless? You can reconnect later.")) return;
    setDisconnecting(true); setMessage("");
    try {
      await api.deleteIntegration("gocardless");
      setMessage("GoCardless disconnected");
      refreshIntegrations();
    } catch { setMessage("Failed to disconnect"); }
    finally { setDisconnecting(false); }
  };

  const gocardlessExisting = integrations.find(i => i.provider === "gocardless");
  const gocardlessConnected = !!gocardlessExisting?.has_key;

  return (
    <div className="space-y-5">
      {message && (
        <div className={`rounded-md px-3 py-2 text-[13px] ${message.includes("Failed") ? "bg-red-50 border border-red-200 text-red-600" : "bg-emerald-50 border border-emerald-200 text-emerald-700"}`}>{message}</div>
      )}

      {/* GoCardless: API key only */}
      <div className="space-y-2">
        <IntegrationCard
          provider={providers[0]}
          existing={gocardlessExisting || null}
          saving={saving === "gocardless"}
          onSave={(data) => handleSave("gocardless", data)}
          tenantSlug={tenantSlug}
        />
        {gocardlessConnected && (
          <div className="flex justify-end">
            <button
              type="button"
              onClick={handleDisconnectGoCardless}
              disabled={disconnecting}
              className="text-[12px] text-red-600 hover:text-red-700 font-medium disabled:opacity-50"
            >
              {disconnecting ? "Disconnecting..." : "Disconnect GoCardless"}
            </button>
          </div>
        )}
      </div>

      {providers.filter(p => p.id !== "gocardless").map((provider) => {
        const existing = integrations.find(i => i.provider === provider.id);
        return (
          <IntegrationCard
            key={provider.id}
            provider={provider}
            existing={existing || null}
            saving={saving === provider.id}
            onSave={(data) => handleSave(provider.id, data)}
            tenantSlug={tenantSlug}
          />
        );
      })}
    </div>
  );
}

function IntegrationCard({ provider, existing, saving, onSave, collapsedByDefault, tenantSlug }: {
  provider: { id: string; name: string; desc: string; hasSecret: boolean; fields: { key: string; secret: string } };
  existing: { environment: string; is_active: boolean; has_key: boolean; has_secret?: boolean; key_hint: string } | null;
  saving: boolean;
  onSave: (data: { api_key: string; api_secret: string; environment: string; is_active: boolean }) => void;
  collapsedByDefault?: boolean;
  tenantSlug?: string;
}) {
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [environment, setEnvironment] = useState(existing?.environment || "sandbox");
  const [isActive, setIsActive] = useState(existing?.is_active ?? true);
  const [expanded, setExpanded] = useState(!existing?.has_key && !collapsedByDefault);
  const isStripe = provider.id === "stripe";
  const webhookUrl = tenantSlug ? `${window.location.origin}/api/webhooks/stripe/${tenantSlug}` : "";
  const stripeReady = !!(existing?.has_key && existing?.has_secret && existing?.is_active);

  return (
    <div className="card overflow-hidden">
      <div className="card-header">
        <div className="flex items-center gap-2">
          <h3>{provider.name}</h3>
          {existing?.has_key ? (
            <span className="badge badge-green">Connected</span>
          ) : (
            <span className="badge badge-gray">Not configured</span>
          )}
        </div>
        <button onClick={() => setExpanded(!expanded)} className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium">
          {expanded ? "Collapse" : existing?.has_key ? "Update" : "Configure"}
        </button>
      </div>
      <div className="card-body">
        <p className="text-[12px] text-gray-500 mb-3">{provider.desc}</p>
        {isStripe && (
          <div className="mb-3 rounded-md border border-indigo-200 bg-indigo-50 p-3">
            <p className="text-[12px] font-semibold text-indigo-900 mb-1">Stripe setup checklist</p>
            <div className="space-y-1 text-[12px] text-indigo-900/90">
              <p>1. Paste Stripe secret key + webhook signing secret and click Save.</p>
              <p>2. In Stripe dashboard, add webhook endpoint:</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 truncate rounded bg-white/80 px-2 py-1 font-mono text-[11px] text-indigo-900">
                  {webhookUrl || "Loading tenant webhook URL..."}
                </code>
                <button
                  type="button"
                  disabled={!webhookUrl}
                  onClick={() => { navigator.clipboard.writeText(webhookUrl); }}
                  className="btn-secondary text-[11px] px-2 py-1 disabled:opacity-50"
                >
                  Copy
                </button>
              </div>
              <p>3. Enable these webhook events:</p>
              <ul className="list-disc pl-5 space-y-0.5 text-[11px]">
                <li><code>checkout.session.completed</code></li>
                <li><code>checkout.session.expired</code></li>
                <li><code>invoice.payment_succeeded</code></li>
                <li><code>invoice.payment_failed</code></li>
                <li><code>customer.subscription.deleted</code></li>
                <li><code>charge.dispute.created</code>, <code>charge.dispute.updated</code>, <code>charge.dispute.closed</code></li>
                <li><code>charge.refunded</code> and <code>refund.updated</code></li>
              </ul>
            </div>
            <div className="mt-2 flex items-center gap-2 text-[11px]">
              <span className={`rounded px-1.5 py-0.5 ${existing?.has_key ? "bg-emerald-100 text-emerald-700" : "bg-gray-100 text-gray-500"}`}>Key {existing?.has_key ? "saved" : "missing"}</span>
              <span className={`rounded px-1.5 py-0.5 ${existing?.has_secret ? "bg-emerald-100 text-emerald-700" : "bg-gray-100 text-gray-500"}`}>Webhook secret {existing?.has_secret ? "saved" : "missing"}</span>
              <span className={`rounded px-1.5 py-0.5 ${existing?.is_active ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>Integration {existing?.is_active ? "active" : "inactive"}</span>
              <span className={`rounded px-1.5 py-0.5 ${stripeReady ? "bg-emerald-100 text-emerald-700" : "bg-red-100 text-red-700"}`}>{stripeReady ? "Ready" : "Not ready"}</span>
            </div>
          </div>
        )}
        {existing?.has_key && !expanded && (
          <p className="text-[12px] text-gray-400">API key ending in <span className="font-mono">{existing.key_hint}</span> · {existing.environment}</p>
        )}
        {expanded && (
          <div className="space-y-3 mt-2">
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">{provider.fields.key}</label>
              <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)}
                className="input-field font-mono text-[13px]" placeholder={existing?.has_key ? "Leave blank to keep existing" : "Enter key..."} />
            </div>
            {provider.hasSecret && (
              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">{provider.fields.secret}</label>
                <input type="password" value={apiSecret} onChange={(e) => setApiSecret(e.target.value)}
                  className="input-field font-mono text-[13px]" placeholder={existing?.has_key ? "Leave blank to keep existing" : "Enter secret..."} />
              </div>
            )}
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">Environment</label>
                <select value={environment} onChange={(e) => setEnvironment(e.target.value)} className="input-field">
                  <option value="sandbox">Sandbox</option>
                  <option value="live">Live</option>
                </select>
              </div>
              <div className="flex items-end pb-1">
                <label className="flex items-center gap-2 text-[13px] text-gray-700">
                  <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
                  Active
                </label>
              </div>
            </div>
            <div className="flex justify-end pt-1">
              <button onClick={() => onSave({ api_key: apiKey, api_secret: apiSecret, environment, is_active: isActive })}
                disabled={saving} className="btn-primary text-[12px]">
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = "text",
  className = "",
}: {
  label: string;
  value?: string;
  onChange: (v: string) => void;
  type?: string;
  className?: string;
}) {
  return (
    <div className={className}>
      <label className="block text-sm font-medium text-gray-700 mb-1">
        {label}
      </label>
      <input
        type={type}
        value={value || ""}
        onChange={(e) => onChange(e.target.value)}
        className="input-field"
      />
    </div>
  );
}
