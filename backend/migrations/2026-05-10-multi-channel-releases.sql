-- Multi-channel update URLs.
--
-- Splits app_releases into per-(platform, channel) rows so we can hold
-- both the store URL and the direct download URL simultaneously, and
-- introduces app_release_policies to drive which channel is preferred.
--
-- BEFORE: app_releases PK = (platform). One URL per platform.
-- AFTER:  app_releases PK = (platform, channel). New table app_release_policies.
--
-- Existing rows are migrated to channel='direct' since that's what
-- release-publish.sh has been writing all along.

BEGIN;

-- 1. Add channel column with default, drop old PK, set new composite PK.
ALTER TABLE app_releases ADD COLUMN IF NOT EXISTS channel VARCHAR(20) NOT NULL DEFAULT 'direct';
ALTER TABLE app_releases ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE app_releases DROP CONSTRAINT IF EXISTS app_releases_pkey;
ALTER TABLE app_releases ADD CONSTRAINT app_releases_pkey PRIMARY KEY (platform, channel);

-- 2. Policy table: which channel to surface + store submission status.
CREATE TABLE IF NOT EXISTS app_release_policies (
  platform     VARCHAR(20) PRIMARY KEY,
  preferred    VARCHAR(20) NOT NULL DEFAULT 'direct',
  store_status VARCHAR(30) NOT NULL DEFAULT 'not_submitted',
  notes        TEXT NOT NULL DEFAULT '',
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Seed policy rows for known platforms with today's real state.
INSERT INTO app_release_policies (platform, preferred, store_status, notes) VALUES
  ('mac',     'direct', 'n/a',          'No Mac App Store path. Direct DMG only.'),
  ('linux',   'direct', 'n/a',          'No Linux store path. Direct tarball only.'),
  ('windows', 'direct', 'in_review',    'Microsoft Store cert pending; sideload via direct EXE meanwhile.'),
  ('android', 'direct', 'in_review',    'Play Store closed testing (14-day clock); sideload APK meanwhile.'),
  ('ios',     'direct', 'in_review',    'App Store review pending; TestFlight only meanwhile.')
ON CONFLICT (platform) DO NOTHING;

COMMIT;
