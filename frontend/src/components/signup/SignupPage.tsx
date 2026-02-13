import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import MembershipSelect from "./MembershipSelect";
import MembershipDetail from "./MembershipDetail";
import AgreementForm from "./AgreementForm";
import ThankYouPage from "./ThankYouPage";

interface Tenant {
  name: string;
  slug: string;
  logo_url?: string;
  primary_colour: string;
}

interface MembershipType {
  id: string;
  title: string;
  slug: string;
  description?: string;
  inclusions?: string;
  top_image_url?: string;
  total_value: number;
  term_options: TermOption[];
}

interface TermOption {
  id: string;
  duration_months: number;
  fee_total: number;
  fee_weekly?: number;
  fee_fortnightly?: number;
  fee_monthly?: number;
  fee_quarterly?: number;
  allow_upfront: boolean;
  num_payments?: number;
  bonuses?: Bonus[];
}

interface Bonus {
  id: string;
  title: string;
  description?: string;
  image_url?: string;
  value: number;
}

type Step = "select" | "detail" | "form" | "thankyou";

export default function SignupPage() {
  const { tenantSlug, membershipSlug } = useParams();
  const navigate = useNavigate();
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [memberships, setMemberships] = useState<MembershipType[]>([]);
  const [selected, setSelected] = useState<MembershipType | null>(null);
  const [selectedTerm, setSelectedTerm] = useState<TermOption | null>(null);
  const [step, setStep] = useState<Step>("select");
  const [thankyou, setThankyou] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    loadData();
  }, [tenantSlug, membershipSlug]);

  const loadData = async () => {
    try {
      if (membershipSlug) {
        // Direct link to specific membership
        const res = await fetch(`/api/signup/${tenantSlug}/${membershipSlug}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setSelected(data.membership);
        setStep("detail");
      } else {
        // Browse all memberships
        const res = await fetch(`/api/signup/${tenantSlug}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error);
        setTenant(data.tenant);
        setMemberships(data.memberships || []);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
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
    <div className="min-h-screen bg-gray-50">
      {/* Branded header */}
      {tenant && (
        <header className="bg-white border-b border-gray-200 px-6 sm:px-10 py-5">
          <div className="mx-auto max-w-6xl flex items-center gap-3">
            {tenant.logo_url ? (
              <img src={tenant.logo_url} alt="" className="h-10 w-auto" />
            ) : (
              <div
                className="flex h-10 w-10 items-center justify-center rounded-lg text-white font-bold"
                style={{ backgroundColor: tenant.primary_colour }}
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
            onSelect={(mt) => { setSelected(mt); setStep("detail"); }}
          />
        )}
        {step === "detail" && selected && (
          <MembershipDetail
            membership={selected}
            tenantSlug={tenantSlug || ""}
            onSelectTerm={(term) => { setSelectedTerm(term); setStep("form"); }}
            onBack={() => {
              if (membershipSlug) {
                // Came via direct link (/:tenantSlug/:membershipSlug) — navigate to browse page
                navigate(`/join/${tenantSlug}`);
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
            onComplete={(ty) => { setThankyou(ty); setStep("thankyou"); }}
            onBack={() => setStep("detail")}
          />
        )}
        {step === "thankyou" && <ThankYouPage data={thankyou} />}
      </div>
    </div>
  );
}

export type { Tenant, MembershipType, TermOption, Bonus };
