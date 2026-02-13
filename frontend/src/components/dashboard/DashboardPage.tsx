import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import ReactECharts from "echarts-for-react";

interface Summary { active_members: number; month_revenue: number; failed_payments: number; new_signups: number; open_disputes: number; active_mandates: number; active_subscriptions: number; pending_payments: number; }
interface MonthlyData { month: string; revenue: number; }
interface BreakdownData { type_title: string; revenue: number; count: number; }
interface StatusData { [key: string]: number; }
interface RecentClient { id: string; name: string; email?: string; created_at: string; }

const COLOURS = ["#6366f1", "#06b6d4", "#f59e0b", "#10b981", "#f43f5e", "#8b5cf6", "#14b8a6"];

export default function DashboardPage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);
  const [breakdown, setBreakdown] = useState<BreakdownData[]>([]);
  const [statuses, setStatuses] = useState<StatusData>({});
  const [recentClients, setRecentClients] = useState<RecentClient[]>([]);

  useEffect(() => {
    const token = localStorage.getItem("access_token");
    const h = { Authorization: `Bearer ${token}` };
    fetch("/api/tenant/reports/dashboard", { headers: h }).then(r => r.json()).then(setSummary).catch(() => {});
    fetch("/api/tenant/reports/revenue/monthly", { headers: h }).then(r => r.json()).then(d => setMonthly(d.months || [])).catch(() => {});
    fetch("/api/tenant/reports/memberships/breakdown", { headers: h }).then(r => r.json()).then(d => setBreakdown(d.breakdown || [])).catch(() => {});
    fetch("/api/tenant/reports/payments/status", { headers: h }).then(r => r.json()).then(d => setStatuses(d.statuses || {})).catch(() => {});
    fetch("/api/tenant/clients?search=", { headers: h }).then(r => r.json()).then(d => setRecentClients((d.clients || []).slice(0, 7))).catch(() => {});
  }, []);

  const totalRevenue = monthly.reduce((s, m) => s + m.revenue, 0);
  const prevMonthRevenue = monthly.length >= 2 ? monthly[monthly.length - 2]?.revenue || 0 : 0;
  const curMonthRevenue = summary?.month_revenue || 0;
  const revDelta = prevMonthRevenue > 0 ? Math.round(((curMonthRevenue - prevMonthRevenue) / prevMonthRevenue) * 100) : 0;

  const totalPayments = Object.values(statuses).reduce((a, b) => a + b, 0);

  /* ─── Revenue Chart — gradient bars + trend line ─── */
  const revenueChart = {
    tooltip: { trigger: "axis" as const, formatter: (params: any) => {
      const bar = params.find((p: any) => p.seriesType === "bar");
      return `<div style="font-size:13px"><b>${bar?.name || params[0]?.name}</b><br/>Revenue: <b>$${(bar?.value || params[0]?.value || 0).toLocaleString()}</b></div>`;
    }},
    grid: { left: 50, right: 20, top: 20, bottom: 55 },
    xAxis: { type: "category" as const, data: monthly.map(m => m.month),
      axisLine: { lineStyle: { color: "#f3f4f6" } },
      axisTick: { show: false },
      axisLabel: { color: "#9ca3af", fontSize: 11, interval: 0 } },
    yAxis: [
      { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${(v/1000).toFixed(0)}k` },
        splitLine: { lineStyle: { color: "#f9fafb", type: "dashed" as const } }, axisLine: { show: false }, axisTick: { show: false } },
      { type: "value" as const, show: false },
    ],
    series: [
      {
        name: "Revenue",
        type: "bar" as const,
        data: monthly.map(m => m.revenue),
        barWidth: "60%",
        itemStyle: {
          borderRadius: [4, 4, 0, 0],
          color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: "#a78bfa" },
              { offset: 0.5, color: "#c4b5fd" },
              { offset: 1, color: "#ede9fe" },
            ]
          },
        },
        emphasis: {
          itemStyle: {
            color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: "#7c3aed" },
                { offset: 0.5, color: "#a78bfa" },
                { offset: 1, color: "#ddd6fe" },
              ]
            },
          },
        },
      },
      {
        name: "Trend",
        type: "line" as const,
        yAxisIndex: 1,
        data: monthly.map(m => m.revenue),
        smooth: true,
        symbol: "circle",
        symbolSize: 5,
        lineStyle: { width: 2, color: "#8b5cf6", type: "dashed" as const },
        itemStyle: { color: "#8b5cf6", borderWidth: 2, borderColor: "#fff" },
        z: 10,
      },
    ],
  };

  /* ─── Payment Health Gauge ─── */
  const successRate = totalPayments > 0 ? Math.round(((statuses.succeeded || 0) / totalPayments) * 100) : 0;
  const paymentGauge = {
    series: [{
      type: "gauge" as const,
      startAngle: 200, endAngle: -20,
      min: 0, max: 100,
      pointer: { show: false },
      progress: { show: true, overlap: false, roundCap: true, clip: false,
        itemStyle: { color: successRate >= 90 ? "#10b981" : successRate >= 70 ? "#f59e0b" : "#ef4444" }
      },
      axisLine: { lineStyle: { width: 18, color: [[1, "#f3f4f6"]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false },
      detail: { valueAnimation: true, fontSize: 28, fontWeight: 700, color: "#111827",
        formatter: "{value}%", offsetCenter: [0, "0%"] },
      title: { fontSize: 12, color: "#9ca3af", offsetCenter: [0, "30%"] },
      data: [{ value: successRate, name: "Success Rate" }],
    }],
  };

  /* ─── Revenue by Type Bar ─── */
  const revenueByType = {
    tooltip: { trigger: "axis" as const, formatter: (params: any) => {
      const p = params[0];
      return `<div style="font-size:13px"><b>${p.name}</b><br/>$${p.value.toLocaleString()}</div>`;
    }},
    grid: { left: 10, right: 40, top: 10, bottom: 30, containLabel: true },
    xAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    yAxis: { type: "category" as const, data: breakdown.map(b => b.type_title), axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: "#374151", fontSize: 12 } },
    series: [{
      type: "bar" as const,
      data: breakdown.map((b, i) => ({ value: b.revenue, itemStyle: { color: COLOURS[i % COLOURS.length], borderRadius: [0, 4, 4, 0] } })),
      barWidth: 24,
    }],
  };

  /* ─── Membership Mix Donut ─── */
  const membershipDonut = {
    tooltip: { trigger: "item" as const, formatter: "{b}: {c} ({d}%)" },
    legend: { bottom: 0, left: "center", textStyle: { color: "#6b7280", fontSize: 12 } },
    series: [{
      type: "pie" as const,
      radius: ["45%", "70%"],
      center: ["50%", "40%"],
      avoidLabelOverlap: true,
      itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 3 },
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: "bold" } },
      data: breakdown.map((b, i) => ({ value: b.count, name: b.type_title, itemStyle: { color: COLOURS[i % COLOURS.length] } })),
    }],
  };

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Overview</h1>
      </div>

      {/* Alert */}
      {summary && summary.failed_payments > 0 && (
        <Link to="/payments" className="mb-5 flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-[13px] text-amber-800 hover:bg-amber-100 transition-colors">
          <svg className="h-4 w-4 text-amber-500 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
          <span><strong>{summary.failed_payments} failed payment{summary.failed_payments !== 1 ? "s" : ""}</strong> require attention</span>
        </Link>
      )}

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4 mb-6">
        <KpiCard label="Active Members" value={summary?.active_members ?? "–"} colour="indigo" />
        <KpiCard label="Revenue This Month" value={`$${curMonthRevenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}`} colour="emerald" delta={revDelta} />
        <KpiCard label="Failed Payments" value={summary?.failed_payments ?? "–"} colour="red" alert={(summary?.failed_payments || 0) > 0} />
        <KpiCard label="Active Direct Debits" value={summary?.active_subscriptions ?? "–"} colour="violet" />
        <KpiCard label="New Signups" value={summary?.new_signups ?? "–"} colour="blue" />
      </div>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Main */}
        <div className="lg:col-span-2 space-y-5">
          {/* Revenue Chart */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Revenue</h3><span className="text-[12px] text-gray-400">Monthly revenue over the past year</span></div>
            <div className="px-5 py-4">
              {monthly.length > 0 ? (
                <ReactECharts option={revenueChart} style={{ height: 280 }} />
              ) : (
                <div className="flex h-64 items-center justify-center text-[13px] text-gray-400">No revenue data yet</div>
              )}
              <div className="flex items-center justify-between mt-3 pt-3 border-t border-gray-100 text-[13px]">
                <div><span className="text-gray-500">Total Revenue</span><p className="font-semibold text-gray-900 tabular-nums">${totalRevenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}</p></div>
                <div className="text-right"><span className="text-gray-500">Avg Monthly</span><p className="font-semibold text-gray-900 tabular-nums">${monthly.length > 0 ? (totalRevenue / monthly.length).toLocaleString(undefined, { minimumFractionDigits: 2 }) : "0.00"}</p></div>
              </div>
            </div>
          </div>

          {/* Payment Health */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Payment Health</h3><span className="text-[12px] text-gray-400">{totalPayments} total payments</span></div>
            <div className="px-5 py-4">
              {totalPayments > 0 ? (
                <div className="flex items-center gap-6">
                  <ReactECharts option={paymentGauge} style={{ height: 180, width: 200 }} />
                  <div className="space-y-2 flex-1">
                    <StatRow label="Succeeded" value={statuses.succeeded || 0} colour="bg-emerald-500" />
                    <StatRow label="Failed" value={statuses.failed || 0} colour="bg-red-500" />
                    <StatRow label="Pending" value={statuses.pending || 0} colour="bg-amber-400" />
                    <StatRow label="Disputed" value={statuses.disputed || 0} colour="bg-orange-400" />
                  </div>
                </div>
              ) : (
                <div className="flex h-32 items-center justify-center text-[13px] text-gray-400">No payments yet</div>
              )}
            </div>
          </div>

          {/* Revenue by Type */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Revenue by Membership Type</h3></div>
            <div className="px-5 py-4">
              {breakdown.length > 0 ? (
                <ReactECharts option={revenueByType} style={{ height: Math.max(120, breakdown.length * 50) }} />
              ) : (
                <div className="flex h-24 items-center justify-center text-[13px] text-gray-400">No data yet</div>
              )}
            </div>
          </div>
        </div>

        {/* Sidebar */}
        <div className="space-y-5">
          {/* Membership Mix Donut */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Membership Mix</h3></div>
            <div className="px-3 py-4">
              {breakdown.length > 0 ? (
                <ReactECharts option={membershipDonut} style={{ height: 280 }} />
              ) : (
                <div className="flex h-40 items-center justify-center text-[13px] text-gray-400">No data yet</div>
              )}
            </div>
          </div>

          {/* Action Required */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Action Required</h3></div>
            <div className="divide-y divide-gray-100">
              <ActionItem label="Failed payments" count={summary?.failed_payments || 0} url="/payments" alert />
              <ActionItem label="Pending DD payments" count={summary?.pending_payments || 0} url="/payments" />
              <ActionItem label="Open disputes" count={summary?.open_disputes || 0} url="/payments" />
            </div>
          </div>

          {/* Recent Clients */}
          <div className="card overflow-hidden">
            <div className="card-header"><h3>Recent Clients</h3></div>
            <div className="divide-y divide-gray-100">
              {recentClients.map(c => (
                <Link key={c.id} to={`/clients/${c.id}`} className="flex items-center justify-between px-5 py-2.5 hover:bg-gray-50 transition-colors">
                  <span className="text-[13px] font-medium text-gray-900 hover:text-indigo-600">{c.name}</span>
                  <span className="text-[11px] text-gray-400">{new Date(c.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "short" })}</span>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Sub-components ─── */
function KpiCard({ label, value, colour, delta, alert }: { label: string; value: string | number; colour: string; delta?: number; alert?: boolean }) {
  const colourMap: Record<string, string> = {
    indigo: "border-t-indigo-500", emerald: "border-t-emerald-500", red: "border-t-red-500", blue: "border-t-blue-500",
  };
  return (
    <div className={`card px-4 py-3 border-t-2 ${colourMap[colour] || "border-t-gray-300"}`}>
      <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">{label}</p>
      <div className="flex items-end gap-2 mt-1">
        <p className={`text-2xl font-bold tabular-nums ${alert ? "text-red-600" : "text-gray-900"}`}>{value}</p>
        {delta !== undefined && delta !== 0 && (
          <span className={`text-[11px] font-semibold rounded-full px-1.5 py-0.5 mb-0.5 ${delta > 0 ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"}`}>
            {delta > 0 ? "↑" : "↓"} {Math.abs(delta)}%
          </span>
        )}
      </div>
    </div>
  );
}

function StatRow({ label, value, colour }: { label: string; value: number; colour: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className={`h-2.5 w-2.5 rounded-full ${colour} flex-shrink-0`} />
      <span className="text-[13px] text-gray-600 flex-1">{label}</span>
      <span className="text-[13px] font-semibold text-gray-900 tabular-nums">{value}</span>
    </div>
  );
}

function ActionItem({ label, count, url, alert }: { label: string; count: number; url: string; alert?: boolean }) {
  return (
    <Link to={url} className="flex items-center justify-between px-5 py-3 hover:bg-gray-50 transition-colors">
      <span className="text-[13px] text-gray-700">{label}</span>
      <span className={`text-[13px] font-semibold tabular-nums ${alert && count > 0 ? "text-red-600" : count > 0 ? "text-amber-600" : "text-gray-400"}`}>{count}</span>
    </Link>
  );
}
