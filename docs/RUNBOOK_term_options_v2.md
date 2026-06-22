# Runbook: Term Options v2 migration (074_term_options_v2)

Migration `074_term_options_v2` introduces a real payment-schedule model for
membership term options. The migration is additive — every existing column on
`membership_term_options` is preserved — but it adds new tables and a new FK
column on `memberships`, so a careful rollout is required.

This runbook is to be followed for the production cut-over.

## 1. Pre-flight backup

The migration writes to:

- `membership_term_options` (adds columns + constraints)
- `term_option_payment_schedules` (new table, populated by backfill)
- `term_option_terms` (new table, populated by backfill)
- `memberships` (adds `payment_schedule_id` + backfills it)
- `migration_unresolved_memberships` (audit table)

Take an explicit per-table backup before applying:

```bash
TS=$(date -u +%Y%m%dT%H%M%SZ)
pg_dump "$DATABASE_URL" \
    --data-only \
    --no-owner --no-privileges \
    -t membership_term_options \
    -t membership_type_terms \
    -t memberships \
    -t membership_setup_invites \
    > "term_options_v2_backup_${TS}.sql"
```

Keep that dump in cold storage (S3 / glacier) for at least 30 days post-cutover.

## 2. Dry-run with the verifier

Compile and run the verifier against a snapshot of production (recommended:
restore a recent backup to a side database; do NOT run against production
directly, even though it rolls the tx back, as long-running transactions can
bloat WAL):

```bash
DATABASE_URL="postgres://...sideDB..." \
    go run ./cmd/verify-term-options -out unresolved.csv
```

Expected output (numbers obviously vary):

```text
Pre-migration: 84 MTOs, 612 memberships, 47 (mt → tc) attachments
Post-migration (dry-run):
  payment schedules created : 187  (upfront=70, recurring=110, instalments=7)
  is_default rows           : 84 / 84 MTOs
  term_option_terms rows    : 240
  memberships resolved      : 608 / 612
  unresolved memberships    : 4 (review before applying)
```

Sign-off criteria:

1. `payment schedules created` ≥ `is_default rows`, and `is_default rows` ==
   the count of MTOs that have at least one fee or `allow_upfront=TRUE`.
2. `term_option_terms rows` ≥ `mt → tc attachments` (the migration fans out
   each junction row to every term option, so the count is normally higher).
3. `unresolved memberships` is either zero, or each row in `unresolved.csv`
   is reviewed and accepted.

For each entry in `unresolved.csv` decide whether to:

- Fix the source data manually before applying (recommended), or
- Apply the migration and back-patch the `payment_schedule_id` afterwards.

## 3. Apply the migration

Inside a low-traffic window:

```bash
DATABASE_URL=... go run ./cmd/migrate up
```

After apply, sanity-check:

```sql
SELECT kind, COUNT(*) FROM term_option_payment_schedules GROUP BY kind;
SELECT COUNT(*) FROM memberships WHERE payment_schedule_id IS NULL;
SELECT * FROM migration_unresolved_memberships ORDER BY created_at;
```

## 4. Rollback procedure

The down migration only drops the new tables and columns. It does NOT touch
the legacy `fee_*` / `custom_schedule_*` columns on `membership_term_options`,
so a rollback is non-destructive *as long as* the application code still
reads the legacy columns.

To roll back:

```bash
DATABASE_URL=... go run ./cmd/migrate down
```

If the application code has already been redeployed to read the new schema,
*do not* run `migrate down` blindly — first downgrade the application binary
so it points at the legacy reads again.

## 5. Soak window

After the migration, run for at least one full billing cycle (default: 7 days)
before scheduling any cleanup migration that drops the legacy
`fee_*` / `custom_schedule_*` columns. Once the soak passes, a separate
migration (75 or later) can drop them; until then they remain in place for
emergency parity reads.

## 6. Backfill unresolved memberships

If `migration_unresolved_memberships` has rows, treat each one as a manual
operation:

```sql
-- Inspect
SELECT * FROM migration_unresolved_memberships;

-- Example fix (after determining the right schedule):
UPDATE memberships
SET    payment_schedule_id = '<chosen-id>'
WHERE  id = '<membership-id>';

-- Once resolved, remove the audit row:
DELETE FROM migration_unresolved_memberships
WHERE  membership_id = '<membership-id>';
```

The audit table is kept around indefinitely so support can answer questions
months later ("why does this membership not have a schedule?"); it's tiny.

## 7. Week-based duration heuristic

The migration sets `duration_unit = 'month'` for every existing row. Term
options that were created as weeks (e.g. a 6-week intro pack) need to be
re-flagged manually in the new admin UI after cutover — see the plan note
`phase1_weeks_heuristic`. Their `duration_months` value remains correct as a
fallback for any reads that still use it.
