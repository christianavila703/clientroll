ALTER TABLE membership_types
    ADD COLUMN IF NOT EXISTS summary TEXT,
    ADD COLUMN IF NOT EXISTS benefits TEXT;
