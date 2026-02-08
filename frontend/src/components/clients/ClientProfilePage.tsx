import { useState, useEffect, useRef } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import api from "../../api/client";

/* ─── Types ─── */
interface Client {
  id: string; name: string; email?: string; phone?: string;
  address?: string; suburb?: string; state?: string; postcode?: string;
  birth_date?: string; gender?: string;
  emergency_contact_name?: string; emergency_contact_phone?: string;
  business_structure?: string; gst_registered?: string; tax_number?: string;
  company_number?: string; company_phone?: string; company_website?: string;
  company_address?: string; company_suburb?: string; company_state?: string; company_postcode?: string;
  created_at: string;
}

interface Membership {
  membership_id: string;
  membership_type: string;
  membership_type_id: string;
  term_months: number;
  start_date: string;
  end_date: string;
  payment_method?: string;
  payment_frequency?: string;
  payment_amount?: number;
  status: string;
  total_paid: number;
  total_expected: number;
  failed_count: number;
  last_payment_date?: string;
  last_payment_status?: string;
}

interface Payment {
  amount: number;
  status: string;
  due_date: string;
  paid_date?: string;
}

/* ─── Main Component ─── */
export default function ClientProfilePage() {
  const { clientId } = useParams();
  const navigate = useNavigate();
  const [client, setClient] = useState<Client | null>(null);
  const [memberships, setMemberships] = useState<Membership[]>([]);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"summary" | "memberships" | "billing" | "payments">("summary");
  const [showEdit, setShowEdit] = useState(false);

  useEffect(() => {
    if (!clientId) return;
    loadAll();
  }, [clientId]);

  const loadAll = async () => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    try {
      const [clientRes, memberRes, payRes] = await Promise.all([
        fetch(`/api/tenant/clients/${clientId}`, { headers: h }).then(r => r.json()),
        fetch(`/api/tenant/clients/${clientId}/memberships`, { headers: h }).then(r => r.json()),
        fetch(`/api/tenant/payments/client/${clientId}`, { headers: h }).then(r => r.json()),
      ]);
      setClient(clientRes);
      setMemberships(memberRes.memberships || []);
      setPayments(payRes.payments || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-32">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-200 border-t-indigo-600" />
      </div>
    );
  }
  if (!client) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Client not found</div>;

  const initials = client.name.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase();
  const activeMemberships = memberships.filter(m => m.status === "active");
  const hasActive = activeMemberships.length > 0;
  const succeededPayments = payments.filter(p => p.status === "succeeded");
  const failedPayments = payments.filter(p => p.status === "failed");
  const totalPaid = succeededPayments.reduce((sum, p) => sum + p.amount, 0);
  const fmtDate = (d: string) => d ? new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" }) : "—";
  const fmtShort = (d: string) => d ? new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" }) : "—";

  const tabs = [
    { key: "summary" as const, label: "Summary" },
    { key: "memberships" as const, label: "Memberships" },
    { key: "billing" as const, label: "Billing schedule" },
    { key: "payments" as const, label: "Payments" },
  ];

  const fullAddress = [client.address, client.suburb, [client.state, client.postcode].filter(Boolean).join(" ")].filter(Boolean).join("\n");

  return (
    <div>
      {/* ─── Breadcrumb header (Ignition-style) ─── */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-1">
        <div className="flex items-center gap-2 text-[15px]">
          <Link to="/clients" className="text-indigo-600 hover:text-indigo-700 font-semibold">Clients</Link>
          <span className="text-gray-300">›</span>
          <span className="font-bold text-gray-900 text-lg">{client.name.toUpperCase()}</span>
          <span className={`badge text-[11px] ${hasActive ? "badge-green" : "badge-gray"}`}>
            {hasActive ? "Active" : "Inactive"}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <MoreActionsMenu clientId={client.id} clientName={client.name} />
          <button onClick={() => setShowEdit(true)} className="btn-secondary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" /></svg>
            Edit client
          </button>
        </div>
      </div>

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
              onViewAll={() => setTab("payments")}
            />
          )}
          {tab === "memberships" && (
            <MembershipsTab memberships={memberships} fmtShort={fmtShort} />
          )}
          {tab === "billing" && (
            <BillingScheduleTab memberships={activeMemberships} fmtDate={fmtDate} />
          )}
          {tab === "payments" && (
            <PaymentsTab payments={payments} fmtShort={fmtShort} />
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
              {client.gst_registered && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">GST registered</span>
                  <span className="text-[13px] text-gray-900">{client.gst_registered}</span>
                </div>
              )}
              {client.company_number && (
                <div className="flex justify-between gap-4">
                  <span className="text-[13px] text-gray-500">Company number</span>
                  <span className="text-[13px] text-gray-900">{client.company_number}</span>
                </div>
              )}
            </div>

            {/* Emergency contact */}
            {client.emergency_contact_name && (
              <div className="border-t border-gray-100 px-5 py-3">
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-100 text-gray-500 text-[11px] font-bold">
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
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Total paid</span>
                <span className="font-semibold text-gray-900 tabular-nums">${totalPaid.toLocaleString(undefined, { minimumFractionDigits: 2 })}</span>
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
            <div className="px-5 py-3">
              <p className="text-[12px] text-gray-400 mb-2">Client pays with</p>
              <div className="flex items-center gap-2.5">
                <div className="flex h-9 w-11 items-center justify-center rounded bg-gray-100">
                  <svg className="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" /></svg>
                </div>
                <div>
                  <p className="text-[13px] font-medium text-gray-700">Direct Debit</p>
                  <p className="text-[11px] text-gray-400">Bank account on file</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Edit modal */}
      {showEdit && (
        <EditClientModal
          client={client}
          onClose={() => setShowEdit(false)}
          onSaved={() => { setShowEdit(false); loadAll(); }}
        />
      )}
    </div>
  );
}

/* ─── Summary Tab ─── */
function SummaryTab({ memberships, payments, fmtShort, onViewAll }: {
  memberships: Membership[]; payments: Payment[]; fmtShort: (d: string) => string; onViewAll: () => void;
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
              <MembershipSummaryRow key={m.membership_id} m={m} fmtShort={fmtShort} />
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
              <MembershipSummaryRow key={m.membership_id} m={m} fmtShort={fmtShort} />
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
                    <td className="text-[13px] tabular-nums font-medium text-gray-900">${p.amount.toFixed(2)}</td>
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
function MembershipSummaryRow({ m, fmtShort }: { m: Membership; fmtShort: (d: string) => string }) {
  const daysLeft = Math.ceil((new Date(m.end_date).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
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
              ${m.payment_amount.toFixed(2)}<span className="text-gray-400 font-normal">/{m.payment_frequency?.slice(0, 2) || ""}</span>
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
function MembershipsTab({ memberships, fmtShort }: { memberships: Membership[]; fmtShort: (d: string) => string }) {
  const active = memberships.filter(m => m.status === "active");
  const inactive = memberships.filter(m => m.status !== "active");

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
      {/* Toggle for inactive */}
      {active.length > 0 && (
        <div>
          <h2 className="text-[13px] font-semibold text-gray-700 mb-3">Active ({active.length})</h2>
          <div className="space-y-4">
            {active.map(m => <MembershipCard key={m.membership_id} m={m} fmtShort={fmtShort} />)}
          </div>
        </div>
      )}
      {inactive.length > 0 && (
        <div>
          <h2 className="text-[13px] font-semibold text-gray-500 mb-3">Past ({inactive.length})</h2>
          <div className="space-y-4">
            {inactive.map(m => <MembershipCard key={m.membership_id} m={m} fmtShort={fmtShort} />)}
          </div>
        </div>
      )}
    </div>
  );
}

function MembershipCard({ m, fmtShort }: { m: Membership; fmtShort: (d: string) => string }) {
  const daysLeft = Math.ceil((new Date(m.end_date).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
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
            <p className="text-[13px] text-gray-500 mt-0.5">{m.term_months}-month term</p>
          </div>
          <div className="text-right">
            <p className="text-[20px] font-bold text-gray-900 tabular-nums">${m.payment_amount?.toFixed(2) || "0.00"}</p>
            <p className="text-[12px] text-gray-400 capitalize">{m.payment_frequency || "—"}</p>
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
                <span className={`badge ml-1 ${m.last_payment_status === "succeeded" ? "badge-green" : "badge-red"}`}>
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
function BillingScheduleTab({ memberships, fmtDate }: { memberships: Membership[]; fmtDate: (d: string) => string }) {
  const upcoming: { date: Date; membership: Membership }[] = [];

  memberships.forEach(m => {
    if (!m.payment_amount || !m.payment_frequency) return;
    const end = new Date(m.end_date);
    const now = new Date();
    let next = new Date(now);

    if (m.payment_frequency === "weekly") {
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
                    <p className="text-[12px] text-gray-500 capitalize">{item.membership.payment_frequency} · Due {item.date.toLocaleDateString("en-AU", { day: "numeric", month: "long" })}</p>
                  </div>
                  <p className="text-[15px] font-semibold text-gray-900 tabular-nums">${item.membership.payment_amount?.toFixed(2)}</p>
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
function PaymentsTab({ payments, fmtShort }: { payments: Payment[]; fmtShort: (d: string) => string }) {
  if (payments.length === 0) {
    return (
      <div className="card px-5 py-12 text-center">
        <svg className="mx-auto h-12 w-12 text-gray-200" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" /></svg>
        <p className="mt-3 text-[13px] text-gray-500">No payments yet</p>
      </div>
    );
  }

  // Group payments by month
  const grouped: Record<string, Payment[]> = {};
  payments.forEach(p => {
    const d = new Date(p.due_date);
    const key = d.toLocaleDateString("en-AU", { month: "long", year: "numeric" });
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(p);
  });

  return (
    <div className="space-y-4">
      {Object.entries(grouped).map(([month, items]) => (
        <div key={month} className="card overflow-hidden">
          <div className="bg-gray-50 px-5 py-2.5 border-b border-gray-200">
            <p className="text-[13px] font-medium text-gray-700">{month}</p>
          </div>
          <div className="divide-y divide-gray-100">
            {items.map((p, i) => (
              <div key={i} className="px-5 py-3">
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[13px] font-medium text-gray-900">{fmtShort(p.due_date)}</span>
                    <span className={`badge ${p.status === "succeeded" ? "badge-green" : p.status === "failed" ? "badge-red" : "badge-yellow"}`}>
                      {p.status === "succeeded" ? "Paid" : p.status}
                    </span>
                  </div>
                  <span className="text-[15px] font-semibold text-gray-900 tabular-nums">${p.amount.toFixed(2)}</span>
                </div>
                {p.paid_date && p.status === "succeeded" && (
                  <p className="text-[11px] text-emerald-600">
                    ✓ Payment completed ({fmtShort(p.paid_date)})
                  </p>
                )}
                {p.status === "failed" && (
                  <p className="text-[11px] text-red-500">
                    ✗ Payment failed
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

/* ─── More Actions Dropdown ─── */
function MoreActionsMenu({ clientId, clientName }: { clientId: string; clientName: string }) {
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
        <div className="absolute right-0 top-full mt-1 w-48 rounded-lg border border-gray-200 bg-white py-1 shadow-lg z-50">
          <button onClick={() => setOpen(false)} className="flex w-full items-center gap-2 px-3 py-2 text-[13px] text-gray-700 hover:bg-gray-50">
            <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" /></svg>
            Send email
          </button>
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
function EditClientModal({ client, onClose, onSaved }: { client: Client; onClose: () => void; onSaved: () => void }) {
  const [tab, setTab] = useState<"details" | "company">("details");
  const [form, setForm] = useState({
    name: client.name, email: client.email || "", phone: client.phone || "",
    address: client.address || "", suburb: client.suburb || "", state: client.state || "",
    postcode: client.postcode || "", birth_date: client.birth_date?.split("T")[0] || "", gender: client.gender || "",
    emergency_contact_name: client.emergency_contact_name || "",
    emergency_contact_phone: client.emergency_contact_phone || "",
    business_structure: client.business_structure || "", gst_registered: client.gst_registered || "",
    tax_number: client.tax_number || "", company_number: client.company_number || "",
    company_phone: client.company_phone || "", company_website: client.company_website || "",
    company_address: client.company_address || "", company_suburb: client.company_suburb || "",
    company_state: client.company_state || "", company_postcode: client.company_postcode || "",
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError("");
    try {
      await api.updateClient(client.id, form);
      onSaved();
    } catch (err) { setError(err instanceof Error ? err.message : "Failed"); }
    finally { setSaving(false); }
  };

  const F = ({ label, value, onChange, type = "text", placeholder = "", required = false, colSpan = false }: {
    label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string; required?: boolean; colSpan?: boolean;
  }) => (
    <div className={colSpan ? "sm:col-span-2" : ""}>
      <label className="block text-[13px] font-medium text-gray-700 mb-1">{label}{required && <span className="text-red-500"> *</span>}</label>
      <input type={type} value={value} onChange={(e) => onChange(e.target.value)} className="input-field" placeholder={placeholder} required={required} />
    </div>
  );

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
              <button type="button" onClick={() => { const f = document.getElementById("edit-client-form") as HTMLFormElement; f?.requestSubmit(); }} disabled={saving} className="btn-primary text-[12px]">
                <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                {saving ? "Saving..." : "Save"}
              </button>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200 px-6">
            {(["details", "company"] as const).map((t) => (
              <button key={t} onClick={() => setTab(t)}
                className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
                  tab === t ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
                }`}>{t === "details" ? "Details" : "Company"}</button>
            ))}
          </div>

          {error && <div className="mx-6 mt-4 rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}

          <form id="edit-client-form" onSubmit={handleSubmit} className="px-6 py-5 max-h-[60vh] overflow-y-auto">
            {tab === "details" && (
              <div className="space-y-5">
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Personal information</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <F label="Full name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} required colSpan />
                    <F label="Email" value={form.email} onChange={(v) => setForm({ ...form, email: v })} type="email" />
                    <F label="Phone" value={form.phone} onChange={(v) => setForm({ ...form, phone: v })} type="tel" />
                    <F label="Date of birth" value={form.birth_date} onChange={(v) => setForm({ ...form, birth_date: v })} type="date" />
                    <div>
                      <label className="block text-[13px] font-medium text-gray-700 mb-1">Gender</label>
                      <select value={form.gender} onChange={(e) => setForm({ ...form, gender: e.target.value })} className="input-field">
                        <option value="">—</option><option value="Female">Female</option><option value="Male">Male</option>
                      </select>
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Address</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <F label="Street address" value={form.address} onChange={(v) => setForm({ ...form, address: v })} colSpan />
                    <F label="Suburb" value={form.suburb} onChange={(v) => setForm({ ...form, suburb: v })} />
                    <F label="State" value={form.state} onChange={(v) => setForm({ ...form, state: v })} />
                    <F label="Postcode" value={form.postcode} onChange={(v) => setForm({ ...form, postcode: v })} />
                  </div>
                </div>
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Emergency contact</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <F label="Contact name" value={form.emergency_contact_name} onChange={(v) => setForm({ ...form, emergency_contact_name: v })} />
                    <F label="Contact phone" value={form.emergency_contact_phone} onChange={(v) => setForm({ ...form, emergency_contact_phone: v })} type="tel" />
                  </div>
                </div>
              </div>
            )}
            {tab === "company" && (
              <div className="space-y-5">
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Company information</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <F label="Business structure" value={form.business_structure} onChange={(v) => setForm({ ...form, business_structure: v })} placeholder="e.g. Sole Trader" />
                    <F label="GST registered" value={form.gst_registered} onChange={(v) => setForm({ ...form, gst_registered: v })} placeholder="Yes / No" />
                    <F label="Tax number" value={form.tax_number} onChange={(v) => setForm({ ...form, tax_number: v })} />
                    <F label="Company number" value={form.company_number} onChange={(v) => setForm({ ...form, company_number: v })} />
                    <F label="Phone" value={form.company_phone} onChange={(v) => setForm({ ...form, company_phone: v })} type="tel" />
                    <F label="Website" value={form.company_website} onChange={(v) => setForm({ ...form, company_website: v })} placeholder="https://" colSpan />
                  </div>
                </div>
                <div>
                  <h4 className="text-[13px] font-semibold text-gray-900 mb-3">Company address</h4>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <F label="Street address" value={form.company_address} onChange={(v) => setForm({ ...form, company_address: v })} colSpan />
                    <F label="Suburb" value={form.company_suburb} onChange={(v) => setForm({ ...form, company_suburb: v })} />
                    <F label="State" value={form.company_state} onChange={(v) => setForm({ ...form, company_state: v })} />
                    <F label="Postcode" value={form.company_postcode} onChange={(v) => setForm({ ...form, company_postcode: v })} />
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
