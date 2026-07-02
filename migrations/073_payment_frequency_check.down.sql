-- Restore the original (overly restrictive) chk_payment_frequency.
-- Note: rows with payment_frequency in ('quarterly','custom_schedule') will block
-- this rollback. Operator should review/repoint such rows before running down.

ALTER TABLE memberships DROP CONSTRAINT IF EXISTS chk_payment_frequency;
ALTER TABLE memberships ADD CONSTRAINT chk_payment_frequency CHECK (
    payment_frequency IN ('weekly', 'fortnightly', 'monthly', 'upfront')
);
