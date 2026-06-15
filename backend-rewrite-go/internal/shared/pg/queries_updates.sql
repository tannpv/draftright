-- internal/shared/pg/queries_updates.sql

-- name: GetReleasePolicy :one
SELECT platform, preferred, store_status, notes, updated_at
FROM app_release_policies
WHERE platform = $1;

-- name: GetEnabledReleaseByChannel :one
SELECT platform, version, download_url, sha256, release_notes, required, updated_at, channel, enabled
FROM app_releases
WHERE platform = $1 AND channel = $2 AND enabled = true;
