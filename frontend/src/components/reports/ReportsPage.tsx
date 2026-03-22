import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import ReactECharts from "echarts-for-react";
import api from "../../api/client";
import type { ComprehensiveReport } from "../../types/reports";

type Report = ComprehensiveReport;

function normalizeReport(raw: any): Report {
  const monthCollected = Number(raw?.month_collected_amount ?? raw?.month_revenue ?? 0);
  const prevMonthCollected = Number(raw?.prev_month_collected_amount ?? raw?.prev_month_revenue ?? 0);
  const totalCollected = Number(raw?.total_collected_amount ?? raw?.total_revenue ?? 0);
  const monthToOrganisation = Number(raw?.month_to_organisation_amount ?? raw?.organisation_month_revenue ?? 0);
  const prevMonthToOrganisation = Number(raw?.prev_month_to_organisation_amount ?? raw?.organisation_prev_month_revenue ?? 0);
  const totalToOrganisation = Number(raw?.total_to_organisation_amount ?? raw?.organisation_total_revenue ?? 0);
  return {
    active_members: Number(raw?.active_members || 0),
    total_clients: Number(raw?.total_clients || 0),
    month_collected_amount: monthCollected,
    prev_month_collected_amount: prevMonthCollected,
    total_collected_amount: totalCollected,
    month_revenue: monthCollected,
    prev_month_revenue: prevMonthCollected,
    total_revenue: totalCollected,
    month_tax_amount: Number(raw?.month_tax_amount || 0),
    prev_month_tax_amount: Number(raw?.prev_month_tax_amount || 0),
    total_tax_amount: Number(raw?.total_tax_amount || 0),
    month_surcharge_amount: Number(raw?.month_surcharge_amount || 0),
    prev_month_surcharge_amount: Number(raw?.prev_month_surcharge_amount || 0),
    total_surcharge_amount: Number(raw?.total_surcharge_amount || 0),
    month_to_organisation_amount: monthToOrganisation,
    prev_month_to_organisation_amount: prevMonthToOrganisation,
    total_to_organisation_amount: totalToOrganisation,
    organisation_month_revenue: monthToOrganisation,
    organisation_prev_month_revenue: prevMonthToOrganisation,
    organisation_total_revenue: totalToOrganisation,
    mrr: Number(raw?.mrr || 0),
    arr: Number(raw?.arr || 0),
    avg_revenue_per_member: Number(raw?.avg_revenue_per_member || 0),
    failed_payments: Number(raw?.failed_payments || 0),
    new_signups_month: Number(raw?.new_signups_month || 0),
    churned_month: Number(raw?.churned_month || 0),
    collection_rate: Number(raw?.collection_rate || 0),
    avg_membership_days: Number(raw?.avg_membership_days || 0),
    monthly_revenue: Array.isArray(raw?.monthly_revenue) ? raw.monthly_revenue : [],
    monthly_signups: Array.isArray(raw?.monthly_signups) ? raw.monthly_signups : [],
    monthly_churn: Array.isArray(raw?.monthly_churn) ? raw.monthly_churn : [],
    membership_breakdown: Array.isArray(raw?.membership_breakdown) ? raw.membership_breakdown : [],
    revenue_by_currency: Array.isArray(raw?.revenue_by_currency) ? raw.revenue_by_currency : [],
    payment_statuses: raw?.payment_statuses && typeof raw.payment_statuses === "object" ? raw.payment_statuses : {},
    frequency_breakdown: Array.isArray(raw?.frequency_breakdown) ? raw.frequency_breakdown : [],
    top_clients: Array.isArray(raw?.top_clients) ? raw.top_clients : [],
    upcoming_expirations: Array.isArray(raw?.upcoming_expirations) ? raw.upcoming_expirations : [],
    recent_failures: Array.isArray(raw?.recent_failures) ? raw.recent_failures : [],
  };
}

const COLOURS = ["#6366f1", "#06b6d4", "#f59e0b", "#10b981", "#f43f5e", "#8b5cf6", "#14b8a6", "#ec4899"];
const STATUS_COLOURS: Record<string, string> = { succeeded: "#10b981", failed: "#ef4444", pending: "#f59e0b", disputed: "#f97316" };
const fmtMoney = (value: number) =>
  value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export default function ReportsPage() {
  const [data, setData] = useState<Report | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<"overview" | "revenue" | "memberships" | "payments" | "clients">("overview");

  useEffect(() => {
    api.getComprehensiveReport()
      .then((payload) => setData(normalizeReport(payload)))
      .catch((e) => { console.error(e); setError("Unable to load reports."); })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="card px-5 py-16 text-center text-[13px] text-gray-400">Loading reports...</div>;
  if (!data) return <div className="card px-5 py-16 text-center text-[13px] text-gray-500">{error || "Unable to load report data."}</div>;
  const safeData = normalizeReport(data);

  const tabs = [
    { key: "overview" as const, label: "Overview" },
    { key: "revenue" as const, label: "Revenue" },
    { key: "memberships" as const, label: "Memberships" },
    { key: "payments" as const, label: "Payments" },
    { key: "clients" as const, label: "Clients" },
  ];

  const totalPayments = Object.values(safeData.payment_statuses).reduce((a, b) => a + b, 0);
  const successRate = totalPayments > 0 ? Math.round(((safeData.payment_statuses.succeeded || 0) / totalPayments) * 100) : 0;
  const revDelta = safeData.prev_month_revenue > 0 ? Math.round(((safeData.month_revenue - safeData.prev_month_revenue) / safeData.prev_month_revenue) * 100) : 0;
  const churnRate = safeData.active_members > 0 ? ((safeData.churned_month / safeData.active_members) * 100).toFixed(1) : "0";
  const fmtDate = (d: string) => new Date(d).toLocaleDateString("en-AU", { day: "numeric", month: "short", year: "numeric" });

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="page-title">Reports</h1>
          <p className="text-[13px] text-gray-500 mt-0.5">Business analytics and performance insights</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-5">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-4 py-2.5 text-[13px] font-medium border-b-2 -mb-px transition-colors ${
              tab === t.key ? "border-indigo-600 text-indigo-600" : "border-transparent text-gray-500 hover:text-gray-700"
            }`}>{t.label}</button>
        ))}
      </div>

      {tab === "overview" && <OverviewTab data={safeData} revDelta={revDelta} successRate={successRate} churnRate={churnRate} />}
      {tab === "revenue" && <RevenueTab data={safeData} />}
      {tab === "memberships" && <MembershipsTab data={safeData} />}
      {tab === "payments" && <PaymentsTab data={safeData} totalPayments={totalPayments} successRate={successRate} />}
      {tab === "clients" && <ClientsTab data={safeData} fmtDate={fmtDate} />}
    </div>
  );
}

/* ─── KPI Card ─── */
function Kpi({ label, value, sub, delta, alert }: { label: string; value: string | number; sub?: string; delta?: number; alert?: boolean }) {
  return (
    <div className="card px-4 py-3">
      <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">{label}</p>
      <div className="flex items-end gap-2 mt-1">
        <p className={`text-xl font-bold tabular-nums ${alert ? "text-red-600" : "text-gray-900"}`}>{value}</p>
        {delta !== undefined && delta !== 0 && (
          <span className={`badge mb-0.5 ${delta > 0 ? "badge-green" : "badge-red"}`}>
            {delta > 0 ? "↑" : "↓"} {Math.abs(delta)}%
          </span>
        )}
      </div>
      {sub && <p className="text-[11px] text-gray-400 mt-0.5">{sub}</p>}
    </div>
  );
}

/* ─── Overview Tab ─── */
function OverviewTab({ data, revDelta, successRate, churnRate }: { data: Report; revDelta: number; successRate: number; churnRate: string }) {
  data = normalizeReport(data as any);
  const revenueChart = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 50, right: 20, top: 20, bottom: 30 },
    xAxis: { type: "category" as const, data: data.monthly_revenue.map(m => m.month), axisLabel: { color: "#9ca3af", fontSize: 11 } },
    yAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${(v/1000).toFixed(0)}k` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    series: [{
      data: data.monthly_revenue.map(m => m.revenue), type: "line" as const, smooth: true, symbolSize: 6,
      lineStyle: { width: 3, color: "#6366f1" }, itemStyle: { color: "#6366f1" },
      areaStyle: { color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: "rgba(99,102,241,0.3)" }, { offset: 1, color: "rgba(99,102,241,0.02)" }] } },
    }],
  };

  return (
    <div className="space-y-5">
      {/* KPI row 1 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi
          label="Collected (this month)"
          value={`$${fmtMoney(data.month_collected_amount || data.month_revenue)}`}
          delta={revDelta}
          sub="Total from customers"
        />
        <Kpi label="MRR" value={`$${fmtMoney(data.mrr)}`} sub="Monthly Recurring Revenue" />
        <Kpi label="ARR" value={`$${fmtMoney(data.arr)}`} sub="Annual Recurring Revenue" />
        <Kpi
          label="Collected (all time)"
          value={`$${fmtMoney(data.total_collected_amount || data.total_revenue)}`}
          sub="Total from customers"
        />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Tax This Month</p>
          <p className="text-xl font-bold text-gray-900 mt-1 tabular-nums">${fmtMoney(data.month_tax_amount || 0)}</p>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Tax Last Month</p>
          <p className="text-xl font-bold text-gray-900 mt-1 tabular-nums">${fmtMoney(data.prev_month_tax_amount || 0)}</p>
        </div>
        <div className="card px-4 py-3">
          <p className="text-[11px] uppercase tracking-wider text-gray-400 font-medium">Tax All Time</p>
          <p className="text-xl font-bold text-gray-900 mt-1 tabular-nums">${fmtMoney(data.total_tax_amount || 0)}</p>
        </div>
      </div>

      {/* KPI row 2 */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        <Kpi label="Active Members" value={data.active_members} sub={`${data.new_signups_month} new this month`} />
        <Kpi label="Collection Rate" value={`${data.collection_rate.toFixed(1)}%`} />
        <Kpi label="Success Rate" value={`${successRate}%`} />
        <Kpi label="Churn Rate" value={`${churnRate}%`} sub={`${data.churned_month} this month`} />
        <Kpi label="Failed Payments" value={data.failed_payments} alert={data.failed_payments > 0} />
      </div>

      {/* Revenue trend */}
      <div className="card overflow-hidden">
        <div className="card-header"><h3>Revenue Trend</h3></div>
        <div className="px-5 py-4">
          {data.monthly_revenue.length > 0 ? <ReactECharts option={revenueChart} style={{ height: 280 }} /> : <Empty />}
        </div>
      </div>
      {(data.revenue_by_currency || []).length > 0 && (
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Revenue by Currency</h3></div>
          <div className="divide-y divide-gray-100">
            {data.revenue_by_currency.map((row) => (
              <div key={row.currency_code} className="px-5 py-2.5 flex items-center justify-between">
                <span className="text-[13px] font-medium text-gray-900">{row.currency_code}</span>
                <span className="text-[13px] text-gray-700 tabular-nums">${fmtMoney(row.revenue)} <span className="text-[11px] text-gray-400">({row.payments} payments)</span></span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Alerts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Upcoming expirations */}
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Expiring Soon</h3><span className="text-[12px] text-gray-400">Next 60 days</span></div>
          {(data.upcoming_expirations || []).length === 0 ? (
            <div className="px-5 py-6 text-center text-[13px] text-gray-400">No upcoming expirations</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {data.upcoming_expirations.map((ue, i) => (
                <div key={i} className="px-5 py-2.5 flex items-center justify-between">
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{ue.client_name}</p>
                    <p className="text-[11px] text-gray-400">{ue.membership_type}</p>
                  </div>
                  <span className={`text-[12px] font-medium tabular-nums ${ue.days_left <= 14 ? "text-red-600" : ue.days_left <= 30 ? "text-amber-600" : "text-gray-500"}`}>{ue.days_left} days left</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Recent failures */}
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Recent Failed Payments</h3></div>
          {(data.recent_failures || []).length === 0 ? (
            <div className="px-5 py-6 text-center text-[13px] text-gray-400">No failed payments</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {data.recent_failures.map((rf, i) => (
                <div key={i} className="px-5 py-2.5 flex items-center justify-between">
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{rf.client_name}</p>
                    <p className="text-[11px] text-red-500">{rf.failure_reason}</p>
                  </div>
                  <span className="text-[13px] font-semibold text-red-600 tabular-nums">${rf.amount.toFixed(2)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/* ─── Revenue Tab ─── */
function RevenueTab({ data }: { data: Report }) {
  data = normalizeReport(data as any);
  const areaChart = {
    tooltip: { trigger: "axis" as const, formatter: (params: any) => {
      const bar = params.find((p: any) => p.seriesType === "bar");
      return `<b>${bar?.name || params[0]?.name}</b><br/>Revenue: <b>$${fmtMoney(bar?.value || params[0]?.value || 0)}</b>`;
    }},
    grid: { left: 55, right: 20, top: 20, bottom: 55 },
    xAxis: { type: "category" as const, data: data.monthly_revenue.map(m => m.month),
      axisLine: { lineStyle: { color: "#f3f4f6" } }, axisTick: { show: false },
      axisLabel: { color: "#9ca3af", fontSize: 11, interval: 0 } },
    yAxis: [
      { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${(v/1000).toFixed(1)}k` },
        splitLine: { lineStyle: { color: "#f9fafb", type: "dashed" as const } }, axisLine: { show: false }, axisTick: { show: false } },
      { type: "value" as const, show: false },
    ],
    series: [
      { name: "Revenue", type: "bar" as const, data: data.monthly_revenue.map(m => m.revenue), barWidth: "60%",
        itemStyle: { borderRadius: [4, 4, 0, 0],
          color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [{ offset: 0, color: "#a78bfa" }, { offset: 0.5, color: "#c4b5fd" }, { offset: 1, color: "#ede9fe" }] } },
        emphasis: { itemStyle: { color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [{ offset: 0, color: "#7c3aed" }, { offset: 0.5, color: "#a78bfa" }, { offset: 1, color: "#ddd6fe" }] } } },
      },
      { name: "Trend", type: "line" as const, yAxisIndex: 1, data: data.monthly_revenue.map(m => m.revenue),
        smooth: true, symbol: "circle", symbolSize: 5,
        lineStyle: { width: 2, color: "#8b5cf6", type: "dashed" as const },
        itemStyle: { color: "#8b5cf6", borderWidth: 2, borderColor: "#fff" }, z: 10 },
    ],
  };

  const barChart = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 55, right: 20, top: 20, bottom: 30 },
    xAxis: { type: "category" as const, data: data.monthly_revenue.map(m => m.month), axisLabel: { color: "#9ca3af", fontSize: 11 } },
    yAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    series: [{ type: "bar" as const, data: data.monthly_revenue.map(m => m.revenue), barWidth: 20,
      itemStyle: { color: { type: "linear" as const, x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: "#818cf8" }, { offset: 1, color: "#6366f1" }] }, borderRadius: [4, 4, 0, 0] } }],
  };

  const typeBar = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 10, right: 40, top: 10, bottom: 10, containLabel: true },
    xAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    yAxis: { type: "category" as const, data: data.membership_breakdown.map(b => b.type_title), axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: "#374151", fontSize: 12 } },
    series: [{ type: "bar" as const, barWidth: 24,
      data: data.membership_breakdown.map((b, i) => ({ value: b.revenue, itemStyle: { color: COLOURS[i % COLOURS.length], borderRadius: [0, 4, 4, 0] } })) }],
  };

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="MRR" value={`$${fmtMoney(data.mrr)}`} />
        <Kpi label="ARR" value={`$${fmtMoney(data.arr)}`} />
        <Kpi label="Avg / Member" value={`$${fmtMoney(data.avg_revenue_per_member)}`} sub="Lifetime" />
        <Kpi
          label="Collected (all time)"
          value={`$${fmtMoney(data.total_collected_amount || data.total_revenue)}`}
          sub="Total from customers"
        />
      </div>
      <div className="card overflow-hidden"><div className="card-header"><h3>Monthly trend (collected)</h3></div><div className="px-5 py-4">{data.monthly_revenue.length > 0 ? <ReactECharts option={areaChart} style={{ height: 320 }} /> : <Empty />}</div></div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <div className="card overflow-hidden"><div className="card-header"><h3>Revenue by Month</h3></div><div className="px-5 py-4">{data.monthly_revenue.length > 0 ? <ReactECharts option={barChart} style={{ height: 280 }} /> : <Empty />}</div></div>
        <div className="card overflow-hidden"><div className="card-header"><h3>Revenue by Membership Type</h3></div><div className="px-5 py-4">{data.membership_breakdown.length > 0 ? <ReactECharts option={typeBar} style={{ height: Math.max(150, data.membership_breakdown.length * 50) }} /> : <Empty />}</div></div>
      </div>
    </div>
  );
}

/* ─── Memberships Tab ─── */
function MembershipsTab({ data }: { data: Report }) {
  data = normalizeReport(data as any);
  const pie = {
    tooltip: { trigger: "item" as const, formatter: "{b}<br/>{c} members ({d}%)" },
    legend: { bottom: 0, textStyle: { color: "#6b7280", fontSize: 11 } },
    series: [{ type: "pie" as const, radius: ["40%", "65%"], center: ["50%", "40%"],
      itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 3 }, label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: "bold" } },
      data: data.membership_breakdown.map((b, i) => ({ value: b.count, name: b.type_title, itemStyle: { color: COLOURS[i % COLOURS.length] } })) }],
  };

  const signupsChart = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 40, right: 20, top: 20, bottom: 30 },
    xAxis: { type: "category" as const, data: (data.monthly_signups || []).map(m => m.month), axisLabel: { color: "#9ca3af", fontSize: 11 } },
    yAxis: { type: "value" as const, minInterval: 1, axisLabel: { color: "#9ca3af", fontSize: 11 }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    series: [
      { name: "Signups", type: "bar" as const, data: (data.monthly_signups || []).map(m => m.count), barWidth: 16, itemStyle: { color: "#10b981", borderRadius: [4, 4, 0, 0] } },
      { name: "Churn", type: "bar" as const, data: (data.monthly_churn || []).map(m => m.count), barWidth: 16, itemStyle: { color: "#ef4444", borderRadius: [4, 4, 0, 0] } },
    ],
    legend: { top: 0, textStyle: { fontSize: 11 } },
  };

  const freqPie = {
    tooltip: { trigger: "item" as const, formatter: "{b}: {c} ({d}%)" },
    series: [{ type: "pie" as const, radius: "65%", center: ["50%", "50%"],
      itemStyle: { borderRadius: 4, borderColor: "#fff", borderWidth: 2 },
      label: { formatter: "{b}\n{d}%", fontSize: 11 },
      data: (data.frequency_breakdown || []).map((f, i) => ({ value: f.count, name: f.frequency.charAt(0).toUpperCase() + f.frequency.slice(1), itemStyle: { color: COLOURS[i % COLOURS.length] } })) }],
  };

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="Active Members" value={data.active_members} />
        <Kpi label="New This Month" value={data.new_signups_month} />
        <Kpi label="Churned This Month" value={data.churned_month} />
        <Kpi label="Avg Duration" value={`${data.avg_membership_days}d`} sub={`~${Math.round(data.avg_membership_days / 30)} months`} />
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <div className="card overflow-hidden"><div className="card-header"><h3>Membership Distribution</h3></div><div className="px-5 py-4">{data.membership_breakdown.length > 0 ? <ReactECharts option={pie} style={{ height: 300 }} /> : <Empty />}</div></div>
        <div className="card overflow-hidden"><div className="card-header"><h3>Signups vs Churn</h3></div><div className="px-5 py-4">{(data.monthly_signups || []).length > 0 ? <ReactECharts option={signupsChart} style={{ height: 300 }} /> : <Empty />}</div></div>
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <div className="card overflow-hidden"><div className="card-header"><h3>Payment Frequency</h3></div><div className="px-5 py-4">{(data.frequency_breakdown || []).length > 0 ? <ReactECharts option={freqPie} style={{ height: 280 }} /> : <Empty />}</div></div>
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Membership Breakdown</h3></div>
          <div className="divide-y divide-gray-100">
            {data.membership_breakdown.map((b, i) => (
              <div key={i} className="px-5 py-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full flex-shrink-0" style={{ background: COLOURS[i % COLOURS.length] }} />
                  <span className="text-[13px] text-gray-900">{b.type_title}{b.currency_code ? ` (${b.currency_code})` : ""}</span>
                </div>
                <div className="flex items-center gap-4">
                  <span className="text-[13px] font-semibold text-gray-900 tabular-nums">{b.count} members</span>
                  <span className="text-[12px] text-gray-400 tabular-nums">${fmtMoney(b.revenue)}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Payments Tab ─── */
function PaymentsTab({ data, totalPayments, successRate }: { data: Report; totalPayments: number; successRate: number }) {
  data = normalizeReport(data as any);
  const donut = {
    tooltip: { trigger: "item" as const, formatter: "{b}: {c} ({d}%)" },
    legend: { bottom: 0, textStyle: { color: "#6b7280", fontSize: 11 } },
    series: [{ type: "pie" as const, radius: ["42%", "68%"], center: ["50%", "40%"],
      itemStyle: { borderRadius: 6, borderColor: "#fff", borderWidth: 3 }, label: { show: false },
      emphasis: { label: { show: true, fontSize: 14, fontWeight: "bold" } },
      data: Object.entries(data.payment_statuses).map(([name, value]) => ({ value, name: name.charAt(0).toUpperCase() + name.slice(1), itemStyle: { color: STATUS_COLOURS[name] || "#9ca3af" } })) }],
  };

  const gauge = {
    series: [{ type: "gauge" as const, startAngle: 200, endAngle: -20, min: 0, max: 100,
      pointer: { show: false },
      progress: { show: true, overlap: false, roundCap: true, clip: false, itemStyle: { color: successRate >= 90 ? "#10b981" : successRate >= 70 ? "#f59e0b" : "#ef4444" } },
      axisLine: { lineStyle: { width: 18, color: [[1, "#f3f4f6"]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false },
      detail: { valueAnimation: true, fontSize: 28, fontWeight: 700, color: "#111827", formatter: "{value}%", offsetCenter: [0, "0%"] },
      title: { fontSize: 12, color: "#9ca3af", offsetCenter: [0, "30%"] },
      data: [{ value: successRate, name: "Success Rate" }] }],
  };

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        <Kpi label="Total Payments" value={totalPayments} />
        <Kpi label="Succeeded" value={data.payment_statuses.succeeded || 0} />
        <Kpi label="Failed" value={data.payment_statuses.failed || 0} alert={(data.payment_statuses.failed || 0) > 0} />
        <Kpi label="Collection Rate" value={`${data.collection_rate.toFixed(1)}%`} />
        <Kpi label="Success Rate" value={`${successRate}%`} />
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <div className="card overflow-hidden"><div className="card-header"><h3>Payment Status</h3></div><div className="px-5 py-4">{totalPayments > 0 ? <ReactECharts option={donut} style={{ height: 320 }} /> : <Empty />}</div></div>
        <div className="card overflow-hidden"><div className="card-header"><h3>Success Rate Gauge</h3></div><div className="px-5 py-4">{totalPayments > 0 ? <ReactECharts option={gauge} style={{ height: 220 }} /> :  <Empty />}
          <div className="space-y-2 mt-4">
            {Object.entries(data.payment_statuses).map(([name, value]) => (
              <div key={name} className="flex items-center gap-2">
                <span className="h-2.5 w-2.5 rounded-full flex-shrink-0" style={{ background: STATUS_COLOURS[name] || "#9ca3af" }} />
                <span className="text-[13px] text-gray-600 flex-1 capitalize">{name}</span>
                <span className="text-[13px] font-semibold text-gray-900 tabular-nums">{value}</span>
              </div>
            ))}
          </div>
        </div></div>
      </div>
      {/* Recent failures */}
      <div className="card overflow-hidden">
        <div className="card-header"><h3>Recent Failed Payments</h3><Link to="/payments" className="text-[12px] text-indigo-600 font-medium">View all →</Link></div>
        {(data.recent_failures || []).length === 0 ? (
          <div className="px-5 py-6 text-center text-[13px] text-gray-400">No failed payments — nice!</div>
        ) : (
          <table className="data-table">
            <thead><tr><th>Client</th><th>Amount</th><th>Reason</th><th>Due Date</th></tr></thead>
            <tbody>
              {data.recent_failures.map((rf, i) => (
                <tr key={i}><td className="font-medium text-gray-900">{rf.client_name}</td><td className="tabular-nums text-red-600">${rf.amount.toFixed(2)}</td><td className="text-[12px] text-gray-500">{rf.failure_reason}</td><td className="text-[12px] text-gray-500">{rf.due_date}</td></tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

/* ─── Clients Tab ─── */
function ClientsTab({ data, fmtDate }: { data: Report; fmtDate: (d: string) => string }) {
  data = normalizeReport(data as any);
  const clientBar = {
    tooltip: { trigger: "axis" as const },
    grid: { left: 10, right: 40, top: 10, bottom: 10, containLabel: true },
    xAxis: { type: "value" as const, axisLabel: { color: "#9ca3af", fontSize: 11, formatter: (v: number) => `$${v.toLocaleString()}` }, splitLine: { lineStyle: { color: "#f3f4f6" } } },
    yAxis: { type: "category" as const, data: (data.top_clients || []).map(c => c.client_name), inverse: true, axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: "#374151", fontSize: 12 } },
    series: [{ type: "bar" as const, barWidth: 20,
      data: (data.top_clients || []).map((c, i) => ({ value: c.total_paid, itemStyle: { color: COLOURS[i % COLOURS.length], borderRadius: [0, 4, 4, 0] } })) }],
  };

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <Kpi label="Total Clients" value={data.total_clients} />
        <Kpi label="Active Members" value={data.active_members} />
        <Kpi label="New This Month" value={data.new_signups_month} />
        <Kpi label="Avg Lifetime Value" value={`$${fmtMoney(data.avg_revenue_per_member)}`} />
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Top Clients by Revenue</h3></div>
          <div className="px-5 py-4">{(data.top_clients || []).length > 0 ? <ReactECharts option={clientBar} style={{ height: Math.max(200, (data.top_clients || []).length * 40) }} /> : <Empty />}</div>
        </div>
        <div className="card overflow-hidden">
          <div className="card-header"><h3>Top 10 Clients</h3></div>
          <div className="divide-y divide-gray-100">
            {(data.top_clients || []).map((c, i) => (
              <Link key={i} to={`/clients/${c.client_id}`} className="px-5 py-2.5 flex items-center justify-between hover:bg-gray-50 transition-colors">
                <div className="flex items-center gap-3">
                  <span className="flex h-7 w-7 items-center justify-center rounded-full bg-gray-100 text-[11px] font-bold text-gray-500">{i + 1}</span>
                  <div>
                    <p className="text-[13px] font-medium text-gray-900">{c.client_name}</p>
                    <p className="text-[11px] text-gray-400">{c.memberships} membership{c.memberships !== 1 ? "s" : ""}</p>
                  </div>
                </div>
                <span className="text-[13px] font-semibold text-gray-900 tabular-nums">${fmtMoney(c.total_paid)}</span>
              </Link>
            ))}
          </div>
        </div>
      </div>
      {/* Upcoming expirations */}
      <div className="card overflow-hidden">
        <div className="card-header"><h3>Upcoming Expirations</h3><span className="text-[12px] text-gray-400">Next 60 days</span></div>
        {(data.upcoming_expirations || []).length === 0 ? (
          <div className="px-5 py-6 text-center text-[13px] text-gray-400">No memberships expiring in the next 60 days</div>
        ) : (
          <table className="data-table">
            <thead><tr><th>Client</th><th>Membership</th><th>Expires</th><th>Days Left</th></tr></thead>
            <tbody>
              {data.upcoming_expirations.map((ue, i) => (
                <tr key={i}>
                  <td className="font-medium text-gray-900">{ue.client_name}</td>
                  <td className="text-gray-600">{ue.membership_type}</td>
                  <td className="text-[12px] text-gray-500">{fmtDate(ue.end_date)}</td>
                  <td><span className={`text-[12px] font-medium ${ue.days_left <= 14 ? "text-red-600" : ue.days_left <= 30 ? "text-amber-600" : "text-gray-500"}`}>{ue.days_left} days</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function Empty() {
  return <div className="flex h-40 items-center justify-center text-[13px] text-gray-400">No data yet</div>;
}
