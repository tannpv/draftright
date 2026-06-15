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
