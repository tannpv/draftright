-- Admin user CRUD (Phase 4c-2). GET /admin/users/:id returns the FULL
-- TypeORM User entity (the `user` field); PATCH /admin/users/:id re-reads
-- it. Only the full-row GET is static — the bespoke paginated list, its
-- COUNT, and the partial UPDATE have runtime WHERE/ORDER/SET and so are
-- assembled in Go on the pool (NOT here). Columns are listed in
-- entity-declaration order (src/users/entities/user.entity.ts) so the
-- scan lines up with user.UserDetail. The two nullable timestamps are
-- timestamptz; the two non-null timestamps are timestamp.

-- name: GetUserFull :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       google_id, facebook_id, tiktok_id, apple_id, avatar_url,
       stripe_customer_id, email_verified, email_verification_code,
       email_verification_expires, password_reset_code, password_reset_expires,
       password_reset_attempts, lemonsqueezy_customer_id, created_at, updated_at
FROM users WHERE id = $1;

-- RecentUsageByUser backs GET /admin/users/:id's recent_usage field
-- (usageService.findRecentByUser, default limit 20, created_at DESC). Relations
-- (user, ai_provider) are NOT loaded by Node, so only the column fields select.
-- name: RecentUsageByUser :many
SELECT id, user_id, tone, input_length, output_length, ai_provider_id, response_time_ms, created_at
FROM usage_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20;

-- GetActiveSubFullByUser backs GET /admin/users/:id's subscription field
-- (subscriptionsService.findActiveByUserId: status ACTIVE, relations ['plan'],
-- created_at DESC, take 1). The `user` relation is NOT loaded → omitted from the
-- serialised subscription. Enum columns cast ::text so sqlc scans plain Go
-- strings (mirrors the temperature::text / billing_period::text pattern). The
-- joined plan columns are aliased plan_* to feed the nested plans.PlanEntity.
-- name: GetActiveSubFullByUser :one
SELECT s.id, s.user_id, s.plan_id, s.status::text AS status, s.store_type::text AS store_type,
       s.store_transaction_id, s.started_at, s.expires_at, s.created_at, s.updated_at,
       p.id AS plan_id_full, p.name AS plan_name, p.daily_limit AS plan_daily_limit,
       p.price_cents AS plan_price_cents, p.currency AS plan_currency,
       p.stripe_price_id AS plan_stripe_price_id, p.trial_days AS plan_trial_days,
       p.billing_period::text AS plan_billing_period, p.is_active AS plan_is_active,
       p.created_at AS plan_created_at, p.updated_at AS plan_updated_at
FROM subscriptions s JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'::subscriptions_status_enum
ORDER BY s.created_at DESC LIMIT 1;
