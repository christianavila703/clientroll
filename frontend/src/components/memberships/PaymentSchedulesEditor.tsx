import { useEffect, useMemo, useState } from "react";

import api from "../../api/client";
import type { PaymentSchedule, TermOption } from "../../types/memberships";
import {
  emptyTermOptionForm,
  formFromPayload,
  payloadFromForm,
  STANDARD_KEYS,
  type CustomPlanForm,
  type StandardKey,
  type TermOptionForm,
} from "../../utils/termOptionAdapter";

interface Props {
  termOption: TermOption;
  currencyCode: string;
  onSaved?: () => void;
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

const initialFormFromTermOption = (to: TermOption): TermOptionForm => {
  const schedules: PaymentSchedule[] = to.payment_schedules ?? [];
  const durationValue = to.duration_value ?? to.duration_months ?? 0;
  const durationUnit = to.duration_unit ?? "month";
  return formFromPayload(
    schedules,
    durationValue,
    durationUnit,
    to.label,
    to.sort_order ?? 0,
  );
};

export default function PaymentSchedulesEditor({
  termOption,
  currencyCode,
  onSaved,
}: Props) {
  const [form, setForm] = useState<TermOptionForm>(() => initialFormFromTermOption(termOption));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [dirty, setDirty] = useState(false);

  // Reset when a different term option is opened.
  useEffect(() => {
    setForm(initialFormFromTermOption(termOption));
    setError("");
    setMessage("");
    setDirty(false);
  }, [termOption.id]);

  const previewPayload = useMemo(() => payloadFromForm(form), [form]);

  const mutate = (next: TermOptionForm) => {
    setDirty(true);
    setError("");
    setMessage("");
    setForm(next);
  };

  const toggleStandard = (key: StandardKey, enabled: boolean) => {
    const patch: Partial<TermOptionForm> = {};
    switch (key) {
      case "upfront":
        patch.upfront_enabled = enabled;
        break;
      case "weekly":
        patch.weekly_enabled = enabled;
        break;
      case "fortnightly":
        patch.fortnightly_enabled = enabled;
        break;
      case "monthly":
        patch.monthly_enabled = enabled;
        break;
      case "quarterly":
        patch.quarterly_enabled = enabled;
        break;
    }
    mutate({ ...form, ...patch });
  };

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

  const setStandardAmount = (key: StandardKey, value: string) =>
    mutate({ ...form, [standardAmountKey(key)]: value });

  const setDefault = (key: string) => mutate({ ...form, default_schedule_key: key });

  const addCustom = () =>
    mutate({ ...form, custom_plans: [...form.custom_plans, newCustomPlan()] });

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

  const handleSave = async () => {
    setError("");
    setMessage("");
    const payload = payloadFromForm(form);
    if (payload.length === 0) {
      setError("Add at least one payment schedule before saving.");
      return;
    }
    setSaving(true);
    try {
      await api.replacePaymentSchedules(termOption.id, payload);
      setDirty(false);
      setMessage("Saved.");
      onSaved?.();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Failed to save payment schedules";
      setError(msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-3 border border-indigo-100 rounded-lg p-4 bg-indigo-50/30 space-y-4 text-[13px]">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="font-semibold text-gray-900">Payment Schedules</h4>
          <p className="text-[12px] text-gray-500">
            Pick which standard frequencies this term option offers, plus any custom instalment plans.
          </p>
        </div>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty}
          className="px-3 py-1.5 rounded-md bg-indigo-600 text-white text-[12px] disabled:bg-gray-300"
        >
          {saving ? "Saving..." : "Save schedules"}
        </button>
      </div>

      {error && <div className="bg-red-50 text-red-700 text-[12px] px-2 py-1 rounded">{error}</div>}
      {message && <div className="bg-green-50 text-green-700 text-[12px] px-2 py-1 rounded">{message}</div>}

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
              <span className="w-24 text-gray-700">{standardLabel[key]}</span>
              <span className="text-gray-400">{currencyCode}</span>
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
                  name={`default-${termOption.id}`}
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

      <details className="bg-white border border-gray-200 rounded-md">
        <summary className="cursor-pointer px-3 py-2 text-[12px] font-medium text-gray-700">
          Custom plans ({form.custom_plans.length})
        </summary>
        <div className="p-3 space-y-3">
          {form.custom_plans.length === 0 && (
            <p className="text-[12px] text-gray-500">No custom plans yet.</p>
          )}
          {form.custom_plans.map(cp => {
            const isDefault = form.default_schedule_key === `custom:${cp.uid}`;
            const kindBadge = cp.num_payments && Number(cp.num_payments) > 0 ? "Instalments" : "Recurring";
            return (
              <div key={cp.uid} className="border border-gray-200 rounded-md p-2 space-y-2 bg-gray-50/40">
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
                    name={`default-${termOption.id}`}
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

      <details className="bg-white border border-gray-200 rounded-md">
        <summary className="cursor-pointer px-3 py-2 text-[12px] font-medium text-gray-700">
          Preview payload ({previewPayload.length} schedule{previewPayload.length === 1 ? "" : "s"})
        </summary>
        <pre className="text-[10px] text-gray-600 p-3 overflow-auto">
          {JSON.stringify(previewPayload, null, 2)}
        </pre>
      </details>
    </div>
  );
}
