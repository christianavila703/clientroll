import { useState, useEffect } from "react";
import api, { ApiError } from "../../api/client";

interface Service {
  id: string;
  name: string;
  description?: string;
  price: number;
  currency_code?: string;
  tax_rate: number;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

const AVAILABLE_CURRENCIES = ["AUD", "USD", "EUR", "GBP", "NZD", "CAD", "SGD"];
type NumericField = number | "";
type ServiceFormState = {
  name: string;
  description: string;
  price: NumericField;
  currency_code: string;
  tax_rate: NumericField;
  is_active: boolean;
  sort_order: NumericField;
};

export default function ServicesPage() {
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Service | null>(null);
  const [search, setSearch] = useState("");

  const load = async () => {
    try {
      setLoadError("");
      const data = await api.listServices() as { services?: Service[] };
      setServices(data.services || []);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Unable to load services.");
    } finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete "${name}"?`)) return;
    try {
      await api.deleteService(id);
      await load();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Unable to delete service. Please try again.");
    }
  };

  const filtered = services.filter(s =>
    !search || s.name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Services</h1>
        <button onClick={() => { setEditing(null); setShowForm(true); }} className="btn-primary">
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
          New service
        </button>
      </div>

      {showForm && (
        <ServiceForm
          initial={editing}
          onSaved={() => { setShowForm(false); setEditing(null); load(); }}
          onCancel={() => { setShowForm(false); setEditing(null); }}
        />
      )}

      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none" />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : loadError ? (
        <div className="card px-5 py-12 text-center text-[13px] text-red-600">{loadError}</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <p className="text-[13px] text-gray-500">No services found</p>
          {services.length === 0 && (
            <button onClick={() => setShowForm(true)} className="mt-3 text-[13px] font-medium text-indigo-600 hover:text-indigo-700">Create your first service</button>
          )}
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="data-table">
            <thead>
              <tr>
                <th>Service</th>
                <th>Price</th>
                <th>Tax</th>
                <th>Status</th>
                <th className="w-20"></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(s => (
                <tr key={s.id} className="cursor-pointer" onClick={() => {
                  if (showForm && editing?.id === s.id) { setShowForm(false); setEditing(null); }
                  else { setEditing(s); setShowForm(true); }
                }}>
                  <td>
                    <p className="font-medium text-gray-900">{s.name}</p>
                    {s.description && <p className="text-[11px] text-gray-400 mt-0.5 line-clamp-1">{s.description}</p>}
                  </td>
                  <td className="tabular-nums text-gray-900">{`${s.currency_code || "AUD"} ${s.price.toFixed(2)}`}</td>
                  <td className="text-gray-500">{s.tax_rate > 0 ? `${s.tax_rate}%` : "—"}</td>
                  <td>
                    <span className={`badge ${s.is_active ? "badge-green" : "badge-gray"}`}>
                      {s.is_active ? "Active" : "Inactive"}
                    </span>
                  </td>
                  <td onClick={e => e.stopPropagation()}>
                    <button onClick={() => handleDelete(s.id, s.name)} className="rounded p-1 text-gray-400 hover:text-red-500 hover:bg-red-50" title="Delete">
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

function ServiceForm({ initial, onSaved, onCancel }: { initial: Service | null; onSaved: () => void; onCancel: () => void }) {
  const toNumberOrZero = (value: NumericField) => (value === "" ? 0 : value);
  const [enabledCurrencies, setEnabledCurrencies] = useState<string[]>(["AUD"]);

  const [form, setForm] = useState<ServiceFormState>({
    name: initial?.name || "",
    description: initial?.description || "",
    price: initial ? initial.price : "",
    currency_code: initial?.currency_code || "AUD",
    tax_rate: initial ? initial.tax_rate : "",
    is_active: initial?.is_active ?? true,
    sort_order: initial ? initial.sort_order : "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.getTenantSettings().then((res) => {
      const t = res as Record<string, unknown>;
      const fromTenant = Array.isArray(t.enabled_currencies)
        ? Array.from(new Set((t.enabled_currencies as unknown[]).map((c) => String(c).toUpperCase()))).filter(Boolean)
        : [];
      const allowed = fromTenant.filter((code) => AVAILABLE_CURRENCIES.includes(code));
      const next = allowed.length ? allowed : ["AUD"];
      if (!next.includes("AUD")) next.unshift("AUD");
      setEnabledCurrencies(Array.from(new Set(next)));
      setForm((prev) => {
        const current = (prev.currency_code || "AUD").toUpperCase();
        return next.includes(current) ? prev : { ...prev, currency_code: "AUD" };
      });
    }).catch(() => {
      setEnabledCurrencies(["AUD"]);
    });
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name) { setError("Name is required"); return; }
    setSaving(true); setError("");
    try {
      if (initial) {
        const payload = {
          ...form,
          description: form.description || null,
          price: toNumberOrZero(form.price),
          currency_code: (form.currency_code || "AUD").toUpperCase(),
          tax_rate: toNumberOrZero(form.tax_rate),
          sort_order: toNumberOrZero(form.sort_order),
        };
        await api.updateService(initial.id, payload);
      } else {
        // Backend create endpoint rejects unknown fields (DisallowUnknownFields).
        // Only send fields accepted by POST /tenant/services.
        const payload = {
          name: form.name,
          description: form.description || null,
          price: toNumberOrZero(form.price),
          currency_code: (form.currency_code || "AUD").toUpperCase(),
          tax_rate: toNumberOrZero(form.tax_rate),
          sort_order: toNumberOrZero(form.sort_order),
        };
        await api.createService(payload);
      }
      onSaved();
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || "Unable to save service.");
      } else {
        setError(err instanceof Error ? err.message : "Failed");
      }
    }
    finally { setSaving(false); }
  };

  return (
    <div className="card mb-5 overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">{initial ? `Edit: ${initial.name}` : "New Service"}</h3>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCancel} className="btn-secondary text-[12px]">Cancel</button>
          <button type="button" onClick={() => { const f = document.getElementById("svc-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary">
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>
      {error && <div className="mx-5 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
      <form id="svc-form" onSubmit={handleSubmit} className="px-5 py-4 space-y-4">
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Name *</label>
          <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} className="input-field" required placeholder="e.g. Personal Training Session" />
        </div>
        <div>
          <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
          <textarea value={form.description} onChange={e => setForm({ ...form, description: e.target.value })}
            className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
            rows={2} placeholder="Brief description of the service" />
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Price</label>
            <input type="number" value={form.price} onChange={e => setForm({ ...form, price: e.target.value === "" ? "" : Number(e.target.value) })} className="input-field" min="0" step="0.01" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Currency</label>
            <select
              value={form.currency_code || "AUD"}
              onChange={e => setForm({ ...form, currency_code: e.target.value })}
              className="input-field"
            >
              {enabledCurrencies.map((code) => (
                <option key={code} value={code}>{code}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Tax Rate (%)</label>
            <input type="number" value={form.tax_rate} onChange={e => setForm({ ...form, tax_rate: e.target.value === "" ? "" : Number(e.target.value) })} className="input-field" min="0" step="0.1" placeholder="e.g. 10" />
          </div>
          <div>
            <label className="block text-[13px] font-medium text-gray-700 mb-1">Sort Order</label>
            <input type="number" value={form.sort_order} onChange={e => setForm({ ...form, sort_order: e.target.value === "" ? "" : Number(e.target.value) })} className="input-field" min="0" />
          </div>
          {initial && (
            <div className="flex items-end pb-1">
              <label className="flex items-center gap-2 text-[13px] text-gray-700">
                <input type="checkbox" checked={form.is_active} onChange={e => setForm({ ...form, is_active: e.target.checked })} className="h-4 w-4 rounded border-gray-300 text-indigo-600" />
                Active
              </label>
            </div>
          )}
        </div>
      </form>
    </div>
  );
}
