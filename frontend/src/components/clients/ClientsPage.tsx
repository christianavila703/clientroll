import { useState, useEffect, useRef } from "react";
import { useSearchParams, useNavigate, Link } from "react-router-dom";
import api from "../../api/client";

interface Client {
  id: string;
  name: string;
  email?: string;
  phone?: string;
  address?: string;
  suburb?: string;
  state?: string;
  postcode?: string;
  created_at: string;
}

const CLIENT_FIELDS = [
  { key: "name", label: "Full Name", required: true },
  { key: "email", label: "Email" },
  { key: "phone", label: "Phone" },
  { key: "address", label: "Address" },
  { key: "suburb", label: "Suburb" },
  { key: "state", label: "State" },
  { key: "postcode", label: "Postcode" },
  { key: "birth_date", label: "Date of Birth" },
  { key: "gender", label: "Gender" },
  { key: "emergency_contact_name", label: "Emergency Contact Name" },
  { key: "emergency_contact_phone", label: "Emergency Contact Phone" },
];

export default function ClientsPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [clients, setClients] = useState<Client[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(searchParams.get("new") === "1");
  const [editingClient, setEditingClient] = useState<Client | null>(null);
  const [statusTab, setStatusTab] = useState("All");
  const [showImport, setShowImport] = useState(false);

  const loadClients = async (q = "", status = statusTab) => {
    setLoading(true);
    try {
      const data = (await api.listClients(q, status)) as { clients: Client[] };
      setClients(data.clients || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  useEffect(() => { loadClients(search, statusTab); }, [statusTab]);

  const handleSearch = () => { loadClients(search, statusTab); };

  const statusTabs = ["All", "Active", "Inactive"];

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Clients</h1>
        <div className="flex items-center gap-2">
          <button onClick={() => exportCSV(clients)} className="btn-secondary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" /></svg>
            Export
          </button>
          <button onClick={() => setShowImport(true)} className="btn-secondary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12M12 16.5V3" /></svg>
            Import
          </button>
          <button
            onClick={() => { setEditingClient(null); setShowForm(true); }}
            className="btn-primary text-[12px]"
          >
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
            New client
          </button>
        </div>
      </div>

      {/* Status tabs */}
      <div className="mb-4">
        <div className="flex items-center gap-1 border-b border-gray-200">
          {statusTabs.map((t) => (
            <button key={t} onClick={() => setStatusTab(t)}
              className={`px-3 py-2 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
                statusTab === t ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
              }`}>{t}</button>
          ))}
        </div>
      </div>

      {/* Search + filters + count */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between mb-4">
        <div className="flex items-center gap-2">
          <div className="relative w-56">
            <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
            <input type="text" value={search}
              onChange={(e) => { setSearch(e.target.value); if (!e.target.value) loadClients(); }}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
              placeholder="Search..."
              className="w-full rounded-md border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:outline-none" />
          </div>
          <button className="btn-ghost text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M10.5 6h9.75M10.5 6a1.5 1.5 0 11-3 0m3 0a1.5 1.5 0 10-3 0M3.75 6H7.5m3 12h9.75m-9.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-3.75 0H7.5m9-6h3.75m-3.75 0a1.5 1.5 0 01-3 0m3 0a1.5 1.5 0 00-3 0m-9.75 0h9.75" /></svg>
            Filters
          </button>
          <span className="text-[13px] text-gray-400">{clients.length} results</span>
        </div>
      </div>

      {showForm && (
        <CreateClientForm editing={editingClient} onCreated={() => { setShowForm(false); setEditingClient(null); loadClients(); }} />
      )}

      {showImport && (
        <ImportModal onClose={() => setShowImport(false)} onImported={() => { setShowImport(false); loadClients(); }} />
      )}

      {/* Table */}
      {loading ? (
        <TableSkeleton />
      ) : clients.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <svg className="mx-auto h-10 w-10 text-gray-300" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
          <p className="mt-3 text-[13px] text-gray-500">No clients found</p>
          <button onClick={() => setShowForm(true)} className="mt-2 text-[13px] font-medium text-indigo-600 hover:text-indigo-700">+ Add your first client</button>
        </div>
      ) : (
        <div className="card overflow-hidden">
          {/* Desktop */}
          <div className="hidden lg:block overflow-x-auto">
            <table className="data-table">
              <thead>
                <tr>
                  <th className="w-10"><input type="checkbox" className="rounded border-gray-300" /></th>
                  <th>Client ▲</th>
                  <th>Status</th>
                  <th>Primary Contact</th>
                  <th>Location</th>
                  <th>Activity</th>
                  <th className="w-10"></th>
                </tr>
              </thead>
              <tbody>
                {clients.map((c) => (
                  <tr key={c.id} className="cursor-pointer hover:bg-gray-50" onClick={() => navigate(`/clients/${c.id}`)}>
                    <td onClick={(e) => e.stopPropagation()}><input type="checkbox" className="rounded border-gray-300" /></td>
                    <td className="font-medium text-gray-900">{c.name}</td>
                    <td><span className="badge badge-green">Active</span></td>
                    <td>
                      <p className="text-gray-900 text-[13px]">{c.name}</p>
                      <p className="text-gray-400 text-[12px]">{c.email || ""}</p>
                    </td>
                    <td className="text-gray-500 text-[13px]">
                      {[c.suburb, c.state].filter(Boolean).join(", ").toUpperCase() || "—"}
                    </td>
                    <td>
                      <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Added</p>
                      <p className="text-[12px] text-gray-700">
                        {new Date(c.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" })}
                      </p>
                    </td>
                    <td onClick={(e) => e.stopPropagation()}>
                      <ClientRowMenu clientId={c.id} clientName={c.name} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Mobile */}
          <div className="lg:hidden divide-y divide-gray-100">
            {clients.map((c) => (
              <div key={c.id} className="px-4 py-3 cursor-pointer hover:bg-gray-50" onClick={() => navigate(`/clients/${c.id}`)}>
                <div className="flex items-start justify-between">
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{c.name}</p>
                    <p className="text-[12px] text-gray-500">{c.email || ""}</p>
                    {c.phone && <p className="text-[11px] text-gray-400">{c.phone}</p>}
                  </div>
                  <span className="badge badge-green">Active</span>
                </div>
                <p className="mt-1 text-[11px] text-gray-400">
                  Added {new Date(c.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" })}
                </p>
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
    <div className="card overflow-hidden">
      {[...Array(6)].map((_, i) => (
        <div key={i} className="flex items-center gap-4 border-b border-gray-50 px-4 py-3.5">
          <div className="h-4 w-4 rounded bg-gray-100" />
          <div className="h-3.5 w-36 rounded bg-gray-100 animate-pulse" />
          <div className="h-3.5 w-16 rounded bg-gray-100 animate-pulse" />
          <div className="h-3.5 w-44 rounded bg-gray-100 animate-pulse" />
          <div className="h-3.5 w-24 rounded bg-gray-100 animate-pulse" />
          <div className="h-3.5 w-20 rounded bg-gray-100 animate-pulse" />
        </div>
      ))}
    </div>
  );
}

function ClientRowMenu({ clientId, clientName }: { clientId: string; clientName: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const handler = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button onClick={(e) => { e.stopPropagation(); setOpen(!open); }} className="rounded p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100">
        <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 w-40 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50">
          <button onClick={(e) => { e.stopPropagation(); setOpen(false); navigate(`/clients/${clientId}`); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-gray-700 hover:bg-gray-50">
            <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" /><path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
            View profile
          </button>
          <div className="my-1 border-t border-gray-100" />
          <button onClick={(e) => { e.stopPropagation(); setOpen(false); if (confirm(`Archive ${clientName}?`)) { /* archive */ } }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-gray-700 hover:bg-gray-50">
            <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
            Archive
          </button>
          <button onClick={(e) => { e.stopPropagation(); setOpen(false); if (confirm(`Delete ${clientName}?`)) { /* delete */ } }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-red-600 hover:bg-red-50">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
            Delete
          </button>
        </div>
      )}
    </div>
  );
}

function CreateClientForm({ editing, onCreated }: { editing: Client | null; onCreated: () => void }) {
  const [tab, setTab] = useState<"details" | "company">("details");
  const [form, setForm] = useState({
    name: editing?.name || "", email: editing?.email || "", phone: editing?.phone || "",
    address: editing?.address || "", suburb: editing?.suburb || "", state: editing?.state || "",
    postcode: editing?.postcode || "", birth_date: "", gender: "",
    emergency_contact_name: "", emergency_contact_phone: "",
    business_structure: "", gst_registered: "", tax_number: "", company_number: "",
    company_phone: "", company_website: "",
    company_address: "", company_suburb: "", company_state: "", company_postcode: "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError("");
    try {
      if (editing) { await api.updateClient(editing.id, form); }
      else { await api.createClient(form); }
      onCreated();
    } catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  const F = ({ label, value, onChange, type = "text", placeholder = "", required = false, colSpan = false }: {
    label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string; required?: boolean; colSpan?: boolean;
  }) => (
    <div className={colSpan ? "sm:col-span-2" : ""}>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">{label}{required && <span className="text-red-500"> *</span>}</label>
      <input type={type} value={value} onChange={(e) => onChange(e.target.value)} className="input-field" placeholder={placeholder} required={required} />
    </div>
  );

  return (
    <div className="mb-5 card overflow-hidden">
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
        <h3 className="text-sm font-semibold text-gray-900">{editing ? `Edit: ${editing.name}` : "New client"}</h3>
        <div className="flex items-center gap-2">
          <button type="button" onClick={onCreated} className="btn-secondary text-[12px]">Cancel</button>
          <button type="button" onClick={() => { const f = document.getElementById("client-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      <div className="flex border-b border-gray-200 px-5">
        {(["details", "company"] as const).map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t === "details" ? "Details" : "Company"}</button>
        ))}
      </div>

      {error && <div className="mx-5 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}

      <form id="client-form" onSubmit={handleSubmit} className="px-5 py-4">
        {tab === "details" && (
          <div className="space-y-5">
            <div>
              <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Personal information</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <F label="Full name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} required colSpan />
                <F label="Email" value={form.email} onChange={(v) => setForm({ ...form, email: v })} type="email" />
                <F label="Phone" value={form.phone} onChange={(v) => setForm({ ...form, phone: v })} type="tel" />
                <F label="Date of birth" value={form.birth_date} onChange={(v) => setForm({ ...form, birth_date: v })} type="date" />
                <div>
                  <label className="block text-[13px] font-medium text-gray-700 mb-1">Gender</label>
                  <select value={form.gender} onChange={(e) => setForm({ ...form, gender: e.target.value })} className="input-field">
                    <option value="">—</option><option value="Female">Female</option><option value="Male">Male</option>
                  </select>
                </div>
              </div>
            </div>
            <div>
              <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Address</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <F label="Street address" value={form.address} onChange={(v) => setForm({ ...form, address: v })} colSpan />
                <F label="Suburb" value={form.suburb} onChange={(v) => setForm({ ...form, suburb: v })} />
                <F label="State" value={form.state} onChange={(v) => setForm({ ...form, state: v })} />
                <F label="Postcode" value={form.postcode} onChange={(v) => setForm({ ...form, postcode: v })} />
              </div>
            </div>
            <div>
              <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Emergency contact</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <F label="Contact name" value={form.emergency_contact_name} onChange={(v) => setForm({ ...form, emergency_contact_name: v })} />
                <F label="Contact phone" value={form.emergency_contact_phone} onChange={(v) => setForm({ ...form, emergency_contact_phone: v })} type="tel" />
              </div>
            </div>
          </div>
        )}

        {tab === "company" && (
          <div className="space-y-5">
            <div>
              <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Company information</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <F label="Business structure" value={form.business_structure || ""} onChange={(v) => setForm({ ...form, business_structure: v })} placeholder="e.g. Sole Trader" />
                <F label="GST registered" value={form.gst_registered || ""} onChange={(v) => setForm({ ...form, gst_registered: v })} placeholder="Yes / No" />
                <F label="Tax number" value={form.tax_number || ""} onChange={(v) => setForm({ ...form, tax_number: v })} placeholder="e.g. 123 456 789" />
                <F label="Company number" value={form.company_number || ""} onChange={(v) => setForm({ ...form, company_number: v })} />
                <F label="Phone" value={form.company_phone || ""} onChange={(v) => setForm({ ...form, company_phone: v })} type="tel" />
                <F label="Website" value={form.company_website || ""} onChange={(v) => setForm({ ...form, company_website: v })} placeholder="https://" colSpan />
              </div>
            </div>
            <div>
              <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Company address</h4>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <F label="Street address" value={form.company_address || ""} onChange={(v) => setForm({ ...form, company_address: v })} colSpan />
                <F label="Suburb" value={form.company_suburb || ""} onChange={(v) => setForm({ ...form, company_suburb: v })} />
                <F label="State" value={form.company_state || ""} onChange={(v) => setForm({ ...form, company_state: v })} />
                <F label="Postcode" value={form.company_postcode || ""} onChange={(v) => setForm({ ...form, company_postcode: v })} />
              </div>
            </div>
          </div>
        )}
      </form>
    </div>
  );
}

/* ─── CSV Export ─── */
function exportCSV(clients: Client[]) {
  const headers = ["Name", "Email", "Phone", "Address", "Suburb", "State", "Postcode", "Created"];
  const rows = clients.map(c => [
    c.name, c.email || "", c.phone || "", c.address || "",
    c.suburb || "", c.state || "", c.postcode || "",
    new Date(c.created_at).toLocaleDateString("en-AU"),
  ]);
  const csv = [headers, ...rows].map(r => r.map(v => `"${(v || "").replace(/"/g, '""')}"`).join(",")).join("\n");
  const blob = new Blob([csv], { type: "text/csv" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `clients-${new Date().toISOString().split("T")[0]}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

/* ─── CSV Import Modal ─── */
function ImportModal({ onClose, onImported }: { onClose: () => void; onImported: () => void }) {
  const [step, setStep] = useState<"upload" | "map" | "preview" | "done">("upload");
  const [csvHeaders, setCsvHeaders] = useState<string[]>([]);
  const [csvRows, setCsvRows] = useState<string[][]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<{ imported: number; errors: string[] }>({ imported: 0, errors: [] });
  const fileRef = useRef<HTMLInputElement>(null);

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => {
      const text = ev.target?.result as string;
      const lines = text.split(/\r?\n/).filter(l => l.trim());
      if (lines.length < 2) return;

      // Parse CSV (handle quoted fields)
      const parseLine = (line: string): string[] => {
        const result: string[] = [];
        let current = "";
        let inQuotes = false;
        for (let i = 0; i < line.length; i++) {
          const ch = line[i];
          if (ch === '"') { inQuotes = !inQuotes; }
          else if (ch === "," && !inQuotes) { result.push(current.trim()); current = ""; }
          else { current += ch; }
        }
        result.push(current.trim());
        return result;
      };

      const headers = parseLine(lines[0]);
      const rows = lines.slice(1).map(parseLine).filter(r => r.some(v => v));

      setCsvHeaders(headers);
      setCsvRows(rows);

      // Auto-map by fuzzy name matching
      const autoMap: Record<string, string> = {};
      headers.forEach((h) => {
        const lower = h.toLowerCase().replace(/[^a-z]/g, "");
        const match = CLIENT_FIELDS.find(f => {
          const fl = f.label.toLowerCase().replace(/[^a-z]/g, "");
          const fk = f.key.toLowerCase().replace(/[^a-z]/g, "");
          return fl === lower || fk === lower || lower.includes(fl) || fl.includes(lower)
            || (lower === "firstname" && f.key === "name")
            || (lower === "fullname" && f.key === "name")
            || (lower === "mobile" && f.key === "phone")
            || (lower === "cell" && f.key === "phone")
            || (lower === "zip" && f.key === "postcode")
            || (lower === "city" && f.key === "suburb")
            || (lower === "dob" && f.key === "birth_date")
            || (lower === "dateofbirth" && f.key === "birth_date");
        });
        autoMap[h] = match ? match.key : "__ignore__";
      });
      setMapping(autoMap);
      setStep("map");
    };
    reader.readAsText(file);
  };

  const handleImport = async () => {
    setImporting(true);
    const errors: string[] = [];
    let imported = 0;

    for (let i = 0; i < csvRows.length; i++) {
      const row = csvRows[i];
      const client: Record<string, string> = {};
      csvHeaders.forEach((h, idx) => {
        const field = mapping[h];
        if (field && field !== "__ignore__" && row[idx]) {
          client[field] = row[idx];
        }
      });

      if (!client.name) {
        errors.push(`Row ${i + 2}: Missing required field "Name"`);
        continue;
      }

      try {
        await api.createClient(client);
        imported++;
      } catch (err) {
        errors.push(`Row ${i + 2} (${client.name}): ${err instanceof Error ? err.message : "Failed"}`);
      }
    }

    setResult({ imported, errors });
    setStep("done");
    setImporting(false);
    if (imported > 0) onImported();
  };

  const mappedPreview = csvRows.slice(0, 5).map(row => {
    const obj: Record<string, string> = {};
    csvHeaders.forEach((h, idx) => {
      const field = mapping[h];
      if (field && field !== "__ignore__") obj[field] = row[idx] || "";
    });
    return obj;
  });

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />
      <div className="relative min-h-screen flex items-start justify-center pt-10 pb-10 px-4">
        <div className="relative w-full max-w-3xl bg-white rounded-xl shadow-2xl">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
            <h2 className="text-[15px] font-bold text-gray-900">
              {step === "upload" && "Import Clients"}
              {step === "map" && "Map Fields"}
              {step === "preview" && "Preview Import"}
              {step === "done" && "Import Complete"}
            </h2>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>

          <div className="px-6 py-5 max-h-[70vh] overflow-y-auto">
            {/* Step 1: Upload */}
            {step === "upload" && (
              <div>
                <p className="text-[13px] text-gray-500 mb-5">Upload a CSV file with your client data. The first row should contain column headers.</p>
                <div
                  onClick={() => fileRef.current?.click()}
                  className="border-2 border-dashed border-gray-300 rounded-xl px-8 py-12 text-center cursor-pointer hover:border-indigo-400 hover:bg-indigo-50/30 transition-all"
                >
                  <svg className="mx-auto h-10 w-10 text-gray-300 mb-3" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m6.75 12l-3-3m0 0l-3 3m3-3v6m-1.5-15H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" /></svg>
                  <p className="text-[14px] font-medium text-gray-700">Click to upload CSV</p>
                  <p className="text-[12px] text-gray-400 mt-1">.csv files only</p>
                </div>
                <input ref={fileRef} type="file" accept=".csv" className="hidden" onChange={handleFile} />
              </div>
            )}

            {/* Step 2: Map fields */}
            {step === "map" && (
              <div>
                <p className="text-[13px] text-gray-500 mb-1">{csvRows.length} rows found. Map your CSV columns to client fields.</p>
                <p className="text-[11px] text-gray-400 mb-5">Set to "— Ignore —" to skip a column.</p>

                <div className="space-y-3">
                  {csvHeaders.map((h) => (
                    <div key={h} className="flex items-center gap-3">
                      <div className="w-1/3">
                        <p className="text-[13px] font-medium text-gray-900 truncate">{h}</p>
                        <p className="text-[11px] text-gray-400 truncate">{csvRows[0]?.[csvHeaders.indexOf(h)] || "—"}</p>
                      </div>
                      <svg className="h-4 w-4 text-gray-300 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" /></svg>
                      <select
                        value={mapping[h] || "__ignore__"}
                        onChange={(e) => setMapping({ ...mapping, [h]: e.target.value })}
                        className="flex-1 input-field text-[13px]"
                      >
                        <option value="__ignore__">— Ignore —</option>
                        {CLIENT_FIELDS.map(f => (
                          <option key={f.key} value={f.key}>{f.label}{f.required ? " *" : ""}</option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>

                {!Object.values(mapping).includes("name") && (
                  <div className="mt-4 rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-[13px] text-red-700">
                    ⚠ "Full Name" must be mapped to a column. It's required for every client.
                  </div>
                )}

                <div className="mt-6 flex justify-between">
                  <button onClick={() => setStep("upload")} className="btn-secondary text-[12px]">Back</button>
                  <button
                    onClick={() => setStep("preview")}
                    disabled={!Object.values(mapping).includes("name")}
                    className="btn-primary text-[12px]"
                  >
                    Preview ({csvRows.length} rows)
                  </button>
                </div>
              </div>
            )}

            {/* Step 3: Preview */}
            {step === "preview" && (
              <div>
                <p className="text-[13px] text-gray-500 mb-4">Preview of first {Math.min(5, csvRows.length)} rows. Check the mapping looks correct.</p>
                <div className="overflow-x-auto rounded-lg border border-gray-200">
                  <table className="w-full text-[12px]">
                    <thead>
                      <tr className="bg-gray-50">
                        {CLIENT_FIELDS.filter(f => Object.values(mapping).includes(f.key)).map(f => (
                          <th key={f.key} className="px-3 py-2 text-left text-[11px] font-medium text-gray-500 uppercase">{f.label}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100">
                      {mappedPreview.map((row, i) => (
                        <tr key={i}>
                          {CLIENT_FIELDS.filter(f => Object.values(mapping).includes(f.key)).map(f => (
                            <td key={f.key} className="px-3 py-2 text-gray-700">{row[f.key] || "—"}</td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div className="mt-6 flex justify-between">
                  <button onClick={() => setStep("map")} className="btn-secondary text-[12px]">Back to mapping</button>
                  <button onClick={handleImport} disabled={importing} className="btn-primary text-[12px]">
                    {importing ? "Importing..." : `Import ${csvRows.length} clients`}
                  </button>
                </div>
              </div>
            )}

            {/* Step 4: Done */}
            {step === "done" && (
              <div className="text-center py-4">
                <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100 mx-auto mb-4">
                  <svg className="h-7 w-7 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                </div>
                <p className="text-lg font-semibold text-gray-900">{result.imported} client{result.imported !== 1 ? "s" : ""} imported</p>
                {result.errors.length > 0 && (
                  <div className="mt-4 text-left rounded-lg bg-red-50 border border-red-200 px-4 py-3 max-h-40 overflow-y-auto">
                    <p className="text-[12px] font-medium text-red-700 mb-2">{result.errors.length} error{result.errors.length !== 1 ? "s" : ""}:</p>
                    {result.errors.map((e, i) => (
                      <p key={i} className="text-[11px] text-red-600">{e}</p>
                    ))}
                  </div>
                )}
                <button onClick={onClose} className="btn-primary text-[12px] mt-6">Done</button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
