import { useState, useEffect } from "react";
import api from "../../api/client";

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
  const [tab, setTab] = useState<"agreement" | "tenant">("agreement");

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="mt-1 text-sm text-gray-500">
          Manage your organisation settings
        </p>
      </div>

      {/* Tabs */}
      <div className="mb-6 flex gap-1 rounded-lg bg-gray-100 p-1 max-w-md">
        <button
          onClick={() => setTab("agreement")}
          className={`flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors min-h-[44px] ${
            tab === "agreement"
              ? "bg-white text-gray-900 shadow-sm"
              : "text-gray-600 hover:text-gray-900"
          }`}
        >
          Agreement Details
        </button>
        <button
          onClick={() => setTab("tenant")}
          className={`flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors min-h-[44px] ${
            tab === "tenant"
              ? "bg-white text-gray-900 shadow-sm"
              : "text-gray-600 hover:text-gray-900"
          }`}
        >
          Organisation
        </button>
      </div>

      {tab === "agreement" && <AgreementSettings />}
      {tab === "tenant" && <TenantSettings />}
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
      .catch(() => {});
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
    <div className="card max-w-2xl px-5 py-4">
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
  return (
    <div className="card max-w-2xl px-5 py-4">
      <h2 className="text-base font-semibold text-gray-900 mb-4">
        Organisation Settings
      </h2>
      <p className="text-sm text-gray-500">
        Timezone, branding, and thank you page defaults. Coming soon.
      </p>
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
