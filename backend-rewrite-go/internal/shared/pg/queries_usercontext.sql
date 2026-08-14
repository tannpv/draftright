-- Per-user rewrite personalization context (#173). Read-only on the Go side:
-- the profile is written by the NestJS /me/context endpoints; Go only reads it
-- to inject the preamble at rewrite time. style_notes comes back encrypted
-- (enc:v1:) and is decrypted in the adapter via secretcipher.

-- name: GetUserContext :one
SELECT enabled, job_title, industry, audience, style_notes
FROM user_contexts
WHERE user_id = $1;

-- name: UpsertUserContext :one
-- One row per user; INSERT-or-UPDATE so /me/context PUT is idempotent.
-- style_notes arrives already encrypted (enc:v1:) from the handler.
INSERT INTO user_contexts (user_id, enabled, job_title, industry, audience, style_notes, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (user_id) DO UPDATE SET
    enabled     = EXCLUDED.enabled,
    job_title   = EXCLUDED.job_title,
    industry    = EXCLUDED.industry,
    audience    = EXCLUDED.audience,
    style_notes = EXCLUDED.style_notes,
    updated_at  = now()
RETURNING enabled, job_title, industry, audience, style_notes;

-- name: DeleteUserContext :exec
DELETE FROM user_contexts WHERE user_id = $1;
