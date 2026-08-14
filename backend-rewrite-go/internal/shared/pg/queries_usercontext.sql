-- Per-user rewrite personalization context (#173). Read-only on the Go side:
-- the profile is written by the NestJS /me/context endpoints; Go only reads it
-- to inject the preamble at rewrite time. style_notes comes back encrypted
-- (enc:v1:) and is decrypted in the adapter via secretcipher.

-- name: GetUserContext :one
SELECT enabled, job_title, industry, audience, style_notes
FROM user_contexts
WHERE user_id = $1;
