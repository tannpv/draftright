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
SELECT s.status, s.store_type, s.started_at, s.expires_at, s.store_transaction_id,
       p.name AS plan_name, p.daily_limit
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'::subscriptions_status_enum
ORDER BY s.created_at DESC
LIMIT 1;

-- name: GetActiveSubWithPlan :one
-- GET /subscription: newest active sub + plan fields incl billing_period.
-- Distinct from GetActiveSubscriptionByUserID (which omits billing_period
-- and is pinned for /auth/account).
SELECT s.status, s.expires_at,
       p.name AS plan_name, p.daily_limit, p.billing_period
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'::subscriptions_status_enum
ORDER BY s.created_at DESC
LIMIT 1;

-- name: GetLastExpiredAt :one
-- updated_at of the user's most-recently expired subscription (free-tier
-- just_expired banner). No row → caller treats as nil.
SELECT updated_at FROM subscriptions
WHERE user_id = $1 AND status = 'expired'::subscriptions_status_enum
ORDER BY updated_at DESC
LIMIT 1;

-- name: CountUsageToday :one
-- Mirrors usageService.countTodayByUser: rows since local midnight.
-- The caller passes the midnight boundary so timezone handling matches
-- the Node process (new Date(); setHours(0,0,0,0)).
SELECT COUNT(*) FROM usage_logs
WHERE user_id = $1 AND created_at >= $2;

-- name: CountUsageTodayAll :one
-- Global rewrite count since local midnight (Node usageService.countToday()).
SELECT COUNT(*) FROM usage_logs WHERE created_at >= $1;

-- name: CountUsageThisMonthAll :one
-- Global rewrite count since local first-of-month (Node usageService.countThisMonth()).
SELECT COUNT(*) FROM usage_logs WHERE created_at >= $1;

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

-- name: FindUserByGoogleId :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE google_id = $1 LIMIT 1;

-- name: FindUserByFacebookId :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE facebook_id = $1 LIMIT 1;

-- name: FindUserByTiktokId :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE tiktok_id = $1 LIMIT 1;

-- name: FindUserByAppleId :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE apple_id = $1 LIMIT 1;

-- name: LinkSocialGoogle :exec
UPDATE users SET google_id = $2, avatar_url = $3, email_verified = true,
  updated_at = now() WHERE id = $1;

-- name: LinkSocialFacebook :exec
UPDATE users SET facebook_id = $2, avatar_url = $3, email_verified = true,
  updated_at = now() WHERE id = $1;

-- name: LinkSocialTiktok :exec
UPDATE users SET tiktok_id = $2, avatar_url = $3, email_verified = true,
  updated_at = now() WHERE id = $1;

-- name: LinkSocialApple :exec
UPDATE users SET apple_id = $2, avatar_url = $3, email_verified = true,
  updated_at = now() WHERE id = $1;

-- name: ListActivePlans :many
-- Mirrors plansService.findAll(): every active plan, cheapest first.
-- No tiebreaker beyond price_cents (matches TypeORM order:{price_cents:'ASC'}).
SELECT id, name, daily_limit, price_cents, currency, stripe_price_id,
       trial_days, billing_period, is_active, created_at, updated_at
FROM plans
WHERE is_active = true
ORDER BY price_cents ASC;

-- name: SubsDueForRenewal :many
-- Cron pass 1: active subs whose expiry falls in the reminder window.
-- Bounds passed by caller (now+2.5d, now+3.5d) to keep tz in the Go process.
SELECT s.user_id, u.email, s.expires_at, p.name AS plan_name
FROM subscriptions s
JOIN users u ON u.id = s.user_id
JOIN plans p ON p.id = s.plan_id
WHERE s.status = 'active'::subscriptions_status_enum
  AND s.expires_at >= $1 AND s.expires_at <= $2;

-- name: ExpireLapsedSubs :many
-- Cron pass 2: flip active|cancelled subs past expiry to expired, returning
-- the affected rows so the caller can email each. expires_at left untouched.
UPDATE subscriptions s
SET status = 'expired'::subscriptions_status_enum, updated_at = now()
FROM users u
WHERE u.id = s.user_id
  AND s.status IN ('active'::subscriptions_status_enum, 'cancelled'::subscriptions_status_enum)
  AND s.expires_at < $1
RETURNING s.user_id, u.email, s.expires_at;

-- name: CancelActiveSubsByUser :exec
UPDATE subscriptions SET status = 'cancelled'::subscriptions_status_enum, updated_at = now()
WHERE user_id = $1 AND status = 'active'::subscriptions_status_enum;

-- name: InsertGrantedSubscription :one
INSERT INTO subscriptions (user_id, plan_id, status, store_type, started_at, expires_at)
VALUES ($1, $2, 'active'::subscriptions_status_enum, $3, now(), $4)
RETURNING id, user_id, plan_id, status, store_type, store_transaction_id, started_at, expires_at, created_at, updated_at;

-- name: StampStoreRefByReference :execrows
UPDATE subscriptions SET store_type = $2, store_transaction_id = $3, updated_at = now()
WHERE id = (
  SELECT sub.id FROM subscriptions sub
  INNER JOIN payments pay ON pay.user_id = sub.user_id AND pay.reference_code = $1
  WHERE sub.status = 'active'::subscriptions_status_enum
  ORDER BY sub.created_at DESC
  LIMIT 1
);

-- name: ExtendByStoreRef :execrows
UPDATE subscriptions SET expires_at = $3, updated_at = now()
WHERE store_type = $1 AND store_transaction_id = $2 AND status = 'active'::subscriptions_status_enum;

-- name: CancelByStoreRef :execrows
UPDATE subscriptions SET status = 'cancelled'::subscriptions_status_enum, updated_at = now()
WHERE store_type = $1 AND store_transaction_id = $2 AND status = 'active'::subscriptions_status_enum;

-- name: ExpireByStoreRef :execrows
UPDATE subscriptions SET status = 'expired'::subscriptions_status_enum, expires_at = now(), updated_at = now()
WHERE store_type = $1 AND store_transaction_id = $2;

-- name: FindByStoreRef :one
SELECT sub.user_id, u.email AS user_email, u.name AS user_name, p.name AS plan_name
FROM subscriptions sub
JOIN users u ON u.id = sub.user_id
JOIN plans p ON p.id = sub.plan_id
WHERE sub.store_type = $1 AND sub.store_transaction_id = $2
LIMIT 1;

-- name: CountActiveSubscriptions :one
-- Mirrors subscriptionsService.countActive(): COUNT where status=active.
SELECT COUNT(*) FROM subscriptions WHERE status = 'active'::subscriptions_status_enum;

-- name: PlansBreakdown :many
-- Mirrors subscriptionsService.getPlansBreakdown(): active subs grouped by plan.
-- LEFT JOIN so subs with no plan row (data-quality gap) still appear.
SELECT p.name AS plan_name, p.price_cents AS price_cents, COUNT(*) AS active_count
FROM subscriptions s
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.status = 'active'::subscriptions_status_enum
GROUP BY p.name, p.price_cents;

-- name: CountNewSubsInWindow :one
-- Mirrors getMonthlyStats new-subs sub-query: ALL subs started in [start, end).
-- Node uses subsRepo.createQueryBuilder('sub').leftJoin('sub.plan','plan')
-- .where('sub.started_at >= :start AND sub.started_at < :end').getCount()
-- No status filter — matches Node exactly.
SELECT COUNT(*) FROM subscriptions
WHERE started_at >= $1 AND started_at < $2;

-- name: SumRevenueInWindow :one
-- Mirrors getMonthlyStats revenue sub-query: SUM(plan.price_cents) for subs
-- started in [start, end). Node: COALESCE(SUM(plan.price_cents),0) via leftJoin.
-- No status filter — matches Node exactly. Cast to bigint for pgx scan.
SELECT COALESCE(SUM(p.price_cents), 0)::bigint AS total
FROM subscriptions s
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.started_at >= $1 AND s.started_at < $2;

-- name: CountChurnedInWindow :one
-- Mirrors getMonthlyStats churned sub-query: subs with status cancelled or expired
-- whose updated_at falls in [start, end). Node:
-- .where('sub.status IN (:...statuses)', {statuses:['cancelled','expired']})
-- .andWhere('sub.updated_at >= :start AND sub.updated_at < :end').getCount()
SELECT COUNT(*) FROM subscriptions
WHERE status IN ('cancelled'::subscriptions_status_enum, 'expired'::subscriptions_status_enum)
  AND updated_at >= $1 AND updated_at < $2;

-- name: ListSubscriptionsPaginated :many
-- Mirrors subscriptionsService.findAllPaginated (admin transactions list):
-- LEFT JOIN users + plans, ORDER BY created_at DESC, optional ILIKE search.
-- When search IS NULL the WHERE branch matches all rows (Node omits WHERE entirely).
-- Limit/offset passed by caller; default limit 20 (bespoke, not shared 10).
SELECT s.id,
       u.email  AS user_email,
       u.name   AS user_name,
       s.user_id,
       p.name   AS plan_name,
       p.price_cents,
       s.store_type,
       s.store_transaction_id,
       s.status,
       s.started_at,
       s.expires_at,
       s.created_at
FROM subscriptions s
LEFT JOIN users u ON u.id = s.user_id
LEFT JOIN plans p ON p.id = s.plan_id
WHERE (sqlc.narg('search')::text IS NULL
       OR u.email ILIKE '%' || sqlc.narg('search') || '%'
       OR u.name  ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY s.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSubscriptionsPaginated :one
-- Count twin of ListSubscriptionsPaginated — no LIMIT/OFFSET.
-- Same optional search filter; LEFT JOIN users only (no plans needed for count).
SELECT COUNT(*) FROM subscriptions s
LEFT JOIN users u ON u.id = s.user_id
WHERE (sqlc.narg('search')::text IS NULL
       OR u.email ILIKE '%' || sqlc.narg('search') || '%'
       OR u.name  ILIKE '%' || sqlc.narg('search') || '%');

-- name: FindLatestStripeForUserPlan :one
-- Newest Stripe subscription for a (user, plan) pair — used by admin refund flow.
-- Any status is fine (the sub may be cancelled/expired after the charge).
-- Returns (id, store_transaction_id); no row → pgx.ErrNoRows.
SELECT id, store_transaction_id FROM subscriptions
WHERE user_id = $1 AND plan_id = $2
  AND store_type = 'stripe'::subscriptions_store_type_enum
ORDER BY created_at DESC LIMIT 1;
