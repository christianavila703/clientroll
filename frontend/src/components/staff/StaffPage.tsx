import { useState, useEffect } from "react";
import api from "../../api/client";
import { useAuth } from "../../store/AuthContext";

interface StaffMember {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  is_active: boolean;
  last_login_at?: string;
}

export default function StaffPage() {
  const { user } = useAuth();
  const [staff, setStaff] = useState<StaffMember[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [loading, setLoading] = useState(true);
  const isAdmin = user?.role === "tenant_admin";

  useEffect(() => {
    loadStaff();
  }, []);

  const loadStaff = async () => {
    try {
      const data = (await api.listStaff()) as { staff: StaffMember[] };
      setStaff(data.staff || []);
    } catch {
      // Handle error
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Team</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage your team members
          </p>
        </div>
        {isAdmin && (
          <button
            onClick={() => setShowForm(!showForm)}
            className="btn-primary"
          >
            New team member
          </button>
        )}
      </div>

      {showForm && <CreateStaffForm onCreated={() => { setShowForm(false); loadStaff(); }} />}

      {loading ? (
        <div className="card text-center text-gray-500">Loading...</div>
      ) : staff.length === 0 ? (
        <div className="card text-center text-gray-500">No staff members yet</div>
      ) : (
        <div className="card overflow-hidden p-0">
          {/* Desktop table */}
          <div className="hidden sm:block overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Name</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Email</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Role</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {staff.map((s) => (
                  <tr key={s.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">
                      {s.first_name} {s.last_name}
                    </td>
                    <td className="px-4 py-3 text-gray-600">{s.email}</td>
                    <td className="px-4 py-3">
                      <span className={`badge ${
                        s.role === "tenant_admin"
                          ? "badge-purple"
                          : "badge-gray"
                      }`}>
                        {s.role === "tenant_admin" ? "Admin" : "Staff"}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`badge ${
                        s.is_active
                          ? "badge-green"
                          : "badge-red"
                      }`}>
                        {s.is_active ? "Active" : "Inactive"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Mobile card view */}
          <div className="sm:hidden divide-y divide-gray-100">
            {staff.map((s) => (
              <div key={s.id} className="p-4">
                <div className="flex items-center justify-between">
                  <p className="font-medium text-gray-900">
                    {s.first_name} {s.last_name}
                  </p>
                  <span className={`badge ${
                    s.is_active ? "badge-green" : "badge-red"
                  }`}>
                    {s.is_active ? "Active" : "Inactive"}
                  </span>
                </div>
                <p className="mt-1 text-sm text-gray-500">{s.email}</p>
                <p className="mt-1 text-xs text-gray-400">
                  {s.role === "tenant_admin" ? "Admin" : "Staff"}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function CreateStaffForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    email: "", password: "", first_name: "", last_name: "", role: "tenant_staff",
  });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api.createStaff(form);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create staff member.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card mb-5 px-5 py-4">
      <h3 className="text-base font-semibold text-gray-900 mb-4">Add Staff Member</h3>
      {error && (
        <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      )}
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">First Name</label>
            <input value={form.first_name} onChange={(e) => setForm({ ...form, first_name: e.target.value })} className="input-field" required />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Last Name</label>
            <input value={form.last_name} onChange={(e) => setForm({ ...form, last_name: e.target.value })} className="input-field" />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
          <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} className="input-field" required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
          <input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} className="input-field" required minLength={10} />
          <p className="mt-1 text-xs text-gray-400">Min 10 chars, must include upper, lower, and digit</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Role</label>
          <select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })} className="input-field">
            <option value="tenant_staff">Staff</option>
            <option value="tenant_admin">Admin</option>
          </select>
        </div>
        <div className="flex gap-3 justify-end">
          <button type="button" onClick={onCreated} className="btn-secondary">Cancel</button>
          <button type="submit" disabled={saving} className="btn-primary">{saving ? "Creating..." : "Create"}</button>
        </div>
      </form>
    </div>
  );
}
