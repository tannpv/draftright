-- Short display numbers for reports — easier to reference in chat/admin
-- ("BUG-1042", "ERR-203", "FR-58") than the raw UUID. The UUID stays the
-- primary key so callers don't break; display_no is a stable secondary
-- identifier surfaced in the API + admin UI + user-facing confirmations.
--
-- Idempotent: ADD COLUMN IF NOT EXISTS + SEQUENCE OWNED BY makes a re-run safe.
-- Existing rows get sequential numbers ordered by created_at so old reports
-- still display nicely.

-- bug_reports (also stores feature requests; UI prefixes by `kind`: BUG / FR)
ALTER TABLE bug_reports
  ADD COLUMN IF NOT EXISTS display_no BIGINT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'bug_reports_display_no_seq'
  ) THEN
    CREATE SEQUENCE bug_reports_display_no_seq OWNED BY bug_reports.display_no;
  END IF;
END $$;

-- Backfill existing rows in chronological order.
UPDATE bug_reports
SET display_no = sub.rn
FROM (
  SELECT id, ROW_NUMBER() OVER (ORDER BY created_at, id) AS rn
  FROM bug_reports
  WHERE display_no IS NULL
) AS sub
WHERE bug_reports.id = sub.id AND bug_reports.display_no IS NULL;

-- Push the sequence past the highest existing value so new INSERTs continue cleanly.
SELECT setval(
  'bug_reports_display_no_seq',
  GREATEST((SELECT COALESCE(MAX(display_no), 0) FROM bug_reports), 1),
  true
);

ALTER TABLE bug_reports
  ALTER COLUMN display_no SET DEFAULT nextval('bug_reports_display_no_seq'),
  ALTER COLUMN display_no SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS bug_reports_display_no_idx
  ON bug_reports(display_no);

-- error_reports (prefix ERR in UI)
ALTER TABLE error_reports
  ADD COLUMN IF NOT EXISTS display_no BIGINT;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_class WHERE relname = 'error_reports_display_no_seq'
  ) THEN
    CREATE SEQUENCE error_reports_display_no_seq OWNED BY error_reports.display_no;
  END IF;
END $$;

UPDATE error_reports
SET display_no = sub.rn
FROM (
  SELECT id, ROW_NUMBER() OVER (ORDER BY first_seen_at, id) AS rn
  FROM error_reports
  WHERE display_no IS NULL
) AS sub
WHERE error_reports.id = sub.id AND error_reports.display_no IS NULL;

SELECT setval(
  'error_reports_display_no_seq',
  GREATEST((SELECT COALESCE(MAX(display_no), 0) FROM error_reports), 1),
  true
);

ALTER TABLE error_reports
  ALTER COLUMN display_no SET DEFAULT nextval('error_reports_display_no_seq'),
  ALTER COLUMN display_no SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS error_reports_display_no_idx
  ON error_reports(display_no);
