# Go Backend Port — Phase 1a: Users + Auth-core

**Date:** 2026-06-13
**Status:** Design approved
**Parent:** `2026-06-13-go-backend-port-design.md` (master) — Phase 1 decomposed into 1a–1d.
**Predecessor:** Phase 0 (parity foundation) merged to `develop` (`2e56019b`).

## 1. Goal

Exact drop-in port of password-based auth + account read/delete from NestJS to
Go. No behavior change, no schema change — byte-for-byte identical routes, JSON,
status codes. First identity slice; `register`/email/social deferred to 1b/1c.

## 2. Scope

### Endpoints shipped (parity with Node `auth.controller.ts`)

| Route | Guard | HTTP | Request | Response |
|---|---|---|---|---|
| `POST /auth/login` | none | 201 | `{email,password}` | `{access_token, refresh_token, user:{id,email,name}}` |
| `POST /auth/refresh` | none | 200 | `{refresh_token}` | `{access_token, refresh_token}` |
| `POST /auth/change-password` | JWT | 201 | `{current_password,new_password}` | `{success:true}` |
| `GET /auth/account` | JWT | 200 | — | account object or `null` (see §6) |
| `DELETE /auth/account` | JWT | 200 | — | `{deleted:true}` |

`GET /auth/me` already shipped in Phase 0. Note HTTP codes: NestJS `@Post`
defaults to **201**; `login` + `change-password` have no `@HttpCode`, so they
return **201**. `refresh` + `delete` are explicitly **200**. Go must reproduce
these exactly (login/change-password = 201).

### Out of scope (later sub-projects)

- 1b: `register`, `verify-email`, `resend-verification`, `forgot-password`,
  `reset-password` (need Email/Resend adapter + free-plan assignment +
  verification codes).
- 1c: `POST /auth/social` (Google/Apple/Facebook id_token verification).
- 1d: `ai_providers` DB-driven selection, `usage` module formalization, fold in
  `rewrite`.
- Phase 2: subscription **writes**, expiry cron, nudges; `plan.FindFree`.

## 3. Module layout (new packages)

Package-by-feature, clean-arch inside, dependency rule enforced (modules →
shared/platform downward; cross-module only via interfaces wired in the
composition root — no `module A → module B/internal`).

```
internal/
  user/
    domain.go            User struct (id,email,password_hash,name,is_active,role,
                         auth_provider,email_verified,lemonsqueezy_customer_id)
    service.go           Service: ByEmail, ByID, UpdatePasswordHash, DeleteAccount
    repo_pg.go           sqlc-backed UserRepo (queries below)
  auth/
    handler.go           login, refresh, changePassword, account, deleteAccount
    usecase.go           AuthService: orchestrates user + signer + readers
    errors.go            exact Node 401 messages as consts
  subscription/
    reader.go            Reader iface + pg impl: ActiveByUser(userID) → AccountSub
  usage/
    counter.go           Counter iface + pg impl: CountToday(userID) int
  plan/                  (DEFER — created in 1b when FindFree needed; not in 1a)
platform/
  settings/
    reader.go            AppSettings reader: TokenExpiryMinutes, RefreshTokenExpiryDays
  auth/  (extend Phase 0)
    signer.go            SignAccess(claims, ttl) [JWT_SECRET]
                         SignRefresh(claims, ttl) [JWT_REFRESH_SECRET]
    verifier.go          existing access Verifier + new RefreshVerifier
  config/  (extend)      add JWTRefreshSecret (JWT_REFRESH_SECRET)
shared/
  password.go            HashPassword / VerifyPassword (bcrypt cost 10)
```

`auth` depends on interfaces: `user.Service`, `subscription.Reader`,
`usage.Counter`, `settings.Reader`, `auth.Signer`/`RefreshVerifier`. All wired
in `cmd/server/main.go`.

## 4. sqlc queries (new, in `internal/shared/pg/queries_*.sql`)

Reuse the live schema as-is. Add:

- `GetUserByEmail` — `SELECT id,email,password_hash,name,is_active,role,
  auth_provider,email_verified,lemonsqueezy_customer_id FROM users WHERE email=$1`
- `GetUserByID` — already exists from Phase 0 (Task 2); extend its column set if
  needed to cover account fields (name, email_verified, lemonsqueezy_customer_id),
  or add `GetUserAccountByID`. Prefer extending the existing query's projection.
- `UpdateUserPasswordHash` — `UPDATE users SET password_hash=$2, updated_at=now()
  WHERE id=$1`
- `GetActiveSubscriptionByUserID` — join `subscriptions` + `plans`:
  `plan_name (plans.name), status, store_type, started_at, expires_at,
  daily_limit (plans.daily_limit)`, filtered to the active subscription (mirror
  Node `subscriptionsService.findActiveByUserId` selection logic exactly).
- `CountUsageToday` — mirror Node `usageService.countTodayByUser` (count
  `usage_logs` rows for user where created_at in today, server timezone — match
  Node's boundary exactly).
- Account delete = a hand-written pgx transaction (8 statements, §7), NOT sqlc —
  sqlc doesn't model multi-statement txns. Lives in `user/repo_pg.go`.

`email` lookup: Node `login` passes the raw email to `findByEmail` (login does
NOT normalize — only register/verify/etc. do `.trim().toLowerCase()`). Reproduce
exactly: `/auth/login` queries with the email as-sent (confirmed: Node `login` calls
`usersService.findByEmail(email)` with no normalization, unlike register).

## 5. Token signing (extend `platform/auth`)

Node `generateTokens`:
- payload `{sub:user.id, email:user.email, role:user.role}`, HS256.
- access: `JWT_SECRET`, `expiresIn = "${token_expiry_minutes}m"` (AppSettings,
  default 15).
- refresh: `JWT_REFRESH_SECRET`, `expiresIn = "${refresh_token_expiry_days}d"`
  (AppSettings, default 90).

Go:
- `Signer.SignAccess(claims, ttl time.Duration)` and `SignRefresh(claims, ttl)`,
  distinct secrets. Use `golang-jwt/jwt/v5`, HS256, `exp`/`iat` claims matching
  Node's `jsonwebtoken` output (NumericDate seconds).
- TTL resolved per request from `settings.Reader` (one `SELECT` from
  `app_settings`; cache acceptable — match Phase 0 health cache pattern, short
  TTL — but defaults apply when the row/columns are NULL: 15m / 90d).
- `/auth/refresh`: new `RefreshVerifier` (JWT_REFRESH_SECRET) verifies the
  inbound refresh token; on any verify failure or inactive/missing user →
  401 `"Invalid refresh token"`.
- `config`: add `JWTRefreshSecret` from `JWT_REFRESH_SECRET` (required when
  auth endpoints are live; reuse Phase 0 config validation pattern).

## 6. `/auth/account` response shape (exact)

```jsonc
// user not found → null (Node returns null, HTTP 200)
{
  "id": "...",
  "email": "...",
  "name": "...",
  "email_verified": true,
  "has_lemonsqueezy_customer": false,            // !!user.lemonsqueezy_customer_id
  "subscription": {                               // null when no active sub
    "plan_name": "Free",                          // sub.plan?.name ?? "Unknown"
    "status": "active",
    "store_type": "…",
    "started_at": "…",                            // ISO timestamp, Node serialization
    "expires_at": "…",                            // nullable
    "daily_limit": 10,                            // sub.plan?.daily_limit ?? 0
    "usage_today": 3
  }
}
```

Match Node JSON serialization of timestamps (TypeORM returns `Date` →
`JSON.stringify` ISO-8601 `…Z`). Go must emit identical format (RFC3339 with
the same precision; verify against a real Node response in the shadow gate).
`plan_name` falls back to `"Unknown"`, `daily_limit` to `0` when the plan join
is null — reproduce the `??` fallbacks.

## 7. Account deletion cascade (exact, single txn)

Port Node `usersService.deleteAccount` verbatim — one pgx transaction, same
statement order:

```sql
DELETE FROM extension_tokens WHERE user_id = $1;
DELETE FROM payments         WHERE user_id = $1;
DELETE FROM usage_logs       WHERE user_id = $1;
DELETE FROM subscriptions    WHERE user_id = $1;
DELETE FROM feature_votes    WHERE user_id = $1;
UPDATE bug_reports  SET user_id = NULL WHERE user_id = $1;
UPDATE error_reports SET user_id = NULL WHERE user_id = $1;
DELETE FROM users WHERE id = $1;
```

Commit on success; rollback on any error. Then handler returns
`{deleted:true}` (HTTP 200).

## 8. Passwords

`shared.HashPassword`/`VerifyPassword` via `golang.org/x/crypto/bcrypt`, cost
`10` (matches `BCRYPT_ROUNDS`). bcryptjs writes `$2a$`/`$2b$` hashes; Go's
bcrypt verifies both — **existing production password hashes work unchanged**.
New hashes Go writes (`$2a$`) are equally verifiable by Node. No migration.

## 9. Login behavior (exact branch parity)

`login(email, password)`:
1. `user = ByEmail(email)`; if none → 401 `"Invalid credentials"`.
2. if `user.password_hash` empty (social-only account) → 401 with friendly
   message: `"This account was created with {Provider}. Use the {Provider}
   button to sign in."` where `{Provider}` = `providerLabel(auth_provider)`
   (Google/Facebook/Apple); fallback (no label) → `"This account uses a social
   sign-in. Use the provider you signed up with."`. Port `providerLabel`
   mapping from Node.
3. `VerifyPassword` false → 401 `"Invalid credentials"`.
4. `!is_active` → 401 `"Account disabled"`.
5. else `generateTokens` + return `{...tokens, user:{id,email,name}}`.

`change-password`: load user by id (401 if gone), verify `current_password`
(401 `"Current password is incorrect"`), hash new, update, `{success:true}`.

## 10. Error envelope parity

Reuse Phase 0 `shared` envelope `{error, code, request_id}`. All auth 401s map
through `StatusForCode` → 401. NestJS `UnauthorizedException(msg)` →
`{error: msg, code: inferCode(401)="invalid-token", request_id}`. Confirm the
guard-level 401 (missing/invalid JWT) already matches from Phase 0; these are
service-level 401s with custom messages — same envelope, custom `error` string,
`code:"invalid-token"`.

## 11. Composition root + dev fallback

`cmd/server/main.go`: when `pool != nil`, build `user.Repo`,
`subscription.Reader`, `usage.Counter`, `settings.Reader`, dual-secret `Signer`
+ `RefreshVerifier`, and the `auth` handlers; mount them. When `pool == nil`
(dev no-DB), these handlers are **unavailable** (login/refresh/account/delete
need the DB) — return a clear 503 or leave unmounted with a startup warning.
Unlike `/auth/me` (DB-less claims echo), 1a endpoints require Postgres. Router
nil-guards each handler.

## 12. Shadow-compare (1a)

Extend `cmd/shadowdiff` fixtures:
- login (valid creds → 201; bad password → 401; unknown email → 401;
  social-only account → 401 friendly msg)
- refresh (valid → 200; garbage token → 401)
- change-password (happy → 201; wrong current → 401)
- account (free-plan user; user with no active sub → `subscription:null`)
- delete (→ 200 `{deleted:true}`) — run last / against a throwaway fixture user;
  do NOT delete a real shared fixture account.

`ignore_value_of`: `request_id`, `access_token`, `refresh_token` (per-issue JWT
strings differ). Verify tokens **structurally**: decode claims, assert
`{sub,email,role}` + HS256 + `exp` delta ≈ configured TTL — a dedicated test,
not raw-string diff. Live gate against dev Node stays **operator-pending**
(needs dev DB + seeded user + `JWT_SECRET`/`JWT_REFRESH_SECRET`); no
production/Caddy change in 1a.

## 13. Testing (TDD per task)

- `shared/password_test.go`: hash→verify round-trip; verify a **known
  bcryptjs-generated `$2a$10$` hash** (hardcoded vector) to prove cross-compat.
- `platform/auth` signer/verifier: sign access+refresh, verify each with the
  right secret, reject cross-secret, `exp` honored.
- `user` service/repo: ByEmail/ByID hit/miss, UpdatePasswordHash, DeleteAccount
  txn ordering + rollback on mid-txn error.
- `subscription.Reader`: active sub with plan join; no-sub → nil; plan-null
  fallbacks.
- `usage.Counter`: today boundary.
- `auth` handler tests: every login branch, refresh branches, change-password,
  account (sub + null-sub + user-not-found→null), delete; assert exact JSON +
  status (201 vs 200 where it matters).
- `go vet ./...`, `go build ./...`, `go test -race ./...` all green; no
  cross-module internal imports; sqlc drift-free.

## 14. Risks

- **Timestamp serialization** mismatch (Node `Date`→ISO vs Go `time.Time`→
  RFC3339) is the most likely diff in `/auth/account` — pin format in a test
  against a captured Node response.
- **AppSettings columns** may be absent in some envs — defaults (15m/90d) must
  apply on NULL, matching Node `?? 15` / `?? 90`.
- **HTTP 201 vs 200**: easy to get wrong porting to Go (handlers default 200).
  login + change-password MUST be 201.
- **usage today boundary** (timezone) must match Node's count query exactly.
