-- Reverse migration 075. Drops the audit table entirely; the live
-- terms_and_conditions rows are untouched.

DROP TABLE IF EXISTS terms_and_conditions_versions;
