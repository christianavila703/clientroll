import { useState } from "react";
import { useNavigate } from "react-router-dom";
import api, { ApiError } from "../../api/client";

export default function PlatformTenantCreatePage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    name: "",
    slug: "",
    timezone: "Australia/Brisbane",
    admin_email: "",
    admin_password: "",
    admin_first_name: "",
    admin_last_name: "",
  });

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await api.createTenant(form);
      navigate("/platform/tenants");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to create tenant.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-3xl">
      <div className="mb-6">
        <h1 className="page-title">Create Tenant</h1>
        <p className="mt-1 text-sm text-gray-500">Create a tenant and initial tenant admin user.</p>
      </div>

      <form onSubmit={onSubmit} className="card space-y-6">
        {error && <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">Tenant Name</span>
            <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          </label>
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">Slug</span>
            <input
              className="input"
              value={form.slug}
              onChange={(e) => setForm({ ...form, slug: e.target.value.toLowerCase().replace(/\s+/g, "-") })}
              required
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-gray-700">Timezone</span>
            <input className="input" value={form.timezone} onChange={(e) => setForm({ ...form, timezone: e.target.value })} />
          </label>
        </div>

        <div>
          <h2 className="mb-3 text-sm font-semibold text-gray-900">Initial Tenant Admin</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">First Name</span>
              <input className="input" value={form.admin_first_name} onChange={(e) => setForm({ ...form, admin_first_name: e.target.value })} required />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">Last Name</span>
              <input className="input" value={form.admin_last_name} onChange={(e) => setForm({ ...form, admin_last_name: e.target.value })} />
            </label>
            <label className="block sm:col-span-2">
              <span className="mb-1 block text-sm font-medium text-gray-700">Email</span>
              <input className="input" type="email" value={form.admin_email} onChange={(e) => setForm({ ...form, admin_email: e.target.value })} required />
            </label>
            <label className="block sm:col-span-2">
              <span className="mb-1 block text-sm font-medium text-gray-700">Password</span>
              <input className="input" type="password" value={form.admin_password} onChange={(e) => setForm({ ...form, admin_password: e.target.value })} required />
            </label>
          </div>
        </div>

        <div className="flex gap-3">
          <button type="submit" className="btn-primary" disabled={loading}>
            {loading ? "Creating..." : "Create Tenant"}
          </button>
          <button type="button" className="btn-secondary" onClick={() => navigate("/platform/tenants")}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
