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

  const loadClients = async (q = "") => {
    try {
      const endpoint = q ? `/tenant/clients?search=${encodeURIComponent(q)}` : "/tenant/clients";
      const data = (await apiRequest(endpoint)) as { clients: Client[] };
      setClients(data.clients || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadClients(); }, []);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    loadClients(search);
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Clients</h1>
          <p className="mt-1 text-sm text-gray-500">Manage your members</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="btn-primary">
          + Add Client
        </button>
      </div>

      {/* Search */}
      <form onSubmit={handleSearch} className="mb-6 flex gap-2 max-w-md">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name, email or phone..."
          className="input-field flex-1"
        />
        <button type="submit" className="btn-secondary">Search</button>
      </form>

      {showForm && <CreateClientForm onCreated={() => { setShowForm(false); loadClients(); }} />}

      {loading ? (
        <div className="card text-center text-gray-500">Loading...</div>
      ) : clients.length === 0 ? (
        <div className="card text-center text-gray-500">No clients found</div>
      ) : (
        <div className="card overflow-hidden p-0">
          {/* Desktop */}
          <div className="hidden md:block overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Name</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Email</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Phone</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Location</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {clients.map((c) => (
                  <tr key={c.id} className="hover:bg-gray-50 cursor-pointer">
                    <td className="px-4 py-3 font-medium text-gray-900">{c.name}</td>
                    <td className="px-4 py-3 text-gray-600">{c.email || "—"}</td>
                    <td className="px-4 py-3 text-gray-600">{c.phone || "—"}</td>
                    <td className="px-4 py-3 text-gray-600">
                      {[c.suburb, c.state].filter(Boolean).join(", ") || "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {/* Mobile */}
          <div className="md:hidden divide-y divide-gray-100">
            {clients.map((c) => (
              <div key={c.id} className="p-4">
                <p className="font-medium text-gray-900">{c.name}</p>
                {c.email && <p className="mt-1 text-sm text-gray-500">{c.email}</p>}
                {c.phone && <p className="text-sm text-gray-500">{c.phone}</p>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// Lightweight request helper (uses fetch directly to support query params)
async function apiRequest(endpoint: string) {
  const token = localStorage.getItem("access_token");
  const res = await fetch(`/api${endpoint}`, {
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
  });
  return res.json();
}

function CreateClientForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({ name: "", email: "", phone: "", address: "", suburb: "", state: "", postcode: "" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api.createClient(form);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally { setSaving(false); }
  };

  return (
    <div className="card mb-6 max-w-2xl">
      <h3 className="text-lg font-semibold text-gray-900 mb-4">Add Client</h3>
      {error && <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="input-field" required />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
            <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} className="input-field" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Phone</label>
            <input type="tel" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} className="input-field" />
          </div>
          <div className="sm:col-span-2">
            <label className="block text-sm font-medium text-gray-700 mb-1">Address</label>
            <input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} className="input-field" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Suburb</label>
            <input value={form.suburb} onChange={(e) => setForm({ ...form, suburb: e.target.value })} className="input-field" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">State</label>
            <input value={form.state} onChange={(e) => setForm({ ...form, state: e.target.value })} className="input-field" />
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
