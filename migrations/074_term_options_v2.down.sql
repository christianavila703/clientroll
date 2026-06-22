-- Reverse the term-options redesign. Only drops the new schema; existing
-- fee_* / custom_schedule_* columns on membership_term_options are untouched
-- (this migration never wrote to them). Data still in the wire-down state
-- remains intact for the legacy code paths.

-- Runner wraps each migration in its own transaction (cmd/migrate/main.go).

-- Drop memberships.payment_schedule_id (must come before dropping the
-- referenced payment-schedules table).
ALTER TABLE memberships DROP COLUMN IF EXISTS payment_schedule_id;

DROP TABLE IF EXISTS migration_unresolved_memberships;
DROP TABLE IF EXISTS term_option_terms;
DROP TABLE IF EXISTS term_option_payment_schedules;

ALTER TABLE membership_term_options
    DROP CONSTRAINT IF EXISTS chk_term_option_duration_unit,
    DROP CONSTRAINT IF EXISTS chk_term_option_duration_value;

ALTER TABLE membership_term_options
    DROP COLUMN IF EXISTS duration_value,
    DROP COLUMN IF EXISTS duration_unit,
    DROP COLUMN IF EXISTS label,
    DROP COLUMN IF EXISTS sort_order,
    DROP COLUMN IF EXISTS archived_at;
