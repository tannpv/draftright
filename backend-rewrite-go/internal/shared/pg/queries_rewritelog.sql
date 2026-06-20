-- internal/shared/pg/queries_rewritelog.sql
-- Admin training-data endpoints (GET /admin/training-data/stats,
-- GET /admin/training-data, PATCH /admin/training-data/:id,
-- GET /admin/training-data/export).
-- Mirrors RewriteLogService (backend/src/rewrite/rewrite-log.service.ts).

-- name: CountRewriteLogs :one
SELECT COUNT(*) FROM rewrite_logs;

-- name: CountRewriteLogsByQuality :many
-- One row per distinct quality value — caller maps to (pending, approved, rejected).
SELECT quality, COUNT(*) AS n FROM rewrite_logs GROUP BY quality;

-- name: ListPendingRewriteLogs :many
-- findPending: quality='pending', newest first, with LIMIT/OFFSET pagination.
-- Node: findAndCount({ where: { quality: 'pending' }, order: { created_at: 'DESC' },
--         skip: (page-1)*limit, take: limit })
SELECT id, tone, input_text, output_text, model, provider_type, response_time_ms, quality, created_at
FROM rewrite_logs
WHERE quality = 'pending'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPendingRewriteLogs :one
-- Count twin of ListPendingRewriteLogs (no LIMIT/OFFSET).
SELECT COUNT(*) FROM rewrite_logs WHERE quality = 'pending';

-- name: UpdateRewriteLogQuality :exec
-- updateQuality(id, quality): update the quality field for one row.
-- Node: rewriteLogRepo.update(id, { quality }) — TypeORM silently no-ops on
-- a missing UUID (0 rows affected, no error). Go mirrors: valid UUID → run UPDATE
-- (0-row no-op is fine); invalid UUID → caller returns nil without touching DB.
UPDATE rewrite_logs SET quality = $2 WHERE id = $1;

-- name: InsertRewriteLog :exec
-- log(): fire-and-forget training-data capture after a successful rewrite.
-- Node: rewriteLogRepo.save({ tone, input_text, output_text, model,
--        provider_type, response_time_ms }) — id/quality/created_at default.
-- Mirrors RewriteService.callAI's this.rewriteLogService.log({...}).catch(()=>{}).
INSERT INTO rewrite_logs (tone, input_text, output_text, model, provider_type, response_time_ms)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListApprovedRewriteLogsAsc :many
-- exportApproved / exportAll: quality='approved', oldest first.
-- Node: find({ where: { quality: 'approved' }, order: { created_at: 'ASC' } })
SELECT id, tone, input_text, output_text, model, provider_type, response_time_ms, quality, created_at
FROM rewrite_logs
WHERE quality = 'approved'
ORDER BY created_at ASC;
