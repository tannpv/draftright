-- Admin-controlled client log verbosity. Surfaced via GET /health and applied
-- by every desktop/mobile client's DRLogger as a minimum-severity threshold:
--   'off'      → write nothing (absolute kill-switch, even errors)
--   'errors'   → ERROR only
--   'warnings' → WARN + ERROR
--   'info'     → everything (default)
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS client_log_level varchar(20) NOT NULL DEFAULT 'info';
