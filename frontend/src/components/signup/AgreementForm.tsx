import { useState } from "react";
import SignatureCanvas from "../signatures/SignatureCanvas";
import type { Tenant, MembershipType, TermOption } from "./SignupPage";

interface Props {
  tenant: Tenant;
  membership: MembershipType;
  termOption: TermOption;
  tenantSlug: string;
  onComplete: (thankyou: Record<string, unknown>) => void;
  onBack: () => void;
}

type FormStep = "member" | "payment" | "bank" | "sign";

export default function AgreementForm({ tenant, membership, termOption, tenantSlug, onComplete, onBack }: Props) {
  const [formStep, setFormStep] = useState<FormStep>("member");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  // Form data
  const [member, setMember] = useState({
    name: "", address: "", suburb: "", state: "", postcode: "",
    email: "", phone: "", birth_date: "", gender: "",
    emergency_contact_name: "", emergency_contact_phone: "",
  });
  const [payment, setPayment] = useState({
    payment_method: "direct_debit",
    payment_frequency: "monthly",
    start_date: new Date().toISOString().split("T")[0],
  });
  const [bank, setBank] = useState({
    institution_name: "", branch_location: "",
    account_holder_name: "", account_number: "", bsb: "",
  });
  const [memberSignature, setMemberSignature] = useState("");

  const steps: { key: FormStep; label: string }[] = [
    { key: "member", label: "Your Details" },
    { key: "payment", label: "Payment" },
    { key: "bank", label: "Bank Details" },
    { key: "sign", label: "Sign" },
  ];

  const currentIdx = steps.findIndex((s) => s.key === formStep);

  const handleSubmit = async () => {
    if (!memberSignature) {
      setError("Please provide your signature");
      return;
    }
    setSubmitting(true);
    setError("");

    try {
      const res = await fetch(`/api/signup/${tenantSlug}/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          client: { ...member, birth_date: member.birth_date || null },
          membership_type_id: membership.id,
          term_option_id: termOption.id,
          start_date: payment.start_date,
          payment_method: payment.payment_method,
          payment_frequency: payment.payment_frequency,
          payment_amount: termOption[`fee_${payment.payment_frequency}` as keyof TermOption] || termOption.fee_total,
          member_signature: memberSignature,
          ...bank,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      onComplete(data.thankyou || {});
    } catch (err) {
      setError(err instanceof Error ? err.message : "Submission failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <button onClick={onBack} className="mb-6 text-sm text-primary-600 hover:text-primary-700 font-medium">
        ← Back to membership details
      </button>

      {/* Progress */}
      <div className="mb-8">
        <div className="flex justify-between mb-2">
          {steps.map((s, i) => (
            <div key={s.key} className={`text-xs font-medium ${i <= currentIdx ? "text-primary-600" : "text-gray-400"}`}>
              {s.label}
            </div>
          ))}
        </div>
        <div className="h-2 bg-gray-200 rounded-full">
          <div
            className="h-2 bg-primary-600 rounded-full transition-all"
            style={{ width: `${((currentIdx + 1) / steps.length) * 100}%` }}
          />
        </div>
      </div>

      {/* Summary bar */}
      <div className="card mb-6 bg-primary-50 border-primary-200">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <span className="font-semibold text-gray-900">{membership.title}</span>
            <span className="text-gray-500 mx-2">·</span>
            <span className="text-gray-600">{termOption.duration_months} months</span>
          </div>
          <span className="text-lg font-bold text-primary-600">${termOption.fee_total.toLocaleString()}</span>
        </div>
      </div>

      {error && <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

      {/* Step content */}
      <div className="card">
        {formStep === "member" && (
          <MemberSection data={member} onChange={setMember} onNext={() => setFormStep("payment")} />
        )}
        {formStep === "payment" && (
          <PaymentSection data={payment} onChange={setPayment} termOption={termOption}
            onNext={() => setFormStep("bank")} onBack={() => setFormStep("member")} />
        )}
        {formStep === "bank" && (
          <BankSection data={bank} onChange={setBank} tenant={tenant}
            onNext={() => setFormStep("sign")} onBack={() => setFormStep("payment")} />
        )}
        {formStep === "sign" && (
          <SignSection signature={memberSignature} onSign={setMemberSignature}
            onSubmit={handleSubmit} onBack={() => setFormStep("bank")} submitting={submitting} />
        )}
      </div>
    </div>
  );
}

// Section B: Member Details
function MemberSection({ data, onChange, onNext }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; onNext: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  return (
    <div>
      <h2 className="text-xl font-bold text-gray-900 mb-6">Member Details</h2>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="sm:col-span-2"><label className="block text-sm font-medium text-gray-700 mb-1">Full Name *</label><input value={data.name} onChange={(e) => set("name", e.target.value)} className="input-field" required /></div>
        <div className="sm:col-span-2"><label className="block text-sm font-medium text-gray-700 mb-1">Address</label><input value={data.address} onChange={(e) => set("address", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Suburb</label><input value={data.suburb} onChange={(e) => set("suburb", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">State</label><input value={data.state} onChange={(e) => set("state", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Postcode</label><input value={data.postcode} onChange={(e) => set("postcode", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Email *</label><input type="email" value={data.email} onChange={(e) => set("email", e.target.value)} className="input-field" required /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Phone *</label><input type="tel" value={data.phone} onChange={(e) => set("phone", e.target.value)} className="input-field" required /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Date of Birth</label><input type="date" value={data.birth_date} onChange={(e) => set("birth_date", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Gender</label>
          <div className="flex gap-4 mt-2">
            {["Female", "Male"].map((g) => (<label key={g} className="flex items-center gap-2 min-h-[44px]"><input type="radio" name="gender" value={g} checked={data.gender === g} onChange={(e) => set("gender", e.target.value)} className="h-4 w-4 text-primary-600" />{g}</label>))}
          </div>
        </div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Emergency Contact Name</label><input value={data.emergency_contact_name} onChange={(e) => set("emergency_contact_name", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Emergency Contact Phone</label><input type="tel" value={data.emergency_contact_phone} onChange={(e) => set("emergency_contact_phone", e.target.value)} className="input-field" /></div>
      </div>
      <div className="mt-6 flex justify-end"><button onClick={onNext} className="btn-primary">Next: Payment Details</button></div>
    </div>
  );
}

// Section C: Payment Details
function PaymentSection({ data, onChange, termOption, onNext, onBack }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; termOption: TermOption; onNext: () => void; onBack: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  const frequencies = [
    { value: "upfront", label: "Upfront pre-payment", enabled: termOption.allow_upfront },
    { value: "weekly", label: "Weekly (in advance)", enabled: !!termOption.fee_weekly },
    { value: "fortnightly", label: "Fortnightly (in advance)", enabled: !!termOption.fee_fortnightly },
    { value: "monthly", label: "Monthly (in advance)", enabled: !!termOption.fee_monthly },
  ].filter((f) => f.enabled);

  return (
    <div>
      <h2 className="text-xl font-bold text-gray-900 mb-6">Payment Details</h2>
      <div className="space-y-4">
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label><input type="date" value={data.start_date} onChange={(e) => set("start_date", e.target.value)} className="input-field max-w-xs" /></div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">Payment Frequency</label>
          <div className="space-y-2">
            {frequencies.map((f) => (
              <label key={f.value} className="flex items-center gap-3 rounded-lg border border-gray-200 p-3 cursor-pointer hover:bg-gray-50 min-h-[44px]">
                <input type="radio" name="frequency" value={f.value} checked={data.payment_frequency === f.value} onChange={(e) => set("payment_frequency", e.target.value)} className="h-4 w-4 text-primary-600" />
                <span className="flex-1">{f.label}</span>
              </label>
            ))}
          </div>
        </div>
      </div>
      <div className="mt-6 flex justify-between">
        <button onClick={onBack} className="btn-secondary">Back</button>
        <button onClick={onNext} className="btn-primary">Next: Bank Details</button>
      </div>
    </div>
  );
}

// Section D: Bank Details
function BankSection({ data, onChange, tenant, onNext, onBack }: { data: Record<string, string>; onChange: (d: Record<string, string>) => void; tenant: Tenant; onNext: () => void; onBack: () => void }) {
  const set = (k: string, v: string) => onChange({ ...data, [k]: v });
  return (
    <div>
      <h2 className="text-xl font-bold text-gray-900 mb-2">Bank Account Details</h2>
      <p className="text-sm text-gray-500 mb-6">Please fill out your bank account details for direct debit.</p>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Name of Institution</label><input value={data.institution_name} onChange={(e) => set("institution_name", e.target.value)} className="input-field" placeholder='e.g. "Commonwealth Bank"' /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Branch Location</label><input value={data.branch_location} onChange={(e) => set("branch_location", e.target.value)} className="input-field" placeholder="Suburb where branch is located" /></div>
        <div className="sm:col-span-2"><label className="block text-sm font-medium text-gray-700 mb-1">Account Holder Name(s)</label><input value={data.account_holder_name} onChange={(e) => set("account_holder_name", e.target.value)} className="input-field" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">BSB</label><input value={data.bsb} onChange={(e) => set("bsb", e.target.value)} className="input-field" placeholder="000-000" /></div>
        <div><label className="block text-sm font-medium text-gray-700 mb-1">Account Number</label><input value={data.account_number} onChange={(e) => set("account_number", e.target.value)} className="input-field" /></div>
      </div>
      <div className="mt-6 rounded-lg bg-gray-50 p-4 text-sm text-gray-600">
        I/we authorise <strong>{tenant.name}</strong> to debit my/our account as described in this agreement.
      </div>
      <div className="mt-6 flex justify-between">
        <button onClick={onBack} className="btn-secondary">Back</button>
        <button onClick={onNext} className="btn-primary">Next: Sign Agreement</button>
      </div>
    </div>
  );
}

// Section E: Signature
function SignSection({ signature, onSign, onSubmit, onBack, submitting }: { signature: string; onSign: (s: string) => void; onSubmit: () => void; onBack: () => void; submitting: boolean }) {
  return (
    <div>
      <h2 className="text-xl font-bold text-gray-900 mb-2">Authorisation</h2>
      <p className="text-sm text-gray-600 mb-6">
        I/we have read through this Agreement in full and understand my/our obligations under this agreement.
        By signing below, I/we agree to be bound by the provisions of this Agreement.
      </p>
      <div className="mb-4">
        <label className="block text-sm font-medium text-gray-700 mb-2">Member Signature</label>
        <SignatureCanvas onSave={onSign} />
        {signature && <p className="mt-2 text-xs text-green-600">Signature captured</p>}
      </div>
      <div className="mt-8 flex justify-between">
        <button onClick={onBack} className="btn-secondary">Back</button>
        <button onClick={onSubmit} disabled={submitting || !signature} className="btn-primary text-base px-8 py-3">
          {submitting ? "Submitting..." : "Submit Agreement"}
        </button>
      </div>
    </div>
  );
}
