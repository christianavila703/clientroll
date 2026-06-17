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
