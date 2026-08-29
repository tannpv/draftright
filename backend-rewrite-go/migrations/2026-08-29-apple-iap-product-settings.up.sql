-- 2026-08-29 — Apple IAP: App Store product ids on app_settings.
-- Complements the existing lemonsqueezy_variant_monthly/yearly and
-- paypal_plan_monthly/yearly columns (task 6 of the Apple IAP server
-- redemption plan, docs/superpowers/plans/2026-08-28-apple-iap-server-redemption.md).
-- Product ids are public (App Store Connect product identifiers, e.g.
-- com.draftright.pro.monthly) — not secrets, so no encryption-at-rest.
-- Idempotent: uses ADD COLUMN IF NOT EXISTS, matching the repo's prior
-- app_settings column additions (see backend/sql/2026-07-21-paypal-subscription-settings.sql,
-- removed with NestJS in #202 — this migrations/ dir is its Go-era successor).
--
-- Run this against dev/prod BEFORE deploying a Go image that reads these
-- columns (internal/payment/settings_pg.go Credentials()/AppleProducts()).
-- After running, refresh the sqlc schema mirror:
--   internal/platform/db/schema.sql (append the two columns to app_settings)
--   sqlc generate
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS apple_product_monthly varchar(200) NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS apple_product_yearly  varchar(200) NOT NULL DEFAULT '';
