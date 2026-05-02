-- 2026-04-30: email verification columns
-- Soft-gate registration flow: new accounts unverified, must enter 6-digit code from email.
-- Existing users (admin + dogfood accounts) are grandfathered to verified=true.

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_code varchar(6);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_expires timestamptz;

UPDATE users SET email_verified = true WHERE email_verified = false;
