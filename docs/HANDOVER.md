# Clientroll — Developer Plan & Handover

**Date:** 2026-07-01
**Project:** Clientroll (MembershipManager repo)

This document explains the entire Clientroll platform to a new developer and outlines exactly what needs to be built, finished, or fixed. Read this top-to-bottom before touching any code.

---

## 1. What Clientroll Is

Clientroll is a multi-tenant SaaS platform for membership management. There are three actors:

1. **Platform owner** (us / Clientroll) — runs the platform, bills tenants for usage
2. **Tenants** (our customers) — businesses like gyms, studios, clubs who use Clientroll to manage their memberships
3. **Members** (tenant's customers) — end users who sign up for memberships and pay the tenant

**There are two completely separate billing systems in this codebase. Do not confuse them.**

- **The product** = tenants billing their members for memberships. This is what the app does. Tenants connect their own Stripe/GoCardless accounts and charge their members. This is working and is the bulk of the codebase.
- **Owner billing** = us (Clientroll) charging tenants $1/active member/month for using the platform. This is a separate Stripe account, separate webhook endpoint, separate database tables, separate code path. It has nothing to do with how tenants charge their members. This needs finishing.

### Tech Stack

| Layer | Tech |
|-------|------|
| Backend | Go, chi router, PostgreSQL with RLS (pgx v5) |
| Frontend | React + TypeScript (Vite) |
| Payments (tenant→member) | Stripe (card + BECS DD), GoCardless (DD) |
| Payments (owner→tenant) | Stripe (separate account, per-member pricing) |
| Email | Pluggable provider (`internal/email`) |
| PDF | Server-side HTML→PDF for signed agreements |
| Auth | JWT, multi-tenant with row-level security |
| Secrets | AES encryption at rest via `MASTER_ENCRYPTION_KEY` |
| Deploy | Docker / docker-compose |

### Directory Layout

```
cmd/
  server/                — main HTTP server entry point
  billing-snapshot/      — daily cron: owner billing (counts members, reports to Stripe)
  verify-term-options/   — one-off migration verification tool

internal/
  handlers/              — all HTTP handlers
    signup.go              — public member signup flow (~3200 lines, most complex file)
    stripe_webhooks.go     — tenant Stripe webhooks (~1756 lines)
    gocardless.go          — tenant GoCardless webhooks
    platform_billing.go    — owner billing admin API
    platform_billing_webhooks.go — owner billing Stripe webhooks
    settings.go            — tenant settings
    payments.go            — payment listing/queries
    services.go            — ad-hoc service charges
    membership_types.go    — membership CRUD
    clients.go             — client CRUD
    reports.go             — financial reports
    terms.go               — T&C management
    agreement*.go          — agreement snapshots
    ...
  billing/               — OWNER BILLING ONLY (us charging tenants — NOT tenant→member)
    stripe.go              — Stripe subscription CRUD for owner billing
    usage.go               — daily active member count snapshots
    mock.go                — dev mock provider
    provider.go            — interface definition
  payment/               — GoCardless client (tenant→member payments, part of the product)
  models/                — domain models
  repository/            — PostgreSQL data access
  config/                — app configuration
  database/              — DB connection + Encryptor
  email/                 — email provider interface
  events/                — webhook dispatcher (outbound)
  pdf/                   — HTML→PDF
  terms/                 — T&C template rendering
  auth/                  — JWT auth
  middleware/            — tenant context, auth

frontend/src/
  components/
    signup/              — public signup UI (the member-facing flow)
    clients/             — client profile, payment history
    memberships/         — membership type management
    payments/            — payment listing
    services/            — ad-hoc service charges
    reports/             — financial reports
    settings/            — tenant settings
    terms/               — T&C editor
    platform/            — OWNER admin panel (billing config, tenant management)
    ...
  utils/
    signupPricing.ts     — frequency/amount resolution for signup
    termOptionAdapter.ts — v1→v2 term option normalisation
    termOptionDisplay.ts — display formatting
    paymentLabels.ts     — payment breakdown labels (base/tax/surcharge)
    termsContent.ts      — T&C variable substitution
  types/                 — TypeScript type definitions

migrations/              — 077 PostgreSQL migrations (golang-migrate format)
```

---

## 2. THE PRODUCT: Tenants Billing Their Members (COMPLETED)

This is what the app does. This is the core product. **This is finished and working — do not break it.**

Tenants use Clientroll to create memberships, publish signup pages, collect payments, and manage their members. Each tenant connects their own Stripe and/or GoCardless accounts. The payments flow directly between the tenant and their members — Clientroll facilitates it but the money never touches us.

### Payment Providers (Per-Tenant)

Each tenant connects their OWN payment accounts. Credentials are stored encrypted in the `tenant_credentials` table.

**Stripe** — used for card payments and Stripe BECS direct debit
- Tenant enters their Stripe API keys in settings
- Checkout Sessions handle initial payment collection
- Subscriptions handle recurring payments
- Webhooks arrive at `POST /api/webhooks/stripe` and are routed by tenant

**GoCardless** — used for direct debit (mandate-based)
- Tenant enters their GoCardless API keys in settings
- Mandates are created during signup
- Subscriptions handle recurring payments
- Webhooks arrive at `POST /api/webhooks/gocardless`

### Payment Record Structure

Every payment stores a line-item breakdown in the `payments` table:
- `amount` — total charged to the member
- `base_amount` — membership fee excluding tax
- `tax_amount` — GST/sales tax
- `processing_fee_amount` — payment processing surcharge (if passed through)

The `processing_fee_amount` is reconciled from Stripe webhooks after the fact. See `stripe_membership_payment_components.go` — it reads from Stripe metadata or invoice line items labelled "processing fee".

### What Exists for Fee Display (But No Toggle)

The system can _record_ and _display_ processing fees, but there is **no setting to control whether fees are passed through to members or absorbed by the tenant**. Currently the fee is just whatever Stripe reports. See Section 6 for what needs building.

---

## 3. How the Signup Pipeline Works (COMPLETED)

This is the most complex subsystem. The signup handler (`signup.go`) is ~3200 lines.

### Public Signup Flow

```
Step  What happens                                          Where
─────────────────────────────────────────────────────────────────────────────
1     Member browses tenant's memberships                   GET /api/signup/{tenantSlug}
2     Member views a specific membership + pricing          GET /api/signup/{tenantSlug}/{membershipSlug}
3     Member selects term option + payment frequency        Frontend: MembershipDetail.tsx
4     Member fills personal details                         Frontend: AgreementForm.tsx
5     Member accepts T&Cs (checkbox per attached T&C)       Frontend: AgreementForm.tsx
6     Member signs via signature pad                        Frontend: AgreementForm.tsx
7     Submit                                                POST /api/signup/{tenantSlug}/submit
8     Server creates/updates client record
9     Server creates membership record (status: pending)
10    Server validates T&C acceptance (server-side check)
11    Server creates agreement snapshot (HTML + PDF)
12    Server routes to payment provider:
        a. Card → Stripe Checkout Session → redirect to hosted Stripe page
        b. Direct Debit (new) → Stripe BECS Checkout → redirect
        c. Direct Debit (existing GoCardless mandate) → GC subscription created directly
13    Stripe/GC webhook confirms payment → membership status set to active
14    Confirmation email sent (optionally with PDF agreement attachment)
```

### Staff Invite Flow

Staff can pre-fill a membership for an existing client and send them a link to complete it:

```
1. Staff creates invite:  POST /api/signup-invites
   - Locks in: membership type, term option, start date, payment frequency
   - Strategy: "ask_client" (member picks payment method) or "use_existing" (reuse saved card/mandate)
2. Email sent to client with secure link: /join/{slug}/invite/{token}
3. Client opens link, sees pre-filled membership details
4. Client fills remaining details (personal info, signature)
5. Payment processed (same paths as public signup)
6. Invite marked completed
```

### Signup Link Security

Signup links are HMAC-signed (`?exp=TIMESTAMP&sig=HMAC_HEX`). The signing secret is `SIGNUP_LINK_SIGNING_SECRET` (falls back to `MASTER_ENCRYPTION_KEY`). Enforcement is optional via `SIGNUP_LINKS_REQUIRE_SIGNATURE` config flag. Links **do not expire**: the `exp` value is kept in the URL so the HMAC payload stays stable (and so previously issued links remain valid), but the server no longer rejects links based on `exp`. Newly generated links use `exp=0` as a "never expires" sentinel.

### Pricing Resolution

The system has two generations of pricing data:

**v1 (legacy):** Fee columns directly on `membership_term_options` table:
- `fee_total`, `fee_weekly`, `fee_fortnightly`, `fee_monthly`, `fee_quarterly`
- `num_payments`, `allow_upfront`, `custom_schedule_*`

**v2 (current):** Separate `term_option_payment_schedules` table with rows per payment plan:
- Each row has `kind` (upfront/recurring/instalments), `amount`, `interval_unit`, `interval_count`, `num_payments`
- Signup can accept a `payment_schedule_id` to pick a specific plan
- `derivePricingFromSchedule()` resolves canonical pricing from the schedule row

Both are supported. The frontend adapter (`termOptionAdapter.ts`) normalises both formats into a unified display model. The backend (`deriveCanonicalMembershipPricing`) handles both paths.

### Frontend Components

```
frontend/src/components/signup/
  SignupPage.tsx           — main orchestrator (step state machine)
  MembershipSelect.tsx     — membership type cards
  MembershipDetail.tsx     — term option + frequency selection + pricing display
  AgreementForm.tsx        — client details form + T&C checkboxes + signature pad
  ThankYouPage.tsx         — post-signup confirmation page
  StaffSignupLauncher.tsx  — staff-side UI for creating invites
```

---

## 4. Data Model

### Core Product Tables

| Table | Purpose |
|-------|---------|
| `tenants` | Organisations using Clientroll. Has timezone, tax settings, branding, signup config. |
| `clients` | Members/customers of a tenant. Personal details, addresses, Stripe/GC customer IDs. |
| `membership_types` | Membership products (e.g. "Gold Membership"). Has pricing, branding, T&C attachments. |
| `membership_term_options` | Duration/pricing options per membership type (e.g. "12 months"). |
| `term_option_payment_schedules` | v2 payment plans per term option (upfront/recurring/instalments). |
| `memberships` | Signed membership agreements. Links client → type → term → payment details. |
| `payments` | Individual payment records. Tracks base/tax/processing_fee breakdown, status, provider reference. |
| `service_charges` | Ad-hoc one-off charges (not tied to memberships). |
| `agreement_snapshots` | Signed agreement HTML + generated PDF. Legal custody record. |
| `terms_and_conditions` | Versioned T&C documents per tenant. |
| `term_option_terms` | Junction: which T&Cs attach to which term options. |
| `bonuses` | Signup bonuses attachable to term options (e.g. "free towel"). |

### Tenant Configuration Tables

| Table | Purpose |
|-------|---------|
| `agreement_settings` | Business details for agreements (company name, ABN, address, phone). |
| `tenant_credentials` | Encrypted API keys per payment provider (Stripe, GoCardless) per tenant. |
| `bank_details` | Tenant bank details shown on direct debit forms. |
| `tenant_email_templates` | Customisable email templates (confirmation, invite, etc.). |

### Owner Billing Tables (SEPARATE from product — see Section 5)

These tables are for us billing tenants. They have nothing to do with tenant→member payments.

| Table | Purpose |
|-------|---------|
| `platform_billing_config` | Single-row global config: price per member, our Stripe keys. |
| `tenant_subscriptions` | Per-tenant: our Stripe subscription for them, status, member count, grace period. |
| `tenant_usage_snapshots` | Daily snapshots of active member counts per tenant (for our billing). |
| `tenant_invoices` | Our Stripe invoice records for billing tenants. |

### Auth Tables

| Table | Purpose |
|-------|---------|
| `users` | Staff users who manage a tenant's dashboard. |
| `platform_admins` | Owner-level admin users (can access platform billing, create tenants). |

---

## 5. OWNER BILLING: Us Charging Tenants For Using The Platform (NEEDS FINISHING)

**This is completely separate from the product (Section 2).** This has nothing to do with how tenants charge their members. This is how Clientroll (us) gets paid.

**Business model:** $1 per active member per month. We count each tenant's active members daily, report the quantity to Stripe, and Stripe invoices the tenant automatically.

This uses a completely separate Stripe account with its own API keys stored in the `platform_billing_config` table — not in `tenant_credentials`. Separate webhook endpoint (`/api/webhooks/stripe/platform-billing`), separate database tables, separate code in `internal/billing/`.

### What's Already Built

**Backend:**
- `internal/billing/provider.go` — `PlatformBillingProvider` interface: CreateSubscription, ReportUsage, GetSubscriptionStatus, CancelSubscription, CreatePortalSession
- `internal/billing/stripe.go` — full Stripe implementation: creates customer + subscription, updates quantity (member count), cancels, opens billing portal
- `internal/billing/usage.go` — `SnapshotAllTenants()`: queries all tenants, counts active memberships, writes daily snapshots to `tenant_usage_snapshots`, updates `tenant_subscriptions`
- `internal/billing/mock.go` — mock provider for development
- `cmd/billing-snapshot/main.go` — standalone binary (intended for daily cron): loads config, runs snapshot, reports usage to Stripe for all tenants with active subscriptions
- `internal/handlers/platform_billing.go` — admin API endpoints:
  - `GET /api/platform/billing/config` — get billing config
  - `PUT /api/platform/billing/config` — update billing config (price, Stripe keys, product/price IDs)
  - `GET /api/platform/billing/tenants` — list all tenants with subscription status
  - `GET /api/platform/billing/tenants/{id}` — detail view (subscription + 90-day usage + invoices)
  - `POST /api/platform/billing/tenants/{id}/portal` — open Stripe billing portal for a tenant
- `internal/handlers/platform_billing_webhooks.go` — handles Stripe webhooks at `POST /api/webhooks/stripe/platform-billing`:
  - Subscription events → upsert `tenant_subscriptions` (status, quantity, period, grace period)
  - Invoice events → upsert `tenant_invoices` (amount, period, status, PDF URL)
  - Payment method events → update card details on subscription record
  - 7-day grace period logic when subscription goes `past_due`
- `internal/repository/platform_billing_config_repo.go` — config CRUD (auto-creates with $2.00 default)
- `internal/repository/tenant_subscription_repo.go` — subscription record CRUD
- `internal/repository/tenant_usage_repo.go` — usage snapshot queries
- `internal/repository/tenant_invoice_repo.go` — invoice record upserts

**Frontend:**
- `frontend/src/components/platform/PlatformBillingPage.tsx` — full admin UI:
  - Configure per-member price, Stripe product/price IDs
  - Enter/update Stripe keys (publishable, secret, webhook signing)
  - Toggle billing active/inactive
  - Tenant table showing: name, subscription status, member count, monthly amount, card details
  - Open Stripe billing portal per tenant
  - Shows webhook URL and required events list
- `frontend/src/components/platform/PlatformTenantBillingDetailPage.tsx` — per-tenant detail:
  - Subscription status, member count, monthly amount
  - Payment method on file (card brand/last4/expiry)
  - Stripe customer/subscription IDs
  - Usage snapshots table (last 90 days)
  - Invoice table with links to Stripe hosted invoice + PDF

### What Needs to Be Done

#### 5.1 Change Price to $1/member (EASY)

The default in `platform_billing_config_repo.go` line 91 is `2.00`. Change to `1.00`. Also need to create/update the Stripe Price object in the owner's Stripe dashboard to match $1/unit/month. The `stripe_price_id` in the config must point to this price.

Files to touch:
- `internal/repository/platform_billing_config_repo.go` — default value
- Stripe dashboard — create Price object

#### 5.2 Deploy the Daily Cron Job (INFRA)

`cmd/billing-snapshot/main.go` is a standalone binary. It needs:
- A daily schedule (systemd timer, CloudWatch scheduled event, Kubernetes CronJob, or similar)
- Access to `DATABASE_URL` and `MASTER_ENCRYPTION_KEY` env vars
- Runs in ~30 seconds, safe to retry

What it does each run:
1. Loads `platform_billing_config` — exits if billing is inactive
2. Queries all tenants + counts active memberships
3. Writes snapshots to `tenant_usage_snapshots`
4. For each tenant with a Stripe subscription: calls `subscription.Update()` to set quantity = active member count
5. Stripe automatically invoices based on the updated quantity at the end of each billing period

#### 5.3 Tenant Onboarding to Billing (NOT BUILT)

Currently there is NO automated flow to create a Stripe subscription when a new tenant is created. The `billing.StripeBillingProvider.CreateSubscription()` method exists and works, but nothing calls it automatically.

Stripe handles all invoicing, payment collection, and retries. We just need to create the subscription and keep the quantity (active member count) up to date — the daily cron already does the quantity updates.

**What needs building:**
- When a tenant is created/activated, automatically:
  1. Create a Stripe customer (using tenant admin's email + tenant name)
  2. Create a subscription with the configured `stripe_price_id`, quantity=0
  3. Store the Stripe customer ID and subscription ID in `tenant_subscriptions`
- The daily cron (`cmd/billing-snapshot`) will then update quantity each day
- Stripe invoices the tenant automatically at the end of each billing period based on quantity

Files to touch:
- `internal/handlers/platform_billing.go` or `tenants.go` — trigger subscription creation when tenant is activated
- `cmd/server/main.go` — wire up if new handler
- `internal/billing/stripe.go` — `CreateSubscription()` already works, just needs to be called

#### 5.4 Grace Period Enforcement (NOT BUILT)

The `tenant_subscriptions` table has `past_due_since` and `billing_grace_until` columns. The webhook handler already populates them (7-day grace when subscription goes `past_due`). But **nothing enforces it**.

**What needs building:**
- Middleware that checks `tenant_subscriptions` on authenticated tenant API requests
- If `billing_grace_until` is set and `NOW() > billing_grace_until`: return 402/403 with a message like "Your billing is overdue. Please update your payment method."
- The Stripe billing portal link (`CreatePortalSession`) should be included in the error response so the tenant can fix it
- Frontend: show a banner when billing is past due, with a link to the billing portal
- Consider: what level of restriction? Block all access? Read-only? Only block new signups?

Files to touch:
- `internal/middleware/` — new billing check middleware
- `cmd/server/main.go` — apply middleware to tenant routes
- Frontend — billing warning banner component

#### 5.5 Tenant-Facing Billing View (NOT BUILT)

Currently only platform admins can see billing. Tenants cannot see their own usage, invoices, or subscription status.

**What needs building:**
- Tenant API endpoint: `GET /api/tenant/billing` — returns their own subscription, usage, invoices
- Frontend page under tenant settings showing:
  - Current plan status + member count
  - Monthly cost
  - Payment method on file
  - Link to Stripe billing portal (to update payment method)
  - Invoice history
  - Usage chart (member count over time)

Files to touch:
- `internal/handlers/` — new tenant billing handler
- `cmd/server/main.go` — wire routes
- `frontend/src/components/settings/` — new billing tab/page

---

## 6. Processing Fee Toggle (NOT BUILT)

Tenants need the ability to either absorb payment processing fees or pass them through to members as a surcharge. Currently there is no toggle — fees are just recorded after the fact from webhook reconciliation.

### What Needs Building

#### 6.1 Database Schema

Add a fee mode setting. Recommended approach — tenant-level default + per-membership override:

```sql
-- Migration: add processing fee mode
ALTER TABLE tenants
  ADD COLUMN processing_fee_mode VARCHAR(20) NOT NULL DEFAULT 'absorb';
  -- Values: 'absorb' (tenant pays fee) | 'pass_through' (member pays fee)

ALTER TABLE membership_types
  ADD COLUMN processing_fee_mode VARCHAR(20) DEFAULT NULL;
  -- NULL = use tenant default, 'absorb' or 'pass_through' = override
```

#### 6.2 Backend — Resolve Fee Mode

Add a helper that resolves the effective fee mode for a given membership type:

```
effectiveFeeMode(tenant, membershipType) → 'absorb' | 'pass_through'
  if membershipType.processing_fee_mode != NULL → return it
  else → return tenant.processing_fee_mode
```

#### 6.3 Backend — Signup/Checkout Changes

In `signup.go`, when creating Stripe Checkout Sessions:

**If fee mode = `pass_through`:**
- Calculate estimated Stripe fee based on payment amount and card type
  - Australian domestic cards: 1.75% + $0.30
  - International cards: 2.9% + $0.30
  - BECS Direct Debit: 1% + $0.30 (capped at $3.50)
  - Note: exact fee depends on Stripe pricing for the tenant's account
- Add a separate line item "Payment processing fee" to the Checkout Session
- Store `base_amount`, `tax_amount`, `processing_fee_amount` in session metadata
- The reconciliation in `stripe_membership_payment_components.go` already reads these values

**If fee mode = `absorb`:**
- Current behaviour — single line item, no surcharge
- `processing_fee_amount` stays 0 in payment records

**GoCardless path:**
- Same logic but fee calculation differs
- GoCardless subscriptions: add fee to the subscription amount, or create a separate charge

#### 6.4 Frontend — Settings UI

Add toggle in tenant settings (`SettingsPage.tsx` or a new payments settings section):
- Global default: "Pass processing fees to members" toggle
- Per membership type: override dropdown on membership type edit form (Inherit / Absorb / Pass through)

#### 6.5 Frontend — Signup Display

When fee mode is `pass_through`, the signup page (`MembershipDetail.tsx`) should show:
- Membership fee: $X
- Processing fee: ~$Y
- Total charged: $Z

The `paymentLabels.ts` utility already has `processingSurcharge()` and `showBreakdown()` functions that handle display — they just need data to work with.

#### 6.6 Reports

Already handled. `reports_repo.go` has surcharge expressions:
- `reportSurchargeMembershipExpr = "COALESCE(processing_fee_amount, 0)"`
- `reportCollectedServiceExpr = "amount + tax_amount + processing_fee_amount"`

These will work correctly once `processing_fee_amount` is populated accurately.

---

## 7. Other Platform Features (Reference)

These are all working — listed here so you understand the full system:

### Membership Types & Term Options
- CRUD for membership types with branding, descriptions, inclusions
- Each type has term options (e.g. "3 months", "12 months")
- Each term option has payment schedules (upfront, weekly, monthly, etc.)
- Payment schedules are the v2 pricing model (see Section 3)

### Terms & Conditions
- Versioned T&C documents per tenant
- Attach T&Cs to specific term options
- Server-side variable substitution (company name, ABN, address)
- Members must accept all attached T&Cs during signup (enforced server-side)
- T&C reacceptance: can force existing members to re-accept updated terms

### Agreement Snapshots
- When a member signs up, an HTML snapshot of the agreement is created
- Includes: client details, membership details, pricing, T&C text, signatures
- PDF generated from HTML for email attachment and download
- Legally binding record stored in `agreement_snapshots`

### Service Charges
- Ad-hoc one-off charges not tied to memberships
- Stripe checkout sessions for payment
- Same base/tax/fee breakdown as membership payments

### Reports
- Financial reporting: collected revenue, surcharges, GST breakdown
- Per-membership and per-service report views
- Monthly comparison views

### Email Templates
- Customisable per-tenant email templates
- Template keys: `membership_confirmation`, `membership_agreement`, etc.
- Variable substitution for client name, membership title, amounts

### Outbound Webhooks
- Tenants can configure a webhook URL for signup completion events
- HMAC-signed payloads with the full membership/client data
- Configurable per tenant

---

## 8. Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `MASTER_ENCRYPTION_KEY` | Yes | AES key for encrypting all secrets at rest |
| `APP_URL` | Yes | Public URL (e.g. `https://app.clientroll.com`) |
| `PORT` | No | HTTP port (default varies) |
| `SIGNUP_LINK_SIGNING_SECRET` | No | HMAC secret for signup links (falls back to master key) |
| `SIGNUP_LINKS_REQUIRE_SIGNATURE` | No | If true, all signup links must be signed |
| `SIGNUP_LINK_TTL_MINUTES` | No | Deprecated — signup links no longer expire; value is ignored for expiry enforcement |

The billing snapshot cron (`cmd/billing-snapshot`) needs `DATABASE_URL` and `MASTER_ENCRYPTION_KEY`.

---

## 9. Running Locally

```bash
# Database
docker-compose -f docker-compose-dev.yml up -d postgres

# Migrations
# Uses golang-migrate. Migrations are in /migrations/ (numbered 001-077).

# Backend
cd cmd/server && go run .

# Frontend
cd frontend && npm install && npm run dev

# Billing snapshot (manual test)
cd cmd/billing-snapshot && go run .
```

---

## 10. Summary: What The Developer Needs To Do

In priority order:

| # | Task | Section | Difficulty |
|---|------|---------|-----------|
| 1 | Change owner billing price from $2 → $1 | 5.1 | Easy |
| 2 | Deploy billing-snapshot as daily cron | 5.2 | Infra |
| 3 | Auto-create Stripe subscription when tenant is activated | 5.3 | Easy-Medium |
| 4 | Build grace period enforcement middleware | 5.4 | Medium |
| 5 | Build tenant-facing billing page | 5.5 | Medium |
| 6 | Build processing fee toggle (DB + backend + frontend) | 6 | Large |

Items 1-2 are prerequisite to actually charging tenants. Items 3-5 complete the billing lifecycle. Item 6 is a separate feature that can be built in parallel.
