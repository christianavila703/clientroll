-- Dated instalment lines: uneven split-payment plans for a term option
-- (e.g. $5,000 on signup, then $8,500 fourteen days later).
--
-- A schedule row of kind='instalments' may now carry explicit lines in
-- term_option_schedule_instalments instead of an interval + num_payments.
-- The parent row's `amount` is the plan total (sum of its lines).

CREATE TABLE IF NOT EXISTS term_option_schedule_instalments (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    schedule_id UUID NOT NULL REFERENCES term_option_payment_schedules(id) ON DELETE CASCADE,
    amount      NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    -- Days after the membership start date this instalment is charged.
    -- The first line of a plan must be 0 (charged at signup).
    offset_days INT NOT NULL CHECK (offset_days >= 0),
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tosi_schedule ON term_option_schedule_instalments(schedule_id);
CREATE INDEX IF NOT EXISTS idx_tosi_tenant   ON term_option_schedule_instalments(tenant_id);

ALTER TABLE term_option_schedule_instalments ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation ON term_option_schedule_instalments
        USING (tenant_id = current_tenant_id());
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Relax the shape constraint: instalments rows may omit the interval when
-- their cadence is expressed as explicit dated lines.
ALTER TABLE term_option_payment_schedules
    DROP CONSTRAINT IF EXISTS chk_payment_schedule_shape;
ALTER TABLE term_option_payment_schedules
    ADD CONSTRAINT chk_payment_schedule_shape CHECK (
        (kind = 'upfront'
            AND interval_unit IS NULL
            AND interval_count IS NULL)
        OR
        (kind = 'recurring'
            AND interval_unit IS NOT NULL
            AND interval_count IS NOT NULL)
        OR
        (kind = 'instalments'
            AND ((interval_unit IS NOT NULL AND interval_count IS NOT NULL)
                 OR (interval_unit IS NULL AND interval_count IS NULL)))
    );
