ALTER TABLE membership_term_options DROP COLUMN IF EXISTS fee_quarterly;
ALTER TABLE membership_term_options DROP COLUMN IF EXISTS num_payments;
ALTER TABLE memberships DROP COLUMN IF EXISTS num_payments;
ALTER TABLE memberships DROP COLUMN IF EXISTS payments_made;
