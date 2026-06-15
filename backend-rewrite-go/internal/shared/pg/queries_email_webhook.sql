-- name: MarkEmailByProviderID :exec
-- Reflect a Resend delivery event onto email_logs.status (by Resend
-- message id). error is only overwritten when a reason is supplied —
-- a NULL reason leaves the existing error untouched, mirroring the
-- NestJS markByProviderId (which omits error from the UPDATE when the
-- detail arg is null).
UPDATE email_logs SET status = $2, error = COALESCE($3, error) WHERE provider_id = $1;

-- name: SuppressEmail :exec
-- Add an address to the suppression list (idempotent). Mirrors the
-- NestJS suppress() orIgnore() → bare ON CONFLICT DO NOTHING.
INSERT INTO email_suppressions (email, reason) VALUES ($1, $2)
ON CONFLICT DO NOTHING;
