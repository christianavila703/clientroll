import { useState, useEffect } from "react";
import api from "../../api/client";

interface MembershipType {
  id: string;
  title: string;
  slug: string;
  description?: string;
  is_active: boolean;
  total_value: number;
  sort_order: number;
  term_options?: TermOption[];
}

interface TermOption {
  id: string;
  duration_months: number;
  fee_total: number;
  fee_monthly?: number;
}

export default function MembershipTypesPage() {
  const [types, setTypes] = useState<MembershipType[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);

  const loadTypes = async () => {
    try {
      const data = (await api.listMembershipTypes()) as { membership_types: MembershipType[] };
      setTypes(data.membership_types || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadTypes(); }, []);

  const handleDuplicate = async (id: string) => {
    try {
      await api.duplicateMembershipType(id);
      loadTypes();
    } catch { /* handle */ }
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Membership Types</h1>
          <p className="mt-1 text-sm text-gray-500">Define your membership offerings</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="btn-primary">
          + New Membership Type
        </button>
      </div>

      {showForm && <CreateMembershipForm onCreated={() => { setShowForm(false); loadTypes(); }} />}

      {loading ? (
        <div className="card text-center text-gray-500">Loading...</div>
      ) : types.length === 0 ? (
        <div className="card text-center text-gray-500 py-12">
          No membership types yet. Create your first one!
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {types.map((mt) => (
            <div key={mt.id} className="card">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-semibold text-gray-900">{mt.title}</h3>
                  <p className="mt-1 text-xs text-gray-400 font-mono">/{mt.slug}</p>
                </div>
                <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  mt.is_active ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
                }`}>
                  {mt.is_active ? "Active" : "Draft"}
                </span>
              </div>

              {mt.description && (
                <p className="mt-3 text-sm text-gray-600 line-clamp-2">{mt.description}</p>
              )}

              <div className="mt-4 flex items-center gap-4 text-sm text-gray-500">
                <span>Value: ${mt.total_value.toLocaleString()}</span>
                <span>{mt.term_options?.length || 0} term options</span>
              </div>

              <div className="mt-4 flex gap-2">
                <button className="btn-secondary text-xs flex-1">Edit</button>
                <button
                  onClick={() => handleDuplicate(mt.id)}
                  className="btn-secondary text-xs"
                  title="Duplicate"
                >
                  Copy
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CreateMembershipForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    title: "", description: "", inclusions: "", total_value: 0, sort_order: 0,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api.createMembershipType(form);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally { setSaving(false); }
  };

  return (
    <div className="card mb-6 max-w-2xl">
      <h3 className="text-lg font-semibold text-gray-900 mb-4">New Membership Type</h3>
      {error && <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Title *</label>
          <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} className="input-field" required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} className="input-field" rows={3} />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Inclusions</label>
          <textarea value={form.inclusions} onChange={(e) => setForm({ ...form, inclusions: e.target.value })} className="input-field" rows={3} placeholder="What's included in this membership..." />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Total Value ($)</label>
            <input type="number" value={form.total_value} onChange={(e) => setForm({ ...form, total_value: Number(e.target.value) })} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Sort Order</label>
            <input type="number" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: Number(e.target.value) })} className="input-field" min="0" />
          </div>
        </div>
        <div className="flex gap-3 justify-end">
          <button type="button" onClick={onCreated} className="btn-secondary">Cancel</button>
          <button type="submit" disabled={saving} className="btn-primary">{saving ? "Creating..." : "Create"}</button>
        </div>
      </form>
    </div>
  );
}
