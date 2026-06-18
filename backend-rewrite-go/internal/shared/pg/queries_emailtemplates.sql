-- name: ListEmailTemplates :many
SELECT template_key, subject, html FROM email_templates;

-- name: UpsertEmailTemplate :exec
INSERT INTO email_templates (template_key, subject, html, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (template_key) DO UPDATE
SET subject = EXCLUDED.subject, html = EXCLUDED.html, updated_at = now();

-- name: DeleteEmailTemplate :exec
DELETE FROM email_templates WHERE template_key = $1;
