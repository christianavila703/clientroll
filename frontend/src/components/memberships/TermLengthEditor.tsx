import { useEffect, useState } from "react";

import api from "../../api/client";
import type { TermOption } from "../../types/memberships";

interface Props {
  mtId: string;
  termOption: TermOption;
  onSaved?: () => void;
}

const initial = (to: TermOption) => ({
  duration_value: String(to.duration_value ?? to.duration_months ?? 0),
  duration_unit: (to.duration_unit ?? "month") as "week" | "month",
  label: to.label ?? "",
});

export default function TermLengthEditor({ mtId, termOption, onSaved }: Props) {
  const [form, setForm] = useState(() => initial(termOption));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setForm(initial(termOption));
    setDirty(false);
    setError("");
    setMessage("");
  }, [termOption.id, termOption.duration_value, termOption.duration_unit, termOption.label]);

  const mutate = <K extends keyof ReturnType<typeof initial>>(key: K, value: ReturnType<typeof initial>[K]) => {
    setForm(prev => ({ ...prev, [key]: value }));
    setDirty(true);
    setError("");
    setMessage("");
  };

  const handleSave = async () => {
    const n = Number(form.duration_value);
    if (!Number.isFinite(n) || n <= 0 || Math.round(n) !== n) {
      setError("Duration must be a whole positive number.");
      return;
    }
    setSaving(true);
    try {
      await api.updateTermOptionMeta(mtId, termOption.id, {
        duration_value: Math.round(n),
        duration_unit: form.duration_unit,
        label: form.label.trim() ? form.label.trim() : null,
      });
      setDirty(false);
      setMessage("Saved.");
      onSaved?.();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to save term length");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-3 border border-gray-200 rounded-lg p-4 bg-white space-y-3 text-[13px]">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="font-semibold text-gray-900">Membership Term</h4>
          <p className="text-[12px] text-gray-500">
            How long this contract runs. Payment schedules below describe how the member pays for it.
          </p>
        </div>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving || !dirty}
          className="px-3 py-1.5 rounded-md bg-gray-800 text-white text-[12px] disabled:bg-gray-300"
        >
          {saving ? "Saving..." : "Save term"}
        </button>
      </div>

      {error && <div className="bg-red-50 text-red-700 text-[12px] px-2 py-1 rounded">{error}</div>}
      {message && <div className="bg-green-50 text-green-700 text-[12px] px-2 py-1 rounded">{message}</div>}

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Length</label>
          <input
            type="number"
            min="1"
            step="1"
            value={form.duration_value}
            onChange={e => mutate("duration_value", e.target.value)}
            className="input-field py-1 px-2 text-[13px] w-full"
          />
        </div>
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Unit</label>
          <select
            value={form.duration_unit}
            onChange={e => mutate("duration_unit", e.target.value as "week" | "month")}
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
            onChange={e => mutate("label", e.target.value)}
            placeholder='e.g. "Foundation Plan"'
            className="input-field py-1 px-2 text-[13px] w-full"
          />
        </div>
      </div>
    </div>
  );
}
