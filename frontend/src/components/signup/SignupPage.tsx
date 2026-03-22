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
  const [step, setStep] = useState<Step>("select");
  const [thankyou, setThankyou] = useState<Record<string, unknown>>({});
  const [inviteContext, setInviteContext] = useState<Record<string, unknown> | null>(null);
  const [inviteClient, setInviteClient] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    loadData();
    // inviteToken must be included: invite finalise URL has no membershipSlug; reloading when token changes matters.
  }, [tenantSlug, membershipSlug, inviteToken, location.search]);

  const loadData = async () => {
    try {
      const params = new URLSearchParams(window.location.search);
      const stripeState = params.get("stripe");
      if (inviteToken) {
        const res = await fetch(`/api/signup/${tenantSlug}/invite/${inviteToken}${location.search}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setSelected(data.membership);
        setTerms(data.terms || []);
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
      if (stripeState === "success") {
        setThankyou({
          heading: "Payment complete",
          body: "Your membership and card payment were processed successfully.",
        });
        setStep("thankyou");
      } else if (stripeState === "cancelled") {
        setError("Card payment was cancelled. You can try again.");
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
            membership={selected}
            termOption={selectedTerm}
            tenantSlug={tenantSlug || ""}
            paymentProviders={tenant.payment_providers || []}
            terms={terms}
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

export type { Tenant, MembershipType, TermOption, Bonus, TermsDoc };
