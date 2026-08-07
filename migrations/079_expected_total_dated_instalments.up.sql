-- Dated split-payment plans store the CONTRACT TOTAL in payment_amount
-- (uneven instalments carry no cadence: payment_interval_unit is NULL,
-- num_payments = line count). The generic "amount × num_payments" rule
-- doubled those contracts (e.g. $11,500 plan shown as $23,000 expected).
--
-- A dated plan is identified as: frequency 'custom_schedule' with no
-- interval unit. Legacy custom_schedule memberships always carry an
-- interval unit, so they keep the multiplication.

CREATE OR REPLACE FUNCTION membership_expected_total(
    p_payment_amount DOUBLE PRECISION,
    p_num_payments INTEGER,
    p_payment_frequency TEXT,
    p_payment_interval_unit TEXT,
    p_payment_interval_count INTEGER,
    p_start_date DATE,
    p_end_date DATE,
    p_duration_months INTEGER
) RETURNS DOUBLE PRECISION
LANGUAGE plpgsql
AS $$
DECLARE
    amt DOUBLE PRECISION := COALESCE(p_payment_amount, 0);
    freq TEXT := LOWER(COALESCE(p_payment_frequency, ''));
    interval_unit TEXT := LOWER(COALESCE(p_payment_interval_unit, ''));
    interval_count INTEGER := GREATEST(COALESCE(p_payment_interval_count, 1), 1);
    days_span DOUBLE PRECISION := GREATEST(
        0,
        EXTRACT(EPOCH FROM (p_end_date::timestamp - p_start_date::timestamp)) / 86400.0
    );
    months_span DOUBLE PRECISION := GREATEST(
        0,
        (DATE_PART('year', AGE(p_end_date, p_start_date)) * 12) + DATE_PART('month', AGE(p_end_date, p_start_date))
    );
BEGIN
    -- Dated split-payment plan: payment_amount already is the contract total.
    IF freq = 'custom_schedule' AND interval_unit = '' THEN
        RETURN amt;
    END IF;

    IF p_num_payments IS NOT NULL AND p_num_payments > 0 THEN
        RETURN amt * p_num_payments;
    END IF;

    IF amt <= 0 THEN
        RETURN 0;
    END IF;

    CASE freq
        WHEN 'upfront' THEN
            RETURN amt;
        WHEN 'weekly' THEN
            RETURN amt * GREATEST(1, FLOOR(days_span / 7.0));
        WHEN 'fortnightly' THEN
            RETURN amt * GREATEST(1, FLOOR(days_span / 14.0));
        WHEN 'monthly' THEN
            RETURN amt * GREATEST(1, FLOOR(months_span / 1.0));
        WHEN 'quarterly' THEN
            RETURN amt * GREATEST(1, FLOOR(months_span / 3.0));
        WHEN 'custom_schedule' THEN
            IF interval_unit = 'week' THEN
                RETURN amt * GREATEST(1, FLOOR(days_span / (7.0 * interval_count)));
            ELSIF interval_unit = 'month' THEN
                RETURN amt * GREATEST(1, FLOOR(months_span / interval_count::DOUBLE PRECISION));
            END IF;
    END CASE;

    RETURN amt * GREATEST(1, COALESCE(p_duration_months, 1));
END;
$$;
