import { useState, useEffect } from "react";
import {
  AreaChart, BarChart, DonutChart, BarList,
  Card, Title, Text, Flex, Bold, Grid, Metric,
  TabGroup, TabList, Tab, TabPanels, TabPanel,
  List, ListItem, BadgeDelta,
} from "@tremor/react";

interface MonthlyData { month: string; revenue: number; }
interface BreakdownData { type_title: string; revenue: number; count: number; }
interface StatusData { [key: string]: number; }
interface Summary { active_members: number; month_revenue: number; failed_payments: number; new_signups: number; }

export default function ReportsPage() {
  const [monthly, setMonthly] = useState<MonthlyData[]>([]);
  const [breakdown, setBreakdown] = useState<BreakdownData[]>([]);
  const [statuses, setStatuses] = useState<StatusData>({});
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);

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

  if (loading) return <Card><div className="flex h-64 items-center justify-center text-sm text-gray-400">Loading reports...</div></Card>;

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h1 className="text-lg font-semibold text-gray-900">Reports</h1>
          <p className="text-[13px] text-gray-500">Financial analytics and performance insights</p>
        </div>
      </div>

      {/* KPI Summary */}
      <Grid numItemsMd={2} numItemsLg={4} className="gap-4 mb-6">
        <Card>
          <Text>Total Revenue</Text>
          <Metric>${totalRevenue.toLocaleString(undefined, { minimumFractionDigits: 2 })}</Metric>
          <Text className="mt-1">{monthly.length} months tracked</Text>
        </Card>
        <Card>
          <Text>Active Members</Text>
          <Metric>{summary?.active_members || 0}</Metric>
          <Flex className="mt-1">
            <Text>{summary?.new_signups || 0} new this month</Text>
            {(summary?.new_signups || 0) > 0 && <BadgeDelta deltaType="increase" size="xs">{summary?.new_signups}</BadgeDelta>}
          </Flex>
        </Card>
        <Card>
          <Text>Payment Success Rate</Text>
          <Metric>{successRate}%</Metric>
          <Text className="mt-1">{totalPayments} total payments</Text>
        </Card>
        <Card>
          <Text>Avg Revenue / Month</Text>
          <Metric>${monthly.length > 0 ? (totalRevenue / monthly.length).toLocaleString(undefined, { minimumFractionDigits: 2 }) : "0.00"}</Metric>
        </Card>
      </Grid>

      <TabGroup>
        <TabList>
          <Tab>Revenue</Tab>
          <Tab>Memberships</Tab>
          <Tab>Payments</Tab>
        </TabList>
        <TabPanels>
          {/* Revenue Tab */}
          <TabPanel>
            <div className="mt-5 space-y-5">
              <Card>
                <Title>Monthly Revenue</Title>
                <Text>Revenue collected over the past 12 months</Text>
                {monthly.length > 0 ? (
                  <AreaChart
                    className="mt-4 h-80"
                    data={monthly}
                    index="month"
                    categories={["revenue"]}
                    colors={["indigo"]}
                    valueFormatter={(v) => `$${v.toLocaleString()}`}
                    showAnimation
                    showGradient
                    curveType="monotone"
                  />
                ) : (
                  <div className="flex h-80 items-center justify-center text-sm text-gray-400">No revenue data yet</div>
                )}
              </Card>

              <Grid numItemsMd={2} className="gap-5">
                <Card>
                  <Title>Revenue by Month</Title>
                  {monthly.length > 0 ? (
                    <BarChart
                      className="mt-4 h-64"
                      data={monthly}
                      index="month"
                      categories={["revenue"]}
                      colors={["indigo"]}
                      valueFormatter={(v) => `$${v.toLocaleString()}`}
                      showAnimation
                    />
                  ) : (
                    <div className="flex h-64 items-center justify-center text-sm text-gray-400">No data</div>
                  )}
                </Card>
                <Card>
                  <Title>Revenue by Membership Type</Title>
                  {breakdown.length > 0 ? (
                    <BarList
                      data={breakdown.map(b => ({ name: b.type_title, value: b.revenue }))}
                      className="mt-4"
                      valueFormatter={(v) => `$${v.toLocaleString()}`}
                      color="indigo"
                    />
                  ) : (
                    <div className="flex h-64 items-center justify-center text-sm text-gray-400">No data</div>
                  )}
                </Card>
              </Grid>
            </div>
          </TabPanel>

          {/* Memberships Tab */}
          <TabPanel>
            <div className="mt-5 space-y-5">
              <Grid numItemsMd={2} className="gap-5">
                <Card>
                  <Title>Membership Distribution</Title>
                  {breakdown.length > 0 ? (
                    <>
                      <DonutChart
                        className="mt-6"
                        data={breakdown.map(b => ({ name: b.type_title, value: b.count }))}
                        category="value"
                        index="name"
                        colors={["indigo", "cyan", "amber", "emerald", "rose"]}
                        showAnimation
                        variant="pie"
                      />
                      <List className="mt-6">
                        {breakdown.map(b => (
                          <ListItem key={b.type_title}>
                            <span>{b.type_title}</span>
                            <Flex justifyContent="end" className="gap-2">
                              <Bold>{b.count}</Bold>
                              <Text>members</Text>
                            </Flex>
                          </ListItem>
                        ))}
                      </List>
                    </>
                  ) : (
                    <div className="flex h-64 items-center justify-center text-sm text-gray-400">No data</div>
                  )}
                </Card>
                <Card>
                  <Title>Revenue per Membership</Title>
                  {breakdown.length > 0 ? (
                    <>
                      <DonutChart
                        className="mt-6"
                        data={breakdown.map(b => ({ name: b.type_title, value: b.revenue }))}
                        category="value"
                        index="name"
                        colors={["indigo", "cyan", "amber", "emerald", "rose"]}
                        valueFormatter={(v) => `$${v.toLocaleString()}`}
                        showAnimation
                      />
                      <List className="mt-6">
                        {breakdown.map(b => (
                          <ListItem key={b.type_title}>
                            <span>{b.type_title}</span>
                            <Bold>${b.revenue.toLocaleString()}</Bold>
                          </ListItem>
                        ))}
                      </List>
                    </>
                  ) : (
                    <div className="flex h-64 items-center justify-center text-sm text-gray-400">No data</div>
                  )}
                </Card>
              </Grid>
            </div>
          </TabPanel>

          {/* Payments Tab */}
          <TabPanel>
            <div className="mt-5 space-y-5">
              <Grid numItemsMd={2} className="gap-5">
                <Card>
                  <Title>Payment Status Breakdown</Title>
                  {totalPayments > 0 ? (
                    <>
                      <DonutChart
                        className="mt-6"
                        data={Object.entries(statuses).map(([name, value]) => ({ name, value }))}
                        category="value"
                        index="name"
                        colors={["emerald", "red", "yellow", "orange", "gray"]}
                        showAnimation
                      />
                      <List className="mt-6">
                        {Object.entries(statuses).map(([name, value]) => (
                          <ListItem key={name}>
                            <span className="capitalize">{name}</span>
                            <Bold>{value}</Bold>
                          </ListItem>
                        ))}
                      </List>
                    </>
                  ) : (
                    <div className="flex h-64 items-center justify-center text-sm text-gray-400">No payments yet</div>
                  )}
                </Card>
                <Card>
                  <Title>Payment Metrics</Title>
                  <List className="mt-4">
                    <ListItem>
                      <span>Total Payments</span>
                      <Bold>{totalPayments}</Bold>
                    </ListItem>
                    <ListItem>
                      <span>Succeeded</span>
                      <span className="text-emerald-600 font-semibold">{statuses.succeeded || 0}</span>
                    </ListItem>
                    <ListItem>
                      <span>Failed</span>
                      <span className="text-red-600 font-semibold">{statuses.failed || 0}</span>
                    </ListItem>
                    <ListItem>
                      <span>Disputed</span>
                      <span className="text-amber-600 font-semibold">{statuses.disputed || 0}</span>
                    </ListItem>
                    <ListItem>
                      <span>Success Rate</span>
                      <Bold>{successRate}%</Bold>
                    </ListItem>
                  </List>
                </Card>
              </Grid>
            </div>
          </TabPanel>
        </TabPanels>
      </TabGroup>
    </div>
  );
}
