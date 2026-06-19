# Go Backend Port — Phase 1b: Finish the Auth Module (Design)

**Date:** 2026-06-13 (started) → 2026-06-14
**Status:** Design — awaiting user review
**Branch:** `feature/go-backend-phase1b-20260614`
**Master design:** `docs/superpowers/specs/2026-06-13-go-backend-port-design.md`
**Predecessor:** Phase 1a (login, refresh, change-password, account, delete) — merged to `develop`.

---

## 1. Goal

Port the **6 remaining `/auth` endpoints** from the NestJS backend to Go, **byte-identical**: same routes, request bodies, JSON responses, status codes, error strings, and check-ordering. After 1b the entire auth surface runs on Go and can be shadow-compared against Node before any Caddy flip.

**Endpoints in scope:**

| Endpoint | Method | Status | Auth | Node handler |
|---|---|---|---|---|
| `/auth/register` | POST | 201 | public | `auth.service.register` |
| `/auth/verify-email` | POST | 200 (`@HttpCode OK`) | public | `auth.service.verifyEmail` |
| `/auth/resend-verification` | POST | 200 (`@HttpCode OK`) | public | `auth.service.resendVerification` |
| `/auth/forgot-password` | POST | 200 (`@HttpCode OK`) | public | `auth.service.forgotPassword` |
| `/auth/reset-password` | POST | 200 (`@HttpCode OK`) | public | `auth.service.resetPassword` |
| `/auth/social` | POST | 201 | public | `auth.service.socialLogin` |

**Out of scope (deferred to Phase 4):** `POST /webhooks/resend` (Svix-signed delivery webhook that WRITES `email_suppressions`). During the parallel run Node's webhook keeps that table fresh; Go only READS it. Also deferred: admin email-log/template surfaces.

---

## 2. Build shape

One spec (this doc). One implementation plan in **two parts on one branch**, executed continuously:

- **Part A — email/password lifecycle:** schema patch, `internal/email` module, user-module write methods, `plans` + `subscription` writers, and the 5 lifecycle endpoints (register, verify-email, resend-verification, forgot-password, reset-password).
- **Part B — social:** the 4 provider verifiers behind a fakeable interface, social-id lookup, account linking + takeover guard, and `POST /auth/social`.

Natural checkpoint between halves; Part B depends on Part A's user-write methods and free-plan assignment.

---

## 3. Endpoint specifications (byte-identical)

All email normalization is `strings.ToLower(strings.TrimSpace(email))` **except `/auth/login`** (Phase 1a, unchanged — Node's login does NOT normalize). Where Node returns `{ success: true }` the Go body is exactly `{"success":true}` with status as noted.

### 3.1 POST /auth/register → 201

**Body:** `{ email, password, name }` — validated by `RegisterDto`:
`email @IsEmail`, `password @IsString @MinLength(8)`, `name @IsString @MinLength(1)`.

**Validation parity** (see §8). On failure → 400 `invalid-input`, message = humanized constraint message(s) joined with `". "`.

**Flow (order matters):**
1. `norm = lower(trim(email))`.
2. `findByEmail(norm)` — if found → **409 `conflict`**, message `"Email already registered"`.
3. `hash = bcrypt(password, cost 10)`.
4. `code = csprng6()` (see §7), `expires = now + 15m`.
5. `users.Create({ email: norm, password_hash: hash, name, email_verification_code: code, email_verification_expires: expires })` → returns new user (id, email, name).
6. `freePlan = plans.FindFreePlan()`; `subscription.CreateFree(user.id, freePlan.id)`.
7. **Fire-and-forget** `email.SendVerification(norm, name, code)` — never blocks or fails the response.
8. `tokens = generateTokens(user)` (Phase 1a machinery, unchanged).

**Response (201):**
```json
{ "access_token": "...", "refresh_token": "...",
  "user": { "id": "...", "email": "...", "name": "...", "email_verified": false } }
```
> `email_verified` is hardcoded `false` here (Node returns the literal, not the column).

### 3.2 POST /auth/verify-email → 200

**Body:** `{ email, code }` (inline, no DTO/class-validator).

**Flow:**
1. `user = findByEmail(lower(trim(email)))`.
2. If `!user || user.email_verification_code == "" || user.email_verification_expires == nil` → **400 `invalid-input`** `"Invalid or expired verification code"`.
3. If `user.email_verification_code != code` → same 400.
4. If `expires.Before(now)` → same 400.
5. `users.Update(id, { email_verified: true, email_verification_code: null, email_verification_expires: null })`.

**Response (200):** `{"success":true}`.

### 3.3 POST /auth/resend-verification → 200

**Body:** `{ email }`.

**Flow (silent — never leaks existence):**
1. `user = findByEmail(lower(trim(email)))`.
2. If `!user || user.email_verified` → return (still 200 `{"success":true}`).
3. `code = csprng6()`, `expires = now + 15m`; `users.Update(id, { email_verification_code: code, email_verification_expires: expires })`.
4. Fire-and-forget `email.SendVerification(user.email, user.name, code)`.

**Response (200):** `{"success":true}` (controller returns it after awaiting the service, which returns void).

### 3.4 POST /auth/forgot-password → 200

**Body:** `{ email }`.

**Flow (silent for unknown AND social-only):**
1. `user = findByEmail(lower(trim(email)))`.
2. If `!user || user.auth_provider != "local" || user.password_hash == ""` → return (200 `{"success":true}`).
3. `code = csprng6()`, `expires = now + 15m`; `users.Update(id, { password_reset_code: code, password_reset_expires: expires, password_reset_attempts: 0 })`.
4. Fire-and-forget `email.SendPasswordReset(user.email, user.name, code)`.

**Response (200):** `{"success":true}`.

### 3.5 POST /auth/reset-password → 200

**Body:** `{ email, code, new_password }`.

**Flow:**
1. If `new_password == "" || len(new_password) < 8` → **400 `invalid-input`** `"Password must be at least 8 characters"`.
2. `user = findByEmail(lower(trim(email)))`.
3. If `!user || user.password_reset_code == "" || user.password_reset_expires == nil` → **400** `"Invalid or expired reset code"`.
4. `badCode = (user.password_reset_code != code) || expires.Before(now)`.
5. If `badCode`:
   - `attempts = user.password_reset_attempts + 1`.
   - `exhausted = attempts >= 5` (MAX_RESET_ATTEMPTS).
   - `users.Update(id, exhausted ? { password_reset_code: null, password_reset_expires: null, password_reset_attempts: 0 } : { password_reset_attempts: attempts })`.
   - throw **400** `"Invalid or expired reset code"`.
6. On success: `hash = bcrypt(new_password, 10)`; `users.Update(id, { password_hash: hash, password_reset_code: null, password_reset_expires: null, password_reset_attempts: 0 })`.

**Response (200):** `{"success":true}`.

### 3.6 POST /auth/social → 201

**Body:** `{ provider, id_token, name?, email?, avatar_url? }` (inline, NO class-validator).

**Flow:**
1. `providerEnum = toAuthProvider(lower(provider))` → one of `google|facebook|tiktok|apple`, else **400 `invalid-input`** `"Unsupported provider: " + provider` (uses the RAW provider string, pre-lowercase).
2. `profile = verifySocialToken(providerEnum, id_token, {name, email, avatar_url})` (see §6) → `SocialProfile{ socialId, email, name, avatar_url, emailVerified }`.
3. If `profile.email == ""` → **400 `invalid-input`** `"Email is required for social login"`.
4. `user = users.FindBySocialId(provider, profile.socialId)` (queries the `{provider}_id` column).
5. `providerVerifiedEmail = (profile.emailVerified == true)`.
6. If `user == nil`:
   - `user = users.FindByEmail(profile.email)`.
   - If `user != nil` (email already exists — link path):
     - If `!providerVerifiedEmail` → **401 `invalid-token`** `"This email is registered. Sign in with your password to link this account."`.
     - `users.Update(user.id, { {provider}_id: profile.socialId, avatar_url: profile.avatar_url || user.avatar_url, email_verified: true })`; reload `user = FindByID(user.id)`.
   - Else (create path):
     - `name = profile.name || profile.email.split("@")[0]`.
     - `user = users.Create({ email: profile.email, name, auth_provider: providerEnum, {provider}_id: profile.socialId, avatar_url: profile.avatar_url, email_verified: providerVerifiedEmail })` (password_hash null).
     - `freePlan = plans.FindFreePlan()`; `subscription.CreateFree(user.id, freePlan.id)`.
7. If `user == nil || !user.is_active` → **401 `invalid-token`** `"Account disabled"`.
8. `tokens = generateTokens(user)`.

**Response (201):**
```json
{ "access_token": "...", "refresh_token": "...",
  "user": { "id": "...", "email": "...", "name": "..." } }
```
> Note: a user found directly by social-id (step 4 hit) skips the takeover guard entirely — existing linked accounts just log in.

---

## 4. Module changes

### 4.1 `internal/user` (extend)

**`domain.go` — add fields to `User`:** `AvatarURL string`, `AppleID/GoogleID/FacebookID/TikTokID` are NOT needed on the read struct (lookups are by-column writes); only `AvatarURL` is read back (link path `|| user.avatar_url`). Keep the struct minimal — add `AvatarURL` only.

**`Repo` port + `Service` — add three methods:**
```go
Create(ctx, NewUser) (User, error)         // INSERT, RETURNING id,email,name,avatar_url
Update(ctx, id string, UserPatch) error    // dynamic partial UPDATE
FindBySocialId(ctx, provider, socialID string) (User, error)  // WHERE {provider}_id = $1
```
`NewUser` and `UserPatch` are exported structs in `domain.go`. `UserPatch` uses pointer/optional fields so a field omitted = column untouched (mirrors TypeORM partial update). `auth` consumes these via its existing `UserService` interface (extend it).

**`FindBySocialId` column mapping:** provider→column is a fixed switch (`google→google_id`, `facebook→facebook_id`, `tiktok→tiktok_id`, `apple→apple_id`). NEVER interpolate the provider into SQL — one prepared query per provider (4 sqlc queries) or a switch selecting among them. The `{provider}_id` columns each have a UNIQUE constraint.

### 4.2 `internal/plans` (NEW, flat)

```go
type Plan struct{ ID string; Name string; DailyLimit int }
type Reader struct{ q sqlc.Querier }
func (r *Reader) FindFreePlan(ctx) (Plan, error)  // WHERE billing_period='none' AND is_active=true LIMIT 1
```
Node throws `"Free plan not found. Run seed first."` if absent — Go returns an error that surfaces as 500 `internal` (seed is a deploy invariant; this path is unreachable in prod, matches Node's uncaught `Error`).

### 4.3 `internal/subscription` (extend)

Add a **writer** alongside the Phase 1a `Reader`:
```go
type Writer struct{ q sqlc.Querier }
func (w *Writer) CreateFree(ctx, userID, planID string) error
// INSERT status='active', store_type='admin_granted', started_at=now(), expires_at=null
```
`auth` declares a tiny `FreeSubWriter` port (`CreateFree`) and a `FreePlanReader` port (`FindFreePlan`) in its `usecase.go`; `main.go` injects the concretes.

### 4.4 `internal/email` (NEW, flat → adapter sub-pkg)

Mirrors `email.service.ts`. **Functional parity, not gate-pinned** — email bodies go out-of-band (Resend), not in the HTTP shadow-compare. Port faithfully so users get the same codes/templates.

```
internal/email/
  service.go    // Service: SendVerification, SendPasswordReset (the 2 auth needs)
  templates.go  // built-in EMAIL_TEMPLATES map + {{var}} substitution
  resend.go     // Resend HTTP client (lazy-init, nil when no key)
  port.go       // Sender interface auth depends on; Suppressor + LogWriter ports
```

**`Service.deliver(to, subject, html, label, throwOnError)`** mirrors Node exactly:
1. `isSuppressed(to)` (count `email_suppressions WHERE email=lower(to)` > 0) → log `'suppressed'`, return (no throw at request level — auth fires-and-forgets).
2. Resolve creds: `app_settings.resend_api_key || env RESEND_API_KEY`, `from = app_settings.email_from || env EMAIL_FROM || "DraftRight <noreply@draftright.info>"`. No key → log `'skipped'`, return.
3. POST `https://api.resend.com/emails` `{ from, to, subject, html }` with `Authorization: Bearer <key>`. On error → log `'failed'`. On success → log `'sent'` with provider id.
4. Every attempt writes an `email_logs` row (best-effort; never throws).

**`auth` consumes only:**
```go
type EmailSender interface {
    SendVerification(ctx, to, name, code string)
    SendPasswordReset(ctx, to, name, code string)
}
```
Called **fire-and-forget** from the usecase (`go svc.email.SendVerification(...)` with its own context + recovered panic + slog on error). Auth NEVER awaits or surfaces email errors — matches Node's `.catch(log)`.

**Templates (subjects byte-exact):** verification `"Welcome to DraftRight — confirm your email"`, password-reset `"Reset your DraftRight password"`. Name fallback `name || "there"`. Built-in HTML ported from `email-templates.ts`; DB override (`email_templates` table) is supported (read by key, fall back to built-in). `{{var}}` substitution is literal string replace.

> **Resend client is a thin HTTP POST**, not the npm SDK — mirror the `emails.send` payload + response (`{ data: { id }, error: { message } }`).

### 4.5 `internal/auth` (extend)

Add to `usecase.go`: `Register`, `VerifyEmail`, `ResendVerification`, `ForgotPassword`, `ResetPassword`, `SocialLogin` methods + the new ports (`FreePlanReader`, `FreeSubWriter`, `EmailSender`, `SocialVerifier`). Extend `UserService` with `Create/Update/FindBySocialId`. Add to `handler.go`: 6 new `http.HandlerFunc`s. Add error constants to `errors.go`.

---

## 5. SQL queries (add to `queries_auth.sql`)

```sql
-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, auth_provider, avatar_url,
                   email_verified, email_verification_code, email_verification_expires,
                   google_id, facebook_id, tiktok_id, apple_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, email, name, avatar_url;

-- name: FindUserByGoogleId :one  (+ FindUserByFacebookId / ByTiktokId / ByAppleId)
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE google_id = $1 LIMIT 1;

-- name: UpdateUserVerification :exec
UPDATE users SET email_verified=$2, email_verification_code=$3,
                 email_verification_expires=$4, updated_at=now() WHERE id=$1;

-- name: SetEmailVerificationCode :exec
UPDATE users SET email_verification_code=$2, email_verification_expires=$3,
                 updated_at=now() WHERE id=$1;

-- name: SetPasswordResetCode :exec
UPDATE users SET password_reset_code=$2, password_reset_expires=$3,
                 password_reset_attempts=$4, updated_at=now() WHERE id=$1;

-- name: SetPasswordResetAttempts :exec
UPDATE users SET password_reset_attempts=$2, updated_at=now() WHERE id=$1;

-- name: ClearPasswordReset :exec
UPDATE users SET password_reset_code=null, password_reset_expires=null,
                 password_reset_attempts=0, updated_at=now() WHERE id=$1;

-- name: ResetPasswordHash :exec  (password + clear reset fields, single statement)
UPDATE users SET password_hash=$2, password_reset_code=null,
                 password_reset_expires=null, password_reset_attempts=0,
                 updated_at=now() WHERE id=$1;

-- name: LinkSocialGoogle :exec  (+ Facebook/Tiktok/Apple)
UPDATE users SET google_id=$2, avatar_url=$3, email_verified=true,
                 updated_at=now() WHERE id=$1;

-- name: CreateFreeSubscription :exec
INSERT INTO subscriptions (user_id, plan_id, status, store_type, started_at, expires_at)
VALUES ($1, $2, 'active'::subscriptions_status_enum,
        'admin_granted'::subscriptions_store_type_enum, now(), null);

-- name: FindFreePlan :one
SELECT id, name, daily_limit FROM plans
WHERE billing_period='none'::plans_billing_period_enum AND is_active=true LIMIT 1;

-- name: IsEmailSuppressed :one
SELECT COUNT(*) FROM email_suppressions WHERE email=$1;

-- name: InsertEmailLog :exec
INSERT INTO email_logs (to_email, email_type, subject, status, provider_id, error)
VALUES ($1,$2,$3,$4,$5,$6);
```

The verification/reset reads need extra columns on the `GetAuthUserByEmail` projection (`email_verification_code`, `email_verification_expires`, `password_reset_code`, `password_reset_expires`, `password_reset_attempts`, `auth_provider`, `avatar_url`). **Add a dedicated `GetUserAuthState` query** rather than widening the login projection, so Phase 1a's login path is untouched.

---

## 6. Social verifiers (`internal/auth/social.go` + adapter)

`SocialVerifier` is a port `auth` declares; the concrete makes real HTTP calls. **Fakeable** so usecase tests never hit the network.

```go
type SocialProfile struct{ SocialID, Email, Name, AvatarURL string; EmailVerified bool }
type SocialVerifier interface {
    Verify(ctx, provider, idToken string, profile InboundProfile) (SocialProfile, error)
}
```

Per-provider (mirrors `auth.service.ts` exactly):

- **Google:** `GET https://oauth2.googleapis.com/tokeninfo?id_token=<t>`. `!ok` → 401 `"Invalid Google token"`. `socialId=sub, email, name, avatar=picture, emailVerified = (email_verified==true || email_verified=="true")` (tokeninfo returns the string `"true"`).
- **Facebook:** `GET https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=<t>`. `!ok` → 401 `"Invalid Facebook token"`. `socialId=id, avatar=picture.data.url, emailVerified = (email != "")`.
- **Apple:** decode JWT header for `kid` (no kid → 401 `"Invalid Apple token (no kid)"`). Fetch JWKS `https://appleid.apple.com/auth/keys` with a **package-level 24h cache** (mutex-guarded; mirrors Node's `static _appleJwksCache`). Fetch `!ok` → 401 `"Could not fetch Apple signing keys"`. kid not in cache → **bust cache + 401 `"Apple key not found for kid=<kid>"`** (Node busts and throws; no retry loop despite the comment). Verify RS256, issuer `https://appleid.apple.com`, audience from `env APPLE_AUDIENCES` (default `com.draftright.draftrightMobile.v2,com.draftright.app.v2`, comma-split + trim). Verify fail → 401 `"Invalid Apple token"`. No `sub` → 401 `"Apple token missing sub claim"`. `email = token.email || profile.email || ""`; `emailVerified = token.email != "" && (token.email_verified==true || =="true")`; `name = profile.name`.
- **TikTok:** no crypto verify. `socialId = idToken` (open_id), `email = profile.email || ""`, `name/avatar = profile.*`, `emailVerified = false`.

JWKS → RSA public key: parse the JWK `n`/`e` (base64url) into `*rsa.PublicKey` with stdlib `crypto/rsa` + `math/big` (no `createPublicKey` equivalent needed; golang-jwt verifies against `*rsa.PublicKey`). Use `golang-jwt/jwt/v5` `ParseWithClaims` + `jwt.NewParser(jwt.WithValidMethods(["RS256"]), jwt.WithIssuer(...), jwt.WithAudience(...))`. **clockTolerance stays zero** (parity invariant).

All 401 messages use code `invalid-token`; the 400s (`Unsupported provider`, `Email is required`) use `invalid-input`.

---

## 7. Token / code machinery

- **6-digit code:** CSPRNG. `n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000)); code := fmt.Sprintf("%06d", n)` — exactly Node `randomInt(0,1_000_000).toString().padStart(6,'0')`. Stored **plaintext** in `email_verification_code` / `password_reset_code` (matches Node; the column is `varchar(6)`).
- **TTL:** `EMAIL_CODE_TTL_MS = 15 min`. `expires = now + 15*time.Minute`.
- **Reset throttle:** `MAX_RESET_ATTEMPTS = 5`. Burn-on-exhaustion (§3.5).
- **`generateTokens`:** unchanged from Phase 1a (dual-secret, TTLs from `app_settings`, claims `{sub,email,role}`, clockTolerance 0).

---

## 8. Validation parity (register DTO)

Node's global `ValidationPipe({ whitelist: true, transform: true })` validates `RegisterDto` and the `AllExceptionsFilter` humanizes the messages. Go must reproduce the **common** cases byte-exact:

| Constraint failure | Node humanized message |
|---|---|
| `email` not an email | `Please enter a valid email address.` |
| `password` length < 8 | `Password must be at least 8 characters.` |
| `name` length < 1 (empty) | `Name must be at least 1 characters.` |
| (missing/non-string field) | falls through to the raw constraint string |

Multiple failures joined with `". "`, in **property-declaration order** (email, password, name), and within a property in constraint-registration order. Code `invalid-input`, status 400.

**Approach:** a small `validateRegister(email,password,name) []string` in the auth handler returns humanized messages in the exact order; empty slice = valid. This is a **known parity risk** (class-validator's exact multi-constraint ordering for exotic inputs) — pinned by the shadow gate against real Node responses, same posture as Phase 1a's timestamp-format risk. Real clients send well-formed payloads or single-field errors; those are covered exactly.

> verify-email / resend / forgot / reset / social use **inline body types** (no DTO) in Node — no class-validator, so Go does plain JSON decode with no field validation beyond the explicit checks in §3.

---

## 9. Schema snapshot patch (`internal/platform/db/schema.sql`)

The read-only Go schema mirror is **stale** vs prod. sqlc compiles new queries against it, so it must be patched (these objects already exist in prod — NO prod migration; this only updates the local mirror so `sqlc generate` succeeds):

1. Add `'apple'` to `users_auth_provider_enum`.
2. Add to `public.users`: `apple_id varchar(255)`, `password_reset_code varchar(6)`, `password_reset_expires timestamptz`, `password_reset_attempts integer DEFAULT 0 NOT NULL`.
3. Add table `public.email_suppressions (id uuid, email varchar UNIQUE, reason varchar, created_at timestamptz)`.

> Ideally refresh via `scripts/dump-prod-schema.sh`, but that needs prod DB access. Hand-patch is acceptable and the diff is reviewable; note in the commit that prod already has these.

---

## 10. Composition root (`cmd/server/main.go`)

Inside the existing `if pool != nil` block, after the Phase 1a wiring:
```go
plansReader  := planspkg.NewReader(q)
subWriter    := subpkg.NewWriter(q)
emailSvc     := emailpkg.NewService(q, cfg)        // nil-client when no Resend key
socialVer    := authpkg.NewHTTPSocialVerifier(cfg) // real HTTP; APPLE_AUDIENCES from cfg
// userSvc gains Create/Update/FindBySocialId (same concrete, extended)
authSvc := authpkg.NewService(userSvc, subReader, usageCounter, ttlReader,
    cfg.JWTSecret, cfg.JWTRefreshSecret,
    plansReader, subWriter, emailSvc, socialVer)   // extended constructor
authHandler := authpkg.NewHandler(authSvc)
core.Register            = http.HandlerFunc(authHandler.Register)
core.VerifyEmail         = http.HandlerFunc(authHandler.VerifyEmail)
core.ResendVerification  = http.HandlerFunc(authHandler.ResendVerification)
core.ForgotPassword      = http.HandlerFunc(authHandler.ForgotPassword)
core.ResetPassword       = http.HandlerFunc(authHandler.ResetPassword)
core.Social              = http.HandlerFunc(authHandler.Social)
```
Add the 6 route fields to `coreHandlers` and mount them in `shared/router.go` (all public, no JWT middleware).

`NewService` grows 4 params — acceptable for the composition root. If it gets unwieldy later, promote to an options struct (YAGNI for now).

---

## 11. Testing

- **Usecase tests** (no DB, fakes for every port) cover each endpoint's branch order: register happy/conflict/validation; verify-email all 3 bad-paths + success; resend silent paths; forgot silent paths (unknown/social-only); reset throttle (attempts 1..5, burn at 5) + success; social: each provider via a **fake `SocialVerifier`**, social-id hit, email-link with/without verified (takeover guard), create path + free-sub, disabled.
- **Email module tests:** suppression short-circuit, no-key skip, template `{{var}}` substitution, subjects exact.
- **Social verifier tests:** Apple JWKS parse + RS256 verify against a **test-generated RSA key** (sign a token in-test, verify it); audience/issuer mismatch → error; kid-miss cache-bust. Google/Facebook via an injected HTTP client hitting a stub. **Live provider tokens stay operator-pending** (shadow gate, not unit).
- **Handler tests:** status codes (201/200), JSON shapes, error envelope `{error,code,request_id}`.
- `go test ./... -race`, `gofmt -l .`, `go vet ./...`, `sqlc generate` clean.

---

## 12. Parity invariants & risks

**Invariants (gate-pinned):**
- bcrypt cost 10; email `lower(trim)` everywhere except login.
- Exact error strings + status codes per §3/§6.
- JWT clockTolerance 0; dual-secret; claims `{sub,email,role}`.
- 6-digit CSPRNG plaintext codes, 15m TTL, reset cap 5 burn-on-exhaustion.
- Register response `email_verified:false` literal; social response trimmed `{id,email,name}`.

**Risks (pinned by shadow gate, not unit-provable here):**
- Class-validator multi-constraint **message ordering** for exotic register payloads (§8).
- Apple JWKS RS256 + external HTTP — verifiers fakeable; **live social shadow-verify operator-pending**.
- Email HTML byte-exactness is NOT gate-checked (out-of-band); ported for functional parity only.

**Deferred to Phase 4:** `POST /webhooks/resend` (Svix verify + suppression writes), admin email-log/template UIs. Go reads `email_suppressions`; Node's webhook keeps it fresh during parallel run.

---

## 13. Ships nothing to prod

Phase 1b, like 1a, ships **no** prod change: no Caddy flip, no live traffic. The live shadow gate remains operator-pending. Output is Go code on a feature branch → review → merge to `develop`.
