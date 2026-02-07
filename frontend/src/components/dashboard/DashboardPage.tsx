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

interface MonthlyData {
  month: string;
  revenue: number;
}

export default function DashboardPage() {
  const { user } = useAuth();
  const [summary, setSummary] = useState<Summary | null>(null);
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const headers = { Authorization: `Bearer ${token}` };

    fetch("/api/tenant/reports/dashboard", { headers })
      .then((r) => r.json())
      .then(setSummary)
      .catch(() => {});

    fetch("/api/tenant/reports/revenue/monthly", { headers })
      .then((r) => r.json())
      .then((d) => setMonthly(d.months || []))
      .catch(() => {});
  }, []);

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="mt-1 text-sm text-gray-500">
          Welcome back{user?.first_name ? `, ${user.first_name}` : ""}
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <SummaryCard
          title="Active Members"
          value={summary?.active_members ?? "—"}
          colour="primary"
          link="/clients"
        />
        <SummaryCard
          title="Revenue (Month)"
          value={summary ? `$${summary.month_revenue.toLocaleString()}` : "—"}
          colour="green"
          link="/reports"
        />
        <SummaryCard
          title="Failed Payments"
          value={summary?.failed_payments ?? "—"}
          colour="red"
          alert={!!summary && summary.failed_payments > 0}
          link="/payments"
        />
        <SummaryCard
          title="New Signups"
          value={summary?.new_signups ?? "—"}
          subtitle="This month"
          colour="blue"
        />
      </div>

      {/* Alert badges */}
      {summary && (summary.failed_payments > 0 || summary.open_disputes > 0) && (
        <div className="mt-4 flex flex-wrap gap-3">
          {summary.failed_payments > 0 && (
            <Link to="/payments" className="rounded-lg bg-red-50 border border-red-200 px-4 py-2 text-sm text-red-700 hover:bg-red-100 transition-colors">
              {summary.failed_payments} failed payment{summary.failed_payments !== 1 ? "s" : ""} need attention
            </Link>
          )}
          {summary.open_disputes > 0 && (
            <Link to="/payments" className="rounded-lg bg-orange-50 border border-orange-200 px-4 py-2 text-sm text-orange-700 hover:bg-orange-100 transition-colors">
              {summary.open_disputes} open dispute{summary.open_disputes !== 1 ? "s" : ""}
            </Link>
          )}
        </div>
      )}

      {/* Revenue chart */}
      <div className="mt-8 card">
        <h3 className="text-lg font-semibold text-gray-900 mb-4">Revenue Overview</h3>
        {monthly.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={monthly}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="month" tick={{ fontSize: 12 }} />
              <YAxis tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
              <Tooltip formatter={(value: number) => [`$${value.toFixed(2)}`, "Revenue"]} />
              <Bar dataKey="revenue" fill="#6366f1" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex h-64 items-center justify-center rounded-lg bg-gray-50 text-sm text-gray-400">
            Revenue data will appear as payments are collected
          </div>
        )}
      </div>
    </div>
  );
}

function SummaryCard({
  title, value, subtitle, colour, alert, link,
}: {
  title: string;
  value: string | number;
  subtitle?: string;
  colour: string;
  alert?: boolean;
  link?: string;
}) {
  const colours: Record<string, string> = {
    primary: "border-l-primary-500", green: "border-l-green-500",
    red: "border-l-red-500", blue: "border-l-blue-500",
  };

  const content = (
    <div className={`card border-l-4 ${colours[colour] || colours.primary} ${link ? "hover:shadow-md transition-shadow cursor-pointer" : ""}`}>
      <div className="flex items-start justify-between">
        <p className="text-sm font-medium text-gray-500">{title}</p>
        {alert && (
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-[10px] text-white font-bold">!</span>
        )}
      </div>
      <p className="mt-1 text-3xl font-bold text-gray-900">{value}</p>
      {subtitle && <p className="mt-1 text-xs text-gray-400">{subtitle}</p>}
    </div>
  );

  return link ? <Link to={link}>{content}</Link> : content;
}
