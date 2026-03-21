import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import api from "../../api/client";
import ImageUpload from "../common/ImageUpload";
import type { Bonus, MembershipType, TermOption } from "../../types/memberships";
interface Terms { id: string; title: string; version: number; is_active: boolean; }
const AVAILABLE_PAYMENT_FREQUENCIES = [
  { key: "upfront", label: "Upfront" },
  { key: "weekly", label: "Weekly" },
  { key: "fortnightly", label: "Fortnightly" },
  { key: "monthly", label: "Monthly" },
  { key: "quarterly", label: "Quarterly" },
  { key: "custom_schedule", label: "Custom instalments" },
];

const membershipDisplayTitle = (mt: Pick<MembershipType, "membership_name" | "membership_level" | "title">) => {
  const name = (mt.membership_name || "").trim();
  const level = (mt.membership_level || "").trim();
  if (name && level) return `${name} - ${level}`;
  if (name) return name;
  return mt.title;
};

export default function MembershipTypeDetail() {
  const { mtId } = useParams();
  const navigate = useNavigate();
  const [mt, setMt] = useState<MembershipType | null>(null);
  const [allBonuses, setAllBonuses] = useState<Bonus[]>([]);
  const [allTerms, setAllTerms] = useState<Terms[]>([]);
  const [attachedTerms, setAttachedTerms] = useState<Terms[]>([]);
  const [loading, setLoading] = useState(true);
  const [updatingPopular, setUpdatingPopular] = useState(false);
  const [savingIdentity, setSavingIdentity] = useState(false);
  const [identityForm, setIdentityForm] = useState({
    membership_name: "",
    membership_level: "",
    currency_code: "AUD",
    apply_sales_tax: true,
    use_tenant_default_payment_frequencies: true,
    enabled_payment_frequencies: ["weekly"] as string[],
  });
  const [enabledCurrencies, setEnabledCurrencies] = useState<string[]>(["AUD"]);
  const [salesTaxName, setSalesTaxName] = useState("GST");
  const [salesTaxPercent, setSalesTaxPercent] = useState(10);
  const [savingContent, setSavingContent] = useState(false);
  const [contentForm, setContentForm] = useState({ summary: "", description: "", inclusions: "", benefits: "" });
  const [tab, setTab] = useState<"terms" | "bonuses" | "tcs">("terms");

  const load = async () => {
    if (!mtId) return;
    try {
      const [mtData, bonusData, termsData, mtTermsData] = await Promise.all([
        api.getMembershipType(mtId),
        api.listBonuses(),
        api.listTerms(),
        fetch(`/api/tenant/membership-types/${mtId}/terms`, {
          headers: { Authorization: `Bearer ${localStorage.getItem("access_token")}` },
        }).then(r => r.json()),
      ]);
      setMt(mtData as MembershipType);
      const mtTyped = mtData as MembershipType;
      setIdentityForm({
        membership_name: mtTyped.membership_name || "",
        membership_level: mtTyped.membership_level || "",
        currency_code: mtTyped.currency_code || "AUD",
        apply_sales_tax: mtTyped.apply_sales_tax !== false,
        use_tenant_default_payment_frequencies: !(mtTyped.enabled_payment_frequencies && mtTyped.enabled_payment_frequencies.length > 0),
        enabled_payment_frequencies: (mtTyped.enabled_payment_frequencies && mtTyped.enabled_payment_frequencies.length > 0)
          ? mtTyped.enabled_payment_frequencies
          : ["weekly"],
      });
      setContentForm({
        summary: (mtTyped as any).summary || "",
        description: mtTyped.description || "",
        inclusions: mtTyped.inclusions || "",
        benefits: (mtTyped as any).benefits || "",
      });
      setAllBonuses(((bonusData as { bonuses: Bonus[] }).bonuses) || []);
      setAllTerms(((termsData as { terms: Terms[] }).terms) || []);
      setAttachedTerms((mtTermsData as { terms: Terms[] }).terms || []);

      // Load bonuses for each term option
      if (mtTyped.term_options) {
        for (const to of mtTyped.term_options) {
          const bonusRes = (await api.getTermOptionBonuses(to.id)) as { bonuses: Bonus[] };
          to.bonuses = bonusRes.bonuses || [];
        }
        setMt({ ...mtTyped });
      }
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => {
    load();
    api.getTenantSettings()
      .then((res) => {
        const tenant = res as { enabled_currencies?: string[]; sales_tax_name?: string; sales_tax_percent?: number };
        const list = tenant.enabled_currencies || ["AUD"];
        const normalized = Array.from(new Set(list.map((c) => String(c).toUpperCase()))).filter(Boolean);
        if (!normalized.includes("AUD")) normalized.unshift("AUD");
        setEnabledCurrencies(normalized);
        setSalesTaxName(String(tenant.sales_tax_name || "GST"));
        setSalesTaxPercent(Number.isFinite(Number(tenant.sales_tax_percent)) ? Number(tenant.sales_tax_percent) : 10);
      })
      .catch(() => {
        setEnabledCurrencies(["AUD"]);
        setSalesTaxName("GST");
        setSalesTaxPercent(10);
      });
  }, [mtId]);

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>;
  if (!mt) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Membership type not found</div>;

  const toggleMostPopular = async (next: boolean) => {
    setUpdatingPopular(true);
    try {
      await api.updateMembershipType(mt.id, { is_most_popular: next });
      await load();
    } catch {
      // no-op
    } finally {
      setUpdatingPopular(false);
    }
  };

  const tabs = [
    { key: "terms" as const, label: "Term Options & Pricing" },
    { key: "bonuses" as const, label: "Bonuses" },
    { key: "tcs" as const, label: "Terms & Conditions" },
  ];

  const saveIdentity = async () => {
    setSavingIdentity(true);
    try {
      await api.updateMembershipType(mt.id, {
        membership_name: identityForm.membership_name.trim(),
        membership_level: identityForm.membership_level.trim(),
        currency_code: identityForm.currency_code || "AUD",
        apply_sales_tax: identityForm.apply_sales_tax,
        enabled_payment_frequencies: identityForm.use_tenant_default_payment_frequencies
          ? null
          : identityForm.enabled_payment_frequencies,
      });
      await load();
    } finally {
      setSavingIdentity(false);
    }
  };

  const saveContent = async () => {
    setSavingContent(true);
    try {
      await api.updateMembershipType(mt.id, {
        summary: contentForm.summary,
        description: contentForm.description,
        inclusions: contentForm.inclusions,
        benefits: contentForm.benefits,
      });
      await load();
    } finally {
      setSavingContent(false);
    }
  };

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-3 mb-5">
        <button onClick={() => navigate("/memberships")} className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100">
          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
        </button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h1 className="page-title">{membershipDisplayTitle(mt)}</h1>
            <span className={`badge ${mt.is_active ? "badge-green" : "badge-gray"}`}>{mt.is_active ? "Active" : "Draft"}</span>
            {mt.is_most_popular && <span className="badge badge-purple">Most Popular</span>}
          </div>
          <p className="text-[12px] text-gray-400 font-mono">/{mt.slug}</p>
          <label className="mt-2 inline-flex items-center gap-2 text-[12px] text-gray-600">
            <input
              type="checkbox"
              checked={!!mt.is_most_popular}
              onChange={(e) => toggleMostPopular(e.target.checked)}
              disabled={updatingPopular}
              className="h-4 w-4 rounded border-gray-300 text-indigo-600"
            />
            {updatingPopular ? "Updating..." : "Mark as Most Popular"}
          </label>
        </div>
      </div>

      <div className="card mb-5">
        <div className="card-header">
          <h3>Membership Identity</h3>
        </div>
        <div className="card-body">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Membership Name</label>
              <input
                value={identityForm.membership_name}
                onChange={(e) => setIdentityForm({ ...identityForm, membership_name: e.target.value })}
                className="input-field"
              />
            </div>
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Level</label>
              <input
                value={identityForm.membership_level}
                onChange={(e) => setIdentityForm({ ...identityForm, membership_level: e.target.value })}
                className="input-field"
                placeholder="Support, Premium, Elite..."
              />
            </div>
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Currency</label>
              <select
                value={identityForm.currency_code}
                onChange={(e) => setIdentityForm({ ...identityForm, currency_code: e.target.value })}
                className="input-field"
              >
                {enabledCurrencies.map((code) => <option key={code} value={code}>{code}</option>)}
              </select>
            </div>
          </div>
          <div className="mt-4">
            <label className="flex items-center gap-2 text-[13px] text-gray-700 mb-3">
              <input
                type="checkbox"
                checked={identityForm.apply_sales_tax}
                onChange={(e) =>
                  setIdentityForm((prev) => ({
                    ...prev,
                    apply_sales_tax: e.target.checked,
                  }))
                }
                className="h-4 w-4 rounded border-gray-300 text-indigo-600"
              />
              Apply {salesTaxName} to this membership (inclusive pricing)
            </label>
            <label className="flex items-center gap-2 text-[13px] text-gray-700">
              <input
                type="checkbox"
                checked={identityForm.use_tenant_default_payment_frequencies}
                onChange={(e) =>
                  setIdentityForm((prev) => ({
                    ...prev,
                    use_tenant_default_payment_frequencies: e.target.checked,
                  }))
                }
                className="h-4 w-4 rounded border-gray-300 text-indigo-600"
              />
              Use tenant default signup payment frequencies
            </label>
            {!identityForm.use_tenant_default_payment_frequencies && (
              <div className="mt-2">
                <label className="block text-[12px] font-medium text-gray-700 mb-1">Per-membership frequency override</label>
                <div className="flex flex-wrap gap-2">
                  {AVAILABLE_PAYMENT_FREQUENCIES.map((item) => (
                    <label key={item.key} className="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-gray-50 px-2.5 py-1.5 text-[12px] text-gray-700">
                      <input
                        type="checkbox"
                        checked={identityForm.enabled_payment_frequencies.includes(item.key)}
                        onChange={(e) => {
                          setIdentityForm((prev) => {
                            const next = new Set(prev.enabled_payment_frequencies);
                            if (e.target.checked) next.add(item.key);
                            else next.delete(item.key);
                            if (next.size === 0) next.add("weekly");
                            return { ...prev, enabled_payment_frequencies: Array.from(next) };
                          });
                        }}
                        className="h-3.5 w-3.5 rounded border-gray-300 text-indigo-600"
                      />
                      {item.label}
                    </label>
                  ))}
                </div>
              </div>
            )}
          </div>
          <div className="mt-3 flex justify-end">
            <button type="button" onClick={saveIdentity} disabled={savingIdentity} className="btn-primary">
              {savingIdentity ? "Saving..." : "Save"}
            </button>
          </div>
        </div>
      </div>

      <div className="card mb-5">
        <div className="card-header">
          <h3>Content & Copy</h3>
        </div>
        <div className="card-body space-y-4">
          <ImageUpload
            label="Hero image"
            hint="Shown at the top of the signup page. Recommended: 1200×600px, landscape."
            currentUrl={mt.top_image_url || null}
            entityType="membership"
            onUploaded={async (url) => {
              await api.updateMembershipType(mt.id, { top_image_url: url });
              await load();
            }}
            onRemoved={async () => {
              await api.updateMembershipType(mt.id, { top_image_url: null });
              await load();
            }}
          />
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Summary</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Short 1-2 line tagline shown on membership cards and the signup form header</p>
            <textarea
              value={contentForm.summary}
              onChange={(e) => setContentForm({ ...contentForm, summary: e.target.value })}
              className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
              rows={2}
              placeholder="Gentle, therapeutic support for the lymphatic system"
            />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Full sales copy shown on the membership detail page under "About This Membership"</p>
            <textarea
              value={contentForm.description}
              onChange={(e) => setContentForm({ ...contentForm, description: e.target.value })}
              className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
              rows={6}
              placeholder="Detailed description of the membership, who it's for, what it does..."
            />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Inclusions</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Structured list of what's included — shown under "What's Included" on the detail page</p>
            <textarea
              value={contentForm.inclusions}
              onChange={(e) => setContentForm({ ...contentForm, inclusions: e.target.value })}
              className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
              rows={5}
              placeholder={"• 3 sessions per week\n• Session duration: 60–90 minutes\n• Progress tracking included"}
            />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Benefits</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Expected outcomes and benefits — shown on the detail page and the signup form summary</p>
            <textarea
              value={contentForm.benefits}
              onChange={(e) => setContentForm({ ...contentForm, benefits: e.target.value })}
              className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
              rows={4}
              placeholder={"• Reduced swelling and fluid retention\n• Improved mobility and comfort\n• Better quality of life"}
            />
          </div>
          <div className="flex justify-end">
            <button type="button" onClick={saveContent} disabled={savingContent} className="btn-primary">
              {savingContent ? "Saving..." : "Save"}
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-5">
        {tabs.map((t) => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 transition-colors -mb-px ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {tab === "terms" && <TermOptionsTab mt={mt} salesTaxName={salesTaxName} salesTaxPercent={salesTaxPercent} onReload={load} />}
      {tab === "bonuses" && <BonusAssignmentTab mt={mt} allBonuses={allBonuses} currencyCode={(mt.currency_code || "AUD").toUpperCase()} onReload={load} />}
      {tab === "tcs" && <TermsAttachTab mtId={mt.id} allTerms={allTerms} attached={attachedTerms} onReload={load} />}
    </div>
  );
}

// Auto-calculate payment frequencies from duration + total
function autoCalcFees(duration: number, total: number) {
  if (!duration || !total) return { weekly: "", fortnightly: "", monthly: "", quarterly: "" };
  const months = duration;
  const weekly = Math.round((total / (months * 4.33)) * 100) / 100;
  const fortnightly = Math.round((total / (months * 2.17)) * 100) / 100;
  const monthly = Math.round((total / months) * 100) / 100;
  const quarterly = months >= 3 ? Math.round((total / (months / 3)) * 100) / 100 : "";
  return { weekly: String(weekly), fortnightly: String(fortnightly), monthly: String(monthly), quarterly: quarterly ? String(quarterly) : "" };
}

function weeksToMonths(weeks: number): number {
  // DB stores integer months; round up so short weekly plans remain valid.
  return Math.max(1, Math.ceil(weeks / 4.345));
}

function toWholeNumber(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

// === Term Options Tab ===
function TermOptionsTab({ mt, salesTaxName, salesTaxPercent, onReload }: { mt: MembershipType; salesTaxName: string; salesTaxPercent: number; onReload: () => void }) {
  type NumericField = number | "";
  const parseNumericInput = (value: string): NumericField => (value === "" ? "" : Number(value));
  const toNumberOrZero = (value: NumericField) => (value === "" ? 0 : value);

  const [showAdd, setShowAdd] = useState(false);
  const currencyCode = (mt.currency_code || "AUD").toUpperCase();
  const [form, setForm] = useState({
    duration_unit: "month" as "month" | "week",
    duration_value: 6,
    fee_total: "" as NumericField,
    fee_weekly: "",
    fee_fortnightly: "",
    fee_monthly: "",
    fee_quarterly: "",
    allow_upfront: true,
    num_payments: "",
    continue_after_term: false,
    custom_schedule_enabled: false,
    custom_schedule_label: "",
    custom_schedule_amount: "",
    custom_schedule_interval_unit: "month",
    custom_schedule_interval_count: "",
    custom_schedule_num_payments: "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const durationMonths = (nextForm = form) =>
    nextForm.duration_unit === "week"
      ? weeksToMonths(Number(nextForm.duration_value))
      : Number(nextForm.duration_value);

  // Auto-calc when duration or total changes in add form
  const updateWithCalc = (key: "duration_value" | "fee_total", value: number | NumericField) => {
    const newForm = { ...form, [key]: value };
    const dur = durationMonths(newForm);
    const tot = key === "fee_total" ? toNumberOrZero(value as NumericField) : toNumberOrZero(form.fee_total);
    const calc = autoCalcFees(dur, tot);
    setForm({ ...newForm, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
  };

  const applyPreset = (preset: "10w" | "20w" | "2x6m" | "3x1m") => {
    const total = toNumberOrZero(form.fee_total);
    if (preset === "10w") {
      setForm({
        ...form,
        duration_unit: "week",
        duration_value: 10,
        custom_schedule_enabled: true,
        custom_schedule_label: "10 weekly payments",
        custom_schedule_interval_unit: "week",
        custom_schedule_interval_count: "1",
        custom_schedule_num_payments: "10",
        custom_schedule_amount: total > 0 ? String((total / 10).toFixed(2)) : "",
      });
      return;
    }
    if (preset === "20w") {
      setForm({
        ...form,
        duration_unit: "week",
        duration_value: 20,
        custom_schedule_enabled: true,
        custom_schedule_label: "20 weekly payments",
        custom_schedule_interval_unit: "week",
        custom_schedule_interval_count: "1",
        custom_schedule_num_payments: "20",
        custom_schedule_amount: total > 0 ? String((total / 20).toFixed(2)) : "",
      });
      return;
    }
    if (preset === "2x6m") {
      setForm({
        ...form,
        duration_unit: "month",
        duration_value: 12,
        custom_schedule_enabled: true,
        custom_schedule_label: "2 payments, 6 months apart",
        custom_schedule_interval_unit: "month",
        custom_schedule_interval_count: "6",
        custom_schedule_num_payments: "2",
        custom_schedule_amount: total > 0 ? String((total / 2).toFixed(2)) : "",
      });
      return;
    }
    setForm({
      ...form,
      duration_unit: "month",
      duration_value: 3,
      custom_schedule_enabled: true,
      custom_schedule_label: "3 monthly payments",
      custom_schedule_interval_unit: "month",
      custom_schedule_interval_count: "1",
      custom_schedule_num_payments: "3",
      custom_schedule_amount: total > 0 ? String((total / 3).toFixed(2)) : "",
    });
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (!Number.isInteger(Number(form.duration_value)) || Number(form.duration_value) <= 0) {
      setError(`Duration must be a whole number of ${form.duration_unit === "week" ? "weeks" : "months"}.`);
      return;
    }
    if (form.num_payments !== "" && (!Number.isInteger(Number(form.num_payments)) || Number(form.num_payments) <= 0)) {
      setError("Number of payments must be a whole number.");
      return;
    }
    if (form.custom_schedule_enabled) {
      if (!form.custom_schedule_amount || Number(form.custom_schedule_amount) <= 0) {
        setError("Custom schedule amount must be greater than 0.");
        return;
      }
      if (!form.custom_schedule_interval_count || !Number.isInteger(Number(form.custom_schedule_interval_count)) || Number(form.custom_schedule_interval_count) <= 0) {
        setError("Custom interval must be a whole number greater than 0.");
        return;
      }
      if (!form.custom_schedule_num_payments || !Number.isInteger(Number(form.custom_schedule_num_payments)) || Number(form.custom_schedule_num_payments) <= 0) {
        setError("Custom number of payments must be a whole number greater than 0.");
        return;
      }
    }
    setSaving(true);
    try {
      await api.createTermOption(mt.id, {
        duration_months: durationMonths(),
        allow_upfront: form.allow_upfront,
        fee_total: toNumberOrZero(form.fee_total),
        fee_weekly: form.fee_weekly ? Number(form.fee_weekly) : null,
        fee_fortnightly: form.fee_fortnightly ? Number(form.fee_fortnightly) : null,
        fee_monthly: form.fee_monthly ? Number(form.fee_monthly) : null,
        fee_quarterly: form.fee_quarterly ? Number(form.fee_quarterly) : null,
        num_payments: form.num_payments ? Number(form.num_payments) : null,
        continue_after_term: form.continue_after_term,
        custom_schedule_label: form.custom_schedule_label || null,
        custom_schedule_amount: form.custom_schedule_amount ? Number(form.custom_schedule_amount) : null,
        custom_schedule_interval_unit: form.custom_schedule_enabled ? form.custom_schedule_interval_unit : null,
        custom_schedule_interval_count: form.custom_schedule_interval_count ? Number(form.custom_schedule_interval_count) : null,
        custom_schedule_num_payments: form.custom_schedule_num_payments ? Number(form.custom_schedule_num_payments) : null,
      });
      setShowAdd(false); onReload();
    } catch { /* handle */ }
    finally { setSaving(false); }
  };

  const handleDelete = async (toId: string) => {
    if (!confirm("Delete this term option?")) return;
    try { await api.deleteTermOption(toId); onReload(); } catch { /* handle */ }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <p className="text-[13px] text-gray-500">Define pricing options for different contract lengths. Prices are tax-inclusive ({salesTaxName} {salesTaxPercent.toFixed(2)}%).</p>
        <button onClick={() => setShowAdd(!showAdd)} className="btn-primary">Add term option</button>
      </div>

      {showAdd && (
        <form onSubmit={handleAdd} className="card px-5 py-4 mb-4 space-y-3">
          {error && <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Term duration *</label>
              <div className="flex gap-2">
                <input type="number" value={form.duration_value} onChange={(e) => updateWithCalc("duration_value", toWholeNumber(e.target.value))} className="input-field" min="1" step="1" required />
                <select
                  value={form.duration_unit}
                  onChange={(e) => {
                    const next = { ...form, duration_unit: e.target.value as "month" | "week" };
                    const calc = autoCalcFees(durationMonths(next), toNumberOrZero(form.fee_total));
                    setForm({ ...next, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
                  }}
                  className="input-field max-w-[130px]"
                >
                  <option value="month">Months</option>
                  <option value="week">Weeks</option>
                </select>
              </div>
              <p className="text-[10px] text-gray-400 mt-0.5">Stored as {durationMonths()} month{durationMonths() === 1 ? "" : "s"} for term-end rules.</p>
            </div>
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Total fee ({currencyCode}, incl {salesTaxName}) *</label>
              <input type="number" value={form.fee_total} onChange={(e) => updateWithCalc("fee_total", parseNumericInput(e.target.value))} className="input-field" min="0" step="0.01" required />
            </div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Weekly ({currencyCode}, incl {salesTaxName}) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_weekly} onChange={(e) => setForm({ ...form, fee_weekly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Fortnightly ({currencyCode}, incl {salesTaxName}) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_fortnightly} onChange={(e) => setForm({ ...form, fee_fortnightly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Monthly ({currencyCode}, incl {salesTaxName}) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_monthly} onChange={(e) => setForm({ ...form, fee_monthly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Quarterly ({currencyCode}, incl {salesTaxName}) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_quarterly} onChange={(e) => setForm({ ...form, fee_quarterly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Fixed # payments</label><input type="number" value={form.num_payments} onChange={(e) => setForm({ ...form, num_payments: e.target.value })} className="input-field" min="1" step="1" placeholder="Leave blank = continuous" /><p className="text-[10px] text-gray-400 mt-0.5">Use whole numbers only (especially for weekly/fortnightly plans).</p></div>
            <div className="flex flex-col gap-2 pb-1 justify-end">
              <label className="flex items-center gap-2 text-[13px] text-gray-700"><input type="checkbox" checked={form.allow_upfront} onChange={(e) => setForm({ ...form, allow_upfront: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />Payment in advance</label>
              <label className="flex items-center gap-2 text-[13px] text-gray-700"><input type="checkbox" checked={form.continue_after_term} onChange={(e) => setForm({ ...form, continue_after_term: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />Continue billing after term</label>
            </div>
            <div className="sm:col-span-2">
              <div className="mb-2">
                <p className="text-[12px] font-medium text-gray-700 mb-1">Quick flexible presets</p>
                <div className="flex flex-wrap gap-2">
                  <button type="button" onClick={() => applyPreset("10w")} className="btn-secondary text-[11px] px-2 py-1">10 weekly payments</button>
                  <button type="button" onClick={() => applyPreset("20w")} className="btn-secondary text-[11px] px-2 py-1">20 weekly payments</button>
                  <button type="button" onClick={() => applyPreset("2x6m")} className="btn-secondary text-[11px] px-2 py-1">2 payments, 6 months apart</button>
                  <button type="button" onClick={() => applyPreset("3x1m")} className="btn-secondary text-[11px] px-2 py-1">3 monthly payments</button>
                </div>
              </div>
              <label className="flex items-center gap-2 text-[13px] text-gray-700 mb-2">
                <input type="checkbox" checked={form.custom_schedule_enabled} onChange={(e) => setForm({ ...form, custom_schedule_enabled: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
                Enable custom instalment plan
              </label>
              {form.custom_schedule_enabled && (
                <div className="grid grid-cols-2 gap-3 rounded-lg border border-indigo-100 bg-indigo-50/40 p-3">
                  <div className="col-span-2">
                    <label className="block text-[12px] font-medium text-gray-700 mb-1">Label (optional)</label>
                    <input type="text" value={form.custom_schedule_label} onChange={(e) => setForm({ ...form, custom_schedule_label: e.target.value })} className="input-field" placeholder="e.g. 2 payments, 3 months apart" />
                  </div>
                  <div>
                    <label className="block text-[12px] font-medium text-gray-700 mb-1">Payment amount ({currencyCode})</label>
                    <input type="number" value={form.custom_schedule_amount} onChange={(e) => setForm({ ...form, custom_schedule_amount: e.target.value })} className="input-field" min="0" step="0.01" />
                  </div>
                  <div>
                    <label className="block text-[12px] font-medium text-gray-700 mb-1"># payments</label>
                    <input type="number" value={form.custom_schedule_num_payments} onChange={(e) => setForm({ ...form, custom_schedule_num_payments: e.target.value })} className="input-field" min="1" step="1" />
                  </div>
                  <div>
                    <label className="block text-[12px] font-medium text-gray-700 mb-1">Every</label>
                    <input type="number" value={form.custom_schedule_interval_count} onChange={(e) => setForm({ ...form, custom_schedule_interval_count: e.target.value })} className="input-field" min="1" step="1" />
                  </div>
                  <div>
                    <label className="block text-[12px] font-medium text-gray-700 mb-1">Interval unit</label>
                    <select value={form.custom_schedule_interval_unit} onChange={(e) => setForm({ ...form, custom_schedule_interval_unit: e.target.value as "week" | "month" })} className="input-field">
                      <option value="week">Weeks</option>
                      <option value="month">Months</option>
                    </select>
                  </div>
                </div>
              )}
            </div>
          </div>
          <div className="flex gap-2 justify-end pt-1">
            <button type="button" onClick={() => setShowAdd(false)} className="btn-secondary text-[12px]">Cancel</button>
            <button type="submit" disabled={saving} className="btn-primary">{saving ? "Adding..." : "Add"}</button>
          </div>
        </form>
      )}

      {(!mt.term_options || mt.term_options.length === 0) ? (
        <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No term options yet</div>
      ) : (
        <div className="space-y-3">
          {mt.term_options.map((to) => (
            <TermOptionCard key={to.id} termOption={to} currencyCode={currencyCode} salesTaxName={salesTaxName} mtId={mt.id} onReload={onReload} onDelete={() => handleDelete(to.id)} />
          ))}
        </div>
      )}
    </div>
  );
}

// === Editable Term Option Card ===
function TermOptionCard({ termOption: to, currencyCode, salesTaxName, mtId, onReload, onDelete }: {
  termOption: TermOption; currencyCode: string; salesTaxName: string; mtId: string; onReload: () => void; onDelete: () => void;
}) {
  type NumericField = number | "";
  const parseNumericInput = (value: string): NumericField => (value === "" ? "" : Number(value));
  const toNumberOrZero = (value: NumericField) => (value === "" ? 0 : value);

  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({
    duration_unit: "month" as "month" | "week",
    duration_value: to.duration_months,
    fee_total: to.fee_total as NumericField,
    fee_weekly: to.fee_weekly ?? "",
    fee_fortnightly: to.fee_fortnightly ?? "",
    fee_monthly: to.fee_monthly ?? "",
    fee_quarterly: to.fee_quarterly ?? "",
    allow_upfront: to.allow_upfront,
    num_payments: to.num_payments ?? "",
    continue_after_term: !!to.continue_after_term,
    custom_schedule_enabled: !!to.custom_schedule_enabled,
    custom_schedule_label: to.custom_schedule_label ?? "",
    custom_schedule_amount: to.custom_schedule_amount ?? "",
    custom_schedule_interval_unit: (to.custom_schedule_interval_unit ?? "month") as "week" | "month",
    custom_schedule_interval_count: to.custom_schedule_interval_count ?? "",
    custom_schedule_num_payments: to.custom_schedule_num_payments ?? "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const editDurationMonths = (nextForm = form) =>
    nextForm.duration_unit === "week"
      ? weeksToMonths(Number(nextForm.duration_value))
      : Number(nextForm.duration_value);

  const updateEditCalc = (key: "duration_value" | "fee_total", value: number | NumericField) => {
    const newForm = { ...form, [key]: value };
    const dur = editDurationMonths(newForm);
    const tot = key === "fee_total" ? toNumberOrZero(value as NumericField) : toNumberOrZero(form.fee_total);
    const calc = autoCalcFees(dur, tot);
    setForm({ ...newForm, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
  };

  const handleSave = async () => {
    setError("");
    if (!Number.isInteger(Number(form.duration_value)) || Number(form.duration_value) <= 0) {
      setError(`Duration must be a whole number of ${form.duration_unit === "week" ? "weeks" : "months"}.`);
      return;
    }
    if (form.num_payments !== "" && (!Number.isInteger(Number(form.num_payments)) || Number(form.num_payments) <= 0)) {
      setError("Number of payments must be a whole number.");
      return;
    }
    if (form.custom_schedule_enabled) {
      if (!form.custom_schedule_amount || Number(form.custom_schedule_amount) <= 0) {
        setError("Custom schedule amount must be greater than 0.");
        return;
      }
      if (!form.custom_schedule_interval_count || !Number.isInteger(Number(form.custom_schedule_interval_count)) || Number(form.custom_schedule_interval_count) <= 0) {
        setError("Custom interval must be a whole number greater than 0.");
        return;
      }
      if (!form.custom_schedule_num_payments || !Number.isInteger(Number(form.custom_schedule_num_payments)) || Number(form.custom_schedule_num_payments) <= 0) {
        setError("Custom number of payments must be a whole number greater than 0.");
        return;
      }
    }
    setSaving(true);
    try {
      await api.updateTermOption(mtId, to.id, {
        duration_months: editDurationMonths(),
        allow_upfront: form.allow_upfront,
        fee_total: toNumberOrZero(form.fee_total),
        fee_weekly: form.fee_weekly !== "" ? Number(form.fee_weekly) : null,
        fee_fortnightly: form.fee_fortnightly !== "" ? Number(form.fee_fortnightly) : null,
        fee_monthly: form.fee_monthly !== "" ? Number(form.fee_monthly) : null,
        fee_quarterly: form.fee_quarterly !== "" ? Number(form.fee_quarterly) : null,
        num_payments: form.num_payments !== "" ? Number(form.num_payments) : null,
        continue_after_term: form.continue_after_term,
        custom_schedule_label: form.custom_schedule_label || null,
        custom_schedule_amount: form.custom_schedule_amount !== "" ? Number(form.custom_schedule_amount) : null,
        custom_schedule_interval_unit: form.custom_schedule_enabled ? form.custom_schedule_interval_unit : null,
        custom_schedule_interval_count: form.custom_schedule_interval_count !== "" ? Number(form.custom_schedule_interval_count) : null,
        custom_schedule_num_payments: form.custom_schedule_num_payments !== "" ? Number(form.custom_schedule_num_payments) : null,
      });
      setEditing(false);
      onReload();
    } catch { /* handle */ }
    finally { setSaving(false); }
  };

  if (!editing) {
    return (
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-100 bg-gray-50/50">
          <div className="flex items-center gap-3">
            <span className="text-[14px] font-semibold text-gray-900">{to.duration_months} months</span>
            <span className="text-[13px] font-semibold text-gray-900 tabular-nums">{currencyCode} ${to.fee_total.toLocaleString()}</span>
            {to.allow_upfront && <span className="badge badge-green">Upfront OK</span>}
            <span className="text-[12px] text-gray-400">{to.bonuses?.length || 0} bonuses</span>
          </div>
          <div className="flex items-center gap-1">
            <button onClick={() => setEditing(true)} className="btn-ghost text-[12px]">Edit</button>
            <button onClick={onDelete} className="rounded p-1.5 text-gray-400 hover:text-red-500 hover:bg-red-50">
              <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
            </button>
          </div>
        </div>
        <div className="px-5 py-3">
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 text-[13px]">
            <div><span className="text-gray-400 block mb-0.5">Weekly</span><span className="tabular-nums text-gray-900">{to.fee_weekly ? `${currencyCode} $${to.fee_weekly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Fortnightly</span><span className="tabular-nums text-gray-900">{to.fee_fortnightly ? `${currencyCode} $${to.fee_fortnightly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Monthly</span><span className="tabular-nums text-gray-900">{to.fee_monthly ? `${currencyCode} $${to.fee_monthly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Quarterly</span><span className="tabular-nums text-gray-900">{to.fee_quarterly ? `${currencyCode} $${to.fee_quarterly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">In Advance</span><span>{to.allow_upfront ? "Yes" : "Post-payment"}</span></div>
            <div><span className="text-gray-400 block mb-0.5"># Payments</span><span className="tabular-nums text-gray-900">{to.num_payments ? `${to.num_payments} then stops` : "Continuous"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">After Term</span><span>{to.continue_after_term ? "Continue billing" : "Stops"}</span></div>
            <div className="sm:col-span-3 lg:col-span-2"><span className="text-gray-400 block mb-0.5">Custom Plan</span><span className="text-gray-900">{to.custom_schedule_enabled ? (to.custom_schedule_label || `${to.custom_schedule_num_payments || 0} payments every ${to.custom_schedule_interval_count || 1} ${to.custom_schedule_interval_unit || "month"}${(to.custom_schedule_interval_count || 1) === 1 ? "" : "s"} @ ${currencyCode} $${to.custom_schedule_amount || 0}`) : "—"}</span></div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="card overflow-hidden border-indigo-200">
      <div className="flex items-center justify-between px-5 py-3 border-b border-indigo-100 bg-indigo-50/50">
        <span className="text-[13px] font-semibold text-indigo-700">Editing — {to.duration_months} months</span>
        <div className="flex items-center gap-2">
          <button onClick={() => setEditing(false)} className="btn-secondary text-[12px]">Cancel</button>
          <button onClick={handleSave} disabled={saving} className="btn-primary">{saving ? "Saving..." : "Save"}</button>
        </div>
      </div>
      <div className="px-5 py-4">
        {error && <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600 mb-3">{error}</div>}
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Term duration</label>
            <div className="flex gap-2">
              <input type="number" value={form.duration_value} onChange={(e) => updateEditCalc("duration_value", toWholeNumber(e.target.value))} className="input-field" min="1" step="1" />
              <select
                value={form.duration_unit}
                onChange={(e) => {
                  const next = { ...form, duration_unit: e.target.value as "month" | "week" };
                  const calc = autoCalcFees(editDurationMonths(next), toNumberOrZero(form.fee_total));
                  setForm({ ...next, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
                }}
                className="input-field max-w-[120px]"
              >
                <option value="month">Months</option>
                <option value="week">Weeks</option>
              </select>
            </div>
            <p className="text-[10px] text-gray-400 mt-0.5">Stored as {editDurationMonths()} month{editDurationMonths() === 1 ? "" : "s"} for term-end rules.</p>
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Total fee ({currencyCode}, incl {salesTaxName})</label>
            <input type="number" value={form.fee_total} onChange={(e) => updateEditCalc("fee_total", parseNumericInput(e.target.value))} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Weekly ({currencyCode}, incl {salesTaxName})</label>
            <input type="number" value={form.fee_weekly} onChange={(e) => setForm({ ...form, fee_weekly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Fortnightly ({currencyCode}, incl {salesTaxName})</label>
            <input type="number" value={form.fee_fortnightly} onChange={(e) => setForm({ ...form, fee_fortnightly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Monthly ({currencyCode}, incl {salesTaxName})</label>
            <input type="number" value={form.fee_monthly} onChange={(e) => setForm({ ...form, fee_monthly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Quarterly ({currencyCode}, incl {salesTaxName})</label>
            <input type="number" value={form.fee_quarterly} onChange={(e) => setForm({ ...form, fee_quarterly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1"># Payments</label>
            <input type="number" value={form.num_payments} onChange={(e) => setForm({ ...form, num_payments: e.target.value })} className="input-field" min="1" step="1" placeholder="Continuous" />
            <p className="text-[10px] text-gray-400 mt-0.5">Blank = pay until end of term</p>
          </div>
          <div className="flex flex-col gap-2 pb-1 justify-end">
            <label className="flex items-center gap-2 text-[12px] text-gray-700">
              <input type="checkbox" checked={form.allow_upfront} onChange={(e) => setForm({ ...form, allow_upfront: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
              Payment in advance
            </label>
            <label className="flex items-center gap-2 text-[12px] text-gray-700">
              <input type="checkbox" checked={form.continue_after_term} onChange={(e) => setForm({ ...form, continue_after_term: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
              Continue billing after term
            </label>
          </div>
          <div className="sm:col-span-3">
            <label className="flex items-center gap-2 text-[12px] text-gray-700 mb-2">
              <input type="checkbox" checked={form.custom_schedule_enabled} onChange={(e) => setForm({ ...form, custom_schedule_enabled: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
              Enable custom instalment plan
            </label>
            {form.custom_schedule_enabled && (
              <div className="grid grid-cols-2 gap-3 rounded-lg border border-indigo-100 bg-indigo-50/40 p-3">
                <div className="col-span-2">
                  <label className="block text-[12px] font-medium text-gray-700 mb-1">Label (optional)</label>
                  <input type="text" value={form.custom_schedule_label} onChange={(e) => setForm({ ...form, custom_schedule_label: e.target.value })} className="input-field" placeholder="e.g. 2 payments, 3 months apart" />
                </div>
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 mb-1">Payment amount ({currencyCode})</label>
                  <input type="number" value={form.custom_schedule_amount} onChange={(e) => setForm({ ...form, custom_schedule_amount: e.target.value })} className="input-field" min="0" step="0.01" />
                </div>
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 mb-1"># payments</label>
                  <input type="number" value={form.custom_schedule_num_payments} onChange={(e) => setForm({ ...form, custom_schedule_num_payments: e.target.value })} className="input-field" min="1" step="1" />
                </div>
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 mb-1">Every</label>
                  <input type="number" value={form.custom_schedule_interval_count} onChange={(e) => setForm({ ...form, custom_schedule_interval_count: e.target.value })} className="input-field" min="1" step="1" />
                </div>
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 mb-1">Interval unit</label>
                  <select value={form.custom_schedule_interval_unit} onChange={(e) => setForm({ ...form, custom_schedule_interval_unit: e.target.value as "week" | "month" })} className="input-field">
                    <option value="week">Weeks</option>
                    <option value="month">Months</option>
                  </select>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// === Bonus Assignment Tab ===
function BonusAssignmentTab({ mt, allBonuses, currencyCode, onReload }: { mt: MembershipType; allBonuses: Bonus[]; currencyCode: string; onReload: () => void }) {
  const [saving, setSaving] = useState<string | null>(null);

  const toggleBonus = async (termOptionId: string, bonusId: string, currentBonuses: Bonus[]) => {
    setSaving(termOptionId);
    const currentIds = currentBonuses.map(b => b.id);
    const newIds = currentIds.includes(bonusId)
      ? currentIds.filter(id => id !== bonusId)
      : [...currentIds, bonusId];
    try {
      await api.setTermOptionBonuses(termOptionId, newIds);
      onReload();
    } catch { /* handle */ }
    finally { setSaving(null); }
  };

  if (!mt.term_options || mt.term_options.length === 0) {
    return <div className="card px-5 py-8 text-center text-[13px] text-gray-400">Add term options first, then assign bonuses to each</div>;
  }
  if (allBonuses.length === 0) {
    return <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No bonuses created yet. <a href="/bonuses" className="text-indigo-600">Create bonuses first</a></div>;
  }

  return (
    <div>
      <p className="text-[13px] text-gray-500 mb-4">Assign bonuses to each term option. Longer terms can offer more bonuses to incentivise commitment.</p>

      <div className="space-y-4">
        {mt.term_options.map((to) => (
          <div key={to.id} className="card overflow-hidden">
            <div className="card-header">
              <h3>{to.duration_months} months — {currencyCode} ${to.fee_total.toLocaleString()}</h3>
              <span className="text-[11px] text-gray-400">{to.bonuses?.length || 0} bonus{(to.bonuses?.length || 0) !== 1 ? "es" : ""} assigned</span>
            </div>
            <div className="px-5 py-3">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {allBonuses.map((bonus) => {
                  const isAssigned = to.bonuses?.some(b => b.id === bonus.id) || false;
                  return (
                    <button
                      key={bonus.id}
                      onClick={() => toggleBonus(to.id, bonus.id, to.bonuses || [])}
                      disabled={saving === to.id}
                      className={`flex items-center gap-3 rounded-md border p-3 text-left transition-all ${
                        isAssigned
                          ? "border-indigo-300 bg-indigo-50"
                          : "border-gray-200 hover:border-gray-300 hover:bg-gray-50"
                      }`}
                    >
                      <div className={`flex h-5 w-5 items-center justify-center rounded border transition-colors ${
                        isAssigned ? "border-indigo-600 bg-indigo-600" : "border-gray-300"
                      }`}>
                        {isAssigned && <svg className="h-3 w-3 text-white" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-[13px] font-medium text-gray-900 truncate">{bonus.title}</p>
                        <p className="text-[11px] text-gray-500">{currencyCode} ${bonus.value.toLocaleString()} value</p>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// === T&C Attachment Tab ===
function TermsAttachTab({ mtId, allTerms, attached, onReload }: { mtId: string; allTerms: Terms[]; attached: Terms[]; onReload: () => void }) {
  const [saving, setSaving] = useState(false);

  const toggle = async (termsId: string) => {
    setSaving(true);
    const isAttached = attached.some(t => t.id === termsId);
    try {
      if (isAttached) {
        await api.detachTerms(mtId, termsId);
      } else {
        await api.attachTerms(mtId, termsId);
      }
      onReload();
    } catch { /* handle */ }
    finally { setSaving(false); }
  };

  if (allTerms.length === 0) {
    return <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No T&C documents created yet. <a href="/terms" className="text-indigo-600">Create terms first</a></div>;
  }

  return (
    <div>
      <p className="text-[13px] text-gray-500 mb-4">Select which terms & conditions apply to this membership. Clients must accept these during signup.</p>

      <div className="card overflow-hidden">
        {allTerms.map((terms) => {
          const isAttached = attached.some(t => t.id === terms.id);
          return (
            <button
              key={terms.id}
              onClick={() => toggle(terms.id)}
              disabled={saving}
              className={`flex w-full items-center gap-3 px-5 py-3 text-left border-b border-gray-100 last:border-b-0 transition-colors ${
                isAttached ? "bg-indigo-50/50" : "hover:bg-gray-50"
              }`}
            >
              <div className={`flex h-5 w-5 items-center justify-center rounded border transition-colors flex-shrink-0 ${
                isAttached ? "border-indigo-600 bg-indigo-600" : "border-gray-300"
              }`}>
                {isAttached && <svg className="h-3 w-3 text-white" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[13px] font-medium text-gray-900">{terms.title}</p>
                <p className="text-[11px] text-gray-400">Version {terms.version} · {terms.is_active ? "Active" : "Inactive"}</p>
              </div>
              {isAttached && <span className="badge badge-green">Attached</span>}
            </button>
          );
        })}
      </div>
    </div>
  );
}
