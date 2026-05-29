# Go Backend — Full NestJS Replication (Clean Architecture, Strangler Migration)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replicate the full DraftRight NestJS backend in Go with clean (hexagonal) architecture, replacing NestJS endpoint-by-endpoint via the Strangler Fig pattern. **No big-bang rewrite.** At every milestone, both backends coexist; production traffic moves one route at a time when each is proven. Final state: NestJS retired, Go owns everything, Postgres + Redis + Caddy + Stripe + Lemon Squeezy + Resend integrations unchanged.

**Architecture:** Single Go module, layered per-subsystem. Top-level layers (cross-cutting): `domain/` (entities + ports), `usecase/` (orchestration), `adapter/` (concrete I/O), `transport/http/` (Axum-equivalent: chi router + handlers). Each subsystem (`auth`, `payment`, `rewrite`, etc.) gets its own slice inside each layer so dependencies stay inward. The `internal/` package boundary enforces "no consumer outside this module" + lint enforces "domain imports nothing else".

**Tech Stack:** Go 1.22+, chi v5 (HTTP router), pgx/v5 + sqlc (typed Postgres with compile-time-checked queries), `go-redis/v9`, `golang-jwt/jwt/v5`, `stripe-go/v76`, custom Lemon Squeezy + Resend clients (no first-party Go SDKs), `slog` (stdlib structured logging), `prometheus/client_golang` (metrics), `testify` + `testcontainers-go` (tests), Docker multi-stage build, docker-compose service alongside NestJS.

**Why this exists:** Studying clean architecture + microservice patterns. Performance is a secondary motive — the NestJS backend is healthy. Treat this as a learning project that, if it succeeds, gracefully replaces NestJS over 9-18 months of weekend work.

---

## ⛔ Three hard gates before this plan starts

1. **Schema-drift safeguard.** NestJS owns migrations until the final cutover. Go uses `sqlc` to generate read bindings against the live Postgres. CI gate: `sqlc generate` + `git diff --exit-code` fails any PR that doesn't regenerate after a schema change.
2. **JWT compatibility.** Until cutover, NestJS keeps signing tokens. Go verifies the same HS256 secret. Token claim shape `{sub, role, exp}` is the contract — never break it without dual-window rollout.
3. **Deploy ordering.** Encoded in `scripts/deploy.sh`: migrations → NestJS deploy → `sqlc generate` → Go deploy. Out-of-order = silent runtime errors in Go.

If any of these isn't in place at the start of a phase, **finish them first**.

---

## Boundary rules (Rule #1, written down day 1)

1. **NestJS owns the schema** until Phase 6 cutover. Go uses `sqlc` read bindings + a separate write-permission audit per table.
2. **Each subsystem has its own clean-arch layering** inside the Go module. `internal/auth/domain`, `internal/auth/usecase`, `internal/auth/adapter`, `internal/auth/transport`. Same for `payment`, `rewrite`, `bug_reports`, etc.
3. **`internal/<subsystem>/domain` imports nothing outside `internal/<subsystem>/domain`.** Period. Linted by `go-cleanarch`. The exception is `internal/shared/domain` for cross-subsystem types (UserID, Plan).
4. **No `internal/<X>` package imports `internal/<Y>` from a different subsystem.** Cross-subsystem talk goes through a small `internal/shared/events` bus (e.g. user-deleted event triggers subscription cleanup). This is the boundary that lets us extract a subsystem into its own microservice later.
5. **Constants over literals.** `const PaymentMethodStripe = "stripe"`, never `"stripe"` scattered. This mirrors Rule #1 from the NestJS side (`PaymentMethod` enum).
6. **No `.unwrap()` equivalent.** Every error returned, never `panic()` in handlers. `slog.Error` + return.
7. **Each subsystem ships an HTTP feature flag.** Until parity is proven, the route hits NestJS. After soak, the flag flips to Go via Caddy config (one-line change).
8. **Postgres role split.** Each subsystem gets a Postgres role with the minimum grants it needs. `dr_auth_rw` can only touch `users`, `extension_tokens`, `refresh_tokens`. `dr_payment_rw` only `payments`, `subscriptions`. Cross-cutting reads use a `dr_reader` role.

---

## Macro architecture

```
backend-go/                                       # new sibling of /backend/
├── cmd/server/main.go                            # composition root — wires all subsystems
├── internal/
│   ├── shared/                                   # cross-cutting kernel — used by all subsystems
│   │   ├── domain/                               # UserID, Plan, money types, common errors
│   │   ├── events/                               # in-process event bus (channel-based v1; NATS later)
│   │   ├── id/                                   # uuid + display_no formatting (BUG-N, ERR-N, FR-N)
│   │   ├── time/                                 # clock abstraction for testability
│   │   └── http/                                 # middleware (logging, correlation, recover, cors, metrics)
│   ├── platform/                                 # infra adapters shared by all subsystems
│   │   ├── auth/                                 # JWT verify/sign, shared HS256 (then RS256)
│   │   ├── db/                                   # pgxpool builders, sqlc generated bindings root
│   │   ├── redis/                                # connection multiplexer
│   │   ├── email/                                # Resend client wrapper
│   │   ├── logger/                               # slog setup, JSON output
│   │   ├── metrics/                              # prometheus registry
│   │   ├── config/                               # typed env via envconfig
│   │   └── throttle/                             # token bucket via Redis (mirrors NestJS @nestjs/throttler)
│   │
│   ├── auth/                                     # ONE SUBSYSTEM — full clean-arch slice
│   │   ├── domain/                               # User, RefreshToken, ExtensionToken entities + ports
│   │   ├── usecase/                              # Register, Login, Refresh, Logout, SocialLogin, DeleteAccount, MintExtensionToken
│   │   ├── adapter/                              # pgUserRepo, pgRefreshRepo, bcryptHasher, googleVerifier
│   │   └── transport/                            # http handlers — POST /auth/login etc.
│   │
│   ├── users/                                    # CRUD + search + paginated listing
│   │   ├── domain/  usecase/  adapter/  transport/
│   ├── plans/                                    # public listing + admin CRUD
│   ├── subscriptions/                            # assign plan, receipt verify
│   ├── ai_providers/                             # OpenAI/Anthropic/Ollama config + provider abstraction
│   ├── rewrite/                                  # /rewrite — quota → provider → log
│   ├── usage/                                    # usage counting, daily reset, history
│   ├── bug_reports/                              # POST /bug-reports (multipart), /feedback, voting
│   ├── errors/                                   # crash report ingest, AI fix proposals
│   ├── payment/                                  # Stripe + LS + VietQR + bank transfer + webhooks
│   ├── updates/                                  # /updates/latest, release notes
│   ├── email/                                    # templates CRUD, logs, send queue
│   ├── extraction/                               # entity extraction service
│   ├── ime_packs/                                # language container manifest endpoint
│   └── admin/                                    # admin auth + every admin/* route
│
├── queries.sql                                   # all sqlc queries (one file or split per subsystem)
├── sqlc.yaml
├── migrations/                                   # READ-ONLY mirror of /backend/sql until cutover
├── Dockerfile
├── docker-compose.prod.yml.snippet               # service block to add
├── deploy/Caddyfile.snippet                      # per-route flip rules
├── scripts/
│   ├── deploy.sh                                 # ordered: migrate → NestJS → sqlc → Go
│   ├── sqlc-check.sh                             # CI gate
│   ├── parity-check.sh                           # compares Go + NestJS responses on the same payload
│   └── role-grant.sh                             # Postgres role + per-table grant setup
├── test/
│   ├── integration/                              # spins up Postgres + Redis + httptest server
│   ├── parity/                                   # diff-tests Go vs NestJS on canned requests
│   └── load/                                     # k6 / vegeta scripts
└── docs/
    ├── ARCH.md                                   # this file's TL;DR pinned next to the source
    ├── BOUNDARY-RULES.md                         # 8 rules above, always-current
    └── SUBSYSTEM-TEMPLATE.md                     # how to slice a new subsystem (Rule #1 reusable)
```

**Total subsystems to migrate: 15.** Listed in dependency-aware migration order in the phase plan below.

---

## Strangler Migration Order

Each subsystem migrates in this 4-step cycle:

1. **Build in parallel.** Go service exposes `/v2/<subsystem>/*` paths; NestJS keeps `/<subsystem>/*`. Both write to same Postgres.
2. **Shadow traffic.** Caddy mirrors writes (or a cron-driven parity check replays canned requests); Go's responses compared to NestJS's. Bugs fixed before flip.
3. **Flip via Caddy.** One-line change in Caddyfile routes the path from NestJS port 3000 to Go port 3100.
4. **Soak 1-2 weeks**, watch logs + metrics. If clean, remove NestJS controller. Otherwise revert the Caddy line (instant rollback).

**Sequencing principle: easiest + lowest-risk first, payments last.**

| Phase | Subsystem | Risk | Why this order |
|---|---|---|---|
| 0 | Foundation (shared kernel + platform) | — | Everything depends on it |
| 1 | `updates` | Lowest | Read-only, public, simple JSON. Proves the deploy pipeline. |
| 2 | `ime_packs` | Low | Read-only public manifest. Same shape as updates. |
| 3 | `errors` | Low-medium | Anonymous + auth, no money. Honeypot + throttler are critical (Rule #1 + anti-spam reuse). |
| 4 | `bug_reports` + feedback | Medium | Multipart file upload, public voting, display_no formatting. |
| 5 | `auth` | High | JWT compatibility window required. Every other subsystem depends on this working. |
| 6 | `users` + `plans` | Medium | Mostly CRUD after auth is proven. |
| 7 | `subscriptions` | Medium | Receipt-verify integrations (App Store, Play Store). |
| 8 | `ai_providers` + `rewrite` + `usage` | Medium-high | The hot path. SSE streaming, multi-provider failover, quota check, write-behind usage log. |
| 9 | `extraction` | Low | Niche endpoint. |
| 10 | `email` | Medium | Resend integration + queued sends. |
| 11 | `admin` | Medium | RolesGuard + every admin route. Touches every other subsystem's repository. |
| 12 | `payment` | **HIGHEST** | Stripe + LS webhooks, signed payloads, money. Done last when everything else is proven. |
| 13 | Final cutover + decommission NestJS | — | Remove NestJS code; schema migrations move to Go ownership; rotate JWT to RS256. |

---

## Phase 0: Foundation (~2 weekends)

### Task 0.1: Module scaffold + workspace structure

**Files:**
- Create: `backend-go/go.mod`, `cmd/server/main.go`, `Dockerfile`, `docker-compose.dev.yml`, `Makefile`, `.gitignore`, `README.md`, `docs/ARCH.md`, `docs/BOUNDARY-RULES.md`

- [ ] **Step 1: Init module + directory skeleton**

```bash
mkdir backend-go && cd backend-go
go mod init github.com/tannpv/draftright-backend-go
mkdir -p cmd/server \
  internal/{shared/{domain,events,id,time,http},platform/{auth,db,redis,email,logger,metrics,config,throttle}} \
  internal/{auth,users,plans,subscriptions,ai_providers,rewrite,usage,bug_reports,errors,payment,updates,email,extraction,ime_packs,admin}/{domain,usecase,adapter,transport}
```

- [ ] **Step 2: Boundary-rules doc as a file (committed for posterity)**

Save the 8 rules from above into `docs/BOUNDARY-RULES.md`. CI lints PRs against violations.

- [ ] **Step 3: Hello-world `cmd/server/main.go`** with `/health` + `/version` endpoints. (See `2026-05-29-go-rewrite-microservice.md` Task 1 for code template.)

- [ ] **Step 4: Multi-stage Dockerfile** (distroless final image). Same template as the rewrite plan.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(backend-go): module scaffold + clean-arch directory skeleton"
```

### Task 0.2: Platform layer — config, logger, metrics, db, redis

**Files:**
- Create: `internal/platform/config/config.go`, `internal/platform/logger/logger.go`, `internal/platform/metrics/registry.go`, `internal/platform/db/pool.go`, `internal/platform/redis/client.go`
- Test: parity with NestJS env var names

- [ ] **Step 1: Typed config via envconfig (single source of truth for env vars)**

```go
// internal/platform/config/config.go
package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
    Listen          string `envconfig:"LISTEN_ADDR" default:":3100"`
    DatabaseURL     string `envconfig:"DATABASE_URL" required:"true"`
    RedisURL        string `envconfig:"REDIS_URL"    required:"true"`
    JWTSecret       string `envconfig:"JWT_SECRET"   required:"true"`
    JWTRefreshSecret string `envconfig:"JWT_REFRESH_SECRET" required:"true"`
    LogLevel        string `envconfig:"LOG_LEVEL" default:"info"`
    OpenAIKey       string `envconfig:"OPENAI_API_KEY"`
    AnthropicKey    string `envconfig:"ANTHROPIC_API_KEY"`
    StripeSecret    string `envconfig:"STRIPE_SECRET_KEY"`
    LemonSqueezyKey string `envconfig:"LEMONSQUEEZY_API_KEY"`
    ResendKey       string `envconfig:"RESEND_API_KEY"`
    AppEnv          string `envconfig:"APP_ENV" default:"development"`
}

func Load() (*Config, error) {
    var c Config
    return &c, envconfig.Process("", &c)
}
```

Names match `/backend/.env` exactly — no migration of env-var names.

- [ ] **Step 2: Logger setup** — slog with JSON output, level from config.
- [ ] **Step 3: pgxpool builder** — `max_conns=25`, `min_conns=5`, ApplicationName tag = `dr-backend-go` so DB logs identify the source.
- [ ] **Step 4: Redis client** with connection multiplexing.
- [ ] **Step 5: Prometheus registry** — exposed via `/metrics`.
- [ ] **Step 6: Commit**

### Task 0.3: Shared kernel — domain types + http middleware + display_no helper

**Files:**
- Create: `internal/shared/domain/{user_id,plan,money}.go`, `internal/shared/id/display.go`, `internal/shared/http/middleware_{logging,correlation,recover,metrics,cors}.go`
- Test: display number parity with NestJS `formatDisplayNumber`

- [ ] **Step 1: Cross-subsystem types**

```go
// internal/shared/domain/user_id.go
type UserID uuid.UUID
```

- [ ] **Step 2: `internal/shared/id/display.go`** — must produce IDENTICAL output to NestJS `formatDisplayNumber`:

```go
// BUG-N, FR-N, ERR-N — matches backend/src/common/display-number.ts
type ReportKind string
const (
    ReportKindBug     ReportKind = "bug"
    ReportKindFeature ReportKind = "feature"
    ReportKindError   ReportKind = "error"
)

func FormatDisplayNumber(kind ReportKind, n *int64) *string {
    if n == nil { return nil }
    prefix := map[ReportKind]string{
        ReportKindBug: "BUG", ReportKindFeature: "FR", ReportKindError: "ERR",
    }[kind]
    s := fmt.Sprintf("%s-%d", prefix, *n)
    return &s
}
```

- [ ] **Step 3: HTTP middleware stack** (logging, correlation, recover, metrics, CORS). Mirrors NestJS's behavior.
- [ ] **Step 4: Test parity** — feed the same `display_no` to both implementations, assert byte-equal output.
- [ ] **Step 5: Commit**

### Task 0.4: sqlc setup + schema mirroring

**Files:**
- Create: `sqlc.yaml`, `queries.sql`, `scripts/sqlc-check.sh`, `.github/workflows/sqlc-check.yml`

- [ ] **Step 1: sqlc.yaml pointing at NestJS migrations**

```yaml
version: "2"
sql:
  - schema: "../backend/sql"          # NestJS-owned
    queries: "queries.sql"
    engine: "postgresql"
    gen:
      go:
        out: "internal/platform/db/sqlc"
        package: "sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_db_tags: true
        emit_interface: true
        rename:
          # match NestJS column → Go field names exactly
```

- [ ] **Step 2: Run `sqlc generate`** against the dev Postgres. Generated files committed.
- [ ] **Step 3: CI gate** — `sqlc generate && git diff --exit-code internal/platform/db/sqlc/` must pass.
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(backend-go): platform layer — config, logger, metrics, pgx + sqlc + shared kernel"
```

### Task 0.5: Throttler + honeypot middleware (Rule #1 — reusable)

**Files:**
- Create: `internal/platform/throttle/{redis_token_bucket,decorator}.go`, `internal/shared/http/honeypot.go`

- [ ] **Step 1: Redis token bucket throttler**

```go
// 5/min per IP for anon endpoints, 60/min for authed.
// Mirrors NestJS @nestjs/throttler config in app.module.ts.
```

- [ ] **Step 2: Honeypot extractor** — if `website` form field non-empty, return 2xx but drop. Same behavior as NestJS controllers.
- [ ] **Step 3: Apply as a chi router middleware on the public report endpoints (Phases 3 + 4 consume it)**

```bash
git commit -m "feat(backend-go): throttler + honeypot platform middleware"
```

---

## Phase 1: `updates` subsystem (~1 weekend)

**Why first:** Read-only, public, no auth, single endpoint (`GET /updates/latest`). Proves the deploy pipeline + Caddy flip mechanism end-to-end with minimum risk.

**Endpoints to replicate:**
- `GET /updates/latest?platform=mac|windows|ios|android|linux` — returns latest version + URL + release notes per platform.

**Files (new under `internal/updates/`):**
- `domain/release.go` — `type Release struct { ... }`
- `domain/ports.go` — `type ReleaseRepo interface { Latest(ctx, platform) (...)}`
- `usecase/get_latest.go`
- `adapter/pg/release_repo.go` (uses sqlc-generated `appReleases.*`)
- `transport/http/handler.go` — `chi.Get("/updates/latest", h.Latest)`

**Tasks:**

- [ ] **0:** sqlc queries: `SELECT * FROM app_releases WHERE platform=$1 ORDER BY updated_at DESC LIMIT 1` × 5 platforms.
- [ ] **1:** Domain types + ports.
- [ ] **2:** Use case.
- [ ] **3:** PG adapter via sqlc.
- [ ] **4:** HTTP handler with chi.
- [ ] **5:** End-to-end test: spin up testcontainers Postgres, seed `app_releases`, hit `/v2/updates/latest`.
- [ ] **6:** Parity test: hit both NestJS `/updates/latest` and Go `/v2/updates/latest` with same query; assert JSON equality.
- [ ] **7:** Caddy snippet — `/v2/updates/latest` routed to Go on port 3100.
- [ ] **8:** Deploy. Soak 3 days. Compare logs.
- [ ] **9:** Flip Caddy — `/updates/latest` now routes to Go. NestJS controller stays as fallback.
- [ ] **10:** Soak 1 week. Remove NestJS UpdatesController + UpdatesModule on success.

```bash
git commit -m "feat(backend-go): /updates subsystem migrated — first Strangler slice"
```

---

## Phase 2: `ime_packs` subsystem (~1 weekend)

**Why next:** Same shape as `updates` (read-only public manifest). Reinforces the pattern. Validates the JSON shape contract between Go and Flutter clients.

**Endpoints:**
- `GET /ime-packs/manifest` — returns language container catalog (Japanese / Chinese / Korean / Vietnamese / etc.).

**Files:** Same shape as Phase 1. Repo reads `app_settings`-style hard-coded catalog initially; later moves to a DB-driven table if needed.

**Tasks (mirror Phase 1's 11 steps).**

Critical: the response JSON shape must match exactly — Flutter clients parse it byte-for-byte. Add a parity test that diffs the full body.

---

## Phase 3: `errors` subsystem (~2 weekends)

**Why:** Public + anon-allowed write endpoint. First test of the throttler + honeypot + display_no infrastructure together.

**Endpoints:**
- `POST /errors` — ingest crash from any client (auth optional)
- Admin endpoints (deferred to Phase 11)

**Files:**
- `internal/errors/domain/{error_report,severity,status,ports}.go`
- `internal/errors/usecase/{ingest,fingerprint}.go`
- `internal/errors/adapter/pg/error_repo.go`
- `internal/errors/transport/http/handler_ingest.go`

**Tasks:**

- [ ] **0:** sqlc queries: `INSERT INTO error_reports (...)`, `SELECT * FROM error_reports WHERE fingerprint=$1`, `UPDATE ... SET count=count+1`.
- [ ] **1:** Domain `ErrorReport` with `Fingerprint()` method (SHA256 of platform + error_type + stack first line). Must match NestJS algorithm byte-for-byte.
- [ ] **2:** Use case `Ingest(ctx, dto)` — finds-or-inserts by fingerprint, increments `count`.
- [ ] **3:** PG adapter.
- [ ] **4:** HTTP handler with `display_no` + `ref` returned (`ERR-N`).
- [ ] **5:** Wire throttler middleware (30/min, 300/hour per IP — matches NestJS).
- [ ] **6:** Wire honeypot extractor (`website` field).
- [ ] **7:** Validation rejection logger (matches `main.ts` `ValidationPipe` exceptionFactory).
- [ ] **8:** Parity test: same 50 crash payloads hit both backends; assert fingerprint + ref + count are identical.
- [ ] **9:** Caddy `/v2/errors` route → Go.
- [ ] **10:** Soak 1 week → flip `/errors`.
- [ ] **11:** Remove NestJS ErrorsController + ErrorsModule.

```bash
git commit -m "feat(backend-go): /errors subsystem migrated, validates throttler + honeypot + display_no platform"
```

---

## Phase 4: `bug_reports` + feedback subsystem (~3 weekends)

**Why:** Adds multipart file upload, public voting (auth required), polymorphic kind (bug vs feature), display_no extension (BUG-N + FR-N).

**Endpoints:**
- `POST /bug-reports` (multipart with optional screenshot, auth optional, honeypot)
- `POST /feedback` (JSON, auth optional, honeypot)
- `GET /feedback?kind=feature&status=open&target_platform=mobile&page=1&limit=20` — public board
- `POST /feedback/:id/vote` (auth required)

**Files:**
- `internal/bug_reports/domain/{bug_report,feature_vote,ports}.go`
- `internal/bug_reports/usecase/{submit,vote,list_public,get_screenshot}.go`
- `internal/bug_reports/adapter/pg/{bug_repo,vote_repo}.go`
- `internal/bug_reports/adapter/screenshot/{filesystem,s3}.go` — pluggable storage; v1 = filesystem under `<DATA_DIR>/screenshots/`.
- `internal/bug_reports/transport/http/{handler_submit,handler_vote,handler_list}.go`

**Tasks:**

- [ ] **0:** sqlc queries — including paginated public feed with `is_public=true AND kind='feature'` filter, vote_count derived.
- [ ] **1:** Multipart parsing with 5 MB cap. Reject `application/octet-stream` (per BUG-18 fix). Whitelist MIMEs: png/jpeg/webp/heic/heif/gif.
- [ ] **2:** Filesystem screenshot adapter — writes to disk, returns URL `/bug-reports/:id/screenshot`. Same path NestJS exposes.
- [ ] **3:** Honeypot + throttler middleware (5/min, 30/hour per IP).
- [ ] **4:** Domain validation — `os_info` slice to 100 chars (matches NestJS service); DTO @MaxLength(255) on ingress.
- [ ] **5:** Vote use case — idempotent toggle (one per user per feature; derived `vote_count` via single SQL update).
- [ ] **6:** Public board paginated query (sort by votes desc, then created_at desc).
- [ ] **7:** Parity test — submit 30 reports + 50 votes via both backends; diff DB rows.
- [ ] **8:** Caddy `/v2/bug-reports`, `/v2/feedback` routed.
- [ ] **9:** Soak 1-2 weeks → flip routes.
- [ ] **10:** Remove NestJS BugReportsController + FeedbackController + module.

```bash
git commit -m "feat(backend-go): /bug-reports + /feedback migrated, multipart + voting parity verified"
```

---

## Phase 5: `auth` subsystem (~4 weekends — highest care)

**Why critical:** Every other subsystem depends on token verification. JWT compatibility window is non-negotiable. Mistake here = users logged out across the fleet.

**Endpoints:**
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/social` (Google ID token verification)
- `DELETE /auth/account` (account deletion, with cascade)
- `POST /auth/extension-tokens` (mint `dr_ext_*` token)
- `GET /auth/extension-tokens` (list current user's tokens)
- `DELETE /auth/extension-tokens/:id` (revoke single token)
- `GET /auth/me` (used by every client)

**Files:**
- `internal/auth/domain/{user,refresh_token,extension_token,credentials,ports,errors}.go`
- `internal/auth/usecase/{register,login,refresh,logout,social_login,delete_account,mint_extension_token,list_extension_tokens,revoke_extension_token,get_me}.go`
- `internal/auth/adapter/pg/{user_repo,refresh_token_repo,extension_token_repo}.go`
- `internal/auth/adapter/{bcrypt,google_oauth}/...go`
- `internal/auth/transport/http/handler.go`
- `internal/auth/transport/http/middleware_jwt_auth.go` — replaces NestJS `JwtAuthGuard`
- `internal/auth/transport/http/middleware_rewrite_auth.go` — accepts session JWT OR `dr_ext_*` token (mirrors NestJS `RewriteAuthGuard`)

**Tasks:**

- [ ] **0:** sqlc queries for `users`, `refresh_tokens`, `extension_tokens`. Match every column NestJS reads/writes.
- [ ] **1:** Bcrypt cost-10 hasher (parity with NestJS). Test: hash same password on both sides, verify the OTHER side's hash matches.
- [ ] **2:** JWT signer/verifier — HS256, 15-min access, 7-day refresh. Identical claim shape `{sub, role, iat, exp}`.
- [ ] **3:** **Critical compatibility test:** sign a token on NestJS, verify on Go. Sign on Go, verify on NestJS. Both directions must work. Lock down with integration tests.
- [ ] **4:** Extension token mint — `dr_ext_` prefix + 32 bytes base64url, store SHA256(token) only. Match NestJS `ExtensionTokenService`.
- [ ] **5:** Social login Google ID-token verification — hit `oauth2.googleapis.com/tokeninfo?id_token=` (no client_secret needed; audience-agnostic per NestJS).
- [ ] **6:** Account deletion with cascade — wraps in a DB transaction: delete `usage_logs`, `bug_reports.user_id` → NULL, `extension_tokens`, `refresh_tokens`, `payments.user_id` → NULL, `feature_votes`, `users`. Order matters; same order as NestJS.
- [ ] **7:** `JwtAuthGuard` equivalent — middleware that injects `UserID` into request context, rejects with 401 on failure.
- [ ] **8:** `RewriteAuthGuard` equivalent — accepts session JWT OR `dr_ext_*` token; the latter is verified by SHA256 lookup.
- [ ] **9:** Parity test suite: register a user, login, refresh, mint extension token, list, revoke, delete account — all via both backends. Compare DB state after each.
- [ ] **10:** Run NestJS auth E2E tests against the Go service via Caddy `/v2/auth/*`. Fix any divergence.
- [ ] **11:** Caddy `/v2/auth/*` → Go. **Do NOT flip the canonical paths yet.**
- [ ] **12:** Have ONE client (a debug build) hit `/v2/auth/*` exclusively for 2 weeks. Daily reconciliation: same user can move between NestJS and Go and continue working.
- [ ] **13:** Flip `/auth/*` to Go. Caddy keeps `/v1/auth/*` (now an alias) hitting NestJS for emergency rollback.
- [ ] **14:** Soak 2 weeks.
- [ ] **15:** Remove NestJS AuthModule, JwtAuthGuard, RewriteAuthGuard, ExtensionTokenService.

```bash
git commit -m "feat(backend-go): auth subsystem migrated — JWT/extension-token compat proven over 4-week soak"
```

---

## Phase 6: `users` + `plans` (~2 weekends)

After auth, these are mostly CRUD + listing.

**Endpoints:**
- `GET /users/me/usage`
- (Admin endpoints deferred to Phase 11)
- `GET /plans` (public listing)

**Tasks:** Standard 11-step pattern. Parity test against the live `/plans` JSON.

---

## Phase 7: `subscriptions` (~2 weekends)

**Endpoints:**
- `POST /subscriptions/receipt` (App Store / Play Store verification)
- `GET /subscriptions/me`

**Tasks:**
- [ ] App Store receipt verification (StoreKit2 JWS path; same shape NestJS uses)
- [ ] Play Store receipt verification (Google Play Developer API)
- [ ] Parity test using sandbox receipts
- [ ] Standard Caddy flip + soak

---

## Phase 8: `ai_providers` + `rewrite` + `usage` (~4 weekends)

**Why grouped:** They're tightly coupled. Need to migrate together to avoid awkward intermediate states.

**Endpoints:**
- `POST /rewrite` (SSE streaming planned; currently full-response)
- `GET /usage/me/today`
- (Admin endpoints deferred to Phase 11)

**Sub-tasks:** Already detailed in [[2026-05-29-go-rewrite-microservice]]. **Reuse that plan as a sub-plan.** Run it as Phase 8 of this larger migration.

Add to that plan: also migrate `/usage` queries (read-only aggregation over `usage_logs`).

---

## Phase 9: `extraction` (~1 weekend)

**Endpoint:** `POST /extraction/entities` — phone, email, URL detection (the existing NestJS phone_detector + ExtractionService).

**Tasks:** Re-implement detector in Go (regexes are language-agnostic, port verbatim). Parity test against the 30+ NestJS test cases.

---

## Phase 10: `email` (~2 weekends)

**Endpoints:**
- `POST /admin/email-templates` (admin CRUD — deferred to Phase 11 transport, but use case + adapter built here)
- Backend service: queued send with Resend

**Tasks:**
- [ ] Domain: `Template`, `EmailLog` entities.
- [ ] Resend HTTP client (no first-party Go SDK).
- [ ] Send queue via Redis Streams (BullMQ-equivalent).
- [ ] Lazy-init pattern matching NestJS Resend (per memory `feedback_resend_lazy_init.md`).
- [ ] Parity test: send a templated email via both backends; assert identical body + headers.

---

## Phase 11: `admin` (~4 weekends)

**Why this size:** Touches every other subsystem's repository (users CRUD, plans CRUD, AI providers CRUD, releases, error/bug review, analytics, payment history, etc.).

**Endpoints:** ~30 admin routes. Categorized:
- Resource CRUD (users, plans, providers, releases, email templates, admin users)
- Reports (bug-reports, errors, feedback, inbox counts)
- Analytics (revenue, subscribers, usage trends)
- Payment ops (refund, dispute review)
- AI cron triggers (run AI fix-proposal cron)

**Tasks:**
- [ ] **0:** Admin auth middleware — same JWT but checks `role='admin'` claim. Mirrors `RolesGuard` + `@Roles('admin')`.
- [ ] **1-10:** Each admin route group migrated, parity-tested, flipped via Caddy. One PR per resource (users-admin, plans-admin, etc.).
- [ ] **11:** Analytics queries reproduced. These touch many tables — biggest sqlc query surface.
- [ ] **12:** Caddy flip `/admin/*` after **3-week** soak (admin bugs are high-blast-radius for ops).
- [ ] **13:** Remove NestJS AdminModule.

---

## Phase 12: `payment` (~6 weekends — highest risk)

**Done last on purpose.** Money flows. Webhook signatures. Stripe + Lemon Squeezy + VietQR + bank-transfer. Idempotency is critical.

**Endpoints:**
- `GET /payment/methods`
- `POST /payment/checkout`
- `GET /payment/status/:ref`
- `GET /payment/history`
- `POST /payment/webhook/stripe` (raw body, HMAC verify)
- `POST /payment/webhook/vietqr`
- `POST /payment/webhook/casso`
- `POST /payment/webhook/sepay`
- `POST /payment/webhook/lemonsqueezy` (raw body, HMAC-SHA256 verify)

**Files:**
- `internal/payment/domain/{payment,subscription_event,reference_code,ports}.go`
- `internal/payment/usecase/{create_checkout,handle_webhook,activate_subscription,renew,cancel,...}.go`
- `internal/payment/adapter/pg/{payment_repo,settings_repo}.go`
- `internal/payment/adapter/stripe/{strategy,webhook_verifier}.go`
- `internal/payment/adapter/lemonsqueezy/{strategy,webhook_verifier}.go`
- `internal/payment/adapter/vietqr/strategy.go` (also handles sepay/casso webhook variants — same parsing as NestJS)
- `internal/payment/transport/http/handler.go`

**Tasks:**

- [ ] **0:** sqlc queries for `payments`, `subscriptions`, `app_settings` (LS + Stripe keys).
- [ ] **1:** `PaymentMethod` enum constants — `PaymentMethodStripe`, `PaymentMethodVietQR`, `PaymentMethodBankTransfer`, `PaymentMethodLemonSqueezy`, `PaymentMethodPayPal`, `PaymentMethodMomo`. Rule #1: no string literals scattered.
- [ ] **2:** `reference_code` generator — must match NestJS `generatePaymentReference` byte-for-byte (it's used to correlate webhooks back to payments).
- [ ] **3:** Stripe strategy via `github.com/stripe/stripe-go/v76`. Webhook verification using `webhook.ConstructEvent` with the official SDK.
- [ ] **4:** Lemon Squeezy strategy — custom HTTP client + HMAC-SHA256 raw-body webhook verify (matches NestJS implementation in `lemonsqueezy.strategy.ts`).
- [ ] **5:** VietQR webhook variants (sepay, casso): parse the bank-transfer body, match by reference_code in the transfer memo. Idempotent by Stripe event ID / LS event ID / sepay transfer ID.
- [ ] **6:** `WebhookAction` enum + dispatch (mirrors NestJS `WebhookAction` interface).
- [ ] **7:** Subscription activation use case — same side effects as NestJS (`activateSubscription`, `handleSubscriptionRenewed`, etc.).
- [ ] **8:** **Critical idempotency test:** replay each provider's webhooks 5× in a row. Database state must be identical to single-fire. NestJS does this via Stripe event ID / LS event_id deduplication; Go must match.
- [ ] **9:** Parity test — drive a sandbox checkout end-to-end on both backends. Compare every DB row, every webhook side-effect.
- [ ] **10:** Run for 2 weeks in shadow mode: Caddy mirrors webhooks to BOTH backends; the Go service's writes go to a separate `payments_shadow` table. Compare nightly.
- [ ] **11:** Flip read endpoints first (`GET /payment/methods`, `/status`, `/history`).
- [ ] **12:** Flip `POST /payment/checkout` after 1 week.
- [ ] **13:** Flip webhooks individually with **per-provider** Caddy rules. Stripe first (most mature), LS next, VietQR last.
- [ ] **14:** Soak 4 weeks total.
- [ ] **15:** Remove NestJS PaymentModule.

```bash
git commit -m "feat(backend-go): payment subsystem migrated — Stripe + LS + VietQR shadow-tested 4 weeks"
```

---

## Phase 13: Final cutover + NestJS decommission (~2 weekends)

**Tasks:**

- [ ] **1:** Rotate JWT signing to RS256 — NestJS already-stopped means private key only ever in Go. Dual-window handled in Phase 5.
- [ ] **2:** Schema migration ownership transfer — `backend/sql/*.sql` files move under `backend-go/migrations/`. Future migrations run via `golang-migrate` from Go's deploy pipeline.
- [ ] **3:** Remove NestJS backend container from `docker-compose.prod.yml`.
- [ ] **4:** Remove `/backend/` directory from the repo (kept in git history).
- [ ] **5:** Caddy config simplified — no more port 3000.
- [ ] **6:** Postgres role audit — remove `dr_backend` role; subsystem-scoped roles (`dr_auth_rw`, etc.) become the only writers.
- [ ] **7:** Tag release `v3.0.0-go` on `main`.
- [ ] **8:** Memory + index updates in `~/.claude/projects/-opt-openAi-DraftRight/memory/`.

```bash
git commit -m "feat(backend-go): final cutover — NestJS decommissioned, Go is now the only backend"
```

---

## Self-Review

- **Strangler discipline:** every phase ships parallel; no big-bang. Each Caddy flip is a 1-line revert away. At any point in Phases 1-12, NestJS still serves prod for everything not yet flipped.
- **Schema ownership:** NestJS owns migrations Phases 0-12. Transfer happens in Phase 13. sqlc CI gate makes drift a build failure.
- **JWT compatibility:** Same HS256 secret across both backends until Phase 13's RS256 swap. Tested both directions in Phase 5.
- **Boundary discipline:** 8 rules in `docs/BOUNDARY-RULES.md` + lint enforced. Each subsystem is self-contained — extractable into its own microservice later without rewriting consumers.
- **Rule #1 across migration:**
  - Each subsystem uses constants/enums, never string literals (`PaymentMethodStripe`, `ReportKindBug`).
  - Shared formatters (`FormatDisplayNumber`) live in `internal/shared/id/`, used by 3 subsystems.
  - Throttler + honeypot middleware are platform-wide, used by every anon endpoint (errors, bug_reports, feedback).
  - Subsystem template (`docs/SUBSYSTEM-TEMPLATE.md`) makes adding the next subsystem mechanical.
- **Test coverage:** domain 100% (pure logic); usecase 100% via in-memory adapters; adapters via testcontainers; HTTP via httptest; cross-system via parity-test harness.
- **Rollback path at every phase:** Caddy line revert = instant rollback. NestJS code stays in repo until each subsystem's soak passes.
- **Operational complexity tax:** two Docker images, two log streams, two health checks while migration is in progress. Plan for this in Grafana dashboards from Phase 0.
- **One project, one developer, no rush:** total weekend hours estimated below. This is years of work at one weekend per task, not weeks.

---

## Quick-reference: estimated weekend pacing

| Phase | Tasks | Weekend-hours | Calendar (one weekend / week) |
|---|---|---|---|
| 0 | Foundation | 16 | ~4 weekends |
| 1 | updates | 6 | 1 weekend |
| 2 | ime_packs | 6 | 1 weekend |
| 3 | errors | 12 | 2 weekends |
| 4 | bug_reports + feedback | 20 | 3 weekends |
| 5 | auth | 32 | 4 weekends |
| 6 | users + plans | 12 | 2 weekends |
| 7 | subscriptions | 12 | 2 weekends |
| 8 | ai_providers + rewrite + usage | 28 | 4 weekends |
| 9 | extraction | 6 | 1 weekend |
| 10 | email | 12 | 2 weekends |
| 11 | admin | 32 | 4 weekends |
| 12 | payment | 48 | 6 weekends |
| 13 | cutover | 12 | 2 weekends |
| **Total** | | **~254 hours** | **~38 weekends ≈ 9 months** |

**Honest estimate:** 9-18 months at one weekend per task, slipping included. Faster only if you go full-time (don't — keep shipping features).

---

## When to abandon this plan

1. **By Phase 3**, if you're not enjoying Go — pause. The /rewrite microservice plan ([[2026-05-29-go-rewrite-microservice]]) gives 80% of the learning at 5% of the effort. Backing off to that is rational.
2. **By Phase 5**, if NestJS is solving everything fine and the only thing this is delivering is study — that's fine. **Stop after Phase 5** with auth migrated and call it a learning artifact.
3. **By Phase 8**, if /rewrite migrated to Go is materially better than NestJS — keep that one, stop the rest, and live with a hybrid backend forever. Hybrid backends are FINE if the boundary is clean (it would be — that's the architecture's whole point).
4. **By Phase 12**, if payment migration feels too risky — STOP. NestJS payment is healthy. There's no shame in "Go owns everything except payment". Many real-world systems keep their oldest, most-battle-tested code exactly where it is.

---

## Decision log (fill in as you build)

| Date | Phase | Decision | Why |
|---|---|---|---|
| 2026-05-29 | — | Plan written; started none | Studying microservices + clean arch; not in a rush |
| | | | |
