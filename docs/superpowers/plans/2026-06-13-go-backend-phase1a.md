# Go Backend Phase 1a — Users + Auth-core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop-in Go port of password auth — `POST /auth/login`, `POST /auth/refresh`, `POST /auth/change-password`, `GET /auth/account`, `DELETE /auth/account` — byte-identical to the NestJS backend.

**Architecture:** Package-by-feature modules (`user`, `auth`, `subscription`, `usage`) plus `platform/settings`, depending downward on `shared`/`platform` only; cross-module wiring in `cmd/server/main.go`. Reuses the Phase 0 error envelope, JWT `Signer`/`Verifier`, request-id middleware, and sqlc-over-live-schema setup. TDD per task, frequent commits.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc, golang-jwt/jwt/v5, golang.org/x/crypto/bcrypt, slog, testify.

**Module path:** `github.com/tannpv/draftright-rewrite` — all imports below use this prefix. Work in `/opt/openAi/DraftRight/backend-rewrite-go` (git root is the `/opt/openAi/DraftRight` monorepo). Branch: `feature/go-backend-phase1a-spec-20260613` (already checked out). Never commit to `main`/`develop`.

**Spec:** `docs/superpowers/specs/2026-06-13-go-backend-phase1a-design.md`.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/shared/password.go` (+`_test.go`) | bcrypt hash/verify, cost 10, bcryptjs-compatible |
| `internal/platform/config/config.go` (modify) | add `JWTRefreshSecret` |
| `internal/platform/settings/reader.go` (+`_test.go`) | AppSettings token-TTL reader, defaults 15m/90d |
| `internal/shared/pg/queries_auth.sql` | new sqlc queries (user/sub/usage/settings) |
| `internal/user/domain.go` | `User` struct |
| `internal/user/service.go` (+`_test.go`) | `Service`: ByEmail/ByID/UpdatePasswordHash/DeleteAccount |
| `internal/user/repo_pg.go` (+`_test.go`) | sqlc-backed repo + delete-cascade txn |
| `internal/subscription/reader.go` (+`_test.go`) | `Reader`: ActiveByUser → `AccountSub` |
| `internal/usage/counter.go` (+`_test.go`) | `Counter`: CountToday |
| `internal/auth/errors.go` | exact Node 401 message constants + `providerLabel` |
| `internal/auth/usecase.go` (+`_test.go`) | `Service`: login/refresh/changePassword/account/delete orchestration + token mint |
| `internal/auth/handler.go` (+`_test.go`) | 5 chi http.Handlers |
| `internal/shared/router.go` (modify) | mount the 5 routes |
| `cmd/server/main.go` (modify) | wire modules when `pool != nil` |
| `cmd/shadowdiff/fixtures/*.json` | login/refresh/change-password/account/delete fixtures |

Confirmed existing helpers (do NOT reimplement): `shared.WriteJSON(w, status, v)`, `shared.WriteError(w, r, code, message)` (status from `StatusForCode`; `"invalid-token"`→401, `"invalid-input"`→400), `shared.RequestIDFromContext(ctx)`, `shared.RequireAuth(v, log)`, `shared.ClaimsFromContext(ctx)`, `auth.Claims{Sub,Email,Role}`, `auth.NewSigner(secret).Sign(claims, ttl)`, `auth.NewVerifier(secret).Verify(token)`, sqlc generated under `internal/shared/pg/sqlc`.

---

### Task 1: Password hashing (bcrypt, bcryptjs-compatible)

**Files:**
- Create: `internal/shared/password.go`
- Test: `internal/shared/password_test.go`

- [ ] **Step 1: Add dependency**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && go get golang.org/x/crypto/bcrypt@latest`
Expected: `go.mod` gains `golang.org/x/crypto`.

- [ ] **Step 2: Write the failing test**

```go
package shared_test

import (
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Hash produced by Node bcryptjs: bcrypt.hashSync("correct horse battery staple", 10)
// Pinned to prove Go's x/crypto/bcrypt verifies hashes the NestJS backend wrote.
const nodeBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func TestVerifyPassword_AcceptsNodeGeneratedHash(t *testing.T) {
	ok, err := shared.VerifyPassword("correct horse battery staple", nodeBcryptHash)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatal("Go bcrypt rejected a valid bcryptjs hash — cross-compat broken")
	}
}

func TestVerifyPassword_RejectsWrongPassword(t *testing.T) {
	ok, _ := shared.VerifyPassword("wrong", nodeBcryptHash)
	if ok {
		t.Fatal("verify accepted wrong password")
	}
}

func TestHashPassword_RoundTrips(t *testing.T) {
	h, err := shared.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := shared.VerifyPassword("s3cret", h)
	if err != nil || !ok {
		t.Fatalf("round-trip failed: ok=%v err=%v", ok, err)
	}
}
```

> NOTE: The hash constant above is illustrative. Before relying on it, the implementer MUST regenerate a real vector: run `node -e 'console.log(require("bcryptjs").hashSync("correct horse battery staple",10))'` in `/opt/openAi/DraftRight/backend` and paste the actual output into `nodeBcryptHash`. A fabricated hash will fail verification and that is the point of the test.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/shared/ -run TestVerifyPassword -v`
Expected: FAIL — `undefined: shared.VerifyPassword`.

- [ ] **Step 4: Write minimal implementation**

```go
// Package-level note (file: internal/shared/password.go).
package shared

import "golang.org/x/crypto/bcrypt"

// bcryptCost matches the NestJS BCRYPT_ROUNDS constant (10). bcryptjs
// and x/crypto/bcrypt share the Modular Crypt Format ($2a$/$2b$), so
// hashes written by either verify under the other — no migration.
const bcryptCost = 10

// HashPassword returns a bcrypt MCF hash for plain.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
// A mismatch returns (false, nil); only malformed hashes return an error.
func VerifyPassword(plain, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err == nil {
		return true, nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	return false, err
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/shared/ -run "Password" -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/shared/password.go internal/shared/password_test.go
git commit -m "feat(go): bcrypt password hash/verify (cost 10, bcryptjs-compatible)"
```

---

### Task 2: Config — JWT_REFRESH_SECRET

**Files:**
- Modify: `internal/platform/config/config.go`
- Test: `internal/platform/config/config_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"os"
	"testing"
)

func TestLoad_ReadsRefreshSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "access-secret")
	os.Setenv("JWT_REFRESH_SECRET", "refresh-secret")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("JWT_REFRESH_SECRET")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.JWTRefreshSecret != "refresh-secret" {
		t.Fatalf("JWTRefreshSecret = %q, want refresh-secret", c.JWTRefreshSecret)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/config/ -run RefreshSecret -v`
Expected: FAIL — `c.JWTRefreshSecret undefined`.

- [ ] **Step 3: Add the field + load it**

In `config.go`, add to the `Config` struct near `JWTSecret`:

```go
	// HS256 secret for REFRESH tokens. Mirrors the NestJS
	// JWT_REFRESH_SECRET. Distinct from JWTSecret (access tokens) —
	// /auth/refresh verifies inbound refresh tokens with this.
	JWTRefreshSecret string
```

In `Load()`'s struct literal, add after `JWTSecret`:

```go
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
```

> Do NOT add it to `validate()` as globally-required — auth endpoints are only mounted when a DB pool exists (Task 11). main.go validates its presence at wire time instead, so the rewrite-only deployment still boots without it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/config/ -run RefreshSecret -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/config/config.go internal/platform/config/config_test.go
git commit -m "feat(go): config JWT_REFRESH_SECRET for refresh-token signing"
```

---

### Task 3: sqlc queries for auth (user, subscription, usage, settings)

**Files:**
- Create: `internal/shared/pg/queries_auth.sql`
- Modify: `sqlc.yaml` (add the new query file)

- [ ] **Step 1: Write the queries**

Create `internal/shared/pg/queries_auth.sql`:

```sql
-- Phase 1a auth queries. Read the live NestJS-owned schema as-is.

-- name: GetAuthUserByEmail :one
-- Full projection for /auth/login. password_hash is nullable
-- (social-only accounts have none). No email normalization — Node's
-- login passes the email through unchanged.
SELECT id, email, password_hash, name, is_active, role,
       auth_provider, email_verified, lemonsqueezy_customer_id
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetAuthUserByID :one
-- Same projection keyed by id — used by refresh, change-password, account.
SELECT id, email, password_hash, name, is_active, role,
       auth_provider, email_verified, lemonsqueezy_customer_id
FROM users
WHERE id = $1
LIMIT 1;

-- name: UpdateUserPasswordHash :exec
UPDATE users SET password_hash = $2, updated_at = now()
WHERE id = $1;

-- name: GetActiveSubscriptionByUserID :one
-- Mirrors subscriptionsService.findActiveByUserId: newest ACTIVE
-- subscription for the user, joined to its plan. ORDER BY created_at
-- DESC + LIMIT 1 reproduces TypeORM order:{created_at:'DESC'} findOne.
SELECT s.status, s.store_type, s.started_at, s.expires_at,
       p.name AS plan_name, p.daily_limit
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'
ORDER BY s.created_at DESC
LIMIT 1;

-- name: CountUsageToday :one
-- Mirrors usageService.countTodayByUser: rows since local midnight.
-- The caller passes the midnight boundary so timezone handling matches
-- the Node process (new Date(); setHours(0,0,0,0)).
SELECT COUNT(*) FROM usage_logs
WHERE user_id = $1 AND created_at >= $2;

-- name: GetAuthTokenSettings :one
-- The single app_settings row's token lifetimes. No row → caller uses
-- defaults (15 / 90), matching Node's `?? 15` / `?? 90`.
SELECT token_expiry_minutes, refresh_token_expiry_days
FROM app_settings
LIMIT 1;
```

- [ ] **Step 2: Register the query file in sqlc.yaml**

In `sqlc.yaml`, under `queries:`, add the line:

```yaml
      - "internal/shared/pg/queries_auth.sql"
```

- [ ] **Step 3: Generate + verify it compiles against the schema**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && sqlc generate`
Expected: no error; new methods appear in `internal/shared/pg/sqlc/queries_auth.sql.go`. If a column/type mismatch errors, fix the SQL to match `internal/platform/db/schema.sql` (e.g. enum casts).

Run: `go build ./internal/shared/pg/...`
Expected: builds.

> If `sqlc` is not installed: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`.

- [ ] **Step 4: Commit**

```bash
git add internal/shared/pg/queries_auth.sql sqlc.yaml internal/shared/pg/sqlc/
git commit -m "feat(go): sqlc auth queries (user/subscription/usage/token-settings)"
```

---

### Task 4: AppSettings token-TTL reader (platform/settings)

**Files:**
- Create: `internal/platform/settings/reader.go`
- Test: `internal/platform/settings/reader_test.go`

- [ ] **Step 1: Write the failing test**

```go
package settings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tannpv/draftright-rewrite/internal/platform/settings"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	row sqlc.GetAuthTokenSettingsRow
	err error
}

func (f fakeQ) GetAuthTokenSettings(ctx context.Context) (sqlc.GetAuthTokenSettingsRow, error) {
	return f.row, f.err
}

func TestTTLs_FromRow(t *testing.T) {
	r := settings.NewReader(fakeQ{row: sqlc.GetAuthTokenSettingsRow{TokenExpiryMinutes: 30, RefreshTokenExpiryDays: 7}})
	a, rf, err := r.TokenTTLs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != 30*time.Minute || rf != 7*24*time.Hour {
		t.Fatalf("got %v / %v", a, rf)
	}
}

func TestTTLs_DefaultsWhenNoRow(t *testing.T) {
	r := settings.NewReader(fakeQ{err: pgx.ErrNoRows})
	a, rf, err := r.TokenTTLs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a != 15*time.Minute || rf != 90*24*time.Hour {
		t.Fatalf("defaults wrong: %v / %v", a, rf)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/settings/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement**

```go
// Package settings reads admin-configurable app_settings values that
// other modules need at request time. Read-only in Phase 1a; the admin
// module (Phase 4) owns writes.
package settings

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Default token lifetimes — applied when the app_settings row is absent,
// matching Node's `settings?.token_expiry_minutes ?? 15` /
// `?? 90`.
const (
	defaultAccessMinutes = 15
	defaultRefreshDays   = 90
)

// Querier is the sqlc subset this reader needs (one method) — narrow
// interface so tests fake it without a DB.
type Querier interface {
	GetAuthTokenSettings(ctx context.Context) (sqlc.GetAuthTokenSettingsRow, error)
}

// Reader resolves token TTLs from app_settings.
type Reader struct{ q Querier }

// NewReader wires the sqlc querier (or any Querier).
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// TokenTTLs returns (accessTTL, refreshTTL). Missing row → defaults.
func (r *Reader) TokenTTLs(ctx context.Context) (time.Duration, time.Duration, error) {
	row, err := r.q.GetAuthTokenSettings(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultAccessMinutes * time.Minute, defaultRefreshDays * 24 * time.Hour, nil
	}
	if err != nil {
		return 0, 0, err
	}
	accMin := int(row.TokenExpiryMinutes)
	if accMin <= 0 {
		accMin = defaultAccessMinutes
	}
	refDays := int(row.RefreshTokenExpiryDays)
	if refDays <= 0 {
		refDays = defaultRefreshDays
	}
	return time.Duration(accMin) * time.Minute, time.Duration(refDays) * 24 * time.Hour, nil
}
```

> If sqlc emits the columns as `int32`, `int(row.TokenExpiryMinutes)` is correct. If it emits pointers (NOT NULL columns shouldn't), drop the pointer. Adjust to the generated type.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/settings/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/settings/
git commit -m "feat(go): app_settings token-TTL reader (defaults 15m/90d)"
```

---

### Task 5: user module — domain + service + repo (reads + password update)

**Files:**
- Create: `internal/user/domain.go`, `internal/user/service.go`, `internal/user/repo_pg.go`
- Test: `internal/user/service_test.go`

- [ ] **Step 1: Write the domain + repo interface (no test yet — pure types)**

`internal/user/domain.go`:

```go
// Package user owns the customer identity record. Phase 1a exposes
// reads + password update + account deletion; register/verification
// land in Phase 1b.
package user

// User is the domain projection used by auth. PasswordHash is empty for
// social-only accounts (no local password).
type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	Name                 string
	IsActive             bool
	Role                 string
	AuthProvider         string // local | google | facebook | apple
	EmailVerified        bool
	LemonsqueezyCustomer string // "" when none
}
```

`internal/user/service.go`:

```go
package user

import (
	"context"
	"errors"
)

// ErrNotFound is returned when no user matches the lookup.
var ErrNotFound = errors.New("user: not found")

// Repo is the persistence port. Implemented by repo_pg in prod and by
// fakes in tests.
type Repo interface {
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id string) (User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
}

// Service is the thin domain entry point. It adds no logic over the
// repo in 1a but gives auth a stable interface and a home for future
// invariants (Phase 1b register/verify).
type Service struct{ repo Repo }

// NewService wires a Repo.
func NewService(repo Repo) *Service { return &Service{repo: repo} }

func (s *Service) ByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.ByEmail(ctx, email)
}
func (s *Service) ByID(ctx context.Context, id string) (User, error) {
	return s.repo.ByID(ctx, id)
}
func (s *Service) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	return s.repo.UpdatePasswordHash(ctx, id, hash)
}
func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	return s.repo.DeleteAccount(ctx, id)
}
```

- [ ] **Step 2: Write the failing service test (with a fake repo)**

`internal/user/service_test.go`:

```go
package user_test

import (
	"context"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

type fakeRepo struct {
	byEmail map[string]user.User
	updated map[string]string
	deleted []string
}

func (f *fakeRepo) ByEmail(_ context.Context, e string) (user.User, error) {
	u, ok := f.byEmail[e]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (f *fakeRepo) ByID(_ context.Context, id string) (user.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return user.User{}, user.ErrNotFound
}
func (f *fakeRepo) UpdatePasswordHash(_ context.Context, id, h string) error {
	if f.updated == nil {
		f.updated = map[string]string{}
	}
	f.updated[id] = h
	return nil
}
func (f *fakeRepo) DeleteAccount(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func TestService_ByEmail_HitAndMiss(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{"a@b.com": {ID: "u1", Email: "a@b.com"}}}
	s := user.NewService(r)
	if u, err := s.ByEmail(context.Background(), "a@b.com"); err != nil || u.ID != "u1" {
		t.Fatalf("hit failed: %v %v", u, err)
	}
	if _, err := s.ByEmail(context.Background(), "missing@b.com"); err != user.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestService_UpdatePasswordHash(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	if err := s.UpdatePasswordHash(context.Background(), "u1", "newhash"); err != nil {
		t.Fatal(err)
	}
	if r.updated["u1"] != "newhash" {
		t.Fatalf("hash not updated: %v", r.updated)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/user/ -v`
Expected: FAIL — package `user` not built / missing symbols (until domain.go + service.go compile).

- [ ] **Step 4: Implement the pg repo**

`internal/user/repo_pg.go`:

```go
package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// pgQuerier is the sqlc subset the read/update methods need.
type pgQuerier interface {
	GetAuthUserByEmail(ctx context.Context, email string) (sqlc.GetAuthUserByEmailRow, error)
	GetAuthUserByID(ctx context.Context, id pgtype.UUID) (sqlc.GetAuthUserByIDRow, error)
	UpdateUserPasswordHash(ctx context.Context, arg sqlc.UpdateUserPasswordHashParams) error
}

// PgRepo implements Repo over Postgres. The delete-cascade txn needs the
// pool directly (multi-statement), so PgRepo holds both.
type PgRepo struct {
	q    pgQuerier
	pool DeleteExecer
}

// NewPgRepo wires the sqlc querier + a transaction-capable executor.
func NewPgRepo(q pgQuerier, pool DeleteExecer) *PgRepo { return &PgRepo{q: q, pool: pool} }

func (r *PgRepo) ByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetAuthUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: row.ID.String(), Email: row.Email, PasswordHash: pgTextStr(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: pgTextStr(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) ByID(ctx context.Context, id string) (User, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return User{}, ErrNotFound
	}
	row, err := r.q.GetAuthUserByID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return User{
		ID: row.ID.String(), Email: row.Email, PasswordHash: pgTextStr(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: pgTextStr(row.LemonsqueezyCustomerID),
	}, nil
}

func (r *PgRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	return r.q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
		ID: uid, PasswordHash: pgText(hash),
	})
}

// helpers — adjust to the actual generated nullable types (sqlc with
// emit_pointers_for_null_types may emit *string instead of pgtype.Text;
// if so, replace pgTextStr/pgText accordingly).
func pgTextStr(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}
func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}
```

> The exact nullable-column Go type depends on what `sqlc generate` emitted in Task 3. Open `internal/shared/pg/sqlc/queries_auth.sql.go`, read the `GetAuthUserByEmailRow` struct, and match `PasswordHash` / `LemonsqueezyCustomerID` types (likely `*string` given `emit_pointers_for_null_types: true`). If they are `*string`, replace `pgTextStr(t)` with `func derefStr(p *string) string { if p==nil {return ""}; return *p }` and `pgText` with a `*string`. The DeleteAccount method is added in Task 6 — leave `DeleteExecer` undefined for now by temporarily commenting the `pool` field, OR implement Task 6 before running. (Recommended: do Task 6's `repo_delete.go` in the same edit so the package compiles.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/user/ -run "Service" -v`
Expected: PASS (after Task 6 supplies `DeleteExecer`/`DeleteAccount`; if doing strictly in order, stub `DeleteAccount` to `return nil` and `DeleteExecer` as `any`, then replace in Task 6).

- [ ] **Step 6: Commit**

```bash
git add internal/user/domain.go internal/user/service.go internal/user/repo_pg.go internal/user/service_test.go
git commit -m "feat(go): user module — domain, service, pg read/update repo"
```

---

### Task 6: user account-deletion cascade (single txn, order-tested)

**Files:**
- Create: `internal/user/repo_delete.go`, `internal/user/repo_delete_test.go`

- [ ] **Step 1: Write the failing test (recording executor asserts statement order + abort-on-error)**

`internal/user/repo_delete_test.go`:

```go
package user

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recExecer struct {
	stmts   []string
	failOn  int // index to fail at; -1 never
	calls   int
}

func (r *recExecer) Exec(_ context.Context, sql string, _ ...any) error {
	r.calls++
	r.stmts = append(r.stmts, sql)
	if r.failOn >= 0 && len(r.stmts)-1 == r.failOn {
		return errors.New("boom")
	}
	return nil
}

func TestDeleteAccountStmts_OrderAndTables(t *testing.T) {
	r := &recExecer{failOn: -1}
	if err := deleteAccountStmts(context.Background(), r, "u1"); err != nil {
		t.Fatal(err)
	}
	wantContains := []string{
		"extension_tokens", "payments", "usage_logs", "subscriptions",
		"feature_votes", "bug_reports", "error_reports", "FROM users",
	}
	if len(r.stmts) != len(wantContains) {
		t.Fatalf("ran %d stmts, want %d", len(r.stmts), len(wantContains))
	}
	for i, frag := range wantContains {
		if !strings.Contains(r.stmts[i], frag) {
			t.Fatalf("stmt[%d]=%q missing %q (order matters)", i, r.stmts[i], frag)
		}
	}
}

func TestDeleteAccountStmts_AbortsOnError(t *testing.T) {
	r := &recExecer{failOn: 2} // fail on usage_logs delete
	err := deleteAccountStmts(context.Background(), r, "u1")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if r.calls != 3 {
		t.Fatalf("ran %d stmts after failure, want 3 (stops at failure)", r.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/user/ -run DeleteAccountStmts -v`
Expected: FAIL — `undefined: deleteAccountStmts`.

- [ ] **Step 3: Implement the cascade + pool wrapper**

`internal/user/repo_delete.go`:

```go
package user

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// execer runs one statement. Both pgx.Tx and the recording test fake
// satisfy it (via the adapter below), so the cascade is unit-tested
// without a database.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) error
}

// DeleteExecer is the production seam: something that can open a
// transaction. *pgxpool.Pool satisfies it.
type DeleteExecer interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// deleteAccountStmts runs the 8-statement cascade in the exact order the
// NestJS usersService.deleteAccount uses. Any error aborts immediately
// (caller rolls back).
func deleteAccountStmts(ctx context.Context, e execer, userID string) error {
	stmts := []string{
		`DELETE FROM extension_tokens WHERE user_id = $1`,
		`DELETE FROM payments WHERE user_id = $1`,
		`DELETE FROM usage_logs WHERE user_id = $1`,
		`DELETE FROM subscriptions WHERE user_id = $1`,
		`DELETE FROM feature_votes WHERE user_id = $1`,
		`UPDATE bug_reports SET user_id = NULL WHERE user_id = $1`,
		`UPDATE error_reports SET user_id = NULL WHERE user_id = $1`,
		`DELETE FROM users WHERE id = $1`,
	}
	for _, s := range stmts {
		if err := e.Exec(ctx, s, userID); err != nil {
			return err
		}
	}
	return nil
}

// txExecer adapts a pgx.Tx to the execer interface (pgx.Tx.Exec returns
// a CommandTag we discard).
type txExecer struct{ tx pgx.Tx }

func (t txExecer) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.tx.Exec(ctx, sql, args...)
	return err
}

// DeleteAccount runs the cascade inside one transaction.
func (r *PgRepo) DeleteAccount(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := deleteAccountStmts(ctx, txExecer{tx}, id); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Compile-time assertion that *pgxpool.Pool is a valid DeleteExecer.
var _ DeleteExecer = (*pgxpool.Pool)(nil)
```

> Remove the temporary `DeleteAccount` stub from Task 5's repo_pg.go if you added one — this file now provides it. Ensure `PgRepo.pool` is typed `DeleteExecer`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/user/ -v`
Expected: PASS (service + delete tests).

- [ ] **Step 5: Commit**

```bash
git add internal/user/repo_delete.go internal/user/repo_delete_test.go internal/user/repo_pg.go
git commit -m "feat(go): user account-deletion cascade (8-table txn, order-tested)"
```

---

### Task 7: subscription read-only reader

**Files:**
- Create: `internal/subscription/reader.go`
- Test: `internal/subscription/reader_test.go`

- [ ] **Step 1: Write the failing test**

```go
package subscription_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sub "github.com/tannpv/draftright-rewrite/internal/subscription"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	row sqlc.GetActiveSubscriptionByUserIDRow
	err error
}

func (f fakeQ) GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error) {
	return f.row, f.err
}

func TestActiveByUser_Found(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r := sub.NewReader(fakeQ{row: sqlc.GetActiveSubscriptionByUserIDRow{
		Status: "active", StoreType: "lemonsqueezy",
		StartedAt: pgtype.Timestamp{Time: started, Valid: true},
		PlanName:  "Pro", DailyLimit: 100,
	}})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.PlanName != "Pro" || got.DailyLimit != 100 || got.Status != "active" {
		t.Fatalf("bad sub: %+v", got)
	}
}

func TestActiveByUser_None(t *testing.T) {
	r := sub.NewReader(fakeQ{err: pgx.ErrNoRows})
	got, err := r.ActiveByUser(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil sub, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/subscription/ -v`
Expected: FAIL — package missing.

- [ ] **Step 3: Implement**

```go
// Package subscription is the read-only entitlement view needed by
// /auth/account in Phase 1a. Subscription WRITES + expiry cron land in
// Phase 2; this reader is the seam they'll extend.
package subscription

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// AccountSub is the subscription shape /auth/account serializes.
// ExpiresAt is nil when the row's expires_at is NULL.
type AccountSub struct {
	PlanName   string
	Status     string
	StoreType  string
	StartedAt  time.Time
	ExpiresAt  *time.Time
	DailyLimit int
}

// Querier is the one-method sqlc subset.
type Querier interface {
	GetActiveSubscriptionByUserID(ctx context.Context, id pgtype.UUID) (sqlc.GetActiveSubscriptionByUserIDRow, error)
}

// Reader resolves the active subscription.
type Reader struct{ q Querier }

// NewReader wires the querier.
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// ActiveByUser returns the newest active subscription, or (nil, nil)
// when the user has none.
func (r *Reader) ActiveByUser(ctx context.Context, userID string) (*AccountSub, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil // malformed id → no sub (account still renders)
	}
	row, err := r.q.GetActiveSubscriptionByUserID(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &AccountSub{
		PlanName:   row.PlanName,
		Status:     string(row.Status),
		StoreType:  string(row.StoreType),
		StartedAt:  row.StartedAt.Time,
		DailyLimit: int(row.DailyLimit),
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		s.ExpiresAt = &t
	}
	return s, nil
}
```

> Match generated types: `row.Status`/`row.StoreType` may be enum string types (cast with `string(...)`); `row.StartedAt`/`row.ExpiresAt` are `pgtype.Timestamp` (timestamp without time zone). Adjust casts to what `queries_auth.sql.go` actually emits.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/subscription/ -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/subscription/
git commit -m "feat(go): subscription read-only reader (active sub + plan join)"
```

---

### Task 8: usage counter (today)

**Files:**
- Create: `internal/usage/counter.go`
- Test: `internal/usage/counter_test.go`

- [ ] **Step 1: Write the failing test**

```go
package usage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/usage"
)

type fakeQ struct {
	gotSince time.Time
	count    int64
}

func (f *fakeQ) CountUsageToday(ctx context.Context, arg sqlc.CountUsageTodayParams) (int64, error) {
	f.gotSince = arg.CreatedAt.Time
	return f.count, nil
}

func TestCountToday_PassesLocalMidnight(t *testing.T) {
	f := &fakeQ{count: 7}
	c := usage.NewCounter(f)
	n, err := c.CountToday(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
	now := time.Now()
	wantMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !f.gotSince.Equal(wantMidnight) {
		t.Fatalf("since = %v, want local midnight %v", f.gotSince, wantMidnight)
	}
}

var _ = pgtype.Timestamp{}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/usage/ -v`
Expected: FAIL — package missing.

- [ ] **Step 3: Implement**

```go
// Package usage is the read-only daily-usage view for /auth/account.
// The full usage module (logging, aggregation) is folded in at Phase 1d;
// this counter is the seam.
package usage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the one-method sqlc subset.
type Querier interface {
	CountUsageToday(ctx context.Context, arg sqlc.CountUsageTodayParams) (int64, error)
}

// Counter counts a user's rewrites since local midnight.
type Counter struct{ q Querier }

// NewCounter wires the querier.
func NewCounter(q Querier) *Counter { return &Counter{q: q} }

// CountToday mirrors Node usageService.countTodayByUser: rows with
// created_at >= local midnight (server timezone, matching new Date();
// setHours(0,0,0,0)).
func (c *Counter) CountToday(ctx context.Context, userID string) (int, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return 0, nil
	}
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n, err := c.q.CountUsageToday(ctx, sqlc.CountUsageTodayParams{
		UserID:    uid,
		CreatedAt: pgtype.Timestamp{Time: midnight, Valid: true},
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
```

> The `CountUsageTodayParams` field names (`UserID`, `CreatedAt`) come from the generated code — confirm against `queries_auth.sql.go`. If the `created_at` column is `timestamp without time zone`, the param is `pgtype.Timestamp`; if `with time zone`, it's `pgtype.Timestamptz` (use `.Time` accordingly).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/usage/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/usage/
git commit -m "feat(go): usage today-counter (local-midnight boundary)"
```

---

### Task 9: auth usecase — login (+ token mint, provider labels, errors)

**Files:**
- Create: `internal/auth/errors.go`, `internal/auth/usecase.go`
- Test: `internal/auth/usecase_test.go`

- [ ] **Step 1: Write error constants + provider labels**

`internal/auth/errors.go`:

```go
package auth

// Exact UnauthorizedException messages from the NestJS auth.service —
// shadow-compare diffs the response `error` string, so these must match
// byte-for-byte.
const (
	msgInvalidCredentials = "Invalid credentials"
	msgAccountDisabled    = "Account disabled"
	msgSocialGeneric      = "This account uses a social sign-in. Use the provider you signed up with."
	msgCurrentPwWrong     = "Current password is incorrect"
	msgInvalidRefresh     = "Invalid refresh token"
)

// providerLabel maps the auth_provider enum to the human label Node uses
// in the social-account login message. Returns "" for local/unknown
// (caller falls back to msgSocialGeneric).
func providerLabel(provider string) string {
	switch provider {
	case "google":
		return "Google"
	case "facebook":
		return "Facebook"
	case "apple":
		return "Apple"
	default:
		return ""
	}
}

// socialLoginMsg builds the friendly "use the X button" message, or the
// generic fallback when there's no label.
func socialLoginMsg(provider string) string {
	if l := providerLabel(provider); l != "" {
		return "This account was created with " + l + ". Use the " + l + " button to sign in."
	}
	return msgSocialGeneric
}
```

- [ ] **Step 2: Write the failing login test**

`internal/auth/usecase_test.go`:

```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

type stubUsers struct {
	byEmail map[string]user.User
	byID    map[string]user.User
}

func (s stubUsers) ByEmail(_ context.Context, e string) (user.User, error) {
	u, ok := s.byEmail[e]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (s stubUsers) ByID(_ context.Context, id string) (user.User, error) {
	u, ok := s.byID[id]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}
func (s stubUsers) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (s stubUsers) DeleteAccount(context.Context, string) error              { return nil }

type stubTTL struct{}

func (stubTTL) TokenTTLs(context.Context) (time.Duration, time.Duration, error) {
	return 15 * time.Minute, 90 * 24 * time.Hour, nil
}

func newSvc(t *testing.T, users UserService) *Service {
	t.Helper()
	return NewService(users, stubTTL{}, "access-secret", "refresh-secret")
}

func TestLogin_HappyPath(t *testing.T) {
	hash, _ := shared.HashPassword("pw123")
	u := user.User{ID: "u1", Email: "a@b.com", Name: "Al", Role: "user", IsActive: true, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})

	res, err := svc.Login(context.Background(), "a@b.com", "pw123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	if res.User.ID != "u1" || res.User.Email != "a@b.com" || res.User.Name != "Al" {
		t.Fatalf("bad user payload: %+v", res.User)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{}})
	_, err := svc.Login(context.Background(), "no@b.com", "x")
	assertAuthErr(t, err, msgInvalidCredentials)
}

func TestLogin_SocialOnlyAccount(t *testing.T) {
	u := user.User{ID: "u1", Email: "g@b.com", IsActive: true, AuthProvider: "google"} // no PasswordHash
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"g@b.com": u}})
	_, err := svc.Login(context.Background(), "g@b.com", "x")
	assertAuthErr(t, err, "This account was created with Google. Use the Google button to sign in.")
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := shared.HashPassword("right")
	u := user.User{ID: "u1", Email: "a@b.com", IsActive: true, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})
	_, err := svc.Login(context.Background(), "a@b.com", "wrong")
	assertAuthErr(t, err, msgInvalidCredentials)
}

func TestLogin_Disabled(t *testing.T) {
	hash, _ := shared.HashPassword("pw")
	u := user.User{ID: "u1", Email: "a@b.com", IsActive: false, PasswordHash: hash}
	svc := newSvc(t, stubUsers{byEmail: map[string]user.User{"a@b.com": u}})
	_, err := svc.Login(context.Background(), "a@b.com", "pw")
	assertAuthErr(t, err, msgAccountDisabled)
}

func assertAuthErr(t *testing.T, err error, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("want *AuthError, got %T (%v)", err, err)
	}
	if ae.Message != wantMsg {
		t.Fatalf("msg = %q, want %q", ae.Message, wantMsg)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/auth/ -run Login -v`
Expected: FAIL — undefined `NewService`, `Service`, `AuthError`, etc.

- [ ] **Step 4: Implement the usecase**

`internal/auth/usecase.go`:

```go
// Package auth ports the NestJS auth module's password flows. Phase 1a:
// login, refresh, change-password, account, delete. register/social/
// email land in 1b/1c. Depends on user/subscription/usage/settings via
// interfaces wired in the composition root (no cross-module internals).
package auth

import (
	"context"
	"errors"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

// AuthError is a 401-class failure carrying the exact Node message. The
// handler maps it to {error: Message, code: "invalid-token"} → 401.
type AuthError struct{ Message string }

func (e *AuthError) Error() string { return e.Message }

func unauthorized(msg string) *AuthError { return &AuthError{Message: msg} }

// UserService is the user-module subset auth needs.
type UserService interface {
	ByEmail(ctx context.Context, email string) (user.User, error)
	ByID(ctx context.Context, id string) (user.User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
}

// TTLReader resolves token lifetimes (platform/settings.Reader).
type TTLReader interface {
	TokenTTLs(ctx context.Context) (access, refresh time.Duration, err error)
}

// Service orchestrates auth flows. Holds two signers (access/refresh
// secrets) + a refresh verifier.
type Service struct {
	users      UserService
	ttl        TTLReader
	access     *platauth.Signer
	refresh    *platauth.Signer
	refreshVer *platauth.Verifier
}

// NewService wires dependencies. accessSecret signs/verifies access
// tokens; refreshSecret signs/verifies refresh tokens.
func NewService(users UserService, ttl TTLReader, accessSecret, refreshSecret string) *Service {
	return &Service{
		users:      users,
		ttl:        ttl,
		access:     platauth.NewSigner(accessSecret),
		refresh:    platauth.NewSigner(refreshSecret),
		refreshVer: platauth.NewVerifier(refreshSecret),
	}
}

// Tokens is the {access_token, refresh_token} pair.
type Tokens struct {
	AccessToken  string
	RefreshToken string
}

// UserPayload is the trimmed user echoed by login.
type UserPayload struct {
	ID    string
	Email string
	Name  string
}

// LoginResult is login's full return.
type LoginResult struct {
	Tokens
	User UserPayload
}

// generateTokens mirrors Node generateTokens: same claims, TTLs from
// app_settings.
func (s *Service) generateTokens(ctx context.Context, u user.User) (Tokens, error) {
	accTTL, refTTL, err := s.ttl.TokenTTLs(ctx)
	if err != nil {
		return Tokens{}, err
	}
	claims := platauth.Claims{Sub: u.ID, Email: u.Email, Role: u.Role}
	at, err := s.access.Sign(claims, accTTL)
	if err != nil {
		return Tokens{}, err
	}
	rt, err := s.refresh.Sign(claims, refTTL)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: at, RefreshToken: rt}, nil
}

// Login ports auth.service.login exactly (branch order matters).
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := s.users.ByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return LoginResult{}, unauthorized(msgInvalidCredentials)
	}
	if err != nil {
		return LoginResult{}, err
	}
	if u.PasswordHash == "" {
		return LoginResult{}, unauthorized(socialLoginMsg(u.AuthProvider))
	}
	ok, err := shared.VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok {
		return LoginResult{}, unauthorized(msgInvalidCredentials)
	}
	if !u.IsActive {
		return LoginResult{}, unauthorized(msgAccountDisabled)
	}
	toks, err := s.generateTokens(ctx, u)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Tokens: toks, User: UserPayload{ID: u.ID, Email: u.Email, Name: u.Name}}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run Login -v`
Expected: PASS (5 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/errors.go internal/auth/usecase.go internal/auth/usecase_test.go
git commit -m "feat(go): auth login usecase — token mint, provider labels, exact 401s"
```

---

### Task 10: auth usecase — refresh, change-password, account, delete

**Files:**
- Modify: `internal/auth/usecase.go`
- Test: `internal/auth/usecase_more_test.go` (create)

- [ ] **Step 1: Write the failing tests**

`internal/auth/usecase_more_test.go`:

```go
package auth

import (
	"context"
	"testing"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

func TestRefresh_ValidToken(t *testing.T) {
	u := user.User{ID: "u1", Email: "a@b.com", Role: "user", IsActive: true}
	svc := newSvc(t, stubUsers{byID: map[string]user.User{"u1": u}})
	// mint a refresh token with the refresh secret
	rt, _ := platauth.NewSigner("refresh-secret").Sign(platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"}, 1<<20)
	toks, err := svc.Refresh(context.Background(), rt)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if toks.AccessToken == "" || toks.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
}

func TestRefresh_GarbageToken(t *testing.T) {
	svc := newSvc(t, stubUsers{})
	_, err := svc.Refresh(context.Background(), "not-a-jwt")
	assertAuthErr(t, err, msgInvalidRefresh)
}

func TestRefresh_AccessTokenRejected(t *testing.T) {
	// a token signed with the ACCESS secret must not pass refresh verify
	svc := newSvc(t, stubUsers{byID: map[string]user.User{"u1": {ID: "u1", IsActive: true}}})
	at, _ := platauth.NewSigner("access-secret").Sign(platauth.Claims{Sub: "u1"}, 1<<20)
	_, err := svc.Refresh(context.Background(), at)
	assertAuthErr(t, err, msgInvalidRefresh)
}

func TestChangePassword_Success(t *testing.T) {
	hash, _ := shared.HashPassword("old")
	u := user.User{ID: "u1", PasswordHash: hash}
	users := stubUsers{byID: map[string]user.User{"u1": u}}
	svc := newSvc(t, users)
	if err := svc.ChangePassword(context.Background(), "u1", "old", "new"); err != nil {
		t.Fatalf("change: %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	hash, _ := shared.HashPassword("old")
	u := user.User{ID: "u1", PasswordHash: hash}
	svc := newSvc(t, stubUsers{byID: map[string]user.User{"u1": u}})
	err := svc.ChangePassword(context.Background(), "u1", "WRONG", "new")
	assertAuthErr(t, err, msgCurrentPwWrong)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run "Refresh|ChangePassword" -v`
Expected: FAIL — undefined `Refresh`, `ChangePassword`.

- [ ] **Step 3: Implement refresh + change-password**

Append to `internal/auth/usecase.go`:

```go
// Refresh ports auth.service.refresh: verify the refresh token with the
// refresh secret, reload the user, require is_active, re-mint. Any
// failure → "Invalid refresh token".
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	claims, err := s.refreshVer.Verify(refreshToken)
	if err != nil {
		return Tokens{}, unauthorized(msgInvalidRefresh)
	}
	u, err := s.users.ByID(ctx, claims.Sub)
	if err != nil || !u.IsActive {
		return Tokens{}, unauthorized(msgInvalidRefresh)
	}
	return s.generateTokens(ctx, u)
}

// ChangePassword ports auth.service.changePassword: verify current,
// hash new, persist.
func (s *Service) ChangePassword(ctx context.Context, userID, current, next string) error {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return unauthorized("") // Node throws bare UnauthorizedException → "Unauthorized"
	}
	ok, err := shared.VerifyPassword(current, u.PasswordHash)
	if err != nil || !ok {
		return unauthorized(msgCurrentPwWrong)
	}
	hash, err := shared.HashPassword(next)
	if err != nil {
		return err
	}
	return s.users.UpdatePasswordHash(ctx, userID, hash)
}
```

> NestJS bare `throw new UnauthorizedException()` (no message) serializes message `"Unauthorized"`. So `unauthorized("")` must render `"Unauthorized"` in the handler — handle the empty-message case in Task 11's handler (map "" → "Unauthorized"). Add a test asserting that if you want; the spec's exact-parity goal makes it worth a one-line guard.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run "Refresh|ChangePassword" -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Implement Account + DeleteAccount usecase methods**

Append to `internal/auth/usecase.go`. First extend the constructor deps — add a subscription reader + usage counter. Change `Service` struct + `NewService` to accept them:

```go
// (Add these interfaces near UserService.)

// SubReader is the subscription-module subset (read-only).
type SubReader interface {
	ActiveByUser(ctx context.Context, userID string) (*AccountSub, error)
}

// AccountSub mirrors subscription.AccountSub — re-declared here to keep
// auth free of a direct import of the subscription package's concrete
// type? NO — prefer importing the real type. Use:
//   sub "github.com/tannpv/draftright-rewrite/internal/subscription"
// and reference *sub.AccountSub. (Delete this placeholder type.)

// UsageCounter is the usage-module subset.
type UsageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}
```

> DECISION: import the real `subscription.AccountSub` type rather than redeclaring it. Update `SubReader` to `ActiveByUser(ctx, userID string) (*subscription.AccountSub, error)` with `subscription "github.com/tannpv/draftright-rewrite/internal/subscription"`. This is an allowed dependency: `auth → subscription` (a sibling module) is fine because it goes through `subscription`'s exported interface/type, wired in the composition root — the forbidden direction is importing another module's *internal* unexported guts, which this is not. (Master design rule: "module A → an interface that module B satisfies".)

Add to `Service` struct: `subs SubReader` and `usage UsageCounter`. Update `NewService` signature to:

```go
func NewService(users UserService, subs SubReader, usage UsageCounter, ttl TTLReader, accessSecret, refreshSecret string) *Service
```

(Update `newSvc` test helper + all call sites accordingly — pass stub `subs`/`usage`.)

Then add:

```go
// AccountView is the /auth/account response (nil → JSON null).
type AccountView struct {
	ID                     string                  `json:"id"`
	Email                  string                  `json:"email"`
	Name                   string                  `json:"name"`
	EmailVerified          bool                    `json:"email_verified"`
	HasLemonsqueezyCustomer bool                   `json:"has_lemonsqueezy_customer"`
	Subscription           *AccountSubView         `json:"subscription"`
}

// AccountSubView is the nested subscription object.
type AccountSubView struct {
	PlanName   string  `json:"plan_name"`
	Status     string  `json:"status"`
	StoreType  string  `json:"store_type"`
	StartedAt  string  `json:"started_at"`
	ExpiresAt  *string `json:"expires_at"`
	DailyLimit int     `json:"daily_limit"`
	UsageToday int     `json:"usage_today"`
}

// Account ports auth.controller.account: user + active sub + usage. nil
// (→ JSON null) when the user row is gone.
func (s *Service) Account(ctx context.Context, userID string) (*AccountView, error) {
	u, err := s.users.ByID(ctx, userID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	usageToday, err := s.usage.CountToday(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := &AccountView{
		ID: u.ID, Email: u.Email, Name: u.Name,
		EmailVerified:           u.EmailVerified,
		HasLemonsqueezyCustomer: u.LemonsqueezyCustomer != "",
	}
	if sub != nil {
		view.Subscription = &AccountSubView{
			PlanName:   sub.PlanName,
			Status:     sub.Status,
			StoreType:  sub.StoreType,
			StartedAt:  sub.StartedAt.Format(time.RFC3339),
			DailyLimit: sub.DailyLimit,
			UsageToday: usageToday,
		}
		if sub.ExpiresAt != nil {
			e := sub.ExpiresAt.Format(time.RFC3339)
			view.Subscription.ExpiresAt = &e
		}
	}
	return view, nil
}

// DeleteAccount ports the cascade delete.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	return s.users.DeleteAccount(ctx, userID)
}
```

> `AccountSub` here refers to `subscription.AccountSub` (imported). Remove the placeholder `AccountSub` redeclaration. **Timestamp format is a known parity risk** (spec §6/§14): `time.RFC3339` is the starting point, but the shadow gate (Task 12) compares against a real Node `/auth/account` response — if Node emits millisecond precision (`…123Z`) or a different offset, change the format string to match (e.g. `"2006-01-02T15:04:05.000Z07:00"`). Pin it with a captured-response test once the live response is known.

- [ ] **Step 6: Run the full auth package tests**

Run: `go test ./internal/auth/ -v`
Expected: PASS (login + refresh + change-password; account/delete covered by handler tests in Task 11). Fix any constructor-arity breakage in the test helper.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/usecase.go internal/auth/usecase_more_test.go
git commit -m "feat(go): auth refresh/change-password/account/delete usecase"
```

---

### Task 11: HTTP handlers + router/main wiring

**Files:**
- Create: `internal/auth/handler.go`, `internal/auth/handler_test.go`
- Modify: `internal/shared/router.go`, `cmd/server/main.go`

- [ ] **Step 1: Write the failing handler test**

`internal/auth/handler_test.go`:

```go
package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/auth"
	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

// minimal stubs (mirror usecase_test stubs but in the _test package)
type users struct{ u map[string]user.User; byID map[string]user.User }
func (s users) ByEmail(_ context.Context, e string) (user.User, error) {
	if v, ok := s.u[e]; ok { return v, nil }
	return user.User{}, user.ErrNotFound
}
func (s users) ByID(_ context.Context, id string) (user.User, error) {
	if v, ok := s.byID[id]; ok { return v, nil }
	return user.User{}, user.ErrNotFound
}
func (s users) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (s users) DeleteAccount(context.Context, string) error              { return nil }

type subs struct{ s *subscription.AccountSub }
func (s subs) ActiveByUser(context.Context, string) (*subscription.AccountSub, error) { return s.s, nil }

type usg struct{ n int }
func (u usg) CountToday(context.Context, string) (int, error) { return u.n, nil }

type ttl struct{}
func (ttl) TokenTTLs(context.Context) (a, r interface{ }, e error) { return } // replace with real durations

func TestLoginHandler_201AndBody(t *testing.T) {
	hash, _ := shared.HashPassword("pw")
	svc := auth.NewService(
		users{u: map[string]user.User{"a@b.com": {ID: "u1", Email: "a@b.com", Name: "Al", IsActive: true, PasswordHash: hash}}},
		subs{}, usg{}, realTTL{}, "access", "refresh",
	)
	h := auth.NewHandler(svc)
	body, _ := json.Marshal(map[string]string{"email": "a@b.com", "password": "pw"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct{ ID, Email, Name string } `json:"user"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.AccessToken == "" || out.User.ID != "u1" {
		t.Fatalf("bad body: %s", rec.Body.String())
	}
}

func TestLoginHandler_401Envelope(t *testing.T) {
	svc := auth.NewService(users{u: map[string]user.User{}}, subs{}, usg{}, realTTL{}, "access", "refresh")
	h := auth.NewHandler(svc)
	body, _ := json.Marshal(map[string]string{"email": "no@b.com", "password": "x"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var out struct{ Error, Code string }
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Error != "Invalid credentials" || out.Code != "invalid-token" {
		t.Fatalf("envelope = %s", rec.Body.String())
	}
}

func TestAccountHandler_Shape(t *testing.T) {
	svc := auth.NewService(
		users{byID: map[string]user.User{"u1": {ID: "u1", Email: "a@b.com", Name: "Al", EmailVerified: true}}},
		subs{}, usg{n: 3}, realTTL{}, "access", "refresh",
	)
	h := auth.NewHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/auth/account", nil)
	ctx := shared.ContextWithClaims(req.Context(), &platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.Account(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var out struct {
		ID            string `json:"id"`
		EmailVerified bool   `json:"email_verified"`
		Subscription  *struct{} `json:"subscription"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.ID != "u1" || !out.EmailVerified || out.Subscription != nil {
		t.Fatalf("bad account body: %s", rec.Body.String())
	}
}

type realTTL struct{}
func (realTTL) TokenTTLs(context.Context) (a, r interface{}, e error) { return }
```

> The `ttl`/`realTTL` stub above is a sketch — the real `TTLReader` returns `(time.Duration, time.Duration, error)`. Replace the placeholder with:
> ```go
> type realTTL struct{}
> func (realTTL) TokenTTLs(context.Context) (time.Duration, time.Duration, error) {
> 	return 15*time.Minute, 90*24*time.Hour, nil
> }
> ```
> and `import "time"`. Drop the bogus `ttl` type. (Plan shows intent; implementer writes the compiling version.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run Handler -v`
Expected: FAIL — undefined `NewHandler`.

- [ ] **Step 3: Implement the handlers**

`internal/auth/handler.go`:

```go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler adapts the auth Service to chi http.HandlerFuncs. Each method
// matches one route. JSON shapes + status codes mirror NestJS exactly
// (login + change-password = 201; refresh + delete = 200).
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// writeAuthErr renders an *AuthError as the canonical envelope. Bare
// (empty) message → "Unauthorized" to match NestJS's default.
func writeAuthErr(w http.ResponseWriter, r *http.Request, err error) bool {
	var ae *AuthError
	if errors.As(err, &ae) {
		msg := ae.Message
		if msg == "" {
			msg = "Unauthorized"
		}
		shared.WriteError(w, r, "invalid-token", msg)
		return true
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return false
	}
	return true
}

// Login: POST /auth/login → 201.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token":  res.AccessToken,
		"refresh_token": res.RefreshToken,
		"user": map[string]any{
			"id": res.User.ID, "email": res.User.Email, "name": res.User.Name,
		},
	})
}

// Refresh: POST /auth/refresh → 200.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	toks, err := h.svc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "refresh failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": toks.AccessToken, "refresh_token": toks.RefreshToken,
	})
}

// ChangePassword: POST /auth/change-password (JWT) → 201.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ChangePassword(r.Context(), claims.Sub, body.CurrentPassword, body.NewPassword); err != nil {
		if writeAuthErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "change-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{"success": true})
}

// Account: GET /auth/account (JWT) → 200 (object or null).
func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.Account(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "account failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, view) // nil view → JSON null
}

// DeleteAccount: DELETE /auth/account (JWT) → 200.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), claims.Sub); err != nil {
		shared.WriteError(w, r, "internal", "delete failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
```

> Confirm `WriteJSON(w, 200, view)` with a nil `*AccountView` marshals to `null` (it does: `json.Marshal((*T)(nil))` == `null`). If `WriteJSON` special-cases nil, adjust.

- [ ] **Step 4: Run handler tests**

Run: `go test ./internal/auth/ -run Handler -v`
Expected: PASS.

- [ ] **Step 5: Mount routes in the router**

In `internal/shared/router.go`, add fields to the `Router` struct:

```go
	Login          http.Handler
	Refresh        http.Handler
	ChangePassword http.Handler
	Account        http.Handler
	DeleteAccount  http.Handler
```

In `Build()`: mount `login` + `refresh` as PUBLIC routes (no auth), and `change-password` + `account` + `delete` INSIDE the existing `RequireAuth` group next to `/auth/me`. Follow the exact pattern already used for `Me`. Example inside the public section:

```go
	if r.Login != nil {
		mux.Method(http.MethodPost, "/auth/login", r.Login)
	}
	if r.Refresh != nil {
		mux.Method(http.MethodPost, "/auth/refresh", r.Refresh)
	}
```

And inside the authenticated `mux.Group(func(api chi.Router){ api.Use(RequireAuth(...)) ... })` block, alongside `r.Me`:

```go
		if r.ChangePassword != nil {
			api.Method(http.MethodPost, "/auth/change-password", r.ChangePassword)
		}
		if r.Account != nil {
			api.Method(http.MethodGet, "/auth/account", r.Account)
		}
		if r.DeleteAccount != nil {
			api.Method(http.MethodDelete, "/auth/account", r.DeleteAccount)
		}
```

> Read the actual current `router.go` to match the existing group/middleware structure exactly — do not duplicate the auth group.

- [ ] **Step 6: Wire in the composition root**

In `cmd/server/main.go`, inside the `if pool != nil` branch of `composeDeps` (where `q := sqlc.New(pool)` already exists), build the auth stack and add the five handlers to the returned `coreHandlers`. Add fields to `coreHandlers`:

```go
type coreHandlers struct {
	health         http.Handler
	me             http.Handler
	login          http.Handler
	refresh        http.Handler
	changePassword http.Handler
	account        http.Handler
	deleteAccount  http.Handler
}
```

In the `pool != nil` branch:

```go
	if cfg.JWTRefreshSecret == "" {
		cleanup()
		return usecase.RewriteDeps{}, coreHandlers{}, nil, errors.New("JWT_REFRESH_SECRET required when auth endpoints are enabled")
	}
	userRepo := userpkg.NewPgRepo(q, pool)
	userSvc := userpkg.NewService(userRepo)
	subReader := subpkg.NewReader(q)
	usageCounter := usagepkg.NewCounter(q)
	ttlReader := settingspkg.NewReader(q)
	authSvc := authpkg.NewService(userSvc, subReader, usageCounter, ttlReader, cfg.JWTSecret, cfg.JWTRefreshSecret)
	authHandler := authpkg.NewHandler(authSvc)
	core.login = http.HandlerFunc(authHandler.Login)
	core.refresh = http.HandlerFunc(authHandler.Refresh)
	core.changePassword = http.HandlerFunc(authHandler.ChangePassword)
	core.account = http.HandlerFunc(authHandler.Account)
	core.deleteAccount = http.HandlerFunc(authHandler.DeleteAccount)
```

Add imports with aliases: `userpkg ".../internal/user"`, `authpkg ".../internal/auth"`, `subpkg ".../internal/subscription"`, `usagepkg ".../internal/usage"`, `settingspkg ".../internal/platform/settings"`. Then in the `Router` literal (the `&shared.Router{...}` in `main`), pass:

```go
		Login:          core.login,
		Refresh:        core.refresh,
		ChangePassword: core.changePassword,
		Account:        core.account,
		DeleteAccount:  core.deleteAccount,
```

> When `pool == nil` (dev no-DB), these stay nil → router nil-guards skip them → routes 404 (acceptable; auth needs the DB). Add a `log.Warn("auth endpoints disabled", "reason", "no DATABASE_URL")` in the else branch.

- [ ] **Step 7: Build + full test**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && go build ./... && go test ./... 2>&1 | grep -v "no test files"`
Expected: builds; all packages `ok`.

- [ ] **Step 8: Commit**

```bash
git add internal/auth/handler.go internal/auth/handler_test.go internal/shared/router.go cmd/server/main.go
git commit -m "feat(go): mount /auth login/refresh/change-password/account/delete + wire composition root"
```

---

### Task 12: shadow-compare fixtures + token structural verification

**Files:**
- Create: `cmd/shadowdiff/fixtures/login.json`, `refresh.json`, `change_password.json`, `account.json`, `delete.json`
- Create: `cmd/shadowdiff/token_verify_test.go`

- [ ] **Step 1: Add fixtures**

Follow the existing fixture schema in `cmd/shadowdiff/fixtures/health.json` / `auth_me.json` (read one first for exact field names). Each new fixture declares method, path, headers, body, and `ignore_value_of`. Example `login.json`:

```json
{
  "name": "login-valid",
  "method": "POST",
  "path": "/auth/login",
  "headers": { "Content-Type": "application/json" },
  "body": { "email": "REPLACE_WITH_DEV_SEED_EMAIL", "password": "REPLACE_WITH_DEV_SEED_PASSWORD" },
  "ignore_value_of": ["request_id", "access_token", "refresh_token"]
}
```

`account.json` (authenticated):

```json
{
  "name": "account",
  "method": "GET",
  "path": "/auth/account",
  "headers": { "Authorization": "Bearer REPLACE_WITH_DEV_NODE_TOKEN" },
  "ignore_value_of": ["request_id", "started_at", "expires_at"]
}
```

> `started_at`/`expires_at` are ignored ONLY until the timestamp format is pinned (Task 10 note). Once a captured Node response confirms the format and the Go output matches byte-for-byte, REMOVE them from `ignore_value_of` so the gate actually verifies timestamps. Document this in the fixture with a comment field `"_note"`.
> Do NOT commit real credentials or a real bearer token — leave the `REPLACE_WITH_*` placeholders (operator fills them at run time, same as Phase 0's `auth_me.json`).

- [ ] **Step 2: Write the token structural-verification test**

`cmd/shadowdiff/token_verify_test.go` — proves a Go-issued access token decodes to the expected claims (the shadow gate ignores raw token strings, so this asserts structure instead):

```go
package main

import (
	"testing"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// A login response's access_token is ignored by value in the JSON diff
// (it differs every issue). This test instead asserts the Go signer
// produces a token whose claims + TTL match what Node issues, so the
// "ignore_value_of" exclusion is safe.
func TestAccessToken_ClaimsAndTTL(t *testing.T) {
	secret := "shared-secret"
	tok, err := platauth.NewSigner(secret).Sign(
		platauth.Claims{Sub: "u1", Email: "a@b.com", Role: "user"}, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := platauth.NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Sub != "u1" || claims.Email != "a@b.com" || claims.Role != "user" {
		t.Fatalf("claims = %+v", claims)
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 14*time.Minute || ttl > 15*time.Minute {
		t.Fatalf("ttl = %v, want ~15m", ttl)
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./cmd/shadowdiff/ -run AccessToken -v`
Expected: PASS.

- [ ] **Step 4: Build the shadowdiff tool**

Run: `go build ./cmd/shadowdiff/`
Expected: builds (no live run — operator-pending, needs dev Node + seeded user).

- [ ] **Step 5: Commit**

```bash
git add cmd/shadowdiff/fixtures/ cmd/shadowdiff/token_verify_test.go
git commit -m "feat(go): shadowdiff auth fixtures + access-token structural verify test"
```

---

### Task 13: Final verification

**Files:** none (verification only).

- [ ] **Step 1: gofmt**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && gofmt -l .`
Expected: empty output. If not: `gofmt -w .` and re-commit.

- [ ] **Step 2: vet + build + race tests**

Run: `go vet ./... && go build ./... && go test -race ./... 2>&1 | grep -v "no test files"`
Expected: vet clean; build ok; all packages `ok`.

- [ ] **Step 3: No cross-module internal imports**

Run:
```bash
grep -rn "internal/rewrite" internal/auth internal/user internal/subscription internal/usage || echo "clean: auth/user/sub/usage don't touch rewrite"
grep -rn "internal/core" internal/auth internal/user internal/subscription internal/usage || echo "clean: no core import"
```
Expected: both print the "clean" line. (`auth → subscription/user/usage` sibling imports via exported types are allowed and expected — only `rewrite`/`core` internal coupling is forbidden.)

- [ ] **Step 4: sqlc drift check**

Run: `sqlc generate && git diff --exit-code internal/shared/pg/sqlc/`
Expected: no diff (generated code matches committed).

- [ ] **Step 5: Commit any formatting fixes, then report**

```bash
git status
# if anything changed:
git add -A && git commit -m "chore(go): gofmt + sqlc drift fixes for Phase 1a"
```

Report: all 1a endpoints implemented, tests green, parity-faithful; live shadow gate operator-pending (needs dev Node + seeded user + JWT_SECRET/JWT_REFRESH_SECRET); no production/Caddy change.

---

## Self-Review

**Spec coverage:**
- §2 endpoints → Tasks 9–11 (login/refresh/change-password/account/delete) ✅; 201/200 codes pinned in handler test ✅
- §3 modules (user/auth/subscription/usage/settings) → Tasks 4–11 ✅
- §4 sqlc queries → Task 3 ✅
- §5 dual-secret signing + AppSettings TTL → Tasks 2,4,9 (two Signer instances + refresh Verifier) ✅
- §6 /auth/account shape → Task 10 `AccountView`/`AccountSubView` ✅ (timestamp format flagged as risk, pinned via shadow gate)
- §7 delete cascade → Task 6 (order-tested) ✅
- §8 bcrypt cross-compat → Task 1 ✅
- §9 login branch parity → Task 9 (5 branch tests) ✅
- §10 error envelope → handler `writeAuthErr` (code "invalid-token") ✅
- §11 composition-root + dev fallback → Task 11 Step 6 ✅
- §12 shadow fixtures + structural token verify → Task 12 ✅
- §13 testing → every task TDD; Task 13 race/vet/drift ✅

**Placeholder scan:** The known soft spots — bcrypt vector (Task 1 regenerate note), generated nullable types (Tasks 5/7/8 "match generated type" notes), timestamp format (Task 10 note), `realTTL` test stub (Task 11 note) — are explicitly called out with the exact action the implementer takes, not left as silent TODOs. No bare "implement later".

**Type consistency:** `User` fields, `NewService(users, subs, usage, ttl, accessSecret, refreshSecret)` arity (final form in Task 10 Step 5; Task 9 uses the 4-arg interim — implementer updates the helper when Task 10 widens it), `AccountSub`/`AccountView`/`AccountSubView`, `TTLReader.TokenTTLs`, `Repo`/`UserService` method sets, `coreHandlers` fields all consistent across tasks. NOTE for implementer: Task 9 introduces `NewService` with 4 args, Task 10 Step 5 WIDENS it to 6 (adds subs+usage) — update Task 9's `newSvc` helper + tests at that point (called out inline).
