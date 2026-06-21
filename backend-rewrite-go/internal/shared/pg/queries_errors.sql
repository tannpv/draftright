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

-- name: AdminListErrors :many
-- Node ErrorsService.list(): optional platform/status/severity filters,
-- ORDER BY last_seen_at DESC, LIMIT/OFFSET. NULL filter param = no filter.
SELECT * FROM error_reports
WHERE (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform'))
  AND (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'))
ORDER BY last_seen_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountErrors :one
SELECT COUNT(*) FROM error_reports
WHERE (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform'))
  AND (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'));

-- name: AdminGetError :one
SELECT * FROM error_reports WHERE id = $1;

-- name: AdminDeleteError :execrows
DELETE FROM error_reports WHERE id = $1;

-- name: AdminSetErrorStatusRaw :one
-- Byte-parity with Node ErrorsService.setStatus, which binds the RAW
-- request body.status (any JSON type) straight to the int4 column and lets
-- Postgres coerce. status_text is the value rendered as node-pg would send
-- it (NULL for json null); `::integer` reproduces the exact PG errors —
-- `invalid input syntax for type integer: "foo"` for non-numeric, and the
-- not-null violation for json null (#37). A typed int param can't surface
-- those, so Go was wrongly returning 400 instead of Node's 500.
-- status_text is `::text::integer`, NOT `::integer`: the double cast keeps the
-- bind param a TEXT so the raw value ("foo", "2.5", "3") reaches Postgres and
-- int4-input runs at runtime — `'foo'::text::integer` throws the exact
-- `invalid input syntax for type integer: "foo"` Node surfaces. A single
-- `::integer` makes sqlc type the param int32, which can't carry those.
UPDATE error_reports
SET status = sqlc.narg(status_text)::text::integer,
    resolved_at = CASE WHEN sqlc.arg(set_resolved)::boolean THEN sqlc.narg(resolved_at) ELSE resolved_at END,
    resolved_by = CASE WHEN sqlc.arg(set_resolved)::boolean THEN sqlc.narg(resolved_by) ELSE resolved_by END,
    last_seen_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: AdminSetErrorFixProposal :one
UPDATE error_reports
SET ai_fix_proposal = $2, status = $3, last_seen_at = now()
WHERE id = $1
RETURNING *;

-- name: AdminErrorFixCandidates :many
-- Cron: status=0 AND ai_fix_proposal IS NULL AND count >= 2,
-- ORDER BY count DESC, last_seen_at DESC, LIMIT 10.
SELECT * FROM error_reports
WHERE status = 0 AND ai_fix_proposal IS NULL AND count >= 2
ORDER BY count DESC, last_seen_at DESC
LIMIT $1;
