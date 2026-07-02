import { useState, useEffect } from "react";
import api, { ApiError } from "../../api/client";
import ImageUpload from "../common/ImageUpload";
import { extractBrandColorsFromImage } from "../../utils/brandColors";

interface AgreementData {
  company_name?: string;
  business_name?: string;
  abn?: string;
  acn?: string;
  trading_name?: string;
  address?: string;
  state?: string;
  postcode?: string;
  email?: string;
  phone?: string;
}

const AVAILABLE_CURRENCIES = ["AUD", "USD", "EUR", "GBP", "NZD", "CAD", "SGD"];
const AVAILABLE_PAYMENT_FREQUENCIES = [
  { key: "upfront", label: "Upfront" },
  { key: "weekly", label: "Weekly" },
  { key: "fortnightly", label: "Fortnightly" },
  { key: "monthly", label: "Monthly" },
  { key: "quarterly", label: "Quarterly" },
  { key: "custom_schedule", label: "Custom instalments" },
];

export default function SettingsPage() {
  const [tab, setTab] = useState<"agreement" | "tenant" | "integrations" | "account">(() => {
    const params = new URLSearchParams(window.location.search);
    const t = params.get("tab");
    if (t === "integrations" || t === "tenant" || t === "account") return t;
    return "agreement";
  });

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
          { key: "tenant" as const, label: "Organisation & Signup" },
          { key: "integrations" as const, label: "Integrations" },
          { key: "account" as const, label: "My Account" },
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
      {tab === "account" && <AccountSettings />}
    </div>
  );
}

function AgreementSettings() {
  const [data, setData] = useState<AgreementData>({});
  const [saving, setSaving] = useState(false);
  const [lookingUp, setLookingUp] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    api
      .getAgreementSettings()
      .then((res) => setData(res as AgreementData))
      .catch((e) => { console.error(e); setMessage("Unable to load agreement settings."); });
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setMessage("");
    try {
      await api.updateAgreementSettings(data);
      setMessage("Settings saved successfully");
    } catch {
      setMessage("Unable to save settings.");
    } finally {
      setSaving(false);
    }
  };

  const updateField = (field: keyof AgreementData, value: string) => {
    setData((prev) => ({ ...prev, [field]: value }));
  };

  const handleLookupBusiness = async () => {
    const company = (data.company_name || "").trim();
    if (!company) {
      setMessage("Enter company name first");
      return;
    }
    setLookingUp(true);
    setMessage("");
    try {
      const res = await api.lookupBusinessByName(company) as {
        results?: Array<{ name: string; abn: string; acn?: string; state?: string; postcode?: string }>;
      };
      const first = res.results?.[0];
      if (!first) {
        setMessage("No ABN matches found");
        return;
      }
      setData((prev) => ({
        ...prev,
        company_name: prev.company_name || first.name,
        abn: first.abn || prev.abn,
        acn: first.acn || prev.acn,
        state: prev.state || first.state || "",
        postcode: prev.postcode || first.postcode || "",
      }));
      setMessage(`Found ${first.name}`);
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Lookup failed");
    } finally {
      setLookingUp(false);
    }
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
        <div>
          <Field label="Company Name" value={data.company_name} onChange={(v) => updateField("company_name", v)} />
          <div className="mt-1">
            <button type="button" onClick={handleLookupBusiness} disabled={lookingUp} className="btn-secondary text-[11px] px-2 py-1">
              {lookingUp ? "Looking up..." : "Lookup ABN/ACN"}
            </button>
          </div>
        </div>
        <Field label="Business Name" value={data.business_name} onChange={(v) => updateField("business_name", v)} />
        <Field label="ABN" value={data.abn} onChange={(v) => updateField("abn", v)} />
        <Field label="ACN" value={data.acn} onChange={(v) => updateField("acn", v)} />
        <Field label="Trading Name" value={data.trading_name} onChange={(v) => updateField("trading_name", v)} />
        <Field label="Address" value={data.address} onChange={(v) => updateField("address", v)} className="sm:col-span-2" />
        <Field label="State" value={data.state} onChange={(v) => updateField("state", v)} />
        <Field label="Postcode" value={data.postcode} onChange={(v) => updateField("postcode", v)} />
        <Field label="Email" value={data.email} onChange={(v) => updateField("email", v)} type="email" />
        <Field label="Phone" value={data.phone} onChange={(v) => updateField("phone", v)} type="tel" />
      </div>

      <div className="mt-6 flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary">
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
}

function TenantSettings() {
  const [data, setData] = useState<{
    slug: string;
    timezone: string;
    sales_tax_name: string;
    sales_tax_percent: number;
    signup_price_display_mode: string;
    primary_colour: string;
    secondary_colour: string;
    logo_url: string;
    default_thankyou_heading: string;
    default_thankyou_body: string;
    default_thankyou_image_url: string;
    default_thankyou_redirect_url: string;
    enabled_currencies: string[];
    enabled_payment_frequencies: string[];
    ask_emergency_contact: boolean;
    ask_company_information: boolean;
    ask_birth_date: boolean;
    signup_webhook_url: string;
    signup_webhook_enabled: boolean;
    signup_webhook_has_secret: boolean;
  }>({
    slug: "",
    timezone: "Australia/Brisbane",
    sales_tax_name: "GST",
    sales_tax_percent: 10,
    signup_price_display_mode: "weekly",
    primary_colour: "#6366f1",
    secondary_colour: "",
    logo_url: "",
    default_thankyou_heading: "",
    default_thankyou_body: "",
    default_thankyou_image_url: "",
    default_thankyou_redirect_url: "",
    enabled_currencies: ["AUD"],
    enabled_payment_frequencies: ["upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"],
    ask_emergency_contact: true,
    ask_company_information: false,
    ask_birth_date: true,
    signup_webhook_url: "",
    signup_webhook_enabled: false,
    signup_webhook_has_secret: false,
  });
  const [webhookSecret, setWebhookSecret] = useState("");
  const [testingWebhook, setTestingWebhook] = useState(false);
  const [webhookTestMessage, setWebhookTestMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [detectingColors, setDetectingColors] = useState(false);
  const [colorMessage, setColorMessage] = useState("");
  const [signedBrowsePath, setSignedBrowsePath] = useState("");

  useEffect(() => {
    Promise.all([api.getTenantSettings(), api.getSignupLinks().catch(() => null)]).then(([res, links]) => {
      const t = res as Record<string, unknown>;
      setData({
        slug: String(t.slug || ""),
        timezone: String(t.timezone || "Australia/Brisbane"),
        sales_tax_name: String(t.sales_tax_name || "GST"),
        sales_tax_percent: Number(t.sales_tax_percent ?? 10),
        signup_price_display_mode: String(t.signup_price_display_mode || "weekly"),
        primary_colour: String(t.primary_colour || "#6366f1"),
        secondary_colour: String(t.secondary_colour || ""),
        logo_url: String(t.logo_url || ""),
        default_thankyou_heading: String(t.default_thankyou_heading || ""),
        default_thankyou_body: String(t.default_thankyou_body || ""),
        default_thankyou_image_url: String(t.default_thankyou_image_url || ""),
        default_thankyou_redirect_url: String(t.default_thankyou_redirect_url || ""),
        enabled_currencies: Array.isArray(t.enabled_currencies)
          ? Array.from(new Set((t.enabled_currencies as unknown[]).map((c) => String(c).toUpperCase()))).filter(Boolean)
          : ["AUD"],
        enabled_payment_frequencies: Array.isArray(t.enabled_payment_frequencies)
          ? Array.from(new Set((t.enabled_payment_frequencies as unknown[]).map((f) => String(f).toLowerCase()))).filter(Boolean)
          : ["upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"],
        ask_emergency_contact: t.ask_emergency_contact !== false,
        ask_company_information: t.ask_company_information === true,
        ask_birth_date: t.ask_birth_date !== false,
        signup_webhook_url: String(t.signup_webhook_url || ""),
        signup_webhook_enabled: t.signup_webhook_enabled === true,
        signup_webhook_has_secret: t.signup_webhook_has_secret === true,
      });
      const linkData = links as { browse_path?: string } | null;
      if (linkData?.browse_path) {
        setSignedBrowsePath(linkData.browse_path);
      }
    }).catch((e) => { console.error(e); setMessage("Unable to load organisation settings."); });
  }, []);

  const handleSave = async () => {
    setSaving(true); setMessage(""); setWebhookSecret("");
    try {
      const res = await api.updateTenantSettings(data) as Record<string, unknown>;
      if (typeof res.signup_webhook_signing_secret === "string" && res.signup_webhook_signing_secret) {
        setWebhookSecret(res.signup_webhook_signing_secret);
      }
      if (res.signup_webhook_has_secret === true) {
        setData((prev) => ({ ...prev, signup_webhook_has_secret: true }));
      }
      setMessage("Settings saved successfully");
    } catch { setMessage("Unable to save changes."); }
    finally { setSaving(false); }
  };

  const handleRegenerateWebhookSecret = async () => {
    setSaving(true); setMessage(""); setWebhookSecret("");
    try {
      const res = await api.updateTenantSettings({
        ...data,
        regenerate_signup_webhook_secret: true,
      }) as Record<string, unknown>;
      if (typeof res.signup_webhook_signing_secret === "string") {
        setWebhookSecret(res.signup_webhook_signing_secret);
      }
      setData((prev) => ({ ...prev, signup_webhook_has_secret: true }));
      setMessage("New signing secret generated");
    } catch {
      setMessage("Unable to regenerate signing secret.");
    } finally {
      setSaving(false);
    }
  };

  const handleTestWebhook = async () => {
    setTestingWebhook(true);
    setWebhookTestMessage("");
    try {
      const res = await api.testSignupWebhook();
      if (res.success) {
        setWebhookTestMessage(`Test delivered (${res.http_status})`);
      } else {
        setWebhookTestMessage(`Destination returned HTTP ${res.http_status}`);
      }
    } catch {
      setWebhookTestMessage("Test failed — check the URL and try again.");
    } finally {
      setTestingWebhook(false);
    }
  };

  const joinUrl = signedBrowsePath
    ? `${window.location.origin}${signedBrowsePath}`
    : (data.slug ? `${window.location.origin}/join/${data.slug}` : "");

  const timezones = [
    "Australia/Brisbane", "Australia/Sydney", "Australia/Melbourne", "Australia/Perth",
    "Australia/Adelaide", "Australia/Darwin", "Australia/Hobart",
    "Pacific/Auckland", "America/New_York", "America/Chicago", "America/Los_Angeles",
    "Europe/London", "Europe/Berlin", "Asia/Singapore", "Asia/Tokyo",
  ];

  const toggleCurrency = (code: string, checked: boolean) => {
    setData((prev) => {
      const next = new Set(prev.enabled_currencies || []);
      if (checked) {
        next.add(code);
      } else if (code !== "AUD") {
        next.delete(code);
      }
      next.add("AUD");
      return { ...prev, enabled_currencies: Array.from(next) };
    });
  };
  const togglePaymentFrequency = (frequency: string, checked: boolean) => {
    setData((prev) => {
      const next = new Set(prev.enabled_payment_frequencies || []);
      if (checked) {
        next.add(frequency);
      } else {
        next.delete(frequency);
      }
      if (next.size === 0) next.add("weekly");
      return { ...prev, enabled_payment_frequencies: Array.from(next) };
    });
  };
  const detectLogoColors = async (logoUrl: string) => {
    if (!logoUrl) return;
    setDetectingColors(true);
    setColorMessage("");
    try {
      const colors = await extractBrandColorsFromImage(logoUrl);
      if (!colors) {
        setColorMessage("Couldn't detect colors from logo");
        return;
      }
      setData((prev) => ({
        ...prev,
        primary_colour: colors.primary,
        secondary_colour: colors.secondary,
      }));
      setColorMessage(`Colors detected: ${colors.primary} and ${colors.secondary}`);
    } catch {
      setColorMessage("Couldn't detect colors from logo");
    } finally {
      setDetectingColors(false);
    }
  };

  return (
    <div className="space-y-5">
      {message && (
        <div className={`rounded-md px-3 py-2 text-[13px] ${message.includes("success") ? "bg-emerald-50 border border-emerald-200 text-emerald-700" : "bg-red-50 border border-red-200 text-red-600"}`}>{message}</div>
      )}

      {/* Sales tax */}
      <div className="card">
        <div className="card-header"><h3>Sales Tax</h3></div>
        <div className="card-body space-y-3">
          <p className="text-[11px] text-gray-500">
            Configure your default sales tax used for memberships and one-off service charging.
          </p>
          <div className="flex items-center gap-2">
            <span className={`badge ${Number(data.sales_tax_percent || 0) > 0 ? "badge-green" : "badge-gray"}`}>
              {Number(data.sales_tax_percent || 0) > 0 ? "Active" : "Inactive"}
            </span>
            <span className="text-[12px] text-gray-500">
              Tax is active when rate is greater than 0%.
            </span>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-w-lg">
            <div>
              <label className="block text-[12px] text-gray-500 mb-1">Tax label</label>
              <input
                type="text"
                value={data.sales_tax_name || "GST"}
                onChange={(e) => setData({ ...data, sales_tax_name: e.target.value.toUpperCase() })}
                className="input-field"
                maxLength={30}
                placeholder="GST"
              />
            </div>
            <div>
              <label className="block text-[12px] text-gray-500 mb-1">Rate (%)</label>
              <input
                type="number"
                min="0"
                max="100"
                step="0.01"
                value={Number.isFinite(data.sales_tax_percent) ? data.sales_tax_percent : 0}
                onChange={(e) => setData({ ...data, sales_tax_percent: Number(e.target.value) })}
                className="input-field"
              />
            </div>
          </div>
        </div>
      </div>

      {/* General */}
      <div className="card">
        <div className="card-header"><h3>General</h3></div>
        <div className="card-body space-y-4">
          <ImageUpload
            value={data.logo_url || ""}
            entityType="tenant"
            onChange={(url) => {
              setData((prev) => ({ ...prev, logo_url: url }));
              void detectLogoColors(url);
            }}
            label="Logo"
            hint="Shown on public signup pages"
          />
          {colorMessage && (
            <p className={`text-[12px] ${colorMessage.startsWith("Colors detected") ? "text-emerald-600" : "text-amber-600"}`}>
              {colorMessage}
            </p>
          )}
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
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Signup price display</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Controls how pricing is shown on the public join page.</p>
            <select
              value={data.signup_price_display_mode || "weekly"}
              onChange={(e) => setData({ ...data, signup_price_display_mode: e.target.value })}
              className="input-field max-w-xs"
            >
              <option value="total">Total value</option>
              <option value="weekly">Weekly value</option>
              <option value="monthly">Monthly value</option>
            </select>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Enabled currencies</label>
            <p className="text-[11px] text-gray-400 mb-1.5">These currencies are available when creating membership types. AUD is always enabled.</p>
            <div className="flex flex-wrap gap-2">
              {AVAILABLE_CURRENCIES.map((code) => (
                <label key={code} className="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-gray-50 px-2.5 py-1.5 text-[12px] text-gray-700">
                  <input
                    type="checkbox"
                    checked={(data.enabled_currencies || []).includes(code)}
                    disabled={code === "AUD"}
                    onChange={(e) => toggleCurrency(code, e.target.checked)}
                    className="h-3.5 w-3.5 rounded border-gray-300 text-indigo-600"
                  />
                  {code}
                </label>
              ))}
            </div>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Client payment frequency options</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Only these options are shown to clients during signup.</p>
            <div className="flex flex-wrap gap-2">
              {AVAILABLE_PAYMENT_FREQUENCIES.map((item) => (
                <label key={item.key} className="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-gray-50 px-2.5 py-1.5 text-[12px] text-gray-700">
                  <input
                    type="checkbox"
                    checked={(data.enabled_payment_frequencies || []).includes(item.key)}
                    onChange={(e) => togglePaymentFrequency(item.key, e.target.checked)}
                    className="h-3.5 w-3.5 rounded border-gray-300 text-indigo-600"
                  />
                  {item.label}
                </label>
              ))}
            </div>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Client signup fields</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Control which sections are requested on signup and client create/edit.</p>
            <div className="space-y-2">
              <label className="flex items-start justify-between gap-4 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 cursor-pointer">
                <span>
                  <span className="block text-[13px] font-medium text-gray-800">Emergency contact</span>
                  <span className="block text-[11px] text-gray-500 mt-0.5">Show and require emergency contact fields.</span>
                </span>
                <input
                  type="checkbox"
                  checked={!!data.ask_emergency_contact}
                  onChange={(e) => setData({ ...data, ask_emergency_contact: e.target.checked })}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-indigo-600"
                />
              </label>
              <label className="flex items-start justify-between gap-4 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 cursor-pointer">
                <span>
                  <span className="block text-[13px] font-medium text-gray-800">Company information</span>
                  <span className="block text-[11px] text-gray-500 mt-0.5">Show and require company fields for B2B signups.</span>
                </span>
                <input
                  type="checkbox"
                  checked={!!data.ask_company_information}
                  onChange={(e) => setData({ ...data, ask_company_information: e.target.checked })}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-indigo-600"
                />
              </label>
              <label className="flex items-start justify-between gap-4 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 cursor-pointer">
                <span>
                  <span className="block text-[13px] font-medium text-gray-800">Date of birth</span>
                  <span className="block text-[11px] text-gray-500 mt-0.5">Show date of birth on signup and client forms.</span>
                </span>
                <input
                  type="checkbox"
                  checked={!!data.ask_birth_date}
                  onChange={(e) => setData({ ...data, ask_birth_date: e.target.checked })}
                  className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-indigo-600"
                />
              </label>
            </div>
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
          <div className="flex">
            <button
              type="button"
              onClick={() => void detectLogoColors(data.logo_url || "")}
              disabled={!data.logo_url || detectingColors}
              className="btn-secondary text-[12px]"
            >
              {detectingColors ? "Detecting colours..." : "Auto-detect colours from logo"}
            </button>
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
            <input value={data.default_thankyou_heading || ""} onChange={(e) => setData({ ...data, default_thankyou_heading: e.target.value })} className="input-field" placeholder="Thank you" />
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

      <div className="card">
        <div className="card-header"><h3>Signup webhook</h3></div>
        <div className="card-body space-y-4">
          <p className="text-[11px] text-gray-400 -mt-1">
            Send a flat JSON payload to Zapier, Make, or your own endpoint when a client completes signup.
            Field names mirror Ignition-style proposal data (contact, company, dates, agreement PDF URL, etc.).
          </p>
          <label className="flex items-start justify-between gap-4 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 cursor-pointer">
            <span>
              <span className="block text-[13px] font-medium text-gray-800">Enable signup webhook</span>
              <span className="block text-[11px] text-gray-500 mt-0.5">Fires once when signup is fully complete (including after Stripe checkout).</span>
            </span>
            <input
              type="checkbox"
              checked={!!data.signup_webhook_enabled}
              onChange={(e) => setData({ ...data, signup_webhook_enabled: e.target.checked })}
              className="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-indigo-600"
            />
          </label>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Webhook URL</label>
            <input
              type="url"
              value={data.signup_webhook_url || ""}
              onChange={(e) => setData({ ...data, signup_webhook_url: e.target.value })}
              className="input-field font-mono text-[12px]"
              placeholder="https://hooks.zapier.com/hooks/catch/..."
            />
          </div>
          {data.signup_webhook_has_secret && !webhookSecret && (
            <p className="text-[11px] text-gray-500">Signing secret configured. Regenerate if you need a new one.</p>
          )}
          {webhookSecret && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5">
              <p className="text-[12px] font-medium text-amber-900">Signing secret (copy now — shown once)</p>
              <code className="mt-1 block break-all text-[11px] text-amber-950">{webhookSecret}</code>
              <p className="mt-1 text-[11px] text-amber-800">Verify requests with HMAC-SHA256 of <code>{`<timestamp>.<body>`}</code> in header <code>X-Webhook-Signature</code>.</p>
            </div>
          )}
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className="btn-secondary text-[12px]"
              disabled={!data.signup_webhook_url || saving}
              onClick={() => void handleTestWebhook()}
            >
              {testingWebhook ? "Sending test..." : "Send test payload"}
            </button>
            <button
              type="button"
              className="btn-secondary text-[12px]"
              disabled={!data.signup_webhook_url || !data.signup_webhook_has_secret || saving}
              onClick={() => void handleRegenerateWebhookSecret()}
            >
              Regenerate signing secret
            </button>
          </div>
          {webhookTestMessage && (
            <p className="text-[12px] text-gray-600">{webhookTestMessage}</p>
          )}
        </div>
      </div>

      <div className="flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary">{saving ? "Saving..." : "Save"}</button>
      </div>
    </div>
  );
}

function IntegrationsSettings() {
  const [integrations, setIntegrations] = useState<{ provider: string; environment: string; is_active: boolean; has_key: boolean; has_secret: boolean; key_hint: string }[]>([]);
  const [xero, setXero] = useState<{
    connected: boolean;
    is_active?: boolean;
    feature_enabled?: boolean;
    oauth_configured?: boolean;
    needs_reauth?: boolean;
    xero_tenant_id?: string;
    xero_tenant_name?: string;
    default_revenue_account_code?: string;
    default_payment_account_code?: string;
  }>({ connected: false });
  const [xeroRevenueCode, setXeroRevenueCode] = useState("200");
  const [xeroPaymentCode, setXeroPaymentCode] = useState("090");
  const [xeroSelectionToken, setXeroSelectionToken] = useState("");
  const [xeroSelections, setXeroSelections] = useState<Array<{ tenant_id: string; tenant_name: string }>>([]);
  const [saving, setSaving] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [messageType, setMessageType] = useState<"success" | "error">("success");
  const [tenantSlug, setTenantSlug] = useState("");

  const showMessage = (msg: string, type: "success" | "error" = "success") => {
    setMessage(msg);
    setMessageType(type);
  };

  const refreshIntegrations = () => {
    api.listIntegrations().then((data) => {
      setIntegrations((data as { integrations: typeof integrations }).integrations || []);
    }).catch((e) => { console.error(e); showMessage("Unable to load integrations.", "error"); });
    api.getXeroStatus().then((status) => {
      setXero(status);
      setXeroRevenueCode(status.default_revenue_account_code || "200");
      setXeroPaymentCode(status.default_payment_account_code || "090");
    }).catch(() => {
      setXero({ connected: false });
    });
  };

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const x = params.get("xero");
    const selectionToken = params.get("selection_token");
    if (x === "connected") showMessage("Xero connected successfully.");
    if (x && x.startsWith("error")) showMessage("Unable to connect Xero. Please try again.", "error");
    if (x === "select_org" && selectionToken) {
      setXeroSelectionToken(selectionToken);
      api.getXeroPendingSelection(selectionToken).then((res) => {
        setXeroSelections(res.items || []);
      }).catch(() => {
        showMessage("Xero organisation selection expired. Please reconnect.", "error");
      });
    }
    refreshIntegrations();
    api.getTenantSettings().then((res) => {
      const t = res as { slug?: string };
      setTenantSlug(t.slug || "");
    }).catch((e) => { console.error(e); showMessage("Unable to load tenant settings.", "error"); });
  }, []);

  const providers = [
    { id: "stripe", name: "Stripe", desc: "Card payments and billing. Add your Stripe secret key and webhook signing secret per tenant.", hasSecret: true,
      fields: { key: "Secret key", secret: "Webhook signing secret" } },
  ];

  const handleSave = async (providerId: string, data: { api_key: string; api_secret: string; environment: string; is_active: boolean }) => {
    setSaving(providerId); setMessage("");
    try {
      await api.saveIntegration(providerId, data);
      showMessage(`${providerId} saved`);
      refreshIntegrations();
    } catch { showMessage("Unable to save changes.", "error"); }
    finally { setSaving(null); }
  };

  const connectXero = async () => {
    setSaving("xero-connect");
    setMessage("");
    if (xero.oauth_configured === false) {
      showMessage("Xero is not enabled by your platform admin yet. Please contact support.", "error");
      setSaving(null);
      return;
    }
    try {
      const { redirect_url } = await api.getXeroConnectUrl();
      window.location.href = redirect_url;
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : "Unable to start Xero connection.";
      showMessage(msg, "error");
      setSaving(null);
    }
  };

  const saveXeroConfig = async () => {
    setSaving("xero-config");
    setMessage("");
    try {
      await api.updateXeroConfig({
        default_revenue_account_code: xeroRevenueCode.trim(),
        default_payment_account_code: xeroPaymentCode.trim(),
      });
      showMessage("Xero config updated.");
      refreshIntegrations();
    } catch (e) {
      showMessage(e instanceof Error ? e.message : "Unable to save Xero config.", "error");
    } finally {
      setSaving(null);
    }
  };

  const disconnectXero = async () => {
    setSaving("xero-disconnect");
    setMessage("");
    try {
      await api.disconnectXero();
      showMessage("Xero disconnected.");
      refreshIntegrations();
    } catch {
      showMessage("Unable to disconnect Xero.", "error");
    } finally {
      setSaving(null);
    }
  };

  const selectXeroOrg = async (orgId: string) => {
    if (!xeroSelectionToken) return;
    setSaving("xero-select");
    setMessage("");
    try {
      await api.selectXeroOrg({ selection_token: xeroSelectionToken, xero_tenant_id: orgId });
      showMessage("Xero connected successfully.");
      setXeroSelectionToken("");
      setXeroSelections([]);
      refreshIntegrations();
    } catch {
      showMessage("Unable to complete Xero org selection.", "error");
    } finally {
      setSaving(null);
    }
  };

  return (
    <div className="space-y-5">
      {message && (
        <div className={`rounded-md px-3 py-2 text-[13px] ${messageType === "error" ? "bg-red-50 border border-red-200 text-red-600" : "bg-emerald-50 border border-emerald-200 text-emerald-700"}`}>{message}</div>
      )}

      {providers.map((provider) => {
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

      <div className="card overflow-hidden">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <h3>Xero</h3>
            {xero.connected ? (
              <span className="badge badge-green">Connected</span>
            ) : (
              <span className="badge badge-gray">Not connected</span>
            )}
            {xero.needs_reauth ? <span className="badge badge-red">Reconnect required</span> : null}
          </div>
        </div>
        <div className="card-body space-y-3">
          <p className="text-[12px] text-gray-500">Connect Xero to create invoices when charges are created and mark them paid when settlements succeed.</p>
          {xero.oauth_configured === false ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
              Xero is not currently enabled for this workspace. Ask your platform admin to enable Xero integration.
            </div>
          ) : null}
          {xero.connected ? (
            <div className="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-[12px] text-gray-700">
              Connected org: <span className="font-medium">{xero.xero_tenant_name || xero.xero_tenant_id}</span>
            </div>
          ) : null}
          {xeroSelectionToken && xeroSelections.length > 0 ? (
            <div className="rounded-md border border-indigo-200 bg-indigo-50 p-3">
              <p className="text-[12px] font-medium text-indigo-800 mb-2">Select a Xero organisation</p>
              <div className="space-y-2">
                {xeroSelections.map((org) => (
                  <button
                    key={org.tenant_id}
                    type="button"
                    onClick={() => selectXeroOrg(org.tenant_id)}
                    className="w-full text-left rounded border border-indigo-200 bg-white px-3 py-2 text-[12px] hover:bg-indigo-50"
                    disabled={saving === "xero-select"}
                  >
                    {org.tenant_name || org.tenant_id}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
          {xero.connected ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">Revenue account code</label>
                <input className="input-field" value={xeroRevenueCode} onChange={(e) => setXeroRevenueCode(e.target.value)} />
              </div>
              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">Payment account code</label>
                <input className="input-field" value={xeroPaymentCode} onChange={(e) => setXeroPaymentCode(e.target.value)} />
              </div>
            </div>
          ) : null}
          <div className="flex gap-2 justify-end">
            {xero.connected ? (
              <>
                <button className="btn-secondary" onClick={saveXeroConfig} disabled={saving === "xero-config"}>
                  {saving === "xero-config" ? "Saving..." : "Save config"}
                </button>
                <button className="btn-secondary" onClick={disconnectXero} disabled={saving === "xero-disconnect"}>
                  {saving === "xero-disconnect" ? "Disconnecting..." : "Disconnect"}
                </button>
              </>
            ) : (
              <button
                className="btn-primary"
                onClick={connectXero}
                disabled={saving === "xero-connect" || xero.oauth_configured === false}
              >
                {saving === "xero-connect" ? "Connecting..." : "Connect Xero"}
              </button>
            )}
          </div>
        </div>
      </div>
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
  const [isActive, setIsActive] = useState(existing?.is_active ?? false);
  const [expanded, setExpanded] = useState(!existing?.has_key && !collapsedByDefault);
  const isStripe = provider.id === "stripe";
  const webhookUrl = tenantSlug ? `${window.location.origin}/api/webhooks/stripe/${tenantSlug}` : "";
  const stripeReady = !!(existing?.has_key && existing?.has_secret && existing?.is_active);
  const maskedPlaceholder = "••••••••••••";

  useEffect(() => {
    setEnvironment(existing?.environment || "sandbox");
    setIsActive(existing?.is_active ?? false);
  }, [existing?.environment, existing?.is_active]);

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
              <span className={`badge ${existing?.has_key ? "badge-green" : "badge-gray"}`}>Key {existing?.has_key ? "saved" : "missing"}</span>
              <span className={`badge ${existing?.has_secret ? "badge-green" : "badge-gray"}`}>Webhook secret {existing?.has_secret ? "saved" : "missing"}</span>
              <span className={`badge ${existing?.is_active ? "badge-green" : "badge-yellow"}`}>Integration {existing?.is_active ? "active" : "inactive"}</span>
              <span className={`badge ${stripeReady ? "badge-green" : "badge-red"}`}>{stripeReady ? "Ready" : "Not ready"}</span>
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
                className="input-field font-mono text-[13px]" placeholder={existing?.has_key ? maskedPlaceholder : "Enter key..."} />
              {existing?.has_key && <p className="mt-1 text-[11px] text-gray-500">Saved. Leave blank to keep existing.</p>}
            </div>
            {provider.hasSecret && (
              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">{provider.fields.secret}</label>
                <input type="password" value={apiSecret} onChange={(e) => setApiSecret(e.target.value)}
                  className="input-field font-mono text-[13px]" placeholder={existing?.has_secret ? maskedPlaceholder : "Enter secret..."} />
                {existing?.has_secret && <p className="mt-1 text-[11px] text-gray-500">Saved. Leave blank to keep existing.</p>}
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
                disabled={saving} className="btn-primary">
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function AccountSettings() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [messageType, setMessageType] = useState<"success" | "error">("success");

  const handleSubmit = async () => {
    setMessage("");
    if (!currentPassword || !newPassword) {
      setMessage("Current and new password are required.");
      setMessageType("error");
      return;
    }
    if (newPassword !== confirmPassword) {
      setMessage("New passwords do not match.");
      setMessageType("error");
      return;
    }
    if (newPassword.length < 8) {
      setMessage("New password must be at least 8 characters.");
      setMessageType("error");
      return;
    }
    setSaving(true);
    try {
      await api.changePassword(currentPassword, newPassword);
      setMessage("Password updated successfully.");
      setMessageType("success");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (e: unknown) {
      const err = e as ApiError;
      setMessage(err.message || "Failed to update password.");
      setMessageType("error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card">
      <div className="card-header">
        <h2>Change Password</h2>
      </div>
      <div className="card-body">
        {message && (
          <div className={`mb-4 rounded-md px-3 py-2 text-[13px] ${messageType === "success" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-700"}`}>
            {message}
          </div>
        )}
        <div className="space-y-3 max-w-md">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Current Password</label>
            <input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)}
              className="input-field" placeholder="Enter current password" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">New Password</label>
            <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)}
              className="input-field" placeholder="Enter new password" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Confirm New Password</label>
            <input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)}
              className="input-field" placeholder="Confirm new password" />
          </div>
        </div>
        <div className="flex justify-end mt-4">
          <button onClick={handleSubmit} disabled={saving} className="btn-primary">
            {saving ? "Updating..." : "Update Password"}
          </button>
        </div>
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
