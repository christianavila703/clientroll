import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import AdminPaymentAmounts from "../common/AdminPaymentAmounts";
import StripeTransactionPanel, { hasStripeDetails } from "./StripeTransactionPanel";
import { customerCharged, fmtMoney, formatFailedPaymentMessage, normalizeFailureReason } from "../../utils/paymentLabels";
import type { AdminPaymentRecord } from "../../types/payments";
import { useAuth } from "../../store/AuthContext";
import { daysUntilDateOnly, formatTenantDate, tenantTimezoneOrUTC } from "../../utils/tenantDate";

export default function PaymentsPage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const tenantTimezone = tenantTimezoneOrUTC(user?.tenant_timezone);
  const [payments, setPayments] = useState<AdminPaymentRecord[]>([]);
  const [filter, setFilter] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [stripeDetail, setStripeDetail] = useState<AdminPaymentRecord | null>(null);
  const [emailingReceiptId, setEmailingReceiptId] = useState<string | null>(null);

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

  const receiptSource = (p: AdminPaymentRecord) => (p.payment_source === "service" ? "service" : "membership");

  const downloadReceipt = async (p: AdminPaymentRecord) => {
    try {
      const token = localStorage.getItem("access_token");
      const res = await fetch(`/api/tenant/payments/${p.payment_id}/receipt.pdf?source=${receiptSource(p)}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        alert(body?.error || "Unable to generate the receipt PDF.");
        return;
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `Receipt-${p.payment_id.slice(0, 8)}.pdf`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch {
      alert("Unable to download the receipt PDF right now.");
    }
  };

  const emailReceipt = async (p: AdminPaymentRecord) => {
    if (!p.client_email) {
      alert("This client has no email address on file.");
      return;
    }
    if (!window.confirm(`Email a PDF receipt to ${p.client_email}?`)) return;
    setEmailingReceiptId(p.payment_id);
    try {
      const token = localStorage.getItem("access_token");
      const res = await fetch(`/api/tenant/payments/${p.payment_id}/send-receipt?source=${receiptSource(p)}`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });
      const body = await res.json().catch(() => null);
      if (!res.ok) {
        alert(body?.error || "Unable to send the receipt.");
        return;
      }
      alert(`Receipt ${body?.receipt_number || ""} sent to ${body?.sent_to || p.client_email}`.replace("  ", " "));
    } catch {
      alert("Unable to send the receipt right now.");
    } finally {
      setEmailingReceiptId(null);
    }
  };

  const filtered = payments.filter(p =>
    !search ||
    p.client_name.toLowerCase().includes(search.toLowerCase()) ||
    p.membership_type.toLowerCase().includes(search.toLowerCase())
  );

  const fmtDate = (d: string) => formatTenantDate(d, tenantTimezone, { day: "numeric", month: "short", year: "numeric" });
  const fmtFreq = (f?: string) => f ? f.charAt(0).toUpperCase() + f.slice(1) : "—";
  const methodBadge = (p: AdminPaymentRecord): { label: string; className: string } | null => {
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
  const daysUntilEnd = (endDate: string) => {
    const diff = daysUntilDateOnly(endDate, tenantTimezone);
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
                  retryCount={stripeDetail.retry_count}
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
                    <th className="min-w-[11rem]">Amount</th>
                    <th className="min-w-[11rem]">Status</th>
                    <th>Due Date</th>
                    <th>Paid Date</th>
                    <th className="min-w-[12rem]">Contract Paid</th>
                    <th className="min-w-[10rem]">Contract Ends</th>
                    <th className="w-[1%] whitespace-nowrap">Stripe</th>
                    <th className="w-[1%] whitespace-nowrap">Receipt</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(p => {
                    const paidPct = p.total_expected > 0 ? Math.round((p.total_paid / p.total_expected) * 100) : 0;
                    const remaining = Math.max(0, p.total_expected - p.total_paid);
                    const end = daysUntilEnd(p.membership_end);
                    const paymentMethodBadge = methodBadge(p);
                    const failedMsg = p.status === "failed" ? formatFailedPaymentMessage(p) : null;
                    const failureNote =
                      p.status !== "failed" ? normalizeFailureReason(p.failure_reason) : "";
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
                        <td className="align-top">
                          <AdminPaymentAmounts p={p} layout="compact" className="leading-tight" />
                        </td>
                        <td className="align-top">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className={`badge whitespace-nowrap ${p.status === "succeeded" || p.status === "confirmed" ? "badge-green" : p.status === "failed" ? "badge-red" : p.status === "pending" || p.status === "submitted" ? "badge-yellow" : "badge-gray"}`}>
                              {p.status === "succeeded" ? "Paid" : p.status}
                            </span>
                            {paymentMethodBadge && (
                              <span className={`badge whitespace-nowrap ${paymentMethodBadge.className}`}>
                                {paymentMethodBadge.label}
                              </span>
                            )}
                          </div>
                          {failedMsg ? (
                            <p className="text-[10px] text-red-400 mt-1 max-w-[240px] leading-snug" title={failedMsg}>
                              {failedMsg}
                            </p>
                          ) : (
                            failureNote && (
                              <p className="text-[10px] text-red-400 mt-1 truncate max-w-[180px]" title={failureNote}>
                                {failureNote}
                              </p>
                            )
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
                              className="text-[12px] font-medium text-indigo-600 hover:text-indigo-800 whitespace-nowrap"
                              onClick={() => setStripeDetail(p)}
                            >
                              Details
                            </button>
                          ) : (
                            <span className="text-[11px] text-gray-300">—</span>
                          )}
                        </td>
                        <td onClick={(e) => e.stopPropagation()}>
                          {p.status === "succeeded" ? (
                            <div className="flex items-center gap-2 whitespace-nowrap">
                              <button
                                type="button"
                                className="text-[12px] font-medium text-indigo-600 hover:text-indigo-800"
                                title="Download receipt PDF"
                                onClick={() => downloadReceipt(p)}
                              >
                                PDF
                              </button>
                              <button
                                type="button"
                                className="text-[12px] font-medium text-indigo-600 hover:text-indigo-800 disabled:text-gray-300"
                                title={p.client_email ? `Email receipt to ${p.client_email}` : "Client has no email on file"}
                                disabled={!p.client_email || emailingReceiptId === p.payment_id}
                                onClick={() => emailReceipt(p)}
                              >
                                {emailingReceiptId === p.payment_id ? "Sending…" : "Email"}
                              </button>
                            </div>
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
                      {p.status === "succeeded" && (
                        <div className="mt-2 flex justify-end gap-3" onClick={(e) => e.stopPropagation()}>
                          <button
                            type="button"
                            className="text-[12px] font-medium text-indigo-600"
                            onClick={() => downloadReceipt(p)}
                          >
                            Receipt PDF
                          </button>
                          <button
                            type="button"
                            className="text-[12px] font-medium text-indigo-600 disabled:text-gray-300"
                            disabled={!p.client_email || emailingReceiptId === p.payment_id}
                            onClick={() => emailReceipt(p)}
                          >
                            {emailingReceiptId === p.payment_id ? "Sending…" : "Email receipt"}
                          </button>
                        </div>
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
