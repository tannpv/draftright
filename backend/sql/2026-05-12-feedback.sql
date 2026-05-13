-- Spec A: feedback (feature requests + public upvote board).
-- Idempotent — safe to re-run. Apply on prod with:
--   docker compose -f docker-compose.prod.yml exec -T postgres psql -U draftright -d draftright < backend/sql/2026-05-12-feedback.sql
-- Dev/test pick these up automatically via TypeORM synchronize.

ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS kind            varchar(20) NOT NULL DEFAULT 'bug';
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS title           varchar(80);
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS target_platform varchar(20);
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS vote_count      integer NOT NULL DEFAULT 0;
ALTER TABLE bug_reports ADD COLUMN IF NOT EXISTS is_public       boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS bug_reports_kind_public_votes_idx
  ON bug_reports (kind, is_public, vote_count);

CREATE TABLE IF NOT EXISTS feature_votes (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  feature_id uuid NOT NULL REFERENCES bug_reports(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (feature_id, user_id)
);

CREATE INDEX IF NOT EXISTS feature_votes_feature_idx ON feature_votes (feature_id);
