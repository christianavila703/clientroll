-- Revert dated instalment lines.
-- NOTE: restoring the strict shape constraint fails if any kind='instalments'
-- row without an interval still exists; delete those schedules first.

DROP TABLE IF EXISTS term_option_schedule_instalments;

ALTER TABLE term_option_payment_schedules
    DROP CONSTRAINT IF EXISTS chk_payment_schedule_shape;
ALTER TABLE term_option_payment_schedules
    ADD CONSTRAINT chk_payment_schedule_shape CHECK (
        (kind = 'upfront'
            AND interval_unit IS NULL
            AND interval_count IS NULL)
        OR
        (kind IN ('recurring', 'instalments')
            AND interval_unit IS NOT NULL
            AND interval_count IS NOT NULL)
    );
