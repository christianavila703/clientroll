import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../../store/AuthContext";
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
  const { user } = useAuth();
  const [summary, setSummary] = useState<Summary | null>(null);
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const headers = { Authorization: `Bearer ${token}` };
    fetch("/api/tenant/reports/dashboard", { headers }).then((r) => r.json()).then(setSummary).catch(() => {});
    fetch("/api/tenant/reports/revenue/monthly", { headers }).then((r) => r.json()).then((d) => setMonthly(d.months || [])).catch(() => {});
  }, []);

  return (
    <div>
      {/* Page title */}
      <div className="mb-6">
        <h1 className="text-xl font-semibold text-gray-900">Home</h1>
      </div>

      {/* Alert banners */}
      {summary && (summary.failed_payments > 0 || summary.open_disputes > 0) && (
        <div className="mb-6 space-y-2">
          {summary.failed_payments > 0 && (
            <Link to="/payments" className="flex items-center gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 hover:bg-amber-100 transition-colors">
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-amber-100">
                <svg className="h-4 w-4 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </div>
              <div>
                <p className="text-sm font-medium text-amber-800">{summary.failed_payments} Failed payment{summary.failed_payments !== 1 ? "s" : ""}</p>
                <p className="text-xs text-amber-600">Action required</p>
              </div>
            </Link>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Main content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Billing section - Ignition style */}
          <div className="rounded-xl border border-gray-200 bg-white">
            <div className="flex items-center justify-between border-b border-gray-100 px-5 py-4">
              <h2 className="text-sm font-semibold text-gray-900">Revenue Overview</h2>
            </div>
            <div className="p-5">
              {monthly.length > 0 ? (
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={monthly} barSize={28}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" vertical={false} />
                    <XAxis dataKey="month" tick={{ fontSize: 11, fill: "#9ca3af" }} axisLine={false} tickLine={false} />
                    <YAxis tick={{ fontSize: 11, fill: "#9ca3af" }} axisLine={false} tickLine={false} tickFormatter={(v) => `$${v >= 1000 ? `${(v / 1000).toFixed(0)}K` : v}`} />
                    <Tooltip formatter={(value: number) => [`$${value.toLocaleString()}`, "Revenue"]} contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e5e7eb" }} />
                    <Bar dataKey="revenue" fill="#6366f1" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="flex h-[220px] items-center justify-center text-sm text-gray-400">
                  Revenue data will appear as payments are collected
                </div>
              )}
            </div>
            <div className="grid grid-cols-3 divide-x divide-gray-100 border-t border-gray-100">
              <StatCell label="This month" value={summary ? `$${formatNumber(summary.month_revenue)}` : "—"} />
              <StatCell label="Active members" value={summary ? String(summary.active_members) : "—"} />
              <StatCell label="New signups" value={summary ? String(summary.new_signups) : "—"} />
            </div>
          </div>

          {/* Payments section */}
          <div className="rounded-xl border border-gray-200 bg-white">
            <div className="flex items-center justify-between border-b border-gray-100 px-5 py-4">
              <h2 className="text-sm font-semibold text-gray-900">Payment Summary</h2>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-4 divide-x divide-gray-100">
              <StatCell label="Succeeded" value={summary ? "—" : "—"} colour="green" />
              <StatCell label="Failed" value={summary ? String(summary.failed_payments) : "—"} colour="red" />
              <StatCell label="Disputes" value={summary ? String(summary.open_disputes) : "—"} colour="amber" />
              <StatCell label="Pending" value="—" colour="gray" />
            </div>
          </div>
        </div>

        {/* Right sidebar - Action required + Recent */}
        <div className="space-y-6">
          {/* Action required */}
          <div className="rounded-xl border border-gray-200 bg-white">
            <div className="border-b border-gray-100 px-5 py-4">
              <h3 className="text-sm font-semibold text-gray-900">Action required</h3>
            </div>
            <div className="divide-y divide-gray-50">
              <ActionItem
                icon={<svg className="h-4 w-4 text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>}
                label={`${summary?.failed_payments || 0} Failed payments`}
                to="/payments"
              />
              <ActionItem
                icon={<svg className="h-4 w-4 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>}
                label={`${summary?.open_disputes || 0} Open disputes`}
                to="/payments"
              />
            </div>
          </div>

          {/* Quick links */}
          <div className="rounded-xl border border-gray-200 bg-white">
            <div className="border-b border-gray-100 px-5 py-4">
              <h3 className="text-sm font-semibold text-gray-900">Quick actions</h3>
            </div>
            <div className="p-3 space-y-1">
              <QuickLink to="/clients" label="Add new client" />
              <QuickLink to="/memberships" label="Create membership type" />
              <QuickLink to="/reports" label="View reports" />
              <QuickLink to="/settings" label="Update settings" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatCell({ label, value, colour }: { label: string; value: string; colour?: string }) {
  const colourMap: Record<string, string> = { green: "text-green-600", red: "text-red-600", amber: "text-amber-600", gray: "text-gray-400" };
  return (
    <div className="px-5 py-4">
      <p className="text-xs text-gray-500 mb-1">{label}</p>
      <p className={`text-lg font-bold ${colour ? colourMap[colour] || "text-gray-900" : "text-gray-900"}`}>{value}</p>
    </div>
  );
}

function ActionItem({ icon, label, to }: { icon: React.ReactNode; label: string; to: string }) {
  return (
    <Link to={to} className="flex items-center gap-3 px-5 py-3 hover:bg-gray-50 transition-colors">
      {icon}
      <span className="text-sm text-gray-700">{label}</span>
    </Link>
  );
}

function QuickLink({ to, label }: { to: string; label: string }) {
  return (
    <Link to={to} className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm text-gray-600 hover:bg-gray-50 hover:text-gray-900 transition-colors">
      <svg className="h-3.5 w-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" /></svg>
      {label}
    </Link>
  );
}

function formatNumber(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}K`;
  return n.toLocaleString();
}
