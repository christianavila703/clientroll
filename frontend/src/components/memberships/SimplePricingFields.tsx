// SimplePricingFields
//
// The shared "price + how is it paid" body used by both the Add Term Option
// form and the inline edit panel on each existing term option card.
//
// One term option = one payment frequency. The admin sets a total price and
// picks ONE method. If the gym wants to offer multiple payment cadences,
// they create one term option per cadence.

import {
  derivedSimpleAmounts,
  splitLinesTotal,
  type SimplePaymentMethod,
  type SimplePricingForm,
  type SplitLineForm,
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

  const selectMethod = (method: SimplePaymentMethod) => update("payment_method", method);

  type Option = {
    method: Exclude<SimplePaymentMethod, "custom">;
    label: string;
    suffix: string;
    amount: number;
  };
  const options: Option[] = [
    { method: "upfront", label: "Pay upfront", suffix: "in one payment", amount: amounts.upfront },
    { method: "weekly", label: "Pay weekly", suffix: "/ week", amount: amounts.weekly },
    { method: "fortnightly", label: "Pay fortnightly", suffix: "/ fortnight", amount: amounts.fortnightly },
    { method: "monthly", label: "Pay monthly", suffix: "/ month", amount: amounts.monthly },
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

      {/* Total price (derived from the lines for split plans) */}
      {form.payment_method === "split" ? (
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Total price</label>
          <div className="text-[14px] font-medium text-gray-900 tabular-nums">
            {fmtMoney(splitLinesTotal(form.split_lines), currencyCode)}
            <span className="ml-2 text-[12px] font-normal text-gray-500">(sum of the payments below)</span>
          </div>
        </div>
      ) : (
        <div>
          <label className="block text-[12px] font-medium text-gray-700 mb-1">Total price *</label>
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
            <span className="text-[12px] text-gray-500">
              total for the entire {form.duration_value || "?"} {form.duration_unit}{Number(form.duration_value) === 1 ? "" : "s"}.
            </span>
          </div>
        </div>
      )}

      {/* Payment method radio */}
      <div>
        <h4 className="text-[13px] font-semibold text-gray-900 mb-1">How is it paid?</h4>
        <p className="text-[12px] text-gray-500 mb-2">
          Pick one. To offer a different payment cadence, add a separate term option for it.
        </p>
        <div className="space-y-1.5">
          {options.map(opt => {
            const checked = form.payment_method === opt.method;
            return (
              <label
                key={opt.method}
                className={`flex items-center justify-between gap-3 px-3 py-2 rounded-md border cursor-pointer ${
                  checked ? "border-indigo-300 bg-indigo-50/50" : "border-gray-200 bg-white"
                }`}
              >
                <div className="flex items-center gap-3">
                  <input
                    type="radio"
                    name="payment_method"
                    checked={checked}
                    onChange={() => selectMethod(opt.method)}
                  />
                  <span className="text-[13px] text-gray-800">{opt.label}</span>
                </div>
                <div className="text-[13px] tabular-nums text-gray-700">
                  {fmtMoney(opt.amount, currencyCode)}
                  <span className="ml-1 text-[11px] text-gray-500">{opt.suffix}</span>
                </div>
              </label>
            );
          })}
          <label
            className={`flex items-center justify-between gap-3 px-3 py-2 rounded-md border cursor-pointer ${
              form.payment_method === "custom" ? "border-indigo-300 bg-indigo-50/50" : "border-gray-200 bg-white"
            }`}
          >
            <div className="flex items-center gap-3">
              <input
                type="radio"
                name="payment_method"
                checked={form.payment_method === "custom"}
                onChange={() => selectMethod("custom")}
              />
              <span className="text-[13px] text-gray-800">Custom plan</span>
            </div>
            <span className="text-[11px] text-gray-500">e.g. "10 weekly payments", "every 6 months"</span>
          </label>
          <label
            className={`flex items-center justify-between gap-3 px-3 py-2 rounded-md border cursor-pointer ${
              form.payment_method === "split" ? "border-indigo-300 bg-indigo-50/50" : "border-gray-200 bg-white"
            }`}
          >
            <div className="flex items-center gap-3">
              <input
                type="radio"
                name="payment_method"
                checked={form.payment_method === "split"}
                onChange={() => selectMethod("split")}
              />
              <span className="text-[13px] text-gray-800">Split payments</span>
            </div>
            <span className="text-[11px] text-gray-500">uneven amounts on set dates, e.g. "$5,000 now + $8,500 in 14 days"</span>
          </label>
        </div>
      </div>

      {/* Custom plan details — only when "custom" is selected */}
      {form.payment_method === "custom" && (
        <div className="rounded-md border border-gray-200 bg-gray-50/40 p-3 space-y-2">
          <input
            type="text"
            value={form.custom_label}
            onChange={e => update("custom_label", e.target.value)}
            placeholder='Plan label (shown to the member, e.g. "10 weekly payments")'
            className="input-field py-1.5 px-2 text-[13px] w-full"
          />
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
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
              <label className="block text-[11px] text-gray-500 mb-0.5">Total payments</label>
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
          <div className="text-[12px] text-gray-600 pt-1">
            Each payment: <span className="font-semibold text-gray-900 tabular-nums">{fmtMoney(amounts.custom, currencyCode)}</span>
            <span className="text-gray-400"> (auto-calculated from total &divide; payments)</span>
          </div>
        </div>
      )}

      {/* Split payment lines — only when "split" is selected */}
      {form.payment_method === "split" && (
        <div className="rounded-md border border-gray-200 bg-gray-50/40 p-3 space-y-2">
          <input
            type="text"
            value={form.split_label}
            onChange={e => update("split_label", e.target.value)}
            placeholder='Plan label (shown to the member, e.g. "2 payments")'
            className="input-field py-1.5 px-2 text-[13px] w-full"
          />
          {form.split_lines.map((line, i) => {
            const setLine = (patch: Partial<SplitLineForm>) =>
              update("split_lines", form.split_lines.map((l, j) => (j === i ? { ...l, ...patch } : l)));
            return (
              <div key={line.uid} className="flex items-center gap-2">
                <span className="text-[12px] text-gray-500 w-20 shrink-0">Payment {i + 1}</span>
                <span className="text-gray-500 text-[13px]">{currencyCode} $</span>
                <input
                  type="number"
                  min="0.01"
                  step="0.01"
                  value={line.amount}
                  onChange={e => setLine({ amount: e.target.value })}
                  className="input-field py-1 px-2 text-[13px] w-32"
                  placeholder="0.00"
                />
                {i === 0 ? (
                  <span className="text-[12px] text-gray-600">charged at signup</span>
                ) : (
                  <>
                    <input
                      type="number"
                      min="1"
                      step="1"
                      value={line.offset_days}
                      onChange={e => setLine({ offset_days: e.target.value })}
                      className="input-field py-1 px-2 text-[13px] w-20"
                    />
                    <span className="text-[12px] text-gray-600">days after start</span>
                  </>
                )}
                {form.split_lines.length > 2 && i > 0 && (
                  <button
                    type="button"
                    onClick={() => update("split_lines", form.split_lines.filter((_, j) => j !== i))}
                    className="ml-auto text-[12px] text-red-500 hover:text-red-700"
                  >
                    Remove
                  </button>
                )}
              </div>
            );
          })}
          <button
            type="button"
            onClick={() => {
              const last = form.split_lines[form.split_lines.length - 1];
              const nextOffset = Math.max(1, Math.round(Number(last?.offset_days) || 0) + 14);
              update("split_lines", [
                ...form.split_lines,
                { uid: `split-${Date.now()}`, amount: "", offset_days: String(nextOffset) },
              ]);
            }}
            className="text-[12px] text-indigo-600 hover:text-indigo-800"
          >
            + Add payment
          </button>
          <div className="text-[12px] text-gray-600 pt-1 border-t border-gray-200">
            The member pays the first amount when they sign up; the rest are charged automatically on their dates.
            Payment dates must be in increasing order.
          </div>
        </div>
      )}
    </div>
  );
}
