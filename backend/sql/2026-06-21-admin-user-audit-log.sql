-- #51 admin-user deactivation audit log. Append-only. No FKs (snapshot must
-- survive a later hard-delete/rename of either party). No `action` column
-- (scope = deactivation only). Run BEFORE deploying the Go image that queries
-- it. Idempotent: safe to re-run.
CREATE TABLE IF NOT EXISTS public.admin_user_audit_log (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    actor_admin_id uuid NOT NULL,
    actor_email text NOT NULL,
    target_admin_id uuid NOT NULL,
    target_email text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT admin_user_audit_log_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_admin_user_audit_log_created_at
    ON public.admin_user_audit_log (created_at DESC);
