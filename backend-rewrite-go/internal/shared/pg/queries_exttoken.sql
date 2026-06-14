-- Extension-token persistence (dr_ext_* keyboard/share tokens).
-- Read the live NestJS-owned schema as-is; mirror ExtensionTokenService
-- (backend/src/auth/extension-token.service.ts) WHERE semantics exactly.
-- Table: public.extension_tokens — scopes text[], device_id uuid,
-- timestamps "without time zone".

-- name: RevokeActiveTokensForDevice :exec
-- mint() rotates: revoke any still-active token for this (user, device)
-- first so the partial unique index idx_ext_tokens_user_device_active
-- (user_id, device_id) WHERE revoked_at IS NULL doesn't collide.
UPDATE extension_tokens
SET revoked_at = now()
WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL;

-- name: InsertExtensionToken :one
-- mint() insert. Returns the projection list() serializes (token_hash and
-- user_id stripped by the controller, so omitted here).
INSERT INTO extension_tokens (user_id, token_hash, scopes, device_id, device_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, scopes, device_id, device_name, last_used_at, created_at, revoked_at;

-- name: ListActiveTokens :many
-- list(): active tokens for the user, newest first
-- (TypeORM where:{user_id, revoked_at:IsNull()} order:{created_at:'DESC'}).
SELECT id, scopes, device_id, device_name, last_used_at, created_at, revoked_at
FROM extension_tokens
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeTokenByID :exec
-- revoke(): owner-scoped UPDATE keyed by (id, user_id). Node does NOT filter
-- revoked_at IS NULL here, so re-revoking an already-revoked row is a no-op
-- touch of revoked_at — idempotent, controller returns 204 either way.
UPDATE extension_tokens
SET revoked_at = now()
WHERE id = $1 AND user_id = $2;

-- name: FindActiveTokenByHash :one
-- validate(): resolve a presented token's hash to owner + scopes. Returns id
-- too — Verify (T13) fires TouchTokenLastUsed by id afterwards.
SELECT id, user_id, scopes, revoked_at
FROM extension_tokens
WHERE token_hash = $1 AND revoked_at IS NULL
LIMIT 1;

-- name: TouchTokenLastUsed :exec
-- validate() write-behind: bump last_used_at. Failures non-fatal (caller
-- ignores the error so the request still succeeds).
UPDATE extension_tokens SET last_used_at = now() WHERE id = $1;
