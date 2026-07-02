-- Replace the chk_payment_frequency CHECK constraint on memberships so it
-- matches every payment frequency the rest of the stack already supports.
-- Migration 005 originally allowed only: weekly, fortnightly, monthly, upfront.
-- Handlers and signup flow added 'quarterly' and 'custom_schedule' over time;
-- without this drop+recreate, INSERTs from those paths fail at the DB layer.

ALTER TABLE memberships DROP CONSTRAINT IF EXISTS chk_payment_frequency;
ALTER TABLE memberships ADD CONSTRAINT chk_payment_frequency CHECK (
    payment_frequency IS NULL
    OR payment_frequency IN ('weekly', 'fortnightly', 'monthly', 'quarterly', 'upfront', 'custom_schedule')
);
