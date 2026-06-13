-- Phase 1a auth queries. Read the live NestJS-owned schema as-is.

-- name: GetAuthUserByEmail :one
-- Full projection for /auth/login. password_hash is nullable
-- (social-only accounts have none). No email normalization — Node's
-- login passes the email through unchanged.
SELECT id, email, password_hash, name, is_active, role,
       auth_provider, email_verified, lemonsqueezy_customer_id
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetAuthUserByID :one
-- Same projection keyed by id — used by refresh, change-password, account.
SELECT id, email, password_hash, name, is_active, role,
       auth_provider, email_verified, lemonsqueezy_customer_id
FROM users
WHERE id = $1
LIMIT 1;

-- name: UpdateUserPasswordHash :exec
UPDATE users SET password_hash = $2, updated_at = now()
WHERE id = $1;

-- name: GetActiveSubscriptionByUserID :one
-- Mirrors subscriptionsService.findActiveByUserId: newest ACTIVE
-- subscription for the user, joined to its plan. ORDER BY created_at
-- DESC + LIMIT 1 reproduces TypeORM order:{created_at:'DESC'} findOne.
SELECT s.status, s.store_type, s.started_at, s.expires_at,
       p.name AS plan_name, p.daily_limit
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'::subscriptions_status_enum
ORDER BY s.created_at DESC
LIMIT 1;

-- name: CountUsageToday :one
-- Mirrors usageService.countTodayByUser: rows since local midnight.
-- The caller passes the midnight boundary so timezone handling matches
-- the Node process (new Date(); setHours(0,0,0,0)).
SELECT COUNT(*) FROM usage_logs
WHERE user_id = $1 AND created_at >= $2;

-- name: GetAuthTokenSettings :one
-- The single app_settings row's token lifetimes. No row → caller uses
-- defaults (15 / 90), matching Node's `?? 15` / `?? 90`.
SELECT token_expiry_minutes, refresh_token_expiry_days
FROM app_settings
LIMIT 1;
