-- 2026-05-26: email_logs — audit row per send attempt (EmailService.deliver).
CREATE TABLE IF NOT EXISTS email_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  to_email varchar(255) NOT NULL,
  email_type varchar(64) NOT NULL,
  subject varchar(255) NOT NULL,
  status varchar(16) NOT NULL,
  provider_id varchar(255),
  error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_email_logs_created ON email_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_logs_status ON email_logs (status);
CREATE INDEX IF NOT EXISTS idx_email_logs_type ON email_logs (email_type);
