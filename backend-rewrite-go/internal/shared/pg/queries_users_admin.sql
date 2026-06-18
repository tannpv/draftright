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
