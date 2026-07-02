import { useEffect, useState } from "react";

import api from "../../api/client";

// ReacceptanceLinksPanel shows the staff the outstanding re-acceptance
// signing links for a single membership. The admin uses this to copy the
// shareable URL and forward it to the member (email, SMS, etc) when the
// automated trigger hasn't gone out yet, or for a manual reminder.

interface StaffLink {
  id: string;
  token: string;
  terms_id: string;
  terms_title: string;
  terms_version: number;
  expires_at: string;
  completed_at?: string | null;
  created_at: string;
}

interface Props {
  membershipId: string;
  /** Public hostname for building the link URL. Defaults to window.location.origin. */
  publicOrigin?: string;
  /** Tenant slug for building the public URL. If omitted, fetched from /tenant/settings. */
  tenantSlug?: string;
  /** Hide the panel entirely when no links exist. Useful when embedded inside a larger card. */
  hideWhenEmpty?: boolean;
}

export default function ReacceptanceLinksPanel({ membershipId, publicOrigin, tenantSlug, hideWhenEmpty }: Props) {
  const [links, setLinks] = useState<StaffLink[] | null>(null);
  const [resolvedSlug, setResolvedSlug] = useState<string | null>(tenantSlug || null);
  const [error, setError] = useState("");
  const [copiedToken, setCopiedToken] = useState<string | null>(null);

  useEffect(() => {
    if (!membershipId) return;
    let cancelled = false;
    api
      .listReacceptanceLinks(membershipId)
      .then((res) => {
        if (cancelled) return;
        const data = res as { links?: StaffLink[] };
        setLinks(data.links || []);
      })
      .catch((e: Error) => {
        if (cancelled) return;
        setError(e.message || "Failed to load links");
      });
    return () => {
      cancelled = true;
    };
  }, [membershipId]);

  // Lazily fetch the tenant slug only when at least one outstanding link is
  // present — saves a round-trip on every membership card where the member
  // has no pending re-acceptances.
  useEffect(() => {
    if (resolvedSlug || !links || links.length === 0) return;
    let cancelled = false;
    api
      .getTenantSettings()
      .then((res) => {
        if (cancelled) return;
        const data = res as { slug?: string };
        if (data.slug) setResolvedSlug(data.slug);
      })
      .catch(() => {
        // non-fatal — the panel can still show "signed/expired" status
      });
    return () => {
      cancelled = true;
    };
  }, [links, resolvedSlug]);

  const origin = (publicOrigin ?? window.location.origin).replace(/\/$/, "");

  const buildUrl = (token: string) =>
    resolvedSlug ? `${origin}/reaccept/${resolvedSlug}/${token}` : "";

  const copy = async (token: string) => {
    try {
      await navigator.clipboard.writeText(buildUrl(token));
      setCopiedToken(token);
      setTimeout(() => setCopiedToken(c => (c === token ? null : c)), 1500);
    } catch {
      // ignore; the user can still hand-copy from the textbox below
    }
  };

  if (error) {
    return <p className="text-[12px] text-red-500">{error}</p>;
  }
  if (links === null) {
    return hideWhenEmpty ? null : <p className="text-[12px] text-gray-400">Loading re-acceptance links...</p>;
  }
  if (links.length === 0) {
    return hideWhenEmpty ? null : (
      <p className="text-[12px] text-gray-400">No re-acceptance links issued for this membership.</p>
    );
  }

  return (
    <div className="space-y-2">
      <p className="text-[12px] font-medium text-gray-700">Re-acceptance links</p>
      {links.map((link) => {
        const url = buildUrl(link.token);
        const expired = !link.completed_at && new Date(link.expires_at).getTime() < Date.now();
        return (
          <div key={link.id} className="rounded-md border border-gray-200 bg-white px-3 py-2 text-[12px]">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <div>
                <p className="font-medium text-gray-800">
                  {link.terms_title} <span className="text-gray-400">v{link.terms_version}</span>
                </p>
                <p className="text-[11px] text-gray-400">
                  {link.completed_at ? (
                    <>Signed {new Date(link.completed_at).toLocaleDateString("en-AU")}</>
                  ) : expired ? (
                    <>Expired {new Date(link.expires_at).toLocaleDateString("en-AU")}</>
                  ) : (
                    <>Expires {new Date(link.expires_at).toLocaleDateString("en-AU")}</>
                  )}
                </p>
              </div>
              {link.completed_at ? (
                <span className="badge badge-green">Signed</span>
              ) : expired ? (
                <span className="badge badge-gray">Expired</span>
              ) : (
                <button
                  type="button"
                  onClick={() => copy(link.token)}
                  disabled={!resolvedSlug}
                  className="text-[11px] px-2 py-1 rounded-md border border-indigo-200 text-indigo-700 hover:bg-indigo-50 disabled:opacity-50"
                >
                  {!resolvedSlug ? "Loading..." : copiedToken === link.token ? "Copied!" : "Copy link"}
                </button>
              )}
            </div>
            {!link.completed_at && !expired && url && (
              <input
                type="text"
                value={url}
                readOnly
                onClick={(e) => (e.target as HTMLInputElement).select()}
                className="mt-2 w-full font-mono text-[10px] px-2 py-1 rounded border border-gray-200 bg-gray-50 text-gray-700"
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
