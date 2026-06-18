-- name: ListEmailTemplates :many
SELECT template_key, subject, html FROM email_templates;
