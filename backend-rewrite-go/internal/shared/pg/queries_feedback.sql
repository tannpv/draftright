-- internal/shared/pg/queries_feedback.sql
-- Public feedback board (POST /feedback, GET /feedback, POST /feedback/:id/vote).
-- Shares the bug_reports table with bug-report ingest (kind='feature' vs 'bug')
-- plus the feature_votes table (one vote per user per feature; vote_count is
-- derived = COUNT(feature_votes)).
--
-- The optional status / target_platform filters use the
-- (@x::text IS NULL OR col = @x) idiom so CountFeatures and ListPublicFeatures
-- share one WHERE clause; the use case passes NULL to skip a filter (mirrors
-- Node only adding the key to its `where` object when the value passes its
-- allow-list check).

-- name: InsertFeedback :one
INSERT INTO bug_reports
  (kind, title, target_platform, vote_count, is_public, source, description,
   app_version, os_info, user_id, user_email, context, status)
VALUES ($1, $2, $3, 0, true, $4, $5, $6, $7, $8, $9, $10, 'new')
RETURNING id, display_no;

-- name: CountFeatures :one
SELECT COUNT(*) FROM bug_reports
WHERE kind = 'feature'
  AND is_public = true
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('target_platform')::text IS NULL OR target_platform = sqlc.narg('target_platform'));

-- name: ListPublicFeatures :many
SELECT
  id, display_no, source, description, screenshot_path, screenshot_filename,
  app_version, os_info, user_id, user_email, context, status, kind, title,
  target_platform, vote_count, is_public, admin_notes, ai_fix_proposal,
  ai_fix_proposed_at, created_at, updated_at
FROM bug_reports
WHERE kind = 'feature'
  AND is_public = true
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('target_platform')::text IS NULL OR target_platform = sqlc.narg('target_platform'))
ORDER BY vote_count DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: FindFeature :one
SELECT id FROM bug_reports WHERE id = $1 AND kind = 'feature';

-- name: FindVote :one
SELECT id FROM feature_votes WHERE feature_id = $1 AND user_id = $2;

-- name: InsertVote :exec
INSERT INTO feature_votes (feature_id, user_id) VALUES ($1, $2);

-- name: DeleteVote :exec
DELETE FROM feature_votes WHERE feature_id = $1 AND user_id = $2;

-- name: CountVotes :one
SELECT COUNT(*) FROM feature_votes WHERE feature_id = $1;

-- name: UpdateFeatureVoteCount :exec
UPDATE bug_reports SET vote_count = $2 WHERE id = $1;

-- name: VotedFeatureIDs :many
SELECT feature_id FROM feature_votes
WHERE feature_id = ANY(@feature_ids::uuid[]) AND user_id = @user_id;
