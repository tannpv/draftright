-- Suppression list populated by the Resend delivery webhook (hard
-- bounces + spam complaints). Checked on every send. Prod runs with
-- synchronize=OFF, so create the table by hand before deploying.
CREATE TABLE IF NOT EXISTS email_suppressions (
  email      VARCHAR(255) PRIMARY KEY,
  reason     VARCHAR(16)  NOT NULL,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
