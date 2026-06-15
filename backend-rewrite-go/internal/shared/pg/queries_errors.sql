-- internal/shared/pg/queries_errors.sql
-- Crash-report ingest: read-then-write dedup (no ON CONFLICT, matching Node).

-- name: FindErrorByFingerprint :one
SELECT * FROM error_reports WHERE fingerprint = $1;

-- name: InsertErrorReport :one
INSERT INTO error_reports
  (platform, app_version, severity, error_type, message, stack_trace, context, user_id, device_id, fingerprint, count, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,0)
RETURNING id, display_no, count, first_seen_at;

-- name: BumpErrorReport :one
UPDATE error_reports SET
  count = count + 1,
  last_seen_at = now(),
  app_version = COALESCE(sqlc.narg('app_version'), app_version),
  user_id = COALESCE(sqlc.narg('user_id'), user_id),
  device_id = COALESCE(sqlc.narg('device_id'), device_id),
  context = COALESCE(sqlc.narg('context'), context)
WHERE fingerprint = $1
RETURNING id, display_no, count, first_seen_at;
