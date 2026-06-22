// SimplePricingFields
//
// The shared "price + how do they pay" body used by both the Add Term Option
// form and the inline edit panel on each existing term option card.
//
// Shape:
//   [Term length] [Unit]          [Display label (optional)]
//   [Total price]                  AUD total
//
//   How can members pay?
//     ☐ Pay upfront                 $X
//     ☐ Pay weekly                  $X / week
//     ☐ Pay fortnightly             $X / fortnight
//     ☐ Pay monthly                 $X / month
//
//   Default option (only shown when 2+ ticked):
//     ( ) Upfront  ( ) Weekly  ( ) Fortnightly  ( ) Monthly  [( ) Custom]
//
//   ▸ Advanced: custom payment plan
//     (single instalment-style row for exotic cadences)
//
// Amounts are auto-derived from total_price ÷ duration so the admin never has
// to type the same number five times.

import {
  derivedSimpleAmounts,
  type SimpleDefaultMethod,
  type SimplePricingForm,
} from "../../utils/termOptionAdapter";

interface Props {
  form: SimplePricingForm;
  setForm: (updater: (prev: SimplePricingForm) => SimplePricingForm) => void;
  currencyCode: string;
}

const fmtMoney = (n: number, currency: string) => {
  if (!Number.isFinite(n) || n <= 0) return `${currency} —`;
  return `${currency} $${n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
};

export default function SimplePricingFields({ form, setForm, currencyCode }: Props) {
  const amounts = derivedSimpleAmounts(form);
  const update = <K extends keyof SimplePricingForm>(key: K, value: SimplePricingForm[K]) =>
    setForm(prev => ({ ...prev, [key]: value }));

  // Helper that ensures `default_method` always points at something that's
  // actually ticked. Called whenever a checkbox toggles.
  const toggleCadence = (
    key: "pay_upfront" | "pay_weekly" | "pay_fortnightly" | "pay_monthly",
    enabled: boolean,
    methodKey: SimpleDefaultMethod,
  ) => {
    setForm(prev => {
      const next: SimplePricingForm = { ...prev, [key]: enabled };
      const stillValid = (m: SimpleDefaultMethod): boolean => {
        switch (m) {
          case "upfront": return next.pay_upfront;
          case "weekly": return next.pay_weekly;
          case "fortnightly": return next.pay_fortnightly;
          case "monthly": return next.pay_monthly;
          case "custom": return next.use_custom;
        }
      };
      if (enabled && !stillValid(prev.default_method)) {
        next.default_method = methodKey;
      } else if (!enabled && prev.default_method === methodKey) {
        // Promote the first remaining ticked option to default.
        if (next.pay_upfront) next.default_method = "upfront";
        else if (next.pay_weekly) next.default_method = "weekly";
        else if (next.pay_fortnightly) next.default_method = "fortnightly";
        else if (next.pay_monthly) next.default_method = "monthly";
        else if (next.use_custom) next.default_method = "custom";
      }
      return next;
    });
  };

  const enabledCount =
    (form.pay_upfront ? 1 : 0) +
    (form.pay_weekly ? 1 : 0) +
    (form.pay_fortnightly ? 1 : 0) +
    (form.pay_monthly ? 1 : 0) +
    (form.use_custom ? 1 : 0);

  type Cadence = {
    method: SimpleDefaultMethod;
    flag: "pay_upfront" | "pay_weekly" | "pay_fortnightly" | "pay_monthly";
    label: string;
    suffix: string;
    amount: number;
  };
  const cadences: Cadence[] = [
    { method: "upfront", flag: "pay_upfront", label: "Pay upfront", suffix: "in one payment", amount: amounts.upfront },
    { method: "weekly", flag: "pay_weekly", label: "Pay weekly", suffix: "/ week", amount: amounts.weekly },
    { method: "fortnightly", flag: "pay_fortnightly", label: "Pay fortnightly", suffix: "/ fortnight", amount: amounts.fortnightly },
    { method: "monthly", flag: "pay_monthly", label: "Pay monthly", suffix: "/ month", amount: amounts.monthly },
  ];

  return (
    <div className="space-y-5">
      {/* Term length + label */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Term length *</label>
          <div className="flex gap-2">
            <input
              type="number"
              min="1"
              step="1"
              value={form.duration_value}
              onChange={e => update("duration_value", e.target.value)}
              required
              className="input-field py-1.5 px-2 text-[13px] w-20"
            />
            <select
              value={form.duration_unit}
              onChange={e => update("duration_unit", e.target.value as "week" | "month")}
              className="input-field py-1.5 px-2 text-[13px] flex-1"
            >
              <option value="month">Month(s)</option>
              <option value="week">Week(s)</option>
            </select>
          </div>
        </div>
        <div className="sm:col-span-2">
          <label className="block text-[12px] font-medium text-gray-700 mb-1">
            Display label <span className="text-gray-400 font-normal">(optional — shown to members)</span>
          </label>
          <input
            type="text"
            value={form.label}
            onChange={e => update("label", e.target.value)}
            placeholder='e.g. "Foundation Plan"'
            className="input-field py-1.5 px-2 text-[13px] w-full"
          />
        </div>
      </div>

      {/* Total price */}
      <div>
        <label className="block text-[12px] font-medium text-gray-700 mb-1">
          Total price *
        </label>
        <div className="flex items-center gap-2">
          <span className="text-gray-500 text-[13px]">{currencyCode} $</span>
          <input
            type="number"
            min="0"
            step="0.01"
            value={form.total_price}
            onChange={e => update("total_price", e.target.value)}
            required
            className="input-field py-1.5 px-2 text-[14px] w-40 font-medium"
            placeholder="0.00"
          />
          <span className="text-[12px] text-gray-500">total for the entire {form.duration_value || "?"} {form.duration_unit}{Number(form.duration_value) === 1 ? "" : "s"}.</span>
        </div>
      </div>

      {/* Payment cadences */}
      <div>
        <h4 className="text-[13px] font-semibold text-gray-900 mb-1">How can members pay?</h4>
        <p className="text-[12px] text-gray-500 mb-2">Tick every payment option you want to offer. Amounts are calculated from the total above.</p>
        <div className="space-y-1.5">
          {cadences.map(c => {
            const enabled = form[c.flag];
            return (
              <label
                key={c.method}
                className={`flex items-center justify-between gap-3 px-3 py-2 rounded-md border ${
                  enabled ? "border-indigo-300 bg-indigo-50/50" : "border-gray-200 bg-white"
                }`}
              >
                <div className="flex items-center gap-3">
                  <input
                    type="checkbox"
                    checked={enabled}
                    onChange={e => toggleCadence(c.flag, e.target.checked, c.method)}
                  />
                  <span className="text-[13px] text-gray-800">{c.label}</span>
                </div>
                <div className="text-[13px] tabular-nums text-gray-700">
                  {fmtMoney(c.amount, currencyCode)}
                  <span className="ml-1 text-[11px] text-gray-500">{c.suffix}</span>
                </div>
              </label>
            );
          })}
        </div>
      </div>

      {/* Default picker (only when there's a choice to make) */}
      {enabledCount > 1 && (
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">
            Default option <span className="text-gray-400 font-normal">(pre-selected at signup)</span>
          </label>
          <div className="flex flex-wrap gap-3 text-[13px]">
            {form.pay_upfront && (
              <label className="inline-flex items-center gap-1.5">
                <input type="radio" name="default_method" checked={form.default_method === "upfront"} onChange={() => update("default_method", "upfront")} />
                Upfront
              </label>
            )}
            {form.pay_weekly && (
              <label className="inline-flex items-center gap-1.5">
                <input type="radio" name="default_method" checked={form.default_method === "weekly"} onChange={() => update("default_method", "weekly")} />
                Weekly
              </label>
            )}
            {form.pay_fortnightly && (
              <label className="inline-flex items-center gap-1.5">
                <input type="radio" name="default_method" checked={form.default_method === "fortnightly"} onChange={() => update("default_method", "fortnightly")} />
                Fortnightly
              </label>
            )}
            {form.pay_monthly && (
              <label className="inline-flex items-center gap-1.5">
                <input type="radio" name="default_method" checked={form.default_method === "monthly"} onChange={() => update("default_method", "monthly")} />
                Monthly
              </label>
            )}
            {form.use_custom && (
              <label className="inline-flex items-center gap-1.5">
                <input type="radio" name="default_method" checked={form.default_method === "custom"} onChange={() => update("default_method", "custom")} />
                Custom plan
              </label>
            )}
          </div>
        </div>
      )}

      {/* Advanced: custom plan */}
      <details
        className="rounded-md border border-gray-200 bg-gray-50/40"
        open={form.use_custom}
      >
        <summary className="cursor-pointer select-none px-3 py-2 text-[12px] font-medium text-gray-700">
          Advanced — non-standard payment plan {form.use_custom ? "(active)" : ""}
        </summary>
        <div className="px-3 pb-3 pt-1 space-y-2">
          <label className="inline-flex items-center gap-2 text-[13px] text-gray-800">
            <input
              type="checkbox"
              checked={form.use_custom}
              onChange={e => {
                const enabled = e.target.checked;
                setForm(prev => {
                  const next: SimplePricingForm = { ...prev, use_custom: enabled };
                  if (!enabled && prev.default_method === "custom") {
                    if (next.pay_upfront) next.default_method = "upfront";
                    else if (next.pay_weekly) next.default_method = "weekly";
                    else if (next.pay_fortnightly) next.default_method = "fortnightly";
                    else if (next.pay_monthly) next.default_method = "monthly";
                    else next.default_method = "monthly";
                  }
                  return next;
                });
              }}
            />
            Offer a custom payment plan (e.g. "10 weekly payments", "Pay every 6 months")
          </label>
          {form.use_custom && (
            <div className="space-y-2 mt-2">
              <input
                type="text"
                value={form.custom_label}
                onChange={e => update("custom_label", e.target.value)}
                placeholder="Plan label (shown to the member, e.g. '10 weekly payments')"
                className="input-field py-1.5 px-2 text-[13px] w-full"
              />
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                <div>
                  <label className="block text-[11px] text-gray-500 mb-0.5">Amount each time</label>
                  <input
                    type="number"
                    step="0.01"
                    value={form.custom_amount}
                    onChange={e => update("custom_amount", e.target.value)}
                    className="input-field py-1 px-2 text-[13px] w-full"
                    placeholder="0.00"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-gray-500 mb-0.5">Every</label>
                  <input
                    type="number"
                    min="1"
                    value={form.custom_interval_count}
                    onChange={e => update("custom_interval_count", e.target.value)}
                    className="input-field py-1 px-2 text-[13px] w-full"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-gray-500 mb-0.5">Unit</label>
                  <select
                    value={form.custom_interval_unit}
                    onChange={e => update("custom_interval_unit", e.target.value as "week" | "month")}
                    className="input-field py-1 px-2 text-[13px] w-full"
                  >
                    <option value="week">Week(s)</option>
                    <option value="month">Month(s)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-[11px] text-gray-500 mb-0.5">
                    Total payments
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={form.custom_num_payments}
                    onChange={e => update("custom_num_payments", e.target.value)}
                    className="input-field py-1 px-2 text-[13px] w-full"
                    placeholder="leave blank for ongoing"
                  />
                </div>
              </div>
            </div>
          )}
        </div>
      </details>
    </div>
  );
}
