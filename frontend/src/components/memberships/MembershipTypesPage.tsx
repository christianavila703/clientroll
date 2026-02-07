import { useState, useEffect } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import api from "../../api/client";
import ImageUpload from "../common/ImageUpload";

interface MembershipType {
  id: string;
  title: string;
  slug: string;
  description?: string;
  is_active: boolean;
  total_value: number;
  sort_order: number;
  term_options?: TermOption[];
  created_at: string;
  updated_at: string;
}

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

  const filtered = types.filter((mt) => {
    if (tab === "active" && !mt.is_active) return false;
    if (tab === "draft" && mt.is_active) return false;
    if (search && !mt.title.toLowerCase().includes(search.toLowerCase())) return false;
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
        <div className="card overflow-hidden">
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
                    <p className="font-medium text-gray-900">{mt.title}</p>
                    <p className="text-[11px] text-gray-400 font-mono mt-0.5">/{mt.slug}</p>
                  </td>
                  <td>
                    <span className={`badge ${mt.is_active ? "badge-green" : "badge-gray"}`}>
                      {mt.is_active ? "Active" : "Draft"}
                    </span>
                  </td>
                  <td className="tabular-nums">${mt.total_value.toLocaleString()}</td>
                  <td className="text-gray-500">{mt.term_options?.length || 0} options</td>
                  <td className="text-gray-500">
                    {new Date(mt.updated_at || mt.created_at).toLocaleDateString("en-AU", {
                      day: "numeric", month: "long", year: "numeric"
                    })}
                  </td>
                  <td>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleDuplicate(mt.id)}
                        className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100"
                        title="Duplicate"
                      >
                        <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75" /></svg>
                      </button>
                      <button className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100" title="More">
                        <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
                      </button>
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

function CreateMembershipForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    title: "", description: "", inclusions: "", total_value: 0, sort_order: 0, top_image_url: "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError("");
    try { await api.createMembershipType(form); onCreated(); }
    catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  return (
    <div className="card mb-5 max-w-3xl overflow-hidden">
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
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Title *</label>
          <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} className="input-field" required />
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
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Total Value ($)</label>
            <input type="number" value={form.total_value} onChange={(e) => setForm({ ...form, total_value: Number(e.target.value) })} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Sort Order</label>
            <input type="number" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: Number(e.target.value) })} className="input-field" min="0" />
          </div>
        </div>
      </form>
    </div>
  );
}
