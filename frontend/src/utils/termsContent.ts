const HTML_TAG_PATTERN = /<\/?[a-z][\s\S]*?>/i;

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

export function looksLikeTermsHtml(raw: string): boolean {
  return HTML_TAG_PATTERN.test(raw.trim());
}

/** Convert plain text (with paragraph/line breaks) to HTML for display or the rich editor. */
export function plainTextToTermsHtml(raw: string): string {
  const content = raw.replace(/\r\n/g, "\n").trim();
  if (!content) return "";

  const parts = content.split(/\n{2,}/);
  return parts
    .map((part) => {
      const block = part.trim();
      if (!block) return "";
      const lines = escapeHtml(block).replace(/\n/g, "<br>");
      return `<p>${lines}</p>`;
    })
    .filter(Boolean)
    .join("");
}

/** Normalize stored terms for HTML display (signup, previews). */
export function formatTermsContentForDisplay(raw: string): string {
  const content = raw?.trim() ?? "";
  if (!content) return "";
  if (looksLikeTermsHtml(content)) return content;
  return plainTextToTermsHtml(content);
}

/** Normalize for TipTap — editor expects HTML, not a single plain-text blob. */
export function normalizeTermsEditorContent(raw: string): string {
  const content = raw?.trim() ?? "";
  if (!content) return "";
  if (looksLikeTermsHtml(content)) return content;
  return plainTextToTermsHtml(content);
}

/**
 * Context for substituting {{variables}} in T&C content at signup time.
 * Mirrors the server-side `replaceKnownTermsPlaceholders` in
 * internal/handlers/signup.go so the client sees the same text it signs.
 */
export interface TermsSubstitutionContext {
  client_name?: string;
  client_email?: string;
  client_phone?: string;
  client_address?: string;
  client_dob?: string;
  client_gender?: string;
  emergency_contact_name?: string;
  emergency_contact_phone?: string;
  membership_title?: string;
  term_months?: number | string;
  term_weeks?: number | string;
  total_fee?: number | string;
  payment_amount?: number | string;
  payment_frequency?: string;
  payment_method?: string;
  start_date?: string;
  end_date?: string;
  company_name?: string;
  company_trading_name?: string;
  company_abn?: string;
  company_address?: string;
  company_email?: string;
  company_phone?: string;
  today_date?: string;
}

const ALIAS_KEYS: Record<string, keyof TermsSubstitutionContext> = {
  "client.name": "client_name",
  "contact.name": "client_name",
  "member.name": "client_name",
  "client.address": "client_address",
};

const PLACEHOLDER_PATTERN = /\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}/g;
const STRIP_REMAINING_VARS = /\{\{\s*[^}]*\s*\}\}/g;
const STRIP_CONTROL_TAGS = /\{%\s*[^%]*\s*%\}/g;

function formatMoney(value: number | string | undefined): string {
  if (value === undefined || value === null || value === "") return "";
  const num = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(num)) return "";
  return num.toFixed(2);
}

function todayFormatted(): string {
  const now = new Date();
  return now.toLocaleDateString("en-AU", { day: "numeric", month: "long", year: "numeric" });
}

/**
 * Replaces every supported {{placeholder}} in `raw` with its value from `ctx`.
 * Unknown placeholders are stripped (same behaviour as the server-side
 * agreement-PDF renderer) so the signup view and the stored PDF agree.
 */
export function substituteTermsVariables(raw: string, ctx: TermsSubstitutionContext): string {
  if (!raw) return raw;

  const lookup: Record<string, string> = {
    client_name: (ctx.client_name || "Client").trim(),
    client_email: (ctx.client_email || "").trim(),
    client_phone: (ctx.client_phone || "").trim(),
    client_address: (ctx.client_address || "Client address").trim(),
    client_dob: (ctx.client_dob || "").trim(),
    client_gender: (ctx.client_gender || "").trim(),
    emergency_contact_name: (ctx.emergency_contact_name || "").trim(),
    emergency_contact_phone: (ctx.emergency_contact_phone || "").trim(),
    membership_title: (ctx.membership_title || "").trim(),
    term_months: ctx.term_months !== undefined && ctx.term_months !== "" ? String(ctx.term_months) : "",
    term_weeks: ctx.term_weeks !== undefined && ctx.term_weeks !== "" ? String(ctx.term_weeks) : "",
    total_fee: formatMoney(ctx.total_fee),
    payment_amount: formatMoney(ctx.payment_amount),
    payment_frequency: (ctx.payment_frequency || "").trim(),
    payment_method: (ctx.payment_method || "").trim(),
    start_date: (ctx.start_date || "").trim(),
    end_date: (ctx.end_date || "").trim(),
    company_name: (ctx.company_name || "").trim(),
    company_trading_name: (ctx.company_trading_name || "").trim(),
    company_abn: (ctx.company_abn || "").trim(),
    company_address: (ctx.company_address || "").trim(),
    company_email: (ctx.company_email || "").trim(),
    company_phone: (ctx.company_phone || "").trim(),
    today_date: (ctx.today_date || todayFormatted()).trim(),
  };

  // Replace known placeholders. Unknown ones are left in place for the strip pass.
  let out = raw.replace(PLACEHOLDER_PATTERN, (match, rawKey) => {
    const key = String(rawKey).trim();
    if (key in lookup) return lookup[key];
    const aliased = ALIAS_KEYS[key];
    if (aliased && aliased in lookup) return lookup[aliased];
    return match;
  });

  // Mirror the server's strip pass for unresolved templates so the visible
  // text matches the eventual PDF (no raw {{...}} on the signature page).
  out = out.replace(STRIP_CONTROL_TAGS, "");
  out = out.replace(STRIP_REMAINING_VARS, "");
  return out;
}
