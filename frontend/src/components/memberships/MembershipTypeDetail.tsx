import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import api from "../../api/client";
import ImageUpload from "../common/ImageUpload";
import type { Bonus, MembershipType, PaymentSchedule, TermOption } from "../../types/memberships";
import PaymentSchedulesEditor from "./PaymentSchedulesEditor";
import TermLengthEditor from "./TermLengthEditor";
import TermOptionAddForm from "./TermOptionAddForm";
import TermOptionTermsEditor from "./TermOptionTermsEditor";
interface Terms { id: string; title: string; version: number; is_active: boolean; }
const AVAILABLE_PAYMENT_FREQUENCIES = [
  { key: "upfront", label: "Upfront" },
  { key: "weekly", label: "Weekly" },
  { key: "fortnightly", label: "Fortnightly" },
  { key: "monthly", label: "Monthly" },
  { key: "quarterly", label: "Quarterly" },
  { key: "custom_schedule", label: "Custom instalments" },
];
const DEFAULT_ENABLED_PAYMENT_FREQUENCIES = ["upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"];

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
  const [tenantEnabledPaymentFrequencies, setTenantEnabledPaymentFrequencies] = useState<string[]>(DEFAULT_ENABLED_PAYMENT_FREQUENCIES);
  const [salesTaxName, setSalesTaxName] = useState("GST");
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
        summary: (mtTyped as { summary?: string }).summary || "",
        description: mtTyped.description || "",
        inclusions: mtTyped.inclusions || "",
        benefits: (mtTyped as { benefits?: string }).benefits || "",
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
        const tenant = res as {
          enabled_currencies?: string[];
          enabled_payment_frequencies?: string[];
          sales_tax_name?: string;
        };
        const list = tenant.enabled_currencies || ["AUD"];
        const normalized = Array.from(new Set(list.map((c) => String(c).toUpperCase()))).filter(Boolean);
        if (!normalized.includes("AUD")) normalized.unshift("AUD");
        setEnabledCurrencies(normalized);
        const enabledFreqs = Array.isArray(tenant.enabled_payment_frequencies)
          ? Array.from(new Set(tenant.enabled_payment_frequencies.map((f) => String(f).toLowerCase()))).filter(Boolean)
          : DEFAULT_ENABLED_PAYMENT_FREQUENCIES;
        setTenantEnabledPaymentFrequencies(enabledFreqs.length > 0 ? enabledFreqs : ["weekly"]);
        setSalesTaxName(String(tenant.sales_tax_name || "GST"));
      })
      .catch(() => {
        setEnabledCurrencies(["AUD"]);
        setTenantEnabledPaymentFrequencies(DEFAULT_ENABLED_PAYMENT_FREQUENCIES);
        setSalesTaxName("GST");
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

      {tab === "terms" && (
        <TermOptionsTab
          mt={mt}
          allTerms={allTerms}
          onReload={load}
        />
      )}
      {tab === "bonuses" && <BonusAssignmentTab mt={mt} allBonuses={allBonuses} currencyCode={(mt.currency_code || "AUD").toUpperCase()} onReload={load} />}
      {tab === "tcs" && <TermsAttachTab mtId={mt.id} allTerms={allTerms} attached={attachedTerms} onReload={load} />}
    </div>
  );
}

// === Term Options Tab (v2) ===
// Each term option is presented as a card with a one-line summary and a single
// "Term, Schedules & T&Cs" disclosure that wraps the three v2 editors:
// duration, payment schedules, and per-term T&Cs. No legacy edit form — the
// inline editors handle every adjustment.
function TermOptionsTab({
  mt,
  allTerms,
  onReload,
}: {
  mt: MembershipType;
  allTerms: Terms[];
  onReload: () => void;
}) {
  const currencyCode = (mt.currency_code || "AUD").toUpperCase();
  const [showAdd, setShowAdd] = useState(false);

  const handleDelete = async (toId: string) => {
    if (!confirm("Delete this term option?")) return;
    try { await api.deleteTermOption(toId); onReload(); } catch { /* handle */ }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4 gap-2 flex-wrap">
        <p className="text-[13px] text-gray-500">
          Each term option pairs a membership length with one or more payment schedules and (optionally) its own T&amp;Cs.
        </p>
        <button onClick={() => setShowAdd(s => !s)} className="btn-primary">
          {showAdd ? "Close add form" : "Add term option"}
        </button>
      </div>

      {showAdd && (
        <TermOptionAddForm
          mtId={mt.id}
          currencyCode={currencyCode}
          onCancel={() => setShowAdd(false)}
          onCreated={() => { setShowAdd(false); onReload(); }}
        />
      )}

      {(!mt.term_options || mt.term_options.length === 0) ? (
        <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No term options yet</div>
      ) : (
        <div className="space-y-3">
          {mt.term_options.map((to) => (
            <TermOptionCard
              key={to.id}
              termOption={to}
              currencyCode={currencyCode}
              mtId={mt.id}
              allTerms={allTerms}
              onReload={onReload}
              onDelete={() => handleDelete(to.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// === Display-only Term Option card ===
// All editing happens inside the disclosure (TermLengthEditor +
// PaymentSchedulesEditor + TermOptionTermsEditor). The card header just shows
// a glanceable summary plus a delete button.
function TermOptionCard({
  termOption: to,
  currencyCode,
  mtId,
  allTerms,
  onReload,
  onDelete,
}: {
  termOption: TermOption;
  currencyCode: string;
  mtId: string;
  allTerms: Terms[];
  onReload: () => void;
  onDelete: () => void;
}) {
  const durationValue = to.duration_value ?? to.duration_months ?? 0;
  const durationUnit = to.duration_unit ?? "month";
  const durationLabel = `${durationValue} ${durationUnit}${durationValue === 1 ? "" : "s"}`;
  const schedules = (to.payment_schedules ?? []).filter(s => !s.archived_at);
  const defaultSchedule = schedules.find(s => s.is_default) ?? schedules[0];
  const attachedCount = to.attached_terms?.length ?? 0;
  const bonusCount = to.bonuses?.length ?? 0;

  return (
    <div className="card overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-100 bg-gray-50/50">
        <div className="flex items-center gap-3 flex-wrap">
          <span className="text-[14px] font-semibold text-gray-900">{durationLabel}</span>
          {to.label && <span className="text-[12px] text-gray-500 italic">"{to.label}"</span>}
          {defaultSchedule && (
            <span className="text-[13px] tabular-nums text-gray-700">
              {defaultScheduleSummary(defaultSchedule, currencyCode)}
            </span>
          )}
          <span className="badge badge-gray">{schedules.length} schedule{schedules.length === 1 ? "" : "s"}</span>
          {attachedCount > 0 && <span className="badge badge-blue">{attachedCount} T&amp;C{attachedCount === 1 ? "" : "s"}</span>}
          {bonusCount > 0 && <span className="badge badge-purple">{bonusCount} bonus{bonusCount === 1 ? "" : "es"}</span>}
        </div>
        <button
          onClick={onDelete}
          className="rounded p-1.5 text-gray-400 hover:text-red-500 hover:bg-red-50"
          title="Delete term option"
        >
          <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
          </svg>
        </button>
      </div>
      <div className="px-5 py-3">
        <details className="group">
          <summary className="cursor-pointer text-[12px] font-medium text-indigo-600 select-none">
            Term, Schedules &amp; T&amp;Cs
          </summary>
          <TermLengthEditor mtId={mtId} termOption={to} onSaved={onReload} />
          <PaymentSchedulesEditor termOption={to} currencyCode={currencyCode} onSaved={onReload} />
          <TermOptionTermsEditor termOption={to} allTerms={allTerms} onSaved={onReload} />
        </details>
      </div>
    </div>
  );
}

// defaultScheduleSummary renders a one-line description of the default
// payment schedule for the card header (e.g. "AUD $25 / week" or
// "AUD $1,200 upfront" or "AUD $100 x 12 monthly").
function defaultScheduleSummary(s: PaymentSchedule, currencyCode: string): string {
  const money = `${currencyCode} $${s.amount.toLocaleString()}`;
  if (s.kind === "upfront") {
    return `${money} upfront`;
  }
  const unit = s.interval_unit ?? "month";
  const count = s.interval_count ?? 1;
  const cadence = count === 1 ? `every ${unit}` : `every ${count} ${unit}s`;
  if (s.kind === "instalments" && s.num_payments) {
    return `${money} × ${s.num_payments} (${cadence})`;
  }
  if (s.kind === "recurring" && s.num_payments) {
    return `${money} × ${s.num_payments} (${cadence})`;
  }
  return `${money} ${cadence}`;
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
        {mt.term_options.map((to) => {
          const durationValue = to.duration_value ?? to.duration_months ?? 0;
          const durationUnit = to.duration_unit ?? "month";
          const durationLabel = `${durationValue} ${durationUnit}${durationValue === 1 ? "" : "s"}`;
          return (
            <div key={to.id} className="card overflow-hidden">
              <div className="card-header">
                <h3>{durationLabel} — {currencyCode} ${to.fee_total.toLocaleString()}</h3>
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
          );
        })}
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
      <p className="text-[13px] text-gray-500 mb-4">
        Membership-wide T&amp;Cs that apply to every term option. Need per-term wording? Use the
        "Term, Schedules &amp; T&amp;Cs" disclosure on each term option above.
      </p>

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
