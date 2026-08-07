import { useEffect, useMemo, useState } from "react";

import api from "../../api/client";
import type { TermOption } from "../../types/memberships";
import {
  effectiveTotalPrice,
  payloadFromSimple,
  simpleFromPayload,
  type SimplePricingForm,
} from "../../utils/termOptionAdapter";
import SimplePricingFields from "./SimplePricingFields";

interface Props {
  termOption: TermOption;
  currencyCode: string;
  mtId: string;
  onSaved?: () => void;
}

const initialFormFromTermOption = (to: TermOption) => {
  return simpleFromPayload(
    to.payment_schedules ?? [],
    to.duration_value ?? to.duration_months ?? 0,
    to.duration_unit ?? "month",
    to.label,
    to.fee_total,
  );
};

export default function PaymentSchedulesEditor({
  termOption,
  currencyCode,
  mtId,
  onSaved,
}: Props) {
  const [form, setForm] = useState<SimplePricingForm>(() => initialFormFromTermOption(termOption).form);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [dirty, setDirty] = useState(false);

  // Reset when a different term option is opened or its server-side fields change.
  useEffect(() => {
    setForm(initialFormFromTermOption(termOption).form);
    setError("");
    setMessage("");
    setDirty(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [termOption.id, termOption.duration_value, termOption.duration_unit, termOption.label]);

  const setFormWrapped = (updater: (prev: SimplePricingForm) => SimplePricingForm) => {
    setForm(prev => {
      const next = updater(prev);
      setDirty(true);
      setError("");
      setMessage("");
      return next;
    });
  };

  const schedules = useMemo(() => payloadFromSimple(form), [form]);

  const handleSave = async () => {
    setError("");
    setMessage("");

    const durationValue = Math.round(Number(form.duration_value));
    if (!Number.isFinite(durationValue) || durationValue <= 0) {
      setError("Term length must be a whole positive number.");
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
      // Save metadata (duration/label/total) and schedules in one logical
      // save. The server stores them separately, so we issue two requests
      // in sequence.
      await api.updateTermOptionMeta(mtId, termOption.id, {
        duration_value: durationValue,
        duration_unit: form.duration_unit,
        label: form.label.trim() ? form.label.trim() : null,
        fee_total: total,
      });
      await api.replacePaymentSchedules(termOption.id, schedules);
      setDirty(false);
      setMessage("Saved.");
      onSaved?.();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to save term option");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-3 border border-indigo-100 rounded-lg p-4 bg-indigo-50/30 space-y-4 text-[13px]">
      <div className="flex items-center justify-between">
        <h4 className="font-semibold text-gray-900 text-[14px]">Term &amp; Pricing</h4>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty}
          className="px-3 py-1.5 rounded-md bg-indigo-600 text-white text-[12px] disabled:bg-gray-300"
        >
          {saving ? "Saving..." : "Save changes"}
        </button>
      </div>

      {error && <div className="bg-red-50 text-red-700 text-[12px] px-2 py-1 rounded">{error}</div>}
      {message && <div className="bg-green-50 text-green-700 text-[12px] px-2 py-1 rounded">{message}</div>}

      <SimplePricingFields form={form} setForm={setFormWrapped} currencyCode={currencyCode} />
    </div>
  );
}
