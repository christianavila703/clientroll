import { useMemo, useState } from "react";

import api from "../../api/client";
import {
  effectiveTotalPrice,
  emptySimplePricingForm,
  payloadFromSimple,
  type SimplePricingForm,
} from "../../utils/termOptionAdapter";
import SimplePricingFields from "./SimplePricingFields";

interface Props {
  mtId: string;
  currencyCode: string;
  onCancel: () => void;
  onCreated: () => void;
}

export default function TermOptionAddForm({ mtId, currencyCode, onCancel, onCreated }: Props) {
  const [form, setForm] = useState<SimplePricingForm>(() => emptySimplePricingForm());
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const schedules = useMemo(() => payloadFromSimple(form), [form]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    const durationValue = Math.round(Number(form.duration_value));
    if (!Number.isFinite(durationValue) || durationValue <= 0) {
      setError(`Term length must be a whole number of ${form.duration_unit === "week" ? "weeks" : "months"}.`);
      return;
    }
    const total = effectiveTotalPrice(form);
    if (!Number.isFinite(total) || total <= 0) {
      setError(form.payment_method === "split"
        ? "Enter an amount for every payment in the split plan."
        : "Enter the total price for this membership.");
      return;
    }
    if (schedules.length === 0) {
      setError(form.payment_method === "split"
        ? "Split plans need at least two payments with increasing dates."
        : "Choose how this term option is paid.");
      return;
    }

    setSaving(true);
    try {
      const legacyDurationMonths = form.duration_unit === "week"
        ? Math.max(1, Math.ceil(durationValue / 4.345))
        : durationValue;
      const created = (await api.createTermOption(mtId, {
        duration_value: durationValue,
        duration_unit: form.duration_unit,
        duration_months: legacyDurationMonths,
        label: form.label.trim() || null,
        fee_total: total,
        allow_upfront: form.payment_method === "upfront",
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
      {error && (
        <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 text-[13px] text-red-600">
          {error}
        </div>
      )}

      <h3 className="text-sm font-semibold text-gray-900">New Term Option</h3>

      <SimplePricingFields form={form} setForm={setForm} currencyCode={currencyCode} />

      <div className="flex gap-2 justify-end pt-1">
        <button type="button" onClick={onCancel} className="btn-secondary text-[12px]">
          Cancel
        </button>
        <button type="submit" disabled={saving} className="btn-primary">
          {saving ? "Adding..." : "Add term option"}
        </button>
      </div>
    </form>
  );
}
