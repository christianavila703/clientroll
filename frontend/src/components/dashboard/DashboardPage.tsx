import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts";

interface Summary { active_members: number; month_revenue: number; failed_payments: number; new_signups: number; open_disputes: number; }
interface MonthlyData { month: string; revenue: number; }
interface BreakdownData { type_title: string; revenue: number; count: number; }
interface RecentClient { id: string; name: string; email?: string; created_at: string; }

export default function DashboardPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);
  const [breakdown, setBreakdown] = useState<BreakdownData[]>([]);
  const [recentClients, setRecentClients] = useState<RecentClient[]>([]);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    fetch("/api/tenant/reports/dashboard", { headers: h }).then(r => r.json()).then(setSummary).catch(() => {});
    fetch("/api/tenant/reports/revenue/monthly", { headers: h }).then(r => r.json()).then(d => setMonthly(d.months || [])).catch(() => {});
    fetch("/api/tenant/reports/memberships/breakdown", { headers: h }).then(r => r.json()).then(d => setBreakdown(d.breakdown || [])).catch(() => {});
    fetch("/api/tenant/clients?search=", { headers: h }).then(r => r.json()).then(d => setRecentClients((d.clients || []).slice(0, 7))).catch(() => {});
  }, []);

  const totalRevenue = monthly.reduce((s, m) => s + m.revenue, 0);
  const totalBreakdownRevenue = breakdown.reduce((s, b) => s + b.revenue, 0);

  return (
    <div>
      <h1 className="page-title mb-5">Home</h1>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2 space-y-5">

          {/* Billing section */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-200 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-semibold text-gray-900">Revenue</h2>
                <span className="text-[11px] text-gray-400">Current financial year</span>
              </div>
            </div>
            <div className="px-5 pt-4 pb-2">
              {monthly.length > 0 ? (
                <ResponsiveContainer width="100%" height={200}>
                  <BarChart data={monthly} barSize={20}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" vertical={false} />
                    <XAxis dataKey="month" tick={{ fontSize: 10, fill: "#9ca3af" }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fontSize: 10, fill: "#9ca3af" }} axisLine={false} tickLine={false} tickFormatter={v => `$${v >= 1000 ? `${(v/1000).toFixed(0)}K` : v}`} width={45} />
                    <Tooltip formatter={(v: number) => [`$${v.toLocaleString(undefined, { minimumFractionDigits: 2 })}`, "Revenue"]} contentStyle={{ fontSize: 11, borderRadius: 6, border: "1px solid #e5e7eb" }} />
                    <Bar dataKey="revenue" fill="#818cf8" radius={[3, 3, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex h-[200px] items-center justify-center text-[13px] text-gray-400">No revenue data yet</div>
              )}
            </div>
            <div className="grid grid-cols-3 divide-x divide-gray-100 border-t border-gray-200">
              <StatBox label="This month" value={`$${fmtK(summary?.month_revenue || 0)}`} />
              <StatBox label="Total revenue" value={`$${fmtK(totalRevenue)}`} sub={`/ $${fmtK(totalRevenue * 1.5)}`} progress={66} />
              <StatBox label="Active members" value={String(summary?.active_members || 0)} action="View" actionTo="/active-memberships" />
            </div>
          </div>

          {/* Payments section */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-200 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-semibold text-gray-900">Payments</h2>
                <span className="text-[11px] text-gray-400">Collected</span>
              </div>
            </div>
            <div className="grid grid-cols-3 divide-x divide-gray-100">
              <StatBox label="This month" value={`$${fmtK(summary?.month_revenue || 0)}`} progress={summary ? Math.min(Math.round((summary.month_revenue / Math.max(totalRevenue / Math.max(monthly.length, 1), 1)) * 100), 100) : 0} progressColour="bg-emerald-500" />
              <StatBox label="New signups" value={String(summary?.new_signups || 0)} />
              <StatBox label="Failed" value={String(summary?.failed_payments || 0)} valueColour={summary && summary.failed_payments > 0 ? "text-red-600" : undefined} />
            </div>
          </div>

          {/* Insights table */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-200 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-semibold text-gray-900">Insights</h2>
                <span className="text-[11px] text-gray-400">Membership performance</span>
              </div>
              <Link to="/reports" className="text-[12px] text-indigo-600 hover:text-indigo-700 font-medium flex items-center gap-1">
                <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" /></svg>
                Export
              </Link>
            </div>
            {breakdown.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th></th>
                      <th className="text-right">Revenue</th>
                      <th className="text-right">%</th>
                      <th className="text-right">Members</th>
                    </tr>
                  </thead>
                  <tbody>
                    {breakdown.map(b => (
                      <tr key={b.type_title}>
                        <td className="font-medium text-gray-900 text-[13px]">{b.type_title}</td>
                        <td className="text-right tabular-nums text-[13px]">${b.revenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}</td>
                        <td className="text-right text-gray-500 text-[13px]">{totalBreakdownRevenue > 0 ? Math.round((b.revenue / totalBreakdownRevenue) * 100) : 0}%</td>
                        <td className="text-right text-gray-500 text-[13px]">{b.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="px-5 py-8 text-center text-[13px] text-gray-400">No data yet</div>
            )}
          </div>
        </div>

        {/* Right sidebar */}
        <div className="space-y-5">
          {/* Action required */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-200">
              <h3 className="text-sm font-semibold text-gray-900">Action required</h3>
            </div>
            <div className="divide-y divide-gray-50">
              <Link to="/payments" className="flex items-center gap-3 px-5 py-3 hover:bg-gray-50 transition-colors">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-amber-50">
                  <svg className="h-4 w-4 text-amber-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                </div>
                <div>
                  <p className="text-[13px] text-gray-900">{summary?.failed_payments || 0} Failed payments</p>
                </div>
              </Link>
              <Link to="/payments" className="flex items-center gap-3 px-5 py-3 hover:bg-gray-50 transition-colors">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-orange-50">
                  <svg className="h-4 w-4 text-orange-500" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
                </div>
                <div>
                  <p className="text-[13px] text-gray-900">{summary?.open_disputes || 0} Open disputes</p>
                </div>
              </Link>
            </div>
          </div>

          {/* Recent clients */}
          <div className="card">
            <div className="px-5 py-3 border-b border-gray-200">
              <h3 className="text-sm font-semibold text-gray-900">Recent</h3>
            </div>
            <div className="divide-y divide-gray-50">
              {recentClients.map(c => (
                <Link key={c.id} to={`/clients/${c.id}`} className="flex items-center gap-3 px-5 py-2.5 hover:bg-gray-50 transition-colors">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-indigo-50 text-indigo-600 text-[11px] font-semibold flex-shrink-0">
                    {c.name.split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-[13px] font-medium text-gray-900 truncate">{c.name}</p>
                    <p className="text-[11px] text-gray-400 truncate">{c.email || "Client"}</p>
                  </div>
                </Link>
              ))}
              {recentClients.length === 0 && (
                <div className="px-5 py-6 text-center text-[13px] text-gray-400">No clients yet</div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatBox({ label, value, sub, action, actionTo, progress, progressColour, valueColour }: {
  label: string; value: string; sub?: string; action?: string; actionTo?: string;
  progress?: number; progressColour?: string; valueColour?: string;
}) {
  return (
    <div className="px-5 py-3">
      <p className="text-[11px] text-gray-500 mb-1">{label}</p>
      <div className="flex items-baseline gap-2">
        <p className={`text-lg font-semibold tabular-nums ${valueColour || "text-gray-900"}`}>{value}</p>
        {sub && <span className="text-[12px] text-gray-400 tabular-nums">{sub}</span>}
        {action && actionTo && (
          <Link to={actionTo} className="ml-auto rounded bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 hover:bg-gray-200 transition-colors">{action}</Link>
        )}
      </div>
      {typeof progress === "number" && (
        <div className="mt-2 h-1.5 bg-gray-100 rounded-full overflow-hidden">
          <div className={`h-full rounded-full ${progressColour || "bg-indigo-500"}`} style={{ width: `${Math.min(progress, 100)}%` }} />
        </div>
      )}
    </div>
  );
}

function fmtK(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}K`;
  return n.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 });
}
