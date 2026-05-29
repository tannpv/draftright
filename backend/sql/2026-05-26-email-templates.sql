-- 2026-05-26: email_templates — admin overrides for built-in email templates.
CREATE TABLE IF NOT EXISTS email_templates (
  template_key varchar(64) PRIMARY KEY,
  subject varchar(255) NOT NULL,
  html text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
