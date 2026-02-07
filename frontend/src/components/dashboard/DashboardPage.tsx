import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts";

interface Summary {
  active_members: number;
  month_revenue: number;
  failed_payments: number;
  new_signups: number;
  open_disputes: number;
}

interface MonthlyData { month: string; revenue: number; }

export default function DashboardPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    fetch("/api/tenant/reports/dashboard", { headers: h }).then(r => r.json()).then(setSummary).catch(() => {});
    fetch("/api/tenant/reports/revenue/monthly", { headers: h }).then(r => r.json()).then(d => setMonthly(d.months || [])).catch(() => {});
  }, []);

  return (
    <div className="max-w-6xl">
      <h1 className="text-lg font-semibold text-gray-900 mb-5">Overview</h1>

      {/* Alerts */}
      {summary && summary.failed_payments > 0 && (
        <Link to="/payments" className="mb-5 flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[13px] text-amber-800 hover:bg-amber-100 transition-colors">
          <svg className="h-4 w-4 text-amber-500 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
          {summary.failed_payments} failed payment{summary.failed_payments !== 1 ? "s" : ""} require attention
        </Link>
      )}

      {/* Metric cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <MetricCard label="Active members" value={summary ? String(summary.active_members) : "–"} />
        <MetricCard label="Revenue this month" value={summary ? `$${summary.month_revenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}` : "–"} />
        <MetricCard label="Failed payments" value={summary ? String(summary.failed_payments) : "–"} alert={!!summary && summary.failed_payments > 0} />
        <MetricCard label="New signups" value={summary ? String(summary.new_signups) : "–"} />
      </div>

      {/* Revenue chart */}
      <div className="card mb-6">
        <div className="card-header">
          <h2>Revenue</h2>
          <span className="text-[12px] text-gray-400">Last 12 months</span>
        </div>
        <div className="px-5 py-4">
          {monthly.length > 0 ? (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={monthly} barSize={24}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" vertical={false} />
                <XAxis dataKey="month" tick={{ fontSize: 11, fill: "#9ca3af" }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fontSize: 11, fill: "#9ca3af" }} axisLine={false} tickLine={false} tickFormatter={v => `$${v >= 1000 ? `${(v/1000).toFixed(0)}K` : v}`} width={50} />
                <Tooltip formatter={(v: number) => [`$${v.toLocaleString(undefined, { minimumFractionDigits: 2 })}`, "Revenue"]} contentStyle={{ fontSize: 12, borderRadius: 6, border: "1px solid #e5e7eb", boxShadow: "0 1px 3px rgb(0 0 0 / 0.06)" }} />
                <Bar dataKey="revenue" fill="#6366f1" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex h-[240px] items-center justify-center text-sm text-gray-400">No revenue data yet</div>
          )}
        </div>
      </div>

      {/* Quick actions */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <QuickAction to="/clients" label="New client" desc="Add a new member" icon="M12 4.5v15m7.5-7.5h-15" />
        <QuickAction to="/memberships" label="Memberships" desc="Manage types" icon="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        <QuickAction to="/reports" label="Reports" desc="Financial data" icon="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
        <QuickAction to="/settings" label="Settings" desc="Configure account" icon="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </div>
    </div>
  );
}

function MetricCard({ label, value, alert }: { label: string; value: string; alert?: boolean }) {
  return (
    <div className="card px-5 py-4">
      <p className="text-[12px] font-medium text-gray-500 mb-1">{label}</p>
      <p className={`text-2xl font-semibold tabular-nums ${alert ? "text-red-600" : "text-gray-900"}`}>{value}</p>
    </div>
  );
}

function QuickAction({ to, label, desc, icon }: { to: string; label: string; desc: string; icon: string }) {
  return (
    <Link to={to} className="card flex items-center gap-3 px-4 py-3.5 hover:border-gray-300 hover:shadow-sm transition-all group">
      <div className="flex h-9 w-9 items-center justify-center rounded-md bg-gray-100 group-hover:bg-indigo-50 transition-colors">
        <svg className="h-4 w-4 text-gray-500 group-hover:text-indigo-600 transition-colors" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d={icon} /></svg>
      </div>
      <div>
        <p className="text-[13px] font-medium text-gray-900">{label}</p>
        <p className="text-[11px] text-gray-500">{desc}</p>
      </div>
    </Link>
  );
}
