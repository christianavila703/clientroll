import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import api from "../../api/client";

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <button onClick={handleCopy} className="btn-secondary text-[12px] relative" title="Copy signup link">
      {copied ? (
        <>
          <svg className="h-3.5 w-3.5 text-emerald-600" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
          <span className="text-emerald-600">Copied</span>
        </>
      ) : (
        <>
          <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" /></svg>
          <span>Copy</span>
        </>
      )}
    </button>
  );
}

interface MembershipType {
  id: string;
  membership_name?: string;
  membership_level?: string;
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
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api.listMembershipTypes(),
      api.getTenantSettings(),
    ]).then(([mtData, tenantData]) => {
      const membershipData = mtData as any;
      const tenantSettings = tenantData as any;
      setTypes((membershipData.membership_types || []).filter((mt: MembershipType) => mt.is_active));
      setTenant({ slug: tenantSettings.slug, name: tenantSettings.name });
    }).catch((e) => { console.error(e); setError("Failed to load signup launcher"); }).finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>;

  const baseUrl = window.location.origin;
  const membershipName = (mt: MembershipType) => (mt.membership_name || "").trim();
  const membershipLevel = (mt: MembershipType) => (mt.membership_level || "").trim();
  const fallbackTitle = (mt: MembershipType) => mt.title;

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Client Signup</h1>
          <p className="text-[13px] text-gray-500">Start a membership signup on iPad or share a link with a client</p>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      )}

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
                <img src={mt.top_image_url} alt={membershipName(mt) || fallbackTitle(mt)} className="w-full h-36 object-cover" />
              )}
              <div className="px-5 py-4">
                <h3 className="text-[15px] font-semibold text-gray-900">{membershipName(mt) || fallbackTitle(mt)}</h3>
                {membershipLevel(mt) && (
                  <p className="mt-0.5 text-[13px] font-medium" style={{ color: "var(--brandPrimary, #4f46e5)" }}>
                    {membershipLevel(mt)}
                  </p>
                )}
                {mt.description && (
                  <p className="text-[12px] text-gray-500 mt-1 line-clamp-2">{mt.description}</p>
                )}
                <div className="flex items-baseline gap-1 mt-3">
                  <span className="text-xl font-bold text-indigo-600 tabular-nums">${(() => {
                    const opts = mt.term_options || [];
                    if (opts.length === 0) return mt.total_value.toLocaleString();
                    if (opts.length === 1) return opts[0].fee_total.toLocaleString();
                    const fees = opts.map(o => o.fee_total).sort((a, b) => a - b);
                    return fees[0] === fees[fees.length - 1] ? fees[0].toLocaleString() : `${fees[0].toLocaleString()} – ${fees[fees.length - 1].toLocaleString()}`;
                  })()}</span>
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
                  <CopyButton text={`${baseUrl}/${tenant?.slug}/${mt.slug}`} />
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
                <p className="text-[13px] font-medium text-gray-900">{membershipName(mt) || fallbackTitle(mt)}</p>
                {membershipLevel(mt) && (
                  <p className="text-[12px] font-medium" style={{ color: "var(--brandPrimary, #4f46e5)" }}>
                    {membershipLevel(mt)}
                  </p>
                )}
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
