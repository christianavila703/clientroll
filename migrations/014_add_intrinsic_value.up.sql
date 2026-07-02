-- Intrinsic value: the base value of the membership itself (without bonuses).
-- total_value becomes dynamic: intrinsic_value + sum(bonuses for selected term)
ALTER TABLE membership_types ADD COLUMN IF NOT EXISTS intrinsic_value NUMERIC(12,2) DEFAULT 0;

-- Seed intrinsic_value from existing total_value
UPDATE membership_types SET intrinsic_value = total_value WHERE intrinsic_value = 0;
