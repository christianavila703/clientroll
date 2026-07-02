-- Add flexible/custom schedule support per term option.
ALTER TABLE membership_term_options
  ADD COLUMN IF NOT EXISTS custom_schedule_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS custom_schedule_label TEXT,
  ADD COLUMN IF NOT EXISTS custom_schedule_amount NUMERIC(12,2),
  ADD COLUMN IF NOT EXISTS custom_schedule_interval_unit TEXT,
  ADD COLUMN IF NOT EXISTS custom_schedule_interval_count INT,
  ADD COLUMN IF NOT EXISTS custom_schedule_num_payments INT;

-- Store the chosen interval details on memberships.
ALTER TABLE memberships
  ADD COLUMN IF NOT EXISTS payment_interval_unit TEXT,
  ADD COLUMN IF NOT EXISTS payment_interval_count INT NOT NULL DEFAULT 1;

