# Implementation Gaps / Placeholders (Internal Review)

This document captures **known placeholders and “not implemented” areas** found by scanning the repository for:

- `PlaceholderPage` usage and “Phase …” markers
- `not implemented` / `not yet implemented` error strings
- Empty error handlers in the frontend (silent `.catch(() => {})`)

It is intended as a developer-facing “what’s missing” index, not a product roadmap.

---

## 1. Confirmed placeholders / missing implementations

### 1.1 Platform Billing (Phase 8) – end-to-end missing

- **Frontend route is a placeholder**
  - `frontend/src/App.tsx` routes `/platform/billing` to `PlaceholderPage` (“Coming in Phase 8”).
  - `frontend/src/components/common/PlaceholderPage.tsx` is a generic placeholder component.
  - `frontend/src/components/layout/Sidebar.tsx` includes a **Platform → Billing** nav item to `/platform/billing`.

- **Backend has no routes/handlers for platform billing**
  - `cmd/server/main.go` includes `// Phase 8: Platform billing` inside `/api/platform` but registers no billing routes.
  - `internal/handlers/platform.go` is limited to tenant CRUD; no billing config/subscription endpoints exist.

- **Database schema already includes billing tables (unused by handlers)**
  - `migrations/002_create_platform_tables.up.sql`: `platform_billing_config` (Stripe keys encrypted, per-member pricing, enabled flag).
  - `migrations/003_create_tenant_core_tables.up.sql`: `tenant_subscriptions`, `tenant_usage_snapshots`, `tenant_invoices`.
  - `migrations/009_enable_rls.up.sql`: RLS policies exist for `tenant_subscriptions`, `tenant_usage_snapshots`, `tenant_invoices`.

- **Billing code exists but is not wired into the server**
  - `internal/billing/provider.go`: interface `PlatformBillingProvider`.
  - `internal/billing/stripe.go`: `StripeBillingProvider` is a stub; every method returns `fmt.Errorf("Stripe integration not yet implemented")`.
  - `internal/billing/mock.go`: `MockBillingProvider` exists for development/testing.
  - `internal/billing/usage.go`: `UsageTracker` can snapshot daily active members, but is **not called anywhere** (“should be called by a daily cron job”).

#### What’s missing (minimum viable platform billing)

- **Backend**
  - Platform-admin endpoints to:
    - configure platform billing (`platform_billing_config`) including encryption/decryption of Stripe keys
    - create/manage tenant subscriptions (`tenant_subscriptions`)
    - expose usage history (`tenant_usage_snapshots`) and invoices (`tenant_invoices`)
    - produce tenant usage snapshots on a schedule (cron/job runner) and mark invoicing state
  - Stripe integration implementation:
    - implement `internal/billing/stripe.go` methods
    - add Stripe webhook handling for billing events (platform-level, separate from tenant “card payments” usage)

- **Frontend**
  - Replace `/platform/billing` placeholder with real screens:
    - billing config (price per member, enable/disable, Stripe keys status)
    - tenant subscription status and usage history
    - invoice list and portal/session link (if supported)

#### Recommended implementation order

1. **Decide billing surface area**: platform-wide billing only vs. also tenant-level Stripe (payments are already partly Stripe-integrated via webhooks).
2. **Backend read-only first**: add endpoints to view config/usage/subscriptions (safe, no Stripe calls).
3. **Job runner**: wire `UsageTracker.SnapshotAllTenants` behind a cron endpoint or a scheduled process (systemd timer, cron, or internal worker).
4. **Stripe integration**: implement provider methods + webhook reconciliation.
5. **Frontend**: replace placeholder UI and add error handling.

---

## 2. “Silent error” placeholders in the frontend

Several pages intentionally swallow fetch errors with empty handlers, e.g. `.catch(() => {})`. This is effectively a UX placeholder: failures become invisible and can look like “empty data”.

Files with silent catch blocks (non-exhaustive, confirmed by scan):

- `frontend/src/components/dashboard/DashboardPage.tsx` (multiple dashboard queries)
- `frontend/src/components/layout/Header.tsx` (notification stats fetch)
- `frontend/src/components/reports/ReportsPage.tsx`
- `frontend/src/components/platform/PlatformDashboard.tsx`
- `frontend/src/components/settings/SettingsPage.tsx`
- `frontend/src/components/signup/StaffSignupLauncher.tsx`

Suggested improvements:

- Surface a small inline error (“Failed to load…”) where appropriate.
- Capture errors to a client-side logger (even `console.error`) in development.
- Standardize a pattern: `useQuery`-style hook or a small `loadState` helper (`loading`, `error`, `data`).

---

## 3. What we did *not* find

- No first-party backend `TODO`/`FIXME` markers in `internal/` at time of scan.
- Many `TODO`/`Not implemented` matches exist in third-party dependencies (for example Playwright), which are not actionable for the application.

---

## 4. Quick reference (files)

### Platform billing (Phase 8)

- **Frontend**
  - `frontend/src/App.tsx`
  - `frontend/src/components/common/PlaceholderPage.tsx`
  - `frontend/src/components/layout/Sidebar.tsx`
- **Backend**
  - `cmd/server/main.go`
  - `internal/handlers/platform.go`
  - `internal/billing/provider.go`
  - `internal/billing/stripe.go`
  - `internal/billing/mock.go`
  - `internal/billing/usage.go`
- **Migrations**
  - `migrations/002_create_platform_tables.up.sql`
  - `migrations/003_create_tenant_core_tables.up.sql`
  - `migrations/009_enable_rls.up.sql`

