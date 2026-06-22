import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";

import SignatureCanvas from "../signatures/SignatureCanvas";
import {
  substituteTermsVariables,
  type TermsSubstitutionContext,
} from "../../utils/termsContent";

// ReacceptanceDetail mirrors the JSON returned by
// GET /api/signup/{tenantSlug}/reaccept/{token}. Field names match the Go
// json tags exactly so we can drop the response straight into state.
interface ReacceptanceDetail {
  invite: {
    id: string;
    token: string;
    membership_id: string;
    terms_id: string;
    terms_version: number;
    expires_at: string;
    completed_at?: string | null;
  };
  tenant_slug: string;
  org_name: string;
  org_logo_url?: string | null;
  client_first_name: string;
  client_last_name: string;
  client_email: string;
  client_id: string;
  membership_title: string;
  terms_id: string;
  terms_title: string;
  terms_content: string;
  terms_change_note?: string | null;
  terms_effective_at?: string | null;
  previous_version?: number | null;
}

type GoneResponse = {
  error: string;
  completed_at?: string;
  expires_at?: string;
  tenant_slug?: string;
  org_name?: string;
  client_name?: string;
};

type Status =
  | { kind: "loading" }
  | { kind: "ready"; detail: ReacceptanceDetail }
  | { kind: "completed"; gone: GoneResponse }
  | { kind: "expired"; gone: GoneResponse }
  | { kind: "missing"; message: string }
  | { kind: "success"; detail: ReacceptanceDetail };

export default function ReacceptPage() {
  const { tenantSlug, token } = useParams<{ tenantSlug: string; token: string }>();
  const [status, setStatus] = useState<Status>({ kind: "loading" });
  const [signedName, setSignedName] = useState("");
  const [signatureImage, setSignatureImage] = useState<string | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState("");

  useEffect(() => {
    if (!tenantSlug || !token) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(`/api/signup/${tenantSlug}/reaccept/${token}`);
        const data = await res.json();
        if (cancelled) return;
        if (res.status === 410) {
          if (data.error === "Link already used") {
            setStatus({ kind: "completed", gone: data as GoneResponse });
          } else {
            setStatus({ kind: "expired", gone: data as GoneResponse });
          }
          return;
        }
        if (!res.ok) {
          setStatus({ kind: "missing", message: data.error || "Link not found" });
          return;
        }
        setStatus({ kind: "ready", detail: data as ReacceptanceDetail });
        setSignedName(`${data.client_first_name || ""} ${data.client_last_name || ""}`.trim());
      } catch (e) {
        if (!cancelled) {
          setStatus({ kind: "missing", message: e instanceof Error ? e.message : "Could not load link" });
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [tenantSlug, token]);

  // Build the substitution context from member + org details so the
  // displayed T&C uses the member's actual data, not raw {{ placeholders }}.
  const substitutionContext = useMemo<TermsSubstitutionContext>(() => {
    if (status.kind !== "ready" && status.kind !== "success") return {};
    const detail = status.kind === "ready" ? status.detail : status.detail;
    return {
      client_name: `${detail.client_first_name} ${detail.client_last_name}`.trim(),
      client_email: detail.client_email,
      membership_title: detail.membership_title,
      company_name: detail.org_name,
    };
  }, [status]);

  const renderedContent = useMemo(() => {
    if (status.kind !== "ready" && status.kind !== "success") return "";
    const raw = status.kind === "ready" ? status.detail.terms_content : status.detail.terms_content;
    return substituteTermsVariables(raw, substitutionContext);
  }, [status, substitutionContext]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (status.kind !== "ready") return;
    setSubmitError("");
    if (!confirmed) {
      setSubmitError("Please tick the confirmation box to continue.");
      return;
    }
    if (!signedName.trim()) {
      setSubmitError("Please type your full name to sign.");
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch(`/api/signup/${tenantSlug}/reaccept/${token}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          signature_typed: signedName.trim(),
          signature_image_data: signatureImage || "",
          confirmed: true,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setSubmitError(data.error || "Failed to record signature. Please try again.");
        setSubmitting(false);
        return;
      }
      setStatus({ kind: "success", detail: status.detail });
    } catch (e) {
      setSubmitError(e instanceof Error ? e.message : "Network error. Please try again.");
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-3xl mx-auto px-4 py-8">
        {status.kind === "loading" && <LoadingCard />}

        {status.kind === "missing" && (
          <ErrorCard
            title="Link not found"
            message={status.message}
            help="If you received this link by email and believe it should still work, please contact your provider for a new link."
          />
        )}

        {status.kind === "expired" && (
          <ErrorCard
            title="This signing link has expired"
            message={
              status.gone.org_name
                ? `Please contact ${status.gone.org_name} for a fresh link.`
                : "Please contact your provider for a fresh link."
            }
          />
        )}

        {status.kind === "completed" && (
          <ErrorCard
            title="This link has already been used"
            message={
              status.gone.completed_at
                ? `You signed these updated terms on ${formatTimestamp(status.gone.completed_at)}.`
                : "These terms have already been re-accepted."
            }
            help="If you need a copy of the signed terms, contact your provider."
          />
        )}

        {status.kind === "ready" && (
          <form onSubmit={onSubmit} className="space-y-6">
            <Header detail={status.detail} />
            <ChangeBanner detail={status.detail} />
            <section className="card overflow-hidden">
              <div className="card-header">
                <h3>{status.detail.terms_title}</h3>
                <span className="text-[11px] text-gray-400">Version {status.detail.invite.terms_version}</span>
              </div>
              <div className="card-body">
                <div
                  className="prose prose-sm max-w-none text-[13px] text-gray-800 leading-relaxed"
                  dangerouslySetInnerHTML={{ __html: renderedContent }}
                />
              </div>
            </section>

            <section className="card">
              <div className="card-header">
                <h3>Sign to accept</h3>
              </div>
              <div className="card-body space-y-4">
                <label className="flex items-start gap-2 text-[13px] text-gray-700">
                  <input
                    type="checkbox"
                    checked={confirmed}
                    onChange={e => setConfirmed(e.target.checked)}
                    className="mt-0.5 h-4 w-4 rounded border-gray-300 text-indigo-600"
                  />
                  <span>
                    I, <strong>{`${status.detail.client_first_name} ${status.detail.client_last_name}`.trim()}</strong>,
                    confirm that I have read and agree to these updated terms and conditions for my
                    {" "}<em>{status.detail.membership_title}</em> membership with {status.detail.org_name}.
                  </span>
                </label>
                <div>
                  <label className="block text-[12px] font-medium text-gray-700 mb-1">Type your full name to sign</label>
                  <input
                    type="text"
                    value={signedName}
                    onChange={e => setSignedName(e.target.value)}
                    placeholder="Your full name"
                    required
                    className="input-field"
                  />
                </div>
                <div>
                  <p className="text-[12px] font-medium text-gray-700 mb-1">Draw your signature (optional)</p>
                  <SignatureCanvas onSave={setSignatureImage} height={160} />
                </div>
                {submitError && (
                  <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-700">{submitError}</div>
                )}
                <div className="flex justify-end pt-1">
                  <button type="submit" disabled={submitting} className="btn-primary">
                    {submitting ? "Recording..." : "Sign and accept"}
                  </button>
                </div>
              </div>
            </section>

            <p className="text-[11px] text-gray-400 text-center">
              By signing you create a binding record of acceptance. A copy can be requested from {status.detail.org_name} at any time.
            </p>
          </form>
        )}

        {status.kind === "success" && (
          <section className="card">
            <div className="card-body text-center py-10 space-y-3">
              <div className="mx-auto h-14 w-14 rounded-full bg-green-100 flex items-center justify-center">
                <svg className="h-7 w-7 text-green-600" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                </svg>
              </div>
              <h2 className="text-lg font-semibold text-gray-900">Thanks, you're all set</h2>
              <p className="text-[13px] text-gray-600">
                Your acceptance of the updated terms has been recorded with {status.detail.org_name}.
              </p>
              <p className="text-[12px] text-gray-400">You can close this page.</p>
            </div>
          </section>
        )}
      </div>
    </div>
  );
}

function Header({ detail }: { detail: ReacceptanceDetail }) {
  return (
    <header className="flex items-center gap-3">
      {detail.org_logo_url && (
        <img src={detail.org_logo_url} alt={detail.org_name} className="h-10 w-10 rounded object-contain bg-white border border-gray-200" />
      )}
      <div>
        <p className="text-[11px] uppercase tracking-wide text-gray-500">{detail.org_name}</p>
        <h1 className="text-lg font-semibold text-gray-900">Updated terms for your membership</h1>
        <p className="text-[12px] text-gray-500">
          Hi {detail.client_first_name}, please review the changes below and sign to confirm your agreement.
        </p>
      </div>
    </header>
  );
}

function ChangeBanner({ detail }: { detail: ReacceptanceDetail }) {
  if (!detail.terms_change_note && detail.previous_version == null) return null;
  return (
    <section className="rounded-lg border border-indigo-100 bg-indigo-50 px-4 py-3 text-[13px] text-indigo-900">
      <p className="font-semibold mb-1">What's changed</p>
      {detail.terms_change_note ? (
        <p className="whitespace-pre-wrap">{detail.terms_change_note}</p>
      ) : (
        <p>
          A new version of these terms has been published
          {detail.previous_version != null
            ? ` (updated from version ${detail.previous_version} to ${detail.invite.terms_version})`
            : ""}.
        </p>
      )}
    </section>
  );
}

function LoadingCard() {
  return (
    <div className="card px-5 py-10 text-center text-[13px] text-gray-400">Loading...</div>
  );
}

function ErrorCard({ title, message, help }: { title: string; message: string; help?: string }) {
  return (
    <div className="card">
      <div className="card-body text-center py-10 space-y-2">
        <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
        <p className="text-[13px] text-gray-600">{message}</p>
        {help && <p className="text-[12px] text-gray-400">{help}</p>}
      </div>
    </div>
  );
}

function formatTimestamp(iso: string): string {
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" });
  } catch {
    return iso;
  }
}
