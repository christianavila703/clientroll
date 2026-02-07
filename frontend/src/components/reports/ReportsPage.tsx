import { useState, useEffect } from "react";
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
} from "recharts";

interface MonthlyData {
  month: string;
  revenue: number;
}

interface BreakdownData {
  type_title: string;
  revenue: number;
  count: number;
}

const COLOURS = ["#6366f1", "#22c55e", "#ef4444", "#f59e0b", "#3b82f6", "#8b5cf6", "#ec4899"];

export default function ReportsPage() {
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);
  const [breakdown, setBreakdown] = useState<BreakdownData[]>([]);
  const [statuses, setStatuses] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadAll();
  }, []);

  const loadAll = async () => {
    const token = localStorage.getItem("access_token");
    const headers = { Authorization: `Bearer ${token}` };

    try {
      const [monthlyRes, breakdownRes, statusRes] = await Promise.all([
        fetch("/api/tenant/reports/revenue/monthly", { headers }),
        fetch("/api/tenant/reports/memberships/breakdown", { headers }),
        fetch("/api/tenant/reports/payments/status", { headers }),
      ]);

      const monthlyData = await monthlyRes.json();
      const breakdownData = await breakdownRes.json();
      const statusData = await statusRes.json();

      setMonthly(monthlyData.months || []);
      setBreakdown(breakdownData.breakdown || []);
      setStatuses(statusData.statuses || {});
    } catch {
      // Handle errors
    } finally {
      setLoading(false);
    }
  };

  const statusPieData = Object.entries(statuses).map(([name, value]) => ({ name, value }));

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-bold text-gray-900 mb-6">Reports</h1>
        <div className="card text-center text-gray-500">Loading reports...</div>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Reports</h1>
        <p className="mt-1 text-sm text-gray-500">Financial analytics and insights</p>
      </div>

      {/* Revenue chart */}
      <div className="card mb-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-4">Monthly Revenue</h3>
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
          <div className="flex h-64 items-center justify-center text-gray-400">
            No revenue data yet
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Membership breakdown */}
        <div className="card">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Revenue by Membership Type</h3>
          {breakdown.length > 0 ? (
            <>
              <ResponsiveContainer width="100%" height={250}>
                <PieChart>
                  <Pie
                    data={breakdown}
                    dataKey="revenue"
                    nameKey="type_title"
                    cx="50%"
                    cy="50%"
                    outerRadius={80}
                    label={({ type_title, percent }) =>
                      `${type_title} (${(percent * 100).toFixed(0)}%)`
                    }
                  >
                    {breakdown.map((_, i) => (
                      <Cell key={i} fill={COLOURS[i % COLOURS.length]} />
                    ))}
                  </Pie>
                  <Tooltip formatter={(value: number) => `$${value.toFixed(2)}`} />
                </PieChart>
              </ResponsiveContainer>
              <div className="mt-4 space-y-2">
                {breakdown.map((b, i) => (
                  <div key={b.type_title} className="flex items-center justify-between text-sm">
                    <div className="flex items-center gap-2">
                      <div className="h-3 w-3 rounded-full" style={{ backgroundColor: COLOURS[i % COLOURS.length] }} />
                      <span className="text-gray-700">{b.type_title}</span>
                    </div>
                    <div className="text-right">
                      <span className="font-medium text-gray-900">${b.revenue.toLocaleString()}</span>
                      <span className="text-gray-400 ml-2">({b.count} members)</span>
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="flex h-64 items-center justify-center text-gray-400">
              No membership data yet
            </div>
          )}
        </div>

        {/* Payment status breakdown */}
        <div className="card">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Payment Status</h3>
          {statusPieData.length > 0 ? (
            <>
              <ResponsiveContainer width="100%" height={250}>
                <PieChart>
                  <Pie
                    data={statusPieData}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="50%"
                    outerRadius={80}
                  >
                    {statusPieData.map((entry, i) => {
                      const colours: Record<string, string> = {
                        succeeded: "#22c55e", failed: "#ef4444",
                        pending: "#f59e0b", disputed: "#f97316",
                        refunded: "#6b7280",
                      };
                      return <Cell key={i} fill={colours[entry.name] || COLOURS[i % COLOURS.length]} />;
                    })}
                  </Pie>
                  <Tooltip />
                  <Legend />
                </PieChart>
              </ResponsiveContainer>
            </>
          ) : (
            <div className="flex h-64 items-center justify-center text-gray-400">
              No payment data yet
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
