const ISO_DATE_ONLY_RE = /^\d{4}-\d{2}-\d{2}$/;

export function tenantTimezoneOrUTC(value?: string | null): string {
  const tz = (value || "").trim();
  return tz || "UTC";
}

export function formatTenantDate(
  value: string | Date | null | undefined,
  timezone: string,
  options: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", year: "numeric" }
): string {
  if (!value) return "—";
  const tz = tenantTimezoneOrUTC(timezone);

  // Date-only values already represent tenant-local calendar dates in the domain model.
  // Keep the day stable in UI regardless of browser locale/timezone.
  if (typeof value === "string" && ISO_DATE_ONLY_RE.test(value)) {
    const [y, m, d] = value.split("-").map(Number);
    const safeUTC = new Date(Date.UTC(y, (m || 1) - 1, d || 1, 0, 0, 0));
    return new Intl.DateTimeFormat("en-AU", { ...options, timeZone: "UTC" }).format(safeUTC);
  }

  const parsed = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(parsed.getTime())) return "—";
  return new Intl.DateTimeFormat("en-AU", { ...options, timeZone: tz }).format(parsed);
}

export function daysUntilDateOnly(targetDateISO: string, timezone: string): number {
  if (!ISO_DATE_ONLY_RE.test(targetDateISO)) return 0;
  const tz = tenantTimezoneOrUTC(timezone);
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date());
  const year = Number(parts.find((p) => p.type === "year")?.value || "0");
  const month = Number(parts.find((p) => p.type === "month")?.value || "1");
  const day = Number(parts.find((p) => p.type === "day")?.value || "1");
  const todayUTC = Date.UTC(year, month - 1, day);

  const [ty, tm, td] = targetDateISO.split("-").map(Number);
  const targetUTC = Date.UTC(ty, tm - 1, td);
  return Math.ceil((targetUTC - todayUTC) / (1000 * 60 * 60 * 24));
}
