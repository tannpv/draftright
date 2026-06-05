-- Add APPLE to the auth_provider enum + new apple_id column.
-- Required on prod (synchronize: off) BEFORE the next backend deploy
-- — otherwise every Apple-sign-in attempt will crash the activation
-- INSERT.  Safe to run repeatedly (IF NOT EXISTS guards both).
--
-- See [[project_session_20260601_late_ls_yearly_toggle]] for the
-- companion store_type migration that ran the same way.

ALTER TYPE users_auth_provider_enum ADD VALUE IF NOT EXISTS 'apple';

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS apple_id VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS users_apple_id_unique
  ON users (apple_id)
  WHERE apple_id IS NOT NULL;
