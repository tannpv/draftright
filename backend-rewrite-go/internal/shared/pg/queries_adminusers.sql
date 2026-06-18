-- Admin-users CRUD (Phase 4c-2). The bare full-list, INSERT, soft-delete,
-- and exact-email dup-check are static; the paginated branch + partial
-- UPDATE have runtime WHERE/ORDER/SET assembled in Go on the pool.
-- Bare list orders created_at ASC (Node adminUserRepo.find order ASC).
-- Every projection omits password_hash so the secret never leaves the DB.

-- name: ListAdminUsers :many
SELECT id, email, name, is_active, role, created_at, updated_at
FROM admin_users ORDER BY created_at ASC;

-- name: AdminEmailExists :one
SELECT EXISTS(SELECT 1 FROM admin_users WHERE email = $1);

-- name: InsertAdminUser :one
INSERT INTO admin_users (email, password_hash, name, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, is_active, role, created_at, updated_at;

-- name: SoftDeleteAdminUser :exec
UPDATE admin_users SET is_active = false WHERE id = $1;
