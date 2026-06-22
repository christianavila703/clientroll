// termOptionAdapter
//
// Pure, side-effect-free conversion between:
//   * The flat admin form (what the React state actually holds while the user
//     edits a term option), and
//   * The PaymentSchedule[] payload the server stores in
//     `term_option_payment_schedules`.
//
// The form mirrors the familiar shape used pre-v2 — one checkbox + amount per
// standard frequency (upfront / weekly / fortnightly / monthly / quarterly)
// plus a list of zero or more "custom plans". This file is the only place that
// knows how those flat fields map to the canonical schedule rows.
//
// Round-trip fidelity (`formFromPayload(payloadFromForm(f)) ~= f`) is covered
// by `termOptionAdapter.spec.ts`.

import type { PaymentSchedule } from "../types/memberships";

export type IntervalUnit = "week" | "month";

export interface CustomPlanForm {
  /** Stable client-side id for React key + diffing. Not sent to the server. */
  uid: string;
  label: string;
  amount: string;
  interval_unit: IntervalUnit;
  interval_count: string;
  num_payments: string;
  is_default: boolean;
}

export interface TermOptionForm {
  duration_value: string;
  duration_unit: IntervalUnit;
  label: string;
  sort_order: number;
  upfront_enabled: boolean;
  upfront_amount: string;
  weekly_enabled: boolean;
  weekly_amount: string;
  fortnightly_enabled: boolean;
  fortnightly_amount: string;
  monthly_enabled: boolean;
  monthly_amount: string;
  quarterly_enabled: boolean;
  quarterly_amount: string;
  custom_plans: CustomPlanForm[];
  default_schedule_key: string;
}

export interface PaymentScheduleInput {
  kind: "upfront" | "recurring" | "instalments";
  label: string | null;
  amount: number;
  interval_unit: IntervalUnit | null;
  interval_count: number | null;
  num_payments: number | null;
  is_default: boolean;
  sort_order: number;
}

export const STANDARD_KEYS = [
  "upfront",
  "weekly",
  "fortnightly",
  "monthly",
  "quarterly",
] as const;
export type StandardKey = (typeof STANDARD_KEYS)[number];

interface StandardSpec {
  kind: "upfront" | "recurring";
  intervalUnit: IntervalUnit | null;
  intervalCount: number | null;
}

const STANDARD_SPEC: Record<StandardKey, StandardSpec> = {
  upfront: { kind: "upfront", intervalUnit: null, intervalCount: null },
  weekly: { kind: "recurring", intervalUnit: "week", intervalCount: 1 },
  fortnightly: { kind: "recurring", intervalUnit: "week", intervalCount: 2 },
  monthly: { kind: "recurring", intervalUnit: "month", intervalCount: 1 },
  quarterly: { kind: "recurring", intervalUnit: "month", intervalCount: 3 },
};

const num = (s: string): number => {
  const n = Number(s);
  return Number.isFinite(n) ? n : 0;
};

const intOrNull = (s: string): number | null => {
  const n = Number(s);
  if (!Number.isFinite(n) || n <= 0) return null;
  return Math.round(n);
};

const matchStandard = (s: PaymentSchedule): StandardKey | null => {
  if (s.kind === "upfront") return "upfront";
  if (s.kind !== "recurring") return null;
  const unit = s.interval_unit;
  const count = s.interval_count ?? 0;
  if (unit === "week" && count === 1) return "weekly";
  if (unit === "week" && count === 2) return "fortnightly";
  if (unit === "month" && count === 1) return "monthly";
  if (unit === "month" && count === 3) return "quarterly";
  return null;
};

export const emptyTermOptionForm = (): TermOptionForm => ({
  duration_value: "",
  duration_unit: "month",
  label: "",
  sort_order: 0,
  upfront_enabled: false,
  upfront_amount: "",
  weekly_enabled: false,
  weekly_amount: "",
  fortnightly_enabled: false,
  fortnightly_amount: "",
  monthly_enabled: false,
  monthly_amount: "",
  quarterly_enabled: false,
  quarterly_amount: "",
  custom_plans: [],
  default_schedule_key: "",
});

/** Convert the React form to the payload shape `replacePaymentSchedules` expects. */
export const payloadFromForm = (form: TermOptionForm): PaymentScheduleInput[] => {
  const out: PaymentScheduleInput[] = [];
  let sortIdx = 0;

  const pushStandard = (key: StandardKey, enabled: boolean, amount: string) => {
    if (!enabled) return;
    const n = num(amount);
    if (n <= 0) return;
    const spec = STANDARD_SPEC[key];
    out.push({
      kind: spec.kind,
      label: null,
      amount: n,
      interval_unit: spec.intervalUnit,
      interval_count: spec.intervalCount,
      // Standard recurring rows continue indefinitely unless the admin uses
      // a custom plan to cap N. This matches the pre-v2 behaviour: the
      // `continue_after_term` flag drove num_payments for legacy frequencies.
      num_payments: null,
      is_default: form.default_schedule_key === key,
      sort_order: sortIdx++,
    });
  };

  pushStandard("upfront", form.upfront_enabled, form.upfront_amount);
  pushStandard("weekly", form.weekly_enabled, form.weekly_amount);
  pushStandard("fortnightly", form.fortnightly_enabled, form.fortnightly_amount);
  pushStandard("monthly", form.monthly_enabled, form.monthly_amount);
  pushStandard("quarterly", form.quarterly_enabled, form.quarterly_amount);

  for (const cp of form.custom_plans) {
    const amount = num(cp.amount);
    if (amount <= 0) continue;
    const intervalCount = intOrNull(cp.interval_count) ?? 1;
    const numPayments = intOrNull(cp.num_payments);
    out.push({
      kind: numPayments !== null ? "instalments" : "recurring",
      label: cp.label.trim() || null,
      amount,
      interval_unit: cp.interval_unit,
      interval_count: intervalCount,
      num_payments: numPayments,
      is_default: form.default_schedule_key === `custom:${cp.uid}`,
      sort_order: sortIdx++,
    });
  }

  // If nothing is flagged default, mark the first row default so the signup UI
  // has something selected. Don't fail silently if the list is empty.
  if (out.length > 0 && !out.some(r => r.is_default)) {
    out[0].is_default = true;
  }
  return out;
};

// =============================================================================
// Simple pricing form (post-redesign)
//
// The flat 5-checkbox × amount UI confused admins ("what does quarterly mean?",
// "why do I type amounts five times?"). The new model is:
//
//   * One total contract price.
//   * Tick which cadences (upfront/weekly/fortnightly/monthly) the member can
//     choose at signup. Amounts are auto-derived from total ÷ duration.
//   * Optional single "custom plan" disclosure for exotic schedules (e.g. 10
//     weekly instalments). Multiple custom plans are not surfaced; if legacy
//     data has more than one, only the first is shown (the rest are still
//     preserved server-side until a save overwrites them).
//
// `payloadFromSimple` and `simpleFromPayload` are the canonical converters.
// =============================================================================

export type SimpleDefaultMethod = "upfront" | "weekly" | "fortnightly" | "monthly" | "custom";

export interface SimplePricingForm {
  duration_value: string;
  duration_unit: IntervalUnit;
  label: string;
  total_price: string;
  pay_upfront: boolean;
  pay_weekly: boolean;
  pay_fortnightly: boolean;
  pay_monthly: boolean;
  default_method: SimpleDefaultMethod;
  use_custom: boolean;
  custom_label: string;
  custom_amount: string;
  custom_interval_count: string;
  custom_interval_unit: IntervalUnit;
  custom_num_payments: string;
}

export const emptySimplePricingForm = (): SimplePricingForm => ({
  duration_value: "12",
  duration_unit: "month",
  label: "",
  total_price: "",
  pay_upfront: false,
  pay_weekly: false,
  pay_fortnightly: false,
  pay_monthly: true,
  default_method: "monthly",
  use_custom: false,
  custom_label: "",
  custom_amount: "",
  custom_interval_count: "1",
  custom_interval_unit: "week",
  custom_num_payments: "",
});

/** Convert a duration into weeks / months for amount derivation. */
const durationParts = (value: number, unit: IntervalUnit) => {
  const v = Math.max(0, value);
  const months = unit === "week" ? Math.max(1, Math.ceil(v / 4.345)) : v;
  const weeks = unit === "week" ? v : Math.round(v * 52 / 12);
  const fortnights = Math.max(1, Math.round(weeks / 2));
  return { months, weeks, fortnights };
};

const round2 = (n: number) => Math.round(n * 100) / 100;

export const derivedSimpleAmounts = (form: SimplePricingForm) => {
  const total = num(form.total_price);
  const { months, weeks, fortnights } = durationParts(num(form.duration_value), form.duration_unit);
  return {
    upfront: total > 0 ? round2(total) : 0,
    weekly: total > 0 && weeks > 0 ? round2(total / weeks) : 0,
    fortnightly: total > 0 && fortnights > 0 ? round2(total / fortnights) : 0,
    monthly: total > 0 && months > 0 ? round2(total / months) : 0,
    months,
    weeks,
    fortnights,
  };
};

/** Convert the SimplePricingForm to the schedule payload. */
export const payloadFromSimple = (form: SimplePricingForm): PaymentScheduleInput[] => {
  const a = derivedSimpleAmounts(form);
  const out: PaymentScheduleInput[] = [];
  let sortIdx = 0;
  const push = (
    method: SimpleDefaultMethod,
    enabled: boolean,
    kind: "upfront" | "recurring",
    amount: number,
    intervalUnit: IntervalUnit | null,
    intervalCount: number | null,
  ) => {
    if (!enabled || amount <= 0) return;
    out.push({
      kind,
      label: null,
      amount,
      interval_unit: intervalUnit,
      interval_count: intervalCount,
      num_payments: null,
      is_default: form.default_method === method,
      sort_order: sortIdx++,
    });
  };
  push("upfront", form.pay_upfront, "upfront", a.upfront, null, null);
  push("weekly", form.pay_weekly, "recurring", a.weekly, "week", 1);
  push("fortnightly", form.pay_fortnightly, "recurring", a.fortnightly, "week", 2);
  push("monthly", form.pay_monthly, "recurring", a.monthly, "month", 1);

  if (form.use_custom) {
    const cAmount = num(form.custom_amount);
    const cN = intOrNull(form.custom_num_payments);
    if (cAmount > 0) {
      out.push({
        kind: cN !== null ? "instalments" : "recurring",
        label: form.custom_label.trim() || null,
        amount: round2(cAmount),
        interval_unit: form.custom_interval_unit,
        interval_count: intOrNull(form.custom_interval_count) ?? 1,
        num_payments: cN,
        is_default: form.default_method === "custom",
        sort_order: sortIdx++,
      });
    }
  }

  // Ensure exactly one default row.
  if (out.length > 0 && !out.some(r => r.is_default)) {
    out[0].is_default = true;
  }
  return out;
};

/** Hydrate a SimplePricingForm from existing schedules. */
export const simpleFromPayload = (
  schedules: PaymentSchedule[],
  durationValue: number,
  durationUnit: IntervalUnit,
  label?: string | null,
): SimplePricingForm => {
  const f = emptySimplePricingForm();
  f.duration_value = String(durationValue || "");
  f.duration_unit = durationUnit;
  f.label = label ?? "";
  // Start with nothing ticked — we'll set flags from data.
  f.pay_upfront = false;
  f.pay_weekly = false;
  f.pay_fortnightly = false;
  f.pay_monthly = false;
  f.default_method = "monthly";

  const active = schedules
    .filter(s => !s.archived_at)
    .sort((a, b) => a.sort_order - b.sort_order);

  const { months, weeks, fortnights } = durationParts(durationValue, durationUnit);
  let total = 0;
  let pickedDefault = false;

  for (const s of active) {
    const isWeeklyOpen = s.kind === "recurring" && s.interval_unit === "week" && s.interval_count === 1 && s.num_payments == null;
    const isFortnightlyOpen = s.kind === "recurring" && s.interval_unit === "week" && s.interval_count === 2 && s.num_payments == null;
    const isMonthlyOpen = s.kind === "recurring" && s.interval_unit === "month" && s.interval_count === 1 && s.num_payments == null;

    if (s.kind === "upfront") {
      f.pay_upfront = true;
      if (total === 0 || s.amount > total) total = s.amount;
      if (s.is_default) { f.default_method = "upfront"; pickedDefault = true; }
    } else if (isWeeklyOpen) {
      f.pay_weekly = true;
      if (total === 0 && weeks > 0) total = s.amount * weeks;
      if (s.is_default) { f.default_method = "weekly"; pickedDefault = true; }
    } else if (isFortnightlyOpen) {
      f.pay_fortnightly = true;
      if (total === 0 && fortnights > 0) total = s.amount * fortnights;
      if (s.is_default) { f.default_method = "fortnightly"; pickedDefault = true; }
    } else if (isMonthlyOpen) {
      f.pay_monthly = true;
      if (total === 0 && months > 0) total = s.amount * months;
      if (s.is_default) { f.default_method = "monthly"; pickedDefault = true; }
    } else if (!f.use_custom) {
      // First non-standard row becomes the visible custom plan.
      f.use_custom = true;
      f.custom_label = s.label ?? "";
      f.custom_amount = String(s.amount);
      f.custom_interval_unit = (s.interval_unit as IntervalUnit) ?? "month";
      f.custom_interval_count = String(s.interval_count ?? 1);
      f.custom_num_payments = s.num_payments != null ? String(s.num_payments) : "";
      if (total === 0 && s.num_payments && s.num_payments > 0) total = s.amount * s.num_payments;
      if (s.is_default) { f.default_method = "custom"; pickedDefault = true; }
    }
  }

  if (total > 0) {
    f.total_price = round2(total).toFixed(2);
  }

  // If nothing was ticked at all, leave monthly as the default ticked option
  // so the form renders sensibly. Otherwise honour whatever the data said.
  const anyTicked = f.pay_upfront || f.pay_weekly || f.pay_fortnightly || f.pay_monthly || f.use_custom;
  if (!anyTicked) {
    f.pay_monthly = true;
    f.default_method = "monthly";
  } else if (!pickedDefault) {
    // Pick the first ticked option as default.
    if (f.pay_upfront) f.default_method = "upfront";
    else if (f.pay_weekly) f.default_method = "weekly";
    else if (f.pay_fortnightly) f.default_method = "fortnightly";
    else if (f.pay_monthly) f.default_method = "monthly";
    else if (f.use_custom) f.default_method = "custom";
  }

  return f;
};

/** Hydrate a form from existing schedule rows returned by the API. */
export const formFromPayload = (
  schedules: PaymentSchedule[],
  durationValue: number,
  durationUnit: IntervalUnit,
  label?: string | null,
  sortOrder = 0,
): TermOptionForm => {
  const form = emptyTermOptionForm();
  form.duration_value = String(durationValue || "");
  form.duration_unit = durationUnit;
  form.label = label ?? "";
  form.sort_order = sortOrder;

  // Sort by sort_order so we deterministically pick the first matching row
  // when the admin (unwisely) saved two schedules with the same shape.
  const sorted = [...schedules].sort((a, b) => a.sort_order - b.sort_order);
  let customIdx = 0;

  for (const s of sorted) {
    if (s.archived_at) continue;
    const key = matchStandard(s);
    if (key) {
      const setStandard = (k: StandardKey) => {
        switch (k) {
          case "upfront":
            form.upfront_enabled = true;
            form.upfront_amount = String(s.amount);
            break;
          case "weekly":
            form.weekly_enabled = true;
            form.weekly_amount = String(s.amount);
            break;
          case "fortnightly":
            form.fortnightly_enabled = true;
            form.fortnightly_amount = String(s.amount);
            break;
          case "monthly":
            form.monthly_enabled = true;
            form.monthly_amount = String(s.amount);
            break;
          case "quarterly":
            form.quarterly_enabled = true;
            form.quarterly_amount = String(s.amount);
            break;
        }
      };
      // Standard recurring with num_payments set is technically still a fixed
      // instalment plan. Surface it as a custom plan so the cap doesn't get
      // silently dropped on round-trip.
      if (s.kind === "recurring" && s.num_payments != null) {
        const uid = `existing-${customIdx++}-${s.id}`;
        form.custom_plans.push({
          uid,
          label: s.label ?? "",
          amount: String(s.amount),
          interval_unit: (s.interval_unit as IntervalUnit) ?? "month",
          interval_count: String(s.interval_count ?? 1),
          num_payments: String(s.num_payments),
          is_default: s.is_default,
        });
        if (s.is_default) form.default_schedule_key = `custom:${uid}`;
        continue;
      }
      setStandard(key);
      if (s.is_default) form.default_schedule_key = key;
    } else {
      const uid = `existing-${customIdx++}-${s.id}`;
      form.custom_plans.push({
        uid,
        label: s.label ?? "",
        amount: String(s.amount),
        interval_unit: (s.interval_unit as IntervalUnit) ?? "month",
        interval_count: String(s.interval_count ?? 1),
        num_payments: s.num_payments != null ? String(s.num_payments) : "",
        is_default: s.is_default,
      });
      if (s.is_default) form.default_schedule_key = `custom:${uid}`;
    }
  }

  return form;
};
