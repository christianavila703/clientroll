import { useState } from "react";
import SignatureCanvas from "../signatures/SignatureCanvas";
import type { Tenant, MembershipType, TermOption } from "./SignupPage";

interface Props {
  tenant: Tenant;
  membership: MembershipType;
  termOption: TermOption;
  tenantSlug: string;
  paymentProviders: string[];
  onComplete: (thankyou: Record<string, unknown>) => void;
  onBack: () => void;
}

type FormStep = "member" | "payment" | "bank" | "sign";

export default function AgreementForm({ tenant, membership, termOption, tenantSlug, paymentProviders, onComplete, onBack }: Props) {
  const supportsStripe = paymentProviders.includes("stripe");
  const supportsGoCardless = paymentProviders.includes("gocardless");
  const defaultMethod = supportsGoCardless ? "direct_debit" : supportsStripe ? "card" : "direct_debit";
  const [formStep, setFormStep] = useState<FormStep>("member");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const [member, setMember] = useState({
    name: "", address: "", suburb: "", state: "", postcode: "",
    email: "", phone: "", birth_date: "", gender: "",
    emergency_contact_name: "", emergency_contact_phone: "",
  });
  const [payment, setPayment] = useState({
    payment_method: defaultMethod,
    payment_frequency: "monthly",
    start_date: new Date().toISOString().split("T")[0],
  });
  const [bank, setBank] = useState({
    institution_name: "", branch_location: "",
    account_holder_name: "", account_number: "", bsb: "",
  });
  const [memberSignature, setMemberSignature] = useState("");

  const usesBankStep = payment.payment_method !== "card";
  const steps: { key: FormStep; label: string; icon: string }[] = [
    { key: "member", label: "Your Details", icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" },
    { key: "payment", label: "Payment", icon: "M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" },
    { key: "sign", label: "Sign", icon: "M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 17.07a4.5 4.5 0 01-1.897 1.13L2.25 19.5l1.245-4.28a4.5 4.5 0 011.13-1.897L16.862 4.487z" },
  ];
  if (usesBankStep) {
    steps.splice(2, 0, { key: "bank", label: "Bank Details", icon: "M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75z" })
  }

  const currentIdx = steps.findIndex((s) => s.key === formStep);

  const handleSubmit = async () => {
    if (!memberSignature) { setError("Please provide your signature"); return; }
    setSubmitting(true); setError("");
    try {
      const isCustomSchedule = payment.payment_frequency === "custom_schedule";
      const paymentAmount = isCustomSchedule
        ? (termOption.custom_schedule_amount || termOption.fee_total)
        : (termOption[`fee_${payment.payment_frequency}` as keyof TermOption] || termOption.fee_total);
      const res = await fetch(`/api/signup/${tenantSlug}/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          client: { ...member, birth_date: member.birth_date || null },
          membership_type_id: membership.id, term_option_id: termOption.id,
          start_date: payment.start_date, payment_method: payment.payment_method,
          payment_frequency: payment.payment_frequency,
          payment_amount: paymentAmount,
          payment_interval_unit: isCustomSchedule ? termOption.custom_schedule_interval_unit : undefined,
          payment_interval_count: isCustomSchedule ? termOption.custom_schedule_interval_count : undefined,
          num_payments: isCustomSchedule ? termOption.custom_schedule_num_payments : termOption.num_payments,
          member_signature: memberSignature,
          ...(payment.payment_method === "direct_debit" ? bank : {}),
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
    } catch (err) { setError(err instanceof Error ? err.message : "Submission failed"); }
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
              const fmt = (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 2 });
              if (mode === "weekly") {
                const w = termOption.fee_weekly || toWeekly(termOption.fee_total, termOption.duration_months);
                if (w) return (<><p className="text-xl font-semibold tabular-nums" style={{ color: "var(--brandPrimary)" }}>${fmt(w)}</p><p className="text-[11px] text-gray-400">per week</p></>);
              }
              if (mode === "monthly") {
                const m = termOption.fee_monthly || toMonthly(termOption.fee_total, termOption.duration_months);
                if (m) return (<><p className="text-xl font-semibold tabular-nums" style={{ color: "var(--brandPrimary)" }}>${fmt(m)}</p><p className="text-[11px] text-gray-400">per month</p></>);
              }
              return (<><p className="text-xl font-semibold tabular-nums" style={{ color: "var(--brandPrimary)" }}>${termOption.fee_total.toLocaleString()}</p><p className="text-[11px] text-gray-400">total contract</p></>);
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

      {error && (
        <div className="mb-6 rounded-xl bg-red-50 border border-red-200 px-5 py-4 flex items-start gap-3">
          <svg className="h-5 w-5 text-red-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" /></svg>
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      {/* Step content */}
      <div className="rounded-2xl border border-gray-200 bg-white shadow-sm overflow-hidden">
        <div className="px-8 py-8 sm:px-10 sm:py-10">
          {formStep === "member" && <MemberSection data={member} onChange={setMember} onNext={() => setFormStep("payment")} />}
          {formStep === "payment" && <PaymentSection data={payment} onChange={setPayment} termOption={termOption} paymentProviders={paymentProviders} onNext={() => setFormStep(payment.payment_method === "card" ? "sign" : "bank")} onBack={() => setFormStep("member")} />}
          {formStep === "bank" && usesBankStep && <BankSection data={bank} onChange={setBank} tenant={tenant} onNext={() => setFormStep("sign")} onBack={() => setFormStep("payment")} />}
          {formStep === "sign" && <SignSection signature={memberSignature} onSign={setMemberSignature} onSubmit={handleSubmit} onBack={() => setFormStep(usesBankStep ? "bank" : "payment")} submitting={submitting} />}
        </div>
      </div>
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
function MemberSection({ data, onChange, onNext }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; onNext: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  const inputCls = "block w-full rounded-xl border border-gray-300 bg-white px-4 py-3 text-[14px] text-gray-900 shadow-sm placeholder:text-gray-400 focus:outline-none transition-all brand-focus";

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900">Your Details</h2>
        <p className="text-[14px] text-gray-500 mt-1">Please fill in your personal information below.</p>
      </div>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <Field label="Full Name" required full><input value={data.name} onChange={(e) => set("name", e.target.value)} className={inputCls} placeholder="John Smith" required /></Field>
        <Field label="Address" full><input value={data.address} onChange={(e) => set("address", e.target.value)} className={inputCls} placeholder="123 Main Street" /></Field>
        <Field label="Suburb"><input value={data.suburb} onChange={(e) => set("suburb", e.target.value)} className={inputCls} placeholder="Melbourne" /></Field>
        <Field label="State"><input value={data.state} onChange={(e) => set("state", e.target.value)} className={inputCls} placeholder="VIC" /></Field>
        <Field label="Postcode"><input value={data.postcode} onChange={(e) => set("postcode", e.target.value)} className={inputCls} placeholder="3000" /></Field>
        <Field label="Email" required><input type="email" value={data.email} onChange={(e) => set("email", e.target.value)} className={inputCls} placeholder="john@example.com" required /></Field>
        <Field label="Phone" required><input type="tel" value={data.phone} onChange={(e) => set("phone", e.target.value)} className={inputCls} placeholder="0400 000 000" required /></Field>
        <Field label="Date of Birth"><input type="date" value={data.birth_date} onChange={(e) => set("birth_date", e.target.value)} className={inputCls} /></Field>
        <Field label="Gender">
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
      <div className="mt-10 pt-8 border-t border-gray-100">
        <h3 className="text-[15px] font-semibold text-gray-900 mb-5">Emergency Contact</h3>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <Field label="Contact Name"><input value={data.emergency_contact_name} onChange={(e) => set("emergency_contact_name", e.target.value)} className={inputCls} placeholder="Jane Smith" /></Field>
          <Field label="Contact Phone"><input type="tel" value={data.emergency_contact_phone} onChange={(e) => set("emergency_contact_phone", e.target.value)} className={inputCls} placeholder="0400 000 000" /></Field>
        </div>
      </div>

      <NavButtons onNext={onNext} nextLabel="Payment Details" />
    </div>
  );
}

/* ─── Section B: Payment Details ─── */
function PaymentSection({ data, onChange, termOption, paymentProviders, onNext, onBack }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; termOption: TermOption; paymentProviders: string[]; onNext: () => void; onBack: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  const inputCls = "block w-full rounded-xl border border-gray-300 bg-white px-4 py-3 text-[14px] text-gray-900 shadow-sm placeholder:text-gray-400 focus:outline-none transition-all brand-focus";
  const supportsStripe = paymentProviders.includes("stripe");
  const supportsGoCardless = paymentProviders.includes("gocardless");

  const frequencies = [
    { value: "upfront", label: "Pay in full", desc: "Single payment in advance", icon: "💳", enabled: termOption.allow_upfront, amount: termOption.fee_total },
    { value: "weekly", label: "Weekly", desc: "Paid weekly in advance", icon: "📅", enabled: !!termOption.fee_weekly, amount: termOption.fee_weekly },
    { value: "fortnightly", label: "Fortnightly", desc: "Paid fortnightly in advance", icon: "📅", enabled: !!termOption.fee_fortnightly, amount: termOption.fee_fortnightly },
    { value: "monthly", label: "Monthly", desc: "Paid monthly in advance", icon: "📅", enabled: !!termOption.fee_monthly, amount: termOption.fee_monthly },
    { value: "quarterly", label: "Quarterly", desc: "Paid quarterly in advance", icon: "📅", enabled: !!(termOption as any).fee_quarterly, amount: (termOption as any).fee_quarterly },
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
  ].filter((f) => f.enabled);

  return (
    <div>
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900">Payment Details</h2>
        <p className="text-[14px] text-gray-500 mt-1">Choose when to start and how you'd like to pay.</p>
      </div>

      <div className="space-y-8">
        <div>
          <label className="block text-[13px] font-semibold text-gray-700 mb-3">Payment Method</label>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {supportsGoCardless && (
              <label className={`flex items-center gap-3 rounded-xl border-2 px-5 py-4 cursor-pointer transition-all ${
                data.payment_method === "direct_debit" ? "shadow-sm" : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
              }`} style={data.payment_method === "direct_debit" ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)" } : undefined}>
                <input type="radio" name="payment_method" value="direct_debit" checked={data.payment_method === "direct_debit"} onChange={(e) => set("payment_method", e.target.value)}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div>
                  <p className="text-[14px] font-semibold text-gray-900">Direct Debit</p>
                  <p className="text-[12px] text-gray-500">Pay from bank account (GoCardless)</p>
                </div>
              </label>
            )}
            {supportsStripe && (
              <label className={`flex items-center gap-3 rounded-xl border-2 px-5 py-4 cursor-pointer transition-all ${
                data.payment_method === "card" ? "shadow-sm" : "border-gray-200 hover:border-gray-300 hover:shadow-sm"
              }`} style={data.payment_method === "card" ? { borderColor: "var(--brandPrimary)", backgroundColor: "color-mix(in srgb, var(--brandPrimary) 6%, white)" } : undefined}>
                <input type="radio" name="payment_method" value="card" checked={data.payment_method === "card"} onChange={(e) => set("payment_method", e.target.value)}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div>
                  <p className="text-[14px] font-semibold text-gray-900">Credit / Debit Card</p>
                  <p className="text-[12px] text-gray-500">Secure Stripe checkout</p>
                </div>
              </label>
            )}
          </div>
        </div>

        <Field label="Start Date">
          <input type="date" value={data.start_date} onChange={(e) => set("start_date", e.target.value)} className={`${inputCls} max-w-xs`} />
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
                <input type="radio" name="frequency" value={f.value} checked={data.payment_frequency === f.value} onChange={(e) => set("payment_frequency", e.target.value)}
                  className="h-5 w-5 accent-[var(--brandPrimary)] border-gray-300" />
                <div className="flex-1">
                  <p className="text-[14px] font-semibold text-gray-900">{f.label}</p>
                  <p className="text-[12px] text-gray-500 mt-0.5">{f.desc}</p>
                </div>
                {f.amount && (
                  <span className="text-[15px] font-bold tabular-nums" style={{ color: data.payment_frequency === f.value ? "var(--brandPrimary)" : "#374151" }}>
                    ${f.amount.toLocaleString()}
                  </span>
                )}
              </label>
            ))}
          </div>
        </div>
      </div>

      <NavButtons onBack={onBack} onNext={onNext} nextLabel={data.payment_method === "card" ? "Sign Agreement" : "Bank Details"} />
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
        <h2 className="text-2xl font-bold text-gray-900">Bank Account Details</h2>
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

      <NavButtons onBack={onBack} onNext={onNext} nextLabel="Sign Agreement" />
    </div>
  );
}

/* ─── Section D: Signature ─── */
function SignSection({ signature, onSign, onSubmit, onBack, submitting }: { signature: string; onSign: (s: string) => void; onSubmit: () => void; onBack: () => void; submitting: boolean }) {
  return (
    <div>
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900">Authorisation</h2>
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
            Signature captured successfully
          </div>
        )}
      </div>

      <NavButtons onBack={onBack} onNext={onSubmit} nextLabel={submitting ? "Submitting..." : "Submit Agreement"} isSubmit disabled={submitting || !signature} />
    </div>
  );
}
