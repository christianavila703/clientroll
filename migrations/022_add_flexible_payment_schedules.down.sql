ALTER TABLE memberships
  DROP COLUMN IF EXISTS payment_interval_count,
  DROP COLUMN IF EXISTS payment_interval_unit;

ALTER TABLE membership_term_options
  DROP COLUMN IF EXISTS custom_schedule_num_payments,
  DROP COLUMN IF EXISTS custom_schedule_interval_count,
  DROP COLUMN IF EXISTS custom_schedule_interval_unit,
  DROP COLUMN IF EXISTS custom_schedule_amount,
  DROP COLUMN IF EXISTS custom_schedule_label,
  DROP COLUMN IF EXISTS custom_schedule_enabled;

