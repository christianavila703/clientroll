import { useMemo, useState } from "react";

import api from "../../api/client";
import {
  emptyTermOptionForm,
  payloadFromForm,
  STANDARD_KEYS,
  type CustomPlanForm,
  type StandardKey,
  type TermOptionForm,
} from "../../utils/termOptionAdapter";

interface Props {
  mtId: string;
  currencyCode: string;
  onCancel: () => void;
  onCreated: () => void;
}

const standardLabel: Record<StandardKey, string> = {
  upfront: "Upfront",
  weekly: "Weekly",
  fortnightly: "Fortnightly",
  monthly: "Monthly",
  quarterly: "Quarterly",
};

const newCustomPlan = (): CustomPlanForm => ({
  uid: `new-${Math.random().toString(36).slice(2, 10)}`,
  label: "",
  amount: "",
  interval_unit: "month",
  interval_count: "1",
  num_payments: "",
  is_default: false,
});

const standardAmountKey = (key: StandardKey) =>
  `${key}_amount` as
    | "upfront_amount"
    | "weekly_amount"
    | "fortnightly_amount"
    | "monthly_amount"
    | "quarterly_amount";

const standardEnabledKey = (key: StandardKey) =>
  `${key}_enabled` as
    | "upfront_enabled"
    | "weekly_enabled"
    | "fortnightly_enabled"
    | "monthly_enabled"
    | "quarterly_enabled";

// Derive a "total contract value" for the legacy fee_total column so reports,
// PDFs, and dashboards keep working during the soak. Preference order:
//   upfront amount > (recurring amount × number of payments fit in term) >
//   custom plan total > 0
function deriveLegacyTotal(form: TermOptionForm): number {
  const upfront = Number(form.upfront_amount);
  if (form.upfront_enabled && Number.isFinite(upfront) && upfront > 0) return upfront;
  const durationValue = Number(form.duration_value) || 0;
  const months = form.duration_unit === "week" ? Math.ceil(durationValue / 4.345) : durationValue;
  const weeks = form.duration_unit === "week" ? durationValue : Math.round(durationValue * 52 / 12);
  const recurringTotals: number[] = [];
  if (form.weekly_enabled && Number(form.weekly_amount) > 0) {
    recurringTotals.push(Number(form.weekly_amount) * weeks);
  }
  if (form.fortnightly_enabled && Number(form.fortnightly_amount) > 0) {
    recurringTotals.push(Number(form.fortnightly_amount) * Math.max(1, Math.round(weeks / 2)));
  }
  if (form.monthly_enabled && Number(form.monthly_amount) > 0) {
    recurringTotals.push(Number(form.monthly_amount) * months);
  }
  if (form.quarterly_enabled && Number(form.quarterly_amount) > 0) {
    recurringTotals.push(Number(form.quarterly_amount) * Math.max(1, Math.round(months / 3)));
  }
  for (const cp of form.custom_plans) {
    const amount = Number(cp.amount);
    const n = Number(cp.num_payments);
    if (Number.isFinite(amount) && amount > 0 && Number.isFinite(n) && n > 0) {
      recurringTotals.push(amount * n);
    }
  }
  if (recurringTotals.length > 0) {
    return Math.round(recurringTotals[0] * 100) / 100;
  }
  return 0;
}

export default function TermOptionAddForm({ mtId, currencyCode, onCancel, onCreated }: Props) {
  const [form, setForm] = useState<TermOptionForm>(() => {
    const f = emptyTermOptionForm();
    f.duration_value = "12";
    f.monthly_enabled = true;
    f.default_schedule_key = "monthly";
    return f;
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const previewPayload = useMemo(() => payloadFromForm(form), [form]);
  const totalEstimate = useMemo(() => deriveLegacyTotal(form), [form]);

  const mutate = (next: TermOptionForm) => {
    setError("");
    setForm(next);
  };

  const toggleStandard = (key: StandardKey, enabled: boolean) =>
    mutate({ ...form, [standardEnabledKey(key)]: enabled });

  const setStandardAmount = (key: StandardKey, value: string) =>
    mutate({ ...form, [standardAmountKey(key)]: value });

  const setDefault = (key: string) => mutate({ ...form, default_schedule_key: key });

  const addCustom = () => mutate({ ...form, custom_plans: [...form.custom_plans, newCustomPlan()] });

  const updateCustom = (uid: string, patch: Partial<CustomPlanForm>) =>
    mutate({
      ...form,
      custom_plans: form.custom_plans.map(cp => (cp.uid === uid ? { ...cp, ...patch } : cp)),
    });

  const removeCustom = (uid: string) =>
    mutate({
      ...form,
      custom_plans: form.custom_plans.filter(cp => cp.uid !== uid),
      default_schedule_key:
        form.default_schedule_key === `custom:${uid}` ? "" : form.default_schedule_key,
    });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    const durationValue = Math.round(Number(form.duration_value));
    if (!Number.isFinite(durationValue) || durationValue <= 0) {
      setError(`Term length must be a whole number of ${form.duration_unit === "week" ? "weeks" : "months"}.`);
      return;
    }
    const schedules = payloadFromForm(form);
    if (schedules.length === 0) {
      setError("Tick at least one payment frequency (or add a custom plan).");
      return;
    }
    setSaving(true);
    try {
      // Step 1: create the term option row using the legacy endpoint. The
      // server's syncSchedulesFromLegacy will derive a placeholder schedule
      // from these fee_* fields; step 2 replaces them with the v2 payload.
      const legacyDurationMonths = form.duration_unit === "week"
        ? Math.max(1, Math.ceil(durationValue / 4.345))
        : durationValue;
      const created = (await api.createTermOption(mtId, {
        duration_value: durationValue,
        duration_unit: form.duration_unit,
        duration_months: legacyDurationMonths,
        label: form.label.trim() || null,
        fee_total: totalEstimate,
        allow_upfront: form.upfront_enabled,
        fee_weekly: null,
        fee_fortnightly: null,
        fee_monthly: null,
        fee_quarterly: null,
        continue_after_term: false,
        custom_schedule_enabled: false,
        custom_schedule_label: null,
        custom_schedule_amount: null,
        custom_schedule_interval_unit: null,
        custom_schedule_interval_count: null,
        custom_schedule_num_payments: null,
      })) as { id: string } | undefined;
      const newId = created?.id;
      if (!newId) {
        throw new Error("Server did not return new term option id");
      }
      // Step 2: replace schedules with the real v2 payload so admin-defined
      // amounts and custom plans land exactly as entered.
      await api.replacePaymentSchedules(newId, schedules);
      onCreated();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to create term option");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="card px-5 py-4 mb-4 space-y-4">
      {error && <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">{error}</div>}

      <div>
        <h3 className="text-sm font-semibold text-gray-900 mb-3">New Term Option</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Term length *</label>
            <input
              type="number"
              value={form.duration_value}
              onChange={e => mutate({ ...form, duration_value: e.target.value })}
              min="1"
              step="1"
              required
              className="input-field py-1 px-2 text-[13px] w-full"
            />
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">Unit</label>
            <select
              value={form.duration_unit}
              onChange={e => mutate({ ...form, duration_unit: e.target.value as "week" | "month" })}
              className="input-field py-1 px-2 text-[13px] w-full"
            >
              <option value="month">Month(s)</option>
              <option value="week">Week(s)</option>
            </select>
          </div>
          <div>
            <label className="block text-[12px] font-medium text-gray-700 mb-1">
              Display label <span className="text-gray-400">(optional)</span>
            </label>
            <input
              type="text"
              value={form.label}
              onChange={e => mutate({ ...form, label: e.target.value })}
              placeholder='e.g. "Foundation Plan"'
              className="input-field py-1 px-2 text-[13px] w-full"
            />
          </div>
        </div>
      </div>

      <div>
        <h4 className="text-[13px] font-semibold text-gray-900 mb-2">Standard payment options</h4>
        <p className="text-[12px] text-gray-500 mb-2">
          Tick each frequency this term offers, then enter the amount the member will pay per period.
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {STANDARD_KEYS.map(key => {
            const enabled = form[standardEnabledKey(key)];
            const amount = form[standardAmountKey(key)];
            const isDefault = form.default_schedule_key === key;
            return (
              <label
                key={key}
                className={`flex items-center gap-2 p-2 rounded-md border ${
                  enabled ? "border-indigo-300 bg-white" : "border-gray-200 bg-white/60"
                }`}
              >
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={e => toggleStandard(key, e.target.checked)}
                />
                <span className="w-24 text-gray-700 text-[13px]">{standardLabel[key]}</span>
                <span className="text-gray-400 text-[12px]">{currencyCode}</span>
                <input
                  type="number"
                  step="0.01"
                  value={amount}
                  disabled={!enabled}
                  onChange={e => setStandardAmount(key, e.target.value)}
                  className="input-field py-1 px-2 text-[12px] flex-1 disabled:bg-gray-100"
                  placeholder="0.00"
                />
                <label className="flex items-center gap-1 text-[11px] text-gray-500">
                  <input
                    type="radio"
                    name="new-default"
                    checked={isDefault}
                    disabled={!enabled}
                    onChange={() => setDefault(key)}
                  />
                  Default
                </label>
              </label>
            );
          })}
        </div>
      </div>

      <details className="bg-gray-50 border border-gray-200 rounded-md">
        <summary className="cursor-pointer px-3 py-2 text-[12px] font-medium text-gray-700">
          Custom plans ({form.custom_plans.length})
        </summary>
        <div className="p-3 space-y-3">
          {form.custom_plans.length === 0 && (
            <p className="text-[12px] text-gray-500">
              Use a custom plan for "10 weekly payments", "Pay every 6 months", or any cadence not in the standard list.
            </p>
          )}
          {form.custom_plans.map(cp => {
            const isDefault = form.default_schedule_key === `custom:${cp.uid}`;
            const kindBadge = cp.num_payments && Number(cp.num_payments) > 0 ? "Instalments" : "Recurring";
            return (
              <div key={cp.uid} className="border border-gray-200 rounded-md p-2 space-y-2 bg-white">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-medium text-gray-500 uppercase tracking-wide">
                    {kindBadge}
                  </span>
                  <button
                    type="button"
                    onClick={() => removeCustom(cp.uid)}
                    className="text-[11px] text-red-600 hover:underline"
                  >
                    Remove
                  </button>
                </div>
                <input
                  type="text"
                  placeholder="Plan label (shown to member)"
                  value={cp.label}
                  onChange={e => updateCustom(cp.uid, { label: e.target.value })}
                  className="input-field py-1 px-2 text-[12px] w-full"
                />
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                  <div>
                    <label className="block text-[11px] text-gray-500 mb-0.5">Amount</label>
                    <input
                      type="number"
                      step="0.01"
                      value={cp.amount}
                      onChange={e => updateCustom(cp.uid, { amount: e.target.value })}
                      className="input-field py-1 px-2 text-[12px] w-full"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-gray-500 mb-0.5">Every</label>
                    <input
                      type="number"
                      min="1"
                      value={cp.interval_count}
                      onChange={e => updateCustom(cp.uid, { interval_count: e.target.value })}
                      className="input-field py-1 px-2 text-[12px] w-full"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-gray-500 mb-0.5">Unit</label>
                    <select
                      value={cp.interval_unit}
                      onChange={e =>
                        updateCustom(cp.uid, {
                          interval_unit: e.target.value as "week" | "month",
                        })
                      }
                      className="input-field py-1 px-2 text-[12px] w-full"
                    >
                      <option value="week">Week(s)</option>
                      <option value="month">Month(s)</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-[11px] text-gray-500 mb-0.5">Total payments</label>
                    <input
                      type="number"
                      min="0"
                      value={cp.num_payments}
                      onChange={e => updateCustom(cp.uid, { num_payments: e.target.value })}
                      className="input-field py-1 px-2 text-[12px] w-full"
                      placeholder="(blank = forever)"
                    />
                  </div>
                </div>
                <label className="flex items-center gap-2 text-[12px] text-gray-600">
                  <input
                    type="radio"
                    name="new-default"
                    checked={isDefault}
                    onChange={() => setDefault(`custom:${cp.uid}`)}
                  />
                  Default for this term option
                </label>
              </div>
            );
          })}
          <button
            type="button"
            onClick={addCustom}
            className="text-[12px] px-2 py-1 rounded-md border border-indigo-200 text-indigo-700 hover:bg-indigo-50"
          >
            Add custom plan
          </button>
        </div>
      </details>

      <div className="flex items-center justify-between text-[11px] text-gray-500">
        <span>Will create {previewPayload.length} payment schedule{previewPayload.length === 1 ? "" : "s"}.</span>
        <span>Estimated contract value: {currencyCode} ${totalEstimate.toFixed(2)}</span>
      </div>

      <div className="flex gap-2 justify-end pt-1">
        <button type="button" onClick={onCancel} className="btn-secondary text-[12px]">Cancel</button>
        <button type="submit" disabled={saving} className="btn-primary">
          {saving ? "Adding..." : "Add term option"}
        </button>
      </div>
    </form>
  );
}
