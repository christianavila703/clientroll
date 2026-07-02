-- Add quarterly payment option and fixed number of payments support.
-- num_payments: if set, payments stop after this many (membership stays active until end).
-- If NULL, payments continue on the chosen frequency until the term ends.
ALTER TABLE membership_term_options ADD COLUMN IF NOT EXISTS fee_quarterly NUMERIC(12,2) DEFAULT NULL;
ALTER TABLE membership_term_options ADD COLUMN IF NOT EXISTS num_payments INT DEFAULT NULL;

-- Also add to memberships for the chosen schedule at signup time
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS num_payments INT DEFAULT NULL;
ALTER TABLE memberships ADD COLUMN IF NOT EXISTS payments_made INT DEFAULT 0;
