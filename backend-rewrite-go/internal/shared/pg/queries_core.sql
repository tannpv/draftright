-- Phase 0 core-endpoint queries (health + /auth/me). Kept separate
-- from the rewrite module's queries.sql so the core package depends on
-- the shared sqlc types only, never on a feature module.

-- name: GetUserByID :one
-- Minimal user projection for GET /auth/me — id, email, role. Mirrors
-- the fields Node's /auth/me returns from the JWT-resolved user.
SELECT id, email, role
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetClientLogLevel :one
-- The single app_settings row's client_log_level, surfaced by
-- GET /health. There is exactly one settings row (Node does
-- `findOne({ where: {} })`); LIMIT 1 matches that.
SELECT client_log_level
FROM app_settings
LIMIT 1;

-- name: GetPaymentMethodsEnabled :one
-- payment_methods_enabled CSV from the singleton app_settings row. NOT NULL
-- (default ''); empty string means unconfigured → caller falls back to env
-- then default. No row at all → pgx.ErrNoRows, mapped to found=false.
SELECT payment_methods_enabled FROM app_settings LIMIT 1;

-- name: GetPaymentCredentials :one
-- Checkout-time provider credentials from the singleton app_settings row.
-- All columns are NOT NULL DEFAULT ''. No row → pgx.ErrNoRows (caller treats
-- as all-empty → env fallback per resolveCredential).
SELECT stripe_secret_key,
       vietqr_bank_id,
       vietqr_account_number,
       vietqr_account_name,
       lemonsqueezy_api_key,
       lemonsqueezy_store_id,
       lemonsqueezy_variant_monthly,
       lemonsqueezy_variant_yearly
FROM app_settings
LIMIT 1;
