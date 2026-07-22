-- 2026-07-21 — PayPal Subscriptions: webhook ID + billing-plan IDs on app_settings.
-- Complements the existing paypal_client_id / paypal_client_secret / paypal_mode
-- columns. Idempotent: uses ADD COLUMN IF NOT EXISTS.
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS paypal_webhook_id  varchar(100) NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS paypal_plan_monthly varchar(100) NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS paypal_plan_yearly  varchar(100) NOT NULL DEFAULT '';
