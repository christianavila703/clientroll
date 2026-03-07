import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import api, { ApiError } from "../../api/client";

type TenantBillingDetail = {
  tenant: { id: string; name: string; slug: string };
  subscription: any;
  usage: any[];
  invoices: any[];
};

export default function PlatformTenantBillingDetailPage() {
  const { tenantId } = useParams<{ tenantId: string }>();
  const [data, setData] = useState<TenantBillingDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = () => {
    if (!tenantId) return;
    setLoading(true);
    setError("");
    api.getPlatformBillingTenant(tenantId)
      .then((d) => setData(d as TenantBillingDetail))
      .catch((e) => setError(e instanceof ApiError ? e.message : "Failed to load tenant billing"))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
  }, [tenantId]);

  const openPortal = async () => {
    if (!tenantId) return;
    setError("");
    try {
      const res = await api.createPlatformBillingPortalSession(tenantId, { return_url: window.location.href });
      const url = (res as any)?.url;
      if (url) window.open(url, "_blank", "noopener,noreferrer");
      else setError("Portal session did not return a URL");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to open billing portal");
    }
  };

  if (loading) return <div className="card px-5 py-16 text-center text-[13px] text-gray-400">Loading tenant billing...</div>;
  if (!data) return <div className="card px-5 py-16 text-center text-[13px] text-gray-500">{error || "No data"}</div>;

  const sub = data.subscription;

  return (
    <div className="max-w-5xl">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{data.tenant.name}</h1>
          <p className="mt-1 text-sm text-gray-500 font-mono">{data.tenant.slug}</p>
          <div className="mt-2">
            <Link to="/platform/billing" className="text-[13px] text-indigo-600 hover:text-indigo-700">← Back to billing</Link>
          </div>
        </div>
        <div className="flex gap-2">
          <button className="btn-secondary" onClick={load}>Refresh</button>
          <button className="btn-primary" onClick={openPortal} disabled={!sub?.stripe_customer_id}>Open Stripe portal</button>
        </div>
      </div>

      {error && <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 mb-6">
        <div className="card border-l-4 border-l-primary-500">
          <p className="text-sm font-medium text-gray-500">Subscription status</p>
          <p className="mt-1 text-2xl font-bold text-gray-900">{sub?.status || "—"}</p>
        </div>
        <div className="card border-l-4 border-l-green-500">
          <p className="text-sm font-medium text-gray-500">Active members</p>
          <p className="mt-1 text-2xl font-bold text-gray-900">{sub?.active_member_count ?? "—"}</p>
        </div>
        <div className="card border-l-4 border-l-indigo-500">
          <p className="text-sm font-medium text-gray-500">Current monthly (USD)</p>
          <p className="mt-1 text-2xl font-bold text-gray-900">{sub ? `$${Number(sub.current_monthly_amount_usd || 0).toFixed(2)}` : "—"}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 mb-6">
        <div className="card px-5 py-4">
          <h2 className="text-[13px] font-semibold text-gray-900 mb-2">Payment method</h2>
          <p className="text-[13px] text-gray-600">
            {sub?.has_valid_payment_method
              ? `${sub.card_brand || "card"} •••• ${sub.card_last4 || "—"} exp ${sub.card_exp_month || "—"}/${sub.card_exp_year || "—"}`
              : "No valid card on file"}
          </p>
        </div>
        <div className="card px-5 py-4">
          <h2 className="text-[13px] font-semibold text-gray-900 mb-2">Stripe IDs</h2>
          <p className="text-[12px] text-gray-600 font-mono break-all">Customer: {sub?.stripe_customer_id || "—"}</p>
          <p className="text-[12px] text-gray-600 font-mono break-all mt-1">Subscription: {sub?.stripe_subscription_id || "—"}</p>
        </div>
      </div>

      <div className="card px-5 py-4 mb-6">
        <h2 className="text-[13px] font-semibold text-gray-900 mb-3">Usage snapshots (last 90 days)</h2>
        {data.usage.length === 0 ? (
          <p className="text-[13px] text-gray-500">No usage snapshots.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Date</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Members</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Amount (USD)</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Invoiced</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {data.usage.map((u: any) => (
                  <tr key={u.id || `${u.snapshot_date}-${u.active_member_count}`}>
                    <td className="px-3 py-2 text-gray-700">{u.snapshot_date ? new Date(u.snapshot_date).toLocaleDateString() : "—"}</td>
                    <td className="px-3 py-2 text-gray-700">{u.active_member_count ?? "—"}</td>
                    <td className="px-3 py-2 text-gray-700">{u.amount_usd !== undefined ? `$${Number(u.amount_usd).toFixed(2)}` : "—"}</td>
                    <td className="px-3 py-2 text-gray-600">{u.invoiced ? "Yes" : "No"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="card px-5 py-4">
        <h2 className="text-[13px] font-semibold text-gray-900 mb-3">Invoices</h2>
        {data.invoices.length === 0 ? (
          <p className="text-[13px] text-gray-500">No invoices.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Created</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Status</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Amount (USD)</th>
                  <th className="px-3 py-2 text-left font-medium text-gray-500">Period</th>
                  <th className="px-3 py-2 text-right font-medium text-gray-500">Links</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {data.invoices.map((inv: any) => (
                  <tr key={inv.id || inv.stripe_invoice_id}>
                    <td className="px-3 py-2 text-gray-700">{inv.created_at ? new Date(inv.created_at).toLocaleDateString() : "—"}</td>
                    <td className="px-3 py-2 text-gray-700">{inv.status || "—"}</td>
                    <td className="px-3 py-2 text-gray-700">{inv.amount_usd !== undefined ? `$${Number(inv.amount_usd).toFixed(2)}` : "—"}</td>
                    <td className="px-3 py-2 text-gray-600">
                      {inv.period_start ? new Date(inv.period_start).toLocaleDateString() : "—"} → {inv.period_end ? new Date(inv.period_end).toLocaleDateString() : "—"}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <div className="flex justify-end gap-2">
                        {inv.hosted_invoice_url && (
                          <a className="btn-secondary text-[12px]" href={inv.hosted_invoice_url} target="_blank" rel="noreferrer">View</a>
                        )}
                        {inv.invoice_pdf_url && (
                          <a className="btn-secondary text-[12px]" href={inv.invoice_pdf_url} target="_blank" rel="noreferrer">PDF</a>
                        )}
                      </div>
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

