-- =============================================================================
-- deploy/shadow/augment.sql
-- Purpose: Idempotent fixture seed applied to draftright_shadow_tmpl (the
--          frozen template DB) after it is cloned from draftright_dev.
--          Every fixture row uses a stable UUID so test code can reference
--          concrete IDs.  All INSERTs are guarded with ON CONFLICT … DO
--          NOTHING (or WHERE NOT EXISTS) so re-running this script is safe.
--
-- Shadow customer: shadow-user@draftright.info  / ShadowPass123
-- Shadow admin:    shadow-admin@draftright.info / ShadowPass123
--
-- bcrypt hash ($2a$10$, cost 10) generated via golang.org/x/crypto/bcrypt;
-- verified before embedding.  Hash = $2a$10$xT9ocK0BCub.GSB9IPcqNOVdfnKgIzC10LDOeDA5/qE0tQWb7Cr/W
--
-- These are DEV-ONLY test credentials.  Committing a bcrypt hash is safe —
-- it is not a password, and the shadow DBs are never exposed to the internet.
-- =============================================================================

-- ─── Stable UUIDs ────────────────────────────────────────────────────────────
-- 00: shadow user          01: shadow subscription
-- 02: shadow admin         03: shadow ext-token
-- 04: shadow plan (Pro)    05: shadow payment
-- 06: shadow ai_provider   07: shadow bug_report
-- 08: shadow error_report  09: shadow app_release row key (platform-based, see below)

-- ─── Plan ────────────────────────────────────────────────────────────────────
-- Insert a shadow "Pro" plan so the subscription fixture has a valid FK.
-- plans has no email unique constraint — guard by name uniqueness (no unique
-- index on name, so guard by id).
INSERT INTO plans (id, name, daily_limit, price_cents, currency, billing_period, is_active)
SELECT
    '00000000-0000-4000-8000-000000000004',
    'Shadow Pro',
    500,
    99900,
    'USD',
    'monthly',
    true
WHERE NOT EXISTS (
    SELECT 1 FROM plans WHERE id = '00000000-0000-4000-8000-000000000004'
);

-- ─── Customer user ───────────────────────────────────────────────────────────
-- users.email has UNIQUE CONSTRAINT UQ_97672ac88f789774dd47f7c8be3
INSERT INTO users (
    id,
    email,
    password_hash,
    name,
    is_active,
    role,
    auth_provider,
    email_verified
)
VALUES (
    '00000000-0000-4000-8000-000000000000',
    'shadow-user@draftright.info',
    '$2a$10$xT9ocK0BCub.GSB9IPcqNOVdfnKgIzC10LDOeDA5/qE0tQWb7Cr/W',
    'Shadow User',
    true,
    'user',
    'local',
    true
)
ON CONFLICT (email) DO NOTHING;

-- ─── Admin user ──────────────────────────────────────────────────────────────
-- admin_users.email has UNIQUE CONSTRAINT UQ_dcd0c8a4b10af9c986e510b9ecc
INSERT INTO admin_users (
    id,
    email,
    password_hash,
    name,
    is_active,
    role
)
VALUES (
    '00000000-0000-4000-8000-000000000002',
    'shadow-admin@draftright.info',
    '$2a$10$xT9ocK0BCub.GSB9IPcqNOVdfnKgIzC10LDOeDA5/qE0tQWb7Cr/W',
    'Shadow Admin',
    true,
    'admin'
)
ON CONFLICT (email) DO NOTHING;

-- ─── Subscription ────────────────────────────────────────────────────────────
-- Depends on: users(00), plans(04)
-- status enum: active | cancelled | expired
-- store_type enum: google_play | apple_iap | admin_granted | lemonsqueezy |
--                  stripe | vietqr | bank_transfer | paypal
INSERT INTO subscriptions (
    id,
    user_id,
    plan_id,
    status,
    store_type,
    store_transaction_id,
    started_at,
    expires_at
)
SELECT
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000000',
    '00000000-0000-4000-8000-000000000004',
    'active',
    'admin_granted',
    'shadow-txn-001',
    now(),
    now() + interval '365 days'
WHERE NOT EXISTS (
    SELECT 1 FROM subscriptions WHERE id = '00000000-0000-4000-8000-000000000001'
);

-- ─── Extension token ─────────────────────────────────────────────────────────
-- token_hash is char(64) — use a deterministic SHA-256-length hex string
-- (the actual token value is irrelevant for fixture purposes)
INSERT INTO extension_tokens (
    id,
    user_id,
    token_hash,
    scopes,
    device_id,
    device_name
)
SELECT
    '00000000-0000-4000-8000-000000000003',
    '00000000-0000-4000-8000-000000000000',
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    ARRAY['rewrite'],
    '00000000-0000-4000-8000-000000000003',
    'shadow-device'
WHERE NOT EXISTS (
    SELECT 1 FROM extension_tokens WHERE id = '00000000-0000-4000-8000-000000000003'
);

-- ─── AI provider ─────────────────────────────────────────────────────────────
-- type enum: openai | anthropic | ollama | custom
INSERT INTO ai_providers (
    id,
    name,
    type,
    endpoint_url,
    api_key,
    model,
    temperature,
    is_default,
    is_active
)
SELECT
    '00000000-0000-4000-8000-000000000006',
    'Shadow Ollama',
    'ollama',
    'http://localhost:11434',
    '',
    'llama3.2',
    0.3,
    true,
    true
WHERE NOT EXISTS (
    SELECT 1 FROM ai_providers WHERE id = '00000000-0000-4000-8000-000000000006'
);

-- ─── Payment ─────────────────────────────────────────────────────────────────
-- payments.reference_code has UNIQUE CONSTRAINT
INSERT INTO payments (
    id,
    user_id,
    plan_id,
    amount,
    currency,
    method,
    status,
    reference_code
)
SELECT
    '00000000-0000-4000-8000-000000000005',
    '00000000-0000-4000-8000-000000000000',
    '00000000-0000-4000-8000-000000000004',
    99900,
    'USD',
    'admin_granted',
    'completed',
    'SHADOW-PAY-001'
WHERE NOT EXISTS (
    SELECT 1 FROM payments WHERE id = '00000000-0000-4000-8000-000000000005'
);

-- ─── Bug report ──────────────────────────────────────────────────────────────
-- source, description are NOT NULL; status default 'new', kind default 'bug'
INSERT INTO bug_reports (
    id,
    source,
    description,
    app_version,
    os_info,
    user_id,
    user_email,
    status,
    kind,
    title,
    target_platform,
    is_public
)
SELECT
    '00000000-0000-4000-8000-000000000007',
    'shadow-fixture',
    'Shadow fixture bug report for parity testing.',
    '2.4.1',
    'shadow-os 1.0',
    '00000000-0000-4000-8000-000000000000',
    'shadow-user@draftright.info',
    'new',
    'bug',
    'Shadow fixture bug',
    'ios',
    true
WHERE NOT EXISTS (
    SELECT 1 FROM bug_reports WHERE id = '00000000-0000-4000-8000-000000000007'
);

-- ─── Error report ────────────────────────────────────────────────────────────
-- fingerprint is char(64) NOT NULL; display_no auto-increments
-- status is integer (0=open), severity default 'error'
INSERT INTO error_reports (
    id,
    platform,
    app_version,
    severity,
    error_type,
    message,
    fingerprint,
    count,
    status
)
SELECT
    '00000000-0000-4000-8000-000000000008',
    'ios',
    '2.4.1',
    'error',
    'ShadowFixtureError',
    'Shadow fixture error report for parity testing.',
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    1,
    0
WHERE NOT EXISTS (
    SELECT 1 FROM error_reports WHERE id = '00000000-0000-4000-8000-000000000008'
);

-- ─── App release ─────────────────────────────────────────────────────────────
-- PK is (platform, channel) — no UUID; guard by PK
INSERT INTO app_releases (
    platform,
    channel,
    version,
    download_url,
    release_notes,
    required,
    enabled,
    sha256
)
SELECT
    'shadow',
    'direct',
    '0.0.1',
    'https://draftright.info/downloads/shadow/0.0.1',
    'Shadow fixture release for parity testing.',
    false,
    true,
    'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
WHERE NOT EXISTS (
    SELECT 1 FROM app_releases WHERE platform = 'shadow' AND channel = 'direct'
);

-- ─── App release policy ──────────────────────────────────────────────────────
-- PK is platform alone
INSERT INTO app_release_policies (
    platform,
    preferred,
    store_status,
    notes
)
SELECT
    'shadow',
    'direct',
    'not_submitted',
    'Shadow fixture release policy.'
WHERE NOT EXISTS (
    SELECT 1 FROM app_release_policies WHERE platform = 'shadow'
);
