import { useState, useEffect } from "react";
import ReactECharts from "echarts-for-react";

interface MonthlyData { month: string; revenue: number; }
interface BreakdownData { type_title: string; revenue: number; count: number; }
interface StatusData { [key: string]: number; }
interface Summary { active_members: number; month_revenue: number; failed_payments: number; new_signups: number; }

const COLOURS = ["#6366f1", "#06b6d4", "#f59e0b", "#10b981", "#f43f5e", "#8b5cf6", "#14b8a6"];
const STATUS_COLOURS: Record<string, string> = { succeeded: "#10b981", failed: "#ef4444", pending: "#f59e0b", disputed: "#f97316", refunded: "#9ca3af" };

export default function ReportsPage() {
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);
  const [breakdown, setBreakdown] = useState<BreakdownData[]>([]);
  const [statuses, setStatuses] = useState<StatusData>({});
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"revenue" | "memberships" | "payments">("revenue");

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    Promise.all([
      fetch("/api/tenant/reports/revenue/monthly", { headers: h }).then(r => r.json()),
      fetch("/api/tenant/reports/memberships/breakdown", { headers: h }).then(r => r.json()),
      fetch("/api/tenant/reports/payments/status", { headers: h }).then(r => r.json()),
      fetch("/api/tenant/reports/dashboard", { headers: h }).then(r => r.json()),
    ]).then(([m, b, s, d]) => {
      setMonthly(m.months || []);
      setBreakdown(b.breakdown || []);
      setStatuses(s.statuses || {});
      setSummary(d);
    }).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const totalRevenue = monthly.reduce((s, m) => s + m.revenue, 0);
  const totalPayments = Object.values(statuses).reduce((a, b) => a + b, 0);
  const successRate = totalPayments > 0 ? Math.round(((statuses.succeeded || 0) / totalPayments) * 100) : 0;

  const tabs = [
    { key: "revenue" as const, label: "Revenue" },
    { key: "memberships" as const, label: "Memberships" },
    { key: "payments" as const, label: "Payments" },
  ];

  if (loading) return <div className="card px-5 py-16 text-center text-[13px] text-gray-400">Loading reports...</div>;

  /* ─── Revenue Area ─── */
  const revenueArea = {
    tooltip: { trigger: "axis" as const, formatter: (params: any) => `<b>${params[0].name}</b><br/>Revenue: <b>$${params[0].value.toLocaleString()}</b>` },
    grid: { left: 55, right: 20, top: 30, bottom: 30 },
    xAxis: { type: "category" as const, data: monthly.map(m => m.month), axisLine: { lineStyle: { color: "#e5e7eb" } }, axisLabel: { color: "#9ca3af", fontSize: 11 } },
    yAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${(v/1000).toFixed(1)}k` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    series: [{
      data: monthly.map(m => m.revenue), type: "line" as const, smooth: true, symbol: "circle", symbolSize: 6,
      lineStyle: { width: 3, color: "#6366f1" }, itemStyle: { color: "#6366f1" },
      areaStyle: { color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [{ offset: 0, color: "rgba(99,102,241,0.35)" }, { offset: 1, color: "rgba(99,102,241,0.02)" }]
      }},
    }],
  };

  /* ─── Revenue Bar ─── */
  const revenueBar = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 55, right: 20, top: 20, bottom: 30 },
    xAxis: { type: "category" as const, data: monthly.map(m => m.month), axisLabel: { color: "#9ca3af", fontSize: 11 } },
    yAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    series: [{
      type: "bar" as const, data: monthly.map(m => m.revenue), barWidth: 20,
      itemStyle: { color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [{ offset: 0, color: "#818cf8" }, { offset: 1, color: "#6366f1" }]
      }, borderRadius: [4, 4, 0, 0] },
    }],
  };

  /* ─── Revenue by Type Horizontal Bar ─── */
  const revenueByType = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 10, right: 40, top: 10, bottom: 10, containLabel: true },
    xAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    yAxis: { type: "category" as const, data: breakdown.map(b => b.type_title), axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: "#374151", fontSize: 12 } },
    series: [{
      type: "bar" as const, barWidth: 24,
      data: breakdown.map((b, i) => ({ value: b.revenue, itemStyle: { color: COLOURS[i % COLOURS.length], borderRadius: [0, 4, 4, 0] } })),
    }],
  };

  /* ─── Membership Pie ─── */
  const membershipPie = {
    tooltip: { trigger: "item" as const, formatter: "{b}<br/>{c} members ({d}%)" },
    legend: { bottom: 0, textStyle: { color: "#6b7280", fontSize: 11 } },
    series: [{
      type: "pie" as const, radius: "65%", center: ["50%", "42%"],
      itemStyle: { borderRadius: 4, borderColor: "#fff", borderWidth: 2 },
      label: { formatter: "{b}\n{d}%", fontSize: 11, color: "#374151" },
      emphasis: { itemStyle: { shadowBlur: 10, shadowColor: "rgba(0,0,0,0.15)" } },
      data: breakdown.map((b, i) => ({ value: b.count, name: b.type_title, itemStyle: { color: COLOURS[i % COLOURS.length] } })),
    }],
  };

  /* ─── Membership Revenue Donut ─── */
  const membershipRevDonut = {
    tooltip: { trigger: "item" as const, formatter: "{b}<br/>${c} ({d}%)" },
    legend: { bottom: 0, textStyle: { color: "#6b7280", fontSize: 11 } },
    series: [{
      type: "pie" as const, radius: ["40%", "65%"], center: ["50%", "42%"],
      itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 3 },
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 13, fontWeight: "bold" } },
      data: breakdown.map((b, i) => ({ value: b.revenue, name: b.type_title, itemStyle: { color: COLOURS[i % COLOURS.length] } })),
    }],
  };

  /* ─── Payment Status Donut ─── */
  const paymentDonut = {
    tooltip: { trigger: "item" as const, formatter: "{b}: {c} ({d}%)" },
    legend: { bottom: 0, textStyle: { color: "#6b7280", fontSize: 11 } },
    series: [{
      type: "pie" as const, radius: ["42%", "68%"], center: ["50%", "40%"],
      itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 3 },
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: "bold" } },
      data: Object.entries(statuses).map(([name, value]) => ({
        value, name: name.charAt(0).toUpperCase() + name.slice(1),
        itemStyle: { color: STATUS_COLOURS[name] || "#9ca3af" },
      })),
    }],
  };

  /* ─── Payment Funnel ─── */
  const paymentFunnel = {
    tooltip: { trigger: "item" as const, formatter: "{b}: {c}" },
    series: [{
      type: "funnel" as const,
      left: "10%", width: "80%", top: 10, bottom: 10,
      sort: "descending" as const,
      gap: 4,
      label: { show: true, position: "inside", formatter: "{b}\n{c}", fontSize: 12 },
      data: [
        { value: totalPayments, name: "Total", itemStyle: { color: "#6366f1" } },
        { value: statuses.succeeded || 0, name: "Succeeded", itemStyle: { color: "#10b981" } },
        { value: statuses.failed || 0, name: "Failed", itemStyle: { color: "#ef4444" } },
        { value: statuses.pending || 0, name: "Pending", itemStyle: { color: "#f59e0b" } },
      ].filter(d => d.value > 0),
    }],
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Reports</h1>
          <p className="text-[13px] text-gray-500 mt-0.5">Financial analytics and performance insights</p>
        </div>
      </div>

      {/* KPI Summary */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <KpiCard label="Total Revenue" value={`$${totalRevenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}`} sub={`${monthly.length} months tracked`} />
        <KpiCard label="Active Members" value={summary?.active_members || 0} sub={`${summary?.new_signups || 0} new this month`} />
        <KpiCard label="Success Rate" value={`${successRate}%`} sub={`${totalPayments} total payments`} />
        <KpiCard label="Avg Revenue / Month" value={`$${monthly.length > 0 ? (totalRevenue / monthly.length).toLocaleString(undefined, { minimumFractionDigits: 2 }) : "0.00"}`} />
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-4 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {/* Revenue Tab */}
      {tab === "revenue" && (
        <div className="space-y-5">
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Monthly Revenue Trend</h3></div>
            <div className="px-5 py-4">
              {monthly.length > 0 ? <ReactECharts option={revenueArea} style={{ height: 340 }} /> : <Empty />}
            </div>
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <div className="card overflow-hidden">
              <div className="card-header"><h3>Revenue by Month</h3></div>
              <div className="px-5 py-4">
                {monthly.length > 0 ? <ReactECharts option={revenueBar} style={{ height: 280 }} /> : <Empty />}
              </div>
            </div>
            <div className="card overflow-hidden">
              <div className="card-header"><h3>Revenue by Membership Type</h3></div>
              <div className="px-5 py-4">
                {breakdown.length > 0 ? <ReactECharts option={revenueByType} style={{ height: Math.max(150, breakdown.length * 50) }} /> : <Empty />}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Memberships Tab */}
      {tab === "memberships" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Membership Distribution</h3></div>
            <div className="px-5 py-4">
              {breakdown.length > 0 ? <ReactECharts option={membershipPie} style={{ height: 340 }} /> : <Empty />}
            </div>
          </div>
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Revenue per Membership</h3></div>
            <div className="px-5 py-4">
              {breakdown.length > 0 ? <ReactECharts option={membershipRevDonut} style={{ height: 340 }} /> : <Empty />}
            </div>
          </div>
        </div>
      )}

      {/* Payments Tab */}
      {tab === "payments" && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Payment Status Breakdown</h3></div>
            <div className="px-5 py-4">
              {totalPayments > 0 ? <ReactECharts option={paymentDonut} style={{ height: 340 }} /> : <Empty />}
            </div>
          </div>
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Payment Funnel</h3></div>
            <div className="px-5 py-4">
              {totalPayments > 0 ? <ReactECharts option={paymentFunnel} style={{ height: 340 }} /> : <Empty />}
            </div>
          </div>
          <div className="card overflow-hidden lg:col-span-2">
            <div className="card-header"><h3>Payment Metrics</h3></div>
            <div className="px-5 py-4">
              <div className="grid grid-cols-2 sm:grid-cols-5 gap-4">
                <MetricBox label="Total" value={totalPayments} />
                <MetricBox label="Succeeded" value={statuses.succeeded || 0} colour="text-emerald-600" />
                <MetricBox label="Failed" value={statuses.failed || 0} colour="text-red-600" />
                <MetricBox label="Disputed" value={statuses.disputed || 0} colour="text-amber-600" />
                <MetricBox label="Success Rate" value={`${successRate}%`} />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function KpiCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="card px-4 py-3">
      <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">{label}</p>
      <p className="text-2xl font-bold text-gray-900 mt-1 tabular-nums">{value}</p>
      {sub && <p className="text-[11px] text-gray-400 mt-0.5">{sub}</p>}
    </div>
  );
}

function MetricBox({ label, value, colour }: { label: string; value: string | number; colour?: string }) {
  return (
    <div className="rounded-lg bg-gray-50 px-4 py-3 text-center">
      <p className="text-[11px] text-gray-400 uppercase tracking-wider font-medium">{label}</p>
      <p className={`text-xl font-bold mt-1 tabular-nums ${colour || "text-gray-900"}`}>{value}</p>
    </div>
  );
}

function Empty() {
  return <div className="flex h-40 items-center justify-center text-[13px] text-gray-400">No data yet</div>;
}
