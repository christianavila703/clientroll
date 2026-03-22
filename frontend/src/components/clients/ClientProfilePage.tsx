import { useState, useEffect, useRef } from "react";
import { useParams, Link, useNavigate, useSearchParams } from "react-router-dom";
import api from "../../api/client";
import AdminPaymentAmounts from "../common/AdminPaymentAmounts";
import StripeTransactionPanel, { hasStripeDetails } from "../payments/StripeTransactionPanel";
import { organisationSubtotal, fmtMoney, customerCharged, estimateStripeProcessingFeeCents } from "../../utils/paymentLabels";
import type { MembershipRecord } from "../../types/memberships";
import type { PaymentRecord } from "../../types/payments";
import { useAuth } from "../../store/AuthContext";
import { daysUntilDateOnly, formatTenantDate, tenantTimezoneOrUTC } from "../../utils/tenantDate";

/* ─── Types ─── */
interface Client {
  id: string; name: string; email?: string; phone?: string;
  unit_number?: string; address?: string; suburb?: string; state?: string; postcode?: string; country?: string;
  birth_date?: string; gender?: string;
  emergency_contact_name?: string; emergency_contact_phone?: string;
  tax_number?: string;
  company_number?: string; company_phone?: string; company_website?: string;
  company_address?: string; company_city?: string; company_state_region?: string; company_postcode?: string; company_country?: string;
  created_at: string;
}

function membershipFrequencyLabel(m: MembershipRecord): string {
  if (!m.payment_frequency) return "—";
  if (m.payment_frequency !== "custom_schedule") return m.payment_frequency;
  const count = m.payment_interval_count || 1;
  const unit = (m.payment_interval_unit || "month") + (count === 1 ? "" : "s");
  if (m.num_payments && m.num_payments > 0) {
    return `${m.num_payments} payments every ${count} ${unit}`;
  }
  return `Every ${count} ${unit}`;
}

/* ─── Main Component ─── */
export default function ClientProfilePage() {
  const { clientId } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantTimezone = tenantTimezoneOrUTC(user?.tenant_timezone);
  const [searchParams, setSearchParams] = useSearchParams();
  const [client, setClient] = useState<Client | null>(null);
  const [memberships, setMemberships] = useState<MembershipRecord[]>([]);
  const [payments, setPayments] = useState<PaymentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"summary" | "memberships" | "billing" | "payments">("summary");
  const [showEdit, setShowEdit] = useState(false);
  const [showSetupMembership, setShowSetupMembership] = useState(false);
  const [showTakePayment, setShowTakePayment] = useState(false);
  const [showAddPaymentMethod, setShowAddPaymentMethod] = useState(false);
  const [mandateCompleteMessage, setMandateCompleteMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [subscriptionDetails, setSubscriptionDetails] = useState<Record<string, { next_charge_date: string; amount: number; status: string }>>({});
  const [tenantTax, setTenantTax] = useState<{ percent: number; name: string }>({ percent: 0, name: "GST" });
  const [clientFieldSettings, setClientFieldSettings] = useState<{ askEmergencyContact: boolean; askCompanyInformation: boolean }>({
    askEmergencyContact: true,
    askCompanyInformation: false,
  });
  const [paymentInfo, setPaymentInfo] = useState<{ hasMandate: boolean; providers: string[]; supportsDirectDebit: boolean }>({
    hasMandate: false,
    providers: [],
    supportsDirectDebit: false,
  });

  useEffect(() => {
    if (!clientId) return;
    loadAll();
  }, [clientId]);

  // Fetch GoCardless subscription details for memberships that have subscription_id (for billing tab)
  useEffect(() => {
    const active = memberships.filter(m => m.status === "active" && m.subscription_id);
    if (active.length === 0) {
      setSubscriptionDetails({});
      return;
    }
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    Promise.all(
      active.map(m =>
        fetch(`/api/tenant/gocardless/subscriptions/${m.subscription_id}`, { headers: h })
          .then(r => (r.ok ? r.json() : null) as Promise<{ NextChargeDate?: string; Amount?: number; Status?: string } | null>)
          .then(data => (data?.NextChargeDate ? { membership_id: m.membership_id, ...data } : null))
      )
    ).then(results => {
      const next: Record<string, { next_charge_date: string; amount: number; status: string }> = {};
      results.forEach(r => {
        if (r?.membership_id && r.NextChargeDate) {
          next[r.membership_id] = {
            next_charge_date: r.NextChargeDate,
            amount: (r.Amount ?? 0) / 100,
            status: r.Status ?? "",
          };
        }
      });
      setSubscriptionDetails(next);
    });
  }, [memberships]);

  // Auto-complete redirect flow when returning from GoCardless (redirect_flow_id + session_token in URL)
  useEffect(() => {
    if (!clientId) return;
    const flowId = searchParams.get("redirect_flow_id");
    const sessionToken = searchParams.get("session_token");
    if (!flowId || !sessionToken) return;

    const token = localStorage.getItem("access_token");
    fetch(`/api/tenant/clients/${clientId}/complete-mandate-link`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ flow_id: flowId, session_token: sessionToken }),
    })
      .then(async (res) => {
        if (res.ok) {
          setMandateCompleteMessage({ type: "success", text: "Direct debit set up successfully." });
          loadAll();
        } else {
          const d = await res.json().catch(() => ({}));
          setMandateCompleteMessage({ type: "error", text: d.error || "Unable to link mandate." });
        }
      })
      .catch(() => setMandateCompleteMessage({ type: "error", text: "Unable to link mandate." }))
      .finally(() => {
        setSearchParams({}, { replace: true });
      });
  }, [clientId, searchParams]);

  const loadAll = async () => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    try {
      const [clientRes, memberRes, payRes, chargesRes, mandateRes, tenantRes] = await Promise.all([
        fetch(`/api/tenant/clients/${clientId}`, { headers: h }).then(r => r.json()),
        fetch(`/api/tenant/clients/${clientId}/memberships`, { headers: h }).then(r => r.json()),
        fetch(`/api/tenant/payments/client/${clientId}`, { headers: h }).then(r => r.json()),
        api.listClientCharges(clientId!).catch(() => ({ charges: [] })) as Promise<any>,
        api.getClientMandateStatus(clientId!).catch(() => ({})) as Promise<any>,
        api.getTenantSettings().catch(() => ({})) as Promise<any>,
      ]);
      setClient(clientRes);
      setPaymentInfo({
        hasMandate: mandateRes?.has_mandate || false,
        providers: mandateRes?.payment_providers || [],
        supportsDirectDebit: !!mandateRes?.supports_direct_debit,
      });
      const txPct = Number(tenantRes?.sales_tax_percent);
      setTenantTax({
        percent: Number.isFinite(txPct) ? txPct : 0,
        name: String(tenantRes?.sales_tax_name || "GST"),
      });
      setClientFieldSettings({
        askEmergencyContact: tenantRes?.ask_emergency_contact !== false,
        askCompanyInformation: tenantRes?.ask_company_information === true,
      });
      const membershipsList = memberRes.memberships || [];
      setMemberships(membershipsList);
      const membershipByID = new Map<string, MembershipRecord>(
        membershipsList.map((m: MembershipRecord) => [m.membership_id, m])
      );

      const membershipPayments: PaymentRecord[] = (payRes.payments || []).map((p: any) => ({
        ...p, _type: "membership" as const,
        membership_type: p.membership_type || membershipByID.get(p.membership_id)?.membership_type,
        term_months: p.term_months || membershipByID.get(p.membership_id)?.term_months,
      }));
      const serviceCharges: PaymentRecord[] = ((chargesRes as any).charges || []).map((c: any) => ({
        id: c.id,
        amount: (c.amount || 0) + (c.tax_amount || 0) + (c.processing_fee_amount || 0),
        currency_code: c.currency_code || "AUD",
        base_amount: c.amount || 0,
        tax_amount: c.tax_amount || 0,
        processing_fee_amount: c.processing_fee_amount || 0,
        status: c.status || "succeeded",
        due_date: c.paid_date || c.created_at,
        paid_date: c.paid_date || c.created_at,
        provider_reference: c.provider_reference,
        payment_method: c.payment_method,
        stripe_details: c.stripe_details || undefined,
        _type: "service" as const,
        _description: c.description,
      }));
      const all = [...membershipPayments, ...serviceCharges].sort(
        (a, b) => new Date(b.due_date).getTime() - new Date(a.due_date).getTime()
      );
      setPayments(all);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  if (loading) {
    return (
      <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Loading...</div>
    );
  }
  if (!client) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Client not found</div>;

  const initials = client.name.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase();
  const activeMemberships = memberships.filter(m => m.status === "active");
  const hasCardMembership = memberships.some((m) => (m.payment_method || "").toLowerCase() === "card");
  const hasDirectDebitMembership = memberships.some((m) => (m.payment_method || "").toLowerCase() === "direct_debit");
  const hasStripe = paymentInfo.providers.includes("stripe");
  const hasGoCardless = paymentInfo.providers.includes("gocardless");
  const showDirectDebit = paymentInfo.hasMandate || hasDirectDebitMembership || hasGoCardless || paymentInfo.supportsDirectDebit;
  const showCardPayments = hasCardMembership || hasStripe;
  const hasActive = activeMemberships.length > 0;
  const succeededPayments = payments.filter(p => p.status === "succeeded");
  const failedPayments = payments.filter(p => p.status === "failed");
  const totalCustomerCharged = succeededPayments.reduce((sum, p) => sum + customerCharged(p), 0);
  const totalToOrganisation = succeededPayments.reduce((sum, p) => sum + organisationSubtotal(p), 0);
  const totalTaxAmount = succeededPayments.reduce((sum, p) => sum + (Number(p.tax_amount) || 0), 0);
  const totalProcessingFees = succeededPayments.reduce((sum, p) => sum + (Number(p.processing_fee_amount) || 0), 0);
  const totalBaseExTax = Math.max(0, totalToOrganisation - totalTaxAmount);
  const fmtDate = (d: string) => formatTenantDate(d, tenantTimezone, { day: "numeric", month: "long", year: "numeric" });
  const fmtShort = (d: string) => formatTenantDate(d, tenantTimezone, { day: "numeric", month: "short", year: "numeric" });

  const tabs = [
    { key: "summary" as const, label: "Summary" },
    { key: "memberships" as const, label: "Memberships" },
    { key: "billing" as const, label: "Billing schedule" },
    { key: "payments" as const, label: "Payments" },
  ];

  const streetLine = [client.unit_number ? `Unit ${client.unit_number},` : null, client.address].filter(Boolean).join(" ");
  const fullAddress = [streetLine, client.suburb, [client.state, client.postcode].filter(Boolean).join(" "), client.country].filter(Boolean).join("\n");

  return (
    <div>
      {/* ─── Breadcrumb header (Ignition-style) ─── */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-1">
        <div className="flex items-center gap-2 text-[15px]">
          <Link to="/clients" className="text-indigo-600 hover:text-indigo-700 font-semibold">Clients</Link>
          <span className="text-gray-300">›</span>
          <span className="font-bold text-gray-900 text-lg">{client.name.toUpperCase()}</span>
          <span className={`badge ${hasActive ? "badge-green" : "badge-gray"}`}>
            {hasActive ? "Active" : "Inactive"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <MoreActionsMenu
            clientId={client.id}
            clientName={client.name}
            onSetupMembership={() => setShowSetupMembership(true)}
            onTakePayment={() => setShowTakePayment(true)}
            onAddPaymentMethod={() => setShowAddPaymentMethod(true)}
          />
          <button onClick={() => setShowEdit(true)} className="btn-secondary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" /></svg>
            Edit client
          </button>
        </div>
      </div>

      {mandateCompleteMessage && (
        <div className={`mb-4 rounded-md px-4 py-3 text-[13px] ${mandateCompleteMessage.type === "success" ? "bg-emerald-50 border border-emerald-200 text-emerald-800" : "bg-red-50 border border-red-200 text-red-700"}`}>
          {mandateCompleteMessage.text}
        </div>
      )}

      {/* ─── Tabs ─── */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-4 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {/* ─── Tab Content ─── */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Left column — main content */}
        <div className="lg:col-span-2">
          {tab === "summary" && (
            <SummaryTab
              memberships={memberships}
              payments={payments}
              fmtShort={fmtShort}
              tenantTimezone={tenantTimezone}
              onViewAll={() => setTab("payments")}
            />
          )}
          {tab === "memberships" && (
            <MembershipsTab memberships={memberships} fmtShort={fmtShort} tenantTimezone={tenantTimezone} onRefresh={loadAll} />
          )}
          {tab === "billing" && (
            <BillingScheduleTab memberships={activeMemberships} subscriptionDetails={subscriptionDetails} fmtDate={fmtDate} tenantTaxPercent={tenantTax.percent} />
          )}
          {tab === "payments" && (
            <PaymentsTab
              payments={payments}
              fmtShort={fmtShort}
              clientName={client.name}
              clientEmail={client.email || ""}
            />
          )}
        </div>

        {/* Right column — sidebar */}
        <div className="space-y-5">
          {/* Client info card */}
          <div className="card overflow-hidden">
            <div className="px-5 py-4 flex items-start gap-3">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-indigo-100 text-indigo-700 text-sm font-bold flex-shrink-0">
                {initials}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-[15px] font-bold text-gray-900">{client.name.toUpperCase()}</p>
                  <button onClick={() => setShowEdit(true)} className="text-indigo-600 hover:text-indigo-700">
                    <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 17.07a4.5 4.5 0 01-1.897 1.13L2.25 19.5l1.245-4.28a4.5 4.5 0 011.13-1.897L16.862 4.487z" /></svg>
                  </button>
                </div>
                <p className="text-[12px] text-gray-400">CLI-{client.id.slice(0, 4).toUpperCase()}</p>
              </div>
            </div>

            <div className="border-t border-gray-100 px-5 py-3 space-y-3">
              {client.phone && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Phone</span>
                  <span className="text-[13px] text-gray-900">{client.phone}</span>
                </div>
              )}
              {client.email && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Email</span>
                  <span className="text-[13px] text-gray-900 truncate">{client.email}</span>
                </div>
              )}
              {fullAddress && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500 flex-shrink-0">Address</span>
                  <span className="text-[13px] text-gray-900 text-right whitespace-pre-line">{fullAddress}</span>
                </div>
              )}
              {client.gender && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Gender</span>
                  <span className="text-[13px] text-gray-900">{client.gender}</span>
                </div>
              )}
              {client.birth_date && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Date of birth</span>
                  <span className="text-[13px] text-gray-900">{fmtDate(client.birth_date)}</span>
                </div>
              )}
              {client.tax_number && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Tax number</span>
                  <span className="text-[13px] text-gray-900">{client.tax_number}</span>
                </div>
              )}
              {client.company_number && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Company number</span>
                  <span className="text-[13px] text-gray-900">{client.company_number}</span>
                </div>
              )}
              {client.company_phone && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Company phone</span>
                  <span className="text-[13px] text-gray-900">{client.company_phone}</span>
                </div>
              )}
              {client.company_website && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Company website</span>
                  <span className="text-[13px] text-gray-900 truncate">{client.company_website}</span>
                </div>
              )}
              {client.company_address && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Company address</span>
                  <span className="text-[13px] text-gray-900 text-right whitespace-pre-line">
                    {[client.company_address, [client.company_city, client.company_state_region].filter(Boolean).join(", "), client.company_postcode, client.company_country]
                      .filter(Boolean)
                      .join("\n")}
                  </span>
                </div>
              )}
            </div>

            {/* Emergency contact */}
            {clientFieldSettings.askEmergencyContact && client.emergency_contact_name && (
              <div className="border-t border-gray-100 px-5 py-3">
                <p className="text-[10px] uppercase tracking-wider text-gray-400 font-medium mb-2">Emergency Contact</p>
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-red-50 text-red-500 text-[11px] font-bold">
                    {client.emergency_contact_name.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase()}
                  </div>
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{client.emergency_contact_name}</p>
                    {client.emergency_contact_phone && (
                      <p className="text-[12px] text-gray-500">{client.emergency_contact_phone}</p>
                    )}
                  </div>
                </div>
              </div>
            )}

            <div className="border-t border-gray-100 px-5 py-2.5 text-[11px] text-gray-400">
              Member since {fmtShort(client.created_at)}
            </div>
          </div>

          {/* Payment summary card */}
          <div className="card overflow-hidden">
            <div className="px-5 py-3 border-b border-gray-100">
              <h3 className="text-[13px] font-semibold text-gray-900">Payment Summary</h3>
            </div>
            <div className="px-5 py-3 space-y-2.5">
              <div className="space-y-1.5 text-[13px]">
                <div className="flex justify-between gap-2">
                  <span className="text-gray-500">Payment</span>
                  <span className="font-medium text-gray-700 tabular-nums">${fmtMoney(totalBaseExTax)}</span>
                </div>
                <div className="flex justify-between gap-2 text-[12px]">
                  <span className="text-gray-500">Tax</span>
                  <span className="font-medium text-gray-700 tabular-nums">${fmtMoney(totalTaxAmount)}</span>
                </div>
                {totalProcessingFees > 0.004 && (
                  <div className="flex justify-between gap-2 text-[12px]">
                    <span className="text-gray-500">Payment processing fee</span>
                    <span className="font-medium text-gray-700 tabular-nums">${fmtMoney(totalProcessingFees)}</span>
                  </div>
                )}
                <div className="flex justify-between gap-2 border-t border-gray-100 pt-1.5">
                  <span className="text-gray-700 font-semibold">Total</span>
                  <span className="font-semibold text-gray-900 tabular-nums">${fmtMoney(totalCustomerCharged)}</span>
                </div>
              </div>
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Succeeded</span>
                <span className="text-emerald-600 font-medium">{succeededPayments.length}</span>
              </div>
              {failedPayments.length > 0 && (
                <div className="flex justify-between text-[13px]">
                  <span className="text-gray-500">Failed</span>
                  <span className="text-red-600 font-medium">{failedPayments.length}</span>
                </div>
              )}
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Active memberships</span>
                <span className="text-gray-900 font-medium">{activeMemberships.length}</span>
              </div>
            </div>
          </div>

          {/* Payment method card */}
          <div className="card overflow-hidden">
            <div className="px-5 py-3 border-b border-gray-100 flex items-center justify-between">
              <h3 className="text-[13px] font-semibold text-gray-900">Payment methods</h3>
            </div>
            <div className="px-5 py-3 space-y-2.5">
              {!showCardPayments && !showDirectDebit && (
                <p className="text-[12px] text-gray-400">No payment providers configured</p>
              )}
              {showDirectDebit && (
                <div className="flex items-center gap-2.5">
                  <div className="flex h-9 w-11 items-center justify-center rounded bg-green-50">
                    <svg className="h-5 w-5 text-green-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 0h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008z" /></svg>
                  </div>
                  <div>
                    <p className="text-[13px] font-medium text-gray-700">Direct Debit</p>
                    <p className={`text-[11px] ${paymentInfo.hasMandate ? "text-green-600" : "text-gray-500"}`}>
                      {paymentInfo.hasMandate
                        ? "Mandate active"
                        : hasDirectDebitMembership
                        ? "Configured on membership"
                        : "Available"}
                    </p>
                  </div>
                </div>
              )}
              {showCardPayments && (
                <div className="flex items-center gap-2.5">
                  <div className="flex h-9 w-11 items-center justify-center rounded bg-blue-50">
                    <svg className="h-5 w-5 text-blue-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" /></svg>
                  </div>
                  <div>
                    <p className="text-[13px] font-medium text-gray-700">Card Payments</p>
                    <p className={`text-[11px] ${hasCardMembership ? "text-blue-600" : "text-gray-500"}`}>
                      {hasCardMembership ? "Configured on membership" : "Stripe available"}
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Edit modal */}
      {showEdit && (
        <EditClientModal
          client={client}
          askEmergencyContact={clientFieldSettings.askEmergencyContact}
          askCompanyInformation={clientFieldSettings.askCompanyInformation}
          onClose={() => setShowEdit(false)}
          onSaved={() => { setShowEdit(false); loadAll(); }}
        />
      )}

      {/* Take Payment modal */}
      {showTakePayment && (
        <TakePaymentModal
          clientId={client.id}
          clientName={client.name}
          onClose={() => setShowTakePayment(false)}
          onCharged={() => { setShowTakePayment(false); loadAll(); }}
        />
      )}

      {/* Setup membership modal */}
      {showSetupMembership && (
        <SetupMembershipModal
          clientId={client.id}
          clientName={client.name}
          onClose={() => setShowSetupMembership(false)}
          onCreated={() => { setShowSetupMembership(false); loadAll(); }}
        />
      )}

      {/* Add payment method modal */}
      {showAddPaymentMethod && (
        <AddPaymentMethodModal
          clientId={client.id}
          clientName={client.name}
          onClose={() => setShowAddPaymentMethod(false)}
        />
      )}
    </div>
  );
}

/* ─── Summary Tab ─── */
function SummaryTab({ memberships, payments, fmtShort, tenantTimezone, onViewAll }: {
  memberships: MembershipRecord[]; payments: PaymentRecord[]; fmtShort: (d: string) => string; tenantTimezone: string; onViewAll: () => void;
}) {
  const active = memberships.filter(m => m.status === "active");
  const past = memberships.filter(m => m.status !== "active");

  return (
    <div className="space-y-5">
      {/* Active memberships */}
      <div className="card overflow-hidden">
        <div className="card-header">
          <h3>Active Memberships</h3>
          <span className="text-[12px] text-gray-400">{active.length} active</span>
        </div>
        {active.length === 0 ? (
          <div className="px-5 py-8 text-center">
            <svg className="mx-auto h-12 w-12 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
            <p className="mt-2 text-[13px] text-gray-500">No currently active memberships</p>
          </div>
        ) : (
          <div className="divide-y divide-gray-100">
            {active.map(m => (
              <MembershipSummaryRow key={m.membership_id} m={m} fmtShort={fmtShort} tenantTimezone={tenantTimezone} />
            ))}
          </div>
        )}
      </div>

      {/* Past memberships */}
      {past.length > 0 && (
        <div className="card overflow-hidden">
          <div className="card-header">
            <h3>Past Memberships</h3>
          </div>
          <div className="divide-y divide-gray-100">
            {past.map(m => (
              <MembershipSummaryRow key={m.membership_id} m={m} fmtShort={fmtShort} tenantTimezone={tenantTimezone} />
            ))}
          </div>
        </div>
      )}

      {/* Recent payments */}
      <div className="card overflow-hidden">
        <div className="card-header">
          <h3>Recent Payments</h3>
          <button onClick={onViewAll} className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium">View all</button>
        </div>
        {payments.length === 0 ? (
          <div className="px-5 py-8 text-center text-[13px] text-gray-400">No payments yet</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="data-table">
              <thead><tr><th>Date</th><th>Amount</th><th>Status</th></tr></thead>
              <tbody>
                {payments.slice(0, 5).map((p, i) => (
                  <tr key={i}>
                    <td className="text-[13px] text-gray-700">{fmtShort(p.due_date)}</td>
                    <td className="text-[13px] tabular-nums font-medium text-gray-900">{p.currency_code || "AUD"} ${customerCharged(p).toFixed(2)}</td>
                    <td>
                      <span className={`badge ${p.status === "succeeded" ? "badge-green" : p.status === "failed" ? "badge-red" : "badge-yellow"}`}>
                        {p.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

/* ─── Membership Summary Row ─── */
function MembershipSummaryRow({ m, fmtShort, tenantTimezone }: { m: MembershipRecord; fmtShort: (d: string) => string; tenantTimezone: string }) {
  const daysLeft = daysUntilDateOnly(m.end_date, tenantTimezone);
  const paidPct = m.total_expected > 0 ? Math.round((m.total_paid / m.total_expected) * 100) : 0;

  return (
    <div className="px-5 py-4">
      <div className="flex items-start justify-between mb-2">
        <div>
          <p className="text-[14px] font-semibold text-gray-900">{m.membership_type}</p>
          <p className="text-[12px] text-gray-500 mt-0.5">
            {m.term_months} months · {fmtShort(m.start_date)} — {fmtShort(m.end_date)}
          </p>
        </div>
        <span className={`badge ${m.status === "active" ? "badge-green" : m.status === "expired" ? "badge-gray" : "badge-red"}`}>
          {m.status}
        </span>
      </div>

      <div className="flex items-center gap-6 text-[12px]">
        {m.payment_amount && (
          <div>
            <span className="text-gray-400">Payment</span>
            <p className="text-gray-900 font-medium tabular-nums mt-0.5">
              ${m.payment_amount.toFixed(2)}<span className="text-gray-400 font-normal">/{membershipFrequencyLabel(m)}</span>
            </p>
          </div>
        )}
        <div>
          <span className="text-gray-400">Paid / Expected</span>
          <p className="text-gray-900 font-medium tabular-nums mt-0.5">
            ${m.total_paid.toLocaleString()} / ${m.total_expected.toLocaleString()}
          </p>
        </div>
        {m.status === "active" && daysLeft > 0 && (
          <div>
            <span className="text-gray-400">Remaining</span>
            <p className={`font-medium mt-0.5 ${daysLeft <= 30 ? "text-amber-600" : "text-gray-700"}`}>
              {daysLeft} days
            </p>
          </div>
        )}
        {m.failed_count > 0 && (
          <div>
            <span className="text-gray-400">Failed</span>
            <p className="text-red-600 font-medium mt-0.5">{m.failed_count}</p>
          </div>
        )}
      </div>

      {/* Progress bar */}
      <div className="mt-3">
        <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${paidPct >= 100 ? "bg-emerald-500" : paidPct >= 50 ? "bg-indigo-500" : "bg-amber-500"}`}
            style={{ width: `${Math.min(paidPct, 100)}%` }}
          />
        </div>
      </div>
    </div>
  );
}

/* ─── Memberships Tab (detailed cards) ─── */
function MembershipsTab({ memberships, fmtShort, tenantTimezone, onRefresh }: { memberships: MembershipRecord[]; fmtShort: (d: string) => string; tenantTimezone: string; onRefresh: () => void }) {
  const active = memberships.filter(m => m.status === "active");
  const inactive = memberships.filter(m => m.status !== "active");
  const [togglingMembershipID, setTogglingMembershipID] = useState<string | null>(null);

  const handleCancel = async (membershipId: string) => {
    if (!confirm("Cancel this membership? This will also cancel any GoCardless subscription and mandate.")) return;
    const token = localStorage.getItem("access_token");
    try {
      const res = await fetch(`/api/tenant/memberships/${membershipId}/cancel`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) { const d = await res.json(); alert(d.error || "Unable to cancel membership. Please try again."); return; }
      onRefresh();
    } catch { alert("Unable to cancel membership. Please try again."); }
  };

  const handleToggleContinueAfterTerm = async (membershipId: string, continueAfterTerm: boolean) => {
    setTogglingMembershipID(membershipId);
    try {
      await api.updateMembershipContinueAfterTerm(membershipId, continueAfterTerm);
      onRefresh();
    } catch (e: any) {
      alert(e?.message || "Unable to update billing continuation right now.");
    } finally {
      setTogglingMembershipID(null);
    }
  };

  if (memberships.length === 0) {
    return (
      <div className="card px-5 py-12 text-center">
        <svg className="mx-auto h-12 w-12 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
        <p className="mt-3 text-[13px] text-gray-500">No memberships yet</p>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      {active.length > 0 && (
        <div>
          <h2 className="text-[13px] font-semibold text-gray-700 mb-3">Active ({active.length})</h2>
          <div className="space-y-4">
            {active.map(m => <MembershipCard key={m.membership_id} m={m} fmtShort={fmtShort} tenantTimezone={tenantTimezone} onCancel={() => handleCancel(m.membership_id)} onToggleContinueAfterTerm={handleToggleContinueAfterTerm} toggleBusy={togglingMembershipID === m.membership_id} />)}
          </div>
        </div>
      )}
      {inactive.length > 0 && (
        <div>
          <h2 className="text-[13px] font-semibold text-gray-500 mb-3">Past ({inactive.length})</h2>
          <div className="space-y-4">
            {inactive.map(m => <MembershipCard key={m.membership_id} m={m} fmtShort={fmtShort} tenantTimezone={tenantTimezone} />)}
          </div>
        </div>
      )}
    </div>
  );
}

function MembershipCard({ m, fmtShort, tenantTimezone, onCancel, onToggleContinueAfterTerm, toggleBusy }: { m: MembershipRecord; fmtShort: (d: string) => string; tenantTimezone: string; onCancel?: () => void; onToggleContinueAfterTerm?: (membershipId: string, continueAfterTerm: boolean) => Promise<void> | void; toggleBusy?: boolean }) {
  const daysLeft = daysUntilDateOnly(m.end_date, tenantTimezone);
  const paidPct = m.total_expected > 0 ? Math.round((m.total_paid / m.total_expected) * 100) : 0;

  return (
    <div className="card overflow-hidden">
      {/* Colour stripe */}
      <div className={`h-1 ${m.status === "active" ? "bg-emerald-500" : m.status === "expired" ? "bg-gray-300" : "bg-red-400"}`} />
      <div className="px-5 py-4">
        <div className="flex items-start justify-between mb-3">
          <div>
            <p className="text-[11px] text-gray-400 mb-1">
              MEM-{m.membership_id.slice(0, 4).toUpperCase()} · {fmtShort(m.start_date)} — {fmtShort(m.end_date)}
            </p>
            <p className="text-[16px] font-bold text-gray-900">{m.membership_type}</p>
            <div className="flex items-center gap-2 mt-0.5">
              <p className="text-[13px] text-gray-500">{m.term_months}-month term</p>
              {m.subscription_id && (
                <span className="badge badge-purple">
                  <svg className="h-2.5 w-2.5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clipRule="evenodd" /></svg>
                  Auto DD
                </span>
              )}
              {m.mandate_id && !m.subscription_id && (
                <span className="badge badge-purple">
                  Mandate active
                </span>
              )}
            </div>
          </div>
          <div className="text-right">
            <p className="text-[20px] font-bold text-gray-900 tabular-nums">${m.payment_amount?.toFixed(2) || "0.00"}</p>
            <p className="text-[12px] text-gray-400">{membershipFrequencyLabel(m)}</p>
            <div className="flex items-center gap-2 mt-1">
              {m.has_agreement ? (
                <button
                  onClick={async () => {
                    try {
                      const token = localStorage.getItem("access_token");
                      const res = await fetch(`/api/tenant/memberships/${m.membership_id}/agreement.pdf`, {
                        headers: { Authorization: `Bearer ${token}` },
                      });
                      if (!res.ok) { alert("No signed agreement was found for this membership."); return; }
                      const blob = await res.blob();
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement("a");
                      a.href = url;
                      a.download = `Agreement-${m.membership_id.slice(0,8)}.pdf`;
                      document.body.appendChild(a);
                      a.click();
                      document.body.removeChild(a);
                      URL.revokeObjectURL(url);
                    } catch {
                      alert("Unable to download the agreement PDF right now.");
                    }
                  }}
                  className="text-[11px] text-indigo-500 hover:text-indigo-700"
                  title="Download signed agreement PDF"
                >Agreement PDF</button>
              ) : (
                <span className="text-[11px] text-gray-400" title="No signed agreement available for this membership">No Agreement</span>
              )}
              {m.status === "active" && onCancel && (
                <button onClick={onCancel} className="text-[11px] text-red-500 hover:text-red-700">Cancel</button>
              )}
            </div>
          </div>
        </div>

        {/* Stats row */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 py-3 border-t border-gray-100">
          <div>
            <p className="text-[11px] text-gray-400">Status</p>
            <span className={`badge mt-1 ${m.status === "active" ? "badge-green" : m.status === "expired" ? "badge-gray" : "badge-red"}`}>
              {m.status}
            </span>
          </div>
          <div>
            <p className="text-[11px] text-gray-400">Paid / Expected</p>
            <p className="text-[13px] font-medium text-gray-900 mt-1 tabular-nums">
              ${m.total_paid.toLocaleString()} / ${m.total_expected.toLocaleString()}
            </p>
          </div>
          <div>
            <p className="text-[11px] text-gray-400">Last Payment</p>
            <p className="text-[13px] text-gray-700 mt-1">
              {m.last_payment_date ? fmtShort(m.last_payment_date) : "—"}
              {m.last_payment_status && (
                <span className={`badge ml-1 ${
                  m.last_payment_status === "succeeded"
                    ? "badge-green"
                    : m.last_payment_status === "pending" || m.last_payment_status === "submitted" || m.last_payment_status === "disputed"
                      ? "badge-yellow"
                      : m.last_payment_status === "cancelled"
                        ? "badge-gray"
                        : "badge-red"
                }`}>
                  {m.last_payment_status}
                </span>
              )}
            </p>
          </div>
          <div>
            <p className="text-[11px] text-gray-400">
              {m.status === "active" ? "Remaining" : "Ended"}
            </p>
            <p className={`text-[13px] font-medium mt-1 ${
              m.status === "active" && daysLeft <= 30 ? "text-amber-600" : "text-gray-700"
            }`}>
              {m.status === "active" && daysLeft > 0 ? `${daysLeft} days` : fmtShort(m.end_date)}
            </p>
          </div>
        </div>

        {m.status === "active" && !!m.stripe_subscription_id && m.payment_frequency !== "custom_schedule" && onToggleContinueAfterTerm && (
          <div className="mt-2 border-t border-gray-100 pt-3">
            <label className="inline-flex items-center gap-2 text-[12px] text-gray-700">
              <input
                type="checkbox"
                checked={!!m.continue_after_term}
                disabled={!!toggleBusy}
                onChange={(e) => onToggleContinueAfterTerm(m.membership_id, e.target.checked)}
                className="h-4 w-4 rounded border-gray-300 text-indigo-600 disabled:opacity-50"
              />
              Continue billing after term
            </label>
          </div>
        )}

        {/* Progress */}
        <div className="mt-3">
          <div className="flex justify-between text-[11px] text-gray-400 mb-1">
            <span>{paidPct}% paid</span>
            {m.failed_count > 0 && <span className="text-red-500">{m.failed_count} failed</span>}
          </div>
          <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full ${paidPct >= 100 ? "bg-emerald-500" : paidPct >= 50 ? "bg-indigo-500" : "bg-amber-500"}`}
              style={{ width: `${Math.min(paidPct, 100)}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Billing Schedule Tab ─── */
function BillingScheduleTab({
  memberships,
  subscriptionDetails,
  fmtDate,
  tenantTaxPercent,
}: {
  memberships: MembershipRecord[];
  subscriptionDetails: Record<string, { next_charge_date: string; amount: number; status: string }>;
  fmtDate: (d: string) => string;
  tenantTaxPercent: number;
}) {
  const upcoming: { date: Date; membership: MembershipRecord; fromGoCardless?: boolean }[] = [];

  const toNumber = (v?: number) => (typeof v === "number" && Number.isFinite(v) ? v : 0);
  const paymentLikeForUpcoming = (membership: MembershipRecord, goCardlessAmount?: number) => {
    const lastAmount = membership.last_payment_amount;
    const fallbackAmount = membership.payment_amount;
    const hasLastSplit = typeof membership.last_base_amount === "number"
      && typeof membership.last_processing_fee_amount === "number"
      && membership.last_processing_fee_amount > 0.004;

    if (hasLastSplit) {
      let amount = typeof goCardlessAmount === "number" && Number.isFinite(goCardlessAmount)
        ? goCardlessAmount
        : (typeof lastAmount === "number" && Number.isFinite(lastAmount) ? lastAmount : toNumber(fallbackAmount));
      let base = membership.last_base_amount!;
      let tax = membership.last_tax_amount ?? 0;
      let fee = membership.last_processing_fee_amount!;
      if (typeof lastAmount === "number" && lastAmount > 0 && Math.abs(amount - lastAmount) > 0.009) {
        const scale = amount / lastAmount;
        base = base * scale;
        tax = tax * scale;
        fee = fee * scale;
      }
      return { amount, base_amount: base, tax_amount: tax, processing_fee_amount: fee, currency_code: membership.payment_currency_code || "AUD" };
    }

    const basePayment = toNumber(fallbackAmount);
    const method: "card" | "direct_debit" = (membership.payment_method || "").toLowerCase() === "direct_debit" ? "direct_debit" : "card";
    const taxRate = tenantTaxPercent > 0 ? tenantTaxPercent : 0;
    const taxAmount = taxRate > 0 ? Math.round(basePayment * 100 * taxRate / (100 + taxRate)) / 100 : 0;
    const baseExTax = Math.round((basePayment - taxAmount) * 100) / 100;
    const feeCents = estimateStripeProcessingFeeCents(Math.round(basePayment * 100), method);
    const fee = feeCents / 100;
    const totalCharged = Math.round((basePayment + fee) * 100) / 100;

    return {
      amount: totalCharged,
      base_amount: baseExTax,
      tax_amount: taxAmount,
      processing_fee_amount: fee,
      currency_code: membership.payment_currency_code || "AUD",
    };
  };

  memberships.forEach(m => {
    const subDetail = m.membership_id ? subscriptionDetails[m.membership_id] : undefined;
    if (subDetail?.next_charge_date) {
      const nextDate = new Date(subDetail.next_charge_date);
      if (!isNaN(nextDate.getTime())) {
        upcoming.push({ date: nextDate, membership: m, fromGoCardless: true });
      }
      return;
    }
    if (!m.payment_amount || !m.payment_frequency) return;
    const end = new Date(m.end_date);
    const now = new Date();
    let next = new Date(now);

    if (m.payment_frequency === "custom_schedule") {
      const count = m.payment_interval_count || 1;
      const maxPayments = m.num_payments && m.num_payments > 0 ? m.num_payments : 6;
      for (let i = 0; i < maxPayments && next <= end; i++) {
        upcoming.push({ date: new Date(next), membership: m });
        if (m.payment_interval_unit === "week") next.setDate(next.getDate() + 7 * count);
        else next.setMonth(next.getMonth() + count);
      }
      return;
    } else if (m.payment_frequency === "weekly") {
      next.setDate(next.getDate() + (7 - next.getDay()) % 7 || 7);
    } else if (m.payment_frequency === "fortnightly") {
      next.setDate(next.getDate() + 14 - (next.getDate() % 14));
    } else {
      next = new Date(now.getFullYear(), now.getMonth() + 1, 1);
    }

    for (let i = 0; i < 6 && next <= end; i++) {
      upcoming.push({ date: new Date(next), membership: m });
      if (m.payment_frequency === "weekly") next.setDate(next.getDate() + 7);
      else if (m.payment_frequency === "fortnightly") next.setDate(next.getDate() + 14);
      else next.setMonth(next.getMonth() + 1);
    }
  });

  upcoming.sort((a, b) => a.date.getTime() - b.date.getTime());

  const grouped: Record<string, typeof upcoming> = {};
  upcoming.forEach(u => {
    const key = u.date.toLocaleDateString("en-AU", { month: "long", year: "numeric" });
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(u);
  });

  if (memberships.length === 0) {
    return (
      <div className="card px-5 py-12 text-center">
        <svg className="mx-auto h-12 w-12 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
        <p className="mt-3 text-[13px] text-gray-500">No active memberships to schedule</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="text-[13px] font-semibold text-gray-700">Billed automatically</h2>
      {Object.entries(grouped).map(([month, items]) => (
        <div key={month} className="card overflow-hidden">
          <div className="bg-gray-50 px-5 py-2.5 border-b border-gray-200">
            <p className="text-[13px] font-medium text-gray-700">{month}</p>
          </div>
          <div className="divide-y divide-gray-100">
            {items.map((item, i) => (
              <div key={i} className="px-5 py-3">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{item.membership.membership_type}</p>
                    <p className="text-[12px] text-gray-500 capitalize">
                      {item.membership.payment_frequency} · Due {item.date.toLocaleDateString("en-AU", { day: "numeric", month: "long" })}
                      {item.fromGoCardless && <span className="ml-1.5 text-indigo-600">(DD)</span>}
                    </p>
                  </div>
                  <div className="text-right shrink-0 min-w-[12rem]">
                    <AdminPaymentAmounts
                      p={paymentLikeForUpcoming(
                        item.membership,
                        subscriptionDetails[item.membership.membership_id]?.amount
                      )}
                      layout="stacked"
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
      {upcoming.length === 0 && (
        <div className="card px-5 py-8 text-center text-[13px] text-gray-400">No upcoming payments scheduled</div>
      )}
    </div>
  );
}

/* ─── Payments Tab (full invoice-style history) ─── */
function PaymentsTab({
  payments,
  fmtShort,
  clientName,
  clientEmail,
}: {
  payments: PaymentRecord[];
  fmtShort: (d: string) => string;
  clientName: string;
  clientEmail: string;
}) {
  const [expandedPaymentKey, setExpandedPaymentKey] = useState<string | null>(null);

  if (payments.length === 0) {
    return (
      <div className="card px-5 py-12 text-center">
        <svg className="mx-auto h-12 w-12 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
        <p className="mt-3 text-[13px] text-gray-500">No payments yet</p>
      </div>
    );
  }

  // Group payments by month
  const grouped: Record<string, PaymentRecord[]> = {};
  payments.forEach(p => {
    const d = new Date(p.due_date);
    const key = d.toLocaleDateString("en-AU", { month: "long", year: "numeric" });
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(p);
  });

  const methodBadge = (p: PaymentRecord): { label: string; className: string } | null => {
    const explicitMethod = String(p.payment_method || "").toLowerCase();
    if (explicitMethod === "direct_debit") {
      return { label: "Direct Debit", className: "badge-purple" };
    }
    if (explicitMethod === "card") {
      return { label: "Card", className: "badge-blue" };
    }
    if (hasStripeDetails(p.stripe_details)) {
      const stripe = p.stripe_details as Record<string, unknown>;
      const pmType = String(stripe.payment_method_type || "").toLowerCase();
      const pmLabel = String(stripe.payment_method_label || "").toLowerCase();
      if (pmType === "card" || pmLabel.includes("card")) {
        return { label: "Card", className: "badge-blue" };
      }
      if (pmType.includes("debit") || pmType.includes("becs") || pmLabel.includes("direct debit")) {
        return { label: "Direct Debit", className: "badge-purple" };
      }
    }
    if (p.provider_reference) {
      return { label: "Direct Debit", className: "badge-purple" };
    }
    return null;
  };

  const paymentKeyFor = (p: PaymentRecord, idx: number) =>
    String(p.payment_id || p.id || `${p.due_date}-${p.amount}-${idx}`);

  return (
    <div className="space-y-4">
      {Object.entries(grouped).map(([month, items]) => (
        <div key={month} className="card overflow-hidden">
          <div className="bg-gray-50 px-5 py-2.5 border-b border-gray-200">
            <p className="text-[13px] font-medium text-gray-700">{month}</p>
          </div>
          <div className="divide-y divide-gray-100">
            {items.map((p, i) => {
              const rowKey = paymentKeyFor(p, i);
              const paymentMethodBadge = methodBadge(p);
              const canExpandStripe = hasStripeDetails(p.stripe_details);
              const isExpanded = expandedPaymentKey === rowKey;
              return (
              <div key={rowKey} className="px-5 py-3">
                <div className="flex items-start justify-between gap-4 mb-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[13px] font-medium text-gray-900">{fmtShort(p.due_date)}</span>
                    <span className={`badge ${
                      p.status === "succeeded" ? "badge-green" :
                      p.status === "confirmed" ? "badge-green" :
                      p.status === "submitted" ? "badge-yellow" :
                      p.status === "pending" ? "badge-yellow" :
                      p.status === "failed" ? "badge-red" :
                      p.status === "cancelled" ? "badge-gray" :
                      "badge-yellow"
                    }`}>
                      {p.status === "succeeded" ? "Paid" : p.status === "confirmed" ? "Confirmed" : p.status}
                    </span>
                    {p._type === "service" && (
                      <span className="badge badge-purple">Service</span>
                    )}
                    {paymentMethodBadge && (
                      <span className={`badge ${paymentMethodBadge.className}`}>
                        {paymentMethodBadge.label}
                      </span>
                    )}
                  </div>
                  <div className="text-right shrink-0 min-w-[10rem]">
                    <AdminPaymentAmounts p={p} layout="stacked" />
                    {canExpandStripe && (
                      <button
                        type="button"
                        className="mt-1 text-[11px] font-medium text-indigo-600 hover:text-indigo-800"
                        onClick={() => setExpandedPaymentKey(isExpanded ? null : rowKey)}
                      >
                        {isExpanded ? "Hide details" : "Show details"}
                      </button>
                    )}
                  </div>
                </div>
                {p._type === "service" && p._description && (
                  <p className="text-[11px] text-gray-500">{p._description}</p>
                )}
                {p._type !== "service" && (p.membership_type || p.membership_id) && (
                  <p className="text-[11px] text-gray-500">
                    {p.membership_type || "Membership payment"}
                    {typeof p.term_months === "number" && p.term_months > 0 ? ` · ${p.term_months}-month term` : ""}
                  </p>
                )}
                {p.paid_date && (p.status === "succeeded" || p.status === "confirmed") && (
                  <p className="text-[11px] text-emerald-600">
                    Payment completed ({fmtShort(p.paid_date)})
                    {p.provider_reference && <span className="text-gray-400 ml-1">Ref: {p.provider_reference}</span>}
                  </p>
                )}
                {p.status === "submitted" && (
                  <p className="text-[11px] text-amber-600">
                    Payment submitted to bank — awaiting confirmation
                  </p>
                )}
                {p.status === "failed" && (
                  <p className="text-[11px] text-red-500">
                    Payment failed{p.failure_reason ? `: ${p.failure_reason}` : ""}
                    {p.retry_count && p.retry_count > 0 ? ` (${p.retry_count} retries)` : ""}
                  </p>
                )}
                {p.status === "cancelled" && (
                  <p className="text-[11px] text-gray-500">
                    Payment cancelled
                  </p>
                )}
                {canExpandStripe && isExpanded && (
                  <div className="mt-3">
                    <StripeTransactionPanel
                      stripeDetails={p.stripe_details}
                      currencyCode={p.currency_code || "AUD"}
                      amount={customerCharged(p)}
                      status={p.status}
                      clientName={clientName}
                      clientEmail={clientEmail}
                    />
                  </div>
                )}
              </div>
            )})}
          </div>
        </div>
      ))}
    </div>
  );
}

function AddPaymentMethodModal({
  clientId,
  clientName,
  onClose,
}: {
  clientId: string;
  clientName: string;
  onClose: () => void;
}) {
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<string[]>([]);
  const [mode, setMode] = useState<"direct_debit" | "stripe">("direct_debit");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [mandateLink, setMandateLink] = useState("");

  useEffect(() => {
    api.getClientMandateStatus(clientId)
      .then((res) => {
        const r = res as any;
        const prov = (r.payment_providers || []) as string[];
        setProviders(prov);
        if (prov.includes("stripe") && !prov.includes("gocardless")) setMode("stripe");
        else setMode("direct_debit");
      })
      .catch(() => setProviders([]))
      .finally(() => setLoading(false));
  }, [clientId]);

  const hasGC = providers.includes("gocardless");
  const hasStripe = providers.includes("stripe");

  const handleAddDirectDebit = async () => {
    setSending(true); setError(""); setSuccess("");
    try {
      const data = await api.sendClientMandateLink(clientId) as any;
      setMandateLink(data.redirect_url || "");
      setSuccess(data.email_sent
        ? "Mandate setup link sent to the client."
        : "Mandate setup link created. Share it with the client.");
    } catch (e: any) {
      setError(e?.message || "Failed to create mandate link.");
    } finally {
      setSending(false);
    }
  };

  const handleAddStripe = async () => {
    setSending(true); setError(""); setSuccess("");
    try {
      const data = await api.createClientStripeSetupSession(clientId) as any;
      if (data.checkout_url) {
        window.open(data.checkout_url, "_blank");
        setSuccess("Stripe setup opened in a new tab. Client can add a card or bank account without charging.");
      } else {
        setError("Unable to open Stripe setup.");
      }
    } catch (e: any) {
      setError(e?.message || "Failed to create Stripe setup session.");
    } finally {
      setSending(false);
    }
  };

  const handleSubmit = () => {
    if (mode === "direct_debit") return handleAddDirectDebit();
    return handleAddStripe();
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />
      <div className="relative min-h-screen flex items-start justify-center pt-10 pb-10 px-4">
        <div className="relative w-full max-w-md bg-white rounded-xl shadow-2xl">
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
            <div>
              <h2 className="text-[15px] font-bold text-gray-900">Add payment method</h2>
              <p className="text-[12px] text-gray-500">for {clientName}</p>
            </div>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          {error && <div className="mx-6 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
          {success && <div className="mx-6 mt-4 rounded-md bg-green-50 border border-green-200 px-3 py-2 text-[13px] text-green-700">{success}</div>}
          {loading ? (
            <div className="px-6 py-10 text-center text-[13px] text-gray-500">Loading...</div>
          ) : (
            <div className="px-6 py-5 space-y-4">
              <p className="text-[12px] text-gray-500">Store a payment method now, without charging the client.</p>
              <div className="space-y-2">
                {hasGC && (
                  <button
                    type="button"
                    onClick={() => setMode("direct_debit")}
                    className={`w-full rounded-lg border-2 px-3 py-3 text-left text-[13px] font-medium ${mode === "direct_debit" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-700 hover:border-gray-300"}`}
                  >
                    Direct debit mandate (GoCardless)
                  </button>
                )}
                {hasStripe && (
                  <button
                    type="button"
                    onClick={() => setMode("stripe")}
                    className={`w-full rounded-lg border-2 px-3 py-3 text-left text-[13px] font-medium ${mode === "stripe" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-700 hover:border-gray-300"}`}
                  >
                    Card / bank setup (Stripe)
                  </button>
                )}
                {!hasGC && !hasStripe && (
                  <div className="rounded-md bg-gray-50 border border-gray-200 px-3 py-2 text-[12px] text-gray-600">
                    No payment provider configured. Connect GoCardless or Stripe in Settings.
                  </div>
                )}
              </div>
              {mode === "direct_debit" && hasGC && (
                <p className="text-[12px] text-gray-500">Creates a secure mandate link you can send to the client.</p>
              )}
              {mode === "stripe" && hasStripe && (
                <p className="text-[12px] text-gray-500">Opens Stripe setup checkout so the client can save a card or bank account.</p>
              )}
              {mandateLink && (
                <div className="rounded-md bg-gray-50 border border-gray-200 px-3 py-2">
                  <p className="text-[11px] text-gray-500 mb-1">Share this link:</p>
                  <div className="flex gap-2 items-center">
                    <input readOnly value={mandateLink} className="flex-1 text-[12px] bg-white border border-gray-200 rounded px-2 py-1.5 text-gray-700 select-all" onClick={e => (e.target as HTMLInputElement).select()} />
                    <button onClick={() => navigator.clipboard.writeText(mandateLink)} className="text-[11px] text-indigo-600 font-semibold hover:text-indigo-700 whitespace-nowrap">Copy</button>
                  </div>
                </div>
              )}
              <div className="flex justify-end gap-2 pt-2">
                <button onClick={onClose} className="btn-secondary text-[12px]">Close</button>
                <button
                  onClick={handleSubmit}
                  disabled={sending || (!hasGC && !hasStripe)}
                  className="btn-primary"
                >
                  {sending ? "Processing..." : mode === "stripe" ? "Open Stripe setup" : "Create mandate link"}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─── More Actions Dropdown ─── */
function MoreActionsMenu({
  clientId,
  clientName,
  onSetupMembership,
  onTakePayment,
  onAddPaymentMethod,
}: {
  clientId: string;
  clientName: string;
  onSetupMembership: () => void;
  onTakePayment: () => void;
  onAddPaymentMethod: () => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button onClick={() => setOpen(!open)} className="btn-secondary text-[12px]">
        More actions
        <svg className="h-3.5 w-3.5 ml-1" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" /></svg>
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 w-56 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50">
          <button onClick={() => { setOpen(false); onSetupMembership(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-indigo-700 hover:bg-indigo-50 font-medium">
            <svg className="h-3.5 w-3.5 text-indigo-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
            Set up membership
          </button>
          <button onClick={() => { setOpen(false); onTakePayment(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-emerald-700 hover:bg-emerald-50 font-medium">
            <svg className="h-3.5 w-3.5 text-emerald-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
            Take payment
          </button>
          <button onClick={() => { setOpen(false); onAddPaymentMethod(); }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-blue-700 hover:bg-blue-50 font-medium">
            <svg className="h-3.5 w-3.5 text-blue-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
            Add payment method
          </button>
          <div className="my-1 border-t border-gray-100" />
          <button onClick={() => setOpen(false)} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-gray-700 hover:bg-gray-50">
            <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" /></svg>
            Archive
          </button>
          <div className="my-1 border-t border-gray-100" />
          <button onClick={() => { setOpen(false); if (confirm(`Delete ${clientName}?`)) { /* delete */ } }} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-red-600 hover:bg-red-50">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" /></svg>
            Delete
          </button>
        </div>
      )}
    </div>
  );
}

/* ─── Edit Client Modal ─── */
function EditClientModal({
  client,
  askEmergencyContact,
  askCompanyInformation,
  onClose,
  onSaved,
}: {
  client: Client;
  askEmergencyContact: boolean;
  askCompanyInformation: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [tab, setTab] = useState<"details" | "company">("details");
  const [form, setForm] = useState({
    name: client.name, email: client.email || "", phone: client.phone || "",
    unit_number: client.unit_number || "", address: client.address || "", suburb: client.suburb || "", state: client.state || "",
    postcode: client.postcode || "", country: client.country || "", birth_date: client.birth_date?.split("T")[0] || "", gender: client.gender || "",
    emergency_contact_name: client.emergency_contact_name || "",
    emergency_contact_phone: client.emergency_contact_phone || "",
    tax_number: client.tax_number || "", company_number: client.company_number || "",
    company_phone: client.company_phone || "", company_website: client.company_website || "",
    company_address: client.company_address || "", company_city: client.company_city || "",
    company_state_region: client.company_state_region || "", company_postcode: client.company_postcode || "",
    company_country: client.company_country || "",
  });
  const [companyAddressSameAsClient, setCompanyAddressSameAsClient] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const formRef = useRef<HTMLFormElement | null>(null);

  const toBirthDateISOString = (value: string): string | null => {
    const trimmed = value.trim();
    if (!trimmed) return null;
    // Backend decodes into time.Time and expects RFC3339.
    const iso = `${trimmed}T00:00:00Z`;
    return Number.isNaN(Date.parse(iso)) ? null : iso;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError("");
    try {
      // Keep payload aligned with backend Client model.
      // Unknown fields are rejected server-side (DisallowUnknownFields).
      const payload: Record<string, string | null> = {
        name: form.name,
        email: form.email,
        phone: form.phone,
        gender: form.gender,
        unit_number: form.unit_number,
        address: form.address,
        suburb: form.suburb,
        state: form.state,
        postcode: form.postcode,
        country: form.country ? form.country.toUpperCase() : null,
        emergency_contact_name: askEmergencyContact ? form.emergency_contact_name : null,
        emergency_contact_phone: askEmergencyContact ? form.emergency_contact_phone : null,
        tax_number: askCompanyInformation ? form.tax_number : null,
        company_number: askCompanyInformation ? form.company_number : null,
        company_phone: askCompanyInformation ? form.company_phone : null,
        company_website: askCompanyInformation ? form.company_website : null,
        company_address: askCompanyInformation ? form.company_address : null,
        company_city: askCompanyInformation ? form.company_city : null,
        company_state_region: askCompanyInformation ? form.company_state_region : null,
        company_postcode: askCompanyInformation ? form.company_postcode : null,
        company_country: askCompanyInformation ? form.company_country : null,
      };
      const birthDateISO = toBirthDateISOString(form.birth_date);
      if (birthDateISO) payload.birth_date = birthDateISO;
      await api.updateClient(client.id, payload);
      onSaved();
      onClose();
    } catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />
      <div className="relative min-h-screen flex items-start justify-center pt-12 pb-12 px-4">
        <div className="relative w-full max-w-2xl bg-white rounded-xl shadow-2xl">
          {/* Header */}
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
            <h2 className="text-[15px] font-bold text-gray-900">Edit: {client.name}</h2>
            <div className="flex items-center gap-2">
              <button type="button" onClick={onClose} className="btn-secondary text-[12px]">Cancel</button>
              <button
                type="button"
                onClick={() => formRef.current?.requestSubmit()}
                disabled={saving}
                className="btn-primary"
              >
                <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200 px-6">
            {(askCompanyInformation ? (["details", "company"] as const) : (["details"] as const)).map((t) => (
              <button key={t} onClick={() => setTab(t)}
                className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
                  tab === t ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
                }`}>{t === "details" ? "Details" : "Company"}</button>
            ))}
          </div>

          {error && <div className="mx-6 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}

          <form ref={formRef} id="edit-client-form" onSubmit={handleSubmit} className="px-6 py-5 max-h-[60vh] overflow-y-auto">
            {tab === "details" && (
              <div className="space-y-5">
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Personal information</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <LabeledInput
                      label="Full name"
                      value={form.name}
                      onChange={(v) => setForm((prev) => ({ ...prev, name: v }))}
                      required
                      colSpan
                    />
                    <LabeledInput
                      label="Email"
                      value={form.email}
                      onChange={(v) => setForm((prev) => ({ ...prev, email: v }))}
                      type="email"
                    />
                    <LabeledInput
                      label="Phone"
                      value={form.phone}
                      onChange={(v) => setForm((prev) => ({ ...prev, phone: v }))}
                      type="tel"
                    />
                    <LabeledInput
                      label="Date of birth"
                      value={form.birth_date}
                      onChange={(v) => setForm((prev) => ({ ...prev, birth_date: v }))}
                      type="date"
                    />
                    <div>
                      <label className="block text-[13px] font-medium text-gray-700 mb-1">Gender</label>
                      <select value={form.gender} onChange={(e) => setForm((prev) => ({ ...prev, gender: e.target.value }))} className="input-field">
                        <option value="">—</option><option value="Female">Female</option><option value="Male">Male</option>
                      </select>
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Address</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <LabeledInput
                      label="Street address"
                      value={form.address}
                      onChange={(v) =>
                        setForm((prev) => ({
                          ...prev,
                          address: v,
                          ...(companyAddressSameAsClient ? { company_address: v } : {}),
                        }))
                      }
                      colSpan
                    />
                    <LabeledInput
                      label="Unit / Apt Number"
                      value={form.unit_number}
                      onChange={(v) => setForm((prev) => ({ ...prev, unit_number: v }))}
                    />
                    <LabeledInput
                      label="Suburb"
                      value={form.suburb}
                      onChange={(v) =>
                        setForm((prev) => ({
                          ...prev,
                          suburb: v,
                          ...(companyAddressSameAsClient ? { company_city: v } : {}),
                        }))
                      }
                    />
                    <LabeledInput
                      label="State"
                      value={form.state}
                      onChange={(v) =>
                        setForm((prev) => ({
                          ...prev,
                          state: v,
                          ...(companyAddressSameAsClient ? { company_state_region: v } : {}),
                        }))
                      }
                    />
                    <LabeledInput
                      label="Postcode"
                      value={form.postcode}
                      onChange={(v) =>
                        setForm((prev) => ({
                          ...prev,
                          postcode: v,
                          ...(companyAddressSameAsClient ? { company_postcode: v } : {}),
                        }))
                      }
                    />
                    <LabeledInput
                      label="Country"
                      value={form.country}
                      onChange={(v) =>
                        setForm((prev) => ({
                          ...prev,
                          country: v.toUpperCase(),
                          ...(companyAddressSameAsClient ? { company_country: v.toUpperCase() } : {}),
                        }))
                      }
                    />
                  </div>
                </div>
                {askEmergencyContact && (
                  <div>
                    <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Emergency contact</h4>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <LabeledInput
                        label="Contact name"
                        value={form.emergency_contact_name}
                        onChange={(v) => setForm((prev) => ({ ...prev, emergency_contact_name: v }))}
                      />
                      <LabeledInput
                        label="Contact phone"
                        value={form.emergency_contact_phone}
                        onChange={(v) => setForm((prev) => ({ ...prev, emergency_contact_phone: v }))}
                        type="tel"
                      />
                    </div>
                  </div>
                )}
              </div>
            )}
            {tab === "company" && askCompanyInformation && (
              <div className="space-y-5">
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Company information</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <LabeledInput label="Tax number" value={form.tax_number} onChange={(v) => setForm((prev) => ({ ...prev, tax_number: v }))} />
                    <LabeledInput label="Company number" value={form.company_number} onChange={(v) => setForm((prev) => ({ ...prev, company_number: v }))} />
                    <LabeledInput label="Phone" value={form.company_phone} onChange={(v) => setForm((prev) => ({ ...prev, company_phone: v }))} type="tel" />
                    <LabeledInput label="Website" value={form.company_website} onChange={(v) => setForm((prev) => ({ ...prev, company_website: v }))} placeholder="https://" colSpan />
                  </div>
                </div>
                <div>
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <h4 className="text-[13px] font-semibold text-gray-900">Client address</h4>
                    <label className="inline-flex items-center gap-2 text-[12px] text-gray-600">
                      <input
                        type="checkbox"
                        checked={companyAddressSameAsClient}
                        onChange={(e) => {
                          const checked = e.target.checked;
                          setCompanyAddressSameAsClient(checked);
                          if (checked) {
                            setForm((prev) => ({
                              ...prev,
                              company_address: prev.address,
                              company_city: prev.suburb,
                              company_state_region: prev.state,
                              company_postcode: prev.postcode,
                              company_country: prev.country,
                            }));
                          }
                        }}
                        className="h-4 w-4 rounded border-gray-300 text-indigo-600"
                      />
                      Same as personal address
                    </label>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <LabeledInput label="Street address" value={form.company_address} onChange={(v) => setForm((prev) => ({ ...prev, company_address: v }))} colSpan />
                    <LabeledInput label="City" value={form.company_city} onChange={(v) => setForm((prev) => ({ ...prev, company_city: v }))} />
                    <LabeledInput label="State / region" value={form.company_state_region} onChange={(v) => setForm((prev) => ({ ...prev, company_state_region: v }))} />
                    <LabeledInput label="ZIP / Postcode" value={form.company_postcode} onChange={(v) => setForm((prev) => ({ ...prev, company_postcode: v }))} />
                    <LabeledInput label="Country" value={form.company_country} onChange={(v) => setForm((prev) => ({ ...prev, company_country: v }))} colSpan />
                  </div>
                </div>
              </div>
            )}
          </form>
        </div>
      </div>
    </div>
  );
}

function LabeledInput({
  label,
  value,
  onChange,
  type = "text",
  placeholder = "",
  required = false,
  colSpan = false,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  required?: boolean;
  colSpan?: boolean;
}) {
  return (
    <div className={colSpan ? "sm:col-span-2" : ""}>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">
        {label}
        {required && <span className="text-red-500"> *</span>}
      </label>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="input-field"
        placeholder={placeholder}
        required={required}
      />
    </div>
  );
}

/* ─── Setup Membership Modal ─── */
interface MembershipTypeOption {
  id: string; title: string; slug: string; is_active: boolean;
  term_options?: {
    id: string; duration_months: number; fee_total: number;
    fee_weekly?: number; fee_fortnightly?: number; fee_monthly?: number; fee_quarterly?: number;
    allow_upfront: boolean; num_payments?: number;
    custom_schedule_enabled?: boolean;
    custom_schedule_label?: string;
    custom_schedule_amount?: number;
    custom_schedule_interval_unit?: "week" | "month";
    custom_schedule_interval_count?: number;
    custom_schedule_num_payments?: number;
    bonuses?: { id: string; title: string; value: number }[];
  }[];
}

function SetupMembershipModal({ clientId, clientName, onClose, onCreated }: {
  clientId: string; clientName: string; onClose: () => void; onCreated: () => void;
}) {
  const [step, setStep] = useState<"type" | "term" | "details" | "confirm">("type");
  const [flowMode, setFlowMode] = useState<"create_now" | "send_invite">("create_now");
  const [paymentStrategy, setPaymentStrategy] = useState<"ask_client" | "use_existing">("ask_client");
  const [types, setTypes] = useState<MembershipTypeOption[]>([]);
  const [selectedType, setSelectedType] = useState<MembershipTypeOption | null>(null);
  const [selectedTerm, setSelectedTerm] = useState<any>(null);
  const [frequency, setFrequency] = useState("monthly");
  const [startDate, setStartDate] = useState(new Date().toISOString().split("T")[0]);
  const [billingMethod, setBillingMethod] = useState<"direct_debit" | "card">("direct_debit");
  const [bsb, setBsb] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [accountHolderName, setAccountHolderName] = useState("");
  const [hasStoredMandate, setHasStoredMandate] = useState(false);
  const [storedMandateID, setStoredMandateID] = useState("");
  const [directDebitMode, setDirectDebitMode] = useState<"stripe_existing" | "gocardless_existing" | "new">("new");
  const [stripeCardMethods, setStripeCardMethods] = useState<Array<{ id: string; type: string; label: string; is_default?: boolean }>>([]);
  const [stripeDirectDebitMethods, setStripeDirectDebitMethods] = useState<Array<{ id: string; type: string; label: string; is_default?: boolean }>>([]);
  const [selectedStripeCardMethodId, setSelectedStripeCardMethodId] = useState("");
  const [selectedStripeDirectDebitMethodId, setSelectedStripeDirectDebitMethodId] = useState("");
  const [inviteURL, setInviteURL] = useState("");
  const [inviteWarning, setInviteWarning] = useState("");
  const [inviteEmailSent, setInviteEmailSent] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api.listMembershipTypes(),
      api.getClientMandateStatus(clientId).catch(() => ({})),
      api.listClientStripePaymentMethods(clientId).catch(() => ({ payment_methods: [] })),
    ]).then(([d, mandate, stripeMethodsData]) => {
      const membershipData = d as any;
      const active = (membershipData.membership_types || []).filter((mt: any) => mt.is_active);
      setTypes(active);
      // Load full details for each type
      active.forEach((mt: any) => {
        api.getMembershipType(mt.id)
          .then(full => {
            const fullType = full as any;
            setTypes(prev => prev.map(t => t.id === fullType.id ? fullType : t));
          });
      });
      const hasMandate = !!(mandate as any)?.has_mandate;
      const mandateID = String((mandate as any)?.mandate_id || "");
      const methods = Array.isArray((stripeMethodsData as any)?.payment_methods) ? (stripeMethodsData as any).payment_methods : [];
      const cardMethods = methods.filter((m: any) => m?.type === "card");
      const directDebitMethods = methods.filter((m: any) => m?.type === "au_becs_debit" || m?.type === "us_bank_account");
      const defaultCard = cardMethods.find((m: any) => m?.is_default) || cardMethods[0];
      const defaultDirectDebit = directDebitMethods.find((m: any) => m?.is_default) || directDebitMethods[0];
      setStripeCardMethods(cardMethods);
      setStripeDirectDebitMethods(directDebitMethods);
      setSelectedStripeCardMethodId(defaultCard?.id || "");
      setSelectedStripeDirectDebitMethodId(defaultDirectDebit?.id || "");
      setHasStoredMandate(hasMandate);
      setStoredMandateID(mandateID);
      setDirectDebitMode(defaultDirectDebit ? "stripe_existing" : (hasMandate ? "gocardless_existing" : "new"));
      setBillingMethod((defaultDirectDebit || hasMandate) ? "direct_debit" : (defaultCard ? "card" : "direct_debit"));
    });
  }, []);

  useEffect(() => {
    if (flowMode === "send_invite" && paymentStrategy === "use_existing" && billingMethod === "direct_debit" && directDebitMode === "new") {
      if (selectedStripeDirectDebitMethodId) {
        setDirectDebitMode("stripe_existing");
      } else if (storedMandateID) {
        setDirectDebitMode("gocardless_existing");
      }
    }
  }, [flowMode, paymentStrategy, billingMethod, directDebitMode, selectedStripeDirectDebitMethodId, storedMandateID]);

  const handleSubmit = async () => {
    if (!selectedType || !selectedTerm) return;
    if (flowMode === "create_now" && frequency !== "upfront" && billingMethod === "direct_debit" && directDebitMode === "new" && (!bsb.trim() || !accountNumber.trim())) {
      setError("BSB and account number are required when adding a new payment method.");
      return;
    }
    if (flowMode === "create_now" && frequency !== "upfront" && billingMethod === "card" && !selectedStripeCardMethodId) {
      setError("Select a saved Stripe card or switch to direct debit.");
      return;
    }
    if (flowMode === "create_now" && frequency !== "upfront" && billingMethod === "direct_debit" && directDebitMode === "stripe_existing" && !selectedStripeDirectDebitMethodId) {
      setError("Select a saved Stripe direct debit method or choose another direct debit option.");
      return;
    }
    if (flowMode === "create_now" && frequency !== "upfront" && billingMethod === "direct_debit" && directDebitMode === "gocardless_existing" && !storedMandateID) {
      setError("No saved GoCardless mandate is available for this client.");
      return;
    }
    if (flowMode === "send_invite" && paymentStrategy === "use_existing") {
      if (billingMethod === "card" && !selectedStripeCardMethodId) {
        setError("Select an existing Stripe card, or switch to 'Ask client for payment method'.");
        return;
      }
      if (billingMethod === "direct_debit") {
        if (directDebitMode === "stripe_existing" && !selectedStripeDirectDebitMethodId) {
          setError("Select an existing Stripe direct debit method, or switch direct debit source.");
          return;
        }
        if (directDebitMode === "gocardless_existing" && !storedMandateID) {
          setError("No existing GoCardless mandate found for this client.");
          return;
        }
        if (directDebitMode === "new") {
          setError("Use existing mode requires a saved Stripe direct debit method or GoCardless mandate.");
          return;
        }
      }
    }
    setSaving(true); setError("");
    try {
      const isCustomSchedule = frequency === "custom_schedule";
      const paymentAmount = isCustomSchedule
        ? (selectedTerm.custom_schedule_amount || selectedTerm.fee_total)
        : (selectedTerm[`fee_${frequency}`] || selectedTerm.fee_total);
      if (flowMode === "send_invite") {
        const invitePayload: Record<string, unknown> = {
          client_id: clientId,
          membership_type_id: selectedType.id,
          term_option_id: selectedTerm.id,
          start_date: startDate,
          payment_frequency: frequency,
          payment_method_strategy: paymentStrategy,
          payment_method: paymentStrategy === "use_existing" ? billingMethod : undefined,
          payment_method_id: paymentStrategy === "use_existing"
            ? (billingMethod === "card"
              ? selectedStripeCardMethodId
              : (directDebitMode === "stripe_existing" ? selectedStripeDirectDebitMethodId : undefined))
            : undefined,
          gocardless_mandate_id: paymentStrategy === "use_existing" && billingMethod === "direct_debit" && directDebitMode === "gocardless_existing" ? storedMandateID : undefined,
        };
        const data = await api.createMembershipInvite(invitePayload) as {
          invite_url?: string;
          warning?: string;
          email_sent?: boolean;
        };
        setInviteURL(data.invite_url || "");
        setInviteWarning(data.warning || "");
        setInviteEmailSent(!!data.email_sent);
        setStep("confirm");
      } else {
        const body: Record<string, unknown> = {
          client_id: clientId,
          membership_type_id: selectedType.id,
          term_option_id: selectedTerm.id,
          start_date: startDate,
          payment_method: billingMethod,
          payment_frequency: frequency,
          payment_amount: paymentAmount,
          payment_interval_unit: isCustomSchedule ? selectedTerm.custom_schedule_interval_unit : undefined,
          payment_interval_count: isCustomSchedule ? selectedTerm.custom_schedule_interval_count : undefined,
          num_payments: isCustomSchedule ? selectedTerm.custom_schedule_num_payments : selectedTerm.num_payments,
          payment_method_id: billingMethod === "card"
            ? selectedStripeCardMethodId
            : (billingMethod === "direct_debit" && directDebitMode === "stripe_existing" ? selectedStripeDirectDebitMethodId : undefined),
          gocardless_mandate_id: billingMethod === "direct_debit" && directDebitMode === "gocardless_existing" ? storedMandateID : undefined,
        };
        if (frequency !== "upfront" && billingMethod === "direct_debit" && directDebitMode === "new" && bsb.trim() && accountNumber.trim()) {
          body.bsb = bsb.trim();
          body.account_number = accountNumber.trim();
          if (accountHolderName.trim()) body.account_holder_name = accountHolderName.trim();
        }
        await api.createMembership(body);
        onCreated();
      }
    } catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  const frequencies = selectedTerm ? [
    { value: "upfront", label: "Pay in full", amount: selectedTerm.fee_total, enabled: selectedTerm.allow_upfront },
    { value: "weekly", label: "Weekly", amount: selectedTerm.fee_weekly, enabled: !!selectedTerm.fee_weekly },
    { value: "fortnightly", label: "Fortnightly", amount: selectedTerm.fee_fortnightly, enabled: !!selectedTerm.fee_fortnightly },
    { value: "monthly", label: "Monthly", amount: selectedTerm.fee_monthly, enabled: !!selectedTerm.fee_monthly },
    { value: "quarterly", label: "Quarterly", amount: selectedTerm.fee_quarterly, enabled: !!selectedTerm.fee_quarterly },
    {
      value: "custom_schedule",
      label: selectedTerm.custom_schedule_label || `${selectedTerm.custom_schedule_num_payments || 0} payments every ${selectedTerm.custom_schedule_interval_count || 1} ${selectedTerm.custom_schedule_interval_unit || "month"}${(selectedTerm.custom_schedule_interval_count || 1) === 1 ? "" : "s"}`,
      amount: selectedTerm.custom_schedule_amount,
      enabled: !!selectedTerm.custom_schedule_enabled && !!selectedTerm.custom_schedule_amount && !!selectedTerm.custom_schedule_num_payments && !!selectedTerm.custom_schedule_interval_count,
    },
  ].filter(f => f.enabled) : [];

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />
      <div className="relative min-h-screen flex items-start justify-center pt-10 pb-10 px-4">
        <div className="relative w-full max-w-2xl bg-white rounded-xl shadow-2xl">
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
            <div>
              <h2 className="text-[15px] font-bold text-gray-900">Set up membership</h2>
              <p className="text-[12px] text-gray-500">for {clientName}</p>
            </div>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>

          {error && <div className="mx-6 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}

          <div className="px-6 py-5 max-h-[70vh] overflow-y-auto">
            {/* Step 1: Select membership type */}
            {step === "type" && (
              <div>
                <p className="text-[13px] text-gray-500 mb-4">Select a membership type for this client:</p>
                <div className="space-y-3">
                  {types.map(mt => (
                    <button key={mt.id} onClick={() => { setSelectedType(mt); setStep("term"); }}
                      className="w-full text-left rounded-xl border-2 border-gray-200 px-5 py-4 hover:border-indigo-400 hover:bg-indigo-50/30 transition-all">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-[14px] font-semibold text-gray-900">{mt.title}</p>
                          <p className="text-[12px] text-gray-400 mt-0.5">{mt.term_options?.length || 0} term options</p>
                        </div>
                        <span className="text-[16px] font-bold text-indigo-600 tabular-nums">${(() => {
                          const opts = mt.term_options || [];
                          if (opts.length === 0) return "0";
                          const fees = opts.map(o => o.fee_total).sort((a, b) => a - b);
                          return fees[0] === fees[fees.length - 1] ? fees[0].toLocaleString() : `${fees[0].toLocaleString()} – $${fees[fees.length - 1].toLocaleString()}`;
                        })()}</span>
                      </div>
                    </button>
                  ))}
                  {types.length === 0 && <p className="text-[13px] text-gray-400 text-center py-6">No active membership types. Create one first.</p>}
                </div>
              </div>
            )}

            {/* Step 2: Select term */}
            {step === "term" && selectedType && (
              <div>
                <button onClick={() => setStep("type")} className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium mb-4 flex items-center gap-1">
                  <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" /></svg>
                  Back to membership types
                </button>
                <p className="text-[14px] font-semibold text-gray-900 mb-1">{selectedType.title}</p>
                <p className="text-[13px] text-gray-500 mb-4">Select a term option:</p>
                <div className="space-y-3">
                  {(selectedType.term_options || []).map(to => (
                    <button key={to.id} onClick={() => { setSelectedTerm(to); setStep("details"); }}
                      className="w-full text-left rounded-xl border-2 border-gray-200 px-5 py-4 hover:border-indigo-400 hover:bg-indigo-50/30 transition-all">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-[14px] font-semibold text-gray-900">{to.duration_months} months</p>
                          {to.fee_monthly && <p className="text-[12px] text-gray-400">${to.fee_monthly}/month</p>}
                          {to.num_payments && <p className="text-[11px] text-indigo-600">{to.num_payments} payments then active</p>}
                        </div>
                        <div className="text-right">
                          <span className="text-[18px] font-bold text-gray-900 tabular-nums">${to.fee_total.toLocaleString()}</span>
                          {to.bonuses && to.bonuses.length > 0 && (
                            <p className="text-[11px] text-emerald-600">{to.bonuses.length} bonus{to.bonuses.length !== 1 ? "es" : ""}</p>
                          )}
                        </div>
                      </div>
                    </button>
                  ))}
                  {(selectedType.term_options || []).length === 0 && <p className="text-[13px] text-gray-400 text-center py-6">No term options. Add them on the membership type page.</p>}
                </div>
              </div>
            )}

            {/* Step 3: Payment details */}
            {step === "details" && selectedType && selectedTerm && (
              <div>
                <button onClick={() => setStep("term")} className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium mb-4 flex items-center gap-1">
                  <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" /></svg>
                  Back to term options
                </button>

                <div className="rounded-xl bg-indigo-50 border border-indigo-100 px-5 py-3 mb-5">
                  <p className="text-[14px] font-semibold text-gray-900">{selectedType.title}</p>
                  <p className="text-[12px] text-gray-500">{selectedTerm.duration_months} months · ${selectedTerm.fee_total.toLocaleString()} total</p>
                </div>

                <div className="space-y-5">
                  <div>
                    <label className="block text-[13px] font-semibold text-gray-700 mb-2">Flow</label>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 max-w-xl">
                      <button
                        type="button"
                        onClick={() => setFlowMode("create_now")}
                        className={`rounded-md border px-3 py-2 text-[12px] font-medium ${flowMode === "create_now" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                      >
                        Create now
                      </button>
                      <button
                        type="button"
                        onClick={() => setFlowMode("send_invite")}
                        className={`rounded-md border px-3 py-2 text-[12px] font-medium ${flowMode === "send_invite" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                      >
                        Send to client to finalise
                      </button>
                    </div>
                    {flowMode === "send_invite" && (
                      <p className="mt-2 text-[12px] text-gray-500">Client receives a unique link to review terms, sign, and complete setup.</p>
                    )}
                  </div>

                  {flowMode === "send_invite" && frequency !== "upfront" && (
                    <div>
                      <label className="block text-[13px] font-semibold text-gray-700 mb-2">Payment collection mode</label>
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 max-w-xl">
                        <button
                          type="button"
                          onClick={() => setPaymentStrategy("ask_client")}
                          className={`rounded-md border px-3 py-2 text-[12px] font-medium ${paymentStrategy === "ask_client" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                        >
                          Ask client for payment method
                        </button>
                        <button
                          type="button"
                          onClick={() => setPaymentStrategy("use_existing")}
                          className={`rounded-md border px-3 py-2 text-[12px] font-medium ${paymentStrategy === "use_existing" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                        >
                          Use existing method on client
                        </button>
                      </div>
                    </div>
                  )}

                  <div>
                    <label className="block text-[13px] font-semibold text-gray-700 mb-2">Start Date</label>
                    <input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} className="input-field max-w-xs" />
                  </div>

                  <div>
                    <label className="block text-[13px] font-semibold text-gray-700 mb-2">Payment Frequency</label>
                    <div className="space-y-2">
                      {frequencies.map(f => (
                        <label key={f.value} className={`flex items-center gap-3 rounded-xl border-2 px-4 py-3 cursor-pointer transition-all ${
                          frequency === f.value ? "border-indigo-500 bg-indigo-50" : "border-gray-200 hover:border-gray-300"
                        }`}>
                          <input type="radio" name="freq" value={f.value} checked={frequency === f.value} onChange={() => setFrequency(f.value)} className="h-4 w-4 text-indigo-600" />
                          <span className="flex-1 text-[13px] font-medium text-gray-900">{f.label}</span>
                          <span className="text-[14px] font-bold text-gray-900 tabular-nums">${f.amount?.toLocaleString()}</span>
                        </label>
                      ))}
                    </div>
                  </div>

                  {frequency !== "upfront" && (flowMode === "create_now" || (flowMode === "send_invite" && paymentStrategy === "use_existing")) && (
                    <div>
                      <label className="block text-[13px] font-semibold text-gray-700 mb-2">Billing method</label>
                      <div className="grid grid-cols-2 gap-2 max-w-md">
                        <button
                          type="button"
                          onClick={() => setBillingMethod("direct_debit")}
                          className={`rounded-md border px-3 py-2 text-[12px] font-medium ${billingMethod === "direct_debit" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                        >
                          Direct debit
                        </button>
                        <button
                          type="button"
                          onClick={() => setBillingMethod("card")}
                          className={`rounded-md border px-3 py-2 text-[12px] font-medium ${billingMethod === "card" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                        >
                          Stripe card
                        </button>
                      </div>
                    </div>
                  )}

                  {selectedTerm.bonuses && selectedTerm.bonuses.length > 0 && (
                    <div>
                      <label className="block text-[13px] font-semibold text-gray-700 mb-2">Included Bonuses</label>
                      <div className="space-y-2">
                        {selectedTerm.bonuses.map((b: any) => (
                          <div key={b.id} className="flex items-center justify-between rounded-lg bg-emerald-50 border border-emerald-200 px-4 py-2">
                            <span className="text-[13px] text-emerald-800">{b.title}</span>
                            <span className="text-[13px] font-semibold text-emerald-700 tabular-nums">${b.value.toLocaleString()}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {frequency !== "upfront" && billingMethod === "direct_debit" && (flowMode === "create_now" || (flowMode === "send_invite" && paymentStrategy === "use_existing")) && (
                    <div className="rounded-xl border border-gray-200 bg-gray-50/50 px-4 py-4">
                      <p className="text-[13px] font-semibold text-gray-700 mb-2">Direct debit (optional)</p>
                      {(stripeDirectDebitMethods.length > 0 || hasStoredMandate) && (
                        <div className="mb-3 grid grid-cols-1 sm:grid-cols-3 gap-2">
                          <button
                            type="button"
                            onClick={() => setDirectDebitMode("stripe_existing")}
                            disabled={stripeDirectDebitMethods.length === 0}
                            className={`rounded-md border px-3 py-2 text-[12px] font-medium ${directDebitMode === "stripe_existing" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"}`}
                          >
                            Use saved Stripe DD
                          </button>
                          <button
                            type="button"
                            onClick={() => setDirectDebitMode("gocardless_existing")}
                            disabled={!hasStoredMandate}
                            className={`rounded-md border px-3 py-2 text-[12px] font-medium ${directDebitMode === "gocardless_existing" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300 disabled:opacity-50 disabled:cursor-not-allowed"}`}
                          >
                            Use GoCardless mandate
                          </button>
                          {flowMode === "create_now" && (
                            <button
                              type="button"
                              onClick={() => setDirectDebitMode("new")}
                              className={`rounded-md border px-3 py-2 text-[12px] font-medium ${directDebitMode === "new" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 text-gray-600 hover:border-gray-300"}`}
                            >
                              Add new payment method
                            </button>
                          )}
                        </div>
                      )}
                      {flowMode === "send_invite" && paymentStrategy === "use_existing" ? (
                        <p className="text-[12px] text-emerald-700">
                          {directDebitMode === "stripe_existing" && selectedStripeDirectDebitMethodId
                            ? "Saved Stripe direct debit method will be reused when client finalises."
                            : (directDebitMode === "gocardless_existing" && storedMandateID ? "Saved GoCardless mandate will be reused when client finalises." : "No saved direct debit method selected.")}
                        </p>
                      ) : directDebitMode === "stripe_existing" ? (
                        stripeDirectDebitMethods.length > 0 ? (
                          <div className="space-y-2">
                            {stripeDirectDebitMethods.map((m) => (
                              <label key={m.id} className="flex items-center gap-2 text-[12px] text-gray-700">
                                <input
                                  type="radio"
                                  name="membership_stripe_direct_debit"
                                  checked={selectedStripeDirectDebitMethodId === m.id}
                                  onChange={() => setSelectedStripeDirectDebitMethodId(m.id)}
                                  className="h-4 w-4 text-indigo-600"
                                />
                                <span>{m.label}</span>
                                {m.is_default && <span className="badge badge-gray">Default</span>}
                              </label>
                            ))}
                          </div>
                        ) : (
                          <p className="text-[12px] text-gray-500">No saved Stripe direct debit method found for this client.</p>
                        )
                      ) : directDebitMode === "gocardless_existing" ? (
                        hasStoredMandate ? (
                          <p className="text-[12px] text-emerald-700">Existing GoCardless mandate will be reused for this membership.</p>
                        ) : (
                          <p className="text-[12px] text-gray-500">No saved GoCardless mandate found for this client.</p>
                        )
                      ) : (
                        <>
                          <p className="text-[12px] text-gray-500 mb-3">Enter bank details to create a new mandate for this client.</p>
                          <div className="space-y-3">
                            <div>
                              <label className="block text-[12px] font-medium text-gray-600 mb-1">Account holder name</label>
                              <input type="text" value={accountHolderName} onChange={e => setAccountHolderName(e.target.value)} placeholder={clientName} className="input-field max-w-md text-[13px]" />
                            </div>
                            <div>
                              <label className="block text-[12px] font-medium text-gray-600 mb-1">BSB</label>
                              <input type="text" value={bsb} onChange={e => setBsb(e.target.value)} placeholder="000-000" className="input-field max-w-[8rem] font-mono text-[13px]" maxLength={9} />
                            </div>
                            <div>
                              <label className="block text-[12px] font-medium text-gray-600 mb-1">Account number</label>
                              <input type="text" value={accountNumber} onChange={e => setAccountNumber(e.target.value)} placeholder="12345678" className="input-field max-w-[12rem] font-mono text-[13px]" />
                            </div>
                          </div>
                        </>
                      )}
                    </div>
                  )}

                  {frequency !== "upfront" && billingMethod === "card" && (flowMode === "create_now" || (flowMode === "send_invite" && paymentStrategy === "use_existing")) && (
                    <div className="rounded-xl border border-gray-200 bg-gray-50/50 px-4 py-4">
                      <p className="text-[13px] font-semibold text-gray-700 mb-2">Saved Stripe card</p>
                      {stripeCardMethods.length > 0 ? (
                        <div className="space-y-2">
                          {stripeCardMethods.map((m) => (
                            <label key={m.id} className="flex items-center gap-2 text-[12px] text-gray-700">
                              <input
                                type="radio"
                                name="membership_stripe_card"
                                checked={selectedStripeCardMethodId === m.id}
                                onChange={() => setSelectedStripeCardMethodId(m.id)}
                                className="h-4 w-4 text-indigo-600"
                              />
                              <span>{m.label}</span>
                              {m.is_default && <span className="badge badge-gray">Default</span>}
                            </label>
                          ))}
                        </div>
                      ) : (
                        <div className="space-y-2">
                          <p className="text-[12px] text-gray-500">No saved Stripe card found for this client.</p>
                          <button type="button" onClick={() => setBillingMethod("direct_debit")} className="text-[12px] text-indigo-600 hover:text-indigo-700">
                            Switch to direct debit
                          </button>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                <div className="mt-6 flex justify-between">
                  <button onClick={() => setStep("term")} className="btn-secondary text-[12px]">Back</button>
                  <button onClick={handleSubmit} disabled={saving} className="btn-primary">
                    {saving ? (flowMode === "send_invite" ? "Creating invite..." : "Creating...") : (flowMode === "send_invite" ? "Create Invite" : "Create Membership")}
                  </button>
                </div>
              </div>
            )}

            {step === "confirm" && (
              <div>
                <p className="text-[16px] font-semibold text-gray-900">Invite created</p>
                <p className="text-[13px] text-gray-500 mt-1">
                  {inviteEmailSent
                    ? "Invite email has been sent to the client."
                    : "Email was not sent. Share this link with the client."}
                </p>
                {inviteWarning && (
                  <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
                    {inviteWarning}
                  </div>
                )}
                {inviteURL && (
                  <div className="mt-4 rounded-md border border-gray-200 bg-gray-50 px-3 py-3">
                    <p className="text-[11px] text-gray-500 mb-2">Client finalisation link</p>
                    <div className="flex items-center gap-2">
                      <input readOnly value={inviteURL} onClick={(e) => (e.target as HTMLInputElement).select()} className="input-field text-[12px] font-mono flex-1" />
                      <button
                        type="button"
                        onClick={() => navigator.clipboard.writeText(inviteURL)}
                        className="btn-secondary text-[12px]"
                      >
                        Copy
                      </button>
                    </div>
                  </div>
                )}
                <div className="mt-6 flex justify-end">
                  <button onClick={onCreated} className="btn-primary">Done</button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Take Payment Modal ─── */
function TakePaymentModal({ clientId, clientName, onClose, onCharged }: {
  clientId: string; clientName: string; onClose: () => void; onCharged: () => void;
}) {
  type NumericField = number | "";
  const toNumberOrZero = (value: NumericField) => (value === "" ? 0 : value);
  const round2 = (value: number) => Math.round(value * 100) / 100;

  const [services, setServices] = useState<{ id: string; name: string; price: number; tax_rate: number; description?: string }[]>([]);
  const [selectedServiceId, setSelectedServiceId] = useState("");
  const [description, setDescription] = useState("");
  const [amount, setAmount] = useState<NumericField>("");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Payment provider info
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<string[]>([]);
  const [hasMandate, setHasMandate] = useState(false);
  const [clientEmail, setClientEmail] = useState<string | null>(null);
  const [salesTaxName, setSalesTaxName] = useState("GST");
  const [salesTaxPercent, setSalesTaxPercent] = useState(10);
  const [stripePaymentMethods, setStripePaymentMethods] = useState<Array<{ id: string; type: string; label: string; is_default?: boolean }>>([]);

  // Selected payment method: "gocardless" | "stripe" | "gocardless_bank" | "stripe_becs"
  const [paymentMethod, setPaymentMethod] = useState<"gocardless" | "stripe" | "gocardless_bank" | "stripe_becs" | "">("");
  const [selectedStripePaymentMethodId, setSelectedStripePaymentMethodId] = useState("");

  // Bank details for GoCardless (new mandate)
  const [bsb, setBsb] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [accountHolderName, setAccountHolderName] = useState(clientName);

  // Send mandate link
  const [sendingLink, setSendingLink] = useState(false);
  const [mandateLink, setMandateLink] = useState("");

  const hasGC = providers.includes("gocardless");
  const hasStripe = providers.includes("stripe");

  useEffect(() => {
    Promise.all([
      api.listServices(),
      api.getClientMandateStatus(clientId),
      api.getTenantSettings(),
      api.listClientStripePaymentMethods(clientId).catch(() => ({ payment_methods: [] })),
    ]).then(([svcData, mandateData, tenantData, stripeMethodsData]) => {
      const servicesData = svcData as any;
      const mandate = mandateData as any;
      const tenant = tenantData as any;
      const stripeMethodsRes = stripeMethodsData as any;
      setServices((servicesData.services || []).filter((s: any) => s.is_active));
      setHasMandate(mandate.has_mandate || false);
      setClientEmail(mandate.client_email || null);
      setSalesTaxName(String(tenant?.sales_tax_name || "GST"));
      setSalesTaxPercent(Number.isFinite(Number(tenant?.sales_tax_percent)) ? Number(tenant.sales_tax_percent) : 10);
      setStripePaymentMethods(Array.isArray(stripeMethodsRes.payment_methods) ? stripeMethodsRes.payment_methods : []);
      const prov: string[] = mandate.payment_providers || [];
      setProviders(prov);
      if (mandate.has_mandate) {
        setPaymentMethod("gocardless");
      } else if (prov.includes("stripe") && !prov.includes("gocardless")) {
        setPaymentMethod("stripe");
      } else if (prov.length === 1) {
        setPaymentMethod(prov[0] as any);
      }
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [clientId]);

  const pickDefaultSavedStripeMethod = (mode: "stripe" | "stripe_becs") => {
    const wantedType = mode === "stripe_becs" ? "au_becs_debit" : "card";
    const candidates = stripePaymentMethods.filter((m) => m.type === wantedType);
    if (candidates.length === 0) {
      setSelectedStripePaymentMethodId("");
      return;
    }
    const preferred = candidates.find((m) => m.is_default) || candidates[0];
    setSelectedStripePaymentMethodId(preferred.id);
  };

  const handleServiceSelect = (serviceId: string) => {
    setSelectedServiceId(serviceId);
    const svc = services.find(s => s.id === serviceId);
    if (svc) {
      setDescription(svc.name);
      setAmount(svc.price);
    }
  };

  const enteredTotalAmount = toNumberOrZero(amount);
  const computedTaxAmount = salesTaxPercent > 0
    ? round2((enteredTotalAmount * Math.max(0, salesTaxPercent)) / (100 + Math.max(0, salesTaxPercent)))
    : 0;
  const computedBaseAmount = round2(Math.max(0, enteredTotalAmount - computedTaxAmount));

  const chargePayload = () => ({
    client_id: clientId,
    service_id: selectedServiceId === "__custom" ? null : selectedServiceId || null,
    description,
    amount: computedBaseAmount,
    tax_amount: computedTaxAmount,
    notes: notes || null,
  });

  const handleChargeGC = async () => {
    if (!description || toNumberOrZero(amount) <= 0) { setError("Description and amount are required"); return; }
    setSaving(true); setError(""); setSuccess("");
    try {
      const data = await api.chargeService(chargePayload()) as any;
      setSuccess(data.gocardless_payment_id
        ? `Direct debit submitted via GoCardless (${data.gocardless_payment_id})`
        : "Charge recorded via GoCardless.");
      setTimeout(() => onCharged(), 2000);
    } catch (err: any) { setError(err.message || "Failed"); }
    finally { setSaving(false); }
  };

  const handleChargeGCBank = async () => {
    if (!description || toNumberOrZero(amount) <= 0) { setError("Description and amount are required"); return; }
    if (!bsb || !accountNumber) { setError("BSB and account number are required"); return; }
    setSaving(true); setError(""); setSuccess("");
    try {
      const data = await api.chargeServiceWithBankDetails({
        ...chargePayload(),
        account_holder_name: accountHolderName,
        account_number: accountNumber,
        bsb,
      }) as any;
      setSuccess(data.warning
        ? `Mandate created (${data.gocardless_mandate_id}). Warning: ${data.warning}`
        : `Mandate created and payment submitted via GoCardless (${data.gocardless_payment_id})`);
      setTimeout(() => onCharged(), 2500);
    } catch (err: any) { setError(err.message || "Failed"); }
    finally { setSaving(false); }
  };

  const handleChargeStripe = async (method?: string) => {
    if (!description || toNumberOrZero(amount) <= 0) { setError("Description and amount are required"); return; }
    setSaving(true); setError(""); setSuccess("");
    try {
      const payload = {
        ...chargePayload(),
        payment_method: method || "card",
        payment_method_id: selectedStripePaymentMethodId || undefined,
      };
      const data = await api.chargeServiceStripe(payload) as any;
      if (data.checkout_url) {
        window.open(data.checkout_url, "_blank");
        const label = method === "direct_debit" ? "Direct debit" : "Stripe";
        setSuccess(`${label} checkout opened in a new tab. The charge will be recorded once the client completes payment.`);
        setTimeout(() => onCharged(), 3000);
      } else if (data.charged_with_saved_method) {
        setSuccess("Charged successfully using saved payment method.");
        setTimeout(() => onCharged(), 2000);
      } else {
        setSuccess("Charge recorded.");
        setTimeout(() => onCharged(), 2000);
      }
    } catch (err: any) { setError(err.message || "Failed"); }
    finally { setSaving(false); }
  };

  const handleSendMandateLink = async () => {
    setSendingLink(true); setError("");
    try {
      const data = await api.sendClientMandateLink(clientId) as any;
      setMandateLink(data.redirect_url || "");
      setSuccess(data.email_sent
        ? `Mandate setup link sent to ${clientEmail}. You can also copy the link below.`
        : "Mandate link created. Copy it below and share with the client.");
    } catch (err: any) { setError(err.message || "Unable to create mandate link."); }
    finally { setSendingLink(false); }
  };

  const handleSubmit = () => {
    if (paymentMethod === "gocardless" && hasMandate) return handleChargeGC();
    if (paymentMethod === "gocardless_bank") return handleChargeGCBank();
    if (paymentMethod === "gocardless" && !hasMandate) return handleChargeGCBank();
    if (paymentMethod === "stripe_becs") return handleChargeStripe("direct_debit");
    if (paymentMethod === "stripe") return handleChargeStripe("card");
  };

  const total = enteredTotalAmount;
  const orgTotalCents = Math.round(Math.max(0, total) * 100);
  const stripeSurchargeCents =
    orgTotalCents > 0 && paymentMethod === "stripe"
      ? estimateStripeProcessingFeeCents(orgTotalCents, "card")
      : orgTotalCents > 0 && paymentMethod === "stripe_becs"
        ? estimateStripeProcessingFeeCents(orgTotalCents, "direct_debit")
        : 0;
  const customerTotalStripe = total + stripeSurchargeCents / 100;
  const noProviders = !hasGC && !hasStripe;

  const canSubmit = (() => {
    if (!paymentMethod || toNumberOrZero(amount) <= 0) return false;
    if (paymentMethod === "gocardless" && hasMandate) return true;
    if (paymentMethod === "gocardless_bank" || (paymentMethod === "gocardless" && !hasMandate)) return !!bsb && !!accountNumber;
    if (paymentMethod === "stripe" || paymentMethod === "stripe_becs") return true;
    return false;
  })();

  const submitLabel = (() => {
    if (saving) return "Processing...";
    if (paymentMethod === "stripe") return selectedStripePaymentMethodId ? `Charge $${total.toFixed(2)} saved Card` : `Pay $${total.toFixed(2)} via Card`;
    if (paymentMethod === "stripe_becs") return selectedStripePaymentMethodId ? `Charge $${total.toFixed(2)} saved Direct Debit` : `Pay $${total.toFixed(2)} via Direct Debit`;
    if (paymentMethod === "gocardless" && hasMandate) return `Charge $${total.toFixed(2)} via Direct Debit`;
    if (paymentMethod === "gocardless_bank" || (paymentMethod === "gocardless" && !hasMandate)) return `Set up mandate & charge $${total.toFixed(2)}`;
    return "Select a payment method";
  })();

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />
      <div className="relative min-h-screen flex items-start justify-center pt-10 pb-10 px-4">
        <div className="relative w-full max-w-lg bg-white rounded-xl shadow-2xl">
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
            <div>
              <h2 className="text-[15px] font-bold text-gray-900">Take Payment</h2>
              <p className="text-[12px] text-gray-500">Charge {clientName}</p>
            </div>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>

          {error && <div className="mx-6 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}
          {success && <div className="mx-6 mt-4 rounded-md bg-green-50 border border-green-200 px-3 py-2 text-[13px] text-green-700">{success}</div>}

          {loading ? (
            <div className="px-6 py-12 text-center text-[13px] text-gray-500">Loading...</div>
          ) : noProviders ? (
            <div className="px-6 py-8 text-center">
              <svg className="h-10 w-10 mx-auto text-gray-300 mb-3" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" /></svg>
              <p className="text-[14px] font-semibold text-gray-700 mb-1">No payment provider configured</p>
              <p className="text-[13px] text-gray-500 mb-4">Set up GoCardless or Stripe in Settings &rarr; Integrations to start collecting payments.</p>
              <a href="/settings" className="btn-primary inline-flex">Go to Settings</a>
            </div>
          ) : (
            <div className="px-6 py-5 space-y-5">

              {/* Payment method selector */}
              <div>
                <label className="block text-[13px] font-semibold text-gray-700 mb-2">Payment method</label>
                <div className="flex gap-2">
                  {hasGC && hasMandate && (
                    <button
                      onClick={() => {
                        setPaymentMethod("gocardless");
                        setSelectedStripePaymentMethodId("");
                      }}
                      className={`flex-1 rounded-lg border-2 px-3 py-3 text-[12px] font-semibold transition-all text-center ${
                        paymentMethod === "gocardless" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                      }`}
                    >
                      <svg className="h-5 w-5 mx-auto mb-1 text-green-600" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 0h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008z" /></svg>
                      Direct Debit
                      <span className="block text-[10px] font-normal text-green-600 mt-0.5">Mandate active</span>
                    </button>
                  )}
                  {hasGC && (
                    <button
                      onClick={() => {
                        setPaymentMethod("gocardless_bank");
                        setSelectedStripePaymentMethodId("");
                      }}
                      className={`flex-1 rounded-lg border-2 px-3 py-3 text-[12px] font-semibold transition-all text-center ${
                        paymentMethod === "gocardless_bank" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                      }`}
                    >
                      <svg className="h-5 w-5 mx-auto mb-1" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 0h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008z" /></svg>
                      {hasMandate ? "Add New Bank" : "Direct Debit"}
                      <span className="block text-[10px] font-normal text-gray-500 mt-0.5">{hasMandate ? "Create new mandate" : "Enter bank details"}</span>
                    </button>
                  )}
                  {!hasGC && hasStripe && (
                    <button
                      onClick={() => {
                        setPaymentMethod("stripe_becs");
                        pickDefaultSavedStripeMethod("stripe_becs");
                      }}
                      className={`flex-1 rounded-lg border-2 px-3 py-3 text-[12px] font-semibold transition-all text-center ${
                        paymentMethod === "stripe_becs" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                      }`}
                    >
                      <svg className="h-5 w-5 mx-auto mb-1" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 0h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008zm0 3h.008v.008h-.008v-.008z" /></svg>
                      Direct Debit
                      <span className="block text-[10px] font-normal text-gray-500 mt-0.5">Bank account</span>
                    </button>
                  )}
                  {hasStripe && (
                    <button
                      onClick={() => {
                        setPaymentMethod("stripe");
                        pickDefaultSavedStripeMethod("stripe");
                      }}
                      className={`flex-1 rounded-lg border-2 px-3 py-3 text-[12px] font-semibold transition-all text-center ${
                        paymentMethod === "stripe" ? "border-indigo-500 bg-indigo-50 text-indigo-700" : "border-gray-200 bg-white text-gray-700 hover:border-gray-300"
                      }`}
                    >
                      <svg className="h-5 w-5 mx-auto mb-1" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" /></svg>
                      Card Payment
                      <span className="block text-[10px] font-normal text-gray-500 mt-0.5">Stripe checkout</span>
                    </button>
                  )}
                </div>

                {hasGC && !hasMandate && (
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      onClick={handleSendMandateLink}
                      disabled={sendingLink}
                      className="text-[11px] text-indigo-600 hover:text-indigo-700 font-medium"
                    >
                      {sendingLink ? "Sending..." : "Or send mandate link to client"}
                    </button>
                  </div>
                )}
                {mandateLink && (
                  <div className="mt-2 rounded-md bg-gray-50 border border-gray-200 px-3 py-2">
                    <p className="text-[11px] text-gray-500 mb-1">Share this link:</p>
                    <div className="flex gap-2 items-center">
                      <input readOnly value={mandateLink} className="flex-1 text-[12px] bg-white border border-gray-200 rounded px-2 py-1.5 text-gray-700 select-all" onClick={e => (e.target as HTMLInputElement).select()} />
                      <button onClick={() => navigator.clipboard.writeText(mandateLink)} className="text-[11px] text-indigo-600 font-semibold hover:text-indigo-700 whitespace-nowrap">Copy</button>
                    </div>
                  </div>
                )}
              </div>

              {(paymentMethod === "stripe" || paymentMethod === "stripe_becs") && (
                <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
                  <p className="text-[12px] font-semibold text-gray-700 mb-2">Use existing payment method</p>
                  {stripePaymentMethods.filter((m) => m.type === (paymentMethod === "stripe_becs" ? "au_becs_debit" : "card")).length > 0 ? (
                    <div className="space-y-2">
                      {stripePaymentMethods
                        .filter((m) => m.type === (paymentMethod === "stripe_becs" ? "au_becs_debit" : "card"))
                        .map((m) => (
                          <label key={m.id} className="flex items-center gap-2 text-[12px] text-gray-700">
                            <input
                              type="radio"
                              name="stripe_saved_pm"
                              checked={selectedStripePaymentMethodId === m.id}
                              onChange={() => setSelectedStripePaymentMethodId(m.id)}
                              className="h-4 w-4 text-indigo-600"
                            />
                            <span>{m.label}</span>
                            {m.is_default && <span className="badge badge-gray">Default</span>}
                          </label>
                        ))}
                      <button
                        type="button"
                        onClick={() => setSelectedStripePaymentMethodId("")}
                        className="text-[11px] text-indigo-600 hover:text-indigo-700"
                      >
                        Use Stripe checkout instead
                      </button>
                    </div>
                  ) : (
                    <p className="text-[12px] text-gray-500">No saved {paymentMethod === "stripe_becs" ? "direct debit" : "card"} methods found. Stripe checkout will be used.</p>
                  )}
                </div>
              )}

              {/* Bank details form — for GC new mandate */}
              {(paymentMethod === "gocardless_bank" || (paymentMethod === "gocardless" && !hasMandate)) && (
                <div className="rounded-lg border border-indigo-200 bg-indigo-50/50 px-4 py-4 space-y-3">
                  <h3 className="text-[13px] font-bold text-gray-900">Bank Details</h3>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-[12px] font-medium text-gray-700 mb-1">BSB</label>
                      <input value={bsb} onChange={e => setBsb(e.target.value)} className="input-field" placeholder="000-000" maxLength={7} />
                    </div>
                    <div>
                      <label className="block text-[12px] font-medium text-gray-700 mb-1">Account Number</label>
                      <input value={accountNumber} onChange={e => setAccountNumber(e.target.value)} className="input-field" placeholder="12345678" />
                    </div>
                  </div>
                  <div>
                    <label className="block text-[12px] font-medium text-gray-700 mb-1">Account Holder Name</label>
                    <input value={accountHolderName} onChange={e => setAccountHolderName(e.target.value)} className="input-field" />
                  </div>
                </div>
              )}

              {/* Service selection */}
              <div>
                <label className="block text-[13px] font-semibold text-gray-700 mb-2">Select a service</label>
                <div className="space-y-2">
                  {services.map(s => (
                    <label key={s.id} className={`flex items-center gap-3 rounded-xl border-2 px-4 py-3 cursor-pointer transition-all ${
                      selectedServiceId === s.id ? "border-indigo-500 bg-indigo-50" : "border-gray-200 hover:border-gray-300"
                    }`}>
                      <input type="radio" name="service" value={s.id} checked={selectedServiceId === s.id}
                        onChange={() => handleServiceSelect(s.id)} className="h-4 w-4 text-indigo-600" />
                      <div className="flex-1">
                        <p className="text-[13px] font-medium text-gray-900">{s.name}</p>
                        {s.description && <p className="text-[11px] text-gray-400">{s.description}</p>}
                      </div>
                      <span className="text-[14px] font-bold text-gray-900 tabular-nums">${s.price.toFixed(2)}</span>
                    </label>
                  ))}
                  {services.length === 0 && (
                    <p className="text-[13px] text-gray-400 text-center py-4">No services defined. <a href="/services" className="text-indigo-600 hover:text-indigo-700">Create one first</a></p>
                  )}
                  <label className={`flex items-center gap-3 rounded-xl border-2 px-4 py-3 cursor-pointer transition-all ${
                    selectedServiceId === "__custom" ? "border-indigo-500 bg-indigo-50" : "border-gray-200 hover:border-gray-300"
                  }`}>
                    <input type="radio" name="service" value="__custom" checked={selectedServiceId === "__custom"}
                      onChange={() => { setSelectedServiceId("__custom"); setDescription(""); setAmount(""); }} className="h-4 w-4 text-indigo-600" />
                    <span className="text-[13px] font-medium text-gray-700">Custom charge</span>
                  </label>
                </div>
              </div>

              {/* Amount details */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-[13px] font-medium text-gray-700 mb-1">Description</label>
                  <input value={description} onChange={e => setDescription(e.target.value)} className="input-field" placeholder="What is this charge for?" />
                </div>
                <div>
                  <label className="block text-[13px] font-medium text-gray-700 mb-1">Customer charge amount ($)</label>
                  <input type="number" value={amount} onChange={e => setAmount(e.target.value === "" ? "" : Number(e.target.value))} className="input-field" min="0" step="0.01" />
                  <p className="text-[10px] text-gray-400 mt-0.5">Tax inclusive — {salesTaxName} is split for records</p>
                </div>
                <div>
                  <label className="block text-[13px] font-medium text-gray-700 mb-1">{salesTaxName} ({salesTaxPercent.toFixed(2)}%)</label>
                  <p className="input-field bg-gray-50 flex items-center text-[14px] font-semibold text-gray-900 tabular-nums">
                    ${computedTaxAmount.toFixed(2)}
                  </p>
                </div>
                <div className="col-span-2">
                  <label className="block text-[13px] font-medium text-gray-700 mb-1">To your organisation (service + {salesTaxName})</label>
                  <p className="input-field bg-emerald-50/60 border-emerald-100 flex items-center text-[14px] font-bold text-emerald-950 tabular-nums">
                    ${total.toFixed(2)}
                  </p>
                </div>
              </div>

              {(paymentMethod === "stripe" || paymentMethod === "stripe_becs") && total > 0 && (
                <div className="rounded-lg border border-amber-200 bg-amber-50/80 px-4 py-3 text-[12px] text-amber-950 space-y-1">
                  <p className="font-semibold text-amber-900">Stripe checkout (customer view)</p>
                  <p>
                    Customer will be charged an estimated{" "}
                    <strong>${(stripeSurchargeCents / 100).toFixed(2)}</strong> processing surcharge on top of your{" "}
                    <strong>${total.toFixed(2)}</strong>, so they pay about{" "}
                    <strong>${customerTotalStripe.toFixed(2)}</strong> total. The surcharge covers card/DD processing; your recorded revenue stays{" "}
                    <strong>${total.toFixed(2)}</strong> (plus any processing fee line stored on the charge).
                  </p>
                </div>
              )}

              {(paymentMethod === "gocardless" || paymentMethod === "gocardless_bank") && total > 0 && (
                <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5 text-[12px] text-gray-700">
                  <strong>Direct debit:</strong> the customer is charged <strong>${total.toFixed(2)}</strong> — the same as to your organisation (no card surcharge in this flow).
                </div>
              )}

              <div>
                <label className="block text-[13px] font-medium text-gray-700 mb-1">Notes (optional)</label>
                <textarea value={notes} onChange={e => setNotes(e.target.value)}
                  className="block w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 resize-y"
                  rows={2} placeholder="Internal notes about this charge" />
              </div>

              {/* Submit */}
              <div className="flex justify-end gap-2 pt-2">
                <button onClick={onClose} className="btn-secondary text-[12px]">Cancel</button>
                <button onClick={handleSubmit} disabled={saving || !canSubmit} className={`btn-primary ${!canSubmit ? "opacity-50 cursor-not-allowed" : ""}`}>
                  <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
                  {submitLabel}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
