import { useState, useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import api from "../../api/client";
import ImageUpload from "../common/ImageUpload";

interface Bonus {
  id: string;
  title: string;
  description?: string;
  image_url?: string;
  value: number;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export default function BonusesPage() {
  const [searchParams] = useSearchParams();
  const [bonuses, setBonuses] = useState<Bonus[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(searchParams.get("new") === "1");
  const [editing, setEditing] = useState<Bonus | null>(null);
  const [search, setSearch] = useState("");

  const loadBonuses = async () => {
    try {
      const data = (await api.listBonuses()) as { bonuses: Bonus[] };
      setBonuses(data.bonuses || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadBonuses(); }, []);

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this bonus?")) return;
    try { await api.deleteBonus(id); loadBonuses(); } catch { /* handle */ }
  };

  const filtered = bonuses.filter((b) =>
    !search || b.title.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div>
      {/* Header */}
      <div className="page-header">
        <h1 className="page-title">Bonuses</h1>
        <button onClick={() => { setEditing(null); setShowForm(!showForm); }} className="btn-primary">
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
          New bonus
        </button>
      </div>

      {showForm && (
        <BonusForm
          initial={editing}
          onSaved={() => { setShowForm(false); setEditing(null); loadBonuses(); }}
          onCancel={() => { setShowForm(false); setEditing(null); }}
        />
      )}

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none" />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {/* Table */}
      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <p className="text-[13px] text-gray-500">No bonuses found</p>
          {bonuses.length === 0 && (
            <button onClick={() => setShowForm(true)} className="mt-3 text-[13px] font-medium text-indigo-600 hover:text-indigo-700">
              + Create your first bonus
            </button>
          )}
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="data-table">
            <thead>
              <tr>
                <th>Bonus</th>
                <th>Value</th>
                <th>Status</th>
                <th>Last Updated</th>
                <th className="w-20"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((b) => (
                <tr key={b.id} className="cursor-pointer" onClick={() => { setEditing(b); setShowForm(true); }}>
                  <td>
                    <div className="flex items-center gap-3">
                      {b.image_url ? (
                        <img src={b.image_url} alt="" className="h-9 w-9 rounded object-cover border border-gray-200 flex-shrink-0" />
                      ) : (
                        <div className="flex h-9 w-9 items-center justify-center rounded bg-amber-50 border border-amber-100 flex-shrink-0">
                          <svg className="h-4 w-4 text-amber-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 8v13m0-13V6a2 2 0 112 2h-2zm0 0V5.5A2.5 2.5 0 109.5 8H12zm-7 4h14M5 12a2 2 0 110-4h14a2 2 0 110 4M5 12v7a2 2 0 002 2h10a2 2 0 002-2v-7" /></svg>
                        </div>
                      )}
                      <div>
                        <p className="font-medium text-gray-900">{b.title}</p>
                        {b.description && <p className="text-[11px] text-gray-400 mt-0.5 line-clamp-1">{b.description}</p>}
                      </div>
                    </div>
                  </td>
                  <td className="tabular-nums text-gray-900">${b.value.toLocaleString()}</td>
                  <td>
                    <span className={`badge ${b.is_active ? "badge-green" : "badge-gray"}`}>
                      {b.is_active ? "Active" : "Inactive"}
                    </span>
                  </td>
                  <td className="text-gray-500">
                    {new Date(b.updated_at || b.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" })}
                  </td>
                  <td>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDelete(b.id); }}
                      className="rounded p-1 text-gray-400 hover:text-red-500 hover:bg-red-50"
                      title="Delete"
                    >
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
                    </button>
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

function BonusForm({ initial, onSaved, onCancel }: {
  initial: Bonus | null;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const [form, setForm] = useState({
    title: initial?.title || "",
    description: initial?.description || "",
    image_url: initial?.image_url || "",
    value: initial?.value || 0,
    is_active: initial?.is_active ?? true,
    sort_order: initial?.sort_order || 0,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.title) { setError("Title is required"); return; }
    setSaving(true); setError("");
    try {
      if (initial) {
        await api.updateBonus(initial.id, form);
      } else {
        await api.createBonus(form);
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
    } finally { setSaving(false); }
  };

  return (
    <div className="card mb-5 overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">
          {initial ? `Edit: ${initial.title}` : "New Bonus"}
        </h3>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCancel} className="btn-secondary text-[12px]">Cancel</button>
          <button type="button" onClick={() => { const f = document.getElementById("bonus-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary text-[12px]">
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
      {error && <div className="mx-5 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
      <form id="bonus-form" onSubmit={handleSubmit} className="px-5 py-4 space-y-4">
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Title *</label>
          <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} className="input-field" required placeholder="e.g. Free Personal Training Session" />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
          <p className="text-[11px] text-gray-400 mb-1.5">Shown to clients during signup when this bonus is included</p>
          <textarea
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={3}
            placeholder="Describe what the client receives..."
          />
        </div>
        <ImageUpload
          value={form.image_url}
          entityType="bonuses"
          onChange={(url) => setForm({ ...form, image_url: url })}
          label="Image"
          hint="Displayed as a visual card during signup"
        />
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Value ($)</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Dollar value shown to clients</p>
            <input type="number" value={form.value} onChange={(e) => setForm({ ...form, value: Number(e.target.value) })} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Sort Order</label>
            <p className="text-[11px] text-gray-400 mb-1.5">Lower numbers first</p>
            <input type="number" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: Number(e.target.value) })} className="input-field" min="0" />
          </div>
        </div>
        {initial && (
          <div className="flex items-center gap-2">
            <input type="checkbox" id="bonus-active" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500" />
            <label htmlFor="bonus-active" className="text-[13px] text-gray-700">Active</label>
          </div>
        )}
      </form>
    </div>
  );
}
