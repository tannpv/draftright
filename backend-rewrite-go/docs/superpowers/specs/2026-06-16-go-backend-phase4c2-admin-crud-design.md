# Phase 4c-2 — Admin Content/Ops CRUD — Design

> Go backend port (`backend-rewrite-go`). Byte-identical drop-in for the
> NestJS admin CRUD surface. **Node is parity authority.** Ships nothing to
> prod — shadow-compared before any Caddy flip.

**Date:** 2026-06-16
**Phase:** 4c-2 (follows 4c-1 admin foundation, merged `29fcd6c1`)
**Scope:** 25 admin endpoints across 7 entity areas. First real consumer of
the `listquery` helper built in 4c-1.

---

## 1. Goal

Port the NestJS `AdminController` content/ops CRUD routes to Go, byte-identical
in route, method, request shape, response JSON (keys IN ORDER), status code, and
error envelope. Reuse existing Go modules; add CRUD where missing.

**Out of scope (deferred to later phases):** releases admin (4), subscriptions
grant (1), bug-reports admin views (6), reporting/stats (4c-3), AI
fix-proposal + cron (4c-4).

---

## 2. Endpoint inventory (25 routes)

| # | Method | Path | Module | List mode |
|---|--------|------|--------|-----------|
| 1 | GET | `/admin/users` | user | bespoke (limit def **20**) |
| 2 | GET | `/admin/users/:id` | user | — (composite) |
| 3 | PATCH | `/admin/users/:id` | user | — |
| 4 | GET | `/admin/plans` | plans | **dual-mode** (legacy array \| listquery) |
| 5 | POST | `/admin/plans` | plans | — |
| 6 | PATCH | `/admin/plans/:id` | plans | — |
| 7 | DELETE | `/admin/plans/:id` | plans | soft → `{success:true}` |
| 8 | GET | `/admin/ai-providers` | aiprovider | unpaginated array |
| 9 | GET | `/admin/ai-providers/paginated` | aiprovider | listquery |
| 10 | POST | `/admin/ai-providers` | aiprovider | — |
| 11 | PATCH | `/admin/ai-providers/:id` | aiprovider | — |
| 12 | DELETE | `/admin/ai-providers/:id` | aiprovider | soft → `{success:true}` |
| 13 | POST | `/admin/ai-providers/:id/test` | aiprovider | live provider call |
| 14 | GET | `/admin/admin-users` | adminauth | **dual-mode** (legacy array \| listquery) |
| 15 | POST | `/admin/admin-users` | adminauth | — (201, bcrypt) |
| 16 | PATCH | `/admin/admin-users/:id` | adminauth | — |
| 17 | DELETE | `/admin/admin-users/:id` | adminauth | soft → `{success:true}` |
| 18 | GET | `/admin/settings` | appsettings | single row |
| 19 | PATCH | `/admin/settings` | appsettings | — |
| 20 | POST | `/admin/settings/test-email` | appsettings | sends email |
| 21 | GET | `/admin/email-logs` | email | bespoke (limit def **50** max **200**) |
| 22 | GET | `/admin/email-templates` | email | merged array |
| 23 | PATCH | `/admin/email-templates/:key` | email | → `{ok:true}` |
| 24 | DELETE | `/admin/email-templates/:key` | email | reset → `{ok:true}` |
| 25 | GET | `/admin/email-templates/:key/preview` | email | rendered `{subject,html}` |

All 25 mount inside the existing admin group (`jwtMW → RequireAdmin`) from 4c-1,
except none are public (admin login alone is public).

---

## 3. Module map

Per the brainstorm decision: extend existing modules; add new flat modules only
for entities lacking one.

| Module | Action | Adds |
|--------|--------|------|
| `internal/user` | EXTEND | `admin_handler.go`, admin list/detail/update repo queries, full-detail user struct |
| `internal/plans` | EXTEND | Create/Update/SoftDelete on Reader + `admin_handler.go` (dual-mode list) |
| `internal/aiprovider` | **NEW flat** | domain/usecase/repo_pg/handler — 6 routes inc. live test |
| `internal/appsettings` | **NEW flat** | domain/usecase/repo_pg/handler — get/patch/test-email |
| `internal/adminauth` | EXTEND | admin-users CRUD (list/create/update/delete) — reuses existing repo+bcrypt |
| `internal/email` | EXTEND | `admin_handler.go` — logs list + templates list/update/reset/preview (reuses existing templates.go + render) |

New module skeletons follow the flat recipe in `CLAUDE.md` (domain.go →
usecase.go → repo_pg.go → handler.go, single package, ports on consumer side).

---

## 4. Per-area parity detail

All field orders below are the EXACT Node entity / response order. Go ordered
structs marshal in declaration order → match. Timestamps render via
`shared.ISOMillis`. Nullable columns use pointer fields (`*string`,
`*time.Time`) so `null` serializes identically.

### 4.1 Users

**GET /admin/users** — bespoke parsing, NOT `parseListQuery`:
- query: `search`, `page` (def 1), `limit` (def **20**), `status`
  (`active|inactive|all`, def unset), `sort_by` (allow:
  `email,name,role,is_active,created_at`, def `created_at`), `sort_order`
  (def `DESC`).
- 200, body `{users:[…],total}`. Per-user fields in order:
  `id,email,name,role,is_active,plan,usage_today,created_at`.
- `plan` = active-subscription plan name, else `"None"`.
  `usage_today` = `usage.CountToday(userID)`.
- N+1 in Node (per-row sub + usage). Go: replicate behavior; may batch
  internally as long as output is byte-identical. (Plan note: a single
  LEFT JOIN query producing the same values is acceptable since output is what
  the shadow gate compares; default to the simplest correct SQL.)

**GET /admin/users/:id** — composite, 200:
`{user, subscription, usage_today, recent_usage}`.
- `user` = **FULL Node user entity, all columns in TypeORM order, INCLUDING
  `password_hash` and reset/verification codes.** Requires a new full-detail
  user struct + admin query selecting all columns (the slim `user.User` is
  insufficient). Field order to mirror `src/users/entities/user.entity.ts`.
- `subscription` = active sub columns + nested `plan` object; **no nested
  `user`** (Node loads relation `['plan']` only). `null` when no active sub.
- `recent_usage` = up to 20 recent usage rows, `created_at DESC`. Exact columns
  + whether `ai_provider` relation is nested → confirm against Node at
  implementation time (TDD vs captured fixture).
- **Security-parity note:** exposing `password_hash`/reset codes is existing
  Node behavior. Port replicates it for byte-parity; do NOT mask here. File a
  follow-up issue to harden Node + Go together later (§8).

**PATCH /admin/users/:id** — body `UpdateUserDto` all optional:
`{is_active?, role? ("user" only), name?}`. 200, returns full updated user
entity (same full struct as getUser.user).

### 4.2 Plans

`PlanEntity` already exists with correct json tags + order. Add Create/Update/
SoftDelete to `plans.Reader` (or a Writer) + queries.

**GET /admin/plans** — dual-mode. Branch condition (exact):
unpaginated when query is empty OR all of `page`,`search`,`status`,`sort_by`
are absent; else paginated.
- unpaginated → `[]PlanEntity` array (all plans, Node `findAll()` order).
- paginated → `{rows,total}` via listquery. searchCols, sortAllow
  (`name,price,currency,billing_period,trial_days,is_active,created_at`),
  default sort `created_at`, statusCol `is_active`. (Note Node sort key
  `price` → column `price_cents`.)

**POST /admin/plans** — body `{name,daily_limit,price_cents,billing_period,
currency?,trial_days?,stripe_price_id?}`. **201**, returns full PlanEntity.

**PATCH /admin/plans/:id** — partial of the create body + `is_active?`. 200,
full PlanEntity.

**DELETE /admin/plans/:id** — soft (`is_active=false`). 200 `{success:true}`.

### 4.3 AI providers (NEW module)

`AiProvider` struct field order: `id,name,type,endpoint_url,api_key,model,
temperature,is_default,is_active,created_at,updated_at`. **`api_key` returned
in plaintext on all reads** (Node does not mask — parity; follow-up §8).

- **GET /admin/ai-providers** — `[]AiProvider`, order `created_at ASC`.
- **GET /admin/ai-providers/paginated** — listquery → `{rows,total}`. searchCols
  `name,type,model`; sortAllow `name,type,model,is_default,is_active,created_at`;
  default `created_at`; statusCol `is_active`.
- **POST** — body `{name,type,endpoint_url,api_key?,model,temperature?}`
  (temperature default 0.3). Returns full provider. **Single-default
  invariant:** setting `is_default` true demotes the prior default (same txn
  semantics as Node service).
- **PATCH /:id** — partial + `is_default?,is_active?`. Same demote logic. 200.
- **DELETE /:id** — soft (`is_active=false, is_default=false`). 200
  `{success:true}`.
- **POST /:id/test** — no body. Loads provider by id; not found →
  `{success:false, error:"Provider not found"}`. Else calls
  `aicall.Completer.Complete(ctx, system, user)` with EXACT strings:
  - system: `"Rewrite this text to be more concise."`
  - user: `"This is a test sentence to verify the connection works properly."`
  - success → `{success:true, response, response_time_ms}`.
  - exception → `{success:false, error:<message>}`.
  Requires constructing the matching `Completer` adapter from the stored
  provider config (reuse `rewrite/adapter/{openai,anthropic,ollama}`).

### 4.4 Admin users (extend adminauth)

Reuse `adminauth.PgRepo` + `shared.HashPassword` (bcrypt cost 10). Add list/
create/update/delete repo methods + an `admin_users_handler.go`.

`AdminUser` response field order (password_hash EXCLUDED):
`id,email,name,is_active,role,created_at,updated_at`.

- **GET /admin/admin-users** — **dual-mode** like plans (legacy array when no
  params; else `{rows,total}` via listquery). searchCols `name,email,role`;
  sortAllow `name,email,role,is_active,created_at`; default `created_at`;
  statusCol `is_active`.
- **POST** — body `{email,password,name,role?(def "admin")}`. Email uniqueness →
  400 (`invalid-input`). bcrypt hash. **201**, returns admin user
  (password_hash stripped).
- **PATCH /:id** — `{name?,email?,role?,is_active?,password?}`; password re-hash
  only when provided. 200, stripped.
- **DELETE /:id** — soft (`is_active=false`). 200 `{success:true}`. No self/
  last-admin guard (Node has none — parity).

### 4.5 Settings (NEW module)

Single-row `app_settings` table. `GET` returns/creates-if-missing the row with
ALL columns in entity order (long list incl. `stripe_secret_key`,
`paypal_client_secret`, `momo_secret_key`, `casso_api_key`, `sepay_api_key`,
`resend_api_key`, etc.) — **all secrets returned unmasked** (Node parity;
follow-up §8). Field order mirrors `src/admin/entities/app-settings.entity.ts`.

- **GET /admin/settings** — 200, full row.
- **PATCH /admin/settings** — `Partial<AppSettings>`. Node validates
  `payment_methods_enabled` via `paymentService.assertMethodsRegisterable()`
  before update — port the validation (reuse `internal/payment` registry).
  200, full updated row.
- **POST /admin/settings/test-email** — body `{to}`, must contain `@` else
  throws `"Valid recipient email required"` (400). Sends test email (subject
  `"DraftRight test email"`) via `email.SendRaw`. Success
  `{sent:true, to:<recipient>}`.

### 4.6 Email logs + templates (extend email)

Reuse existing `internal/email/templates.go` (6 built-in defs + `renderTemplate`
+ `substitute`) and the `GetEmailTemplate` override query. Add an EmailLog list
query + an EmailTemplate upsert/delete query + `admin_handler.go`.

- **GET /admin/email-logs** — bespoke parsing: `limit` def **50** max **200**,
  `page` def 1, `status` exact filter. 200 `{rows,total}`. Log fields in order:
  `id,to_email,email_type,subject,status,provider_id,error,created_at`.
- **GET /admin/email-templates** — merged ARRAY (built-in defs + DB overrides).
  Per item: `key,label,variables,subject,html,customized,default_subject,
  default_html`. `subject`/`html` = override if present else default;
  `customized` = override exists.
- **PATCH /admin/email-templates/:key** — body `{subject,html}`. Key must be in
  the built-in map else 404 (`not-found`). Upsert override. 200 `{ok:true}`.
- **DELETE /admin/email-templates/:key** — delete override row (reset to
  default). 200 `{ok:true}`. No key validation (soft fail, parity).
- **GET /admin/email-templates/:key/preview** — key must exist else 404.
  Renders with hardcoded sample vars `{name:"Tan", code:"123456", plan:"Pro",
  amount:"124.000 ₫", expires:<30 days out, toDateString()>}`, HTML-escaped.
  200 `{subject,html}`. (Plan note: `expires` uses a relative date — port must
  compute it the same way Node does; pin the format in the task.)

---

## 5. listquery integration

`listquery.Parse(r.URL.Query()) → Query`; `listquery.Build(q, searchCols,
sortAllow, defaultSort, statusCol) → Built{Where,Args,Order,Limit,Offset}`.
Consumer composes two queries (page rows + count) and returns `{rows,total}`.

listquery consumers in 4c-2: **plans (paginated branch), ai-providers/paginated,
admin-users (paginated branch)**. NOT users (bespoke def-20) and NOT email-logs
(bespoke def-50/max-200) — those parse params by hand to match Node exactly.

Each consumer owns its `searchCols`/`sortAllow`/`defaultSort`/`statusCol`
constants (per §4). sortAllow maps the public sort key to the SQL column
(injection-safe allow-list) — including Node aliases like `price`→`price_cents`.

---

## 6. Cross-module ports (consumer-side interfaces)

The admin user list/detail need data the user module doesn't own. Declare small
ports in `user`'s admin usecase, inject concretes from `main.go`:
- usage count → `usage.Counter.CountToday`.
- active subscription → `subscription.Reader.ActiveByUser` (+ a plan-name view).

The ai-provider test needs to build a completer from provider config — depend on
the `aicall.Completer` abstraction + an adapter factory, not concrete adapters.
Settings PATCH validation reuses the `payment` method registry.

No module imports another's constructor or guts (CLAUDE.md rule 2). `main.go` is
the only wiring point.

---

## 7. Parity invariants

- Response key order via ordered structs; nullable → pointer fields.
- Status: POST-create **201** (plans, admin-users); other POST/PATCH/GET/DELETE
  **200**. soft-DELETE 200 `{success:true}` (plans, ai-providers, admin-users);
  template ops `{ok:true}`.
- JWT/admin guard unchanged (4c-1): all routes inside `jwtMW → RequireAdmin`,
  403 code `forbidden`.
- Error envelope `{error,code,request_id}` via `shared.WriteError`; codes:
  `invalid-input` 400 (e.g. duplicate admin email, bad test-email),
  `not-found` 404 (unknown template key), default `internal` 500. request_id
  dynamic → tests assert by Contains, not exact body.
- Secrets exposed exactly as Node (api_key plaintext, settings secrets,
  getUser password_hash) — byte-parity over hardening; §8 tracks the fix.

---

## 8. Follow-up issues (file as GitHub issues, do not fix in 4c-2)

These would break byte-parity if changed now; track for a coordinated Node+Go
hardening pass:
1. `GET /admin/ai-providers*` returns `api_key` in plaintext.
2. `GET /admin/settings` returns all provider secrets unmasked.
3. `GET /admin/users/:id` returns full user entity incl. `password_hash` +
   password-reset / email-verification codes.
4. `DELETE /admin/admin-users/:id` has no self / last-admin guard.

Plus the standing 4c-1 backlog (feedback fixture, GET /feedback disclosure,
toggleVote TOCTOU, `sub||user_id` fallback).

---

## 9. Testing strategy

- Per module: handler tests asserting exact body bytes + key order + status,
  using fakes for the consumer-side ports (no DB). For composite/getUser and the
  merged-template list, assert against captured Node fixtures.
- listquery-config tests per consumer: feed representative params, assert the
  resulting `Where`/`Order`/`Args` match the intended SQL (catches sort-alias
  and statusCol mistakes).
- Router integration: each new route guarded (401 without JWT, 403 non-admin) —
  reuse the 4c-1 pattern.
- Full gate green before merge: `go build ./...`, `go vet ./...`, `gofmt -l .`
  empty, `go test ./... -race`.

---

## 10. Decomposition

One plan, ~30 TDD tasks, subagent-driven (fresh implementer + spec-then-quality
review per task). Order: new modules first (aiprovider, appsettings) → extend
user/plans/adminauth/email → wire main.go + router → integration tests → final
whole-branch review. Each task stages ONLY its own files.
