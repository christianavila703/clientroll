import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import {
  AreaChart, BarList, DonutChart,
  Card, Metric, Text, Flex, BadgeDelta, Grid, Title, Bold, List, ListItem,
  Tracker, Color,
} from "@tremor/react";

interface Summary { active_members: number; month_revenue: number; failed_payments: number; new_signups: number; open_disputes: number; }
interface MonthlyData { month: string; revenue: number; }
interface BreakdownData { type_title: string; revenue: number; count: number; }
interface StatusData { [key: string]: number; }
interface RecentClient { id: string; name: string; email?: string; created_at: string; }

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
  const deltaType = (summary?.month_revenue || 0) >= prevMonthRevenue ? "increase" : "decrease";

  // Payment tracker data
  const totalPayments = Object.values(statuses).reduce((a, b) => a + b, 0);
  const trackerData: { color: Color; tooltip: string }[] = [];
  for (let i = 0; i < (statuses.succeeded || 0); i++) trackerData.push({ color: "emerald", tooltip: "Succeeded" });
  for (let i = 0; i < (statuses.failed || 0); i++) trackerData.push({ color: "red", tooltip: "Failed" });
  for (let i = 0; i < (statuses.pending || 0); i++) trackerData.push({ color: "yellow", tooltip: "Pending" });
  for (let i = 0; i < (statuses.disputed || 0); i++) trackerData.push({ color: "orange", tooltip: "Disputed" });

  // BarList data for membership breakdown
  const barListData = breakdown.map(b => ({ name: b.type_title, value: b.revenue }));

  return (
    <div>
      <h1 className="text-lg font-semibold text-gray-900 mb-5">Overview</h1>

      {/* Alert */}
      {summary && summary.failed_payments > 0 && (
        <Link to="/payments" className="mb-5 flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[13px] text-amber-800 hover:bg-amber-100 transition-colors">
          <svg className="h-4 w-4 text-amber-500 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
          {summary.failed_payments} failed payment{summary.failed_payments !== 1 ? "s" : ""} require attention
        </Link>
      )}

      {/* KPI Cards */}
      <Grid numItemsMd={2} numItemsLg={4} className="gap-4 mb-6">
        <Card decoration="top" decorationColor="indigo">
          <Flex justifyContent="between" alignItems="center">
            <div>
              <Text>Active Members</Text>
              <Metric>{summary?.active_members ?? "–"}</Metric>
            </div>
          </Flex>
        </Card>
        <Card decoration="top" decorationColor="emerald">
          <Flex justifyContent="between" alignItems="center">
            <div>
              <Text>Revenue This Month</Text>
              <Metric>${(summary?.month_revenue || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}</Metric>
            </div>
            {prevMonthRevenue > 0 && (
              <BadgeDelta deltaType={deltaType} size="xs">
                {Math.abs(Math.round(((summary?.month_revenue || 0) - prevMonthRevenue) / prevMonthRevenue * 100))}%
              </BadgeDelta>
            )}
          </Flex>
        </Card>
        <Card decoration="top" decorationColor="red">
          <Flex justifyContent="between" alignItems="center">
            <div>
              <Text>Failed Payments</Text>
              <Metric className={summary && summary.failed_payments > 0 ? "text-red-600" : ""}>{summary?.failed_payments ?? "–"}</Metric>
            </div>
          </Flex>
        </Card>
        <Card decoration="top" decorationColor="blue">
          <Flex justifyContent="between" alignItems="center">
            <div>
              <Text>New Signups</Text>
              <Metric>{summary?.new_signups ?? "–"}</Metric>
            </div>
          </Flex>
        </Card>
      </Grid>

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* Main */}
        <div className="lg:col-span-2 space-y-5">
          {/* Revenue Chart */}
          <Card>
            <Title>Revenue</Title>
            <Text>Monthly revenue over the past year</Text>
            {monthly.length > 0 ? (
              <AreaChart
                className="mt-4 h-64"
                data={monthly}
                index="month"
                categories={["revenue"]}
                colors={["indigo"]}
                valueFormatter={(v) => `$${v.toLocaleString()}`}
                showAnimation
                curveType="monotone"
              />
            ) : (
              <div className="flex h-64 items-center justify-center text-sm text-gray-400">No revenue data yet</div>
            )}
            <Flex className="mt-4 border-t border-gray-200 pt-4">
              <div>
                <Text>Total Revenue</Text>
                <Bold>${totalRevenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}</Bold>
              </div>
              <div className="text-right">
                <Text>Avg Monthly</Text>
                <Bold>${monthly.length > 0 ? (totalRevenue / monthly.length).toLocaleString(undefined, { minimumFractionDigits: 2 }) : "0.00"}</Bold>
              </div>
            </Flex>
          </Card>

          {/* Payment Health */}
          <Card>
            <Title>Payment Health</Title>
            <Text>{totalPayments} total payments</Text>
            {trackerData.length > 0 ? (
              <Tracker data={trackerData} className="mt-4" />
            ) : (
              <div className="flex h-10 items-center justify-center text-sm text-gray-400 mt-4">No payments yet</div>
            )}
            <Flex className="mt-4">
              <div className="flex items-center gap-4 text-sm">
                <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-emerald-500" />{statuses.succeeded || 0} succeeded</span>
                <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-red-500" />{statuses.failed || 0} failed</span>
                <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-full bg-yellow-500" />{statuses.pending || 0} pending</span>
              </div>
            </Flex>
          </Card>

          {/* Revenue by Membership */}
          <Card>
            <Title>Revenue by Membership Type</Title>
            <Text>Breakdown of revenue per offering</Text>
            {barListData.length > 0 ? (
              <BarList
                data={barListData}
                className="mt-4"
                valueFormatter={(v) => `$${v.toLocaleString()}`}
                color="indigo"
              />
            ) : (
              <div className="flex h-24 items-center justify-center text-sm text-gray-400 mt-4">No data yet</div>
            )}
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-5">
          {/* Membership Breakdown Donut */}
          <Card>
            <Title>Membership Mix</Title>
            {breakdown.length > 0 ? (
              <>
                <DonutChart
                  className="mt-4"
                  data={breakdown.map(b => ({ name: b.type_title, value: b.count }))}
                  category="value"
                  index="name"
                  colors={["indigo", "cyan", "amber", "emerald", "rose"]}
                  showAnimation
                />
                <List className="mt-4">
                  {breakdown.map(b => (
                    <ListItem key={b.type_title}>
                      <span>{b.type_title}</span>
                      <span><Bold>{b.count}</Bold> members</span>
                    </ListItem>
                  ))}
                </List>
              </>
            ) : (
              <div className="flex h-40 items-center justify-center text-sm text-gray-400 mt-4">No data yet</div>
            )}
          </Card>

          {/* Action Required */}
          <Card>
            <Title>Action Required</Title>
            <List className="mt-2">
              <ListItem>
                <Link to="/payments" className="text-sm hover:text-indigo-600">Failed payments</Link>
                <span className={`text-sm font-semibold ${summary && summary.failed_payments > 0 ? "text-red-600" : ""}`}>{summary?.failed_payments || 0}</span>
              </ListItem>
              <ListItem>
                <Link to="/payments" className="text-sm hover:text-indigo-600">Open disputes</Link>
                <span className={`text-sm font-semibold ${summary && summary.open_disputes > 0 ? "text-amber-600" : ""}`}>{summary?.open_disputes || 0}</span>
              </ListItem>
            </List>
          </Card>

          {/* Recent Clients */}
          <Card>
            <Title>Recent Clients</Title>
            <List className="mt-2">
              {recentClients.map(c => (
                <ListItem key={c.id}>
                  <Link to={`/clients/${c.id}`} className="text-sm font-medium hover:text-indigo-600">{c.name}</Link>
                  <span className="text-xs text-gray-400">{new Date(c.created_at).toLocaleDateString("en-AU", { day: "numeric", month: "short" })}</span>
                </ListItem>
              ))}
            </List>
          </Card>
        </div>
      </div>
    </div>
  );
}
