-- 2026-05-25: app_settings columns that lagged the entity in production.
-- Prod runs migrations (synchronize is off), so AppSettings entity columns added
-- in code do NOT auto-create. Apply by hand on prod when adding a column.
--
-- Idempotent — safe to re-run.

ALTER TABLE app_settings
  ADD COLUMN IF NOT EXISTS client_log_level varchar(20) NOT NULL DEFAULT 'info';

-- Note: a global `payment_test_mode` column was briefly added and then removed
-- in favour of per-provider modes (stripe_mode / paypal_mode / momo_mode /
-- sepay_mode). Drop it if present:
ALTER TABLE app_settings DROP COLUMN IF EXISTS payment_test_mode;
