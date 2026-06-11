-- Add app_releases.sha256 — the hex SHA-256 of the artifact at download_url.
--
-- Desktop updaters (Windows .exe, Linux .AppImage) verify the downloaded
-- file against this hash before executing it. Empty = unrecorded (the
-- client treats it as "unverified": logs a warning but still installs, for
-- backwards-compat with releases published before this column existed).
--
-- On dev `synchronize: true` adds this column automatically when TypeORM
-- starts. Prod (`synchronize: false`) needs this migration run BEFORE the
-- new backend image starts.
--
-- Operator step on prod:
--   docker exec -i postgres-prod psql -U draftright -d draftright \
--     < backend/sql/2026-06-11-app-release-sha256.sql
--   (then) deploy new backend image
--
-- Idempotent: ADD COLUMN IF NOT EXISTS skips when already present.

ALTER TABLE app_releases
  ADD COLUMN IF NOT EXISTS sha256 VARCHAR(64) NOT NULL DEFAULT '';
