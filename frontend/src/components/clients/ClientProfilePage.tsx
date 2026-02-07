import { useState, useEffect } from "react";
import { useParams, Link } from "react-router-dom";
import api from "../../api/client";

interface Client {
  id: string; name: string; email?: string; phone?: string;
  address?: string; suburb?: string; state?: string; postcode?: string;
  birth_date?: string; gender?: string;
  emergency_contact_name?: string; emergency_contact_phone?: string;
  created_at: string;
}

interface Membership {
  id: string; membership_type_id: string; start_date: string;
  min_term_end_date: string; payment_frequency?: string;
  payment_amount?: number; status: string; created_at: string;
}

export default function ClientProfilePage() {
  const { clientId } = useParams();
  const [client, setClient] = useState<Client | null>(null);
  const [memberships, setMemberships] = useState<Membership[]>([]);
  const [payments, setPayments] = useState<{ amount: number; status: string; due_date: string; paid_date?: string }[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"summary" | "memberships" | "billing" | "payments" | "details">("summary");

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

  if (loading) return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>;
  if (!client) return <div className="card px-5 py-12 text-center text-[13px] text-gray-500">Client not found</div>;

  const initials = client.name.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase();
  const activeMemberships = memberships.filter(m => m.status === "active");
  const succeededPayments = payments.filter(p => p.status === "succeeded");
  const failedPayments = payments.filter(p => p.status === "failed");
  const totalPaid = succeededPayments.reduce((sum, p) => sum + p.amount, 0);
  const fmtDate = (d: string) => d ? new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" }) : "—";
  const fmtShort = (d: string) => d ? new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" }) : "—";

  const tabs = [
    { key: "summary" as const, label: "Summary" },
    { key: "memberships" as const, label: "Memberships" },
    { key: "billing" as const, label: "Billing Schedule" },
    { key: "payments" as const, label: "Payments" },
    { key: "details" as const, label: "Details" },
  ];

  return (
    <div>
      {/* Breadcrumb header */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-1.5 text-[15px]">
          <Link to="/clients" className="text-indigo-600 hover:text-indigo-700 font-semibold">Clients</Link>
          <span className="text-gray-400">›</span>
          <span className="font-semibold text-gray-900">{client.name}</span>
          {activeMemberships.length > 0 && <span className="badge badge-green ml-1">Active</span>}
        </div>
        <div className="flex items-center gap-2">
          <Link to={`/clients`} className="btn-secondary text-[12px]">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z" /></svg>
            Edit client
          </Link>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-5">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2">
          {tab === "summary" && (
            <div className="space-y-5">
              {/* Memberships */}
              <div className="card">
                <div className="card-header"><h3>Memberships</h3></div>
                {memberships.length === 0 ? (
                  <div className="px-5 py-8 text-center text-[13px] text-gray-400">No memberships yet</div>
                ) : (
                  <div className="divide-y divide-gray-100">
                    {memberships.map(m => (
                      <div key={m.id} className="px-5 py-3 flex items-center justify-between">
                        <div>
                          <div className="flex items-center gap-2">
                            <span className={`badge ${m.status === "active" ? "badge-green" : m.status === "expired" ? "badge-gray" : "badge-red"}`}>
                              {m.status.charAt(0).toUpperCase() + m.status.slice(1)}
                            </span>
                          </div>
                          <p className="text-[12px] text-gray-500 mt-1">
                            {fmtShort(m.start_date)} — {fmtShort(m.min_term_end_date)}
                            {m.payment_amount && ` · $${m.payment_amount}/${m.payment_frequency?.slice(0, 2) || ""}`}
                          </p>
                        </div>
                        <p className="text-[13px] font-medium text-gray-900 tabular-nums">${m.payment_amount?.toFixed(2) || "—"}</p>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Recent payments */}
              <div className="card">
                <div className="card-header">
                  <h3>Recent Payments</h3>
                  <button onClick={() => setTab("payments")} className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium">View all</button>
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
                            <td className="text-[13px] tabular-nums text-gray-900">${p.amount.toFixed(2)}</td>
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
          )}

          {tab === "memberships" && (
            <div className="space-y-4">
              {memberships.length === 0 ? (
                <div className="card px-5 py-12 text-center text-[13px] text-gray-400">No memberships yet</div>
              ) : (
                memberships.map(m => {
                  const daysLeft = Math.ceil((new Date(m.min_term_end_date).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
                  return (
                    <div key={m.id} className="card overflow-hidden">
                      {/* Top stripe */}
                      <div className={`h-1 ${m.status === "active" ? "bg-emerald-500" : m.status === "expired" ? "bg-gray-300" : "bg-red-400"}`} />

                      <div className="px-5 py-4">
                        {/* Header: ID + dates */}
                        <p className="text-[12px] text-gray-400 mb-2">
                          MEM-{m.id.slice(0, 4).toUpperCase()} · Started {fmtShort(m.start_date)}
                        </p>

                        {/* Main row: name + price */}
                        <div className="flex items-start justify-between">
                          <div>
                            <p className="text-[15px] font-semibold text-gray-900">Membership</p>
                            <p className="text-[13px] text-gray-500 mt-0.5">
                              {m.status === "active" && daysLeft > 0
                                ? `${daysLeft} days remaining`
                                : m.status === "expired"
                                ? "Expired"
                                : `Ends ${fmtShort(m.min_term_end_date)}`
                              }
                            </p>
                          </div>
                          <div className="text-right">
                            <p className="text-[18px] font-semibold text-gray-900 tabular-nums">
                              ${m.payment_amount?.toFixed(2) || "0.00"}
                            </p>
                            <p className="text-[13px] text-gray-400 capitalize">
                              {m.payment_frequency || "—"}
                            </p>
                          </div>
                        </div>

                        {/* Meta row */}
                        <div className="flex items-center gap-3 mt-4 pt-3 border-t border-gray-100">
                          <span className={`badge ${m.status === "active" ? "badge-green" : m.status === "expired" ? "badge-gray" : "badge-red"}`}>
                            {m.status.charAt(0).toUpperCase() + m.status.slice(1)}
                          </span>
                          <span className="text-[12px] text-gray-400">
                            {fmtShort(m.start_date)} — {fmtShort(m.min_term_end_date)}
                          </span>
                        </div>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}

          {tab === "billing" && (
            <BillingScheduleTab memberships={activeMemberships} fmtDate={fmtDate} />
          )}

          {tab === "payments" && (
            <div className="card overflow-hidden">
              <div className="card-header"><h3>All Payments</h3></div>
              {payments.length === 0 ? (
                <div className="px-5 py-8 text-center text-[13px] text-gray-400">No payments</div>
              ) : (
                <table className="data-table">
                  <thead><tr><th>Due Date</th><th>Paid Date</th><th>Amount</th><th>Status</th></tr></thead>
                  <tbody>
                    {payments.map((p, i) => (
                      <tr key={i}>
                        <td className="text-[13px]">{fmtShort(p.due_date)}</td>
                        <td className="text-[13px] text-gray-500">{p.paid_date ? fmtShort(p.paid_date) : "—"}</td>
                        <td className="text-[13px] tabular-nums font-medium">${p.amount.toFixed(2)}</td>
                        <td><span className={`badge ${p.status === "succeeded" ? "badge-green" : p.status === "failed" ? "badge-red" : "badge-yellow"}`}>{p.status}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {tab === "details" && (
            <div className="card">
              <div className="card-header"><h3>Client Details</h3></div>
              <div className="card-body">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <Detail label="Full name" value={client.name} />
                  <Detail label="Email" value={client.email} />
                  <Detail label="Phone" value={client.phone} />
                  <Detail label="Date of birth" value={client.birth_date ? fmtDate(client.birth_date) : undefined} />
                  <Detail label="Gender" value={client.gender} />
                  <Detail label="Address" value={[client.address, client.suburb, client.state, client.postcode].filter(Boolean).join(", ")} />
                  <Detail label="Emergency contact" value={client.emergency_contact_name} />
                  <Detail label="Emergency phone" value={client.emergency_contact_phone} />
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Right sidebar — client card */}
        <div className="space-y-5">
          {/* Client info card */}
          <div className="card">
            <div className="px-5 py-4 flex items-start gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-full bg-indigo-100 text-indigo-700 text-sm font-semibold flex-shrink-0">
                {initials}
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[15px] font-semibold text-gray-900">{client.name}</p>
                <p className="text-[12px] text-gray-400">CLI-{client.id.slice(0, 4).toUpperCase()}</p>
              </div>
            </div>

            <div className="border-t border-gray-100 px-5 py-3 space-y-2.5">
              <InfoRow label="Phone" value={client.phone || "—"} />
              <InfoRow label="Email" value={client.email || "—"} />
              <InfoRow label="Address" value={
                [client.address, client.suburb, client.state, client.postcode].filter(Boolean).join(", ") || "—"
              } />
              {client.gender && <InfoRow label="Gender" value={client.gender} />}
            </div>

            {client.emergency_contact_name && (
              <div className="border-t border-gray-100 px-5 py-3">
                <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium mb-1.5">Emergency Contact</p>
                <p className="text-[13px] text-gray-900">{client.emergency_contact_name}</p>
                {client.emergency_contact_phone && <p className="text-[12px] text-gray-500">{client.emergency_contact_phone}</p>}
              </div>
            )}

            <div className="border-t border-gray-100 px-5 py-3 text-[12px] text-gray-400">
              Member since {fmtShort(client.created_at)}
            </div>
          </div>

          {/* Stats card */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-100">
              <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Payment Summary</p>
            </div>
            <div className="px-5 py-3 space-y-2">
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Total paid</span>
                <span className="font-medium text-gray-900 tabular-nums">${totalPaid.toLocaleString(undefined, { minimumFractionDigits: 2 })}</span>
              </div>
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Payments</span>
                <span className="text-gray-700">{succeededPayments.length} succeeded</span>
              </div>
              {failedPayments.length > 0 && (
                <div className="flex justify-between text-[13px]">
                  <span className="text-gray-500">Failed</span>
                  <span className="text-red-600 font-medium">{failedPayments.length}</span>
                </div>
              )}
              <div className="flex justify-between text-[13px]">
                <span className="text-gray-500">Active memberships</span>
                <span className="text-gray-700">{activeMemberships.length}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4">
      <span className="text-[13px] text-gray-500 flex-shrink-0">{label}</span>
      <span className="text-[13px] text-gray-900 text-right">{value}</span>
    </div>
  );
}

function BillingScheduleTab({ memberships, fmtDate }: { memberships: Membership[]; fmtDate: (d: string) => string }) {
  // Generate upcoming payment dates for active memberships
  const upcoming: { date: Date; dateStr: string; membership: Membership }[] = [];

  memberships.forEach(m => {
    if (!m.payment_amount || !m.payment_frequency) return;
    const end = new Date(m.min_term_end_date);
    const now = new Date();
    let next = new Date(now);

    // Find next payment date
    if (m.payment_frequency === "weekly") {
      next.setDate(next.getDate() + (7 - next.getDay()) % 7 || 7);
    } else if (m.payment_frequency === "fortnightly") {
      next.setDate(next.getDate() + 14 - (next.getDate() % 14));
    } else {
      next = new Date(now.getFullYear(), now.getMonth() + 1, 1);
    }

    // Generate next 6 payments
    for (let i = 0; i < 6 && next <= end; i++) {
      upcoming.push({ date: new Date(next), dateStr: next.toISOString(), membership: m });
      if (m.payment_frequency === "weekly") next.setDate(next.getDate() + 7);
      else if (m.payment_frequency === "fortnightly") next.setDate(next.getDate() + 14);
      else next.setMonth(next.getMonth() + 1);
    }
  });

  upcoming.sort((a, b) => a.date.getTime() - b.date.getTime());

  // Group by month
  const grouped: Record<string, typeof upcoming> = {};
  upcoming.forEach(u => {
    const key = u.date.toLocaleDateString("en-AU", { month: "long", year: "numeric" });
    if (!grouped[key]) grouped[key] = [];
    grouped[key].push(u);
  });

  if (memberships.length === 0) {
    return <div className="card px-5 py-12 text-center text-[13px] text-gray-400">No active memberships to schedule</div>;
  }

  return (
    <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
      <div className="lg:col-span-2 space-y-5">
        {/* Upcoming payments */}
        <div>
          <h2 className="text-sm font-semibold text-gray-900 mb-3">Billed automatically</h2>
          {Object.entries(grouped).map(([month, items]) => (
            <div key={month} className="card overflow-hidden mb-4">
              <div className="bg-gray-50 px-5 py-2.5 border-b border-gray-200">
                <p className="text-[13px] font-medium text-gray-700">{month}</p>
              </div>
              <div className="divide-y divide-gray-100">
                {items.map((item, i) => (
                  <div key={i} className="px-5 py-3">
                    <p className="text-[12px] text-gray-400 mb-1">
                      MEM-{item.membership.id.slice(0, 4).toUpperCase()}
                    </p>
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-[13px] font-medium text-gray-900">Membership Payment</p>
                        <p className="text-[12px] text-gray-500 capitalize">{item.membership.payment_frequency}</p>
                      </div>
                      <div className="text-right">
                        <p className="text-[15px] font-semibold text-gray-900 tabular-nums">${item.membership.payment_amount?.toFixed(2)}</p>
                      </div>
                    </div>
                    <div className="flex items-center justify-between mt-2 pt-2 border-t border-gray-50 text-[12px] text-gray-400">
                      <span>Due {item.date.toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" })}</span>
                      <span>Total ${item.membership.payment_amount?.toFixed(2)}</span>
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
      </div>

      {/* Sidebar info */}
      <div className="space-y-5">
        <div className="card">
          <div className="px-5 py-4">
            <h3 className="text-sm font-semibold text-gray-900 mb-2">Billing schedule</h3>
            <p className="text-[12px] text-gray-500 leading-relaxed">
              View upcoming scheduled payments for this client. Payments are automatically debited on the agreed frequency.
            </p>
          </div>
        </div>

        <div className="card">
          <div className="px-5 py-3 border-b border-gray-100 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">Payment method</h3>
          </div>
          <div className="px-5 py-3">
            <p className="text-[13px] text-gray-500 mb-2">Client pays with</p>
            <div className="flex items-center gap-2">
              <div className="flex h-8 w-10 items-center justify-center rounded bg-gray-100">
                <svg className="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z" /></svg>
              </div>
              <div>
                <p className="text-[13px] text-gray-700">Direct Debit</p>
                <p className="text-[11px] text-gray-400">Bank account on file</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Detail({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <p className="text-[12px] text-gray-400 mb-0.5">{label}</p>
      <p className="text-[13px] text-gray-900">{value || "—"}</p>
    </div>
  );
}
