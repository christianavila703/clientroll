import { useState, useEffect } from "react";
import api from "../../api/client";

interface Client {
  id: string;
  name: string;
  email?: string;
  phone?: string;
  suburb?: string;
  state?: string;
  created_at: string;
}

export default function ClientsPage() {
  const [clients, setClients] = useState<Client[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [tab, setTab] = useState("All");

  const loadClients = async (q = "") => {
    setLoading(true);
    try {
      const data = (await api.listClients(q)) as { clients: Client[] };
      setClients(data.clients || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadClients(); }, []);

  const handleSearch = () => { loadClients(search); };

  const tabs = ["All", "Active", "Inactive"];

  return (
    <div>
      {/* Page header - Ignition style */}
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-semibold text-gray-900">Clients</h1>
          <span className="text-sm text-gray-400">{clients.length} results</span>
        </div>
        <div className="flex items-center gap-2">
          <button className="rounded-lg border border-gray-200 bg-white px-3.5 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors flex items-center gap-1.5">
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>
            Export
          </button>
          <button
            onClick={() => setShowForm(!showForm)}
            className="rounded-lg bg-primary-600 px-3.5 py-2 text-sm font-semibold text-white hover:bg-primary-700 transition-colors flex items-center gap-1.5"
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" /></svg>
            New client
          </button>
        </div>
      </div>

      {/* Tabs + Search */}
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
                tab === t ? "bg-gray-900 text-white" : "text-gray-500 hover:text-gray-900 hover:bg-gray-100"
              }`}
            >
              {t}
            </button>
          ))}
        </div>
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
          <input
            type="text"
            value={search}
            onChange={(e) => { setSearch(e.target.value); if (!e.target.value) loadClients(); }}
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            placeholder="Search..."
            className="w-full rounded-lg border border-gray-200 bg-white pl-9 pr-3 py-2 text-sm placeholder:text-gray-400 focus:border-primary-300 focus:outline-none focus:ring-1 focus:ring-primary-300"
          />
        </div>
      </div>

      {showForm && <CreateClientForm onCreated={() => { setShowForm(false); loadClients(); }} />}

      {/* Table */}
      {loading ? (
        <TableSkeleton />
      ) : clients.length === 0 ? (
        <div className="rounded-xl border border-gray-200 bg-white py-16 text-center">
          <svg className="mx-auto h-12 w-12 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
          <p className="mt-3 text-sm text-gray-500">No clients found</p>
          <button onClick={() => setShowForm(true)} className="mt-4 text-sm font-medium text-primary-600 hover:text-primary-700">
            + Add your first client
          </button>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border border-gray-200 bg-white">
          {/* Desktop table */}
          <div className="hidden md:block overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-100 bg-gray-50/50">
                  <th className="w-10 px-4 py-3"><input type="checkbox" className="rounded border-gray-300" /></th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">Client</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">Contact</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">Location</th>
                  <th className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500">Added</th>
                  <th className="w-10 px-4 py-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {clients.map((c) => (
                  <tr key={c.id} className="hover:bg-gray-50/50 transition-colors cursor-pointer">
                    <td className="px-4 py-3"><input type="checkbox" className="rounded border-gray-300" /></td>
                    <td className="px-4 py-3">
                      <p className="font-medium text-gray-900">{c.name}</p>
                    </td>
                    <td className="px-4 py-3">
                      <p className="text-gray-600">{c.email || "—"}</p>
                      <p className="text-gray-400 text-xs">{c.phone || ""}</p>
                    </td>
                    <td className="px-4 py-3 text-gray-500">
                      {[c.suburb, c.state].filter(Boolean).join(", ") || "—"}
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-xs">
                      {new Date(c.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" })}
                    </td>
                    <td className="px-4 py-3">
                      <button className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100">
                        <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {/* Mobile cards */}
          <div className="md:hidden divide-y divide-gray-50">
            {clients.map((c) => (
              <div key={c.id} className="px-4 py-3">
                <div className="flex items-center justify-between">
                  <p className="font-medium text-gray-900 text-sm">{c.name}</p>
                  <button className="rounded p-1 text-gray-400 hover:text-gray-600"><svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg></button>
                </div>
                {c.email && <p className="text-xs text-gray-500 mt-0.5">{c.email}</p>}
                {c.phone && <p className="text-xs text-gray-400">{c.phone}</p>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function TableSkeleton() {
  return (
    <div className="overflow-hidden rounded-xl border border-gray-200 bg-white">
      {[...Array(5)].map((_, i) => (
        <div key={i} className="flex items-center gap-4 border-b border-gray-50 px-4 py-4">
          <div className="h-4 w-4 rounded bg-gray-100" />
          <div className="h-4 w-32 rounded bg-gray-100 animate-pulse" />
          <div className="h-4 w-40 rounded bg-gray-100 animate-pulse" />
          <div className="h-4 w-24 rounded bg-gray-100 animate-pulse" />
        </div>
      ))}
    </div>
  );
}

function CreateClientForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({ name: "", email: "", phone: "", address: "", suburb: "", state: "", postcode: "" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError("");
    try { await api.createClient(form); onCreated(); }
    catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  return (
    <div className="mb-6 rounded-xl border border-gray-200 bg-white p-6 max-w-2xl">
      <h3 className="text-base font-semibold text-gray-900 mb-4">Add Client</h3>
      {error && <div className="mb-4 rounded-lg bg-red-50 border border-red-100 px-4 py-2.5 text-sm text-red-600">{error}</div>}
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="sm:col-span-2"><label className="block text-sm font-medium text-gray-700 mb-1">Name *</label><input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="input-field" required /></div>
          <div><label className="block text-sm font-medium text-gray-700 mb-1">Email</label><input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} className="input-field" /></div>
          <div><label className="block text-sm font-medium text-gray-700 mb-1">Phone</label><input type="tel" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} className="input-field" /></div>
          <div className="sm:col-span-2"><label className="block text-sm font-medium text-gray-700 mb-1">Address</label><input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} className="input-field" /></div>
          <div><label className="block text-sm font-medium text-gray-700 mb-1">Suburb</label><input value={form.suburb} onChange={(e) => setForm({ ...form, suburb: e.target.value })} className="input-field" /></div>
          <div><label className="block text-sm font-medium text-gray-700 mb-1">State</label><input value={form.state} onChange={(e) => setForm({ ...form, state: e.target.value })} className="input-field" /></div>
        </div>
        <div className="flex gap-2 justify-end pt-2">
          <button type="button" onClick={onCreated} className="rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
          <button type="submit" disabled={saving} className="rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white hover:bg-primary-700 disabled:opacity-50">{saving ? "Creating..." : "Create"}</button>
        </div>
      </form>
    </div>
  );
}
