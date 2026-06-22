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
