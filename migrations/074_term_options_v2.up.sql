-- Phase 1: Term-options redesign.
-- Splits the conflated `membership_term_options` row into:
--   * duration metadata on `membership_term_options` (duration_value + duration_unit)
--   * `term_option_payment_schedules` rows for each way the term can be paid
--   * `term_option_terms` rows for per-term T&C attachment
-- Existing fee_* / custom_schedule_* columns are LEFT IN PLACE for safety
-- so a down-migration (or a partial revert) can still read the old shape.
-- A separate cleanup migration removes them after a successful soak.

-- Runner wraps each migration in its own transaction (cmd/migrate/main.go).

-- 1. Extend membership_term_options with explicit duration unit + display fields.
ALTER TABLE membership_term_options
    ADD COLUMN IF NOT EXISTS duration_value INT,
    ADD COLUMN IF NOT EXISTS duration_unit  TEXT NOT NULL DEFAULT 'month',
    ADD COLUMN IF NOT EXISTS label          TEXT,
    ADD COLUMN IF NOT EXISTS sort_order     INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS archived_at    TIMESTAMPTZ;

-- Backfill duration_value from duration_months. Heuristic: every existing row
-- is treated as month-based; week-based plans get re-flagged by the admin in
-- the new UI (this is intentional — see plan `phase1_weeks_heuristic`).
UPDATE membership_term_options
SET duration_value = duration_months
WHERE duration_value IS NULL;

ALTER TABLE membership_term_options
    ALTER COLUMN duration_value SET NOT NULL;

DO $$ BEGIN
    ALTER TABLE membership_term_options
        ADD CONSTRAINT chk_term_option_duration_unit
        CHECK (duration_unit IN ('week', 'month'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    ALTER TABLE membership_term_options
        ADD CONSTRAINT chk_term_option_duration_value
        CHECK (duration_value > 0);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;


-- 2. Payment schedule rows. One MTO can have many: upfront + weekly + custom etc.
CREATE TABLE IF NOT EXISTS term_option_payment_schedules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    term_option_id  UUID NOT NULL REFERENCES membership_term_options(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('upfront', 'recurring', 'instalments')),
    label           TEXT,
    amount          NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
    interval_unit   TEXT          CHECK (interval_unit IS NULL OR interval_unit IN ('week', 'month')),
    interval_count  INT           CHECK (interval_count IS NULL OR interval_count > 0),
    num_payments    INT           CHECK (num_payments  IS NULL OR num_payments  > 0),
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order      INT NOT NULL DEFAULT 0,
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_payment_schedule_shape CHECK (
        (kind = 'upfront'
            AND interval_unit IS NULL
            AND interval_count IS NULL)
        OR
        (kind IN ('recurring', 'instalments')
            AND interval_unit IS NOT NULL
            AND interval_count IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_tops_tenant ON term_option_payment_schedules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tops_term   ON term_option_payment_schedules(term_option_id);
-- Only one default schedule per term option (NULLs/archived don't count).
CREATE UNIQUE INDEX IF NOT EXISTS uq_tops_one_default_per_term
    ON term_option_payment_schedules(term_option_id)
    WHERE is_default = TRUE AND archived_at IS NULL;

ALTER TABLE term_option_payment_schedules ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation ON term_option_payment_schedules
        USING (tenant_id = current_tenant_id());
EXCEPTION WHEN duplicate_object THEN NULL; END $$;


-- 3. Backfill payment schedules from the conflated columns.
-- One INSERT per "kind" so the source of each row is obvious in code review.

-- Upfront: always create when allow_upfront=true so $0 free-memberships migrate.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, amount, interval_unit, interval_count, num_payments, is_default, sort_order)
SELECT tenant_id, id, 'upfront', COALESCE(fee_total, 0), NULL, NULL, 1, TRUE, 0
FROM membership_term_options
WHERE allow_upfront = TRUE
  AND archived_at IS NULL;

-- Weekly recurring.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, amount, interval_unit, interval_count, num_payments, sort_order)
SELECT
    tenant_id, id, 'recurring', fee_weekly, 'week', 1,
    CASE WHEN continue_after_term THEN NULL
         ELSE GREATEST(1, ROUND((duration_months * 52.0) / 12.0)::INT) END,
    10
FROM membership_term_options
WHERE fee_weekly IS NOT NULL AND fee_weekly > 0 AND archived_at IS NULL;

-- Fortnightly recurring.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, amount, interval_unit, interval_count, num_payments, sort_order)
SELECT
    tenant_id, id, 'recurring', fee_fortnightly, 'week', 2,
    CASE WHEN continue_after_term THEN NULL
         ELSE GREATEST(1, ROUND((duration_months * 26.0) / 12.0)::INT) END,
    20
FROM membership_term_options
WHERE fee_fortnightly IS NOT NULL AND fee_fortnightly > 0 AND archived_at IS NULL;

-- Monthly recurring.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, amount, interval_unit, interval_count, num_payments, sort_order)
SELECT
    tenant_id, id, 'recurring', fee_monthly, 'month', 1,
    CASE WHEN continue_after_term THEN NULL
         ELSE GREATEST(1, duration_months) END,
    30
FROM membership_term_options
WHERE fee_monthly IS NOT NULL AND fee_monthly > 0 AND archived_at IS NULL;

-- Quarterly recurring.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, amount, interval_unit, interval_count, num_payments, sort_order)
SELECT
    tenant_id, id, 'recurring', fee_quarterly, 'month', 3,
    CASE WHEN continue_after_term THEN NULL
         ELSE GREATEST(1, CEIL(duration_months / 3.0)::INT) END,
    40
FROM membership_term_options
WHERE fee_quarterly IS NOT NULL AND fee_quarterly > 0 AND archived_at IS NULL;

-- Custom schedule -> instalments.
INSERT INTO term_option_payment_schedules
    (tenant_id, term_option_id, kind, label, amount, interval_unit, interval_count, num_payments, sort_order)
SELECT
    tenant_id, id, 'instalments', custom_schedule_label,
    custom_schedule_amount, custom_schedule_interval_unit,
    custom_schedule_interval_count, custom_schedule_num_payments, 50
FROM membership_term_options
WHERE custom_schedule_enabled = TRUE
  AND custom_schedule_amount IS NOT NULL
  AND custom_schedule_amount > 0
  AND custom_schedule_interval_unit IS NOT NULL
  AND custom_schedule_interval_count IS NOT NULL
  AND archived_at IS NULL;

-- For term options without an upfront row, promote the lowest-sort_order
-- recurring row to is_default=TRUE.
WITH wo_default AS (
    SELECT DISTINCT term_option_id
    FROM term_option_payment_schedules
    WHERE NOT EXISTS (
        SELECT 1 FROM term_option_payment_schedules d
        WHERE d.term_option_id = term_option_payment_schedules.term_option_id
          AND d.is_default = TRUE
    )
),
pick AS (
    SELECT DISTINCT ON (s.term_option_id) s.id
    FROM term_option_payment_schedules s
    JOIN wo_default w ON w.term_option_id = s.term_option_id
    ORDER BY s.term_option_id, s.sort_order, s.created_at
)
UPDATE term_option_payment_schedules s
SET is_default = TRUE
FROM pick p
WHERE s.id = p.id;


-- 4. Per-term T&C attachment table + backfill from membership_type_terms.
CREATE TABLE IF NOT EXISTS term_option_terms (
    tenant_id                UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    term_option_id           UUID NOT NULL REFERENCES membership_term_options(id) ON DELETE CASCADE,
    terms_and_conditions_id  UUID NOT NULL REFERENCES terms_and_conditions(id) ON DELETE CASCADE,
    is_required              BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order               INT NOT NULL DEFAULT 0,
    PRIMARY KEY (term_option_id, terms_and_conditions_id)
);
CREATE INDEX IF NOT EXISTS idx_tot_tenant ON term_option_terms(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tot_term   ON term_option_terms(term_option_id);
CREATE INDEX IF NOT EXISTS idx_tot_tc     ON term_option_terms(terms_and_conditions_id);

ALTER TABLE term_option_terms ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation ON term_option_terms
        USING (tenant_id = current_tenant_id());
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

INSERT INTO term_option_terms
    (tenant_id, term_option_id, terms_and_conditions_id, is_required, sort_order)
SELECT mto.tenant_id, mto.id, mtt.terms_and_conditions_id, TRUE, 0
FROM membership_type_terms mtt
JOIN membership_term_options mto
    ON  mto.membership_type_id = mtt.membership_type_id
    AND mto.tenant_id          = mtt.tenant_id
    AND mto.archived_at        IS NULL
ON CONFLICT (term_option_id, terms_and_conditions_id) DO NOTHING;


-- 5. Audit table — populated by the backfill below for any membership whose
-- payment_schedule_id cannot be resolved unambiguously. Operators can review
-- this table after the migration and fix the rows manually.
CREATE TABLE IF NOT EXISTS migration_unresolved_memberships (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id         UUID,
    membership_id     UUID,
    term_option_id    UUID,
    payment_frequency TEXT,
    payment_amount    NUMERIC(12,2),
    reason            TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_mu_mems_tenant ON migration_unresolved_memberships(tenant_id);


-- 6. memberships.payment_schedule_id + backfill.
ALTER TABLE memberships
    ADD COLUMN IF NOT EXISTS payment_schedule_id UUID REFERENCES term_option_payment_schedules(id);

CREATE INDEX IF NOT EXISTS idx_memberships_payment_schedule
    ON memberships(payment_schedule_id);

DO $$
DECLARE
    m         RECORD;
    matched   UUID;
BEGIN
    FOR m IN
        SELECT id, tenant_id, term_option_id,
               LOWER(COALESCE(payment_frequency, '')) AS freq,
               payment_amount,
               payment_interval_unit,
               payment_interval_count
        FROM memberships
        WHERE payment_schedule_id IS NULL
    LOOP
        matched := NULL;

        IF m.freq = 'upfront' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'upfront'
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;

        ELSIF m.freq = 'weekly' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'recurring' AND interval_unit = 'week' AND interval_count = 1
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;

        ELSIF m.freq = 'fortnightly' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'recurring' AND interval_unit = 'week' AND interval_count = 2
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;

        ELSIF m.freq = 'monthly' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'recurring' AND interval_unit = 'month' AND interval_count = 1
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;

        ELSIF m.freq = 'quarterly' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'recurring' AND interval_unit = 'month' AND interval_count = 3
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;

        ELSIF m.freq = 'custom_schedule' THEN
            SELECT id INTO matched FROM term_option_payment_schedules
            WHERE term_option_id = m.term_option_id
              AND kind = 'instalments'
              AND interval_unit  = m.payment_interval_unit
              AND interval_count = COALESCE(m.payment_interval_count, 1)
              AND ABS(amount - COALESCE(m.payment_amount, amount)) <= 0.01
            ORDER BY sort_order LIMIT 1;
        END IF;

        IF matched IS NOT NULL THEN
            UPDATE memberships SET payment_schedule_id = matched WHERE id = m.id;
        ELSE
            INSERT INTO migration_unresolved_memberships
                (tenant_id, membership_id, term_option_id,
                 payment_frequency, payment_amount, reason)
            VALUES (m.tenant_id, m.id, m.term_option_id,
                    m.freq, m.payment_amount,
                    'no matching schedule row for term_option_id + frequency + amount');
        END IF;
    END LOOP;
END
$$;
