import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";

interface Membership {
  membership_id: string;
  client_id: string;
  client_name: string;
  client_email?: string;
  client_phone?: string;
  membership_type: string;
  term_months: number;
  payment_frequency?: string;
  payment_amount?: number;
  payment_method?: string;
  status: string;
  start_date: string;
  end_date: string;
  mandate_id?: string;
  subscription_id?: string;
  last_payment_date?: string;
  last_payment_amount?: number;
  last_payment_status?: string;
  total_paid: number;
  total_expected: number;
  failed_count: number;
}

export default function ActiveMembershipsPage() {
  const navigate = useNavigate();
  const [memberships, setMemberships] = useState<Membership[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState("active");
  const [search, setSearch] = useState("");

  useEffect(() => { load(); }, [tab]);

  const load = async () => {
    setLoading(true);
    try {
      const token = localStorage.getItem("access_token");
      const res = await fetch(`/api/tenant/active-memberships?status=${tab}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      setMemberships(data.memberships || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  const filtered = memberships.filter(m =>
    !search || m.client_name.toLowerCase().includes(search.toLowerCase()) ||
    m.membership_type.toLowerCase().includes(search.toLowerCase())
  );

  const tabs = [
    { key: "active", label: "Active" },
    { key: "expired", label: "Expired" },
    { key: "cancelled", label: "Cancelled" },
    { key: "all", label: "All" },
  ];

  const statusBadge = (s: string) => {
    const map: Record<string, string> = {
      active: "badge-green", expired: "badge-gray", cancelled: "badge-red",
      pending: "badge-yellow", suspended: "badge-yellow",
    };
    return map[s] || "badge-gray";
  };

  const paymentBadge = (s?: string) => {
    if (!s) return "";
    const map: Record<string, string> = {
      succeeded: "badge-green", failed: "badge-red", pending: "badge-yellow", disputed: "badge-yellow",
    };
    return map[s] || "badge-gray";
  };

  const daysUntilEnd = (endDate: string) => {
    const diff = Math.ceil((new Date(endDate).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
    if (diff < 0) return { text: `${Math.abs(diff)}d overdue`, colour: "text-red-600" };
    if (diff <= 30) return { text: `${diff}d left`, colour: "text-amber-600" };
    return { text: `${diff}d left`, colour: "text-gray-500" };
  };

  const fmtDate = (d: string) => new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" });
  const fmtFreq = (f?: string) => f ? f.charAt(0).toUpperCase() + f.slice(1) : "—";

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Memberships</h1>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by client or type..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none" />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-500">No memberships found</div>
      ) : (
        <>
          {/* Desktop table */}
          <div className="card overflow-hidden hidden lg:block">
            <div className="overflow-x-auto">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Client</th>
                    <th>Membership</th>
                    <th>Term</th>
                    <th>Payment</th>
                    <th>Last Payment</th>
                    <th>Paid / Expected</th>
                    <th>Ends</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(m => {
                    const end = daysUntilEnd(m.end_date);
                    const paidPct = m.total_expected > 0 ? Math.round((m.total_paid / m.total_expected) * 100) : 0;
                    return (
                      <tr key={m.membership_id} onClick={() => navigate(`/clients/${m.client_id}`)} className="cursor-pointer hover:bg-gray-50">
                        <td>
                          <p className="font-medium text-gray-900">{m.client_name}</p>
                          <p className="text-[11px] text-gray-400">{m.client_email || m.client_phone || ""}</p>
                        </td>
                        <td className="text-gray-900">{m.membership_type}</td>
                        <td className="text-gray-500">{m.term_months}mo</td>
                        <td>
                          <p className="text-gray-900 tabular-nums">${m.payment_amount?.toFixed(2) || "—"}</p>
                          <p className="text-[11px] text-gray-400">{fmtFreq(m.payment_frequency)}</p>
                          {m.mandate_id ? (
                            <span className="inline-flex items-center gap-1 mt-0.5 text-[10px] font-medium text-green-700 bg-green-50 rounded px-1.5 py-0.5">
                              <svg className="h-2.5 w-2.5" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clipRule="evenodd" /></svg>
                              Direct Debit
                            </span>
                          ) : m.payment_method === "direct_debit" ? (
                            <span className="inline-flex items-center gap-1 mt-0.5 text-[10px] font-medium text-amber-700 bg-amber-50 rounded px-1.5 py-0.5">
                              No mandate
                            </span>
                          ) : null}
                        </td>
                        <td>
                          {m.last_payment_date ? (
                            <div>
                              <p className="text-gray-700 text-[12px]">{fmtDate(m.last_payment_date)}</p>
                              <div className="flex items-center gap-1 mt-0.5">
                                <span className={`badge ${paymentBadge(m.last_payment_status)}`}>{m.last_payment_status}</span>
                                {m.failed_count > 0 && <span className="badge badge-red">{m.failed_count} failed</span>}
                              </div>
                            </div>
                          ) : (
                            <span className="text-gray-400 text-[12px]">No payments</span>
                          )}
                        </td>
                        <td>
                          <div className="w-24">
                            <div className="flex items-center justify-between text-[11px] mb-1">
                              <span className="text-gray-500 tabular-nums">${m.total_paid.toLocaleString()}</span>
                              <span className="text-gray-400 tabular-nums">${m.total_expected.toLocaleString()}</span>
                            </div>
                            <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                              <div className={`h-full rounded-full ${paidPct >= 100 ? "bg-emerald-500" : paidPct >= 50 ? "bg-indigo-500" : "bg-amber-500"}`} style={{ width: `${Math.min(paidPct, 100)}%` }} />
                            </div>
                          </div>
                        </td>
                        <td>
                          <p className="text-[12px] text-gray-700">{fmtDate(m.end_date)}</p>
                          <p className={`text-[11px] ${end.colour}`}>{end.text}</p>
                        </td>
                        <td><span className={`badge ${statusBadge(m.status)}`}>{m.status}</span></td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {/* Mobile cards */}
          <div className="lg:hidden space-y-3">
            {filtered.map(m => {
              const end = daysUntilEnd(m.end_date);
              return (
                <div key={m.membership_id} className="card px-4 py-3 cursor-pointer hover:shadow-md transition-shadow" onClick={() => navigate(`/clients/${m.client_id}`)}>
                  <div className="flex items-start justify-between mb-2">
                    <div>
                      <p className="text-[13px] font-medium text-gray-900">{m.client_name}</p>
                      <p className="text-[12px] text-gray-500">{m.membership_type} · {m.term_months}mo</p>
                    </div>
                    <span className={`badge ${statusBadge(m.status)}`}>{m.status}</span>
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-[12px]">
                    <div><span className="text-gray-400">Payment</span><p className="text-gray-900 tabular-nums">${m.payment_amount?.toFixed(2) || "—"}/{m.payment_frequency?.slice(0,2) || ""}</p></div>
                    <div><span className="text-gray-400">Last paid</span><p className="text-gray-700">{m.last_payment_date ? fmtDate(m.last_payment_date) : "—"}</p></div>
                    <div><span className="text-gray-400">Paid</span><p className="tabular-nums">${m.total_paid.toLocaleString()} / ${m.total_expected.toLocaleString()}</p></div>
                    <div><span className="text-gray-400">Ends</span><p className={end.colour}>{fmtDate(m.end_date)} ({end.text})</p></div>
                  </div>
                  {m.failed_count > 0 && <p className="mt-2 text-[11px] text-red-600">{m.failed_count} failed payment{m.failed_count !== 1 ? "s" : ""}</p>}
                </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
