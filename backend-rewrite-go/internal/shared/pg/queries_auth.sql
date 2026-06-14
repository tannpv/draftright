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

-- name: CreateUser :one
INSERT INTO users (
  email, password_hash, name, auth_provider, avatar_url, email_verified,
  email_verification_code, email_verification_expires,
  google_id, facebook_id, tiktok_id, apple_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, email, name, avatar_url, email_verified;

-- name: GetUserAuthState :one
SELECT id, email, name, password_hash, auth_provider, is_active,
       email_verified, email_verification_code, email_verification_expires,
       password_reset_code, password_reset_expires, password_reset_attempts
FROM users WHERE email = $1 LIMIT 1;

-- name: UpdateUserVerification :exec
UPDATE users SET email_verified = $2, email_verification_code = $3,
  email_verification_expires = $4, updated_at = now() WHERE id = $1;

-- name: SetEmailVerificationCode :exec
UPDATE users SET email_verification_code = $2, email_verification_expires = $3,
  updated_at = now() WHERE id = $1;

-- name: SetPasswordResetCode :exec
UPDATE users SET password_reset_code = $2, password_reset_expires = $3,
  password_reset_attempts = $4, updated_at = now() WHERE id = $1;

-- name: SetPasswordResetAttempts :exec
UPDATE users SET password_reset_attempts = $2, updated_at = now() WHERE id = $1;

-- name: ResetPasswordHash :exec
UPDATE users SET password_hash = $2, password_reset_code = null,
  password_reset_expires = null, password_reset_attempts = 0,
  updated_at = now() WHERE id = $1;

-- name: FindFreePlan :one
SELECT id, name, daily_limit FROM plans
WHERE billing_period = 'none'::plans_billing_period_enum AND is_active = true
LIMIT 1;

-- name: CreateFreeSubscription :exec
INSERT INTO subscriptions (user_id, plan_id, status, store_type, started_at, expires_at)
VALUES ($1, $2, 'active'::subscriptions_status_enum,
        'admin_granted'::subscriptions_store_type_enum, now(), null);

-- name: IsEmailSuppressed :one
-- Lowercased-email suppression check (bounce/complaint list).
SELECT COUNT(*) > 0 FROM email_suppressions WHERE email = $1;

-- name: InsertEmailLog :exec
-- Audit row for every deliver attempt (suppressed/skipped/sent/failed).
INSERT INTO email_logs (to_email, email_type, subject, status, provider_id, error)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEmailSettings :one
-- app_settings creds: both columns are NOT NULL (default '').
SELECT resend_api_key, email_from FROM app_settings LIMIT 1;

-- name: GetEmailTemplateByKey :one
-- DB template override. PK column is template_key (not key).
SELECT subject, html FROM email_templates WHERE template_key = $1 LIMIT 1;
