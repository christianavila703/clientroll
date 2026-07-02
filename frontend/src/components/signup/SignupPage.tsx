import { useState, useEffect } from "react";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import MembershipSelect from "./MembershipSelect";
import MembershipDetail from "./MembershipDetail";
import AgreementForm from "./AgreementForm";
import ThankYouPage from "./ThankYouPage";
import type { Bonus, MembershipType, TermOption } from "../../types/memberships";
import type { TenantBranding } from "../../types/tenant";

interface TermsDoc {
  id: string;
  title: string;
  content: string;
  version: number;
  is_active: boolean;
}

interface OrgPayload {
  company_name?: string;
  company_trading_name?: string;
  company_abn?: string;
  company_address?: string;
  company_email?: string;
  company_phone?: string;
}

type Step = "select" | "detail" | "form" | "thankyou";
type Tenant = TenantBranding;

export default function SignupPage() {
  const { tenantSlug, membershipSlug, inviteToken } = useParams();
  const navigate = useNavigate();
  const location = useLocation();
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [memberships, setMemberships] = useState<MembershipType[]>([]);
  const [selected, setSelected] = useState<MembershipType | null>(null);
  const [selectedTerm, setSelectedTerm] = useState<TermOption | null>(null);
  const [terms, setTerms] = useState<TermsDoc[]>([]);
  const [org, setOrg] = useState<OrgPayload | null>(null);
  const [step, setStep] = useState<Step>("select");
  const [thankyou, setThankyou] = useState<Record<string, unknown>>({});
  const [inviteContext, setInviteContext] = useState<Record<string, unknown> | null>(null);
  const [inviteClient, setInviteClient] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingDetail, setLoadingDetail] = useState(false);

  useEffect(() => { window.scrollTo(0, 0); }, [step]);
  const [error, setError] = useState("");

  useEffect(() => {
    loadData();
    // inviteToken must be included: invite finalise URL has no membershipSlug; reloading when token changes matters.
  }, [tenantSlug, membershipSlug, inviteToken, location.search]);

  const loadData = async () => {
    try {
      const params = new URLSearchParams(window.location.search);
      const stripeState = params.get("stripe");
      const checkoutSessionId = params.get("session_id");
      // When returning from Stripe checkout, skip the signup data fetch
      // (the URL has no sig/exp so it would 401). Go straight to verification.
      if (stripeState === "success" || stripeState === "cancelled") {
        if (stripeState === "success") {
          if (checkoutSessionId && tenantSlug) {
            try {
              const vr = await fetch(
                `/api/signup/${tenantSlug}/checkout-session?session_id=${encodeURIComponent(checkoutSessionId)}`
              );
              const v = (await vr.json()) as Record<string, unknown>;
              if (!vr.ok) {
                setThankyou({
                  heading: "Payment status unclear",
                  body: String(v.error || "We could not verify your payment. Please contact the organisation or check your email."),
                });
              } else if (v.verified === true) {
                const mode = String(v.checkout_mode || "");
                let body = "Your membership setup is complete.";
                if (mode === "payment") {
                  body = "Your pay-in-full payment was received. Your membership is now active.";
                } else if (mode === "subscription" && v.stripe_subscription_id) {
                  body = "Your membership and recurring billing are set up.";
                } else if (mode === "subscription") {
                  body = "Your checkout finished. If you used bank debit, the mandate or first charge may still be processing.";
                }
                setThankyou({ heading: "Payment complete", body });
              } else {
                setThankyou({
                  heading: "Payment not finished",
                  body: "Stripe reports this checkout is not complete. You can open your membership link and try again.",
                });
              }
            } catch {
              setThankyou({
                heading: "Payment status unclear",
                body: "We could not verify your payment. Please contact the organisation.",
              });
            }
          } else {
            setThankyou({
              heading: "Thanks",
              body: "If you just paid in Stripe, confirmation can take a moment. If something looks wrong, contact us.",
            });
          }
          setStep("thankyou");
        } else {
          setError("Card payment was cancelled. You can try again.");
        }
        return;
      }
      if (inviteToken) {
        const res = await fetch(`/api/signup/${tenantSlug}/invite/${inviteToken}${location.search}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setSelected(data.membership);
        setTerms(data.terms || []);
        setOrg((data.org as OrgPayload) || null);
        setInviteContext(data.invite || null);
        setInviteClient(data.invite_client || null);
        const termsForMembership = data.membership?.term_options || [];
        const termId = String((data.invite || {}).term_option_id || "");
        const term = termsForMembership.find((t: any) => String(t.id) === termId) || termsForMembership[0] || null;
        if (!term) {
          throw new Error("This invite is missing a valid term option. Please request a new invite.");
        }
        setSelectedTerm(term);
        setStep("form");
      } else if (membershipSlug) {
        // Direct link to specific membership
        const res = await fetch(`/api/signup/${tenantSlug}/${membershipSlug}${location.search}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setSelected(data.membership);
        setTerms(data.terms || []);
        setOrg((data.org as OrgPayload) || null);
        if (data.payment_providers && data.tenant) {
          setTenant({ ...data.tenant, payment_providers: data.payment_providers });
        }
        setStep("detail");
      } else {
        // Browse all memberships
        const res = await fetch(`/api/signup/${tenantSlug}${location.search}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setMemberships(data.memberships || []);
        if (data.payment_providers && data.tenant) {
          setTenant({ ...data.tenant, payment_providers: data.payment_providers });
        }
        setStep("select");
        setSelected(null);
        setInviteContext(null);
        setInviteClient(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load signup page.");
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-200 border-t-primary-600" />
      </div>
    );
  }

  const openMembershipDetail = async (mt: MembershipType) => {
    if (!tenantSlug) return;
    setLoadingDetail(true);
    setError("");
    try {
      const qs = location.search || "";
      const res = await fetch(`/api/signup/${tenantSlug}/${mt.slug}${qs}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Unable to load membership details.");
      setSelected((data.membership as MembershipType) || mt);
      setTerms(data.terms || []);
      setOrg((data.org as OrgPayload) || null);
      if (data.tenant) {
        setTenant({
          ...(data.tenant as Tenant),
          payment_providers: data.payment_providers || data.tenant.payment_providers || [],
        });
      }
      setStep("detail");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load membership details.");
    } finally {
      setLoadingDetail(false);
    }
  };

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
        <div className="card text-center max-w-md">
          <p className="text-lg font-medium text-red-600">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div
      className="min-h-screen bg-gray-50"
      style={{
        ["--brandPrimary" as any]: tenant?.primary_colour || "#6366f1",
        ["--brandSecondary" as any]: tenant?.secondary_colour || tenant?.primary_colour || "#4f46e5",
      }}
    >
      {/* Branded header */}
      {tenant && (
        <header className="bg-white border-b border-gray-200 px-6 sm:px-10 py-5">
          <div className="mx-auto max-w-6xl flex items-center gap-3">
            {tenant.logo_url ? (
              <img src={tenant.logo_url} alt="" className="h-10 w-auto" />
            ) : (
              <div
                className="flex h-10 w-10 items-center justify-center rounded-lg text-white font-bold"
                style={{ backgroundColor: "var(--brandPrimary)" }}
              >
                {tenant.name[0]}
              </div>
            )}
            <span className="text-lg font-semibold text-gray-900">{tenant.name}</span>
          </div>
        </header>
      )}

      <div className="mx-auto max-w-6xl px-6 sm:px-10 py-10">
        {step === "select" && (
          <MembershipSelect
            memberships={memberships}
            tenantSlug={tenantSlug || ""}
            priceDisplayMode={tenant?.signup_price_display_mode || "weekly"}
            onSelect={openMembershipDetail}
          />
        )}
        {loadingDetail && (
          <div className="mt-4 text-[13px] text-gray-500">Loading membership details...</div>
        )}
        {step === "detail" && selected && (
          <MembershipDetail
            membership={selected}
            tenantSlug={tenantSlug || ""}
            priceDisplayMode={tenant?.signup_price_display_mode || "weekly"}
            enabledPaymentFrequencies={tenant?.enabled_payment_frequencies || ["upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"]}
            paymentProviders={tenant?.payment_providers || []}
            onSelectTerm={(term) => { setSelectedTerm(term); setStep("form"); }}
            onBack={() => {
              if (membershipSlug) {
                // Came via direct link (/:tenantSlug/:membershipSlug) — navigate to browse page
                navigate(`/join/${tenantSlug}${location.search || ""}`);
              } else {
                // Already on browse route, just switch step
                setStep("select");
              }
            }}
          />
        )}
        {step === "form" && selected && selectedTerm && tenant && (
          <AgreementForm
            tenant={tenant}
            org={org}
            membership={selected}
            termOption={selectedTerm}
            tenantSlug={tenantSlug || ""}
            paymentProviders={tenant.payment_providers || []}
            // v2: prefer the per-term-option T&Cs when the admin attached any;
            // fall back to the type-level set for older memberships.
            terms={
              selectedTerm.attached_terms && selectedTerm.attached_terms.length > 0
                ? selectedTerm.attached_terms.map(a => ({
                    id: a.id,
                    title: a.title,
                    content: a.content,
                    version: a.version,
                    is_active: a.is_active,
                  }))
                : terms
            }
            inviteContext={inviteContext || undefined}
            inviteClient={inviteClient || undefined}
            onComplete={(ty) => { setThankyou(ty); setStep("thankyou"); }}
            onBack={() => setStep("detail")}
          />
        )}
        {step === "thankyou" && <ThankYouPage data={thankyou} />}
      </div>
    </div>
  );
}

export type { Tenant, MembershipType, TermOption, Bonus, TermsDoc, OrgPayload };
