ALTER TABLE memberships DROP CONSTRAINT IF EXISTS chk_membership_status;

ALTER TABLE memberships
    ADD CONSTRAINT chk_membership_status CHECK (
        status IN ('pending', 'scheduled', 'active', 'past_due', 'cancelled', 'expired', 'suspended')
    );
