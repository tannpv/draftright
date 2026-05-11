-- 2026-05-09 — add Stripe + SePay mode flags + Resend creds to app_settings.
-- Idempotent: uses ADD COLUMN IF NOT EXISTS.
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS stripe_mode varchar(20) NOT NULL DEFAULT 'test';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS sepay_mode varchar(20) NOT NULL DEFAULT 'sandbox';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS resend_api_key varchar(500) NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS email_from varchar(200) NOT NULL DEFAULT 'DraftRight <noreply@draftright.info>';
