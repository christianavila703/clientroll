// ClientRoll brand assets — use these everywhere for a consistent SaaS look
import logoUrl from "./logo.png";
import logoDarkTextUrl from "./logo-dark-text.png";
import logoWhiteUrl from "./logo-white.png";
import iconUrl from "./icon.png";

export const brand = {
  /** Main logo (light background) */
  logo: logoUrl,
  /** Logo with dark text (light background, alternate) */
  logoDarkText: logoDarkTextUrl,
  /** Logo for dark backgrounds */
  logoWhite: logoWhiteUrl,
  /** Icon / favicon / small placements */
  icon: iconUrl,
  appName: "ClientRoll",
} as const;
