DROP TABLE IF EXISTS redirect_flow_sessions;
DROP TABLE IF EXISTS bank_details;
ALTER TABLE memberships DROP COLUMN IF EXISTS gocardless_subscription_id;
