import { useState, useEffect } from "react";
import api from "../../api/client";

interface Payment {
  id: string;
  client_id: string;
  amount: number;
  status: string;
  due_date: string;
  paid_date?: string;
  failure_reason?: string;
  retry_count: number;
  created_at: string;
}

export default function PaymentsPage() {
  const [payments, setPayments] = useState<Payment[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);

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

  const statusColours: Record<string, string> = {
    pending: "bg-yellow-100 text-yellow-700",
    succeeded: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
    disputed: "bg-orange-100 text-orange-700",
    refunded: "bg-gray-100 text-gray-700",
  };

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Payments</h1>
        <p className="mt-1 text-sm text-gray-500">Track all payment activity</p>
      </div>

      {/* Filters */}
      <div className="mb-6 flex flex-wrap gap-2">
        {["", "pending", "succeeded", "failed", "disputed"].map((s) => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={`rounded-full px-4 py-2 text-sm font-medium min-h-[44px] transition-colors ${
              filter === s ? "bg-primary-100 text-primary-700" : "bg-gray-100 text-gray-600 hover:bg-gray-200"
            }`}
          >
            {s || "All"}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="card text-center text-gray-500">Loading...</div>
      ) : payments.length === 0 ? (
        <div className="card text-center text-gray-500 py-12">No payments found</div>
      ) : (
        <div className="card overflow-hidden p-0">
          {/* Desktop */}
          <div className="hidden md:block overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Amount</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Due Date</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Paid Date</th>
                  <th className="px-4 py-3 text-left font-medium text-gray-500">Retries</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {payments.map((p) => (
                  <tr key={p.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3 font-medium text-gray-900">${p.amount.toFixed(2)}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${statusColours[p.status] || "bg-gray-100"}`}>
                        {p.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-600">{new Date(p.due_date).toLocaleDateString()}</td>
                    <td className="px-4 py-3 text-gray-600">{p.paid_date ? new Date(p.paid_date).toLocaleDateString() : "—"}</td>
                    <td className="px-4 py-3 text-gray-600">{p.retry_count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {/* Mobile */}
          <div className="md:hidden divide-y divide-gray-100">
            {payments.map((p) => (
              <div key={p.id} className="p-4">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-gray-900">${p.amount.toFixed(2)}</span>
                  <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${statusColours[p.status] || "bg-gray-100"}`}>
                    {p.status}
                  </span>
                </div>
                <p className="mt-1 text-sm text-gray-500">Due: {new Date(p.due_date).toLocaleDateString()}</p>
                {p.failure_reason && <p className="mt-1 text-xs text-red-500">{p.failure_reason}</p>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
