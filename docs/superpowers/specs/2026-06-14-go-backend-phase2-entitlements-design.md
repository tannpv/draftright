# Go Backend Port — Phase 2: Entitlements Design

> Sub-spec of the master port design
> (`docs/superpowers/specs/2026-06-13-go-backend-port-design.md`).
> Scope chosen with the owner: **all three entitlement modules bundled** —
> `plans`, `subscription`, `extension_token` — built in one continuous
> subagent-driven run, byte-identical to the NestJS reference.

**Goal:** Port the three entitlement-bearing modules of the NestJS backend to
Go so the running Go server answers `GET /plans`, `GET /subscription`,
`POST /subscription/verify-receipt`, the `auth/extension-tokens` CRUD, AND
accepts `dr_ext_*` bearer tokens on `/v1/rewrite` — every byte (route, status,
JSON, error envelope) identical to Node.

**Non-goals:** No prod deploy, no Caddy change. The expiry cron is implemented
but is NOT shadow-gated (background job, not an HTTP surface). Receipt
verification against real stores is OUT (Node's `verify-receipt` is already a
body-ignoring no-op — we mirror that exactly, nothing more).

---

## 1. Parity contract (the byte-level truth, from the Node maps)

### 1.1 `plans` module — `backend/src/plans/`

| Property | Value |
|---|---|
| Route | `GET /plans` → **200** |
| Auth | none |
| Params/headers/body | none |
| Query | `find({ where: { is_active: true }, order: { price_cents: 'ASC' } })` |
| Body shape | **JSON array of RAW entities** — no ClassSerializer, no transform |

Row fields, snake_case, in entity-declaration order:

```
id            string (uuid)
name          string
daily_limit   integer
price_cents   integer            // RAW integer, NOT divided by 100
currency      string | null
stripe_price_id string | null
trial_days    integer
billing_period "none" | "monthly" | "yearly"
is_active     boolean            // always true on this route
created_at    string             // ISO-8601, ms precision, UTC "Z"
updated_at    string             // ISO-8601, ms precision, UTC "Z"
```

No tiebreaker beyond `price_cents ASC`. Throws nothing. Typical 5 rows
(Free 0, USD monthly 499, USD yearly 3999, VND monthly 99000, VND yearly
799000).

### 1.2 `subscription` module — `backend/src/subscriptions/`

Controller `@Controller('subscription')`. `req.user` = `{id,email,role,isAdmin}`.

**`GET /subscription`** (JWT) → **200**:

```jsonc
{
  "plan": { "name": string, "daily_limit": int, "billing_period": string } | null,
  "status": string | null,            // active sub's status, else null
  "expires_at": string | null,        // ISO, ms, Z — else null
  "usage_today": int,                 // count of usage_logs today (server-local midnight)
  "nudge": {
    "tier": "pro" | "free",
    "usageToday": int,                // camelCase — mirrors Node DTO
    "dailyLimit": int,
    "expiresAt": string | null,       // ISO, ms, Z
    "banner": "none" | "pro_expiring" | "just_expired" | "free_counter"
  }
}
```

- Active sub = `findOne({ where:{ user_id, status:ACTIVE }, relations:['plan'],
  order:{ created_at:'DESC' } })`. **`status='active'` ONLY** — `expires_at`
  is NOT in the query.
- Pro vs Free = `plan.billing_period !== 'none'`.
- Free `dailyLimit` fallback constant = **10**.
- `usage_today` = `countTodayByUser` = `usage_logs` rows with
  `created_at >= local-midnight` (SERVER timezone).

**`deriveBanner`** (`subscriptions/nudge.ts`):

```
DAY_MS = 86_400_000
PRO_EXPIRING_DAYS = 3
JUST_EXPIRED_DAYS = 7
enum NudgeBanner { NONE='none', PRO_EXPIRING='pro_expiring',
                  JUST_EXPIRED='just_expired', FREE_COUNTER='free_counter' }

deriveBanner(entitlement, _usageToday /*IGNORED*/, lastExpiredAt: Date|null, now: Date):
  if tier == 'pro':
    if expiresAt != null:
      daysLeft = (expiresAt - now) / DAY_MS        // unrounded float
      if daysLeft <= 3: return 'pro_expiring'      // inclusive <=
    return 'none'
  // free
  if lastExpiredAt != null:
    daysSince = (now - lastExpiredAt) / DAY_MS      // unrounded float
    if daysSince <= 7: return 'just_expired'        // inclusive <=
  return 'free_counter'
```

`lastExpiredAt` = `updated_at` of the most-recent **EXPIRED** sub row for the
user (null if none).

**`POST /subscription/verify-receipt`** (JWT) → **201** (default POST, NO
`@HttpCode`). **Body ignored entirely** — no validation, no provider call, no
write. Returns:

```jsonc
{ "subscription": { "plan": string|undefined, "status": string, "expires_at": string|null } | null }
```

`sub` resolved by the same `findActiveByUserId`. `plan` = `sub.plan?.name`
(omitted/undefined when plan relation missing — JSON omits the key).

**Expiry cron** (`subscriptions.cron.ts`): `@Cron(EVERY_DAY_AT_9AM)` = cron
expr `"0 09 * * *"`, server timezone.
- Pass 1 `sendRenewalReminders`: ACTIVE subs with `expires_at` in
  `[now+2.5d, now+3.5d]` → renewal-reminder email. No mutation.
- Pass 2 `expireLapsedSubscriptions`: ACTIVE or CANCELLED subs with
  `expires_at < now` → bulk `UPDATE status='expired'` (expires_at UNTOUCHED)
  → expired email per row.
- Notifier best-effort: never throws, never blocks the status flip.

`subscriptions` table: `id, user_id, plan_id, status enum(active|cancelled|
expired), store_type enum(google_play|apple_iap|admin_granted|lemonsqueezy|
stripe|vietqr|bank_transfer|paypal), store_transaction_id varchar(500) null,
started_at, expires_at null, created_at, updated_at`.

### 1.3 `extension_token` module — `backend/src/auth/` (`auth/extension-tokens`)

Entire controller JWT-guarded.

**`POST /auth/extension-tokens`** (mint) → **200** (explicit `@HttpCode(200)`).
Body DTO `MintExtensionTokenDto`:
- `device_id` `@IsUUID('4')`
- `device_name` `@IsString @Length(1,64) @Matches(/^[A-Za-z0-9 _.\-]+$/)`,
  message `'device_name must be alphanumeric/space/_/./- only'`

Pre-mint: rotate (revoke) existing active token for `(user, device_id)`.
Returns ONLY:

```jsonc
{ "token": "dr_ext_<43 base64url chars>", "id": "<uuid>" }
```

Raw token shown ONLY here. Format: prefix `dr_ext_` +
`crypto.randomBytes(32).toString('base64url')` (43 chars, no padding) = **50
chars total**. Storage: `sha256(FULL token incl prefix)` → hex 64 chars. Raw
NOT stored. **No expiry / TTL.**

**`GET /auth/extension-tokens`** (list) → **200**. Active-only
(`revoked_at IS NULL`), `order created_at DESC`, each row
`rows.map(({token_hash, user_id, ...rest}) => rest)` — strips `token_hash` +
`user_id`, keeps:

```jsonc
{ "id", "scopes": ["rewrite"], "device_id", "device_name",
  "last_used_at": string|null, "created_at", "revoked_at": null }
```

**`DELETE /auth/extension-tokens/:id`** → **204**, silent + idempotent. 204 for
ANY id (unknown / foreign / valid) — NO 404, NO 403. Scoped to the caller's
own rows on the UPDATE (`user_id = $caller AND id = $id`), but the response is
204 regardless of rows affected.

**Rewrite-path verification** (`rewrite-auth.guard.ts`, guards `POST /rewrite`
in Node; our Go route is `POST /v1/rewrite`): Bearer token →
- starts with `dr_ext_` → ext-token path: `sha256(token)` lookup, row must
  have `revoked_at IS NULL` (no expiry check), `scopes` must include
  `'rewrite'`, fire-and-forget `last_used_at` update. Errors (all **401**):
  `"Missing bearer token"`, `"Invalid extension token"`,
  `"Token missing rewrite scope"`.
- else → existing JWT path (unchanged).

`extension_tokens` table (migration `2026-05-02`): `id, user_id,
token_hash CHAR(64), scopes TEXT[] default ['rewrite'], device_id,
device_name varchar(64) default 'mobile', last_used_at null, created_at,
revoked_at null`. Unique partial indexes: `(token_hash) WHERE revoked_at IS
NULL` and `(user_id, device_id) WHERE revoked_at IS NULL`.

---

## 2. Go architecture (per CLAUDE.md package-by-feature)

Three feature modules, each a flat clean-arch slice. Reuse existing
collaborators — do NOT fork (`plans.Reader`, `subscription.Reader/Writer`,
`usage.Counter` already exist from Phase 1a).

### 2.1 `internal/plans/` (extend)

Already has `reader.go` (`FindFreePlan`). Add:
- `domain.go`: `type Plan struct{…}` with **explicit JSON tags** in the exact
  snake_case field order + a marshaller producing ms-precision UTC `Z`
  timestamps (reuse the shared ISO formatter used by Phase 1a account view).
- `usecase.go`: `Service.ListActive(ctx) ([]Plan, error)` → repo
  `ListActivePlans` (`WHERE is_active=true ORDER BY price_cents ASC`).
- `repo_pg.go`: `ListActivePlans` query (sqlc) + row→domain map. `price_cents`
  stays raw int. `currency`/`stripe_price_id` are `*string` (null → JSON
  `null`).
- `handler.go`: `List(w,r)` → 200, `shared.WriteJSON(w,200,plans)` where empty
  slice still serialises `[]` (NOT `null` — allocate non-nil slice).

### 2.2 `internal/subscription/` (extend)

Has `reader.go` (`ActiveByUser`), `writer.go` (`CreateFree`). Add:
- `domain.go`: `NudgeBanner` string consts; `Nudge` struct (camelCase JSON
  tags `tier,usageToday,dailyLimit,expiresAt,banner`); `SubscriptionView`
  struct (`plan,status,expires_at,usage_today,nudge`); `DAY_MS`,
  `PRO_EXPIRING_DAYS=3`, `JUST_EXPIRED_DAYS=7`, `FreeDailyLimit=10` consts;
  pure `DeriveBanner(tier, expiresAt *time.Time, lastExpiredAt *time.Time,
  now time.Time) NudgeBanner`.
- `usecase.go`: extend `Service` with:
  - `GetSubscription(ctx, userID) (SubscriptionView, error)` — ActiveByUser +
    usage.Counter.CountToday + lastExpired lookup + DeriveBanner.
  - `VerifyReceipt(ctx, userID) (ReceiptView, error)` — ignores body, returns
    `{subscription: …}` shape. ReceiptView marshals `plan` as omitempty
    pointer to mirror `sub.plan?.name` (omit key when nil).
  - New ports: `LastExpiredAt(ctx,userID) (*time.Time, error)` on the
    subscription repo; `usage.Counter` already injected pattern from auth.
- `repo_pg.go`: `ActiveByUserWithPlan` (joins plan name/daily_limit/
  billing_period, status='active', ORDER created_at DESC LIMIT 1),
  `LastExpiredAt` (status='expired' ORDER updated_at DESC LIMIT 1 → updated_at).
- `cron.go`: `ExpiryCron` struct + `RunOnce(ctx,now)` doing Pass 1 + Pass 2
  exactly as Node. Repo methods `DueForRenewal(now)` (active, expires_at in
  [now+2.5d, now+3.5d]) and `ExpireLapsed(now)` (active|cancelled,
  expires_at<now → UPDATE status='expired' RETURNING rows for emails).
  Notifier port best-effort (errors swallowed + logged).
- `handler.go`: `Get(w,r)` 200, `VerifyReceipt(w,r)` **201**.
- `scheduler.go` (in `cmd/server` or `internal/platform`): a `time`-based
  daily-at-09:00 server-local trigger calling `ExpiryCron.RunOnce`. NO new
  dependency — a goroutine that computes next 09:00 and `time.Timer`s to it,
  re-arming each fire. (robfig/cron avoided — one job, YAGNI.)

### 2.3 `internal/exttoken/` (new module)

- `domain.go`: `TokenRow` (id, scopes, device_id, device_name, last_used_at,
  created_at, revoked_at) with the list-view JSON tags; sentinel errors;
  `const TokenPrefix = "dr_ext_"`; `MintResult{Token,ID string}`.
- `usecase.go`: `Service`:
  - `Mint(ctx, userID, deviceID, deviceName) (MintResult, error)` — generate
    32 CSPRNG bytes → base64url-no-pad → prefix → sha256(full) hex → revoke
    existing active `(user,device)` → insert → return raw token + id.
  - `List(ctx, userID) ([]TokenRow, error)` — active only, created_at DESC.
  - `Revoke(ctx, userID, id) error` — UPDATE revoked_at where user+id+active;
    idempotent (no error on 0 rows).
  - `Verify(ctx, rawToken) (userID string, err error)` — sha256 lookup, active,
    scopes∋'rewrite', fire-and-forget last_used_at. Error sentinels mapping to
    the three 401 messages.
  - Port `Repo` (insert/revoke-by-device/list/revoke-by-id/find-by-hash/
    touch-last-used). Port `Clock`/CSPRNG injected for tests (or accept a
    `randRead func([]byte)` + `now func() time.Time`).
- `repo_pg.go`: the SQL. `scopes TEXT[]`.
- `validate.go`: `ValidateMint(deviceID, deviceName) string` — UUIDv4 check +
  length 1..64 + regex `^[A-Za-z0-9 _.\-]+$`, returning the exact Node message
  on the regex failure and class-validator-style messages otherwise
  (joined per the Phase 1a `validate.go` convention).
- `handler.go`: `Mint(w,r)` **200**, `List(w,r)` 200, `Revoke(w,r)` **204**.
  All read `claims.Sub` from context (JWT-guarded group).
- `auth_rewrite.go` (or extend rewrite transport): an `ExtTokenVerifier`
  port consumed by the rewrite auth middleware so `Bearer dr_ext_*` resolves
  to a user before falling through to JWT. Wired in `main.go`.

### 2.4 Router + composition (`shared/router.go`, `cmd/server/main.go`)

- Public: `mux.Method(GET, "/plans", r.Plans)`.
- JWT group additions: `GET /subscription`, `POST /subscription/verify-receipt`,
  `POST /auth/extension-tokens`, `GET /auth/extension-tokens`,
  `DELETE /auth/extension-tokens/{id}`.
- Rewrite middleware: before `RequireAuth`, a dual-auth shim — if
  `Authorization: Bearer dr_ext_…`, call `ExtTokenVerifier.Verify`, inject the
  resolved user into the same context shape `RequireAuth` would, and skip JWT;
  else run `RequireAuth` unchanged. Keep the existing JWT path byte-identical.
- All new `Router` fields nil-guarded like the Phase 1b additions.

---

## 3. Status-code & shape cross-check (the easy-to-miss table)

| Route | Method | Status | Body |
|---|---|---|---|
| `/plans` | GET | **200** | raw entity array (`[]` when empty) |
| `/subscription` | GET | **200** | view object (plan nullable) |
| `/subscription/verify-receipt` | POST | **201** | `{subscription:…\|null}` |
| `/auth/extension-tokens` | POST | **200** | `{token,id}` |
| `/auth/extension-tokens` | GET | **200** | stripped row array |
| `/auth/extension-tokens/:id` | DELETE | **204** | empty |
| `/v1/rewrite` (dr_ext_) | POST | unchanged | unchanged; 401s on bad ext-token |

Three deviations from REST intuition to NOT "fix": verify-receipt is **201**
(default POST), mint is **200** (explicit override), delete is **204** silent
(no 404 ever).

---

## 4. Testing

Per module, table-driven, fakes satisfy ports (no DB):
- **plans**: empty→`[]`, ordering by price_cents, null currency→JSON `null`,
  raw price_cents (no /100), ms-Z timestamp format.
- **subscription**: `DeriveBanner` truth table (all 4 banners + boundary
  `daysLeft==3.0`, `daysSince==7.0` inclusive); GET shape pro vs free vs
  no-sub (plan null, status null); verify-receipt 201 + `plan` key omitted
  when relation nil; cron RunOnce — renewal window inclusive bounds, expire
  flips active+cancelled only, notifier-throws-doesn't-abort.
- **exttoken**: mint token format (`dr_ext_` + 43 chars, total 50), sha256
  storage, device rotation revokes prior, list strips token_hash+user_id,
  delete idempotent 204 on unknown id, Verify three 401 messages + scope
  check + revoked rejected, validate.go regex message exact.
- **rewrite dual-auth**: `dr_ext_` valid → passes; revoked → 401 "Invalid
  extension token"; missing scope → 401 "Token missing rewrite scope"; plain
  JWT still works unchanged.

`go test ./... -race` green; `go build ./...` clean; `gofmt -l .` empty;
`go vet ./...` clean. pg repos gated by `go build ./...`.

## 5. Out of scope / operator-pending

- Live shadow gate vs dev Node (needs dev token + Go on dev DB) — operator.
- No prod/Caddy change. Cron runs but is not shadow-compared.
- Real store receipt verification — Node itself doesn't do it; neither do we.
