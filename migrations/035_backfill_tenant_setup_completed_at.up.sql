-- Backfill setup completion for existing tenants present at migration time
UPDATE tenants
SET setup_completed_at = COALESCE(setup_completed_at, NOW()),
    updated_at = NOW()
WHERE setup_completed_at IS NULL;
