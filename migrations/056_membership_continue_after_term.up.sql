ALTER TABLE memberships
ADD COLUMN IF NOT EXISTS continue_after_term BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE memberships m
SET continue_after_term = COALESCE(mto.continue_after_term, FALSE)
FROM membership_term_options mto
WHERE m.term_option_id = mto.id
  AND m.tenant_id = mto.tenant_id;
