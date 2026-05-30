# Go /rewrite Microservice — Clean Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract DraftRight's `/rewrite` endpoint into a standalone Go microservice running alongside the existing NestJS backend, using clean (hexagonal / ports-and-adapters) architecture. NestJS continues to own auth, payments, admin, schema migrations; Go owns only `/rewrite`. The two services share Postgres + Redis + JWT secret.

**Architecture:** 3-layer clean architecture inside a single Go module — `domain/` (entities + ports), `usecase/` (orchestration), `adapter/` (concrete I/O). Composition root in `cmd/server/main.go` wires adapters to use cases. No DI container; Go's "accept interfaces, return structs" idiom is the wiring mechanism. Schema stays NestJS-owned via TypeORM migrations; Go consumes it via `sqlc`-generated read-only bindings.

**Tech Stack:** Go 1.22+, chi (HTTP router), pgx/v5 + sqlc (typed Postgres), `golang-jwt/jwt/v5`, `redis/go-redis/v9`, `slog` (stdlib structured logging), `testify` (test helpers), Docker multi-stage build (distroless final image, ~15 MB), docker-compose service alongside NestJS.

---

## ⛔ Gate: Operational dual-service readiness must be solved before Task 4

Tasks 1–3 (scaffold + JWT + DB) are low risk and self-contained. **Task 4 onward exposes the service to real traffic.** Before that, two operational things MUST be solved:

1. **Schema-drift safeguard.** Adding `sqlc` to CI: any NestJS migration that changes a Go-read table must trigger `sqlc generate` + Go redeploy. Without this guard, schema drift causes silent runtime errors in Go.
2. **Deploy ordering.** Migrations apply → NestJS deploys → Go regenerates bindings → Go deploys. Encode in `scripts/deploy.sh` to prevent ordering accidents.

If either isn't ready when Task 4 begins, STOP and finish them first.

---

## Boundary rules (write down day 1, never violate)

1. **NestJS owns the schema.** Go uses `sqlc` to generate read bindings against the live Postgres. No Go-side migrations.
2. **JWT_SECRET shared via env.** Same hash check on both sides; rotate via dual-secret window or move to RS256 (NestJS signs with private key, Go verifies with public).
3. **No cross-service HTTP in the hot path.** Go talks directly to Postgres + Redis. Calling NestJS for quota check would defeat the purpose.
4. **Go service is stateless.** Anything stateful goes to Postgres or Redis. Restart-safe.
5. **Schema changes follow this order:** NestJS migration → deploy NestJS → `sqlc generate` → deploy Go.
6. **Domain package imports nothing from `adapter/` or `http/`.** Enforced via `internal/` + linter (`go-cleanarch`).
7. **Postgres role split.** Go uses `dr_rewrite` role: R on `users`/`plans`/`ai_providers`, RW on `usage_logs`. Cannot drop tables, cannot touch `payments`, `bug_reports`, `extension_tokens`.

---

## File Structure

```
backend-rewrite-go/                         # new sibling of /backend/
├── go.mod
├── go.sum
├── cmd/server/
│   └── main.go                             # composition root — wires adapters into use cases
├── internal/
│   ├── domain/                             # innermost — no internal deps
│   │   ├── user.go                         # type User struct, NewUser, CheckQuota
│   │   ├── rewrite_request.go              # type RewriteRequest, validation
│   │   ├── tone.go                         # enum-like consts
│   │   ├── errors.go                       # sentinel errors per domain concern
│   │   └── ports.go                        # interface UserRepo, AiProvider, UsageWriter, RateLimiter
│   ├── usecase/                            # depends only on domain
│   │   └── rewrite.go                      # func Rewrite(ctx, deps, req) (stream, error)
│   ├── adapter/
│   │   ├── pg/                             # implements domain.UserRepo via pgx
│   │   │   ├── pool.go
│   │   │   ├── user_repo.go
│   │   │   ├── usage_writer.go             # write-behind batch
│   │   │   └── sqlc/queries.sql + generated bindings
│   │   ├── openai/
│   │   │   ├── client.go                   # implements domain.AiProvider (streaming)
│   │   │   └── stream.go
│   │   ├── anthropic/
│   │   ├── ollama/
│   │   ├── redis_rate_limiter/             # implements domain.RateLimiter (token bucket)
│   │   └── memory/                         # in-memory adapters for tests
│   ├── http/
│   │   ├── router.go                       # chi router setup
│   │   ├── handler_rewrite.go              # POST /rewrite — SSE stream handler
│   │   ├── middleware_auth.go              # JWT extractor
│   │   ├── middleware_logging.go           # structured request logging
│   │   ├── middleware_recover.go           # panic recovery → 500 + log
│   │   └── middleware_correlation.go       # X-Request-ID propagation
│   └── platform/                           # cross-cutting infra
│       ├── config/config.go                # typed env via envconfig
│       ├── logger/logger.go                # slog setup, JSON output
│       └── ctxutil/                        # context helpers (user id, request id)
├── sqlc.yaml                               # sqlc config
├── queries.sql                             # SELECT/INSERT statements sqlc compiles
├── Dockerfile                              # multi-stage; distroless final
├── docker-compose.dev.yml                  # local dev (Postgres + Redis + this service)
├── deploy/
│   ├── service.yml                         # production docker-compose service block
│   └── Caddyfile.snippet                   # reverse proxy rule for /rewrite-go
├── scripts/
│   ├── deploy.sh                           # ordered deploy: migrations → NestJS → sqlc → Go
│   ├── sqlc-check.sh                       # CI gate: fails if generated bindings dirty
│   └── load-test.sh                        # k6 / vegeta scenario
├── test/
│   ├── integration/                        # end-to-end with real Postgres + httptest
│   └── load/                               # k6 scripts
├── Makefile
└── README.md                               # boundary rules + quick-start
```

**Existing files to modify (in main repo):**
- `Caddyfile` — add route for `/rewrite-go` (Task 10) and later `/rewrite` (Task 12).
- `deploy.sh` — ensure migration → NestJS → Go ordering enforced.
- Mobile clients (`DraftRightMobile/lib/services/backend_client.dart` etc.) — Task 11 adds an opt-in feature flag.

---

## Task 1: Scaffold + Hello World + Docker

**Files:**
- Create: `backend-rewrite-go/go.mod`, `cmd/server/main.go`, `Dockerfile`, `docker-compose.dev.yml`, `Makefile`, `.gitignore`, `README.md`
- Modify: none

- [ ] **Step 1: Init module**

```bash
mkdir backend-rewrite-go && cd backend-rewrite-go
go mod init github.com/tannpv/draftright-rewrite
mkdir -p cmd/server internal/{domain,usecase,adapter,http,platform}
```

- [ ] **Step 2: Write minimal `cmd/server/main.go`**

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"ok","service":"rewrite-go"}`))
    })
    mux.HandleFunc("/rewrite", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"text":"Hello from Go!","tone":"placeholder"}`))
    })
    srv := &http.Server{
        Addr:         ":3001",
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 60 * time.Second,
    }
    go func() {
        log.Info("listening", "addr", srv.Addr)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("server failed", "err", err)
        }
    }()
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    log.Info("shutting down")
    srv.Shutdown(ctx)
}
```

- [ ] **Step 3: Multi-stage Dockerfile**

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Run stage — distroless static for ~15 MB final image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/server /server
USER nonroot:nonroot
EXPOSE 3001
ENTRYPOINT ["/server"]
```

- [ ] **Step 4: `docker-compose.dev.yml`**

```yaml
services:
  rewrite-go:
    build: .
    ports:
      - "3001:3001"
    environment:
      - LOG_LEVEL=debug
    depends_on:
      - postgres
      - redis
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: draftright
      POSTGRES_USER: draftright
      POSTGRES_DB: draftright
    ports: ["5432:5432"]
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
```

- [ ] **Step 5: Run + verify**

```bash
docker compose -f docker-compose.dev.yml up --build
curl http://localhost:3001/health
# {"status":"ok","service":"rewrite-go"}
curl -X POST http://localhost:3001/rewrite
# {"text":"Hello from Go!","tone":"placeholder"}
```

- [ ] **Step 6: Commit**

```bash
git add backend-rewrite-go
git commit -m "feat(rewrite-go): scaffold + Docker + healthcheck"
```

---

## Task 2: JWT auth middleware (shared HS256 secret)

**Files:**
- Create: `internal/platform/auth/jwt.go`, `internal/http/middleware_auth.go`
- Modify: `cmd/server/main.go`, `internal/platform/config/config.go`
- Test: `internal/platform/auth/jwt_test.go`

- [ ] **Step 1: Add config package**

```go
// internal/platform/config/config.go
package config

import (
    "errors"
    "os"
)

type Config struct {
    JWTSecret       string
    DatabaseURL     string
    RedisURL        string
    OpenAIKey       string
    AnthropicKey    string
    Listen          string
}

func Load() (*Config, error) {
    c := &Config{
        JWTSecret:    os.Getenv("JWT_SECRET"),
        DatabaseURL:  os.Getenv("DATABASE_URL"),
        RedisURL:     os.Getenv("REDIS_URL"),
        OpenAIKey:    os.Getenv("OPENAI_API_KEY"),
        AnthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
        Listen:       envOr("LISTEN_ADDR", ":3001"),
    }
    if c.JWTSecret == "" {
        return nil, errors.New("JWT_SECRET required")
    }
    return c, nil
}

func envOr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

- [ ] **Step 2: JWT verifier**

```go
// internal/platform/auth/jwt.go
package auth

import (
    "errors"
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    Sub  string `json:"sub"`
    Role string `json:"role"`
    jwt.RegisteredClaims
}

type Verifier struct {
    secret []byte
}

func NewVerifier(secret string) *Verifier {
    return &Verifier{secret: []byte(secret)}
}

func (v *Verifier) Verify(token string) (*Claims, error) {
    parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("unexpected signing method")
        }
        return v.secret, nil
    })
    if err != nil { return nil, err }
    claims, ok := parsed.Claims.(*Claims)
    if !ok || !parsed.Valid { return nil, errors.New("invalid token") }
    if claims.Sub == "" { return nil, errors.New("missing sub claim") }
    return claims, nil
}
```

- [ ] **Step 3: Write the failing test**

```go
// internal/platform/auth/jwt_test.go
package auth

import (
    "testing"
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/stretchr/testify/require"
)

func TestVerifier_ValidToken(t *testing.T) {
    secret := "test-secret"
    v := NewVerifier(secret)
    // Sign a token NestJS-style
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
        Sub:  "user-123",
        Role: "user",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
        },
    })
    signed, err := token.SignedString([]byte(secret))
    require.NoError(t, err)
    claims, err := v.Verify(signed)
    require.NoError(t, err)
    require.Equal(t, "user-123", claims.Sub)
}

func TestVerifier_RejectsBadSignature(t *testing.T) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{Sub: "x"})
    bad, _ := token.SignedString([]byte("wrong"))
    _, err := NewVerifier("right").Verify(bad)
    require.Error(t, err)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/platform/auth/...
```

Expected: PASS for both cases.

- [ ] **Step 5: HTTP middleware that injects claims into context**

```go
// internal/http/middleware_auth.go
package http

import (
    "context"
    "net/http"
    "strings"
    "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

type ctxKey int
const userClaimsKey ctxKey = 1

func RequireAuth(v *auth.Verifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := r.Header.Get("Authorization")
            if !strings.HasPrefix(h, "Bearer ") {
                http.Error(w, `{"error":"missing bearer"}`, http.StatusUnauthorized)
                return
            }
            claims, err := v.Verify(strings.TrimPrefix(h, "Bearer "))
            if err != nil {
                http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), userClaimsKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
    c, ok := ctx.Value(userClaimsKey).(*auth.Claims)
    return c, ok
}
```

- [ ] **Step 6: Wire into main.go + commit**

```bash
git add . && git commit -m "feat(rewrite-go): JWT middleware sharing NestJS HS256 secret"
```

---

## Task 3: Postgres via pgx + sqlc bindings

**Files:**
- Create: `sqlc.yaml`, `queries.sql`, `internal/adapter/pg/pool.go`, `internal/adapter/pg/sqlc/` (generated)
- Modify: `Dockerfile` (sqlc not needed at runtime, generate at dev time), `Makefile`

- [ ] **Step 1: Install sqlc + golang-migrate locally**

```bash
brew install sqlc
go install github.com/jackc/pgx/v5@latest
```

- [ ] **Step 2: Write `sqlc.yaml`**

```yaml
version: "2"
sql:
  - schema: "../backend/sql"            # NestJS-owned migrations
    queries: "queries.sql"
    engine: "postgresql"
    gen:
      go:
        out: "internal/adapter/pg/sqlc"
        package: "sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

- [ ] **Step 3: Write `queries.sql` (start small)**

```sql
-- name: FindUserByID :one
SELECT id, email, role, plan_id, usage_today
FROM users
WHERE id = $1;

-- name: IncrementUserUsage :exec
UPDATE users SET usage_today = usage_today + 1 WHERE id = $1;

-- name: ActiveAIProvider :one
SELECT id, kind, api_key, model, is_active
FROM ai_providers
WHERE is_active = TRUE
ORDER BY priority ASC
LIMIT 1;

-- name: InsertUsageLog :exec
INSERT INTO usage_logs (id, user_id, provider_id, tone, input_chars, output_chars, latency_ms, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, NOW());
```

- [ ] **Step 4: Generate bindings**

```bash
sqlc generate
# generates internal/adapter/pg/sqlc/{queries.sql.go, models.go, db.go}
```

- [ ] **Step 5: Pool initializer**

```go
// internal/adapter/pg/pool.go
package pg

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(url)
    if err != nil { return nil, err }
    cfg.MaxConns = 25
    cfg.MinConns = 5
    return pgxpool.NewWithConfig(ctx, cfg)
}
```

- [ ] **Step 6: CI gate — `scripts/sqlc-check.sh`**

```bash
#!/bin/bash
set -e
sqlc generate
if ! git diff --exit-code -- internal/adapter/pg/sqlc/; then
    echo "ERROR: sqlc-generated files are dirty. Run sqlc generate locally and commit."
    exit 1
fi
```

Add to `.github/workflows/`.

- [ ] **Step 7: Commit**

```bash
git add . && git commit -m "feat(rewrite-go): pgx pool + sqlc-generated read bindings"
```

---

## Task 4: Domain layer (entities + ports)

**Files:**
- Create: `internal/domain/{user,rewrite_request,tone,errors,ports}.go`
- Test: `internal/domain/user_test.go`, `rewrite_request_test.go`

- [ ] **Step 1: Sentinel errors**

```go
// internal/domain/errors.go
package domain

import "errors"

var (
    ErrQuotaExceeded     = errors.New("quota exceeded")
    ErrUserNotFound      = errors.New("user not found")
    ErrProviderUnavailable = errors.New("no active ai provider")
    ErrInvalidInput      = errors.New("invalid input")
)
```

- [ ] **Step 2: User entity with invariants**

```go
// internal/domain/user.go
package domain

import "github.com/google/uuid"

type UserID uuid.UUID

type Plan struct {
    ID         string
    DailyLimit int
}

type User struct {
    ID         UserID
    Email      string
    Plan       Plan
    UsageToday int
}

// CheckQuota returns ErrQuotaExceeded if the user has hit their daily limit.
// Plan with DailyLimit == 0 is treated as unlimited.
func (u *User) CheckQuota() error {
    if u.Plan.DailyLimit > 0 && u.UsageToday >= u.Plan.DailyLimit {
        return ErrQuotaExceeded
    }
    return nil
}
```

- [ ] **Step 3: Rewrite request value object**

```go
// internal/domain/rewrite_request.go
package domain

import "strings"

type Tone string

const (
    ToneSimple    Tone = "simple"
    ToneNatural   Tone = "natural"
    TonePolished  Tone = "polished"
    ToneConcise   Tone = "concise"
    ToneTechnical Tone = "technical"
    ToneClaude    Tone = "claude"
    ToneTranslate Tone = "translate"
)

type RewriteRequest struct {
    Text string
    Tone Tone
    Lang string  // optional, for translate
}

func NewRewriteRequest(text string, tone string, lang string) (RewriteRequest, error) {
    text = strings.TrimSpace(text)
    if text == "" || len(text) > 5000 {
        return RewriteRequest{}, ErrInvalidInput
    }
    t := Tone(tone)
    switch t {
    case ToneSimple, ToneNatural, TonePolished, ToneConcise, ToneTechnical, ToneClaude, ToneTranslate:
    default:
        return RewriteRequest{}, ErrInvalidInput
    }
    return RewriteRequest{Text: text, Tone: t, Lang: lang}, nil
}
```

- [ ] **Step 4: Ports**

```go
// internal/domain/ports.go
package domain

import "context"

type UserRepo interface {
    Find(ctx context.Context, id UserID) (*User, error)
    IncrementUsage(ctx context.Context, id UserID) error
}

type AiProvider interface {
    // Stream returns a token channel + error channel. Both close when done.
    // ctx cancellation must abort the upstream call.
    Stream(ctx context.Context, req RewriteRequest) (<-chan string, <-chan error)
    Name() string  // for logging
}

type UsageWriter interface {
    Record(ctx context.Context, log UsageLog)
}

type UsageLog struct {
    UserID      UserID
    ProviderID  string
    Tone        Tone
    InputChars  int
    OutputChars int
    LatencyMs   int64
}

type RateLimiter interface {
    Allow(ctx context.Context, userID UserID) (bool, error)
}
```

- [ ] **Step 5: Tests**

```go
// internal/domain/user_test.go
package domain_test

import (
    "testing"
    "github.com/stretchr/testify/require"
    . "github.com/tannpv/draftright-rewrite/internal/domain"
)

func TestUser_CheckQuota_UnderLimit(t *testing.T) {
    u := &User{Plan: Plan{DailyLimit: 100}, UsageToday: 50}
    require.NoError(t, u.CheckQuota())
}

func TestUser_CheckQuota_AtLimit(t *testing.T) {
    u := &User{Plan: Plan{DailyLimit: 100}, UsageToday: 100}
    require.ErrorIs(t, u.CheckQuota(), ErrQuotaExceeded)
}

func TestUser_CheckQuota_Unlimited(t *testing.T) {
    u := &User{Plan: Plan{DailyLimit: 0}, UsageToday: 999999}
    require.NoError(t, u.CheckQuota())
}
```

- [ ] **Step 6: Run tests + commit**

```bash
go test ./internal/domain/...
git add . && git commit -m "feat(rewrite-go): domain entities + ports"
```

---

## Task 5: Use case (orchestration only)

**Files:**
- Create: `internal/usecase/rewrite.go`, `internal/usecase/rewrite_test.go`

- [ ] **Step 1: Use case function**

```go
// internal/usecase/rewrite.go
package usecase

import (
    "context"
    "fmt"
    "github.com/tannpv/draftright-rewrite/internal/domain"
)

type RewriteDeps struct {
    Users     domain.UserRepo
    Provider  domain.AiProvider
    Usage     domain.UsageWriter
    RateLimit domain.RateLimiter
}

// Rewrite orchestrates the full /rewrite flow. Returns a token stream + error
// channel that callers consume; both close when the request finishes.
func Rewrite(
    ctx context.Context,
    deps RewriteDeps,
    userID domain.UserID,
    req domain.RewriteRequest,
) (<-chan string, <-chan error, error) {
    // 1. Rate limit (cheap pre-check before DB read)
    if ok, err := deps.RateLimit.Allow(ctx, userID); err != nil {
        return nil, nil, fmt.Errorf("rate limit check: %w", err)
    } else if !ok {
        return nil, nil, domain.ErrQuotaExceeded
    }
    // 2. Load user, check plan quota
    user, err := deps.Users.Find(ctx, userID)
    if err != nil {
        return nil, nil, err
    }
    if err := user.CheckQuota(); err != nil {
        return nil, nil, err
    }
    // 3. Increment usage BEFORE the AI call. If the call fails, the user
    //    still gets debited — symmetric with NestJS behavior. Refunds are
    //    a future enhancement (Task 13).
    if err := deps.Users.IncrementUsage(ctx, userID); err != nil {
        return nil, nil, fmt.Errorf("increment usage: %w", err)
    }
    // 4. Stream
    tokens, errs := deps.Provider.Stream(ctx, req)
    return tokens, errs, nil
}
```

- [ ] **Step 2: Use case test with fake adapters**

```go
// internal/usecase/rewrite_test.go
package usecase_test
// (full test with fake UserRepo + fake AiProvider + fake RateLimiter
//  proving: quota exceeded path, happy path, provider failure propagation)
```

- [ ] **Step 3: Commit**

```bash
go test ./internal/usecase/...
git add . && git commit -m "feat(rewrite-go): rewrite use case orchestration"
```

---

## Task 6: Adapters — pg.UserRepo, openai.Provider, memory.RateLimiter

**Files:**
- Create: `internal/adapter/pg/user_repo.go`, `internal/adapter/pg/usage_writer.go`, `internal/adapter/openai/{client,stream}.go`, `internal/adapter/redis_rate_limiter/limiter.go`, `internal/adapter/memory/` (test fakes)

- [ ] **Step 1: PG UserRepo (uses sqlc bindings)**

```go
// internal/adapter/pg/user_repo.go
package pg
// Implements domain.UserRepo using sqlc bindings. Maps sqlc.User → domain.User.
// Look up plan join in a single query (LEFT JOIN plans).
```

- [ ] **Step 2: OpenAI streaming adapter**

```go
// internal/adapter/openai/client.go
// Implements domain.AiProvider.Stream:
//   POST https://api.openai.com/v1/chat/completions with stream:true
//   parses SSE chunks → forwards token strings to channel
//   ctx cancellation cancels in-flight HTTP request
//   closes channels on completion / error
```

- [ ] **Step 3: Redis rate limiter (token bucket)**

```go
// internal/adapter/redis_rate_limiter/limiter.go
// Implements domain.RateLimiter using INCR + EXPIRE pattern.
// 60 requests/min/user; matches NestJS throttler.
```

- [ ] **Step 4: In-memory adapters for tests**

```go
// internal/adapter/memory/{user_repo,provider,rate_limiter}.go
// Hand-written fakes used by usecase tests.
```

- [ ] **Step 5: Test each adapter against testcontainers Postgres / Redis**

```go
// internal/adapter/pg/user_repo_integration_test.go
// Spins up postgres:16-alpine via testcontainers-go,
// applies NestJS schema, runs UserRepo against it.
```

- [ ] **Step 6: Commit**

```bash
git add . && git commit -m "feat(rewrite-go): adapters — pg, openai, redis-rate-limiter, memory test fakes"
```

---

## Task 7: HTTP handler with SSE streaming

**Files:**
- Create: `internal/http/{router,handler_rewrite,middleware_logging,middleware_recover,middleware_correlation}.go`
- Modify: `cmd/server/main.go` (wire everything)

- [ ] **Step 1: SSE handler**

```go
// internal/http/handler_rewrite.go
package http

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "github.com/tannpv/draftright-rewrite/internal/domain"
    "github.com/tannpv/draftright-rewrite/internal/usecase"
)

type RewriteHandler struct {
    deps usecase.RewriteDeps
}

func NewRewriteHandler(deps usecase.RewriteDeps) *RewriteHandler {
    return &RewriteHandler{deps: deps}
}

type rewriteRequestBody struct {
    Text string `json:"text"`
    Tone string `json:"tone"`
    Lang string `json:"lang,omitempty"`
}

func (h *RewriteHandler) Handle(w http.ResponseWriter, r *http.Request) {
    claims, ok := ClaimsFromContext(r.Context())
    if !ok {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
        return
    }
    var body rewriteRequestBody
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
        return
    }
    req, err := domain.NewRewriteRequest(body.Text, body.Tone, body.Lang)
    if err != nil {
        http.Error(w, `{"error":"invalid input"}`, http.StatusBadRequest)
        return
    }
    userID, err := parseUUID(claims.Sub)
    if err != nil {
        http.Error(w, `{"error":"bad sub"}`, http.StatusUnauthorized)
        return
    }
    tokens, errs, err := usecase.Rewrite(r.Context(), h.deps, domain.UserID(userID), req)
    if err != nil {
        switch err {
        case domain.ErrQuotaExceeded:
            http.Error(w, `{"error":"quota exceeded"}`, http.StatusTooManyRequests)
        case domain.ErrProviderUnavailable:
            http.Error(w, `{"error":"no provider"}`, http.StatusServiceUnavailable)
        default:
            http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
        }
        return
    }
    // SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
        return
    }
    // Stream tokens
    for {
        select {
        case <-r.Context().Done():
            return
        case tok, open := <-tokens:
            if !open {
                fmt.Fprint(w, "event: done\ndata: {}\n\n")
                flusher.Flush()
                return
            }
            fmt.Fprintf(w, "data: %s\n\n", jsonString(tok))
            flusher.Flush()
        case e := <-errs:
            if e != nil {
                fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonString(e.Error()))
                flusher.Flush()
                return
            }
        case <-time.After(30 * time.Second):
            // Heartbeat keeps proxies from killing the connection
            fmt.Fprint(w, ": keepalive\n\n")
            flusher.Flush()
        }
    }
}
```

- [ ] **Step 2: Router + middleware stack**

```go
// internal/http/router.go
package http

import (
    "github.com/go-chi/chi/v5"
    "github.com/tannpv/draftright-rewrite/internal/platform/auth"
    "github.com/tannpv/draftright-rewrite/internal/usecase"
)

func NewRouter(deps usecase.RewriteDeps, verifier *auth.Verifier) *chi.Mux {
    r := chi.NewRouter()
    r.Use(MiddlewareCorrelation)
    r.Use(MiddlewareLogging)
    r.Use(MiddlewareRecover)
    r.Get("/health", healthHandler)
    r.Group(func(r chi.Router) {
        r.Use(RequireAuth(verifier))
        h := NewRewriteHandler(deps)
        r.Post("/rewrite", h.Handle)
    })
    return r
}
```

- [ ] **Step 3: Wire in main.go**

- [ ] **Step 4: End-to-end test against the real handler**

```go
// internal/http/handler_rewrite_test.go
// Uses httptest.NewServer + in-memory adapters; signs a real JWT and
// hits POST /rewrite, asserts SSE chunks.
```

- [ ] **Step 5: Commit**

```bash
git add . && git commit -m "feat(rewrite-go): SSE handler + chi router + middleware stack"
```

---

## Task 8: Provider failover (Anthropic, Ollama)

**Files:**
- Create: `internal/adapter/anthropic/client.go`, `internal/adapter/ollama/client.go`, `internal/usecase/provider_chain.go`
- Modify: `cmd/server/main.go` (chain composition)

- [ ] **Step 1: Anthropic adapter (mirror OpenAI shape, different endpoint + auth header)**

- [ ] **Step 2: Ollama adapter (localhost or remote)**

- [ ] **Step 3: `ProviderChain` — wraps multiple providers, falls back on error**

```go
// internal/usecase/provider_chain.go
type ProviderChain struct {
    primary  domain.AiProvider
    fallbacks []domain.AiProvider
}

// Stream tries primary first; on 429/503 (rate limit / unavailable),
// retries with next fallback. Permanent errors (400/401) bubble up.
```

- [ ] **Step 4: Test failover with fake providers that return errors**

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(rewrite-go): multi-provider failover chain"
```

---

## Task 9: Observability (slog + Prometheus + tracing)

**Files:**
- Create: `internal/platform/metrics/metrics.go`, `internal/http/middleware_metrics.go`

- [ ] **Step 1: Structured slog with request ID**
- [ ] **Step 2: Prometheus `/metrics` endpoint (use `promhttp.Handler()`)**
- [ ] **Step 3: HTTP middleware: histogram of request duration by route + status**
- [ ] **Step 4: Counter: rewrite_provider_calls_total{provider,status}**
- [ ] **Step 5: Histogram: rewrite_provider_latency_seconds**
- [ ] **Step 6: Wire into Grafana** (or just `curl localhost:3001/metrics` for now)

```bash
git commit -m "feat(rewrite-go): slog + Prometheus metrics"
```

---

## Task 10: Caddy side-by-side routing + production deploy

**Files:**
- Create: `deploy/Caddyfile.snippet`, `deploy/service.yml` (docker-compose block)
- Modify: production `Caddyfile`, `docker-compose.prod.yml`

- [ ] **Step 1: Caddy snippet — route `/rewrite-go` to Go service, leave `/rewrite` on NestJS**

```caddyfile
# Inside api.draftright.info { ... }
@rewriteGo path /rewrite-go
handle @rewriteGo {
    rewrite * /rewrite
    reverse_proxy rewrite-go:3001
}
```

- [ ] **Step 2: docker-compose.prod.yml — add `rewrite-go` service block**

- [ ] **Step 3: Deploy script**

```bash
# scripts/deploy.sh
sudo docker compose -f docker-compose.prod.yml up -d --build rewrite-go
```

- [ ] **Step 4: Smoke test on prod**

```bash
TOKEN=$(curl ... /auth/login | jq -r .access_token)
curl -N -X POST https://api.draftright.info/rewrite-go \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text":"hello","tone":"polished"}'
# Expect: SSE stream
```

- [ ] **Step 5: Commit**

```bash
git commit -m "deploy(rewrite-go): Caddy side-by-side route + production compose"
```

---

## Task 11: Client-side feature flag (opt-in A/B)

**Files:**
- Modify: `DraftRightMobile/lib/services/backend_client.dart`, settings UI

- [ ] **Step 1: Add a hidden settings toggle "Use experimental rewrite service (beta)"**
- [ ] **Step 2: When enabled, BackendClient.rewrite() hits `/rewrite-go` instead of `/rewrite`**
- [ ] **Step 3: Both endpoints return the same response shape so the rest of the client is unchanged**
- [ ] **Step 4: Ship to 1 user (you) for a week. Compare latency + reliability vs NestJS**

```bash
git commit -m "feat(mobile): opt-in feature flag for Go /rewrite service"
```

---

## Task 12: Full cutover (when proven over 2+ weeks of A/B)

**Files:**
- Modify: Caddy routes (`/rewrite` → Go service), NestJS RewriteController removed

- [ ] **Step 1: Update Caddyfile — `/rewrite` now routes to Go service. `/rewrite-go` becomes an alias.**
- [ ] **Step 2: Add deprecation warning to NestJS RewriteController**
- [ ] **Step 3: Soak for 1 week. Monitor /errors + latency.**
- [ ] **Step 4: Remove NestJS RewriteController + RewriteModule. Backend rebuild.**

```bash
git commit -m "deploy(rewrite-go): full cutover — Go service owns /rewrite, NestJS controller removed"
```

---

## Task 13 (future, not in this plan)

- Refund usage on AI provider failure
- Streaming usage receipt (`X-DR-Usage` trailer header)
- Per-user cost tracking with billing reconciliation
- gRPC interface for high-frequency clients
- Move JWT to RS256 (NestJS signs with private key, Go verifies with public)
- Read replica for `users` lookups
- Consider extracting `/errors` + `/bug-reports` to Go if they ever become the bottleneck (don't volunteer)

---

## Quick-reference: weekend pacing

| Weekend | Tasks | Hours | Outcome |
|---|---|---|---|
| 1 | Task 1 (scaffold) | 4-6 | `curl :3001/health` returns ok; Docker image builds |
| 2 | Task 2 (JWT) | 4-6 | Real NestJS-signed token verifies against Go service |
| 3 | Task 3 (sqlc + pgx) | 6-8 | `SELECT id FROM users WHERE id = $1` works from Go |
| 4 | Task 4 (domain) | 3-4 | Pure logic + tests; no Go-specific surprises |
| 5 | Task 5 (use case) | 4-6 | Faked adapters; happy + quota paths green |
| 6 | Task 6 (real adapters) | 8-12 | OpenAI call returns text; pg writes usage_logs |
| 7 | Task 7 (HTTP + SSE) | 8-12 | curl streams Go-generated tokens to terminal |
| 8 | Task 8 (failover) | 4-6 | OpenAI 429 → Anthropic returns; logged |
| 9 | Task 9 (observability) | 4-6 | `/metrics` endpoint; structured logs in JSON |
| 10 | Task 10 (deploy) | 4-8 | `/rewrite-go` live on prod, smoke-tested |
| 11 | Task 11 (feature flag) | 4 | Mobile flag flips, you try it for a week |
| 12-14 | Soak + watch | — | Real-world A/B observation |
| 15 | Task 12 (cutover) | 4 | Caddy flips `/rewrite` to Go, NestJS controller deprecated |

**Total: ~60-90 hours over ~3-4 months at one weekend/week.** Not a sprint. Each weekend leaves something working you can show off + revisit.

---

## Self-Review

- **Boundary discipline**: domain/ imports nothing outside itself; usecase/ depends only on domain/; adapters/ depend on domain/; http/ + main.go depend on all. Compile-time enforced through Go's import-cycle prohibition + `internal/` visibility.
- **Schema ownership**: NestJS owns `backend/sql/*.sql` migrations. Go's `sqlc generate` is downstream, gated in CI (Task 3 step 6).
- **JWT sync**: shared HS256 secret today (Task 2); RS256 upgrade path noted in Task 13.
- **Production safety**: parallel deploy (Task 10) → opt-in flag (Task 11) → soak (3 weeks) → cutover (Task 12). NestJS owns `/rewrite` until the Go service has run side-by-side for at least 2 weeks with matching latency/reliability.
- **Test coverage**: domain has 100% (pure logic); usecase has 100% via in-memory adapters; adapters have integration tests against testcontainers; HTTP layer has handler tests with httptest.
- **Rollback path**: any commit can be reverted; Caddy snippet swap is a 1-line change; NestJS RewriteController stays in git history for emergency revival.
- **Operational complexity tax**: one more Docker image, one more health check, one more metrics endpoint, one more log stream. Budget ~2 hours/month maintenance once stable.
- **Hard rule**: this is the ONLY service moving to Go for now. No "while I'm here let's also move /errors." Boil-the-frog risk acknowledged + resisted.

---

## When to abandon this plan

Honest stop conditions:

1. **By Task 4, if you've spent more time fighting Go's type system than learning anything new** — pause, do a Go-Tour week, resume.
2. **By Task 7, if /rewrite latency in Go isn't materially better than NestJS** — that's fine, the value was learning. Keep the service for the experience; don't cut over.
3. **If maintenance burden of two languages starts crowding feature work** — pause new tasks, keep existing Go service running in parallel as a learning artifact.

---

## Decision log (fill in as you build)

| Date | Decision | Why |
|---|---|---|
| 2026-05-29 | Plan written; started none of it | Studying microservices, not in a rush |
| | | |
