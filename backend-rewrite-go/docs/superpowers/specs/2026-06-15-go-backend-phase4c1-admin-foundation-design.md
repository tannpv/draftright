# Phase 4c-1 — Admin Foundation (admin-auth + admin-role guard + listquery) Design

**Status:** approved (brainstorm 2026-06-15)
**Branch:** `feature/go-backend-phase4c1-admin-foundation-20260615`
**Parity authority:** NestJS `backend/src` — Node wins on any disagreement.

## Goal

Port the foundation the rest of the NestJS admin surface depends on, as a
byte-identical drop-in: the 3 `admin/auth` endpoints, the admin-role gate
(`RolesGuard`), and the shared dynamic list-query helper
(`parseListQuery`/`applyListQuery`). Phase 4c-2/3/4 (CRUD, reporting, AI
fix-proposal+cron) build on this and ship in their own spec→plan→implement
cycles.

## Scope

In 4c-1:
- `POST /admin/auth/login` (public)
- `POST /admin/auth/change-password` (admin-guarded)
- `GET /admin/auth/me` (admin-guarded)
- `RequireAdmin` middleware (the `RolesGuard('admin')` equivalent)
- `internal/shared/listquery` package (infra; first consumer is 4c-2)

Out of scope (later sub-plans): every route on `AdminController` (55 endpoints).

## Decomposition (Phase 4c, by capability)

1. **4c-1 (this spec)** — admin-auth + admin-role guard + listquery foundation.
2. 4c-2 — content/ops CRUD (users, plans, admin-users, settings, ai-providers,
   email-templates/logs).
3. 4c-3 — reporting (stats, analytics, transactions, training-data, payments admin).
4. 4c-4 — AI fix-proposal + suggest-fix + hourly cron.

## Module layout

```
internal/adminauth/                NEW flat clean-arch slice
  domain.go      AdminUser type + sentinel errors
  usecase.go     Service (Login/ChangePassword/GetProfile) + Repo + Minter ports
  repo_pg.go     NewPgRepo → FindByEmailLower / FindByID / UpdatePasswordHash
  handler.go     3 HTTP handlers
  *_test.go      fakes, no DB

internal/shared/listquery/         NEW pure helper (no DB)
  listquery.go   Query, Parse(url.Values), Built, Build(...)
  listquery_test.go

internal/platform/auth/jwt.go      EXTEND: add isAdmin claim field
internal/platform/auth/admin.go    NEW: RequireAdmin middleware (+ test)
internal/shared/pg/queries_adminauth.sql   NEW sqlc queries (3)
internal/shared/router.go          mount 3 routes (1 public, 2 admin-guarded)
cmd/server/main.go                 wire adminauth + access/refresh signers + RequireAdmin
internal/platform/config/config.go reuse existing IsProduction() (no change expected)
```

Admin tokens are signed with the **same `JWT_SECRET`** as customer access
tokens, so the existing `auth.Verifier` validates signature + expiry already.
4c-1 only widens decoded claims with `isAdmin` and adds the role gate — no
second verifier, no change to customer-token behavior.

## Admin-role guard

Node chains `JwtAuthGuard` (passport-jwt) then `RolesGuard`:

```ts
// RolesGuard for @Roles('admin')
if (requiredRoles.includes('admin') && user.isAdmin) return true;   // isAdmin flag
if (!requiredRoles.includes(user.role)) throw new ForbiddenException('Admin access required');
return true;                                                        // role === 'admin'
```

Net allow condition = `user.isAdmin === true || user.role === 'admin'`; else 403.

**Go two-stage gate** on the admin routes:
1. **Stage 1 — `RequireAuth`** (existing access-token middleware). No/expired/
   malformed/bad-signature token → its existing 401 envelope
   (`code:"invalid-token"`). Unchanged.
2. **Stage 2 — `RequireAdmin`** (new). Reads `*auth.Claims` from context.
   Allow iff `claims.IsAdminFlag || claims.Role == "admin"`. Otherwise:
   `shared.WriteError(w, r, "forbidden", "Admin access required")`.

**403 envelope is `code:"forbidden"`, NOT `http-403`.** A bare
`ForbiddenException('Admin access required')` body carries no `code`, so the
filter calls `inferCode(403)` → `ERROR_CODES.forbidden` = `"forbidden"`
(`all-exceptions.filter.ts:171`). Go `shared.StatusForCode("forbidden")`
already returns 403 (`errenvelope.go:27-28`). Envelope:
`{error:"Admin access required", code:"forbidden", request_id}`.

### Claims widening

`auth.Claims` currently has `Sub`, `Email`, `Role`. Node's admin payload is
`{sub, email, role:'admin', isAdmin:true}`. Add:

```go
IsAdminFlag bool `json:"isAdmin,omitempty"`
```

Customer tokens lack `isAdmin` → decodes `false`; with `role:"user"` they fail
the allow condition → 403 on admin routes, exactly as Node. The existing
`Claims.IsAdmin()` method (role-only) is left intact; `RequireAdmin` checks
`IsAdminFlag || Role=="admin"` to mirror the guard's two branches precisely.

## Admin-auth service parity

All three handlers read the raw JSON body with `json.Unmarshal` (NO
`DisallowUnknownFields`): Node's controller binds `@Body() body: {email; password}`
— an inline type literal with no class-validator metatype, so the global
ValidationPipe does not whitelist it. Unknown keys are ignored by both.

### `POST /admin/auth/login` → **201**

NestJS `@Post('login')` with no `@HttpCode` → 201 (same as customer login).

1. `LOWER(email) = LOWER($1)` on `strings.TrimSpace(email)`.
2. Not found → 401 `"Invalid credentials"`. Bad bcrypt verify → 401
   `"Invalid credentials"`. `!is_active` → 401 `"Account disabled"`.
   Order: identity/password checked before is_active (matches Node).
   All three: `code:"invalid-token"` (UnauthorizedException body has no code →
   `inferCode(401)` = `invalid-token`).
3. Mint payload `{sub:id, email, role, isAdmin:true}`:
   - **access** signed `JWT_SECRET`, TTL `15m if config.IsProduction() else 24h`.
   - **refresh** signed `JWT_REFRESH_SECRET`, TTL `7d` always.
   - ⚠️ TTLs are hardcoded by environment — NOT the `app_settings`
     token_expiry values customer tokens use.
4. Response (ordered struct):
   `{access_token, refresh_token, user:{id, email, name, role}}`.

bcrypt verify via `x/crypto/bcrypt` (existing `$2a/$2b` hashes verify natively).

### `POST /admin/auth/change-password` → **201**

Admin-guarded. Body `{current_password, new_password}`.
1. Load admin by `req.user.id` (the `sub` claim). Not found → 401 `"Unauthorized"`
   (bare `UnauthorizedException()` → NestJS default message `"Unauthorized"`,
   `code:"invalid-token"`).
2. Wrong current password → 401 `"Current password is incorrect"`
   (`code:"invalid-token"`).
3. bcrypt-hash `new_password`, `UPDATE admin_users SET password_hash=$2 WHERE id=$1`.
4. Response **201** `{success:true}`.

### `GET /admin/auth/me` → **200**

Admin-guarded. Load admin by `sub`. Not found → 401 `"Unauthorized"`
(`code:"invalid-token"`). Success: AdminUser minus `password_hash`, key order =
entity decorator order:

```
{ id, email, name, is_active, role, created_at, updated_at }
```

`created_at`/`updated_at` serialized via `shared.ISOMillis` — `Date.toISOString()`
parity, actual fractional millis (`YYYY-MM-DDTHH:mm:ss.SSSZ`, UTC). Ordered
struct, never `map[string]any`.

### Service ports (consumer-side)

```go
// in adminauth/usecase.go
type Repo interface {
    FindByEmailLower(ctx context.Context, email string) (*AdminUser, error) // nil,nil if none
    FindByID(ctx context.Context, id string) (*AdminUser, error)            // nil,nil if none
    UpdatePasswordHash(ctx context.Context, id, hash string) error
}
type Minter interface {                        // satisfied by a small main.go adapter
    MintAccess(claims auth.Claims) (string, error)  // chooses 15m/24h via IsProduction
    MintRefresh(claims auth.Claims) (string, error) // 7d, refresh secret
}
```

`AdminUser` carries `password_hash` for verification; the handler strips it
when shaping `me`/`login.user`.

## listquery contract (byte-exact vs `common/list-query.ts`)

```go
type Query struct {
    Search    string  // "" when absent
    Status    string  // "active" | "inactive" | "all" | "" (unset)
    SortBy    string
    SortOrder string  // "ASC" | "DESC" | "" (unset)
    Page      *int    // nil when absent (distinguishes "absent" from 0)
    Limit     *int
}

func Parse(v url.Values) Query

type Built struct {
    Where  string  // "" or "WHERE ..." (parameterized $1..$n)
    Args   []any
    Order  string  // "ORDER BY <field> ASC|DESC"
    Limit  int
    Offset int
}

func Build(q Query, searchCols []string, sortAllow map[string]string,
           defaultSort, statusCol string) Built
```

**`Parse` mirrors `parseListQuery`:**
- `search` → value if the param is present, else "".
- `status` → only `active|inactive|all`, otherwise unset ("").
- `sort_by` → value.
- `sort_order` → uppercased; only `ASC|DESC`, otherwise unset.
- `page`/`limit` → **JS `parseInt(str,10)` semantics** via a local
  `jsParseInt(string) (int, bool)` helper: leading optional sign + digits,
  stop at first non-digit (`"12abc"`→12, `"  3"`→3, `"2.5"`→2, `"abc"`→absent).
  `strconv.Atoi` is WRONG (errors on `"12abc"`). Absent/empty → nil pointer.
  Same gotcha already handled in `feedback` pagination.

**`Build` mirrors `applyListQuery`:**
- **Search:** `strings.TrimSpace(Search) != "" && len(searchCols) > 0` →
  `(c0 ILIKE $k OR c1 ILIKE $k ...)`, single arg `"%"+trimmed+"%"` (one
  placeholder reused across columns; Postgres permits `$k` reuse). Columns are
  caller-supplied literals (`alias.field`), never user input.
- **Status:** `statusCol != "" && Status != "" && Status != "all"` →
  `statusCol = $k` with bool arg (`active`→true, `inactive`→false).
- **Sort:** `field := sortAllow[SortBy]`; if empty → `defaultSort`. `order :=
  "DESC"`; `if SortOrder == "ASC" { order = "ASC" }` (default DESC). Field comes
  only from the allow-list map → **injection-safe, never parameterized**. A
  `sort_by` not in the map silently falls to `defaultSort` (matches Node).
- **Page/limit:** `page := max(1, deref(Page, 1))` where a nil/zero page → 1
  (`query.page || 1`); `limit := min(100, max(1, deref(Limit, 10)))`
  (**default 10**, cap 100). `Offset = (page-1)*limit`. Limit/Offset are ints,
  inlined into SQL (safe).

**Caller composition** (TypeORM `getManyAndCount` = 2 queries sharing the WHERE):
```
rows  : SELECT <cols> FROM <base/joins> {Where} {Order} LIMIT {Limit} OFFSET {Offset}
total : SELECT count(*) FROM <base/joins> {Where}        -- ignores order/limit
```
Both use `Built.Args`. Returns `{rows, total}` shaped per resource in 4c-2.
Run via `pgxpool` directly (NOT sqlc) — the allow-listed dynamic sort cannot
be expressed as a static sqlc query. This is a deliberate, scoped deviation
from sqlc-only; the SQL is still fully parameterized for all user-derived values.

## sqlc queries (`queries_adminauth.sql`)

- `FindAdminByEmailLower :one` — `WHERE LOWER(email)=LOWER($1)` (all columns).
- `FindAdminByID :one` — `WHERE id=$1`.
- `UpdateAdminPasswordHash :exec` — `UPDATE admin_users SET password_hash=$2,
  updated_at=now() WHERE id=$1`.

`admin_users` columns (schema.sql:112-121): `id uuid`, `email varchar(255)`,
`password_hash varchar(255)`, `name varchar(255)`, `is_active bool default
true`, `role varchar(20) default 'admin'`, `created_at`, `updated_at` (both
`timestamp without time zone default now()`).

Note: Node's `update(adminId, {password_hash})` does NOT set `updated_at`
explicitly, but TypeORM `@UpdateDateColumn` auto-bumps it. The SQL mirrors that
with `updated_at=now()`.

## Router wiring

- `POST /admin/auth/login` → public block (no guard), like `/auth/login`.
- `POST /admin/auth/change-password`, `GET /admin/auth/me` → behind
  `RequireAuth` then `RequireAdmin`.
- All handler fields nil-guarded (house pattern).

`main.go` `composeDeps` (inside `pool != nil`): build `adminauth` repo →
service (inject Repo + a Minter adapter wrapping access `Signer(JWT_SECRET)` +
refresh `Signer(JWT_REFRESH_SECRET)` + `cfg.IsProduction()`) → handler. Build
`RequireAdmin` from the access verifier's claims-in-context. Assign route
fields. No concrete constructor named outside main.go.

## Error handling summary

| Case | Status | error | code |
|---|---|---|---|
| login bad creds / no user | 401 | `Invalid credentials` | `invalid-token` |
| login disabled | 401 | `Account disabled` | `invalid-token` |
| change-pw / me admin missing | 401 | `Unauthorized` | `invalid-token` |
| change-pw wrong current | 401 | `Current password is incorrect` | `invalid-token` |
| non-admin token on guarded route | 403 | `Admin access required` | `forbidden` |
| no/invalid token on guarded route | 401 | (existing RequireAuth envelope) | `invalid-token` |

## Known malformed-input divergence (documented, not fixed)

Node admin login binds an inline body type (no DTO). A request missing `email`
hits `email.trim()` on `undefined` → `TypeError` → 500 internal. Go will treat
a missing `email` as `""` → no match → 401 `"Invalid credentials"`. This is an
adversarial/malformed-only path the live shadow gate (well-formed traffic)
never exercises; faithful 500-reproduction is not worth a panic seam. Recorded
here for completeness, consistent with prior phases' malformed-input notes.

## Testing (TDD, table-driven, no DB)

- `listquery_test.go` — Parse coercion (status allow-list, sort_order upcase,
  jsParseInt `"12abc"`/`"abc"`/`"2.5"`/absent); Build (search OR-clause + single
  `%term%` arg, status bool, allow-list sort vs default, DESC default,
  page/limit clamp + default 10/cap 100, offset math, `sort_by` injection
  attempt → falls to defaultSort).
- `adminauth` handler/service tests with Repo+Minter fakes — login (bad creds /
  disabled / success: 201 + token + `user` shape + key order), change-password
  (missing admin 401 / wrong current 401 / success 201 `{success:true}`),
  getProfile (200, `password_hash` stripped, key order).
- `admin_test.go` — `RequireAdmin`: admin-flag claim passes; `role:"admin"`
  claim passes; customer claim → 403 `forbidden` `Admin access required`;
  chained after RequireAuth (no token → 401, not 403).
- Router test — `/admin/auth/login` reachable without JWT (non-401);
  `/admin/auth/me` + `change-password` → 401 without JWT.

## Gate

`go build ./...` · `go test ./... -race` · `gofmt -l .` · `go vet ./...` · run
`sqlc generate` and commit the in-sync generated code.
