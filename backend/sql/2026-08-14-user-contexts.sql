-- #173 per-user AI personalization — Phase 1.
-- One row per user holding the explicit profile injected into the rewrite
-- prompt. Run BEFORE deploying code that reads/writes it (prod synchronize is
-- OFF). Idempotent: safe to re-run.
--
-- style_notes is `text` (not varchar) because it carries the #50 at-rest
-- encryption (ciphertext is longer than the plaintext). FK to users with
-- ON DELETE CASCADE so deleting a user erases their context (GDPR).
CREATE TABLE IF NOT EXISTS public.user_contexts (
    user_id uuid NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    job_title varchar(120) DEFAULT '' NOT NULL,
    industry varchar(120) DEFAULT '' NOT NULL,
    audience varchar(120) DEFAULT '' NOT NULL,
    style_notes text DEFAULT '' NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_contexts_pkey PRIMARY KEY (user_id),
    CONSTRAINT user_contexts_user_fk FOREIGN KEY (user_id)
        REFERENCES public.users (id) ON DELETE CASCADE
);

-- ── Down (manual rollback) ────────────────────────────────────────────────
-- DROP TABLE IF EXISTS public.user_contexts;
