import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import api from "../../api/client";

interface Tenant {
  id: string;
  name: string;
  slug: string;
  timezone: string;
  is_active: boolean;
  created_at: string;
}

export default function PlatformDashboard() {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .listTenants()
      .then((data) => {
        const d = data as { tenants: Tenant[] };
        setTenants(d.tenants || []);
      })
      .catch((e) => { console.error(e); setError("Failed to load tenants"); })
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">
            Platform Dashboard
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage all tenants on the platform
          </p>
        </div>
        <Link to="/platform/tenants/new" className="btn-primary">
          + Create Tenant
        </Link>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      )}

      {/* Summary */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 mb-8">
        <div className="card border-l-4 border-l-primary-500">
          <p className="text-sm font-medium text-gray-500">Total Tenants</p>
          <p className="mt-1 text-3xl font-bold text-gray-900">
            {tenants.length}
          </p>
        </div>
        <div className="card border-l-4 border-l-green-500">
          <p className="text-sm font-medium text-gray-500">Active</p>
          <p className="mt-1 text-3xl font-bold text-gray-900">
            {tenants.filter((t) => t.is_active).length}
          </p>
        </div>
        <div className="card border-l-4 border-l-red-500">
          <p className="text-sm font-medium text-gray-500">Inactive</p>
          <p className="mt-1 text-3xl font-bold text-gray-900">
            {tenants.filter((t) => !t.is_active).length}
          </p>
        </div>
      </div>

      {/* Tenant list */}
      {loading ? (
        <div className="card text-center text-gray-500">Loading...</div>
      ) : tenants.length === 0 ? (
        <div className="card text-center text-gray-500">
          No tenants yet. Create your first one!
        </div>
      ) : (
        <div className="card overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Name</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Slug</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Timezone</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {tenants.map((t) => (
                  <tr key={t.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">{t.name}</td>
                    <td className="px-4 py-3 text-gray-600 font-mono text-xs">{t.slug}</td>
                    <td className="px-4 py-3 text-gray-600">{t.timezone}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        t.is_active ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
                      }`}>
                        {t.is_active ? "Active" : "Inactive"}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-500 text-xs">
                      {new Date(t.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
