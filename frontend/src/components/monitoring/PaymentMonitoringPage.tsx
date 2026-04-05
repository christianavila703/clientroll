import { useState, useEffect } from "react";
import api from "../../api/client";

interface OverduePayment {
  client_name: string;
  membership_title: string;
  amount: number;
  currency_code: string;
  due_date: string;
  days_overdue: number;
  payment_method: string;
}

interface ExpectedVsReceived {
  total_expected: number;
  total_succeeded: number;
  total_pending: number;
  total_overdue: number;
  overdue_items: OverduePayment[];
}

interface WebhookHealth {
  last_stripe_webhook: string | null;
  last_gocardless_webhook: string | null;
  processed_24h: number;
  failed_24h: number;
  processed_7d: number;
  failed_7d: number;
  stuck: number;
}

interface WebhookAuditMismatch {
  event_id: string;
  provider: string;
  resource_type: string;
  expected_outcome: string;
  actual_status: string;
  payment_id?: string;
  membership_id?: string;
  received_at: string;
  category: string;
}

interface WebhookAudit {
  total_audited: number;
  total_verified: number;
  outcome_mismatches: WebhookAuditMismatch[];
  stuck_events: number;
  unlinked_processed: number;
}

interface AgingBucket {
  label: string;
  count: number;
}

interface PaymentAging {
  buckets: AgingBucket[];
  active_unpaid_30d: number;
}

interface ReconciliationMismatch {
  client_name: string;
  membership_title: string;
  local_status: string;
  provider_status: string;
  severity: string;
  category: string;
  recommended_action: string;
  auto_fixed: boolean;
}

interface ReconciliationRun {
  id: string;
  run_type: string;
  started_at: string;
  completed_at: string | null;
  total_checked: number;
  total_matched: number;
  total_mismatched: number;
  total_auto_fixed: number;
  details: ReconciliationMismatch[];
}

interface AlertConfig {
  alert_email: string;
  immediate_payment_failed: boolean;
  immediate_dispute: boolean;
  immediate_mandate_cancelled: boolean;
  immediate_pipeline_break: boolean;
  daily_digest_enabled: boolean;
  daily_digest_hour: number;
  daily_digest_always: boolean;
  grace_days_card: number;
  grace_days_direct_debit: number;
}

const defaultConfig: AlertConfig = {
  alert_email: "",
  immediate_payment_failed: true,
  immediate_dispute: true,
  immediate_mandate_cancelled: true,
  immediate_pipeline_break: true,
  daily_digest_enabled: true,
  daily_digest_hour: 6,
  daily_digest_always: true,
  grace_days_card: 1,
  grace_days_direct_debit: 3,
};

interface Tenant {
  id: string;
  name: string;
  slug: string;
}

export default function PaymentMonitoringPage() {
  const [tab, setTab] = useState<"overview" | "reconciliation" | "settings">("overview");
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedTenantId, setSelectedTenantId] = useState<string>("");
  const [evr, setEvr] = useState<ExpectedVsReceived | null>(null);
  const [wh, setWh] = useState<WebhookHealth | null>(null);
  const [aging, setAging] = useState<PaymentAging | null>(null);
  const [reconRuns, setReconRuns] = useState<ReconciliationRun[]>([]);
  const [audit, setAudit] = useState<WebhookAudit | null>(null);
  const [config, setConfig] = useState<AlertConfig>(defaultConfig);
  const [configLoaded, setConfigLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState("");
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    api.listTenants().then((data) => {
      const list = (data as any)?.tenants || [];
      setTenants(list);
      if (list.length > 0) {
        setSelectedTenantId(list[0].id);
      }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (selectedTenantId) loadData(selectedTenantId);
  }, [selectedTenantId]);

  async function loadData(tenantId: string) {
    setLoadError("");
    setEvr(null);
    setWh(null);
    setAging(null);
    setReconRuns([]);
    setAudit(null);
    try {
      const [evrRes, whRes, agingRes, reconRes, cfgRes, auditRes] = await Promise.all([
        api.getExpectedVsReceived(tenantId),
        api.getWebhookHealth(tenantId),
        api.getPaymentAging(tenantId),
        api.getReconciliationReport(tenantId),
        api.getAlertConfig(tenantId),
        api.getWebhookAudit(tenantId),
      ]);
      const evrData = evrRes as ExpectedVsReceived;
      if (evrData) evrData.overdue_items = evrData.overdue_items || [];
      setEvr(evrData);
      setWh(whRes as WebhookHealth);
      const agingData = agingRes as PaymentAging;
      if (agingData) agingData.buckets = agingData.buckets || [];
      setAging(agingData);
      setReconRuns((reconRes as any)?.runs || []);
      const auditData = auditRes as WebhookAudit;
      if (auditData) auditData.outcome_mismatches = auditData.outcome_mismatches || [];
      setAudit(auditData);
      if ((cfgRes as any)?.configured !== false) {
        setConfig(cfgRes as AlertConfig);
      } else {
        setConfig(defaultConfig);
      }
      setConfigLoaded(true);
    } catch {
      setLoadError("Failed to load monitoring data. Please try again.");
    }
  }

  async function saveConfig() {
    if (!selectedTenantId) return;
    setSaving(true);
    setSaveMsg("");
    try {
      await api.updateAlertConfig(selectedTenantId, config);
      setSaveMsg("Alert settings saved.");
    } catch {
      setSaveMsg("Failed to save. Please try again.");
    } finally {
      setSaving(false);
    }
  }

  const hasIssues = (evr?.total_overdue ?? 0) > 0 || (wh?.failed_24h ?? 0) > 0 || (wh?.stuck ?? 0) > 0 || (aging?.active_unpaid_30d ?? 0) > 0 || (audit?.stuck_events ?? 0) > 0 || (audit?.outcome_mismatches?.length ?? 0) > 0;

  return (
    <div className="max-w-6xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Payment Monitoring</h1>
          <p className="text-sm text-gray-500 mt-1">
            Triple-checked against your database, payment provider APIs, and webhook event log.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <select
            value={selectedTenantId}
            onChange={(e) => setSelectedTenantId(e.target.value)}
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
          >
            {(tenants || []).map((t) => (
              <option key={t.id} value={t.id}>{t.name || t.slug}</option>
            ))}
          </select>
        {hasIssues && (
          <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-red-100 text-red-800">
            Issues detected
          </span>
        )}
        {!hasIssues && evr && (
          <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-green-100 text-green-800">
            All clear
          </span>
        )}
        </div>
      </div>

      {loadError && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
          <p className="text-sm text-red-800">{loadError}</p>
        </div>
      )}

      <div className="flex gap-1 mb-6 border-b">
        {(["overview", "reconciliation", "settings"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === t
                ? "border-indigo-600 text-indigo-600"
                : "border-transparent text-gray-500 hover:text-gray-700"
            }`}
          >
            {t === "overview" ? "Overview" : t === "reconciliation" ? "Reconciliation" : "Alert Settings"}
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <div className="space-y-6">
          <ExpectedVsReceivedCard data={evr} />
          <WebhookHealthCard data={wh} />
          <WebhookAuditCard data={audit} />
          <PaymentAgingCard data={aging} />
        </div>
      )}

      {tab === "reconciliation" && (
        <ReconciliationTab runs={reconRuns} tenantId={selectedTenantId} onRefresh={() => loadData(selectedTenantId)} />
      )}

      {tab === "settings" && configLoaded && (
        <AlertSettingsTab
          config={config}
          onChange={setConfig}
          onSave={saveConfig}
          saving={saving}
          saveMsg={saveMsg}
        />
      )}
    </div>
  );
}

function ExpectedVsReceivedCard({ data }: { data: ExpectedVsReceived | null }) {
  if (!data) return <CardSkeleton />;
  const statusColor = data.total_overdue > 0 ? "text-red-600" : "text-green-600";
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Expected vs Received</h2>
          <p className="text-xs text-gray-500">Source: payment calendar (our database)</p>
        </div>
      </div>
      <div className="grid grid-cols-4 gap-4 mb-4">
        <Stat label="Expected" value={data.total_expected} />
        <Stat label="Collected" value={data.total_succeeded} color="text-green-600" />
        <Stat label="Pending" value={data.total_pending} color={data.total_pending > 0 ? "text-amber-600" : undefined} />
        <Stat label="Overdue" value={data.total_overdue} color={statusColor} />
      </div>
      {(data.overdue_items?.length ?? 0) > 0 && (
        <div className="mt-4">
          <h3 className="text-sm font-medium text-gray-700 mb-2">Overdue Payments</h3>
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="bg-red-50">
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Client</th>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Membership</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Amount</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Due Date</th>
                  <th className="text-right px-3 py-2 font-medium text-gray-600">Days Overdue</th>
                </tr>
              </thead>
              <tbody>
                {(data.overdue_items || []).map((item, i) => (
                  <tr key={i} className="border-t border-gray-100">
                    <td className="px-3 py-2">{item.client_name}</td>
                    <td className="px-3 py-2">{item.membership_title}</td>
                    <td className="px-3 py-2 text-right">{item.currency_code} ${item.amount.toFixed(2)}</td>
                    <td className="px-3 py-2 text-right">{new Date(item.due_date).toLocaleDateString()}</td>
                    <td className="px-3 py-2 text-right font-semibold text-red-600">{item.days_overdue}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function WebhookHealthCard({ data }: { data: WebhookHealth | null }) {
  if (!data) return <CardSkeleton />;
  const formatTime = (t: string | null) => (t ? new Date(t).toLocaleString() : "Never");
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-gray-900">Pipeline Health</h2>
        <p className="text-xs text-gray-500">Source: webhook event log</p>
      </div>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        <div>
          <p className="text-xs text-gray-500">Last Stripe webhook</p>
          <p className="text-sm font-medium text-gray-900">{formatTime(data.last_stripe_webhook)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Last GoCardless webhook</p>
          <p className="text-sm font-medium text-gray-900">{formatTime(data.last_gocardless_webhook)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Processed (24h)</p>
          <p className="text-sm font-medium text-gray-900">{data.processed_24h}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Failed (24h)</p>
          <p className={`text-sm font-medium ${data.failed_24h > 0 ? "text-red-600" : "text-gray-900"}`}>{data.failed_24h}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Processed (7d)</p>
          <p className="text-sm font-medium text-gray-900">{data.processed_7d}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Failed (7d)</p>
          <p className={`text-sm font-medium ${data.failed_7d > 0 ? "text-red-600" : "text-gray-900"}`}>{data.failed_7d}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500">Stuck events</p>
          <p className={`text-sm font-medium ${(data.stuck ?? 0) > 0 ? "text-red-600 font-bold" : "text-gray-900"}`}>{data.stuck ?? 0}</p>
        </div>
      </div>
    </div>
  );
}

function WebhookAuditCard({ data }: { data: WebhookAudit | null }) {
  if (!data) return <CardSkeleton />;
  const mismatches = data.outcome_mismatches || [];
  const hasProblems = mismatches.length > 0 || data.stuck_events > 0 || data.unlinked_processed > 0;
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Webhook Audit Trail</h2>
          <p className="text-xs text-gray-500">Leg 3: Verifies that each webhook produced the expected DB change</p>
        </div>
        {!hasProblems && data.total_audited > 0 && (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
            All verified
          </span>
        )}
      </div>
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-4">
        <Stat label="Audited (7d)" value={data.total_audited} />
        <Stat label="Verified" value={data.total_verified} color="text-green-600" />
        <Stat label="Mismatches" value={mismatches.length} color={mismatches.length > 0 ? "text-red-600" : undefined} />
        <Stat label="Stuck" value={data.stuck_events} color={data.stuck_events > 0 ? "text-red-600" : undefined} />
        <Stat label="Unlinked" value={data.unlinked_processed} color={data.unlinked_processed > 0 ? "text-amber-600" : undefined} />
      </div>
      {mismatches.length > 0 && (
        <div className="mt-4">
          <h3 className="text-sm font-medium text-gray-700 mb-2">Outcome Mismatches</h3>
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="bg-red-50">
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Event</th>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Provider</th>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Expected</th>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Actual</th>
                  <th className="text-left px-3 py-2 font-medium text-gray-600">Received</th>
                </tr>
              </thead>
              <tbody>
                {mismatches.map((m, i) => (
                  <tr key={i} className="border-t border-gray-100">
                    <td className="px-3 py-2 font-mono text-xs">{m.event_id.substring(0, 20)}...</td>
                    <td className="px-3 py-2">{m.provider}</td>
                    <td className="px-3 py-2">
                      <span className="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">{m.expected_outcome}</span>
                    </td>
                    <td className="px-3 py-2">
                      <span className="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">{m.actual_status || "unknown"}</span>
                    </td>
                    <td className="px-3 py-2 text-xs text-gray-500">{new Date(m.received_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function PaymentAgingCard({ data }: { data: PaymentAging | null }) {
  if (!data) return <CardSkeleton />;
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-gray-900">Payment Aging</h2>
        <p className="text-xs text-gray-500">Source: payments table</p>
      </div>
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {(data.buckets || []).map((b) => (
          <Stat key={b.label} label={b.label} value={b.count} color={b.count > 0 ? "text-amber-600" : undefined} />
        ))}
        <Stat label="Active, unpaid 30d+" value={data.active_unpaid_30d} color={data.active_unpaid_30d > 0 ? "text-red-600" : undefined} />
      </div>
    </div>
  );
}

function ReconciliationTab({ runs, tenantId, onRefresh }: { runs: ReconciliationRun[]; tenantId: string; onRefresh: () => void }) {
  const [expanded, setExpanded] = useState<string | null>(null);
  const [runningStripe, setRunningStripe] = useState(false);
  const [runningGC, setRunningGC] = useState(false);
  const [runError, setRunError] = useState("");
  const [filter, setFilter] = useState<"all" | "daily" | "gocardless">("all");

  async function runStripe() {
    if (!tenantId) return;
    setRunningStripe(true);
    setRunError("");
    try {
      await api.runReconciliationNow(tenantId);
      onRefresh();
    } catch (err: any) {
      setRunError(err?.message || "Stripe reconciliation failed");
    } finally {
      setRunningStripe(false);
    }
  }

  async function runGC() {
    if (!tenantId) return;
    setRunningGC(true);
    setRunError("");
    try {
      await api.runGCReconciliationNow(tenantId);
      onRefresh();
    } catch (err: any) {
      setRunError(err?.message || "GoCardless reconciliation failed");
    } finally {
      setRunningGC(false);
    }
  }

  async function runAll() {
    if (!tenantId) return;
    setRunningStripe(true);
    setRunningGC(true);
    setRunError("");
    try {
      await Promise.all([
        api.runReconciliationNow(tenantId).catch(() => {}),
        api.runGCReconciliationNow(tenantId).catch(() => {}),
      ]);
      onRefresh();
    } catch (err: any) {
      setRunError(err?.message || "Reconciliation failed");
    } finally {
      setRunningStripe(false);
      setRunningGC(false);
    }
  }

  const filteredRuns = filter === "all" ? runs : runs.filter((r) => r.run_type === filter);
  const running = runningStripe || runningGC;

  return (
    <div className="space-y-4">
      <div className="bg-white rounded-xl border border-gray-200 p-5">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">Provider API Verification</h2>
            <p className="text-sm text-gray-500 mt-1">
              Calls provider APIs directly to compare subscription status, payment amounts, and statuses against our records.
              {runs.length === 0 && " No runs yet — click a button to verify."}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3 flex-wrap">
          <button
            onClick={runAll}
            disabled={running || !tenantId}
            className="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors whitespace-nowrap"
          >
            {running ? "Running..." : "Run All"}
          </button>
          <button
            onClick={runStripe}
            disabled={runningStripe || !tenantId}
            className="px-4 py-2 bg-white border border-gray-300 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors whitespace-nowrap"
          >
            {runningStripe ? "Running..." : "Stripe Only"}
          </button>
          <button
            onClick={runGC}
            disabled={runningGC || !tenantId}
            className="px-4 py-2 bg-white border border-gray-300 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors whitespace-nowrap"
          >
            {runningGC ? "Running..." : "GoCardless Only"}
          </button>
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-gray-500">Filter:</span>
            {(["all", "daily", "gocardless"] as const).map((f) => (
              <button
                key={f}
                onClick={() => setFilter(f)}
                className={`px-2.5 py-1 text-xs rounded-full font-medium transition-colors ${
                  filter === f ? "bg-indigo-100 text-indigo-700" : "text-gray-500 hover:bg-gray-100"
                }`}
              >
                {f === "all" ? "All" : f === "daily" ? "Stripe" : "GoCardless"}
              </button>
            ))}
          </div>
          {runError && <p className="text-sm text-red-600 w-full mt-2">{runError}</p>}
        </div>
      </div>
      {filteredRuns.map((run) => (
        <div key={run.id} className="bg-white rounded-xl border border-gray-200 p-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-900">
                {run.completed_at ? new Date(run.completed_at).toLocaleString() : "In progress..."}
              </p>
              <p className="text-xs text-gray-500">
                <span className={`inline-flex px-1.5 py-0.5 rounded text-xs font-medium mr-1 ${
                  run.run_type === "gocardless" ? "bg-teal-100 text-teal-700" : "bg-violet-100 text-violet-700"
                }`}>
                  {run.run_type === "gocardless" ? "GoCardless" : "Stripe"}
                </span>
                run
              </p>
            </div>
            <div className="flex items-center gap-4 text-sm">
              <span className="text-gray-600">{run.total_checked} checked</span>
              <span className="text-green-600">{run.total_matched} matched</span>
              <span className={run.total_mismatched > 0 ? "text-red-600 font-semibold" : "text-gray-600"}>
                {run.total_mismatched} mismatched
              </span>
              <span className="text-blue-600">{run.total_auto_fixed} auto-fixed</span>
              {(run.details?.length ?? 0) > 0 && (
                <button
                  onClick={() => setExpanded(expanded === run.id ? null : run.id)}
                  className="text-indigo-600 hover:text-indigo-800 text-xs font-medium"
                >
                  {expanded === run.id ? "Hide" : "Details"}
                </button>
              )}
            </div>
          </div>
          {expanded === run.id && (run.details?.length ?? 0) > 0 && (
            <div className="mt-4 overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className="bg-amber-50">
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Client</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Our Status</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Provider Status</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Category</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {(run.details || []).map((d, i) => (
                    <tr key={i} className="border-t border-gray-100">
                      <td className="px-3 py-2">{d.client_name}</td>
                      <td className="px-3 py-2">{d.local_status}</td>
                      <td className="px-3 py-2">{d.provider_status}</td>
                      <td className="px-3 py-2">
                        <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          d.severity === "critical" ? "bg-red-100 text-red-800" :
                          d.category === "amount_mismatch" ? "bg-orange-100 text-orange-800" :
                          "bg-amber-100 text-amber-800"
                        }`}>
                          {d.category}
                        </span>
                      </td>
                      <td className="px-3 py-2 text-sm text-gray-600">
                        {d.recommended_action}
                        {d.auto_fixed && <span className="ml-1 text-green-600 text-xs">(auto-fixed)</span>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function AlertSettingsTab({
  config,
  onChange,
  onSave,
  saving,
  saveMsg,
}: {
  config: AlertConfig;
  onChange: (c: AlertConfig) => void;
  onSave: () => void;
  saving: boolean;
  saveMsg: string;
}) {
  const update = (field: keyof AlertConfig, value: unknown) =>
    onChange({ ...config, [field]: value });

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6 max-w-2xl">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Alert Configuration</h2>

      <div className="space-y-5">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Alert email</label>
          <input
            type="email"
            value={config.alert_email}
            onChange={(e) => update("alert_email", e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
            placeholder="ben@businessnet.com.au"
          />
          <p className="text-xs text-gray-500 mt-1">Immediate alerts and daily digests will be sent to this address.</p>
        </div>

        <fieldset>
          <legend className="text-sm font-medium text-gray-700 mb-2">Immediate Alerts</legend>
          <div className="space-y-2">
            <Toggle label="Payment failed" checked={config.immediate_payment_failed} onChange={(v) => update("immediate_payment_failed", v)} />
            <Toggle label="Payment disputed" checked={config.immediate_dispute} onChange={(v) => update("immediate_dispute", v)} />
            <Toggle label="Mandate/subscription cancelled" checked={config.immediate_mandate_cancelled} onChange={(v) => update("immediate_mandate_cancelled", v)} />
            <Toggle label="Pipeline break detected" checked={config.immediate_pipeline_break} onChange={(v) => update("immediate_pipeline_break", v)} />
          </div>
        </fieldset>

        <fieldset>
          <legend className="text-sm font-medium text-gray-700 mb-2">Daily Digest</legend>
          <div className="space-y-2">
            <Toggle label="Enable daily digest" checked={config.daily_digest_enabled} onChange={(v) => update("daily_digest_enabled", v)} />
            <Toggle label="Send even when all clear" checked={config.daily_digest_always} onChange={(v) => update("daily_digest_always", v)} />
            <div className="flex items-center gap-2">
              <label className="text-sm text-gray-600">Send at</label>
              <input
                type="number"
                min={0}
                max={23}
                value={config.daily_digest_hour}
                onChange={(e) => update("daily_digest_hour", parseInt(e.target.value) || 6)}
                className="w-16 px-2 py-1 border border-gray-300 rounded text-sm"
              />
              <span className="text-sm text-gray-500">:00 (tenant timezone)</span>
            </div>
          </div>
        </fieldset>

        <fieldset>
          <legend className="text-sm font-medium text-gray-700 mb-2">Grace Periods</legend>
          <p className="text-xs text-gray-500 mb-3">How many days after the expected payment date before flagging as overdue.</p>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-gray-600 mb-1">Card payments (days)</label>
              <input
                type="number"
                min={0}
                max={14}
                value={config.grace_days_card}
                onChange={(e) => update("grace_days_card", parseInt(e.target.value) || 1)}
                className="w-full px-2 py-1 border border-gray-300 rounded text-sm"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-600 mb-1">Direct debit (days)</label>
              <input
                type="number"
                min={0}
                max={14}
                value={config.grace_days_direct_debit}
                onChange={(e) => update("grace_days_direct_debit", parseInt(e.target.value) || 3)}
                className="w-full px-2 py-1 border border-gray-300 rounded text-sm"
              />
            </div>
          </div>
        </fieldset>

        <div className="flex items-center gap-3 pt-2">
          <button
            onClick={onSave}
            disabled={saving || !config.alert_email}
            className="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
          >
            {saving ? "Saving..." : "Save Settings"}
          </button>
          {saveMsg && <p className="text-sm text-gray-600">{saveMsg}</p>}
        </div>
      </div>
    </div>
  );
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 cursor-pointer">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="w-4 h-4 text-indigo-600 border-gray-300 rounded focus:ring-indigo-500"
      />
      <span className="text-sm text-gray-700">{label}</span>
    </label>
  );
}

function Stat({ label, value, color }: { label: string; value: number; color?: string }) {
  return (
    <div>
      <p className="text-xs text-gray-500">{label}</p>
      <p className={`text-xl font-bold ${color || "text-gray-900"}`}>{value}</p>
    </div>
  );
}

function CardSkeleton() {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6 animate-pulse">
      <div className="h-4 bg-gray-200 rounded w-1/3 mb-4" />
      <div className="grid grid-cols-3 gap-4">
        <div className="h-8 bg-gray-200 rounded" />
        <div className="h-8 bg-gray-200 rounded" />
        <div className="h-8 bg-gray-200 rounded" />
      </div>
    </div>
  );
}
