import { useState, useEffect } from "react";
import { Link } from "react-router-dom";

interface MembershipType {
  id: string;
  title: string;
  slug: string;
  description?: string;
  total_value: number;
  is_active: boolean;
  top_image_url?: string;
  term_options?: { id: string; duration_months: number; fee_total: number }[];
}

interface TenantInfo {
  slug: string;
  name: string;
}

export default function StaffSignupLauncher() {
  const [types, setTypes] = useState<MembershipType[]>([]);
  const [tenant, setTenant] = useState<TenantInfo | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    Promise.all([
      fetch("/api/tenant/membership-types", { headers: h }).then(r => r.json()),
      fetch("/api/tenant/settings/tenant", { headers: h }).then(r => r.json()),
    ]).then(([mtData, tenantData]) => {
      setTypes((mtData.membership_types || []).filter((mt: MembershipType) => mt.is_active));
      setTenant({ slug: tenantData.slug, name: tenantData.name });
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>;

  const baseUrl = window.location.origin;

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Client Signup</h1>
          <p className="text-[13px] text-gray-500">Start a membership signup on iPad or share a link with a client</p>
        </div>
      </div>

      {/* Quick launch — iPad mode */}
      <div className="card mb-5 overflow-hidden">
        <div className="bg-indigo-600 px-6 py-5 text-white">
          <h2 className="text-lg font-semibold">iPad Signup Mode</h2>
          <p className="text-indigo-200 text-[13px] mt-1">Select a membership below, then hand the device to your client to complete the signup.</p>
        </div>
        <div className="px-6 py-4">
          <p className="text-[13px] text-gray-500 mb-3">Or share this link for clients to browse all memberships:</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded-lg bg-gray-50 border border-gray-200 px-3 py-2 text-[13px] font-mono text-gray-700 truncate">
              {baseUrl}/join/{tenant?.slug}
            </code>
            <button
              onClick={() => { navigator.clipboard.writeText(`${baseUrl}/join/${tenant?.slug}`); }}
              className="btn-secondary text-[12px] flex-shrink-0"
            >
              Copy link
            </button>
          </div>
        </div>
      </div>

      {/* Membership cards */}
      {types.length === 0 ? (
        <div className="card px-5 py-12 text-center">
          <p className="text-[13px] text-gray-500">No active membership types. Create one first.</p>
          <Link to="/memberships?new=1" className="mt-2 text-[13px] font-medium text-indigo-600 hover:text-indigo-700 inline-block">+ Create membership type</Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {types.map((mt) => (
            <div key={mt.id} className="card overflow-hidden group hover:border-indigo-200 hover:shadow-md transition-all">
              {mt.top_image_url && (
                <img src={mt.top_image_url} alt={mt.title} className="w-full h-36 object-cover" />
              )}
              <div className="px-5 py-4">
                <h3 className="text-[15px] font-semibold text-gray-900">{mt.title}</h3>
                {mt.description && (
                  <p className="text-[12px] text-gray-500 mt-1 line-clamp-2">{mt.description}</p>
                )}
                <div className="flex items-baseline gap-1 mt-3">
                  <span className="text-xl font-bold text-indigo-600 tabular-nums">${mt.total_value.toLocaleString()}</span>
                  <span className="text-[12px] text-gray-400">total value</span>
                </div>
                {mt.term_options && mt.term_options.length > 0 && (
                  <p className="text-[11px] text-gray-400 mt-1">{mt.term_options.length} term option{mt.term_options.length !== 1 ? "s" : ""}</p>
                )}

                <div className="mt-4 flex gap-2">
                  {/* Start signup on this device (iPad handoff) */}
                  <a
                    href={`/${tenant?.slug}/${mt.slug}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="btn-primary text-[12px] flex-1 justify-center"
                  >
                    Open on iPad
                  </a>
                  {/* Copy link */}
                  <button
                    onClick={() => navigator.clipboard.writeText(`${baseUrl}/${tenant?.slug}/${mt.slug}`)}
                    className="btn-secondary text-[12px]"
                    title="Copy signup link"
                  >
                    <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.86-3.06a4.5 4.5 0 00-1.242-7.244l-4.5-4.5a4.5 4.5 0 00-6.364 6.364L4.757 8.818" /></svg>
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Individual links */}
      <div className="card mt-5">
        <div className="card-header"><h3>Signup Links</h3></div>
        <div className="divide-y divide-gray-100">
          <div className="px-5 py-3 flex items-center justify-between">
            <div>
              <p className="text-[13px] font-medium text-gray-900">Browse all memberships</p>
              <p className="text-[11px] text-gray-400 font-mono">{baseUrl}/join/{tenant?.slug}</p>
            </div>
            <button onClick={() => navigator.clipboard.writeText(`${baseUrl}/join/${tenant?.slug}`)} className="btn-secondary text-[12px]">Copy</button>
          </div>
          {types.map((mt) => (
            <div key={mt.id} className="px-5 py-3 flex items-center justify-between">
              <div>
                <p className="text-[13px] font-medium text-gray-900">{mt.title}</p>
                <p className="text-[11px] text-gray-400 font-mono">{baseUrl}/{tenant?.slug}/{mt.slug}</p>
              </div>
              <button onClick={() => navigator.clipboard.writeText(`${baseUrl}/${tenant?.slug}/${mt.slug}`)} className="btn-secondary text-[12px]">Copy</button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
