import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import AdminPaymentAmounts from "../common/AdminPaymentAmounts";
import StripeTransactionPanel, { hasStripeDetails } from "./StripeTransactionPanel";
import { customerCharged, fmtMoney } from "../../utils/paymentLabels";
import type { AdminPaymentRecord } from "../../types/payments";

export default function PaymentsPage() {
  const navigate = useNavigate();
  const [payments, setPayments] = useState<AdminPaymentRecord[]>([]);
  const [filter, setFilter] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [stripeDetail, setStripeDetail] = useState<AdminPaymentRecord | null>(null);

  useEffect(() => { loadPayments(); }, [filter]);

  const loadPayments = async () => {
    setLoading(true);
    try {
      const endpoint = filter ? `/tenant/payments?status=${filter}` : "/tenant/payments";
      const token = localStorage.getItem("access_token");
      const res = await fetch(`/api${endpoint}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      setPayments(data.payments || []);
    } catch { /* handle */ }
    finally { setLoading(false); }
  };

  const filtered = payments.filter(p =>
    !search ||
    p.client_name.toLowerCase().includes(search.toLowerCase()) ||
    p.membership_type.toLowerCase().includes(search.toLowerCase())
  );

  const fmtDate = (d: string) => d ? new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" }) : "—";
  const fmtFreq = (f?: string) => f ? f.charAt(0).toUpperCase() + f.slice(1) : "—";
  const daysUntilEnd = (endDate: string) => {
    const diff = Math.ceil((new Date(endDate).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
    if (diff < 0) return { text: `Ended ${Math.abs(diff)}d ago`, colour: "text-gray-400" };
    if (diff <= 30) return { text: `${diff} days left`, colour: "text-amber-600" };
    return { text: `${diff} days left`, colour: "text-gray-500" };
  };

  const tabs = [
    { key: "", label: "All" },
    { key: "succeeded", label: "Succeeded" },
    { key: "failed", label: "Failed" },
    { key: "pending", label: "Pending" },
    { key: "disputed", label: "Disputed" },
  ];

  // Summary stats
  const totalCount = payments.length;
  const succeededCount = payments.filter(p => p.status === "succeeded").length;
  const failedCount = payments.filter(p => p.status === "failed").length;
  const succeeded = payments.filter(p => p.status === "succeeded");
  const totalCollected = succeeded.reduce((s, p) => s + customerCharged(p), 0);
  const totalTax = succeeded.reduce((s, p) => s + (Number(p.tax_amount ?? 0) || 0), 0);
  const totalProcessingFee = succeeded.reduce((s, p) => s + (Number(p.processing_fee_amount ?? 0) || 0), 0);
  const totalPayment = succeeded.reduce((s, p) => {
    const base = Number(p.base_amount ?? 0);
    if (!Number.isNaN(base) && base > 0) return s + base;
    const charged = customerCharged(p);
    return s + Math.max(0, charged - (Number(p.tax_amount ?? 0) || 0) - (Number(p.processing_fee_amount ?? 0) || 0));
  }, 0);

  return (
    <div>
      {stripeDetail && hasStripeDetails(stripeDetail.stripe_details) && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          <button
            type="button"
            className="fixed inset-0 bg-black/40 border-0 cursor-default w-full h-full"
            aria-label="Close"
            onClick={() => setStripeDetail(null)}
          />
          <div className="relative min-h-screen flex items-start justify-center pt-8 pb-12 px-4">
            <div
              role="dialog"
              aria-modal="true"
              aria-labelledby="stripe-tx-title"
              className="relative w-full max-w-lg bg-white rounded-xl shadow-2xl border border-gray-200"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200">
                <div>
                  <h2 id="stripe-tx-title" className="text-[15px] font-bold text-gray-900">
                    Transaction details
                  </h2>
                  <p className="text-[12px] text-gray-500 mt-0.5">
                    {stripeDetail.client_name}
                    {stripeDetail.client_email ? ` · ${stripeDetail.client_email}` : ""}
                  </p>
                </div>
                <button type="button" className="btn-secondary text-[12px]" onClick={() => setStripeDetail(null)}>
                  Close
                </button>
              </div>
              <div className="p-5 max-h-[min(80vh,720px)] overflow-y-auto">
                <StripeTransactionPanel
                  stripeDetails={stripeDetail.stripe_details}
                  currencyCode={stripeDetail.currency_code || "AUD"}
                  amount={customerCharged(stripeDetail)}
                  status={stripeDetail.status}
                  clientName={stripeDetail.client_name}
                  clientEmail={stripeDetail.client_email}
                />
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="page-header">
        <div>
          <h1 className="page-title">Payments</h1>
          <p className="text-[13px] text-gray-500">Track all payment activity across memberships and services</p>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-5 gap-4 mb-5">
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Total Payments</p>
          <p className="text-xl font-bold text-gray-900 mt-1 tabular-nums">{totalCount}</p>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Total Charged (Succeeded)</p>
          <p className="text-xl font-bold text-emerald-600 mt-1 tabular-nums">AUD ${fmtMoney(totalCollected)}</p>
          <p className="text-[10px] text-gray-500 mt-1">Includes tax and processing fees</p>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Breakdown (Succeeded)</p>
          <div className="mt-2 space-y-1 text-[12px]">
            <div className="flex items-center justify-between gap-2">
              <span className="text-gray-500">Payment</span>
              <span className="font-semibold text-gray-900 tabular-nums">AUD ${fmtMoney(totalPayment)}</span>
            </div>
            <div className="flex items-center justify-between gap-2">
              <span className="text-gray-500">Tax</span>
              <span className="font-semibold text-gray-900 tabular-nums">AUD ${fmtMoney(totalTax)}</span>
            </div>
            <div className="flex items-center justify-between gap-2">
              <span className="text-gray-500">Processing fee</span>
              <span className="font-semibold text-gray-900 tabular-nums">AUD ${fmtMoney(totalProcessingFee)}</span>
            </div>
          </div>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Succeeded</p>
          <p className="text-xl font-bold text-emerald-600 mt-1 tabular-nums">{succeededCount}</p>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Failed</p>
          <p className="text-xl font-bold text-red-600 mt-1 tabular-nums">{failedCount}</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setFilter(t.key)}
            className={`px-3 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              filter === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}{t.key === "failed" && failedCount > 0 ? ` (${failedCount})` : ""}</button>
        ))}
      </div>

      {/* Search + count */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative max-w-xs w-full">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" /></svg>
          <input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder="Search by client or membership..."
            className="w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-[13px] placeholder:text-gray-400 focus:border-gray-300 focus:bg-white focus:outline-none" />
        </div>
        <span className="text-[13px] text-gray-400">{filtered.length} results</span>
      </div>

      {loading ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-400">Loading...</div>
      ) : filtered.length === 0 ? (
        <div className="card px-5 py-12 text-center text-[13px] text-gray-500">No payments found</div>
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
                    <th className="min-w-[9rem]">Amount</th>
                    <th>Status</th>
                    <th>Due Date</th>
                    <th>Paid Date</th>
                    <th>Contract Paid</th>
                    <th>Contract Ends</th>
                    <th className="w-[1%] whitespace-nowrap">Stripe</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(p => {
                    const paidPct = p.total_expected > 0 ? Math.round((p.total_paid / p.total_expected) * 100) : 0;
                    const remaining = Math.max(0, p.total_expected - p.total_paid);
                    const end = daysUntilEnd(p.membership_end);
                    return (
                      <tr
                        key={p.payment_id}
                        className="cursor-pointer hover:bg-gray-50"
                        onClick={() => navigate(`/clients/${p.client_id}`)}
                      >
                        <td>
                          <p className="font-medium text-gray-900">{p.client_name}</p>
                          {p.client_email && <p className="text-[11px] text-gray-400">{p.client_email}</p>}
                        </td>
                        <td>
                          <p className="text-gray-900">{p.membership_type}</p>
                          <p className="text-[11px] text-gray-400">{p.term_months}mo · {fmtFreq(p.payment_frequency)}</p>
                        </td>
                        <td>
                          <AdminPaymentAmounts p={p} layout="compact" />
                        </td>
                        <td>
                          <span className={`badge ${p.status === "succeeded" || p.status === "confirmed" ? "badge-green" : p.status === "failed" ? "badge-red" : p.status === "pending" || p.status === "submitted" ? "badge-yellow" : "badge-gray"}`}>
                            {p.status === "succeeded" ? "paid" : p.status}
                          </span>
                          {p.provider_reference && (
                            <span className="ml-1 text-[10px] font-medium text-indigo-600 bg-indigo-50 rounded px-1 py-0.5">DD</span>
                          )}
                          {p.retry_count > 0 && (
                            <span className="ml-1 text-[10px] text-red-500">{p.retry_count} retries</span>
                          )}
                          {p.failure_reason && (
                            <p className="text-[10px] text-red-400 mt-0.5 truncate max-w-[150px]" title={p.failure_reason}>{p.failure_reason}</p>
                          )}
                        </td>
                        <td className="text-[13px] text-gray-700">{fmtDate(p.due_date)}</td>
                        <td className="text-[13px] text-gray-500">{p.paid_date ? fmtDate(p.paid_date) : "—"}</td>
                        <td>
                          <div className="w-28">
                            <div className="flex items-center justify-between text-[11px] mb-1">
                              <span className="text-gray-500 tabular-nums">{(p.currency_code || "AUD")} ${fmtMoney(p.total_paid)}</span>
                              <span className="text-gray-400 tabular-nums">{(p.currency_code || "AUD")} ${fmtMoney(p.total_expected)}</span>
                            </div>
                            <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                              <div
                                className={`h-full rounded-full ${paidPct >= 100 ? "bg-emerald-500" : paidPct >= 50 ? "bg-indigo-500" : "bg-amber-500"}`}
                                style={{ width: `${Math.min(paidPct, 100)}%` }}
                              />
                            </div>
                            <p className="text-[10px] text-gray-400 mt-0.5">AUD ${fmtMoney(remaining)} remaining</p>
                          </div>
                        </td>
                        <td>
                          <p className="text-[12px] text-gray-700">{fmtDate(p.membership_end)}</p>
                          <p className={`text-[11px] ${end.colour}`}>{end.text}</p>
                        </td>
                        <td onClick={(e) => e.stopPropagation()}>
                          {hasStripeDetails(p.stripe_details) ? (
                            <button
                              type="button"
                              className="text-[12px] font-medium text-indigo-600 hover:text-indigo-800"
                              onClick={() => setStripeDetail(p)}
                            >
                              Details
                            </button>
                          ) : (
                            <span className="text-[11px] text-gray-300">—</span>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {/* Mobile cards */}
          <div className="lg:hidden space-y-3">
            {filtered.map(p => {
              const remaining = Math.max(0, p.total_expected - p.total_paid);
              const end = daysUntilEnd(p.membership_end);
              return (
                <div
                  key={p.payment_id}
                  className="card px-4 py-3 cursor-pointer hover:shadow-md transition-shadow"
                  onClick={() => navigate(`/clients/${p.client_id}`)}
                >
                  <div className="flex items-start justify-between mb-2">
                    <div>
                      <p className="text-[13px] font-medium text-gray-900">{p.client_name}</p>
                      <p className="text-[12px] text-gray-500">{p.membership_type} · {p.term_months}mo</p>
                    </div>
                    <div className="text-right min-w-[8rem]">
                      <AdminPaymentAmounts p={p} layout="compact" />
                      <span className={`badge mt-1 inline-block ${p.status === "succeeded" ? "badge-green" : p.status === "failed" ? "badge-red" : "badge-yellow"}`}>
                        {p.status}
                      </span>
                      {hasStripeDetails(p.stripe_details) && (
                        <button
                          type="button"
                          className="mt-2 text-[12px] font-medium text-indigo-600 block w-full"
                          onClick={(e) => {
                            e.stopPropagation();
                            setStripeDetail(p);
                          }}
                        >
                          Stripe details
                        </button>
                      )}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-[12px]">
                    <div><span className="text-gray-400">Due</span><p className="text-gray-700">{fmtDate(p.due_date)}</p></div>
                    <div><span className="text-gray-400">Paid</span><p className="text-gray-700">{p.paid_date ? fmtDate(p.paid_date) : "—"}</p></div>
                    <div><span className="text-gray-400">Contract</span><p className="tabular-nums">AUD ${fmtMoney(p.total_paid)} / AUD ${fmtMoney(p.total_expected)}</p></div>
                    <div><span className="text-gray-400">Ends</span><p className={end.colour}>{fmtDate(p.membership_end)} ({end.text})</p></div>
                  </div>
                  {p.failure_reason && <p className="mt-2 text-[11px] text-red-500">{p.failure_reason}</p>}
                </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
