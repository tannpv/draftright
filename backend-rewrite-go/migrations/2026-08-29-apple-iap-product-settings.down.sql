-- Down migration for 2026-08-29-apple-iap-product-settings.up.sql.
-- Reversible per RULE #1 / the dev checklist's "DB drops need a reversible
-- up/down migration" — drops both columns, restoring app_settings to its
-- pre-Apple-IAP shape. Idempotent: IF EXISTS.
--
-- After running, revert the sqlc schema mirror the same way (remove the two
-- columns from internal/platform/db/schema.sql, re-run sqlc generate) and
-- redeploy a Go image built from before this change.
ALTER TABLE app_settings DROP COLUMN IF EXISTS apple_product_monthly;
ALTER TABLE app_settings DROP COLUMN IF EXISTS apple_product_yearly;
