-- internal/shared/pg/queries_adminauth.sql
-- Admin authentication (POST /admin/auth/login, change-password, GET me).
-- admin_users is the portal-admin table, separate from `users` (customers).

-- name: FindAdminByEmailLower :one
SELECT * FROM admin_users WHERE LOWER(email) = LOWER($1);

-- name: FindAdminByID :one
SELECT * FROM admin_users WHERE id = $1;

-- name: UpdateAdminPasswordHash :exec
UPDATE admin_users SET password_hash = $2, updated_at = now() WHERE id = $1;
