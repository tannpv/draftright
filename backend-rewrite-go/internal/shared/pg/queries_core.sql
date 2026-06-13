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
