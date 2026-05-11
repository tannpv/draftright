-- 2026-05-10-bug-reports.sql
-- User-submitted bug reports from any client surface (admin portal,
-- marketing site, web playground, native apps, mobile keyboards/share
-- extensions). Distinct from `error_reports`, which is auto-collected
-- crash telemetry.
--
-- Submission endpoint is public (POST /bug-reports). Auth is optional —
-- a logged-in JWT in the Authorization header gets stamped on user_id;
-- anonymous submissions are accepted too. Screenshots are stored on
-- disk at /var/lib/draftright/bug-reports/YYYY-MM-DD/<uuid>.<ext> and
-- streamed back via the admin-only GET endpoint.
--
-- Migration is idempotent so it's safe to re-run.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS bug_reports (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  source              VARCHAR(50) NOT NULL,
  description         TEXT NOT NULL,
  screenshot_path     VARCHAR(500),
  screenshot_filename VARCHAR(200),
  app_version         VARCHAR(50),
  os_info             VARCHAR(100),
  user_id             UUID REFERENCES users(id),
  user_email          VARCHAR(255),
  context             JSONB,
  status              VARCHAR(20) DEFAULT 'new',
  admin_notes         TEXT,
  created_at          TIMESTAMP DEFAULT NOW(),
  updated_at          TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS bug_reports_status_idx
  ON bug_reports (status, created_at DESC);

CREATE INDEX IF NOT EXISTS bug_reports_source_idx
  ON bug_reports (source);
