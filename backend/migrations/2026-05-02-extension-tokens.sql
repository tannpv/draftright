-- 2026-05-02-extension-tokens.sql
-- Long-lived, scoped tokens for the iOS keyboard, iOS share extension,
-- and Android keyboard. Each row represents a token currently issued to
-- a single (user, device) pair. Plaintext tokens are never stored —
-- only sha256(token).
--
-- Migration is idempotent (IF NOT EXISTS on table and indexes) so it is
-- safe to re-run during development or against the testing database
-- after manual edits.

CREATE TABLE IF NOT EXISTS extension_tokens (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash    CHAR(64) NOT NULL,
  scopes        TEXT[] NOT NULL DEFAULT ARRAY['rewrite'],
  device_id     UUID NOT NULL,
  device_name   VARCHAR(64) NOT NULL DEFAULT 'mobile',
  last_used_at  TIMESTAMP WITHOUT TIME ZONE,
  created_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
  revoked_at    TIMESTAMP WITHOUT TIME ZONE
);

-- Active-token lookup (used by the auth guard on every /rewrite call).
-- Partial unique index on token_hash WHERE revoked_at IS NULL ensures
-- collisions only matter for live tokens.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_tokens_hash_active
  ON extension_tokens (token_hash)
  WHERE revoked_at IS NULL;

-- Re-mint replaces the existing active token for a (user, device) pair.
-- Partial unique on (user_id, device_id) WHERE revoked_at IS NULL
-- enforces "at most one active token per device per user."
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_tokens_user_device_active
  ON extension_tokens (user_id, device_id)
  WHERE revoked_at IS NULL;

-- Audit / "active devices" listing.
CREATE INDEX IF NOT EXISTS idx_ext_tokens_user
  ON extension_tokens (user_id);
