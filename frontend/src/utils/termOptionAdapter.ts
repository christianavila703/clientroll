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
// One term option = one payment frequency. The admin sets a total price and
// picks ONE method (upfront / weekly / fortnightly / monthly / custom plan).
// If the gym wants to offer both "$1,200 upfront" and "$100/month", that's
// two separate term options.
//
// `payloadFromSimple` and `simpleFromPayload` are the canonical converters.
// =============================================================================

export type SimplePaymentMethod = "upfront" | "weekly" | "fortnightly" | "monthly" | "custom";

export interface SimplePricingForm {
  duration_value: string;
  duration_unit: IntervalUnit;
  label: string;
  total_price: string;
  payment_method: SimplePaymentMethod;
  // Only used when payment_method === "custom":
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
  payment_method: "monthly",
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
    custom: derivedCustomAmount(form),
    months,
    weeks,
    fortnights,
  };
};

/** Derive the per-payment amount for the custom plan from total + cadence.
 *
 *   - If `custom_num_payments` is N>0: amount = total / N
 *   - Else (ongoing recurring): amount = total / (term span / interval_count)
 *   - Returns 0 when total is not set or cadence inputs are invalid.
 */
const derivedCustomAmount = (form: SimplePricingForm): number => {
  const total = num(form.total_price);
  if (total <= 0) return 0;
  const intervalCount = intOrNull(form.custom_interval_count) ?? 1;
  const explicitN = intOrNull(form.custom_num_payments);
  if (explicitN !== null) {
    return round2(total / explicitN);
  }
  const { months, weeks } = durationParts(num(form.duration_value), form.duration_unit);
  const span = form.custom_interval_unit === "week" ? weeks : months;
  const n = Math.max(1, Math.floor(span / intervalCount));
  return n > 0 ? round2(total / n) : 0;
};

/** Convert the SimplePricingForm to the schedule payload (exactly one row). */
export const payloadFromSimple = (form: SimplePricingForm): PaymentScheduleInput[] => {
  const a = derivedSimpleAmounts(form);
  switch (form.payment_method) {
    case "upfront":
      if (a.upfront <= 0) return [];
      return [{
        kind: "upfront",
        label: null,
        amount: a.upfront,
        interval_unit: null,
        interval_count: null,
        num_payments: null,
        is_default: true,
        sort_order: 0,
      }];
    case "weekly":
      if (a.weekly <= 0) return [];
      return [{
        kind: "recurring",
        label: null,
        amount: a.weekly,
        interval_unit: "week",
        interval_count: 1,
        num_payments: null,
        is_default: true,
        sort_order: 0,
      }];
    case "fortnightly":
      if (a.fortnightly <= 0) return [];
      return [{
        kind: "recurring",
        label: null,
        amount: a.fortnightly,
        interval_unit: "week",
        interval_count: 2,
        num_payments: null,
        is_default: true,
        sort_order: 0,
      }];
    case "monthly":
      if (a.monthly <= 0) return [];
      return [{
        kind: "recurring",
        label: null,
        amount: a.monthly,
        interval_unit: "month",
        interval_count: 1,
        num_payments: null,
        is_default: true,
        sort_order: 0,
      }];
    case "custom": {
      if (a.custom <= 0) return [];
      const cN = intOrNull(form.custom_num_payments);
      return [{
        kind: cN !== null ? "instalments" : "recurring",
        label: form.custom_label.trim() || null,
        amount: a.custom,
        interval_unit: form.custom_interval_unit,
        interval_count: intOrNull(form.custom_interval_count) ?? 1,
        num_payments: cN,
        is_default: true,
        sort_order: 0,
      }];
    }
  }
};

export interface SimpleHydrateResult {
  form: SimplePricingForm;
  /** Count of additional active schedules that the simplified single-method form cannot represent. */
  extraScheduleCount: number;
}

const classifySchedule = (s: PaymentSchedule): SimplePaymentMethod => {
  if (s.kind === "upfront") return "upfront";
  if (s.kind === "recurring" && s.interval_unit === "week" && s.interval_count === 1 && s.num_payments == null) return "weekly";
  if (s.kind === "recurring" && s.interval_unit === "week" && s.interval_count === 2 && s.num_payments == null) return "fortnightly";
  if (s.kind === "recurring" && s.interval_unit === "month" && s.interval_count === 1 && s.num_payments == null) return "monthly";
  return "custom";
};

/** Hydrate a SimplePricingForm from existing schedules.
 *
 * `feeTotal` (the term option's `fee_total` column) is preferred as the
 * canonical total when it's positive. This avoids the 1¢ rounding drift you
 * get from re-multiplying the per-period amount back to a total (e.g.
 * $5,000 ÷ 3 = $1,666.67 each → $1,666.67 × 3 = $5,000.01).
 */
export const simpleFromPayload = (
  schedules: PaymentSchedule[],
  durationValue: number,
  durationUnit: IntervalUnit,
  label?: string | null,
  feeTotal?: number | null,
): SimpleHydrateResult => {
  const f = emptySimplePricingForm();
  f.duration_value = String(durationValue || "");
  f.duration_unit = durationUnit;
  f.label = label ?? "";

  const active = schedules
    .filter(s => !s.archived_at)
    .sort((a, b) => a.sort_order - b.sort_order);

  // Pick the schedule the form will represent: prefer the row flagged
  // is_default, fall back to the first sort_order.
  const chosen = active.find(s => s.is_default) ?? active[0];
  if (!chosen) {
    if (feeTotal && feeTotal > 0) f.total_price = round2(feeTotal).toFixed(2);
    return { form: f, extraScheduleCount: 0 };
  }

  const method = classifySchedule(chosen);
  f.payment_method = method;

  // For custom plans we still need to surface the cadence fields.
  if (method === "custom") {
    f.custom_label = chosen.label ?? "";
    f.custom_amount = String(chosen.amount);
    f.custom_interval_unit = (chosen.interval_unit as IntervalUnit) ?? "month";
    f.custom_interval_count = String(chosen.interval_count ?? 1);
    f.custom_num_payments = chosen.num_payments != null ? String(chosen.num_payments) : "";
  }

  // Prefer the term option's stored fee_total (what the admin typed). Fall
  // back to amount × periods when fee_total is missing (e.g. legacy data).
  let total = feeTotal && feeTotal > 0 ? feeTotal : 0;
  if (total <= 0) {
    const { months, weeks, fortnights } = durationParts(durationValue, durationUnit);
    switch (method) {
      case "upfront":
        total = chosen.amount;
        break;
      case "weekly":
        total = chosen.amount * weeks;
        break;
      case "fortnightly":
        total = chosen.amount * fortnights;
        break;
      case "monthly":
        total = chosen.amount * months;
        break;
      case "custom":
        if (chosen.num_payments && chosen.num_payments > 0) {
          total = chosen.amount * chosen.num_payments;
        }
        break;
    }
  }

  if (total > 0) {
    f.total_price = round2(total).toFixed(2);
  }

  const extraScheduleCount = Math.max(0, active.length - 1);
  return { form: f, extraScheduleCount };
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
