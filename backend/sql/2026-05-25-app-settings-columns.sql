-- 2026-05-25: app_settings columns that lagged the entity in production.
-- Prod runs migrations (synchronize is off), so AppSettings entity columns added
-- in code do NOT auto-create. These were applied by hand on prod after a deploy
-- 500'd ("column AppSettings.<x> does not exist"); this file records them so any
-- other environment / fresh deploy stays in sync.
--
-- Idempotent — safe to re-run.

ALTER TABLE app_settings
  ADD COLUMN IF NOT EXISTS payment_test_mode boolean NOT NULL DEFAULT false;

ALTER TABLE app_settings
  ADD COLUMN IF NOT EXISTS client_log_level varchar(20) NOT NULL DEFAULT 'info';
