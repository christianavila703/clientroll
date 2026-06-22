import { describe, expect, it } from "vitest";

import type { PaymentSchedule } from "../types/memberships";
import {
  emptyTermOptionForm,
  formFromPayload,
  payloadFromForm,
} from "./termOptionAdapter";

const makeSchedule = (overrides: Partial<PaymentSchedule>): PaymentSchedule => ({
  id: overrides.id ?? "00000000-0000-0000-0000-000000000001",
  tenant_id: "tenant",
  term_option_id: "term",
  kind: overrides.kind ?? "recurring",
  label: overrides.label ?? null,
  amount: overrides.amount ?? 0,
  interval_unit: overrides.interval_unit ?? null,
  interval_count: overrides.interval_count ?? null,
  num_payments: overrides.num_payments ?? null,
  is_default: overrides.is_default ?? false,
  sort_order: overrides.sort_order ?? 0,
  archived_at: overrides.archived_at ?? null,
  created_at: "now",
  updated_at: "now",
});

describe("payloadFromForm", () => {
  it("emits one row per enabled standard frequency", () => {
    const form = emptyTermOptionForm();
    form.duration_value = "12";
    form.duration_unit = "month";
    form.upfront_enabled = true;
    form.upfront_amount = "1200";
    form.weekly_enabled = true;
    form.weekly_amount = "25";
    form.default_schedule_key = "weekly";

    const payload = payloadFromForm(form);

    expect(payload).toHaveLength(2);
    expect(payload[0]).toMatchObject({ kind: "upfront", amount: 1200, is_default: false });
    expect(payload[1]).toMatchObject({
      kind: "recurring",
      amount: 25,
      interval_unit: "week",
      interval_count: 1,
      is_default: true,
    });
  });

  it("skips standard frequencies with non-positive amounts", () => {
    const form = emptyTermOptionForm();
    form.upfront_enabled = true;
    form.upfront_amount = "0";
    form.monthly_enabled = true;
    form.monthly_amount = "";
    expect(payloadFromForm(form)).toEqual([]);
  });

  it("classifies custom plans without num_payments as recurring, otherwise instalments", () => {
    const form = emptyTermOptionForm();
    form.custom_plans = [
      {
        uid: "a",
        label: "Pay every two weeks indefinitely",
        amount: "50",
        interval_unit: "week",
        interval_count: "2",
        num_payments: "",
        is_default: false,
      },
      {
        uid: "b",
        label: "Three instalments",
        amount: "400",
        interval_unit: "month",
        interval_count: "1",
        num_payments: "3",
        is_default: true,
      },
    ];
    form.default_schedule_key = "custom:b";
    const payload = payloadFromForm(form);
    expect(payload).toHaveLength(2);
    expect(payload[0].kind).toBe("recurring");
    expect(payload[0].num_payments).toBeNull();
    expect(payload[1].kind).toBe("instalments");
    expect(payload[1].num_payments).toBe(3);
    expect(payload[1].is_default).toBe(true);
  });

  it("auto-marks first row as default when none flagged", () => {
    const form = emptyTermOptionForm();
    form.weekly_enabled = true;
    form.weekly_amount = "10";
    form.default_schedule_key = "";
    const payload = payloadFromForm(form);
    expect(payload).toHaveLength(1);
    expect(payload[0].is_default).toBe(true);
  });
});

describe("formFromPayload", () => {
  it("hydrates standard frequency rows back into checkbox+amount", () => {
    const schedules: PaymentSchedule[] = [
      makeSchedule({ kind: "upfront", amount: 600, sort_order: 0 }),
      makeSchedule({
        kind: "recurring",
        amount: 50,
        interval_unit: "month",
        interval_count: 1,
        sort_order: 1,
        is_default: true,
      }),
    ];
    const form = formFromPayload(schedules, 12, "month");
    expect(form.upfront_enabled).toBe(true);
    expect(form.upfront_amount).toBe("600");
    expect(form.monthly_enabled).toBe(true);
    expect(form.monthly_amount).toBe("50");
    expect(form.default_schedule_key).toBe("monthly");
  });

  it("treats a recurring row with num_payments as a custom instalment plan", () => {
    const schedules: PaymentSchedule[] = [
      makeSchedule({
        kind: "recurring",
        amount: 100,
        interval_unit: "month",
        interval_count: 1,
        num_payments: 4,
        is_default: true,
      }),
    ];
    const form = formFromPayload(schedules, 4, "month");
    expect(form.monthly_enabled).toBe(false);
    expect(form.custom_plans).toHaveLength(1);
    expect(form.custom_plans[0].num_payments).toBe("4");
    expect(form.default_schedule_key.startsWith("custom:")).toBe(true);
  });

  it("ignores archived rows", () => {
    const schedules: PaymentSchedule[] = [
      makeSchedule({ kind: "upfront", amount: 600, archived_at: "yesterday" }),
    ];
    const form = formFromPayload(schedules, 12, "month");
    expect(form.upfront_enabled).toBe(false);
  });
});

describe("round-trip fidelity", () => {
  it("standard frequencies survive form -> payload -> form", () => {
    const form = emptyTermOptionForm();
    form.duration_value = "6";
    form.duration_unit = "month";
    form.upfront_enabled = true;
    form.upfront_amount = "1200";
    form.weekly_enabled = true;
    form.weekly_amount = "25";
    form.fortnightly_enabled = true;
    form.fortnightly_amount = "50";
    form.monthly_enabled = true;
    form.monthly_amount = "200";
    form.default_schedule_key = "monthly";

    const payload = payloadFromForm(form);
    const fakeSchedules: PaymentSchedule[] = payload.map((p, idx) =>
      makeSchedule({
        id: String(idx),
        kind: p.kind,
        label: p.label ?? null,
        amount: p.amount,
        interval_unit: p.interval_unit ?? null,
        interval_count: p.interval_count ?? null,
        num_payments: p.num_payments ?? null,
        is_default: p.is_default,
        sort_order: p.sort_order,
      })
    );
    const round = formFromPayload(fakeSchedules, 6, "month");
    expect(round.upfront_enabled).toBe(true);
    expect(round.upfront_amount).toBe("1200");
    expect(round.weekly_amount).toBe("25");
    expect(round.fortnightly_amount).toBe("50");
    expect(round.monthly_amount).toBe("200");
    expect(round.default_schedule_key).toBe("monthly");
  });

  it("custom instalments survive form -> payload -> form", () => {
    const form = emptyTermOptionForm();
    form.duration_value = "12";
    form.duration_unit = "month";
    form.custom_plans = [
      {
        uid: "x",
        label: "10 weekly payments",
        amount: "120",
        interval_unit: "week",
        interval_count: "1",
        num_payments: "10",
        is_default: true,
      },
    ];
    form.default_schedule_key = "custom:x";
    const payload = payloadFromForm(form);
    expect(payload[0]).toMatchObject({
      kind: "instalments",
      amount: 120,
      interval_unit: "week",
      interval_count: 1,
      num_payments: 10,
      is_default: true,
    });
    const fakeSchedules = payload.map((p, idx) =>
      makeSchedule({
        id: String(idx),
        kind: p.kind,
        label: p.label ?? null,
        amount: p.amount,
        interval_unit: p.interval_unit ?? null,
        interval_count: p.interval_count ?? null,
        num_payments: p.num_payments ?? null,
        is_default: p.is_default,
        sort_order: p.sort_order,
      })
    );
    const round = formFromPayload(fakeSchedules, 12, "month");
    expect(round.custom_plans).toHaveLength(1);
    expect(round.custom_plans[0].num_payments).toBe("10");
    expect(round.custom_plans[0].label).toBe("10 weekly payments");
    expect(round.default_schedule_key.startsWith("custom:")).toBe(true);
  });
});
