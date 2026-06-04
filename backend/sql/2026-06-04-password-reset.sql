-- Forgot/reset-password support for local accounts.
-- Prod runs with synchronize=OFF, so these columns must be added by hand
-- before deploying the auth changes (otherwise every users query 500s).
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_code VARCHAR(6);
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_expires TIMESTAMPTZ;
-- Brute-force throttle: code is invalidated after N wrong attempts.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_attempts INT NOT NULL DEFAULT 0;
