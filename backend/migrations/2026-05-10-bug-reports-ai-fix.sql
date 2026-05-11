-- AI fix proposals for bug reports.
--
-- Mirrors the ai_fix_proposal field on error_reports so the same
-- "self-driving triage" experience works for human-submitted bugs.
-- A new cron tick analyzes open bugs and writes a proposed fix; admin
-- reviews and either applies or rejects, same flow as errors.
--
-- Status workflow gains 'fix_proposed' between 'new' and 'resolved'.
-- Existing bug rows keep their current status; only new analyses set it.

BEGIN;

ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS ai_fix_proposal TEXT;
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS ai_fix_proposed_at TIMESTAMPTZ;

COMMIT;
