-- Queries for the /rewrite microservice's Postgres adapter.
-- sqlc compiles these against schema.sql at build time; mistakes are
-- caught BEFORE the service ever boots (Rule #1 — compile-time over
-- runtime).
--
-- Each query has three sqlc directives:
--   :one  → returns exactly one row (errors if zero)
--   :many → returns a slice of rows
--   :exec → no rows; returns affected-row count
--
-- Naming convention: <Verb><Subject>, mirrors Go function names sqlc
-- generates (FindUserWithPlan, IncrementUsageToday, …).

-- name: FindUserWithPlan :one
-- Resolves user + their active subscription's plan in one query so the
-- quota check is a single round-trip. LEFT JOIN because users without
-- an active subscription fall back to the free tier (plan_id NULL).
-- Filter on subscriptions.status = 'active' so cancelled subs don't
-- still grant their old plan.
SELECT
    u.id          AS user_id,
    u.email       AS user_email,
    u.role        AS user_role,
    p.id          AS plan_id,
    p.name        AS plan_name,
    p.daily_limit AS plan_daily_limit
FROM users u
LEFT JOIN subscriptions s
    ON s.user_id = u.id
    AND s.status = 'active'
    AND (s.expires_at IS NULL OR s.expires_at > NOW())
LEFT JOIN plans p ON p.id = s.plan_id
WHERE u.id = $1
LIMIT 1;

-- name: CountTodayUsage :one
-- Today's daily-quota tally for the user. Compared against
-- plan.daily_limit in the application layer (so the limit can come
-- from a fallback when the user has no subscription).
SELECT COUNT(*)::bigint AS used
FROM usage_logs
WHERE user_id = $1
  AND created_at::date = CURRENT_DATE;

-- name: ActiveAIProvider :one
-- Selects the active default AI provider. NestJS picks the same row in
-- AiProvidersService.findActive — keep the ORDER BY in sync there + here
-- (is_default DESC, is_active = TRUE).
SELECT id, name, type, endpoint_url, api_key, model, temperature
FROM ai_providers
WHERE is_active = TRUE
ORDER BY is_default DESC, created_at ASC
LIMIT 1;

-- name: InsertUsageLog :exec
-- Records one /rewrite call for daily quota tracking + analytics.
-- Mirrors NestJS UsageService.log: same columns, same names, same
-- precision so the two backends populate identical rows.
INSERT INTO usage_logs
    (user_id, tone, input_length, output_length, ai_provider_id, response_time_ms)
VALUES
    ($1, $2, $3, $4, $5, $6);
