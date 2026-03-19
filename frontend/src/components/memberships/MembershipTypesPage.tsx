import { useState, useEffect, useRef } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import api from "../../api/client";
import ImageUpload from "../common/ImageUpload";

interface MembershipType {
  id: string;
  membership_name?: string;
  membership_level?: string;
  title: string;
  slug: string;
  description?: string;
  currency_code?: string;
  is_active: boolean;
  is_most_popular?: boolean;
  sort_order: number;
  term_options?: TermOption[];
  created_at: string;
  updated_at: string;
}

const membershipDisplayTitle = (mt: Pick<MembershipType, "membership_name" | "membership_level" | "title">) => {
  const name = (mt.membership_name || "").trim();
  const level = (mt.membership_level || "").trim();
  if (name && level) return `${name} - ${level}`;
  if (name) return name;
  return mt.title;
};

interface TermOption {
  id: string;
  duration_months: number;
  fee_total: number;
  fee_monthly?: number;
}

export default function MembershipTypesPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [types, setTypes] = useState<MembershipType[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(searchParams.get("new") === "1");
  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<"all" | "active" | "draft">("all");

  const loadTypes = async () => {
    try {
      const data = (await api.listMembershipTypes()) as { membership_types: MembershipType[] };
      setTypes(data.membership_types || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadTypes(); }, []);

  const handleDuplicate = async (id: string) => {
    try { await api.duplicateMembershipType(id); loadTypes(); } catch { /* handle */ }
  };

  const handleArchive = async (id: string) => {
    try {
      const token = localStorage.getItem("access_token");
      await fetch(`/api/tenant/membership-types/${id}/archive`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      loadTypes();
    } catch { /* handle */ }
  };

  const handleActivate = async (id: string) => {
    try {
      const token = localStorage.getItem("access_token");
      await fetch(`/api/tenant/membership-types/${id}/activate`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      loadTypes();
    } catch { /* handle */ }
  };

  const handleDelete = async (id: string, title: string) => {
    if (!confirm(`Remove "${title}" from the list?\n\nThis will hide it from your membership types. Existing clients with this membership are not affected. You can create a new type with the same name later.`)) return;
    try {
      const token = localStorage.getItem("access_token");
      const res = await fetch(`/api/tenant/membership-types/${id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        const data = await res.json();
        alert(data.error || "Failed to delete");
        return;
      }
      loadTypes();
    } catch { /* handle */ }
  };

  const filtered = types.filter((mt) => {
    if (tab === "active" && !mt.is_active) return false;
    if (tab === "draft" && mt.is_active) return false;
    const haystack = `${membershipDisplayTitle(mt)} ${mt.membership_name || ""} ${mt.membership_level || ""}`.toLowerCase();
    if (search && !haystack.includes(search.toLowerCase())) return false;
    return true;
  });

  const tabs = [
    { key: "all" as const, label: "All" },
    { key: "active" as const, label: "Active" },
    { key: "draft" as const, label: "Draft" },
  ];

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Membership Types</h1>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="btn-primary">
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
          New membership type
        </button>
      </div>

      {showForm && <CreateMembershipForm onCreated={() => { setShowForm(false); loadTypes(); }} />}

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map((t) => (
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

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none"
          />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {/* Table */}
      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <p className="text-[13px] text-gray-500">No membership types found</p>
          {types.length === 0 && (
            <button onClick={() => setShowForm(true)} className="mt-3 text-[13px] font-medium text-indigo-600 hover:text-indigo-700">
              + Create your first membership type
            </button>
          )}
        </div>
      ) : (
        <div className="card">
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Value</th>
                <th>Terms</th>
                <th>Last Updated</th>
                <th className="w-20"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((mt) => (
                <tr key={mt.id} className="cursor-pointer" onClick={() => navigate(`/memberships/${mt.id}`)}>
                  <td>
                    <p className="font-medium text-gray-900">{(mt.membership_name || "").trim() || mt.title}</p>
                    {(mt.membership_level || "").trim() && (
                      <p className="text-[12px] font-medium text-indigo-600 mt-0.5">{mt.membership_level}</p>
                    )}
                    <p className="text-[11px] text-gray-400 font-mono mt-0.5">/{mt.slug}</p>
                  </td>
                  <td>
                    <div className="flex items-center gap-1">
                      <span className={`badge ${mt.is_active ? "badge-green" : "badge-gray"}`}>
                        {mt.is_active ? "Active" : "Draft"}
                      </span>
                      {mt.is_most_popular && <span className="badge badge-purple">Most Popular</span>}
                    </div>
                  </td>
                  <td className="tabular-nums">{mt.currency_code || "AUD"} ${(() => {
                    const opts = mt.term_options || [];
                    if (opts.length === 0) return "0";
                    if (opts.length === 1) return opts[0].fee_total.toLocaleString();
                    const fees = opts.map(o => o.fee_total).sort((a, b) => a - b);
                    return fees[0] === fees[fees.length - 1]
                      ? fees[0].toLocaleString()
                      : `${fees[0].toLocaleString()} – ${fees[fees.length - 1].toLocaleString()}`;
                  })()}</td>
                  <td className="text-gray-500">{mt.term_options?.length || 0} options</td>
                  <td className="text-gray-500">
                    {new Date(mt.updated_at || mt.created_at).toLocaleDateString("en-AU", {
                      day: "numeric", month: "long", year: "numeric"
                    })}
                  </td>
                  <td onClick={e => e.stopPropagation()}>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleDuplicate(mt.id)}
                        className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100"
                        title="Duplicate"
                      >
                        <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75" /></svg>
                      </button>
                      <MtRowMenu
                        mt={mt}
                        onArchive={() => handleArchive(mt.id)}
                        onActivate={() => handleActivate(mt.id)}
                        onDelete={() => handleDelete(mt.id, membershipDisplayTitle(mt))}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function MtRowMenu({ mt, onArchive, onActivate, onDelete }: {
  mt: MembershipType; onArchive: () => void; onActivate: () => void; onDelete: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [openUp, setOpenUp] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  useEffect(() => {
    if (!open) return;

    const updatePlacement = () => {
      const trigger = ref.current?.getBoundingClientRect();
      if (!trigger) return;
      const menuHeight = menuRef.current?.offsetHeight || 180;
      const spaceBelow = window.innerHeight - trigger.bottom;
      const spaceAbove = trigger.top;
      setOpenUp(spaceBelow < menuHeight && spaceAbove > spaceBelow);
    };

    updatePlacement();
    window.addEventListener("resize", updatePlacement);
    window.addEventListener("scroll", updatePlacement, true);
    return () => {
      window.removeEventListener("resize", updatePlacement);
      window.removeEventListener("scroll", updatePlacement, true);
    };
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <button onClick={() => setOpen(!open)} className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100" title="More">
        <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
      </button>
      {open && (
        <div
          ref={menuRef}
          className={`absolute right-0 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50 ${openUp ? "bottom-full mb-1" : "top-full mt-1"}`}
        >
          {mt.is_active ? (
            <button onClick={() => { setOpen(false); onArchive(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-gray-700 hover:bg-gray-50">
              <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
              Archive (set inactive)
            </button>
          ) : (
            <button onClick={() => { setOpen(false); onActivate(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-emerald-700 hover:bg-emerald-50">
              <svg className="h-3.5 w-3.5 text-emerald-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              Activate
            </button>
          )}
          <div className="my-1 border-t border-gray-100" />
          <button onClick={() => { setOpen(false); onDelete(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-red-600 hover:bg-red-50">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
            Remove from list
          </button>
        </div>
      )}
    </div>
  );
}

function CreateMembershipForm({ onCreated }: { onCreated: () => void }) {
  type NumericField = number | "";
  const toNumberOrZero = (value: NumericField) => (value === "" ? 0 : value);

  const [enabledCurrencies, setEnabledCurrencies] = useState<string[]>(["AUD"]);
  const [form, setForm] = useState({
    membership_name: "", membership_level: "", description: "", inclusions: "", sort_order: "", top_image_url: "", is_most_popular: false, currency_code: "AUD",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.getTenantSettings()
      .then((res) => {
        const list = (res as { enabled_currencies?: string[] }).enabled_currencies || ["AUD"];
        const normalized = Array.from(new Set(list.map((c) => String(c).toUpperCase()))).filter(Boolean);
        if (!normalized.includes("AUD")) normalized.unshift("AUD");
        setEnabledCurrencies(normalized);
      })
      .catch(() => setEnabledCurrencies(["AUD"]));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const membershipName = form.membership_name.trim();
    const membershipLevel = form.membership_level.trim();
    if (!membershipName) {
      setError("Membership name is required");
      return;
    }
    setSaving(true); setError("");
    try {
      await api.createMembershipType({
        title: membershipLevel ? `${membershipName} - ${membershipLevel}` : membershipName,
        membership_name: membershipName,
        membership_level: membershipLevel || null,
        description: form.description || "",
        inclusions: form.inclusions || "",
        sort_order: toNumberOrZero(form.sort_order),
        top_image_url: form.top_image_url || "",
        is_most_popular: form.is_most_popular,
        currency_code: form.currency_code || "AUD",
      });
      onCreated();
    }
    catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  return (
    <div className="card mb-5 overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">New Membership Type</h3>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCreated} className="btn-secondary text-[12px]">Cancel</button>
          <button type="button" onClick={() => { const f = document.getElementById("create-mt-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary text-[12px]">
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
      {error && <div className="mx-5 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
      <form id="create-mt-form" onSubmit={handleSubmit} className="px-5 py-4 space-y-4">
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Membership Name *</label>
          <input value={form.membership_name} onChange={(e) => setForm({ ...form, membership_name: e.target.value })} className="input-field" required />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Level</label>
          <input value={form.membership_level} onChange={(e) => setForm({ ...form, membership_level: e.target.value })} className="input-field" placeholder="Support, Premium, Elite..." />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
          <p className="text-[11px] text-gray-400 mb-1.5">Marketing copy for the public signup page</p>
          <textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={5} placeholder="Describe the benefits and value..." />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Inclusions</label>
          <p className="text-[11px] text-gray-400 mb-1.5">Shown to clients during signup</p>
          <textarea value={form.inclusions} onChange={(e) => setForm({ ...form, inclusions: e.target.value })}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={5} placeholder={"• Unlimited access to all classes\n• Personal training session monthly\n• Nutrition consultation"} />
        </div>
        <ImageUpload
          value={form.top_image_url}
          entityType="memberships"
          onChange={(url) => setForm({ ...form, top_image_url: url })}
          label="Hero Image"
          hint="Banner displayed at the top of the public signup page"
        />
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Sort Order</label>
            <input type="number" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: e.target.value === "" ? "" : Number(e.target.value) })} className="input-field" min="0" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Currency</label>
            <select
              value={form.currency_code}
              onChange={(e) => setForm({ ...form, currency_code: e.target.value })}
              className="input-field"
            >
              {enabledCurrencies.map((code) => <option key={code} value={code}>{code}</option>)}
            </select>
          </div>
        </div>
        <label className="flex items-center gap-2 text-[13px] text-gray-700">
          <input
            type="checkbox"
            checked={form.is_most_popular}
            onChange={(e) => setForm({ ...form, is_most_popular: e.target.checked })}
            className="h-4 w-4 rounded border-gray-300 text-indigo-600"
          />
          Mark as Most Popular
        </label>
      </form>
    </div>
  );
}
