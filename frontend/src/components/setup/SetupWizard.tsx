import { useState, useEffect } from "react";
import api from "../../api/client";
import { useAuth } from "../../store/AuthContext";
import ImageUpload from "../common/ImageUpload";

interface Props {
  onClose: () => void;
}

const STEPS = [
  { key: "agreement", label: "Business Details", required: true },
  { key: "branding", label: "Branding", required: true },
  { key: "integrations", label: "Payments", required: true },
  { key: "membership", label: "Membership", required: false },
  { key: "terms", label: "Terms & Conditions", required: false },
] as const;

type StepKey = (typeof STEPS)[number]["key"];

export default function SetupWizard({ onClose }: Props) {
  const [step, setStep] = useState(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const { markSetupComplete } = useAuth();

  const currentStep = STEPS[step];
  const isLast = step === STEPS.length - 1;

  const handleNext = () => {
    setError("");
    setSuccess("");
    if (isLast) {
      handleFinish();
    } else {
      setStep((s) => Math.min(s + 1, STEPS.length - 1));
    }
  };

  const handleBack = () => {
    setError("");
    setSuccess("");
    setStep((s) => Math.max(s - 1, 0));
  };

  const handleFinish = async () => {
    setSaving(true);
    try {
      await api.completeSetup();
      markSetupComplete();
      onClose();
    } catch {
      setError("Failed to complete setup");
    } finally {
      setSaving(false);
    }
  };

  const handleSkipWizard = () => {
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/50 backdrop-blur-sm" />
      <div className="relative min-h-screen flex items-start justify-center pt-8 pb-8 px-4">
        <div className="relative w-full max-w-2xl bg-white rounded-2xl shadow-2xl">
          {/* Header */}
          <div className="px-6 pt-6 pb-4 border-b border-gray-100">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">Set up your account</h2>
                <p className="text-[13px] text-gray-500 mt-0.5">
                  Complete these steps to get started with ClientRoll
                </p>
              </div>
              <button
                onClick={handleSkipWizard}
                className="text-[12px] text-gray-400 hover:text-gray-600 font-medium"
              >
                Skip for now
              </button>
            </div>

            {/* Progress */}
            <div className="flex gap-1.5">
              {STEPS.map((s, i) => (
                <button
                  key={s.key}
                  onClick={() => { setError(""); setSuccess(""); setStep(i); }}
                  className="flex-1 min-w-0 group text-center"
                >
                  <div
                    className={`h-1.5 rounded-full transition-colors ${
                      i <= step ? "bg-indigo-600" : "bg-gray-200"
                    }`}
                  />
                  <span
                    className={`mt-2 flex h-8 items-start justify-center px-1 text-[10px] leading-tight font-medium transition-colors ${
                      i === step ? "text-indigo-600" : i < step ? "text-gray-500" : "text-gray-400"
                    }`}
                  >
                    {s.label}
                    {!s.required && " (optional)"}
                  </span>
                </button>
              ))}
            </div>
          </div>

          {/* Body */}
          <div className="px-6 py-5 min-h-[320px]">
            {error && (
              <div className="mb-4 rounded-lg bg-red-50 border border-red-200 px-4 py-2.5 text-[13px] text-red-700">
                {error}
              </div>
            )}
            {success && (
              <div className="mb-4 rounded-lg bg-emerald-50 border border-emerald-200 px-4 py-2.5 text-[13px] text-emerald-700">
                {success}
              </div>
            )}

            {currentStep.key === "agreement" && (
              <AgreementStep onSaved={() => setSuccess("Saved")} onError={setError} />
            )}
            {currentStep.key === "branding" && (
              <BrandingStep onSaved={() => setSuccess("Saved")} onError={setError} />
            )}
            {currentStep.key === "integrations" && (
              <IntegrationsStep onSaved={() => setSuccess("Saved")} onError={setError} />
            )}
            {currentStep.key === "membership" && (
              <MembershipStep onSaved={() => setSuccess("Saved")} onError={setError} />
            )}
            {currentStep.key === "terms" && (
              <TermsStep onSaved={() => setSuccess("Saved")} onError={setError} />
            )}
          </div>

          {/* Footer */}
          <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
            <div>
              {step > 0 && (
                <button onClick={handleBack} className="btn-secondary text-[13px]">
                  Back
                </button>
              )}
            </div>
            <div className="flex items-center gap-3">
              {!currentStep.required && (
                <button
                  onClick={handleNext}
                  className="text-[13px] text-gray-500 hover:text-gray-700 font-medium"
                >
                  Skip this step
                </button>
              )}
              {isLast ? (
                <button onClick={handleFinish} disabled={saving} className="btn-primary text-[13px]">
                  {saving ? "Finishing..." : "Finish Setup"}
                </button>
              ) : (
                <button onClick={handleNext} className="btn-primary text-[13px]">
                  Next
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Step 1: Agreement Details ─── */
function AgreementStep({ onSaved, onError }: { onSaved: () => void; onError: (msg: string) => void }) {
  const [data, setData] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api.getAgreementSettings()
      .then((res) => { setData(res as Record<string, string>); setLoaded(true); })
      .catch(() => { onError("Failed to load agreement settings"); setLoaded(true); });
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.updateAgreementSettings(data);
      onSaved();
    } catch { onError("Failed to save"); }
    finally { setSaving(false); }
  };

  const update = (field: string, value: string) => setData((prev) => ({ ...prev, [field]: value }));

  if (!loaded) return <LoadingSpinner />;

  return (
    <div>
      <p className="text-[13px] text-gray-500 mb-4">
        These details appear on membership agreements your clients sign.
      </p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <WizardField label="Company Name" value={data.company_name} onChange={(v) => update("company_name", v)} />
        <WizardField label="Business Name" value={data.business_name} onChange={(v) => update("business_name", v)} />
        <WizardField label="ABN/ACN" value={data.abn_acn} onChange={(v) => update("abn_acn", v)} />
        <WizardField label="Trading Name" value={data.trading_name} onChange={(v) => update("trading_name", v)} />
        <WizardField label="Address" value={data.address} onChange={(v) => update("address", v)} className="sm:col-span-2" />
        <WizardField label="State" value={data.state} onChange={(v) => update("state", v)} />
        <WizardField label="Postcode" value={data.postcode} onChange={(v) => update("postcode", v)} />
        <WizardField label="Email" value={data.email} onChange={(v) => update("email", v)} type="email" />
        <WizardField label="Phone" value={data.phone} onChange={(v) => update("phone", v)} type="tel" />
      </div>
      <div className="mt-4 flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary text-[13px]">
          {saving ? "Saving..." : "Save Details"}
        </button>
      </div>
    </div>
  );
}

/* ─── Step 2: Branding ─── */
function BrandingStep({ onSaved, onError }: { onSaved: () => void; onError: (msg: string) => void }) {
  const [data, setData] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api.getTenantSettings()
      .then((res) => {
        const t = res as Record<string, string>;
        setData({
          slug: t.slug || "",
          primary_colour: t.primary_colour || "#6366f1",
          secondary_colour: t.secondary_colour || "",
          logo_url: t.logo_url || "",
        });
        setLoaded(true);
      })
      .catch(() => { onError("Failed to load settings"); setLoaded(true); });
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.updateTenantSettings(data);
      onSaved();
    } catch { onError("Failed to save"); }
    finally { setSaving(false); }
  };

  const joinUrl = data.slug ? `${window.location.origin}/join/${data.slug}` : "";

  if (!loaded) return <LoadingSpinner />;

  return (
    <div className="space-y-4">
      <p className="text-[13px] text-gray-500">
        Customise how your brand appears on public signup pages.
      </p>

      <ImageUpload
        value={data.logo_url || ""}
        entityType="tenant"
        onChange={(url) => setData({ ...data, logo_url: url })}
        label="Logo"
        hint="Shown on public signup pages"
      />

      <div>
        <label className="block text-[13px] font-medium text-gray-700 mb-1">Join page URL slug</label>
        <p className="text-[11px] text-gray-400 mb-1.5">Your public signup URL</p>
        <input
          type="text"
          value={data.slug || ""}
          onChange={(e) => setData({ ...data, slug: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "") })}
          className="input-field max-w-sm font-mono text-[13px]"
          placeholder="your-business-slug"
        />
        {joinUrl && (
          <div className="mt-2 flex items-center gap-2">
            <code className="rounded bg-gray-50 px-2 py-1 text-[11px] text-gray-600 border border-gray-200">
              {joinUrl}
            </code>
            <button
              type="button"
              onClick={() => navigator.clipboard.writeText(joinUrl)}
              className="btn-secondary text-[11px] px-2 py-1"
            >
              Copy
            </button>
          </div>
        )}
      </div>

      <div>
        <label className="block text-[13px] font-medium text-gray-700 mb-1">Brand colour</label>
        <div className="flex items-center gap-3">
          <input
            type="color"
            value={data.primary_colour || "#6366f1"}
            onChange={(e) => setData({ ...data, primary_colour: e.target.value })}
            className="h-9 w-9 rounded border border-gray-300 cursor-pointer p-0.5"
          />
          <input
            type="text"
            value={data.primary_colour || ""}
            onChange={(e) => setData({ ...data, primary_colour: e.target.value })}
            className="input-field max-w-[120px] font-mono text-[13px]"
            placeholder="#6366f1"
          />
        </div>
      </div>

      <div>
        <label className="block text-[13px] font-medium text-gray-700 mb-1">Secondary colour</label>
        <div className="flex items-center gap-3">
          <input
            type="color"
            value={data.secondary_colour || "#4f46e5"}
            onChange={(e) => setData({ ...data, secondary_colour: e.target.value })}
            className="h-9 w-9 rounded border border-gray-300 cursor-pointer p-0.5"
          />
          <input
            type="text"
            value={data.secondary_colour || ""}
            onChange={(e) => setData({ ...data, secondary_colour: e.target.value })}
            className="input-field max-w-[120px] font-mono text-[13px]"
            placeholder="#4f46e5"
          />
        </div>
      </div>

      <div className="flex justify-end">
        <button onClick={handleSave} disabled={saving} className="btn-primary text-[13px]">
          {saving ? "Saving..." : "Save Branding"}
        </button>
      </div>
    </div>
  );
}

/* ─── Step 3: Payment Integrations ─── */
function IntegrationsStep({ onSaved, onError }: { onSaved: () => void; onError: (msg: string) => void }) {
  const [integrations, setIntegrations] = useState<{ provider: string; environment: string; is_active: boolean; has_key: boolean; has_secret?: boolean; key_hint: string }[]>([]);
  const [tenantSlug, setTenantSlug] = useState("");
  const [loaded, setLoaded] = useState(false);

  const refresh = () => {
    Promise.all([
      api.listIntegrations(),
      api.getTenantSettings(),
    ]).then(([intData, tenantData]) => {
      setIntegrations((intData as { integrations: typeof integrations }).integrations || []);
      setTenantSlug((tenantData as { slug?: string }).slug || "");
      setLoaded(true);
    }).catch(() => { onError("Failed to load integrations"); setLoaded(true); });
  };

  useEffect(() => { refresh(); }, []);

  const handleSave = async (providerId: string, data: { api_key: string; api_secret: string; environment: string; is_active: boolean }) => {
    try {
      await api.saveIntegration(providerId, data);
      onSaved();
      refresh();
    } catch { onError("Failed to save " + providerId); }
  };

  const gc = integrations.find((i) => i.provider === "gocardless");
  const stripe = integrations.find((i) => i.provider === "stripe");
  const webhookUrl = tenantSlug ? `${window.location.origin}/api/webhooks/stripe/${tenantSlug}` : "";

  if (!loaded) return <LoadingSpinner />;

  return (
    <div className="space-y-5">
      <p className="text-[13px] text-gray-500">
        Connect your payment providers to start collecting payments.
      </p>

      <IntegrationForm
        name="GoCardless"
        desc="Direct debit payments for your members"
        keyLabel="API key"
        existing={gc || null}
        hasSecret={false}
        onSave={(data) => handleSave("gocardless", data)}
      />

      <IntegrationForm
        name="Stripe"
        desc="Card payments and billing"
        keyLabel="Secret key"
        secretLabel="Webhook signing secret"
        existing={stripe || null}
        hasSecret={true}
        onSave={(data) => handleSave("stripe", data)}
        webhookUrl={webhookUrl}
      />
    </div>
  );
}

function IntegrationForm({ name, desc, keyLabel, secretLabel, existing, hasSecret, onSave, webhookUrl }: {
  name: string;
  desc: string;
  keyLabel: string;
  secretLabel?: string;
  existing: { environment: string; is_active: boolean; has_key: boolean; has_secret?: boolean; key_hint: string } | null;
  hasSecret: boolean;
  onSave: (data: { api_key: string; api_secret: string; environment: string; is_active: boolean }) => void;
  webhookUrl?: string;
}) {
  const [apiKey, setApiKey] = useState("");
  const [apiSecret, setApiSecret] = useState("");
  const [environment, setEnvironment] = useState(existing?.environment || "sandbox");
  const [isActive, setIsActive] = useState(existing?.is_active ?? true);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async () => {
    setSaving(true);
    try {
      await onSave({ api_key: apiKey, api_secret: apiSecret, environment, is_active: isActive });
    } finally { setSaving(false); }
  };

  return (
    <div className="rounded-lg border border-gray-200 p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="text-[13px] font-semibold text-gray-900">{name}</h4>
          <p className="text-[11px] text-gray-500">{desc}</p>
        </div>
        {existing?.has_key ? (
          <span className="badge badge-green text-[11px]">Connected</span>
        ) : (
          <span className="badge badge-gray text-[11px]">Not configured</span>
        )}
      </div>

      {webhookUrl && (
        <div className="rounded-md bg-indigo-50 border border-indigo-200 p-3 space-y-1.5">
          <p className="text-[11px] font-medium text-indigo-900">Webhook URL for Stripe dashboard:</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded bg-white/80 px-2 py-1 font-mono text-[10px] text-indigo-900">
              {webhookUrl}
            </code>
            <button
              type="button"
              onClick={() => navigator.clipboard.writeText(webhookUrl)}
              className="btn-secondary text-[10px] px-2 py-0.5"
            >
              Copy
            </button>
          </div>
        </div>
      )}

      <div>
        <label className="block text-[12px] font-medium text-gray-700 mb-1">{keyLabel}</label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          className="input-field font-mono text-[12px]"
          placeholder={existing?.has_key ? "Leave blank to keep existing" : "Enter key..."}
        />
      </div>

      {hasSecret && secretLabel && (
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">{secretLabel}</label>
          <input
            type="password"
            value={apiSecret}
            onChange={(e) => setApiSecret(e.target.value)}
            className="input-field font-mono text-[12px]"
            placeholder={existing?.has_secret ? "Leave blank to keep existing" : "Enter secret..."}
          />
        </div>
      )}

      <div className="flex items-center gap-4">
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Environment</label>
          <select value={environment} onChange={(e) => setEnvironment(e.target.value)} className="input-field text-[12px]">
            <option value="sandbox">Sandbox</option>
            <option value="live">Live</option>
          </select>
        </div>
        <label className="flex items-center gap-2 text-[12px] text-gray-700 mt-5">
          <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
          Active
        </label>
      </div>

      <div className="flex justify-end">
        <button onClick={handleSubmit} disabled={saving} className="btn-primary text-[12px]">
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
}

/* ─── Step 4: Create First Membership ─── */
function MembershipStep({ onSaved, onError }: { onSaved: () => void; onError: (msg: string) => void }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [totalValue, setTotalValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  const handleCreate = async () => {
    if (!title.trim()) { onError("Membership name is required"); return; }
    setSaving(true);
    try {
      await api.createMembershipType({
        title: title.trim(),
        description: description.trim() || undefined,
        total_value: parseFloat(totalValue) || 0,
      });
      setCreated(true);
      onSaved();
    } catch { onError("Failed to create membership type"); }
    finally { setSaving(false); }
  };

  if (created) {
    return (
      <div className="text-center py-8">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-emerald-50 mb-3">
          <svg className="w-6 h-6 text-emerald-600" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
          </svg>
        </div>
        <p className="text-[14px] font-medium text-gray-900">Membership type created</p>
        <p className="text-[12px] text-gray-500 mt-1">You can add term options and pricing in the Memberships section later.</p>
      </div>
    );
  }

  return (
    <div>
      <p className="text-[13px] text-gray-500 mb-4">
        Create your first membership type. You can add more and configure pricing later.
      </p>
      <div className="space-y-3">
        <WizardField label="Membership Name" value={title} onChange={setTitle} placeholder="e.g. Premium Membership" />
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={3}
            placeholder="Brief description of this membership..."
          />
        </div>
        <WizardField label="Total Value ($)" value={totalValue} onChange={setTotalValue} type="number" placeholder="0.00" />
      </div>
      <div className="mt-4 flex justify-end">
        <button onClick={handleCreate} disabled={saving} className="btn-primary text-[13px]">
          {saving ? "Creating..." : "Create Membership"}
        </button>
      </div>
    </div>
  );
}

/* ─── Step 5: Terms & Conditions ─── */
function TermsStep({ onSaved, onError }: { onSaved: () => void; onError: (msg: string) => void }) {
  const [title, setTitle] = useState("Standard Terms & Conditions");
  const [content, setContent] = useState("");
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  const handleCreate = async () => {
    if (!title.trim()) { onError("Title is required"); return; }
    if (!content.trim()) { onError("Content is required"); return; }
    setSaving(true);
    try {
      await api.createTerms({ title: title.trim(), content: content.trim() });
      setCreated(true);
      onSaved();
    } catch { onError("Failed to create terms"); }
    finally { setSaving(false); }
  };

  if (created) {
    return (
      <div className="text-center py-8">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-full bg-emerald-50 mb-3">
          <svg className="w-6 h-6 text-emerald-600" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
          </svg>
        </div>
        <p className="text-[14px] font-medium text-gray-900">Terms & conditions created</p>
        <p className="text-[12px] text-gray-500 mt-1">You can edit and attach them to memberships later.</p>
      </div>
    );
  }

  return (
    <div>
      <p className="text-[13px] text-gray-500 mb-4">
        Add your terms and conditions. These can be attached to membership types.
      </p>
      <div className="space-y-3">
        <WizardField label="Title" value={title} onChange={setTitle} />
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Content</label>
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={8}
            placeholder="Enter your terms and conditions..."
          />
        </div>
      </div>
      <div className="mt-4 flex justify-end">
        <button onClick={handleCreate} disabled={saving} className="btn-primary text-[13px]">
          {saving ? "Creating..." : "Create Terms"}
        </button>
      </div>
    </div>
  );
}

/* ─── Shared Components ─── */
function WizardField({ label, value, onChange, type = "text", className = "", placeholder }: {
  label: string;
  value?: string;
  onChange: (v: string) => void;
  type?: string;
  className?: string;
  placeholder?: string;
}) {
  return (
    <div className={className}>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">{label}</label>
      <input
        type={type}
        value={value || ""}
        onChange={(e) => onChange(e.target.value)}
        className="input-field"
        placeholder={placeholder}
      />
    </div>
  );
}

function LoadingSpinner() {
  return (
    <div className="flex justify-center py-12">
      <div className="h-5 w-5 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-600" />
    </div>
  );
}
