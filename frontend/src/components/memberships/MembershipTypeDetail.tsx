import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import api from "../../api/client";

interface MembershipType {
  id: string; title: string; slug: string; description?: string;
  inclusions?: string; top_image_url?: string; is_active: boolean;
  total_value: number; sort_order: number; term_options?: TermOption[];
}
interface TermOption {
  id: string; duration_months: number; fee_total: number;
  fee_weekly?: number; fee_fortnightly?: number; fee_monthly?: number; fee_quarterly?: number;
  allow_upfront: boolean; num_payments?: number; bonuses?: Bonus[];
}
interface Bonus { id: string; title: string; value: number; image_url?: string; }
interface Terms { id: string; title: string; version: number; is_active: boolean; }

export default function MembershipTypeDetail() {
  const { mtId } = useParams();
  const navigate = useNavigate();
  const [mt, setMt] = useState<MembershipType | null>(null);
  const [allBonuses, setAllBonuses] = useState<Bonus[]>([]);
  const [allTerms, setAllTerms] = useState<Terms[]>([]);
  const [attachedTerms, setAttachedTerms] = useState<Terms[]>([]);
  const [loading, setLoading] = useState(true);
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
      setAllBonuses(((bonusData as { bonuses: Bonus[] }).bonuses) || []);
      setAllTerms(((termsData as { terms: Terms[] }).terms) || []);
      setAttachedTerms((mtTermsData as { terms: Terms[] }).terms || []);

      // Load bonuses for each term option
      const mtTyped = mtData as MembershipType;
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

  useEffect(() => { load(); }, [mtId]);

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>;
  if (!mt) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Membership type not found</div>;

  const tabs = [
    { key: "terms" as const, label: "Term Options & Pricing" },
    { key: "bonuses" as const, label: "Bonuses" },
    { key: "tcs" as const, label: "Terms & Conditions" },
  ];

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-3 mb-5">
        <button onClick={() => navigate("/memberships")} className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100">
          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" /></svg>
        </button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h1 className="page-title">{mt.title}</h1>
            <span className={`badge ${mt.is_active ? "badge-green" : "badge-gray"}`}>{mt.is_active ? "Active" : "Draft"}</span>
          </div>
          <p className="text-[12px] text-gray-400 font-mono">/{mt.slug}</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map((t) => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 transition-colors -mb-px ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {tab === "terms" && <TermOptionsTab mt={mt} onReload={load} />}
      {tab === "bonuses" && <BonusAssignmentTab mt={mt} allBonuses={allBonuses} onReload={load} />}
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

// === Term Options Tab ===
function TermOptionsTab({ mt, onReload }: { mt: MembershipType; onReload: () => void }) {
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState({ duration_months: 6, fee_total: 0, fee_weekly: "", fee_fortnightly: "", fee_monthly: "", fee_quarterly: "", allow_upfront: true, num_payments: "" });
  const [saving, setSaving] = useState(false);

  // Auto-calc when duration or total changes in add form
  const updateWithCalc = (key: "duration_months" | "fee_total", value: number) => {
    const newForm = { ...form, [key]: value };
    const dur = key === "duration_months" ? value : form.duration_months;
    const tot = key === "fee_total" ? value : form.fee_total;
    const calc = autoCalcFees(dur, tot);
    setForm({ ...newForm, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
  };

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault(); setSaving(true);
    try {
      await api.createTermOption(mt.id, {
        ...form,
        fee_weekly: form.fee_weekly ? Number(form.fee_weekly) : null,
        fee_fortnightly: form.fee_fortnightly ? Number(form.fee_fortnightly) : null,
        fee_monthly: form.fee_monthly ? Number(form.fee_monthly) : null,
        fee_quarterly: form.fee_quarterly ? Number(form.fee_quarterly) : null,
        num_payments: form.num_payments ? Number(form.num_payments) : null,
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
        <p className="text-[13px] text-gray-500">Define pricing options for different contract lengths</p>
        <button onClick={() => setShowAdd(!showAdd)} className="btn-primary text-[12px]">+ Add term option</button>
      </div>

      {showAdd && (
        <form onSubmit={handleAdd} className="card px-5 py-4 mb-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Duration (months) *</label>
              <input type="number" value={form.duration_months} onChange={(e) => updateWithCalc("duration_months", Number(e.target.value))} className="input-field" min="1" required />
            </div>
            <div>
              <label className="block text-[13px] font-medium text-gray-700 mb-1">Total fee ($) *</label>
              <input type="number" value={form.fee_total} onChange={(e) => updateWithCalc("fee_total", Number(e.target.value))} className="input-field" min="0" step="0.01" required />
            </div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Weekly ($) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_weekly} onChange={(e) => setForm({ ...form, fee_weekly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Fortnightly ($) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_fortnightly} onChange={(e) => setForm({ ...form, fee_fortnightly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Monthly ($) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_monthly} onChange={(e) => setForm({ ...form, fee_monthly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Quarterly ($) <span className="text-[10px] text-gray-400 font-normal">auto-calc</span></label><input type="number" value={form.fee_quarterly} onChange={(e) => setForm({ ...form, fee_quarterly: e.target.value })} className="input-field" step="0.01" placeholder="Auto" /></div>
            <div><label className="block text-[13px] font-medium text-gray-700 mb-1">Fixed # payments</label><input type="number" value={form.num_payments} onChange={(e) => setForm({ ...form, num_payments: e.target.value })} className="input-field" min="1" placeholder="Leave blank = continuous" /><p className="text-[10px] text-gray-400 mt-0.5">e.g. "3" = 3 payments then stops, membership stays active</p></div>
            <div className="flex items-end pb-1">
              <label className="flex items-center gap-2 text-[13px] text-gray-700"><input type="checkbox" checked={form.allow_upfront} onChange={(e) => setForm({ ...form, allow_upfront: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />Payment in advance</label>
            </div>
          </div>
          <div className="flex gap-2 justify-end pt-1">
            <button type="button" onClick={() => setShowAdd(false)} className="btn-secondary text-[12px]">Cancel</button>
            <button type="submit" disabled={saving} className="btn-primary text-[12px]">{saving ? "Adding..." : "Add"}</button>
          </div>
        </form>
      )}

      {(!mt.term_options || mt.term_options.length === 0) ? (
        <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No term options yet</div>
      ) : (
        <div className="space-y-3">
          {mt.term_options.map((to) => (
            <TermOptionCard key={to.id} termOption={to} mtId={mt.id} onReload={onReload} onDelete={() => handleDelete(to.id)} />
          ))}
        </div>
      )}
    </div>
  );
}

// === Editable Term Option Card ===
function TermOptionCard({ termOption: to, mtId, onReload, onDelete }: {
  termOption: TermOption; mtId: string; onReload: () => void; onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({
    duration_months: to.duration_months,
    fee_total: to.fee_total,
    fee_weekly: to.fee_weekly ?? "",
    fee_fortnightly: to.fee_fortnightly ?? "",
    fee_monthly: to.fee_monthly ?? "",
    fee_quarterly: to.fee_quarterly ?? "",
    allow_upfront: to.allow_upfront,
    num_payments: to.num_payments ?? "",
  });
  const [saving, setSaving] = useState(false);

  const updateEditCalc = (key: "duration_months" | "fee_total", value: number) => {
    const newForm = { ...form, [key]: value };
    const dur = key === "duration_months" ? value : form.duration_months;
    const tot = key === "fee_total" ? value : form.fee_total;
    const calc = autoCalcFees(dur, tot);
    setForm({ ...newForm, fee_weekly: calc.weekly, fee_fortnightly: calc.fortnightly, fee_monthly: calc.monthly, fee_quarterly: calc.quarterly });
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.updateTermOption(mtId, to.id, {
        ...form,
        fee_weekly: form.fee_weekly !== "" ? Number(form.fee_weekly) : null,
        fee_fortnightly: form.fee_fortnightly !== "" ? Number(form.fee_fortnightly) : null,
        fee_monthly: form.fee_monthly !== "" ? Number(form.fee_monthly) : null,
        fee_quarterly: form.fee_quarterly !== "" ? Number(form.fee_quarterly) : null,
        num_payments: form.num_payments !== "" ? Number(form.num_payments) : null,
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
            <span className="text-[13px] font-semibold text-gray-900 tabular-nums">${to.fee_total.toLocaleString()}</span>
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
            <div><span className="text-gray-400 block mb-0.5">Weekly</span><span className="tabular-nums text-gray-900">{to.fee_weekly ? `$${to.fee_weekly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Fortnightly</span><span className="tabular-nums text-gray-900">{to.fee_fortnightly ? `$${to.fee_fortnightly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Monthly</span><span className="tabular-nums text-gray-900">{to.fee_monthly ? `$${to.fee_monthly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">Quarterly</span><span className="tabular-nums text-gray-900">{to.fee_quarterly ? `$${to.fee_quarterly}` : "—"}</span></div>
            <div><span className="text-gray-400 block mb-0.5">In Advance</span><span>{to.allow_upfront ? "Yes" : "Post-payment"}</span></div>
            <div><span className="text-gray-400 block mb-0.5"># Payments</span><span className="tabular-nums text-gray-900">{to.num_payments ? `${to.num_payments} then stops` : "Continuous"}</span></div>
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
          <button onClick={handleSave} disabled={saving} className="btn-primary text-[12px]">{saving ? "Saving..." : "Save"}</button>
        </div>
      </div>
      <div className="px-5 py-4">
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Duration (months)</label>
            <input type="number" value={form.duration_months} onChange={(e) => updateEditCalc("duration_months", Number(e.target.value))} className="input-field" min="1" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Total fee ($)</label>
            <input type="number" value={form.fee_total} onChange={(e) => updateEditCalc("fee_total", Number(e.target.value))} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Weekly ($)</label>
            <input type="number" value={form.fee_weekly} onChange={(e) => setForm({ ...form, fee_weekly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Fortnightly ($)</label>
            <input type="number" value={form.fee_fortnightly} onChange={(e) => setForm({ ...form, fee_fortnightly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Monthly ($)</label>
            <input type="number" value={form.fee_monthly} onChange={(e) => setForm({ ...form, fee_monthly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Quarterly ($)</label>
            <input type="number" value={form.fee_quarterly} onChange={(e) => setForm({ ...form, fee_quarterly: e.target.value })} className="input-field" step="0.01" placeholder="—" />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1"># Payments</label>
            <input type="number" value={form.num_payments} onChange={(e) => setForm({ ...form, num_payments: e.target.value })} className="input-field" min="1" placeholder="Continuous" />
            <p className="text-[10px] text-gray-400 mt-0.5">Blank = pay until end of term</p>
          </div>
          <div className="flex items-end pb-1">
            <label className="flex items-center gap-2 text-[12px] text-gray-700">
              <input type="checkbox" checked={form.allow_upfront} onChange={(e) => setForm({ ...form, allow_upfront: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
              Payment in advance
            </label>
          </div>
        </div>
      </div>
    </div>
  );
}

// === Bonus Assignment Tab ===
function BonusAssignmentTab({ mt, allBonuses, onReload }: { mt: MembershipType; allBonuses: Bonus[]; onReload: () => void }) {
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
              <h3>{to.duration_months} months — ${to.fee_total.toLocaleString()}</h3>
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
                        <p className="text-[11px] text-gray-500">${bonus.value.toLocaleString()} value</p>
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
