import { useEffect, useState } from "react";
import SignatureCanvas from "../signatures/SignatureCanvas";
import AddressAutocompleteInput from "../common/AddressAutocompleteInput";
import { estimateStripeProcessingFeeCents } from "../../utils/paymentLabels";
import type { Bonus, Tenant, MembershipType, TermOption, TermsDoc } from "./SignupPage";

interface Props {
  tenant: Tenant;
  membership: MembershipType;
  termOption: TermOption;
  tenantSlug: string;
  paymentProviders: string[];
  terms?: TermsDoc[];
  inviteContext?: {
    token: string;
    payment_method_strategy: "ask_client" | "use_existing";
    payment_method?: string | null;
    payment_method_id?: string | null;
    gocardless_mandate_id?: string | null;
    payment_frequency?: string;
    start_date?: string;
  };
  inviteClient?: Record<string, unknown>;
  onComplete: (thankyou: Record<string, unknown>) => void;
  onBack: () => void;
}

type FormStep = "member" | "payment" | "bank" | "terms" | "sign";
const DEFAULT_ENABLED_PAYMENT_FREQUENCIES = ["upfront", "weekly", "fortnightly", "monthly", "quarterly", "custom_schedule"];

function positiveNumber(value: unknown): number | null {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? n : null;
}

function fallbackWeeklyAmount(termOption: TermOption): number | null {
  const total = positiveNumber(termOption.fee_total);
  const months = Number(termOption.duration_months || 0);
  if (!total || !months) return null;
  const weeks = (months * 52) / 12;
  if (!weeks) return null;
  return total / weeks;
}

function fallbackFortnightlyAmount(termOption: TermOption): number | null {
  const weekly = fallbackWeeklyAmount(termOption);
  if (!weekly) return null;
  return weekly * 2;
}

function fallbackMonthlyAmount(termOption: TermOption): number | null {
  const total = positiveNumber(termOption.fee_total);
  const months = Number(termOption.duration_months || 0);
  if (!total || !months) return null;
  return total / months;
}

function fallbackQuarterlyAmount(termOption: TermOption): number | null {
  const total = positiveNumber(termOption.fee_total);
  const months = Number(termOption.duration_months || 0);
  if (!total || months < 3) return null;
  return total / (months / 3);
}

function resolveFrequencyAmount(termOption: TermOption, frequency: string): number {
  switch (frequency) {
    case "weekly":
      return positiveNumber(termOption.fee_weekly) || fallbackWeeklyAmount(termOption) || termOption.fee_total;
    case "fortnightly":
      return positiveNumber(termOption.fee_fortnightly) || fallbackFortnightlyAmount(termOption) || termOption.fee_total;
    case "monthly":
      return positiveNumber(termOption.fee_monthly) || fallbackMonthlyAmount(termOption) || termOption.fee_total;
    case "quarterly":
      return positiveNumber((termOption as any).fee_quarterly) || fallbackQuarterlyAmount(termOption) || termOption.fee_total;
    case "upfront":
      return termOption.fee_total;
    default:
      return termOption.fee_total;
  }
}

function inferCountryCodeFromCurrency(currencyCode: string): string {
  const cur = String(currencyCode || "").trim().toUpperCase();
  if (cur === "AUD") return "AU";
  if (cur === "USD") return "US";
  return "";
}

function hasText(value: unknown): boolean {
  return String(value || "").trim().length > 0;
}

export default function AgreementForm({ tenant, membership, termOption, tenantSlug, paymentProviders, terms = [], inviteContext, inviteClient, onComplete, onBack }: Props) {
  const normalizedProviders = (paymentProviders || []).map((p) => String(p).toLowerCase().trim());
  const hasProviderHints = normalizedProviders.length > 0;
  const supportsStripe = normalizedProviders.some((p) => p.includes("stripe")) || !hasProviderHints;
  const supportsDirectDebit = supportsStripe;
  const defaultMethod = supportsDirectDebit ? "direct_debit" : supportsStripe ? "card" : "direct_debit";
  const [formStep, setFormStep] = useState<FormStep>("member");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const defaultCountryCode = inferCountryCodeFromCurrency(String((membership as any).currency_code || "AUD"));

  const [member, setMember] = useState({
    name: "", unit_number: "", address: "", suburb: "", state: "", postcode: "", country: defaultCountryCode,
    email: "", phone: "", birth_date: "", gender: "",
    emergency_contact_name: "", emergency_contact_phone: "",
    tax_number: "", company_name: "", company_number: "", company_phone: "", company_website: "",
    company_address: "", company_city: "", company_state_region: "", company_postcode: "", company_country: defaultCountryCode,
  });
  const [companyAddressSameAsClient, setCompanyAddressSameAsClient] = useState(false);
  const [payment, setPayment] = useState({
    payment_method: inviteContext?.payment_method || defaultMethod,
    payment_frequency: inviteContext?.payment_frequency || "monthly",
    start_date: inviteContext?.start_date || new Date().toISOString().split("T")[0],
  });
  const [bank, setBank] = useState({
    institution_name: "", branch_location: "",
    account_holder_name: "", account_number: "", bsb: "",
  });
  const [termBonuses, setTermBonuses] = useState<Bonus[]>(
    Array.isArray(termOption.bonuses) ? termOption.bonuses : []
  );
  const [memberSignature, setMemberSignature] = useState("");
  const [acceptedTerms, setAcceptedTerms] = useState<Set<string>>(new Set());
  const [sendAgreementEmail, setSendAgreementEmail] = useState(true);
  const useExistingPayment = inviteContext?.payment_method_strategy === "use_existing";
  const paymentMethodLocked = useExistingPayment && !!inviteContext?.payment_method;
  const paymentFrequencyLocked = !!inviteContext?.payment_frequency;
  const startDateLocked = !!inviteContext?.start_date;
  const currencyCode = ((membership as any).currency_code || "AUD").toUpperCase();
  const effectiveEnabledPaymentFrequencies =
    membership.enabled_payment_frequencies && membership.enabled_payment_frequencies.length > 0
      ? membership.enabled_payment_frequencies
      : (tenant.enabled_payment_frequencies || DEFAULT_ENABLED_PAYMENT_FREQUENCIES);
  const effectiveFrequencySet = new Set((effectiveEnabledPaymentFrequencies || []).map((f) => String(f).toLowerCase()));

  const usesBankStep = false;
  const hasTerms = terms.length > 0;
  const steps: { key: FormStep; label: string; icon: string }[] = [
    { key: "member", label: "Your Details", icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" },
    { key: "payment", label: "Payment", icon: "M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" },
    { key: "sign", label: "Sign", icon: "M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 17.07a4.5 4.5 0 01-1.897 1.13L2.25 19.5l1.245-4.28a4.5 4.5 0 011.13-1.897L16.862 4.487z" },
  ];
  if (usesBankStep) {
    steps.splice(2, 0, { key: "bank", label: "Bank Details", icon: "M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75z" })
  }
  if (hasTerms) {
    const signIdx = steps.findIndex((s) => s.key === "sign");
    steps.splice(signIdx, 0, { key: "terms", label: "Terms", icon: "M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" });
  }

  const currentIdx = steps.findIndex((s) => s.key === formStep);

  useEffect(() => {
    if (!inviteClient) return;
    setMember((prev) => ({
      ...prev,
      name: String(inviteClient.name || prev.name || ""),
      unit_number: String(inviteClient.unit_number || prev.unit_number || ""),
      address: String(inviteClient.address || prev.address || ""),
      suburb: String(inviteClient.suburb || prev.suburb || ""),
      state: String(inviteClient.state || prev.state || ""),
      postcode: String(inviteClient.postcode || prev.postcode || ""),
      country: String(inviteClient.country || inviteClient.company_country || prev.country || defaultCountryCode),
      email: String(inviteClient.email || prev.email || ""),
      phone: String(inviteClient.phone || prev.phone || ""),
      birth_date: String(inviteClient.birth_date || "").split("T")[0] || prev.birth_date,
      gender: String(inviteClient.gender || prev.gender || ""),
      emergency_contact_name: String(inviteClient.emergency_contact_name || prev.emergency_contact_name || ""),
      emergency_contact_phone: String(inviteClient.emergency_contact_phone || prev.emergency_contact_phone || ""),
      tax_number: String(inviteClient.tax_number || prev.tax_number || ""),
      company_name: String(inviteClient.company_name || prev.company_name || ""),
      company_number: String(inviteClient.company_number || prev.company_number || ""),
      company_phone: String(inviteClient.company_phone || prev.company_phone || ""),
      company_website: String(inviteClient.company_website || prev.company_website || ""),
      company_address: String(inviteClient.company_address || prev.company_address || ""),
      company_city: String(inviteClient.company_city || prev.company_city || ""),
      company_state_region: String(inviteClient.company_state_region || prev.company_state_region || ""),
      company_postcode: String(inviteClient.company_postcode || prev.company_postcode || ""),
      company_country: String(inviteClient.company_country || inviteClient.country || prev.company_country || defaultCountryCode),
    }));
  }, [inviteClient, defaultCountryCode]);

  useEffect(() => {
    let active = true;
    const embedded = Array.isArray(termOption.bonuses) ? termOption.bonuses : [];
    if (embedded.length > 0) {
      setTermBonuses(embedded);
      return () => {
        active = false;
      };
    }
    if (!termOption?.id) {
      setTermBonuses([]);
      return () => {
        active = false;
      };
    }
    fetch(`/api/signup/${tenantSlug}/${membership.slug}/term/${termOption.id}/bonuses${window.location.search || ""}`)
      .then((r) => r.json())
      .then((data) => {
        if (!active) return;
        setTermBonuses(Array.isArray(data?.bonuses) ? data.bonuses : []);
      })
      .catch(() => {
        if (!active) return;
        setTermBonuses([]);
      });
    return () => {
      active = false;
    };
  }, [termOption, tenantSlug, membership.slug]);

  const handleSubmit = async () => {
    if (!memberSignature) {
      setError("Please provide your signature");
      setTimeout(() => document.getElementById("signup-error")?.scrollIntoView({ behavior: "smooth", block: "center" }), 100);
      return;
    }
    setSubmitting(true); setError("");
    try {
      const isCustomSchedule = payment.payment_frequency === "custom_schedule";
      const paymentAmount = isCustomSchedule
        ? (termOption.custom_schedule_amount || termOption.fee_total)
        : resolveFrequencyAmount(termOption, payment.payment_frequency);
      const submitParams = new URLSearchParams(window.location.search);
      if (inviteContext?.token) submitParams.set("invite", inviteContext.token);
      const submitQS = submitParams.toString();
      const res = await fetch(`/api/signup/${tenantSlug}/submit${submitQS ? `?${submitQS}` : ""}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          client: { ...member, birth_date: member.birth_date ? member.birth_date + "T00:00:00Z" : null },
          invite_token: inviteContext?.token || undefined,
          membership_type_id: membership.id, term_option_id: termOption.id,
          start_date: payment.start_date ? payment.start_date + "T00:00:00Z" : new Date().toISOString(),
          payment_method: payment.payment_method,
          payment_frequency: payment.payment_frequency,
          payment_amount: paymentAmount,
          payment_interval_unit: isCustomSchedule ? termOption.custom_schedule_interval_unit : undefined,
          payment_interval_count: isCustomSchedule ? termOption.custom_schedule_interval_count : undefined,
          num_payments: isCustomSchedule
            ? termOption.custom_schedule_num_payments
            : ((termOption as any).continue_after_term ? undefined : termOption.num_payments),
          member_signature: memberSignature,
          accepted_terms_ids: terms.length > 0 ? Array.from(acceptedTerms) : undefined,
          send_agreement_email: sendAgreementEmail,
          payment_method_id: useExistingPayment ? (inviteContext?.payment_method_id || undefined) : undefined,
          gocardless_mandate_id: useExistingPayment ? (inviteContext?.gocardless_mandate_id || undefined) : undefined,
          ...(payment.payment_method === "direct_debit" && usesBankStep ? bank : {}),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      if (data.checkout_url) {
        window.location.href = data.checkout_url;
        return;
      }
      const tyData = data.thankyou || {};
      if (data.gocardless_error) {
        tyData.gocardless_error = data.gocardless_error;
      }
      onComplete(tyData);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Submission failed";
      setError(msg);
      setTimeout(() => document.getElementById("signup-error")?.scrollIntoView({ behavior: "smooth", block: "center" }), 100);
    }
    finally { setSubmitting(false); }
  };

  return (
    <div className="max-w-3xl mx-auto">
      {/* Back link */}
      <button onClick={onBack} className="mb-8 text-sm font-medium flex items-center gap-1.5 group" style={{ color: "var(--brandPrimary)" }}>
        <svg className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" /></svg>
        Back to membership details
      </button>

      {/* Stepper */}
      <div className="mb-10">
        <div className="flex items-center justify-between">
          {steps.map((s, i) => (
            <div key={s.key} className="flex items-center gap-2">
              <div
                className={`flex h-9 w-9 items-center justify-center rounded-full text-sm font-semibold transition-all ${
                  i <= currentIdx ? "text-white" : "bg-gray-100 text-gray-400"
                }`}
                style={i <= currentIdx ? {
                  backgroundColor: "var(--brandPrimary)",
                  ...(i === currentIdx ? { boxShadow: "0 0 0 4px color-mix(in srgb, var(--brandPrimary) 15%, transparent)" } : {}),
                } : undefined}
              >
                {i < currentIdx ? (
                  <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                ) : (
                  <span>{i + 1}</span>
                )}
              </div>
              <span className={`text-[13px] font-medium hidden sm:block ${
                i <= currentIdx ? "text-gray-900" : "text-gray-400"
              }`}>{s.label}</span>
              {i < steps.length - 1 && (
                <div className="hidden sm:block h-px w-8 lg:w-16 mx-2" style={{ backgroundColor: i < currentIdx ? "var(--brandPrimary)" : "#e5e7eb" }} />
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Summary bar */}
      <div className="mb-8 rounded-2xl border px-6 py-5" style={{ backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)", borderColor: "color-mix(in srgb, var(--brandPrimary) 15%, transparent)" }}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl text-white flex-shrink-0" style={{ backgroundColor: "var(--brandPrimary)" }}>
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
            </div>
            <div>
              <p className="font-semibold text-gray-900">{(membership.membership_name || "").trim() || membership.title}</p>
              {(membership.membership_level || "").trim() && (
                <p className="text-[12px] font-medium" style={{ color: "var(--brandPrimary)" }}>{membership.membership_level}</p>
              )}
              <p className="text-[13px] text-gray-500">{termOption.duration_months} month term</p>
            </div>
          </div>
          <div className="text-right">
            {(() => {
              const mode = tenant.signup_price_display_mode || "total";
              const toWeekly = (total: number, months: number) => months ? total / ((months * 52) / 12) : null;
              const toMonthly = (total: number, months: number) => months ? total / months : null;
              const fmt = (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
              const weeklyValue = termOption.fee_weekly || toWeekly(termOption.fee_total, termOption.duration_months);
              const monthlyValue = termOption.fee_monthly || toMonthly(termOption.fee_total, termOption.duration_months);
              const resolvedMode =
                mode === "weekly" && effectiveFrequencySet.has("weekly") && weeklyValue
                  ? "weekly"
                  : mode === "monthly" && effectiveFrequencySet.has("monthly") && monthlyValue
                  ? "monthly"
                  : effectiveFrequencySet.has("monthly") && monthlyValue
                  ? "monthly"
                  : effectiveFrequencySet.has("weekly") && weeklyValue
                  ? "weekly"
                  : "total";
              if (resolvedMode === "weekly") {
                const w = termOption.fee_weekly || toWeekly(termOption.fee_total, termOption.duration_months);
                if (w) return (<><p className="text-[15px] font-semibold tabular-nums text-gray-900">{currencyCode} ${fmt(w)}</p><p className="text-[11px] text-gray-400">per week</p></>);
              }
              if (resolvedMode === "monthly") {
                const m = termOption.fee_monthly || toMonthly(termOption.fee_total, termOption.duration_months);
                if (m) return (<><p className="text-[15px] font-semibold tabular-nums text-gray-900">{currencyCode} ${fmt(m)}</p><p className="text-[11px] text-gray-400">per month</p></>);
              }
              return (<><p className="text-[15px] font-semibold tabular-nums text-gray-900">{currencyCode} ${termOption.fee_total.toLocaleString()}</p><p className="text-[11px] text-gray-400">total contract</p></>);
            })()}
          </div>
        </div>
        {((membership as any).summary || (membership as any).benefits) && (
          <div className="mt-4 pt-4 border-t" style={{ borderColor: "color-mix(in srgb, var(--brandPrimary) 12%, transparent)" }}>
            {(membership as any).summary && (
              <p className="text-[13px] text-gray-600 leading-relaxed">{(membership as any).summary}</p>
            )}
            {(membership as any).benefits && (
              <div className="mt-2 text-[12px] text-gray-500 leading-relaxed whitespace-pre-line">{(membership as any).benefits}</div>
            )}
          </div>
        )}
      </div>

      {formStep === "member" && (
        <MembershipOverview membership={membership} termOption={termOption} bonuses={termBonuses} currencyCode={currencyCode} />
      )}

      {error && (
        <div id="signup-error" className="mb-6 rounded-xl bg-red-50 border border-red-200 px-5 py-4 flex items-start gap-3">
          <svg className="h-5 w-5 text-red-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" /></svg>
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      {/* Step content */}
      <div className="rounded-2xl border border-gray-200 bg-white shadow-sm overflow-hidden">
        <div className="px-8 py-8 sm:px-10 sm:py-10">
          {formStep === "member" && (
            <MemberSection
              data={member}
              onChange={(next) => setMember(next as typeof member)}
              googlePlacesApiKey={tenant.google_places_api_key}
              askEmergencyContact={tenant.ask_emergency_contact !== false}
              askCompanyInformation={tenant.ask_company_information === true}
              companyAddressSameAsClient={companyAddressSameAsClient}
              onCompanyAddressSameAsClientChange={setCompanyAddressSameAsClient}
              onNext={() => setFormStep("payment")}
            />
          )}
          {formStep === "payment" && <PaymentSection data={payment} onChange={(next) => setPayment(next as typeof payment)} termOption={termOption} paymentProviders={paymentProviders} currencyCode={currencyCode} enabledPaymentFrequencies={effectiveEnabledPaymentFrequencies} onNext={() => setFormStep(usesBankStep ? "bank" : (hasTerms ? "terms" : "sign"))} onBack={() => setFormStep("member")} paymentMethodLocked={paymentMethodLocked} paymentFrequencyLocked={paymentFrequencyLocked} startDateLocked={startDateLocked} useExistingPayment={useExistingPayment} />}
          {formStep === "bank" && usesBankStep && <BankSection data={bank} onChange={(next) => setBank(next as typeof bank)} tenant={tenant} onNext={() => setFormStep(hasTerms ? "terms" : "sign")} onBack={() => setFormStep("payment")} />}
          {formStep === "terms" && hasTerms && <TermsSection terms={terms} accepted={acceptedTerms} onAcceptChange={setAcceptedTerms} onNext={() => setFormStep("sign")} onBack={() => setFormStep(usesBankStep ? "bank" : "payment")} />}
          {formStep === "sign" && <SignSection signature={memberSignature} onSign={setMemberSignature} onSubmit={handleSubmit} onBack={() => setFormStep(hasTerms ? "terms" : (usesBankStep ? "bank" : "payment"))} submitting={submitting} sendEmail={sendAgreementEmail} onSendEmailChange={setSendAgreementEmail} />}
        </div>
      </div>
    </div>
  );
}

function MembershipOverview({ membership, termOption, bonuses, currencyCode }: { membership: MembershipType; termOption: TermOption; bonuses: Bonus[]; currencyCode: string }) {
  const sections = [
    { label: "Summary", value: membership.summary },
    { label: "Description", value: membership.description },
    { label: "Inclusions", value: membership.inclusions },
    { label: "Benefits", value: membership.benefits },
  ].filter((s) => hasText(s.value));

  if (sections.length === 0 && bonuses.length === 0) return null;

  return (
    <div className="mb-8 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
      <h3 className="text-[16px] font-semibold text-gray-900">What&apos;s Included In This Membership</h3>
      <p className="mt-1 text-[13px] text-gray-500">
        Review these details before continuing with your signup.
      </p>

      <div className="mt-5 space-y-5">
        {sections.map((section) => (
          <div key={section.label}>
            <h4 className="text-[12px] font-semibold uppercase tracking-wide text-gray-500">{section.label}</h4>
            <div className="mt-1.5 whitespace-pre-wrap break-words text-[14px] leading-relaxed text-gray-800">
              {String(section.value || "")}
            </div>
          </div>
        ))}
      </div>

      {bonuses.length > 0 && (
        <div className="mt-6 border-t border-gray-200 pt-5">
          <h4 className="text-[12px] font-semibold uppercase tracking-wide text-gray-500">
            Bonuses ({termOption.duration_months}-Month Term)
          </h4>
          <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
            {bonuses.map((bonus) => (
              <div key={bonus.id} className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3">
                <p className="text-[14px] font-semibold text-gray-900">{bonus.title}</p>
                {hasText(bonus.description) && (
                  <p className="mt-1 text-[13px] leading-relaxed text-gray-700">{bonus.description}</p>
                )}
                <p className="mt-2 text-[12px] font-medium text-emerald-700">
                  Value: {currencyCode} ${Number(bonus.value || 0).toLocaleString()}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

/* ─── Reusable field component ─── */
function Field({ label, children, required, full }: { label: string; children: React.ReactNode; required?: boolean; full?: boolean }) {
  return (
    <div className={full ? "sm:col-span-2" : ""}>
      <label className="block text-[13px] font-semibold text-gray-700 mb-2">{label}{required && <span className="text-red-500 ml-0.5">*</span>}</label>
      {children}
    </div>
  );
}

/* ─── Navigation buttons ─── */
function NavButtons({ onBack, onNext, nextLabel, isSubmit, disabled }: { onBack?: () => void; onNext: () => void; nextLabel: string; isSubmit?: boolean; disabled?: boolean }) {
  return (
    <div className="mt-10 pt-6 border-t border-gray-100 flex justify-between">
      {onBack ? (
        <button onClick={onBack} className="inline-flex items-center gap-2 rounded-xl border border-gray-300 bg-white px-6 py-3 text-[14px] font-medium text-gray-700 shadow-sm hover:bg-gray-50 transition-all">
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" /></svg>
          Back
        </button>
      ) : <div />}
      <button
        onClick={onNext}
        disabled={disabled}
        className="inline-flex items-center gap-2 rounded-xl px-7 py-3 text-[14px] font-semibold shadow-sm transition-all text-white disabled:opacity-50"
        style={{ backgroundColor: isSubmit ? "#059669" : "var(--brandPrimary)" }}
      >
        {nextLabel}
        {!isSubmit && <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" /></svg>}
      </button>
    </div>
  );
}

/* ─── Section A: Member Details ─── */
function MemberSection({ data, onChange, onNext, googlePlacesApiKey, askEmergencyContact, askCompanyInformation, companyAddressSameAsClient, onCompanyAddressSameAsClientChange }: {
  data: Record<string, string>;
  onChange: (d: Record<string, string>) => void;
  onNext: () => void;
  googlePlacesApiKey?: string;
  askEmergencyContact: boolean;
  askCompanyInformation: boolean;
  companyAddressSameAsClient: boolean;
  onCompanyAddressSameAsClientChange: (next: boolean) => void;
}) {
  const personalStreetWithUnit = (unit: string, street: string) => {
    const u = (unit || "").trim();
    const s = (street || "").trim();
    if (!u) return s;
    if (!s) return `Unit ${u}`;
    return `Unit ${u}, ${s}`;
  };
  const set = (k: string, v: string) => {
    const next = { ...data, [k]: v };
    if (companyAddressSameAsClient) {
      if (k === "address" || k === "unit_number") next.company_address = personalStreetWithUnit(k === "unit_number" ? v : next.unit_number, k === "address" ? v : next.address);
      if (k === "suburb") next.company_city = v;
      if (k === "state") next.company_state_region = v;
      if (k === "postcode") next.company_postcode = v;
      if (k === "country") next.company_country = v;
    }
    onChange(next);
  };
  const inputCls = "block w-full rounded-xl border border-gray-300 bg-white px-4 py-3 text-[14px] text-gray-900 shadow-sm placeholder:text-gray-400 focus:outline-none transition-all brand-focus";

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-gray-900">Your Details</h2>
        <p className="text-[14px] text-gray-500 mt-1">Please fill in your personal information below.</p>
      </div>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <Field label="Full Name" required full><input value={data.name} onChange={(e) => set("name", e.target.value)} className={inputCls} placeholder="John Smith" required /></Field>
        <Field label="Street Address" required full>
          <AddressAutocompleteInput
            value={data.address}
            onChange={(v) => set("address", v)}
            onAddressSelect={({ unit_number, address, suburb, state, postcode }) => {
              const next: Record<string, string> = {
                ...data,
                unit_number: unit_number || data.unit_number,
                address: address || data.address,
                suburb: suburb || data.suburb,
                state: state || data.state,
                postcode: postcode || data.postcode,
              };
              if (companyAddressSameAsClient) {
                next.company_address = personalStreetWithUnit(unit_number || data.unit_number, address || data.address);
                next.company_city = suburb || data.suburb;
                next.company_state_region = state || data.state;
                next.company_postcode = postcode || data.postcode;
              }
              onChange(next);
            }}
            apiKey={googlePlacesApiKey}
            className={inputCls}
            placeholder="Start typing an address..."
          />
        </Field>
        <Field label="Unit / Apt Number"><input value={data.unit_number} onChange={(e) => set("unit_number", e.target.value)} className={inputCls} placeholder="Unit 4" /></Field>
        <Field label="Suburb" required><input value={data.suburb} onChange={(e) => set("suburb", e.target.value)} className={inputCls} placeholder="Melbourne" required /></Field>
        <Field label="State" required><input value={data.state} onChange={(e) => set("state", e.target.value)} className={inputCls} placeholder="VIC" required /></Field>
        <Field label="Postcode" required><input value={data.postcode} onChange={(e) => set("postcode", e.target.value)} className={inputCls} placeholder="3000" required /></Field>
        <Field label="Country" required><input value={data.country} onChange={(e) => set("country", e.target.value.toUpperCase())} className={inputCls} placeholder="AU or US" required maxLength={2} /></Field>
        <Field label="Email" required><input type="email" value={data.email} onChange={(e) => set("email", e.target.value)} className={inputCls} placeholder="john@example.com" required /></Field>
        <Field label="Phone" required><input type="tel" value={data.phone} onChange={(e) => set("phone", e.target.value)} className={inputCls} placeholder="0400 000 000" required /></Field>
        <Field label="Date of Birth" required><input type="date" value={data.birth_date} onChange={(e) => set("birth_date", e.target.value)} className={inputCls} required /></Field>
        <Field label="Gender" required>
          <div className="flex gap-4 mt-1">
            {["Female", "Male"].map((g) => (
              <label key={g} className={`flex items-center gap-2.5 rounded-xl border px-4 py-3 cursor-pointer transition-all min-h-[48px] ${
                data.gender === g ? "text-gray-900" : "border-gray-200 hover:border-gray-300 text-gray-700"
              }`} style={data.gender === g ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 8%, white)" } : undefined}>
                <input type="radio" name="gender" value={g} checked={data.gender === g} onChange={(e) => set("gender", e.target.value)} className="h-4 w-4 accent-[var(--brandPrimary)]" />
                <span className="text-[14px] font-medium">{g}</span>
              </label>
            ))}
          </div>
        </Field>
      </div>

      {/* Emergency contact section */}
      {askEmergencyContact && (
        <div className="mt-10 pt-8 border-t border-gray-100">
          <h3 className="text-[15px] font-semibold text-gray-900 mb-5">Emergency Contact</h3>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <Field label="Contact Name" required><input value={data.emergency_contact_name} onChange={(e) => set("emergency_contact_name", e.target.value)} className={inputCls} placeholder="Jane Smith" required /></Field>
            <Field label="Contact Phone" required><input type="tel" value={data.emergency_contact_phone} onChange={(e) => set("emergency_contact_phone", e.target.value)} className={inputCls} placeholder="0400 000 000" required /></Field>
          </div>
        </div>
      )}

      {askCompanyInformation && (
        <div className="mt-10 pt-8 border-t border-gray-100 space-y-6">
          <div>
            <h3 className="text-[15px] font-semibold text-gray-900 mb-5">Company Information</h3>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <Field label="Company Name" required><input value={data.company_name} onChange={(e) => set("company_name", e.target.value)} className={inputCls} placeholder="Company name" required /></Field>
              <Field label="Tax Number" required><input value={data.tax_number} onChange={(e) => set("tax_number", e.target.value)} className={inputCls} placeholder="Tax number" required /></Field>
              <Field label="Company Number" required><input value={data.company_number} onChange={(e) => set("company_number", e.target.value)} className={inputCls} placeholder="Company number" required /></Field>
              <Field label="Phone" required><input type="tel" value={data.company_phone} onChange={(e) => set("company_phone", e.target.value)} className={inputCls} placeholder="03 9000 0000" required /></Field>
              <Field label="Website" full><input type="url" value={data.company_website} onChange={(e) => set("company_website", e.target.value)} className={inputCls} placeholder="https://example.com" /></Field>
            </div>
          </div>

          <div>
            <h4 className="text-[14px] font-semibold text-gray-900 mb-3">Client Address</h4>
            <label className="inline-flex items-center gap-2 text-[13px] text-gray-700 mb-4">
              <input
                type="checkbox"
                checked={companyAddressSameAsClient}
                onChange={(e) => {
                  const checked = e.target.checked;
                  onCompanyAddressSameAsClientChange(checked);
                  if (checked) {
                    onChange({
                      ...data,
                      company_address: personalStreetWithUnit(data.unit_number, data.address),
                      company_city: data.suburb,
                      company_state_region: data.state,
                      company_postcode: data.postcode,
                      company_country: data.country,
                    });
                  }
                }}
                className="h-4 w-4 rounded border-gray-300 text-indigo-600"
              />
              Same as personal address
            </label>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <Field label="Address" required full><input value={data.company_address} onChange={(e) => set("company_address", e.target.value)} className={inputCls} placeholder="Company street address" required /></Field>
              <Field label="City" required><input value={data.company_city} onChange={(e) => set("company_city", e.target.value)} className={inputCls} placeholder="City" required /></Field>
              <Field label="State / Region" required><input value={data.company_state_region} onChange={(e) => set("company_state_region", e.target.value)} className={inputCls} placeholder="State or region" required /></Field>
              <Field label="ZIP / Postcode" required><input value={data.company_postcode} onChange={(e) => set("company_postcode", e.target.value)} className={inputCls} placeholder="Postcode" required /></Field>
              <Field label="Country" required full><input value={data.company_country} onChange={(e) => set("company_country", e.target.value.toUpperCase())} className={inputCls} placeholder="AU or US" required maxLength={2} /></Field>
            </div>
          </div>
        </div>
      )}

      <NavButtons onNext={() => {
        const required: [string, string][] = [
          ["name", "Full Name"], ["address", "Street Address"], ["suburb", "Suburb"],
          ["state", "State"], ["postcode", "Postcode"], ["country", "Country"], ["email", "Email"],
          ["phone", "Phone"], ["birth_date", "Date of Birth"], ["gender", "Gender"],
        ];
        if (askEmergencyContact) {
          required.push(["emergency_contact_name", "Emergency Contact Name"], ["emergency_contact_phone", "Emergency Contact Phone"]);
        }
        if (askCompanyInformation) {
          required.push(
            ["tax_number", "Tax Number"],
            ["company_name", "Company Name"],
            ["company_number", "Company Number"],
            ["company_phone", "Company Phone"],
            ["company_address", "Company Address"],
            ["company_city", "Company City"],
            ["company_state_region", "Company State / Region"],
            ["company_postcode", "Company ZIP / Postcode"],
            ["company_country", "Company Country"],
          );
        }
        const missing = required.filter(([k]) => !data[k]?.trim());
        if (missing.length) {
          alert("Please complete: " + missing.map(([, l]) => l).join(", "));
          return;
        }
        onNext();
      }} nextLabel="Payment Details" />
    </div>
  );
}

/* ─── Section B: Payment Details ─── */
function PaymentSection({ data, onChange, termOption, paymentProviders, currencyCode, enabledPaymentFrequencies, onNext, onBack, paymentMethodLocked, paymentFrequencyLocked, startDateLocked, useExistingPayment }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; termOption: TermOption; paymentProviders: string[]; currencyCode: string; enabledPaymentFrequencies: string[]; onNext: () => void; onBack: () => void; paymentMethodLocked: boolean; paymentFrequencyLocked: boolean; startDateLocked: boolean; useExistingPayment: boolean }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  const inputCls = "block w-full rounded-xl border border-gray-300 bg-white px-4 py-3 text-[14px] text-gray-900 shadow-sm placeholder:text-gray-400 focus:outline-none transition-all brand-focus";
  const supportsStripe = paymentProviders.includes("stripe");

  const weeklyAmount = positiveNumber(termOption.fee_weekly) || fallbackWeeklyAmount(termOption);
  const fortnightlyAmount = positiveNumber(termOption.fee_fortnightly) || fallbackFortnightlyAmount(termOption);
  const monthlyAmount = positiveNumber(termOption.fee_monthly) || fallbackMonthlyAmount(termOption);
  const quarterlyAmount = positiveNumber((termOption as any).fee_quarterly) || fallbackQuarterlyAmount(termOption);
  const enabledSet = new Set(
    (enabledPaymentFrequencies || DEFAULT_ENABLED_PAYMENT_FREQUENCIES).map((f) => String(f).toLowerCase())
  );

  const frequencies = [
    { value: "upfront", label: "Pay in full", desc: "Single payment in advance", icon: "💳", enabled: termOption.allow_upfront, amount: positiveNumber(termOption.fee_total) },
    { value: "weekly", label: "Weekly", desc: "Paid weekly in advance", icon: "📅", enabled: !!weeklyAmount, amount: weeklyAmount },
    { value: "fortnightly", label: "Fortnightly", desc: "Paid fortnightly in advance", icon: "📅", enabled: !!fortnightlyAmount, amount: fortnightlyAmount },
    { value: "monthly", label: "Monthly", desc: "Paid monthly in advance", icon: "📅", enabled: !!monthlyAmount, amount: monthlyAmount },
    { value: "quarterly", label: "Quarterly", desc: "Paid quarterly in advance", icon: "📅", enabled: !!quarterlyAmount, amount: quarterlyAmount },
    {
      value: "custom_schedule",
      label: termOption.custom_schedule_label || `${termOption.custom_schedule_num_payments || 0} instalments`,
      desc: termOption.custom_schedule_interval_count && termOption.custom_schedule_interval_unit
        ? `${termOption.custom_schedule_num_payments || 0} payments every ${termOption.custom_schedule_interval_count} ${termOption.custom_schedule_interval_unit}${termOption.custom_schedule_interval_count === 1 ? "" : "s"}`
        : "Custom instalment plan",
      icon: "📆",
      enabled: !!termOption.custom_schedule_enabled && !!termOption.custom_schedule_amount,
      amount: termOption.custom_schedule_amount,
    },
  ].filter((f) => f.enabled && enabledSet.has(f.value));

  const selectedFrequency = frequencies.find((f) => f.value === data.payment_frequency);
  const selectedAmount = selectedFrequency?.amount || 0;
  const computeFee = (amount: number): number => {
    if (!amount) return 0;
    if (data.payment_method === "card" && supportsStripe) {
      return estimateStripeProcessingFeeCents(Math.round(amount * 100), "card") / 100;
    }
    if (data.payment_method === "direct_debit" && supportsStripe) {
      return estimateStripeProcessingFeeCents(Math.round(amount * 100), "direct_debit") / 100;
    }
    return 0;
  };
  const selectedFee = computeFee(selectedAmount);
  const feeKnown = selectedFee > 0;
  const selectedTotal = selectedAmount + selectedFee;

  useEffect(() => {
    if (!frequencies.some((f) => f.value === data.payment_frequency) && frequencies[0]) {
      set("payment_frequency", frequencies[0].value);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data.payment_frequency, frequencies.map((f) => f.value).join("|")]);

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-gray-900">Payment Details</h2>
        <p className="text-[14px] text-gray-500 mt-1">Choose when to start and how you'd like to pay.</p>
      </div>

      <div className="space-y-8">
        <div>
          <label className="block text-[13px] font-semibold text-gray-700 mb-3">Payment Method</label>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {supportsStripe && (
              <label className={`flex items-center gap-3 rounded-xl border-2 px-5 py-4 cursor-pointer transition-all ${
                data.payment_method === "direct_debit" ? "shadow-sm" : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
              }`} style={data.payment_method === "direct_debit" ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)" } : undefined}>
                <input type="radio" name="payment_method" value="direct_debit" checked={data.payment_method === "direct_debit"} onChange={(e) => set("payment_method", e.target.value)} disabled={paymentMethodLocked}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div>
                  <p className="text-[14px] font-semibold text-gray-900">Direct Debit</p>
                  <p className="text-[12px] text-gray-500">Pay from your bank account</p>
                </div>
              </label>
            )}
            {supportsStripe && (
              <label className={`flex items-center gap-3 rounded-xl border-2 px-5 py-4 cursor-pointer transition-all ${
                data.payment_method === "card" ? "shadow-sm" : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
              }`} style={data.payment_method === "card" ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)" } : undefined}>
                <input type="radio" name="payment_method" value="card" checked={data.payment_method === "card"} onChange={(e) => set("payment_method", e.target.value)} disabled={paymentMethodLocked}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div>
                  <p className="text-[14px] font-semibold text-gray-900">Credit / Debit Card</p>
                  <p className="text-[12px] text-gray-500">Secure Stripe checkout</p>
                </div>
              </label>
            )}
          </div>
          {useExistingPayment && (
            <p className="mt-2 text-[12px] text-emerald-700">A saved payment method will be used for this membership.</p>
          )}
        </div>

        <Field label="Start Date">
          <input type="date" value={data.start_date} onChange={(e) => set("start_date", e.target.value)} className={`${inputCls} max-w-xs`} disabled={startDateLocked} />
        </Field>

        <div>
          <label className="block text-[13px] font-semibold text-gray-700 mb-3">Payment Frequency</label>
          <div className="space-y-3">
            {frequencies.map((f) => (
              <label key={f.value} className={`flex items-center gap-4 rounded-xl border-2 px-5 py-4 cursor-pointer transition-all ${
                data.payment_frequency === f.value
                  ? "shadow-sm"
                  : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
              }`} style={data.payment_frequency === f.value ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)" } : undefined}>
                <input type="radio" name="frequency" value={f.value} checked={data.payment_frequency === f.value} onChange={(e) => set("payment_frequency", e.target.value)} disabled={paymentFrequencyLocked}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div className="flex-1">
                  <p className="text-[14px] font-semibold text-gray-900">{f.label}</p>
                  <p className="text-[12px] text-gray-500 mt-0.5">{f.desc}</p>
                </div>
                {f.amount && (
                  <span className="text-[15px] font-bold tabular-nums" style={{ color: data.payment_frequency === f.value ? "var(--brandPrimary)" : "#374151" }}>
                    {currencyCode} ${f.amount.toLocaleString()}
                  </span>
                )}
              </label>
            ))}
          </div>
        </div>
        {selectedAmount > 0 && (
          <div className="rounded-xl border border-gray-200 bg-gray-50 p-4">
            <p className="text-[13px] font-semibold text-gray-800">Estimated charge</p>
            <div className="mt-2 space-y-1.5 text-[12px]">
              <div className="flex justify-between gap-3">
                <span className="text-gray-600">Membership amount</span>
                <span className="font-medium text-gray-900 tabular-nums">{currencyCode} ${selectedAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
              </div>
              {feeKnown ? (
                <div className="flex justify-between gap-3">
                  <span className="text-gray-600">Processing fee</span>
                  <span className="font-medium text-gray-900 tabular-nums">{currencyCode} ${selectedFee.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                </div>
              ) : (
                <div className="flex justify-between gap-3">
                  <span className="text-gray-600">Processing fee</span>
                  <span className="font-medium text-gray-900">+ payment processing fee</span>
                </div>
              )}
              <div className="flex justify-between gap-3 border-t border-gray-200 pt-1.5">
                <span className="text-gray-700 font-semibold">Total charged</span>
                {feeKnown ? (
                  <span className="font-semibold text-gray-900 tabular-nums">{currencyCode} ${selectedTotal.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : (
                  <span className="font-semibold text-gray-900">{currencyCode} ${selectedAmount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} + processing fee</span>
                )}
              </div>
            </div>
          </div>
        )}
      </div>

      <NavButtons onBack={onBack} onNext={onNext} nextLabel="Continue" />
    </div>
  );
}

/* ─── Section C: Bank Details ─── */
function BankSection({ data, onChange, tenant, onNext, onBack }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; tenant: Tenant; onNext: () => void; onBack: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  const inputCls = "block w-full rounded-xl border border-gray-300 bg-white px-4 py-3 text-[14px] text-gray-900 shadow-sm placeholder:text-gray-400 focus:outline-none transition-all brand-focus";

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-gray-900">Bank Account Details</h2>
        <p className="text-[14px] text-gray-500 mt-1">Your bank details for direct debit payments.</p>
      </div>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <Field label="Name of Institution"><input value={data.institution_name} onChange={(e) => set("institution_name", e.target.value)} className={inputCls} placeholder="Commonwealth Bank" /></Field>
        <Field label="Branch Location"><input value={data.branch_location} onChange={(e) => set("branch_location", e.target.value)} className={inputCls} placeholder="Melbourne CBD" /></Field>
        <Field label="Account Holder Name(s)" full><input value={data.account_holder_name} onChange={(e) => set("account_holder_name", e.target.value)} className={inputCls} placeholder="John Smith" /></Field>
        <Field label="BSB"><input value={data.bsb} onChange={(e) => set("bsb", e.target.value)} className={inputCls} placeholder="000-000" /></Field>
        <Field label="Account Number"><input value={data.account_number} onChange={(e) => set("account_number", e.target.value)} className={inputCls} placeholder="12345678" /></Field>
      </div>

      <div className="mt-8 rounded-xl bg-gray-50 border border-gray-200 p-5 flex items-start gap-3">
        <svg className="h-5 w-5 text-gray-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" /></svg>
        <p className="text-[13px] text-gray-600 leading-relaxed">
          I/we authorise <strong>{tenant.name}</strong> to debit my/our account as described in this agreement. Your details are encrypted and stored securely.
        </p>
      </div>

      <NavButtons onBack={onBack} onNext={onNext} nextLabel="Continue" />
    </div>
  );
}

/* ─── Section D: Terms & Conditions ─── */
function TermsSection({ terms, accepted, onAcceptChange, onNext, onBack }: {
  terms: TermsDoc[];
  accepted: Set<string>;
  onAcceptChange: (s: Set<string>) => void;
  onNext: () => void;
  onBack: () => void;
}) {
  const allAccepted = terms.every((t) => accepted.has(t.id));

  const toggleTerm = (id: string) => {
    const next = new Set(accepted);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onAcceptChange(next);
  };

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-gray-900">Terms & Conditions</h2>
        <p className="text-[14px] text-gray-500 mt-1">
          Please read and accept the following terms before signing.
        </p>
      </div>

      <div className="space-y-6">
        {terms.map((t) => (
          <div key={t.id} className="rounded-xl border border-gray-200 overflow-hidden">
            <div className="px-6 py-4 bg-gray-50 border-b border-gray-200">
              <h3 className="text-[15px] font-semibold text-gray-900">{t.title}</h3>
              {t.version > 1 && <span className="text-[11px] text-gray-400 ml-2">v{t.version}</span>}
            </div>
            <div
              className="px-6 py-5 max-h-[400px] overflow-y-auto text-[13px] text-gray-700 leading-relaxed whitespace-pre-wrap break-words tc-content"
              dangerouslySetInnerHTML={{ __html: t.content }}
            />
            <div className="px-6 py-4 bg-gray-50 border-t border-gray-200">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={accepted.has(t.id)}
                  onChange={() => toggleTerm(t.id)}
                  className="mt-0.5 h-5 w-5 rounded border-gray-300 accent-[var(--brandPrimary)]"
                />
                <span className="text-[13px] text-gray-700 font-medium">
                  I have read and agree to the <strong>{t.title}</strong>
                </span>
              </label>
            </div>
          </div>
        ))}
      </div>

      <NavButtons onBack={onBack} onNext={onNext} nextLabel="Sign Agreement" disabled={!allAccepted} />
    </div>
  );
}

/* ─── Section E: Signature ─── */
function SignSection({ signature, onSign, onSubmit, onBack, submitting, sendEmail, onSendEmailChange }: {
  signature: string; onSign: (s: string) => void; onSubmit: () => void; onBack: () => void; submitting: boolean;
  sendEmail: boolean; onSendEmailChange: (v: boolean) => void;
}) {
  return (
    <div>
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-gray-900">Authorisation</h2>
        <p className="text-[14px] text-gray-500 mt-1">
          Please review and sign to complete your membership agreement.
        </p>
      </div>

      <div className="rounded-xl bg-amber-50 border border-amber-200 p-5 mb-8 flex items-start gap-3">
        <svg className="h-5 w-5 text-amber-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
        <p className="text-[13px] text-amber-800 leading-relaxed">
          By signing below, I/we confirm that I/we have read and understood this Agreement in full,
          and agree to be bound by its terms and conditions.
        </p>
      </div>

      <div className="mb-6">
        <label className="block text-[13px] font-semibold text-gray-700 mb-3">Member Signature</label>
        <div className="rounded-xl border-2 border-dashed border-gray-300 overflow-hidden">
          <SignatureCanvas onSave={onSign} />
        </div>
        {signature && (
          <div className="mt-3 flex items-center gap-2 text-[13px] text-emerald-600">
            <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
            Signature saved
          </div>
        )}
      </div>

      <div className="mb-8 rounded-xl border border-gray-200 bg-gray-50 p-5">
        <label className="flex items-start gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={sendEmail}
            onChange={(e) => onSendEmailChange(e.target.checked)}
            className="mt-0.5 h-5 w-5 rounded border-gray-300 accent-[var(--brandPrimary)]"
          />
          <div>
            <span className="text-[13px] font-medium text-gray-700">Send me a copy of this agreement by email</span>
            <p className="text-[11px] text-gray-400 mt-0.5">A signed copy of the full agreement will be emailed to you for your records.</p>
          </div>
        </label>
      </div>

      <NavButtons onBack={onBack} onNext={onSubmit} nextLabel={submitting ? "Submitting..." : "Submit Agreement"} isSubmit disabled={submitting || !signature} />
    </div>
  );
}
