-- internal/shared/pg/queries_bugreports.sql
-- Public bug-report ingest (POST /bug-reports, multipart with optional
-- screenshot). user_id is nulled when the JWT outlives its user.

-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1);

-- name: InsertBugReport :one
INSERT INTO bug_reports
  (source, description, screenshot_path, screenshot_filename, app_version,
   os_info, user_id, user_email, context, status, kind)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'new','bug')
RETURNING id, display_no;

-- Admin triage (GET/:id, DELETE/:id, PATCH/:id, fix-proposal, cron).
-- The admin LIST (GET /admin/bug-reports) is a parseListQuery consumer with
-- dynamic WHERE/ORDER → assembled in Go on the pgxpool (admin_repo_pg.go),
-- NOT a static sqlc query.

-- name: AdminGetBugReport :one
SELECT * FROM bug_reports WHERE id = $1;

-- name: AdminDeleteBugReport :execrows
DELETE FROM bug_reports WHERE id = $1;

-- name: AdminUpdateBugReport :one
-- The use case (admin_usecase.go) does the partial-merge in Go (load → apply
-- only the provided DTO fields → write the merged values), mirroring Node's
-- per-field `if (dto.x !== undefined)`. This writes the already-merged values.
UPDATE bug_reports
SET status = $2, admin_notes = $3, title = $4, target_platform = $5, is_public = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AdminSetBugFixProposal :one
UPDATE bug_reports
SET ai_fix_proposal = $2, ai_fix_proposed_at = $3, status = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AdminBugFixCandidates :many
-- Cron: status='new' AND kind='bug' AND ai_fix_proposal IS NULL,
-- ORDER BY created_at DESC, LIMIT $1 (5 per run).
SELECT * FROM bug_reports
WHERE status = 'new' AND kind = 'bug' AND ai_fix_proposal IS NULL
ORDER BY created_at DESC
LIMIT $1;
