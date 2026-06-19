-- internal/shared/pg/queries_updates.sql

-- name: GetReleasePolicy :one
SELECT platform, preferred, store_status, notes, updated_at
FROM app_release_policies
WHERE platform = $1;

-- name: GetEnabledReleaseByChannel :one
SELECT platform, version, download_url, sha256, release_notes, required, updated_at, channel, enabled
FROM app_releases
WHERE platform = $1 AND channel = $2 AND enabled = true;

-- name: ListAllReleases :many
-- Node ReleasesService.listAll(): releaseRepo.find() — every (platform, channel) row.
SELECT * FROM app_releases;

-- name: ListAllReleasePolicies :many
-- Node ReleasesService.listAll(): policyRepo.find() — every policy row.
SELECT * FROM app_release_policies;

-- name: GetReleaseChannel :one
-- Node upsertChannel(): releaseRepo.findOne({ where: { platform, channel } }).
SELECT * FROM app_releases WHERE platform = $1 AND channel = $2;

-- name: InsertReleaseChannel :one
-- Node upsertChannel() create path. The use case supplies merged values.
INSERT INTO app_releases (platform, channel, version, download_url, sha256, release_notes, required, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateReleaseChannel :one
-- Node upsertChannel() update path. The use case supplies merged values.
UPDATE app_releases
SET version = $3,
    download_url = $4,
    sha256 = $5,
    release_notes = $6,
    required = $7,
    enabled = $8,
    updated_at = now()
WHERE platform = $1 AND channel = $2
RETURNING *;

-- name: DeleteReleaseChannel :execrows
-- Node deleteChannel(): releaseRepo.delete({ platform, channel }).
DELETE FROM app_releases WHERE platform = $1 AND channel = $2;

-- name: InsertReleasePolicy :one
-- Node PoliciesService.upsert() create path. The use case supplies merged values.
INSERT INTO app_release_policies (platform, preferred, store_status, notes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateReleasePolicy :one
-- Node PoliciesService.upsert() update path. The use case supplies merged values.
UPDATE app_release_policies
SET preferred = $2,
    store_status = $3,
    notes = $4,
    updated_at = now()
WHERE platform = $1
RETURNING *;
