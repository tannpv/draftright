-- AI provider admin CRUD (Phase 4c-2). The temperature column is
-- decimal(3,2); the Node entity declares it WITHOUT a numeric transformer,
-- so TypeORM returns it as a JS string ("0.30"). To reproduce that exact
-- 2-decimal string in Go (and dodge pgtype.Numeric formatting), every read
-- casts `temperature::text AS temperature` so sqlc types it as a Go string
-- with Postgres-preserved scale-2 text.

-- name: ListAiProviders :many
SELECT id, name, type, endpoint_url, api_key, model, temperature::text AS temperature,
       is_default, is_active, created_at, updated_at
FROM ai_providers
ORDER BY created_at ASC;

-- name: GetAiProviderByID :one
SELECT id, name, type, endpoint_url, api_key, model, temperature::text AS temperature,
       is_default, is_active, created_at, updated_at
FROM ai_providers WHERE id = $1;

-- name: DemoteDefaultAiProviders :exec
UPDATE ai_providers SET is_default = false WHERE is_default = true;

-- name: InsertAiProvider :one
INSERT INTO ai_providers (name, type, endpoint_url, api_key, model, temperature, is_default, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, name, type, endpoint_url, api_key, model, temperature::text AS temperature,
          is_default, is_active, created_at, updated_at;

-- name: SoftDeleteAiProvider :exec
UPDATE ai_providers SET is_default = false, is_active = false WHERE id = $1;
