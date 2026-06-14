# Go Backend Port — Phase 1b Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the 6 remaining `/auth` endpoints (register, verify-email, resend-verification, forgot-password, reset-password, social) from NestJS to Go, byte-identical.

**Architecture:** Extend the existing `internal/auth` clean-arch module. New modules: `internal/email` (Resend adapter), `internal/plans` (free-plan reader). Extend `internal/user` (write methods) + `internal/subscription` (free-sub writer). Social verifiers behind a fakeable `SocialVerifier` port. All wired in `cmd/server/main.go`.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + sqlc, golang-jwt/jwt/v5 (RS256 for Apple), x/crypto/bcrypt, crypto/rand.

**Spec:** `docs/superpowers/specs/2026-06-13-go-backend-phase1b-design.md` — read it; this plan implements it.

**Conventions (from `backend-rewrite-go/CLAUDE.md`):** accept interfaces / return structs; consumer-side small ports; flat package until a 2nd impl; `gofmt`; `go test ./... -race` is the source of truth (build cache masks syntax errors). After any `.sql` change run `sqlc generate` then `go test ./...`.

**Parity invariants (every task honors):** bcrypt cost 10; email `strings.ToLower(strings.TrimSpace(x))` (NOT on login); JWT clockTolerance 0; exact error strings + status codes; CSPRNG 6-digit plaintext codes; 15m TTL; reset cap 5.

---

# Part A — Email / Password Lifecycle

## Task A0: Patch schema snapshot + regen sqlc

**Files:**
- Modify: `internal/platform/db/schema.sql`

These objects already exist in prod (added by `2026-06-01-apple-auth-provider.sql` + the password-reset migration). The Go read-only mirror is stale; sqlc compiles against it, so patch it. **No prod migration.**

- [ ] **Step 1: Add `'apple'` to the auth-provider enum**

In `schema.sql`, find `CREATE TYPE public.users_auth_provider_enum AS ENUM (` and add `'apple'`:
```sql
CREATE TYPE public.users_auth_provider_enum AS ENUM (
    'local',
    'google',
    'facebook',
    'tiktok',
    'apple'
);
```

- [ ] **Step 2: Add columns to `public.users`**

In the `CREATE TABLE public.users (...)` block, after `stripe_customer_id`:
```sql
    apple_id character varying(255),
    password_reset_code character varying(6),
    password_reset_expires timestamp with time zone,
    password_reset_attempts integer DEFAULT 0 NOT NULL
```
(add a comma after the previous last column).

- [ ] **Step 3: Add the `email_suppressions` table**

Near the other email tables:
```sql
CREATE TABLE public.email_suppressions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    reason character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
```

- [ ] **Step 4: Regenerate sqlc + verify build still green**

Run: `sqlc generate && go test ./... -race`
Expected: sqlc succeeds (no query references these yet, but the schema must parse); all existing tests PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/platform/db/schema.sql internal/shared/pg/sqlc
git commit -m "chore(go-port): patch schema mirror — apple enum, apple_id, password_reset_*, email_suppressions"
```

---

## Task A1: Config — Resend + Apple env vars

**Files:**
- Modify: `internal/platform/config/config.go`
- Test: `internal/platform/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `config_test.go`:
```go
func TestLoad_ReadsEmailAndAppleEnv(t *testing.T) {
	os.Setenv("JWT_SECRET", "s")
	os.Setenv("RESEND_API_KEY", "re_123")
	os.Setenv("EMAIL_FROM", "DraftRight <x@y.z>")
	os.Setenv("APPLE_AUDIENCES", "com.a,com.b")
	defer func() {
		os.Unsetenv("JWT_SECRET"); os.Unsetenv("RESEND_API_KEY")
		os.Unsetenv("EMAIL_FROM"); os.Unsetenv("APPLE_AUDIENCES")
	}()
	c, err := Load()
	if err != nil { t.Fatal(err) }
	if c.ResendAPIKey != "re_123" || c.EmailFrom != "DraftRight <x@y.z>" || c.AppleAudiences != "com.a,com.b" {
		t.Fatalf("got %+v", c)
	}
}
```

- [ ] **Step 2: Run test, verify it fails** — Run: `go test ./internal/platform/config/ -run EmailAndApple -v` → FAIL (unknown fields).

- [ ] **Step 3: Add the fields + loads**

In the `Config` struct (near OllamaURL):
```go
	// Resend transactional-email creds. Empty key = email disabled
	// (sends are logged + skipped, never error the request). Mirrors
	// the NestJS RESEND_API_KEY / EMAIL_FROM; app_settings overrides
	// both at request time (admin portal).
	ResendAPIKey string
	EmailFrom    string

	// APPLE_AUDIENCES — comma-separated allowed `aud` values for Apple
	// id_token verification. Empty → the two-app default applied in the
	// verifier. Mirrors the NestJS APPLE_AUDIENCES.
	AppleAudiences string
```
In `Load()`:
```go
		ResendAPIKey:   os.Getenv("RESEND_API_KEY"),
		EmailFrom:      os.Getenv("EMAIL_FROM"),
		AppleAudiences: os.Getenv("APPLE_AUDIENCES"),
```

- [ ] **Step 4: Run test, verify it passes** — Run: `go test ./internal/platform/config/ -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/platform/config/
git commit -m "feat(go-port): config — Resend + Apple audience env vars"
```

---

## Task A2: User module — `NewUser` / `UserPatch` domain + Create/Update SQL

**Files:**
- Modify: `internal/user/domain.go`, `internal/shared/pg/queries_auth.sql`

- [ ] **Step 1: Add domain types to `domain.go`**

Add `AvatarURL` to `User`:
```go
type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	Name                 string
	IsActive             bool
	Role                 string
	AuthProvider         string
	EmailVerified        bool
	LemonsqueezyCustomer string
	AvatarURL            string // "" when null
}
```
Add the write DTOs + extend the `Repo` port:
```go
// NewUser is the insert payload. Optional string fields are "" when
// absent; SocialProvider/SocialID set exactly one *_id column (""
// provider = local signup, no social column).
type NewUser struct {
	Email                    string
	PasswordHash             string // "" → NULL (social accounts)
	Name                     string
	AuthProvider             string // "local" default at the repo
	AvatarURL                string
	EmailVerified            bool
	EmailVerificationCode    string // "" → NULL
	EmailVerificationExpires *time.Time
	SocialProvider           string // "google"|"facebook"|"tiktok"|"apple"|""
	SocialID                 string
}

// UserPatch is a partial update — nil pointer = column untouched.
// Mirrors TypeORM's partial update object.
type UserPatch struct {
	PasswordHash             *string
	EmailVerified            *bool
	EmailVerificationCode    NullableString // set+null vs untouched
	EmailVerificationExpires NullableTime
	PasswordResetCode        NullableString
	PasswordResetExpires     NullableTime
	PasswordResetAttempts    *int
	AvatarURL                *string
	SocialProvider           string // for social link: which *_id col
	SocialID                 string
}

// NullableString distinguishes "leave column alone" (Set=false) from
// "set column to NULL" (Set=true, Value=nil) from "set value".
type NullableString struct {
	Set   bool
	Value *string
}

// NullableTime mirrors NullableString for timestamptz columns.
type NullableTime struct {
	Set   bool
	Value *time.Time
}
```
Add `import "time"` to `domain.go`. Extend the `Repo` interface in `service.go`:
```go
type Repo interface {
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id string) (User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
	Create(ctx context.Context, in NewUser) (User, error)
	Update(ctx context.Context, id string, p UserPatch) error
	FindBySocialId(ctx context.Context, provider, socialID string) (User, error)
	AuthState(ctx context.Context, email string) (AuthState, error)
}
```
> `FindBySocialId` + `AuthState` are implemented now (repo) but `FindBySocialId` is only USED in Part B. `AuthState` (below) carries the verification/reset columns the lifecycle endpoints read.

Add the read struct to `domain.go`:
```go
// AuthState is the verification/reset projection the lifecycle
// endpoints read (separate from the login projection so Phase 1a's
// path is untouched).
type AuthState struct {
	ID                       string
	Email                    string
	Name                     string
	PasswordHash             string
	AuthProvider             string
	IsActive                 bool
	EmailVerified            bool
	EmailVerificationCode    string
	EmailVerificationExpires *time.Time
	PasswordResetCode        string
	PasswordResetExpires     *time.Time
	PasswordResetAttempts    int
}
```

- [ ] **Step 2: Add SQL queries to `queries_auth.sql`**

```sql
-- name: CreateUser :one
INSERT INTO users (
  email, password_hash, name, auth_provider, avatar_url, email_verified,
  email_verification_code, email_verification_expires,
  google_id, facebook_id, tiktok_id, apple_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, email, name, avatar_url, email_verified;

-- name: GetUserAuthState :one
SELECT id, email, name, password_hash, auth_provider, is_active,
       email_verified, email_verification_code, email_verification_expires,
       password_reset_code, password_reset_expires, password_reset_attempts
FROM users WHERE email = $1 LIMIT 1;

-- name: UpdateUserVerification :exec
UPDATE users SET email_verified = $2, email_verification_code = $3,
  email_verification_expires = $4, updated_at = now() WHERE id = $1;

-- name: SetEmailVerificationCode :exec
UPDATE users SET email_verification_code = $2, email_verification_expires = $3,
  updated_at = now() WHERE id = $1;

-- name: SetPasswordResetCode :exec
UPDATE users SET password_reset_code = $2, password_reset_expires = $3,
  password_reset_attempts = $4, updated_at = now() WHERE id = $1;

-- name: SetPasswordResetAttempts :exec
UPDATE users SET password_reset_attempts = $2, updated_at = now() WHERE id = $1;

-- name: ResetPasswordHash :exec
UPDATE users SET password_hash = $2, password_reset_code = null,
  password_reset_expires = null, password_reset_attempts = 0,
  updated_at = now() WHERE id = $1;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `sqlc generate`
Expected: succeeds. Note the generated param struct names + field types (nullable varchar → `*string`, nullable timestamptz → `pgtype.Timestamptz`, int → `int32`). Step A3 uses them.

- [ ] **Step 4: Commit**
```bash
git add internal/user/domain.go internal/user/service.go internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc
git commit -m "feat(go-port): user write DTOs + lifecycle SQL (create/verify/reset)"
```

---

## Task A3: User repo — Create / Update / AuthState (+ service passthrough)

**Files:**
- Modify: `internal/user/repo_pg.go`, `internal/user/service.go`
- Test: `internal/user/service_test.go`

- [ ] **Step 1: Write failing tests**

Extend the `fakeRepo` in `service_test.go` to satisfy the new `Repo` methods, then add:
```go
func TestService_Create_Passthrough(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	u, err := s.Create(context.Background(), user.NewUser{Email: "n@b.com", Name: "N", PasswordHash: "h"})
	if err != nil || u.Email != "n@b.com" {
		t.Fatalf("create: %v %v", u, err)
	}
}

func TestService_Update_Passthrough(t *testing.T) {
	r := &fakeRepo{byEmail: map[string]user.User{}}
	s := user.NewService(r)
	on := true
	if err := s.Update(context.Background(), "u1", user.UserPatch{EmailVerified: &on}); err != nil {
		t.Fatal(err)
	}
	if r.lastPatch.EmailVerified == nil || !*r.lastPatch.EmailVerified {
		t.Fatal("patch not forwarded")
	}
}
```
Add `lastPatch user.UserPatch`, `created []user.NewUser` fields to `fakeRepo` and implement the new methods on it (Create records + returns a User from the NewUser; Update records `lastPatch`; FindBySocialId/AuthState return `ErrNotFound`/zero).

- [ ] **Step 2: Run, verify fail** — Run: `go test ./internal/user/ -run Passthrough -v` → FAIL (Service lacks Create/Update).

- [ ] **Step 3: Add service passthroughs**

In `service.go`:
```go
func (s *Service) Create(ctx context.Context, in NewUser) (User, error) { return s.repo.Create(ctx, in) }
func (s *Service) Update(ctx context.Context, id string, p UserPatch) error { return s.repo.Update(ctx, id, p) }
func (s *Service) FindBySocialId(ctx context.Context, provider, socialID string) (User, error) {
	return s.repo.FindBySocialId(ctx, provider, socialID)
}
func (s *Service) AuthState(ctx context.Context, email string) (AuthState, error) {
	return s.repo.AuthState(ctx, email)
}
```

- [ ] **Step 4: Implement repo methods in `repo_pg.go`**

Extend `pgQuerier` with the new generated methods (`CreateUser`, `GetUserAuthState`, `UpdateUserVerification`, `SetEmailVerificationCode`, `SetPasswordResetCode`, `SetPasswordResetAttempts`, `ResetPasswordHash`, plus the social find/link methods added in Part B — add those signatures in B1). Then:

```go
// helpers
func ptr(s string) *string { if s == "" { return nil }; return &s }
func ts(t *time.Time) pgtype.Timestamptz {
	var v pgtype.Timestamptz
	if t != nil { v.Time = *t; v.Valid = true }
	return v
}
func tsValue(v pgtype.Timestamptz) *time.Time {
	if !v.Valid { return nil }
	t := v.Time; return &t
}

func (r *PgRepo) Create(ctx context.Context, in NewUser) (User, error) {
	ap := in.AuthProvider
	if ap == "" { ap = "local" }
	args := sqlc.CreateUserParams{
		Email: in.Email, PasswordHash: ptr(in.PasswordHash), Name: in.Name,
		AuthProvider: sqlc.UsersAuthProviderEnum(ap), AvatarUrl: ptr(in.AvatarURL),
		EmailVerified: in.EmailVerified,
		EmailVerificationCode: ptr(in.EmailVerificationCode),
		EmailVerificationExpires: ts(in.EmailVerificationExpires),
	}
	switch in.SocialProvider {
	case "google":   args.GoogleID = ptr(in.SocialID)
	case "facebook": args.FacebookID = ptr(in.SocialID)
	case "tiktok":   args.TiktokID = ptr(in.SocialID)
	case "apple":    args.AppleID = ptr(in.SocialID)
	}
	row, err := r.q.CreateUser(ctx, args)
	if err != nil { return User{}, err }
	return User{ID: uuidStr(row.ID), Email: row.Email, Name: row.Name,
		AvatarURL: strOrEmpty(row.AvatarUrl), EmailVerified: row.EmailVerified}, nil
}
```
> Confirm generated field names against the sqlc output (`AvatarUrl` vs `AvatarURL`, enum type name `UsersAuthProviderEnum`). Match exactly what `sqlc generate` produced.

`Update` dispatches to the narrowest generated query matching the patch shape. Because TypeORM did one dynamic UPDATE but our SQL is fixed-shape, implement `Update` as a small switch over which fields the patch sets — each lifecycle caller sets a known combination, so map them:
```go
func (r *PgRepo) Update(ctx context.Context, id string, p UserPatch) error {
	uid, err := parseUUID(id)
	if err != nil { return ErrNotFound }
	switch {
	// verify-email success: email_verified + clear code/expires
	case p.EmailVerified != nil && p.EmailVerificationCode.Set:
		return r.q.UpdateUserVerification(ctx, sqlc.UpdateUserVerificationParams{
			ID: uid, EmailVerified: *p.EmailVerified,
			EmailVerificationCode: p.EmailVerificationCode.Value,
			EmailVerificationExpires: ts(p.EmailVerificationExpires.Value),
		})
	// resend: set new code+expires
	case p.EmailVerificationCode.Set:
		return r.q.SetEmailVerificationCode(ctx, sqlc.SetEmailVerificationCodeParams{
			ID: uid, EmailVerificationCode: p.EmailVerificationCode.Value,
			EmailVerificationExpires: ts(p.EmailVerificationExpires.Value),
		})
	// forgot: set reset code+expires+attempts=0
	case p.PasswordResetCode.Set:
		var att int32
		if p.PasswordResetAttempts != nil { att = int32(*p.PasswordResetAttempts) }
		return r.q.SetPasswordResetCode(ctx, sqlc.SetPasswordResetCodeParams{
			ID: uid, PasswordResetCode: p.PasswordResetCode.Value,
			PasswordResetExpires: ts(p.PasswordResetExpires.Value),
			PasswordResetAttempts: att,
		})
	// reset throttle: attempts only
	case p.PasswordResetAttempts != nil:
		return r.q.SetPasswordResetAttempts(ctx, sqlc.SetPasswordResetAttemptsParams{
			ID: uid, PasswordResetAttempts: int32(*p.PasswordResetAttempts),
		})
	// reset success: new hash + clear reset fields
	case p.PasswordHash != nil:
		return r.q.ResetPasswordHash(ctx, sqlc.ResetPasswordHashParams{
			ID: uid, PasswordHash: p.PasswordHash,
		})
	// social link (Part B)
	case p.SocialProvider != "":
		return r.linkSocial(ctx, uid, p) // implemented in Task B1
	}
	return nil
}
```
> The reset-throttle-burn case (`exhausted`) sets PasswordResetCode (Set, Value=nil) AND attempts=0 → reuse `SetPasswordResetCode` with nil code/expires. The usecase builds the right patch; see Task A11. The `ClearPasswordReset` shape == `SetPasswordResetCode` with nulls, so no separate query needed.

`AuthState`:
```go
func (r *PgRepo) AuthState(ctx context.Context, email string) (AuthState, error) {
	row, err := r.q.GetUserAuthState(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) { return AuthState{}, ErrNotFound }
	if err != nil { return AuthState{}, err }
	return AuthState{
		ID: uuidStr(row.ID), Email: row.Email, Name: row.Name,
		PasswordHash: strOrEmpty(row.PasswordHash), AuthProvider: string(row.AuthProvider),
		IsActive: row.IsActive, EmailVerified: row.EmailVerified,
		EmailVerificationCode: strOrEmpty(row.EmailVerificationCode),
		EmailVerificationExpires: tsValue(row.EmailVerificationExpires),
		PasswordResetCode: strOrEmpty(row.PasswordResetCode),
		PasswordResetExpires: tsValue(row.PasswordResetExpires),
		PasswordResetAttempts: int(row.PasswordResetAttempts),
	}, nil
}
```

- [ ] **Step 5: Run tests** — Run: `go test ./internal/user/ -race -v` → PASS. Also `go build ./...`.

- [ ] **Step 6: Commit**
```bash
git add internal/user/
git commit -m "feat(go-port): user repo Create/Update/AuthState over pgx"
```

---

## Task A4: Plans module — `FindFreePlan`

**Files:**
- Create: `internal/plans/reader.go`, `internal/plans/reader_test.go`
- Modify: `internal/shared/pg/queries_auth.sql`

- [ ] **Step 1: Add query to `queries_auth.sql`**
```sql
-- name: FindFreePlan :one
SELECT id, name, daily_limit FROM plans
WHERE billing_period = 'none'::plans_billing_period_enum AND is_active = true
LIMIT 1;
```
Run `sqlc generate`.

- [ ] **Step 2: Write failing test** — `internal/plans/reader_test.go`:
```go
package plans_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/plans"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct{ row sqlc.FindFreePlanRow; err error }
func (f fakeQ) FindFreePlan(context.Context) (sqlc.FindFreePlanRow, error) { return f.row, f.err }

func TestFindFreePlan(t *testing.T) {
	var id pgtype.UUID; _ = id.Scan("11111111-1111-1111-1111-111111111111")
	r := plans.NewReader(fakeQ{row: sqlc.FindFreePlanRow{ID: id, Name: "Free", DailyLimit: 10}})
	p, err := r.FindFreePlan(context.Background())
	if err != nil || p.ID == "" || p.Name != "Free" || p.DailyLimit != 10 {
		t.Fatalf("got %+v err %v", p, err)
	}
}
```

- [ ] **Step 3: Run, verify fail** — `go test ./internal/plans/ -v` → FAIL (no package).

- [ ] **Step 4: Implement `reader.go`**
```go
// Package plans reads plan rows. Phase 1b needs only the free plan
// (assigned on register + social signup). Plan WRITES are admin (Phase 4).
package plans

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Plan is the trimmed projection callers need.
type Plan struct {
	ID         string
	Name       string
	DailyLimit int
}

// Querier is the one-method sqlc subset.
type Querier interface {
	FindFreePlan(ctx context.Context) (sqlc.FindFreePlanRow, error)
}

// Reader resolves plans.
type Reader struct{ q Querier }

// NewReader wires the querier.
func NewReader(q Querier) *Reader { return &Reader{q: q} }

// FindFreePlan returns the active billing_period='none' plan. Node
// throws "Free plan not found. Run seed first." when absent; here the
// pgx.ErrNoRows propagates → 500 (seed is a deploy invariant).
func (r *Reader) FindFreePlan(ctx context.Context) (Plan, error) {
	row, err := r.q.FindFreePlan(ctx)
	if err != nil {
		return Plan{}, err
	}
	return Plan{ID: row.ID.String(), Name: row.Name, DailyLimit: int(row.DailyLimit)}, nil
}
```

- [ ] **Step 5: Run** — `go test ./internal/plans/ -race -v` → PASS.

- [ ] **Step 6: Commit**
```bash
git add internal/plans/ internal/shared/pg/
git commit -m "feat(go-port): plans.Reader.FindFreePlan"
```

---

## Task A5: Subscription — `Writer.CreateFree`

**Files:**
- Create: `internal/subscription/writer.go`, `internal/subscription/writer_test.go`
- Modify: `internal/shared/pg/queries_auth.sql`

- [ ] **Step 1: Add query**
```sql
-- name: CreateFreeSubscription :exec
INSERT INTO subscriptions (user_id, plan_id, status, store_type, started_at, expires_at)
VALUES ($1, $2, 'active'::subscriptions_status_enum,
        'admin_granted'::subscriptions_store_type_enum, now(), null);
```
Run `sqlc generate`. (Confirm the enum type names against schema.sql — `subscriptions_status_enum` / `subscriptions_store_type_enum`.)

- [ ] **Step 2: Write failing test** — `writer_test.go`:
```go
package subscription_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/tannpv/draftright-rewrite/internal/subscription"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeWQ struct{ called bool; arg sqlc.CreateFreeSubscriptionParams }
func (f *fakeWQ) CreateFreeSubscription(_ context.Context, a sqlc.CreateFreeSubscriptionParams) error {
	f.called = true; f.arg = a; return nil
}

func TestCreateFree(t *testing.T) {
	q := &fakeWQ{}
	w := subscription.NewWriter(q)
	if err := w.CreateFree(context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatal(err)
	}
	if !q.called { t.Fatal("query not called") }
	var want pgtype.UUID; _ = want.Scan("11111111-1111-1111-1111-111111111111")
	if q.arg.UserID != want { t.Fatalf("user id not bound: %v", q.arg.UserID) }
}
```

- [ ] **Step 3: Run, verify fail** — `go test ./internal/subscription/ -run CreateFree -v` → FAIL.

- [ ] **Step 4: Implement `writer.go`**
```go
package subscription

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// WriteQuerier is the sqlc subset the writer needs.
type WriteQuerier interface {
	CreateFreeSubscription(ctx context.Context, arg sqlc.CreateFreeSubscriptionParams) error
}

// Writer creates subscription rows. Phase 1b: the free grant on signup.
// Paid grants + lifecycle land in Phase 2 on this same seam.
type Writer struct{ q WriteQuerier }

// NewWriter wires the querier.
func NewWriter(q WriteQuerier) *Writer { return &Writer{q: q} }

// CreateFree inserts an ACTIVE admin_granted free subscription
// (started_at=now, expires_at=null), mirroring
// subscriptionsService.createFreeSubscription.
func (w *Writer) CreateFree(ctx context.Context, userID, planID string) error {
	var uid, pid pgtype.UUID
	if err := uid.Scan(userID); err != nil { return err }
	if err := pid.Scan(planID); err != nil { return err }
	return w.q.CreateFreeSubscription(ctx, sqlc.CreateFreeSubscriptionParams{UserID: uid, PlanID: pid})
}
```

- [ ] **Step 5: Run** — `go test ./internal/subscription/ -race -v` → PASS.

- [ ] **Step 6: Commit**
```bash
git add internal/subscription/ internal/shared/pg/
git commit -m "feat(go-port): subscription.Writer.CreateFree"
```

---

## Task A6: Email templates — render + subjects

**Files:**
- Create: `internal/email/templates.go`, `internal/email/templates_test.go`

- [ ] **Step 1: Write failing test** — `templates_test.go`:
```go
package email

import "testing"

func TestRender_VerificationSubjectAndSubst(t *testing.T) {
	subj, html := renderTemplate("verification", map[string]string{"name": "Al", "code": "123456"})
	if subj != "Welcome to DraftRight — confirm your email" {
		t.Fatalf("subject: %q", subj)
	}
	if !contains(html, "123456") || !contains(html, "Al") {
		t.Fatalf("substitution missing in html")
	}
}

func TestRender_PasswordResetSubject(t *testing.T) {
	subj, _ := renderTemplate("password-reset", map[string]string{"name": "Al", "code": "000111"})
	if subj != "Reset your DraftRight password" { t.Fatalf("subject: %q", subj) }
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ { if s[i:i+len(sub)] == sub { return i } }
	return -1
}
```

- [ ] **Step 2: Run, verify fail** — `go test ./internal/email/ -run Render -v` → FAIL (no package).

- [ ] **Step 3: Implement `templates.go`**

Port `EMAIL_TEMPLATES` (verification + password-reset are the two auth needs; include the others for completeness/Phase-2 reuse). `{{var}}` substitution is literal replace:
```go
package email

import "strings"

type templateDef struct {
	subject string
	html    string
}

func shell(title, body string) string {
	return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">` + title + `</h1>
    ` + body + `
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`
}

// builtinTemplates mirrors email-templates.ts EMAIL_TEMPLATE_MAP. Bodies
// use {{var}} placeholders substituted at send. NOT shadow-gated (email
// is out-of-band) — ported for functional parity so users get the same
// mail the Node backend sent.
var builtinTemplates = map[string]templateDef{
	"verification": {
		subject: "Welcome to DraftRight — confirm your email",
		html: shell("Welcome to DraftRight, {{name}} 👋",
			`<p style="color:#444;line-height:1.6;margin:0 0 16px;">Thanks for joining DraftRight — your AI writing companion. Select any text, pick a tone, and get a polished rewrite in a tap, right from the keyboard, the apps, or the web playground.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 8px;">One quick step to activate your account — enter this code in the app:</p>
    <p style="font-size:30px;font-weight:700;letter-spacing:6px;color:#5b3df6;background:#f3f0ff;border-radius:10px;text-align:center;padding:14px 0;margin:0 0 8px;">{{code}}</p>
    <p style="color:#888;font-size:13px;line-height:1.5;margin:0 0 20px;">The code expires in 15 minutes. If you didn't create a DraftRight account, you can safely ignore this email.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 4px;">Once you're in, try the tones — Simple, Polished, Concise, Natural and more.</p>
    <p style="color:#444;line-height:1.6;margin:0;">Questions? Just reply to this email.</p>`),
	},
	"password-reset": {
		subject: "Reset your DraftRight password",
		html: shell("Reset your password, {{name}}",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your password reset code is:</p>
    <p style="font-size:28px;font-weight:700;letter-spacing:4px;color:#5b3df6;margin:0 0 16px;">{{code}}</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Enter it on the reset page to choose a new password. It expires in 15 minutes. If you didn't request this, you can ignore this email.</p>`),
	},
}

// renderTemplate substitutes {{var}} tokens. Unknown key → empty subject
// + empty html (caller logs). DB template override (email_templates) is
// applied by the Service before calling this (Task A7).
func renderTemplate(key string, vars map[string]string) (subject, html string) {
	t, ok := builtinTemplates[key]
	if !ok {
		return "", ""
	}
	return substitute(t.subject, vars), substitute(t.html, vars)
}

func substitute(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
```

- [ ] **Step 4: Run** — `go test ./internal/email/ -run Render -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/email/templates.go internal/email/templates_test.go
git commit -m "feat(go-port): email templates + {{var}} render"
```

---

## Task A7: Email service — Resend adapter + deliver path

**Files:**
- Create: `internal/email/service.go`, `internal/email/resend.go`, `internal/email/service_test.go`
- Modify: `internal/shared/pg/queries_auth.sql`

- [ ] **Step 1: Add suppression + log queries**
```sql
-- name: IsEmailSuppressed :one
SELECT COUNT(*) > 0 FROM email_suppressions WHERE email = $1;

-- name: InsertEmailLog :exec
INSERT INTO email_logs (to_email, email_type, subject, status, provider_id, error)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEmailSettings :one
SELECT resend_api_key, email_from FROM app_settings LIMIT 1;

-- name: GetEmailTemplateByKey :one
SELECT subject, html FROM email_templates WHERE key = $1 LIMIT 1;
```
Run `sqlc generate`. (Confirm `email_templates` has `key/subject/html` columns; if its PK/columns differ, adjust — check schema.sql `CREATE TABLE public.email_templates`.)

- [ ] **Step 2: Write failing tests** — `service_test.go`:
```go
package email

import (
	"context"
	"testing"
)

type fakeEQ struct {
	suppressed bool
	logs       []string
	apiKey     string
	from       string
}
func (f *fakeEQ) IsEmailSuppressed(_ context.Context, _ string) (bool, error) { return f.suppressed, nil }
func (f *fakeEQ) InsertEmailLog(_ context.Context, a InsertEmailLogArgs) error {
	f.logs = append(f.logs, a.Status); return nil
}
func (f *fakeEQ) GetEmailSettings(_ context.Context) (string, string, error) { return f.apiKey, f.from, nil }
func (f *fakeEQ) GetEmailTemplate(_ context.Context, _ string) (string, string, bool) { return "", "", false }

type fakeSender struct{ called bool; id string; err error }
func (s *fakeSender) Send(_ context.Context, from, to, subject, html string) (string, error) {
	s.called = true; return s.id, s.err
}

func TestDeliver_SuppressedSkips(t *testing.T) {
	q := &fakeEQ{suppressed: true}
	snd := &fakeSender{}
	svc := newServiceWith(q, snd, "")
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if snd.called { t.Fatal("must not send to suppressed") }
	if len(q.logs) == 0 || q.logs[0] != "suppressed" { t.Fatalf("logs: %v", q.logs) }
}

func TestDeliver_NoKeySkips(t *testing.T) {
	q := &fakeEQ{}
	snd := &fakeSender{}
	svc := newServiceWith(q, snd, "") // env key empty + settings empty
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if snd.called { t.Fatal("no key → no send") }
	if q.logs[0] != "skipped" { t.Fatalf("logs: %v", q.logs) }
}

func TestDeliver_SendsAndLogsSent(t *testing.T) {
	q := &fakeEQ{apiKey: "re_1"}
	snd := &fakeSender{id: "msg_1"}
	svc := newServiceWith(q, snd, "")
	svc.SendVerification(context.Background(), "x@y.z", "Al", "123456")
	svc.wait()
	if !snd.called { t.Fatal("should send") }
	if q.logs[0] != "sent" { t.Fatalf("logs: %v", q.logs) }
}
```
> The test fires-and-forgets via goroutine; `newServiceWith` + `svc.wait()` are test seams: the Service tracks an internal `sync.WaitGroup` for in-flight sends (used ONLY in tests via `wait()`), so the assertion is deterministic. In prod nothing calls `wait()`.

- [ ] **Step 3: Run, verify fail** — `go test ./internal/email/ -run Deliver -v` → FAIL.

- [ ] **Step 4: Implement `resend.go`**
```go
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Sender posts one email. The real impl hits Resend; tests fake it.
type Sender interface {
	Send(ctx context.Context, from, to, subject, html string) (providerID string, err error)
}

// resendSender posts to the Resend HTTP API (no SDK). apiKey is resolved
// per-send by the Service and passed in via a closure-bound key, so key
// rotation in app_settings takes effect without a restart.
type resendClient struct{ http *http.Client }

func newResendClient() *resendClient { return &resendClient{http: &http.Client{}} }

func (c *resendClient) send(ctx context.Context, apiKey, from, to, subject, html string) (string, error) {
	body, _ := json.Marshal(map[string]string{"from": from, "to": to, "subject": subject, "html": html})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	var out struct {
		Data  *struct{ ID string `json:"id"` } `json:"data"`
		Error *struct{ Message string `json:"message"` } `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 400 || out.Error != nil {
		msg := "send failed"
		if out.Error != nil { msg = out.Error.Message }
		return "", fmt.Errorf("%s", msg)
	}
	if out.Data != nil { return out.Data.ID, nil }
	return "", nil
}
```

- [ ] **Step 5: Implement `service.go`**
```go
// Package email ports the NestJS EmailService: a single deliver path
// (suppression check → creds → Resend POST → email_logs row) behind two
// auth-facing methods. Sends are FIRE-AND-FORGET — they never block or
// fail the HTTP request (mirrors auth.service's `.catch(log)`). Email
// HTML is out-of-band, so it is NOT shadow-gated.
package email

import (
	"context"
	"log/slog"
	"sync"
)

// Querier is the sqlc subset the service needs.
type Querier interface {
	IsEmailSuppressed(ctx context.Context, email string) (bool, error)
	InsertEmailLog(ctx context.Context, a InsertEmailLogArgs) error
	GetEmailSettings(ctx context.Context) (apiKey, from string, err error)
	GetEmailTemplate(ctx context.Context, key string) (subject, html string, ok bool)
}

// InsertEmailLogArgs is the audit-row payload (thin wrapper so the
// Querier port doesn't leak sqlc param structs to fakes).
type InsertEmailLogArgs struct {
	To, Type, Subject, Status string
	ProviderID, Error         *string
}

// Config carries the env fallbacks (app_settings overrides them).
type Config struct {
	EnvAPIKey string
	EnvFrom   string
}

const defaultFrom = "DraftRight <noreply@draftright.info>"

// Service is the email sender. send goroutines are tracked by wg so
// tests can deterministically await them; prod never waits.
type Service struct {
	q      Querier
	cfg    Config
	client *resendClient
	wg     sync.WaitGroup
}

// NewService wires the querier + env config + real Resend client.
func NewService(q Querier, cfg Config) *Service {
	return &Service{q: q, cfg: cfg, client: newResendClient()}
}

// SendVerification + SendPasswordReset are the two auth needs. Both
// fire-and-forget.
func (s *Service) SendVerification(ctx context.Context, to, name, code string) {
	s.fire(ctx, "verification", to, map[string]string{"name": orThere(name), "code": code})
}
func (s *Service) SendPasswordReset(ctx context.Context, to, name, code string) {
	s.fire(ctx, "password-reset", to, map[string]string{"name": orThere(name), "code": code})
}

func orThere(n string) string { if n == "" { return "there" }; return n }

func (s *Service) fire(ctx context.Context, key, to string, vars map[string]string) {
	subject, html := s.render(ctx, key, vars)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { _ = recover() }() // never let an email panic escape
		s.deliver(context.WithoutCancel(ctx), to, subject, html, key)
	}()
}

// render applies a DB template override when present, else the built-in.
func (s *Service) render(ctx context.Context, key string, vars map[string]string) (string, string) {
	if subj, html, ok := s.q.GetEmailTemplate(ctx, key); ok {
		return substitute(subj, vars), substitute(html, vars)
	}
	return renderTemplate(key, vars)
}

func (s *Service) deliver(ctx context.Context, to, subject, html, label string) {
	if sup, err := s.q.IsEmailSuppressed(ctx, lower(to)); err == nil && sup {
		s.log(ctx, to, subject, label, "suppressed", nil, strp("Recipient on suppression list (bounce/complaint)"))
		return
	}
	apiKey, from := s.creds(ctx)
	if apiKey == "" {
		s.log(ctx, to, subject, label, "skipped", nil, strp("Resend not configured"))
		return
	}
	id, err := s.client.send(ctx, apiKey, from, to, subject, html)
	if err != nil {
		slog.Warn("email send failed", "label", label, "to", to, "err", err)
		s.log(ctx, to, subject, label, "failed", nil, strp(err.Error()))
		return
	}
	s.log(ctx, to, subject, label, "sent", strpOrNil(id), nil)
}

func (s *Service) creds(ctx context.Context) (string, string) {
	apiKey, from, err := s.q.GetEmailSettings(ctx)
	if err != nil { apiKey, from = "", "" }
	if apiKey == "" { apiKey = s.cfg.EnvAPIKey }
	if from == "" { from = s.cfg.EnvFrom }
	if from == "" { from = defaultFrom }
	return apiKey, from
}

func (s *Service) log(ctx context.Context, to, subject, label, status string, providerID, errMsg *string) {
	_ = s.q.InsertEmailLog(ctx, InsertEmailLogArgs{
		To: to, Type: label, Subject: subject, Status: status, ProviderID: providerID, Error: errMsg,
	})
}

func lower(s string) string { return toLower(s) }
func strp(s string) *string { return &s }
func strpOrNil(s string) *string { if s == "" { return nil }; return &s }
```
Add small helpers `toLower` (use `strings.ToLower`) — or just inline `strings.ToLower`. Add the test seam in `service_test.go` support file: `newServiceWith(q, sender, envKey)` constructs a `Service` with the fake sender (introduce an unexported `send func` field OR an interface; simplest: make `client` an interface `sender` with `send(ctx,apiKey,...)`). Refactor `resendClient` to satisfy it and let tests inject `fakeSender`-equivalent. **Add `func (s *Service) wait() { s.wg.Wait() }`** (test-only helper, unexported, fine to live in service.go).

> Adjust the fake/real seam so `fakeSender.Send(ctx,from,to,subject,html)` matches the production call — bind apiKey inside the Service before calling `sender.Send`. Keep the port shape the test expects (`Send(ctx, from, to, subject, html)`); pass apiKey via a Service field set in `creds`. Implementer: reconcile the two — the test in Step 2 is the contract.

- [ ] **Step 6: Implement the email-module repo wrapper** that adapts sqlc → `email.Querier` (lives in `internal/email/repo_pg.go` or wired in main via an adapter). Map `GetEmailSettings`→`GetEmailSettings` row, `GetEmailTemplate`→`GetEmailTemplateByKey` (return ok=false on `pgx.ErrNoRows`), `IsEmailSuppressed`→bool, `InsertEmailLog`→params struct (nullable provider_id/error as `*string`).

- [ ] **Step 7: Run** — `go test ./internal/email/ -race -v` → PASS. `go build ./...`.

- [ ] **Step 8: Commit**
```bash
git add internal/email/ internal/shared/pg/
git commit -m "feat(go-port): email.Service — Resend deliver path, fire-and-forget"
```

---

## Task A8: Auth — code machinery + new ports

**Files:**
- Create: `internal/auth/codes.go`, `internal/auth/codes_test.go`
- Modify: `internal/auth/usecase.go`, `internal/auth/errors.go`

- [ ] **Step 1: Write failing test** — `codes_test.go`:
```go
package auth

import "testing"

func TestGenerateCode_SixDigitsPadded(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := generateCode()
		if len(c) != 6 { t.Fatalf("len %q", c) }
		for _, r := range c { if r < '0' || r > '9' { t.Fatalf("non-digit %q", c) } }
	}
}
```

- [ ] **Step 2: Run, verify fail** — `go test ./internal/auth/ -run GenerateCode -v` → FAIL.

- [ ] **Step 3: Implement `codes.go`**
```go
package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// emailCodeTTL mirrors EMAIL_CODE_TTL_MS (app-config.ts) — 15 minutes.
const emailCodeTTL = 15 * time.Minute

// maxResetAttempts mirrors AuthService.MAX_RESET_ATTEMPTS.
const maxResetAttempts = 5

// generateCode returns a zero-padded 6-digit CSPRNG code, exactly Node's
// randomInt(0, 1_000_000).toString().padStart(6, '0').
func generateCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		// crypto/rand failure is catastrophic; Node would throw too.
		panic("auth: csprng unavailable: " + err.Error())
	}
	return fmt.Sprintf("%06d", n.Int64())
}
```

- [ ] **Step 4: Add error constants to `errors.go`**
```go
const (
	msgEmailRegistered   = "Email already registered"
	msgInvalidVerifyCode = "Invalid or expired verification code"
	msgInvalidResetCode  = "Invalid or expired reset code"
	msgPasswordTooShort  = "Password must be at least 8 characters"
	msgEmailRequired     = "Email is required for social login"
	msgEmailRegisteredSocial = "This email is registered. Sign in with your password to link this account."
)
```
Add a `BadRequestError` type (400, code `invalid-input`) alongside `AuthError` (401, code `invalid-token`):
```go
// BadRequestError is a 400-class failure carrying the exact Node
// message. The handler maps it to {error, code:"invalid-input"} → 400.
type BadRequestError struct{ Message string }
func (e *BadRequestError) Error() string { return e.Message }
func badRequest(msg string) *BadRequestError { return &BadRequestError{Message: msg} }

// ConflictError → 409, code "conflict".
type ConflictError struct{ Message string }
func (e *ConflictError) Error() string { return e.Message }
func conflict(msg string) *ConflictError { return &ConflictError{Message: msg} }
```

- [ ] **Step 5: Extend ports in `usecase.go`**

Extend `UserService`:
```go
type UserService interface {
	ByEmail(ctx context.Context, email string) (user.User, error)
	ByID(ctx context.Context, id string) (user.User, error)
	UpdatePasswordHash(ctx context.Context, id, hash string) error
	DeleteAccount(ctx context.Context, id string) error
	Create(ctx context.Context, in user.NewUser) (user.User, error)
	Update(ctx context.Context, id string, p user.UserPatch) error
	FindBySocialId(ctx context.Context, provider, socialID string) (user.User, error)
	AuthState(ctx context.Context, email string) (user.AuthState, error)
}
```
Add the new ports:
```go
// FreePlanReader is the plans-module subset (free-plan lookup on signup).
type FreePlanReader interface {
	FindFreePlan(ctx context.Context) (plans.Plan, error)
}

// FreeSubWriter is the subscription-module subset (free grant on signup).
type FreeSubWriter interface {
	CreateFree(ctx context.Context, userID, planID string) error
}

// EmailSender is the email-module subset. Both methods fire-and-forget.
type EmailSender interface {
	SendVerification(ctx context.Context, to, name, code string)
	SendPasswordReset(ctx context.Context, to, name, code string)
}

// SocialVerifier verifies a provider id_token → SocialProfile (Part B).
type SocialVerifier interface {
	Verify(ctx context.Context, provider, idToken string, profile InboundProfile) (SocialProfile, error)
}
```
Extend `Service` struct + `NewService` to hold/accept `plans FreePlanReader`, `subWriter FreeSubWriter`, `email EmailSender`, `social SocialVerifier`. Update `NewService` signature (append the 4 params after the secrets). Add `import` for `internal/plans`.

> **This breaks existing `NewService` callers** (`main.go` + `usecase_test.go`'s `newSvc`). Update `newSvc` in the test helper to pass fakes (`stubPlans{}`, `stubSubWriter{}`, `stubEmail{}`, `stubSocial{}`); update `main.go` in Task A15/B9. Add the stub types to the test file (no-op implementations).

- [ ] **Step 6: Run** — `go test ./internal/auth/ -run GenerateCode -race -v` → PASS; `go build ./internal/auth/` compiles (after updating `newSvc` stub).

- [ ] **Step 7: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): auth code machinery + lifecycle/social ports"
```

---

## Task A9: Auth — Register usecase

**Files:**
- Modify: `internal/auth/usecase.go`
- Test: `internal/auth/register_test.go`

- [ ] **Step 1: Write failing tests** — `register_test.go`:
```go
package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func TestRegister_HappyPath(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	res, err := svc.Register(context.Background(), "  NEW@B.com ", "password8", "Al")
	if err != nil { t.Fatalf("register: %v", err) }
	if res.User.Email != "new@b.com" { t.Fatalf("email not normalized: %q", res.User.Email) }
	if res.User.EmailVerified { t.Fatal("email_verified must be false") }
	if res.AccessToken == "" || res.RefreshToken == "" { t.Fatal("missing tokens") }
	if !users.created { t.Fatal("user not created") }
}

func TestRegister_DuplicateEmail(t *testing.T) {
	users := newStubUsers()
	users.byEmail["dup@b.com"] = user.User{ID: "u1", Email: "dup@b.com"}
	svc := newSvc(t, users)
	_, err := svc.Register(context.Background(), "dup@b.com", "password8", "Al")
	assertConflict(t, err, "Email already registered")
}
```
Add helpers to the auth test files: `newStubUsers()` returning a stub satisfying the extended `UserService` (with `created bool`, `lastNew user.NewUser`, `byEmail`, `byID` maps; `Create` records + returns a `user.User{ID:"new", Email: in.Email, Name: in.Name}`; `AuthState`/`FindBySocialId` zero/ErrNotFound). `assertConflict` mirrors `assertAuthErr` but checks `*ConflictError`. Add `RegisterResult` expectations.

- [ ] **Step 2: Run, verify fail** — `go test ./internal/auth/ -run Register -v` → FAIL.

- [ ] **Step 3: Implement `Register`** in `usecase.go`:
```go
// RegisterResult is register's return — tokens + the trimmed user with
// the literal email_verified:false (Node returns the literal, not the col).
type RegisterResult struct {
	Tokens
	User RegisterUser
}
type RegisterUser struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
}

// Register ports auth.service.register exactly (order matters).
func (s *Service) Register(ctx context.Context, email, password, name string) (RegisterResult, error) {
	norm := normalizeEmail(email)
	if _, err := s.users.ByEmail(ctx, norm); err == nil {
		return RegisterResult{}, conflict(msgEmailRegistered)
	} else if !errors.Is(err, user.ErrNotFound) {
		return RegisterResult{}, err
	}
	hash, err := shared.HashPassword(password)
	if err != nil { return RegisterResult{}, err }
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	u, err := s.users.Create(ctx, user.NewUser{
		Email: norm, PasswordHash: hash, Name: name,
		EmailVerificationCode: code, EmailVerificationExpires: &expires,
	})
	if err != nil { return RegisterResult{}, err }
	free, err := s.plans.FindFreePlan(ctx)
	if err != nil { return RegisterResult{}, err }
	if err := s.subWriter.CreateFree(ctx, u.ID, free.ID); err != nil {
		return RegisterResult{}, err
	}
	s.email.SendVerification(ctx, norm, name, code) // fire-and-forget
	toks, err := s.generateTokens(ctx, u)
	if err != nil { return RegisterResult{}, err }
	return RegisterResult{Tokens: toks, User: RegisterUser{
		ID: u.ID, Email: u.Email, Name: u.Name, EmailVerified: false,
	}}, nil
}
```
Add helpers (in `usecase.go` or `codes.go`):
```go
func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
func nowUTC() time.Time { return time.Now().UTC() }
```
(import `strings`).

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run Register -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): auth.Register usecase"
```

---

## Task A10: Auth — VerifyEmail / ResendVerification usecases

**Files:**
- Modify: `internal/auth/usecase.go`
- Test: `internal/auth/verify_test.go`

- [ ] **Step 1: Write failing tests** — `verify_test.go`:
```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/user"
)

func authState(code string, exp *time.Time) user.AuthState {
	return user.AuthState{ID: "u1", Email: "a@b.com", Name: "Al",
		EmailVerificationCode: code, EmailVerificationExpires: exp}
}

func TestVerifyEmail_Success(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("123456", &future)
	svc := newSvc(t, users)
	if err := svc.VerifyEmail(context.Background(), "A@b.com ", "123456"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if users.lastPatch.EmailVerified == nil || !*users.lastPatch.EmailVerified {
		t.Fatal("should set email_verified")
	}
}

func TestVerifyEmail_NoCodeOnUser(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = authState("", nil)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "123456"), "Invalid or expired verification code")
}

func TestVerifyEmail_WrongCode(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("111111", &future)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "999999"), "Invalid or expired verification code")
}

func TestVerifyEmail_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	users := newStubUsers()
	users.state["a@b.com"] = authState("123456", &past)
	svc := newSvc(t, users)
	assertBadReq(t, svc.VerifyEmail(context.Background(), "a@b.com", "123456"), "Invalid or expired verification code")
}

func TestVerifyEmail_UnknownUser(t *testing.T) {
	svc := newSvc(t, newStubUsers())
	assertBadReq(t, svc.VerifyEmail(context.Background(), "no@b.com", "123456"), "Invalid or expired verification code")
}

func TestResendVerification_SilentUnknown(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "no@b.com"); err != nil { t.Fatal(err) }
	if users.updateCalled { t.Fatal("must not update unknown user") }
}

func TestResendVerification_SilentAlreadyVerified(t *testing.T) {
	users := newStubUsers()
	st := authState("", nil); st.EmailVerified = true
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "a@b.com"); err != nil { t.Fatal(err) }
	if users.updateCalled { t.Fatal("must not update verified user") }
}

func TestResendVerification_SetsNewCode(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = authState("", nil)
	svc := newSvc(t, users)
	if err := svc.ResendVerification(context.Background(), "a@b.com"); err != nil { t.Fatal(err) }
	if !users.lastPatch.EmailVerificationCode.Set || users.lastPatch.EmailVerificationCode.Value == nil {
		t.Fatal("should set a new code")
	}
}
```
Extend `newStubUsers` to carry `state map[string]user.AuthState`, `lastPatch user.UserPatch`, `updateCalled bool`; `AuthState` reads `state` (ErrNotFound if absent); `Update` records. Add `assertBadReq` (checks `*BadRequestError`).

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement** in `usecase.go`:
```go
func (s *Service) VerifyEmail(ctx context.Context, email, code string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) {
		return badRequest(msgInvalidVerifyCode)
	}
	if err != nil { return err }
	if st.EmailVerificationCode == "" || st.EmailVerificationExpires == nil {
		return badRequest(msgInvalidVerifyCode)
	}
	if st.EmailVerificationCode != code {
		return badRequest(msgInvalidVerifyCode)
	}
	if st.EmailVerificationExpires.Before(nowUTC()) {
		return badRequest(msgInvalidVerifyCode)
	}
	on := true
	return s.users.Update(ctx, st.ID, user.UserPatch{
		EmailVerified:            &on,
		EmailVerificationCode:    user.NullableString{Set: true, Value: nil},
		EmailVerificationExpires: user.NullableTime{Set: true, Value: nil},
	})
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) { return nil } // silent
	if err != nil { return err }
	if st.EmailVerified { return nil } // silent
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	if err := s.users.Update(ctx, st.ID, user.UserPatch{
		EmailVerificationCode:    user.NullableString{Set: true, Value: &code},
		EmailVerificationExpires: user.NullableTime{Set: true, Value: &expires},
	}); err != nil { return err }
	s.email.SendVerification(ctx, st.Email, st.Name, code)
	return nil
}
```
> Compare timestamps in UTC. Node compares `expires.getTime() < Date.now()` (ms wall clock). Storing UTC + comparing UTC matches; the column is `timestamptz` so pgx round-trips an instant — no zone drift.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run "Verify|Resend" -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): auth.VerifyEmail + ResendVerification"
```

---

## Task A11: Auth — ForgotPassword / ResetPassword usecases

**Files:**
- Modify: `internal/auth/usecase.go`
- Test: `internal/auth/reset_test.go`

- [ ] **Step 1: Write failing tests** — `reset_test.go`:
```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/user"
)

func localState(hash string) user.AuthState {
	return user.AuthState{ID: "u1", Email: "a@b.com", Name: "Al",
		PasswordHash: hash, AuthProvider: "local", IsActive: true}
}

func TestForgotPassword_SilentUnknown(t *testing.T) {
	users := newStubUsers()
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "no@b.com"); err != nil { t.Fatal(err) }
	if users.updateCalled { t.Fatal("no update for unknown") }
}

func TestForgotPassword_SilentSocialAccount(t *testing.T) {
	users := newStubUsers()
	st := localState(""); st.AuthProvider = "google"
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "a@b.com"); err != nil { t.Fatal(err) }
	if users.updateCalled { t.Fatal("no update for social-only") }
}

func TestForgotPassword_SetsResetCode(t *testing.T) {
	h, _ := shared.HashPassword("x")
	users := newStubUsers()
	users.state["a@b.com"] = localState(h)
	svc := newSvc(t, users)
	if err := svc.ForgotPassword(context.Background(), "a@b.com"); err != nil { t.Fatal(err) }
	if !users.lastPatch.PasswordResetCode.Set || users.lastPatch.PasswordResetCode.Value == nil {
		t.Fatal("should set reset code")
	}
}

func TestResetPassword_ShortPassword(t *testing.T) {
	svc := newSvc(t, newStubUsers())
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "123456", "short"), "Password must be at least 8 characters")
}

func TestResetPassword_NoResetCode(t *testing.T) {
	users := newStubUsers()
	users.state["a@b.com"] = localState("h") // no reset code set
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "123456", "newpassword"), "Invalid or expired reset code")
}

func TestResetPassword_WrongCodeIncrementsAttempts(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h"); st.PasswordResetCode = "111111"; st.PasswordResetExpires = &future; st.PasswordResetAttempts = 1
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "999999", "newpassword"), "Invalid or expired reset code")
	if users.lastPatch.PasswordResetAttempts == nil || *users.lastPatch.PasswordResetAttempts != 2 {
		t.Fatalf("attempts not incremented: %+v", users.lastPatch.PasswordResetAttempts)
	}
}

func TestResetPassword_BurnsCodeAtCap(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h"); st.PasswordResetCode = "111111"; st.PasswordResetExpires = &future; st.PasswordResetAttempts = 4
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	assertBadReq(t, svc.ResetPassword(context.Background(), "a@b.com", "999999", "newpassword"), "Invalid or expired reset code")
	// attempts 4+1=5 >= 5 → burn: code set to null, attempts 0
	if !users.lastPatch.PasswordResetCode.Set || users.lastPatch.PasswordResetCode.Value != nil {
		t.Fatal("should burn code (set null)")
	}
}

func TestResetPassword_Success(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	users := newStubUsers()
	st := localState("h"); st.PasswordResetCode = "123456"; st.PasswordResetExpires = &future
	users.state["a@b.com"] = st
	svc := newSvc(t, users)
	if err := svc.ResetPassword(context.Background(), "a@b.com", "123456", "newpassword"); err != nil { t.Fatal(err) }
	if users.lastPatch.PasswordHash == nil { t.Fatal("should set new hash") }
}
```

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement** in `usecase.go`:
```go
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) { return nil }
	if err != nil { return err }
	if st.AuthProvider != "local" || st.PasswordHash == "" { return nil } // silent
	code := generateCode()
	expires := nowUTC().Add(emailCodeTTL)
	zero := 0
	if err := s.users.Update(ctx, st.ID, user.UserPatch{
		PasswordResetCode:    user.NullableString{Set: true, Value: &code},
		PasswordResetExpires: user.NullableTime{Set: true, Value: &expires},
		PasswordResetAttempts: &zero,
	}); err != nil { return err }
	s.email.SendPasswordReset(ctx, st.Email, st.Name, code)
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	if len(newPassword) < 8 {
		return badRequest(msgPasswordTooShort)
	}
	st, err := s.users.AuthState(ctx, normalizeEmail(email))
	if errors.Is(err, user.ErrNotFound) { return badRequest(msgInvalidResetCode) }
	if err != nil { return err }
	if st.PasswordResetCode == "" || st.PasswordResetExpires == nil {
		return badRequest(msgInvalidResetCode)
	}
	badCode := st.PasswordResetCode != code || st.PasswordResetExpires.Before(nowUTC())
	if badCode {
		attempts := st.PasswordResetAttempts + 1
		if attempts >= maxResetAttempts {
			// burn the code
			zero := 0
			_ = s.users.Update(ctx, st.ID, user.UserPatch{
				PasswordResetCode:    user.NullableString{Set: true, Value: nil},
				PasswordResetExpires: user.NullableTime{Set: true, Value: nil},
				PasswordResetAttempts: &zero,
			})
		} else {
			_ = s.users.Update(ctx, st.ID, user.UserPatch{PasswordResetAttempts: &attempts})
		}
		return badRequest(msgInvalidResetCode)
	}
	hash, err := shared.HashPassword(newPassword)
	if err != nil { return err }
	return s.users.Update(ctx, st.ID, user.UserPatch{PasswordHash: &hash})
}
```
> The reset-success `Update` sets only `PasswordHash`; the repo's `ResetPasswordHash` query already clears reset code/expires/attempts in the same statement (Task A2/A3), matching Node's combined update. Verify the repo `Update` switch routes `PasswordHash != nil` → `ResetPasswordHash`.

> `len(newPassword) < 8` matches Node `!newPassword || newPassword.length < 8` (empty string has len 0 < 8). Note: this counts **bytes**, JS `.length` counts UTF-16 code units — for ASCII passwords identical; multibyte passwords are a negligible parity edge pinned by the gate. Acceptable.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run Reset -race -v` and `-run Forgot` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): auth.ForgotPassword + ResetPassword (throttle+burn)"
```

---

## Task A12: Register validation (humanized messages)

**Files:**
- Create: `internal/auth/validate.go`, `internal/auth/validate_test.go`

- [ ] **Step 1: Write failing test** — `validate_test.go`:
```go
package auth

import "testing"

func TestValidateRegister(t *testing.T) {
	cases := []struct{ email, pw, name, want string }{
		{"a@b.com", "password8", "Al", ""},
		{"notanemail", "password8", "Al", "Please enter a valid email address."},
		{"a@b.com", "short", "Al", "Password must be at least 8 characters."},
		{"a@b.com", "password8", "", "Name must be at least 1 characters."},
		{"bad", "short", "", "Please enter a valid email address. Password must be at least 8 characters. Name must be at least 1 characters."},
	}
	for _, c := range cases {
		got := validateRegister(c.email, c.pw, c.name)
		if got != c.want { t.Fatalf("validateRegister(%q,%q,%q) = %q, want %q", c.email, c.pw, c.name, got, c.want) }
	}
}
```

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement `validate.go`**
```go
package auth

import (
	"regexp"
	"strings"
)

// emailRe is a permissive RFC-ish check matching class-validator's
// @IsEmail acceptance for the inputs real clients send. Exotic-input
// parity is pinned by the shadow gate (see spec §8).
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// validateRegister reproduces the NestJS ValidationPipe + humanizer for
// RegisterDto, in property-declaration order (email, password, name),
// joined with ". ". Empty result = valid. Code invalid-input, status 400.
func validateRegister(email, password, name string) string {
	var msgs []string
	if !emailRe.MatchString(email) {
		msgs = append(msgs, "Please enter a valid email address.")
	}
	if len(password) < 8 {
		msgs = append(msgs, "Password must be at least 8 characters.")
	}
	if len(name) < 1 {
		msgs = append(msgs, "Name must be at least 1 characters.")
	}
	return strings.Join(msgs, ". ")
}
```

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run ValidateRegister -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/validate.go internal/auth/validate_test.go
git commit -m "feat(go-port): register DTO validation + humanized messages"
```

---

## Task A13: Lifecycle handlers (5 endpoints)

**Files:**
- Modify: `internal/auth/handler.go`
- Test: `internal/auth/handler_lifecycle_test.go`

- [ ] **Step 1: Write failing handler tests** — `handler_lifecycle_test.go`:

Cover: register 201 body + `email_verified:false`; register validation → 400 `invalid-input`; register duplicate → 409 `conflict` body `{error:"Email already registered",code:"conflict",...}`; verify-email 200 `{"success":true}`; verify-email bad → 400; resend 200 `{"success":true}`; forgot 200; reset 200; reset short-pw 400. Use `httptest.NewRecorder` + a `Handler` built from a `Service` over stub ports (reuse the usecase stubs; build `Service` via a test constructor). Assert status + decoded JSON.

```go
func TestHandler_Register_201(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvc(t, users))
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"a@b.com","password":"password8","name":"Al"}`))
	rr := httptest.NewRecorder()
	h.Register(rr, req)
	if rr.Code != http.StatusCreated { t.Fatalf("status %d", rr.Code) }
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	u := body["user"].(map[string]any)
	if u["email_verified"] != false { t.Fatalf("email_verified: %v", u["email_verified"]) }
}
```
(write the rest analogously).

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement handlers** in `handler.go`.

Extend `writeAuthErr` to also map `*BadRequestError` → `invalid-input` and `*ConflictError` → `conflict`:
```go
func writeDomainErr(w http.ResponseWriter, r *http.Request, err error) bool {
	var ae *AuthError
	if errors.As(err, &ae) {
		msg := ae.Message
		if msg == "" { msg = "Unauthorized" }
		shared.WriteError(w, r, "invalid-token", msg)
		return true
	}
	var be *BadRequestError
	if errors.As(err, &be) {
		shared.WriteError(w, r, "invalid-input", be.Message)
		return true
	}
	var ce *ConflictError
	if errors.As(err, &ce) {
		shared.WriteError(w, r, "conflict", ce.Message)
		return true
	}
	return false
}
```
> Keep `writeAuthErr` for the Phase 1a handlers OR migrate them to `writeDomainErr` (it's a superset). Migrating is cleaner (Rule #1) — replace `writeAuthErr` calls with `writeDomainErr` and delete the old one. Confirm Phase 1a handler tests still pass.

Handlers:
```go
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password, Name string }
	if !decodeJSON(w, r, &body) { return }
	if msg := validateRegister(body.Email, body.Password, body.Name); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	res, err := h.svc.Register(r.Context(), body.Email, body.Password, body.Name)
	if err != nil {
		if writeDomainErr(w, r, err) { return }
		shared.WriteError(w, r, "internal", "register failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token": res.AccessToken, "refresh_token": res.RefreshToken,
		"user": map[string]any{
			"id": res.User.ID, "email": res.User.Email, "name": res.User.Name,
			"email_verified": res.User.EmailVerified,
		},
	})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Code string }
	if !decodeJSON(w, r, &body) { return }
	if err := h.svc.VerifyEmail(r.Context(), body.Email, body.Code); err != nil {
		if writeDomainErr(w, r, err) { return }
		shared.WriteError(w, r, "internal", "verify-email failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email string }
	if !decodeJSON(w, r, &body) { return }
	if err := h.svc.ResendVerification(r.Context(), body.Email); err != nil {
		shared.WriteError(w, r, "internal", "resend-verification failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email string }
	if !decodeJSON(w, r, &body) { return }
	if err := h.svc.ForgotPassword(r.Context(), body.Email); err != nil {
		shared.WriteError(w, r, "internal", "forgot-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Code, NewPassword string `json:"new_password"` } // see note
	if !decodeJSON(w, r, &body) { return }
	if err := h.svc.ResetPassword(r.Context(), body.Email, body.Code, body.NewPassword); err != nil {
		if writeDomainErr(w, r, err) { return }
		shared.WriteError(w, r, "internal", "reset-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
}
```
> The inline struct tag on a multi-field line is invalid Go — declare the reset body as a proper struct:
> ```go
> var body struct {
> 	Email       string `json:"email"`
> 	Code        string `json:"code"`
> 	NewPassword string `json:"new_password"`
> }
> ```
> Apply the same explicit-tag form to every body that has a snake_case field. Register/verify/resend/forgot fields (`email`,`password`,`name`,`code`) lower-case-match Go's default JSON decode (case-insensitive), so plain fields work — but prefer explicit tags for clarity.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -race -v` → all PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): lifecycle handlers (register/verify/resend/forgot/reset)"
```

---

## Task A14: Wire Part A into main.go + router

**Files:**
- Modify: `cmd/server/main.go`, `internal/shared/router.go`
- Create: `internal/email/repo_pg.go` (sqlc→email.Querier adapter, if not done in A7)

- [ ] **Step 1: Add router fields + mounts** in `router.go`

Add to the `Handlers` struct (near Login/Refresh):
```go
	Register           http.Handler // POST /auth/register (public)
	VerifyEmail        http.Handler // POST /auth/verify-email (public)
	ResendVerification http.Handler // POST /auth/resend-verification (public)
	ForgotPassword     http.Handler // POST /auth/forgot-password (public)
	ResetPassword      http.Handler // POST /auth/reset-password (public)
```
Mount them next to Login/Refresh (all public, before the auth group):
```go
	if r.Register != nil { mux.Method(http.MethodPost, "/auth/register", r.Register) }
	if r.VerifyEmail != nil { mux.Method(http.MethodPost, "/auth/verify-email", r.VerifyEmail) }
	if r.ResendVerification != nil { mux.Method(http.MethodPost, "/auth/resend-verification", r.ResendVerification) }
	if r.ForgotPassword != nil { mux.Method(http.MethodPost, "/auth/forgot-password", r.ForgotPassword) }
	if r.ResetPassword != nil { mux.Method(http.MethodPost, "/auth/reset-password", r.ResetPassword) }
```

- [ ] **Step 2: Wire composition root** in `main.go` `composeDeps`

After the existing Phase 1a auth wiring (`q := sqlc.New(pool)` already exists):
```go
	plansReader := planspkg.NewReader(q)
	subWriter := subpkg.NewWriter(q)
	emailRepo := emailpkg.NewPgRepo(q)            // sqlc→email.Querier adapter
	emailSvc := emailpkg.NewService(emailRepo, emailpkg.Config{
		EnvAPIKey: cfg.ResendAPIKey, EnvFrom: cfg.EmailFrom,
	})
	socialVer := authpkg.NewHTTPSocialVerifier(cfg.AppleAudiences) // Part B; nil-safe until B9
	authSvc := authpkg.NewService(
		userSvc, subReader, usageCounter, ttlReader,
		cfg.JWTSecret, cfg.JWTRefreshSecret,
		plansReader, subWriter, emailSvc, socialVer,
	)
	authHandler := authpkg.NewHandler(authSvc)
	core.Register = http.HandlerFunc(authHandler.Register)
	core.VerifyEmail = http.HandlerFunc(authHandler.VerifyEmail)
	core.ResendVerification = http.HandlerFunc(authHandler.ResendVerification)
	core.ForgotPassword = http.HandlerFunc(authHandler.ForgotPassword)
	core.ResetPassword = http.HandlerFunc(authHandler.ResetPassword)
```
Add imports: `planspkg "…/internal/plans"`, `emailpkg "…/internal/email"`, `subpkg` already imported. `socialVer` is built here but only used by `Social` (Task B9); `NewHTTPSocialVerifier` is created in Part B — for Part A, pass `nil` and add the `Social` handler in B9. **To keep Part A compiling: pass `nil` for the social verifier now**; `Register` etc. don't touch it.

- [ ] **Step 3: Build + full test** — `go test ./... -race` → PASS. `gofmt -l .` empty. `go vet ./...` clean.

- [ ] **Step 4: Smoke (optional, if local pg+seed)** — boot, `curl -s -XPOST localhost:3001/auth/register -d '{"email":"t@t.co","password":"password8","name":"T"}'` → 201 with tokens; second call → 409 `Email already registered`.

- [ ] **Step 5: Commit**
```bash
git add cmd/server/main.go internal/shared/router.go internal/email/repo_pg.go
git commit -m "feat(go-port): wire lifecycle endpoints into router + composition root"
```

**Part A checkpoint.** 5 endpoints live on the Go router. Proceed to Part B.

---

# Part B — Social

## Task B1: User — FindBySocialId + social-link queries

**Files:**
- Modify: `internal/shared/pg/queries_auth.sql`, `internal/user/repo_pg.go`
- Test: `internal/user/repo_social_test.go` (or extend service_test)

- [ ] **Step 1: Add queries** to `queries_auth.sql`
```sql
-- name: FindUserByGoogleId :one
SELECT id, email, password_hash, name, is_active, role, auth_provider,
       email_verified, lemonsqueezy_customer_id, avatar_url
FROM users WHERE google_id = $1 LIMIT 1;
-- (repeat verbatim for FindUserByFacebookId / FindUserByTiktokId / FindUserByAppleId,
--  swapping the WHERE column)

-- name: LinkSocialGoogle :exec
UPDATE users SET google_id = $2, avatar_url = $3, email_verified = true,
  updated_at = now() WHERE id = $1;
-- (repeat for LinkSocialFacebook / LinkSocialTiktok / LinkSocialApple)
```
Run `sqlc generate`.

- [ ] **Step 2: Write failing test** — extend `service_test.go` fakeRepo + add:
```go
func TestService_FindBySocialId(t *testing.T) {
	r := &fakeRepo{bySocial: map[string]user.User{"google|sid1": {ID: "u1", Email: "g@b.com"}}}
	s := user.NewService(r)
	u, err := s.FindBySocialId(context.Background(), "google", "sid1")
	if err != nil || u.ID != "u1" { t.Fatalf("got %v %v", u, err) }
	if _, err := s.FindBySocialId(context.Background(), "google", "nope"); err != user.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 3: Implement repo** in `repo_pg.go`

Add the generated methods to `pgQuerier`. Implement:
```go
func (r *PgRepo) FindBySocialId(ctx context.Context, provider, socialID string) (User, error) {
	var (
		row sqlc.FindUserByGoogleIdRow // all four *Row types share columns; map per provider
		err error
	)
	switch provider {
	case "google":
		row, err = r.q.FindUserByGoogleId(ctx, ptr(socialID))
	case "facebook":
		var fr sqlc.FindUserByFacebookIdRow
		fr, err = r.q.FindUserByFacebookId(ctx, ptr(socialID))
		row = sqlc.FindUserByGoogleIdRow(fr) // identical shape; or map field-by-field
	case "tiktok":
		var tr sqlc.FindUserByTiktokIdRow
		tr, err = r.q.FindUserByTiktokId(ctx, ptr(socialID))
		row = sqlc.FindUserByGoogleIdRow(tr)
	case "apple":
		var ar sqlc.FindUserByAppleIdRow
		ar, err = r.q.FindUserByAppleId(ctx, ptr(socialID))
		row = sqlc.FindUserByGoogleIdRow(ar)
	default:
		return User{}, ErrNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrNotFound }
	if err != nil { return User{}, err }
	return User{
		ID: uuidStr(row.ID), Email: row.Email, PasswordHash: strOrEmpty(row.PasswordHash),
		Name: row.Name, IsActive: row.IsActive, Role: string(row.Role),
		AuthProvider: string(row.AuthProvider), EmailVerified: row.EmailVerified,
		LemonsqueezyCustomer: strOrEmpty(row.LemonsqueezyCustomerID),
		AvatarURL: strOrEmpty(row.AvatarUrl),
	}, nil
}
```
> The cross-type conversion `sqlc.FindUserByGoogleIdRow(fr)` only compiles if the generated row structs are field-identical. If sqlc names them differently (it shouldn't — same column list), map field-by-field instead. Implementer: check the generated types; prefer an explicit `toUser(...)` helper taking the shared fields.

Implement `linkSocial` (called by `Update` when `p.SocialProvider != ""`):
```go
func (r *PgRepo) linkSocial(ctx context.Context, uid pgtype.UUID, p UserPatch) error {
	avatar := ptr("")
	if p.AvatarURL != nil { avatar = ptr(*p.AvatarURL) }
	switch p.SocialProvider {
	case "google":   return r.q.LinkSocialGoogle(ctx, sqlc.LinkSocialGoogleParams{ID: uid, GoogleID: ptr(p.SocialID), AvatarUrl: avatar})
	case "facebook": return r.q.LinkSocialFacebook(ctx, sqlc.LinkSocialFacebookParams{ID: uid, FacebookID: ptr(p.SocialID), AvatarUrl: avatar})
	case "tiktok":   return r.q.LinkSocialTiktok(ctx, sqlc.LinkSocialTiktokParams{ID: uid, TiktokID: ptr(p.SocialID), AvatarUrl: avatar})
	case "apple":    return r.q.LinkSocialApple(ctx, sqlc.LinkSocialAppleParams{ID: uid, AppleID: ptr(p.SocialID), AvatarUrl: avatar})
	}
	return nil
}
```

- [ ] **Step 4: Run** — `go test ./internal/user/ -race -v` → PASS. `go build ./...`.

- [ ] **Step 5: Commit**
```bash
git add internal/user/ internal/shared/pg/
git commit -m "feat(go-port): user FindBySocialId + social-link queries"
```

---

## Task B2: Social domain — SocialProfile, ports, errors

**Files:**
- Create: `internal/auth/social.go` (types only this task)

- [ ] **Step 1: Define types** (no test — pure declarations, exercised by B3+):
```go
package auth

// InboundProfile is the client-supplied social profile hints
// (name/email/avatar) from POST /auth/social.
type InboundProfile struct {
	Name      string
	Email     string
	AvatarURL string
}

// SocialProfile is the verified identity a provider asserts.
type SocialProfile struct {
	SocialID      string
	Email         string
	Name          string
	AvatarURL     string
	EmailVerified bool
}

// toAuthProvider maps the raw provider string (any case) to the
// canonical enum value, or a *BadRequestError mirroring Node's
// `Unsupported provider: ${provider}` (uses the RAW, pre-lowercase arg).
func toAuthProvider(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "google":   return "google", nil
	case "facebook": return "facebook", nil
	case "tiktok":   return "tiktok", nil
	case "apple":    return "apple", nil
	default:         return "", badRequest("Unsupported provider: " + raw)
	}
}
```
(import `strings`). Add social-specific error consts to `errors.go` if not present (the 401 social messages already exist for `socialLoginMsg`; add the verifier messages used in B3–B6 as plain string literals in those files).

- [ ] **Step 2: Build** — `go build ./internal/auth/` → compiles.

- [ ] **Step 3: Commit**
```bash
git add internal/auth/social.go internal/auth/errors.go
git commit -m "feat(go-port): social domain types + toAuthProvider"
```

---

## Task B3: Google + Facebook verifiers (HTTP-injectable)

**Files:**
- Create: `internal/auth/social_http.go`, `internal/auth/social_http_test.go`

- [ ] **Step 1: Write failing tests** — `social_http_test.go` using `httptest.Server`:
```go
func TestVerifyGoogle_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sub":"g1","email":"g@b.com","name":"G","picture":"p","email_verified":"true"}`))
	}))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token="}
	p, err := v.verifyGoogle(context.Background(), "tok")
	if err != nil { t.Fatal(err) }
	if p.SocialID != "g1" || p.Email != "g@b.com" || !p.EmailVerified { t.Fatalf("%+v", p) }
}

func TestVerifyGoogle_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), googleURL: srv.URL + "?id_token="}
	_, err := v.verifyGoogle(context.Background(), "tok")
	assertAuthErr(t, err, "Invalid Google token")
}

func TestVerifyFacebook_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"f1","name":"F","email":"f@b.com","picture":{"data":{"url":"u"}}}`))
	}))
	defer srv.Close()
	v := &httpVerifier{http: srv.Client(), facebookURL: srv.URL + "?access_token="}
	p, err := v.verifyFacebook(context.Background(), "tok")
	if err != nil { t.Fatal(err) }
	if p.SocialID != "f1" || p.AvatarURL != "u" || !p.EmailVerified { t.Fatalf("%+v", p) }
}
```

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement** `social_http.go`:
```go
package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// httpVerifier verifies Google/Facebook/Apple tokens via real HTTP. URLs
// are fields so tests inject httptest servers. Apple JWKS lives in
// social_apple.go (Task B5).
type httpVerifier struct {
	http        *http.Client
	googleURL   string // "...tokeninfo?id_token="
	facebookURL string // "...me?fields=...&access_token="
	appleKeyURL string
	appleAuds   []string
}

func (v *httpVerifier) verifyGoogle(ctx context.Context, idToken string) (SocialProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, v.googleURL+idToken, nil)
	resp, err := v.http.Do(req)
	if err != nil { return SocialProfile{}, unauthorized("Invalid Google token") }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 { return SocialProfile{}, unauthorized("Invalid Google token") }
	var d struct {
		Sub, Email, Name, Picture string
		EmailVerified any `json:"email_verified"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)
	return SocialProfile{
		SocialID: d.Sub, Email: d.Email, Name: d.Name, AvatarURL: d.Picture,
		EmailVerified: truthy(d.EmailVerified),
	}, nil
}

func (v *httpVerifier) verifyFacebook(ctx context.Context, accessToken string) (SocialProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, v.facebookURL+accessToken, nil)
	resp, err := v.http.Do(req)
	if err != nil { return SocialProfile{}, unauthorized("Invalid Facebook token") }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 { return SocialProfile{}, unauthorized("Invalid Facebook token") }
	var d struct {
		ID, Name, Email string
		Picture struct{ Data struct{ URL string } } `json:"picture"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)
	return SocialProfile{
		SocialID: d.ID, Email: d.Email, Name: d.Name, AvatarURL: d.Picture.Data.URL,
		EmailVerified: d.Email != "",
	}, nil
}

// truthy matches Node's `x === true || x === 'true'` (tokeninfo returns
// the string "true"; some payloads a bool).
func truthy(v any) bool {
	switch t := v.(type) {
	case bool:   return t
	case string: return t == "true"
	}
	return false
}
```

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run "Google|Facebook" -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/social_http.go internal/auth/social_http_test.go
git commit -m "feat(go-port): Google + Facebook social verifiers"
```

---

## Task B4: TikTok + Verify dispatch

**Files:**
- Modify: `internal/auth/social_http.go`
- Test: `internal/auth/social_http_test.go`

- [ ] **Step 1: Write failing tests**:
```go
func TestVerifyTikTok_TrustsProfile(t *testing.T) {
	v := &httpVerifier{}
	p, err := v.Verify(context.Background(), "tiktok", "openid123", InboundProfile{Email: "t@b.com", Name: "T"})
	if err != nil { t.Fatal(err) }
	if p.SocialID != "openid123" || p.Email != "t@b.com" || p.EmailVerified { t.Fatalf("%+v", p) }
}
```

- [ ] **Step 2: Run, verify fail** — FAIL (no `Verify`).

- [ ] **Step 3: Implement dispatch** in `social_http.go`:
```go
// Verify implements SocialVerifier. provider is the canonical enum value.
func (v *httpVerifier) Verify(ctx context.Context, provider, idToken string, profile InboundProfile) (SocialProfile, error) {
	switch provider {
	case "google":
		return v.verifyGoogle(ctx, idToken)
	case "facebook":
		return v.verifyFacebook(ctx, idToken)
	case "apple":
		return v.verifyApple(ctx, idToken, profile) // Task B5
	case "tiktok":
		return SocialProfile{
			SocialID: idToken, Email: profile.Email, Name: profile.Name,
			AvatarURL: profile.AvatarURL, EmailVerified: false,
		}, nil
	}
	return SocialProfile{}, badRequest("Unsupported provider")
}
```
> Until B5 lands, stub `verifyApple` to `return SocialProfile{}, unauthorized("Invalid Apple token")` so this compiles; B5 replaces it.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run TikTok -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/social_http.go internal/auth/social_http_test.go
git commit -m "feat(go-port): TikTok verifier + Verify dispatch"
```

---

## Task B5: Apple verifier — JWKS cache + RS256

**Files:**
- Create: `internal/auth/social_apple.go`, `internal/auth/social_apple_test.go`

- [ ] **Step 1: Write failing test** — generate an RSA key in-test, serve a JWKS, sign a token, verify:
```go
func TestVerifyApple_ValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "testkid"
	jwks := buildJWKS(t, key, kid) // helper: marshal n/e base64url + kid
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jwks))
	}))
	defer srv.Close()
	tok := signApple(t, key, kid, "applesub", "a@b.com", true, "com.draftright.app.v2")
	v := &httpVerifier{http: srv.Client(), appleKeyURL: srv.URL, appleAuds: []string{"com.draftright.app.v2"}}
	p, err := v.verifyApple(context.Background(), tok, InboundProfile{})
	if err != nil { t.Fatal(err) }
	if p.SocialID != "applesub" || p.Email != "a@b.com" || !p.EmailVerified { t.Fatalf("%+v", p) }
}

func TestVerifyApple_NoKid(t *testing.T) {
	v := &httpVerifier{}
	_, err := v.verifyApple(context.Background(), "not.a.jwt", InboundProfile{})
	assertAuthErr(t, err, "Invalid Apple token (no kid)")
}

func TestVerifyApple_WrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "k"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(buildJWKS(t, key, kid)))
	}))
	defer srv.Close()
	tok := signApple(t, key, kid, "s", "a@b.com", true, "com.evil.app")
	v := &httpVerifier{http: srv.Client(), appleKeyURL: srv.URL, appleAuds: []string{"com.draftright.app.v2"}}
	_, err := v.verifyApple(context.Background(), tok, InboundProfile{})
	assertAuthErr(t, err, "Invalid Apple token")
}
```
Write `buildJWKS`, `signApple` test helpers (use `golang-jwt/jwt/v5` `jwt.NewWithClaims(jwt.SigningMethodRS256, ...)`, set header `kid`; JWKS = `{"keys":[{"kty":"RSA","kid":...,"n":base64url(N),"e":base64url(E)}]}`).

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement `social_apple.go`**
```go
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// appleJWKSCache is a process-wide 24h cache mirroring Node's static
// _appleJwksCache. Guarded by mu.
type appleJWKSCache struct {
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey // kid → key
	expiresAt time.Time
}

var appleCache = &appleJWKSCache{}

const appleDefaultAuds = "com.draftright.draftrightMobile.v2,com.draftright.app.v2"

func (v *httpVerifier) verifyApple(ctx context.Context, idToken string, profile InboundProfile) (SocialProfile, error) {
	// Decode header for kid WITHOUT verifying (need kid to fetch the key).
	kid, err := appleKidOf(idToken)
	if err != nil || kid == "" {
		return SocialProfile{}, unauthorized("Invalid Apple token (no kid)")
	}
	pub, err := v.applePublicKey(ctx, kid)
	if err != nil {
		return SocialProfile{}, err // already an *AuthError with the right message
	}
	auds := v.appleAuds
	if len(auds) == 0 { auds = splitAuds(appleDefaultAuds) }

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://appleid.apple.com"),
		// clockTolerance stays ZERO (parity invariant) — do NOT add jwt.WithLeeway.
	)
	claims := jwt.MapClaims{}
	_, err = parser.ParseWithClaims(idToken, claims, func(*jwt.Token) (any, error) { return pub, nil })
	if err != nil {
		return SocialProfile{}, unauthorized("Invalid Apple token")
	}
	// audience: accept if ANY configured aud matches (Node passes the list).
	if !audienceMatches(claims, auds) {
		return SocialProfile{}, unauthorized("Invalid Apple token")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return SocialProfile{}, unauthorized("Apple token missing sub claim")
	}
	email, _ := claims["email"].(string)
	if email == "" { email = profile.Email }
	verified := email != "" && truthy(claims["email_verified"]) && hasOwnEmail(claims)
	return SocialProfile{
		SocialID: sub, Email: email, Name: profile.Name, AvatarURL: profile.AvatarURL,
		EmailVerified: verified,
	}, nil
}

// hasOwnEmail mirrors Node `!!payload.email` — only the TOKEN's email
// counts toward verified, not the profile fallback.
func hasOwnEmail(c jwt.MapClaims) bool { e, _ := c["email"].(string); return e != "" }

func (v *httpVerifier) applePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	appleCache.mu.Lock()
	defer appleCache.mu.Unlock()
	if appleCache.keys == nil || appleCache.expiresAt.Before(time.Now()) {
		if err := v.fetchAppleKeys(ctx); err != nil {
			return nil, unauthorized("Could not fetch Apple signing keys")
		}
	}
	pub, ok := appleCache.keys[kid]
	if !ok {
		appleCache.keys = nil // bust cache (matches Node) — no retry, just throw
		return nil, unauthorized("Apple key not found for kid=" + kid)
	}
	return pub, nil
}

func (v *httpVerifier) fetchAppleKeys(ctx context.Context) error {
	url := v.appleKeyURL
	if url == "" { url = "https://appleid.apple.com/auth/keys" }
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := v.http.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 { return errors.New("apple keys http error") }
	var doc struct{ Keys []struct{ Kid, N, E string } `json:"keys"` }
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil { return err }
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		nb, err := base64.RawURLEncoding.DecodeString(k.N); if err != nil { continue }
		eb, err := base64.RawURLEncoding.DecodeString(k.E); if err != nil { continue }
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nb),
			E: int(new(big.Int).SetBytes(eb).Int64()),
		}
	}
	appleCache.keys = keys
	appleCache.expiresAt = time.Now().Add(24 * time.Hour)
	return nil
}
```
Add helpers: `appleKidOf(token)` (split JWT, base64url-decode header, json → `{kid}`), `splitAuds(csv)` (comma-split+trim+drop empties), `audienceMatches(claims, auds)` (read `aud` which may be string or []any; true if intersects `auds`). Use `v.http` defaulting to `http.DefaultClient` when nil.

> **Test isolation:** `appleCache` is package-global; tests that populate it must reset it (`appleCache.keys = nil`) in setup, or the "no kid" / cache-bust tests interfere. Add `t.Cleanup(func(){ appleCache.keys = nil })` to each Apple test.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run Apple -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/social_apple.go internal/auth/social_apple_test.go
git commit -m "feat(go-port): Apple verifier — JWKS 24h cache + RS256, clockTolerance 0"
```

---

## Task B6: HTTP verifier constructor

**Files:**
- Modify: `internal/auth/social_http.go`

- [ ] **Step 1: Add the production constructor**
```go
// NewHTTPSocialVerifier builds the real verifier with Apple audiences
// from APPLE_AUDIENCES (empty → the two-app default). Wired in main.go.
func NewHTTPSocialVerifier(appleAudiencesCSV string) *httpVerifier {
	auds := splitAuds(appleAudiencesCSV)
	if len(auds) == 0 { auds = splitAuds(appleDefaultAuds) }
	return &httpVerifier{
		http:        http.DefaultClient,
		googleURL:   "https://oauth2.googleapis.com/tokeninfo?id_token=",
		facebookURL: "https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=",
		appleKeyURL: "https://appleid.apple.com/auth/keys",
		appleAuds:   auds,
	}
}
```

- [ ] **Step 2: Build + vet** — `go build ./... && go vet ./...` → clean.

- [ ] **Step 3: Commit**
```bash
git add internal/auth/social_http.go
git commit -m "feat(go-port): NewHTTPSocialVerifier constructor"
```

---

## Task B7: Auth — SocialLogin usecase

**Files:**
- Modify: `internal/auth/usecase.go`
- Test: `internal/auth/social_login_test.go`

- [ ] **Step 1: Write failing tests** with a **fake `SocialVerifier`**:
```go
type fakeSocial struct{ profile SocialProfile; err error }
func (f fakeSocial) Verify(_ context.Context, _ , _ string, _ InboundProfile) (SocialProfile, error) {
	return f.profile, f.err
}

func TestSocial_CreateNewUser(t *testing.T) {
	users := newStubUsers() // FindBySocialId + ByEmail miss
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", Name: "G", EmailVerified: true}})
	res, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	if err != nil { t.Fatal(err) }
	if res.User.Email != "g@b.com" { t.Fatalf("%+v", res.User) }
	if !users.created { t.Fatal("should create user") }
}

func TestSocial_UnsupportedProvider(t *testing.T) {
	svc := newSvcSocial(t, newStubUsers(), fakeSocial{})
	_, err := svc.SocialLogin(context.Background(), "myspace", "tok", InboundProfile{})
	assertBadReq(t, err, "Unsupported provider: myspace")
}

func TestSocial_NoEmail(t *testing.T) {
	svc := newSvcSocial(t, newStubUsers(), fakeSocial{profile: SocialProfile{SocialID: "g1", Email: ""}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertBadReq(t, err, "Email is required for social login")
}

func TestSocial_TakeoverGuard(t *testing.T) {
	users := newStubUsers()
	users.byEmail["g@b.com"] = user.User{ID: "u1", Email: "g@b.com", IsActive: true} // exists
	// FindBySocialId misses → falls to byEmail hit; profile NOT verified → block
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: false}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertAuthErr(t, err, "This email is registered. Sign in with your password to link this account.")
}

func TestSocial_LinkVerified(t *testing.T) {
	users := newStubUsers()
	users.byEmail["g@b.com"] = user.User{ID: "u1", Email: "g@b.com", Name: "G", IsActive: true}
	users.byID["u1"] = users.byEmail["g@b.com"]
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: true}})
	res, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	if err != nil { t.Fatal(err) }
	if res.User.ID != "u1" { t.Fatalf("%+v", res.User) }
	if !users.updateCalled { t.Fatal("should link") }
}

func TestSocial_ExistingBySocialId_Disabled(t *testing.T) {
	users := newStubUsers()
	users.bySocial["google|g1"] = user.User{ID: "u1", Email: "g@b.com", IsActive: false}
	svc := newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", EmailVerified: true}})
	_, err := svc.SocialLogin(context.Background(), "google", "tok", InboundProfile{})
	assertAuthErr(t, err, "Account disabled")
}
```
Add `newSvcSocial(t, users, verifier)` — like `newSvc` but injects the fake `SocialVerifier`. Extend stubUsers with `bySocial map[string]user.User` (key `provider|socialID`), `FindBySocialId` reads it.

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement `SocialLogin`** in `usecase.go`:
```go
// SocialResult is the social-login return (trimmed user, no email_verified).
type SocialResult struct {
	Tokens
	User UserPayload
}

func (s *Service) SocialLogin(ctx context.Context, provider, idToken string, profile InboundProfile) (SocialResult, error) {
	prov, err := toAuthProvider(provider)
	if err != nil { return SocialResult{}, err } // *BadRequestError "Unsupported provider: ..."
	sp, err := s.social.Verify(ctx, prov, idToken, profile)
	if err != nil { return SocialResult{}, err } // verifier returns *AuthError (401)
	if sp.Email == "" {
		return SocialResult{}, badRequest(msgEmailRequired)
	}
	u, err := s.users.FindBySocialId(ctx, prov, sp.SocialID)
	found := err == nil
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return SocialResult{}, err
	}
	verified := sp.EmailVerified
	if !found {
		byEmail, e := s.users.ByEmail(ctx, sp.Email)
		if e == nil {
			// link path
			if !verified {
				return SocialResult{}, unauthorized(msgEmailRegisteredSocial)
			}
			avatar := sp.AvatarURL
			if avatar == "" { avatar = byEmail.AvatarURL }
			on := true
			if err := s.users.Update(ctx, byEmail.ID, user.UserPatch{
				SocialProvider: prov, SocialID: sp.SocialID,
				AvatarURL: &avatar, EmailVerified: &on,
			}); err != nil { return SocialResult{}, err }
			u, err = s.users.ByID(ctx, byEmail.ID)
			if err != nil { return SocialResult{}, err }
		} else if errors.Is(e, user.ErrNotFound) {
			// create path
			name := sp.Name
			if name == "" { name = emailLocalPart(sp.Email) }
			u, err = s.users.Create(ctx, user.NewUser{
				Email: sp.Email, Name: name, AuthProvider: prov,
				SocialProvider: prov, SocialID: sp.SocialID,
				AvatarURL: sp.AvatarURL, EmailVerified: verified,
			})
			if err != nil { return SocialResult{}, err }
			free, ferr := s.plans.FindFreePlan(ctx)
			if ferr != nil { return SocialResult{}, ferr }
			if serr := s.subWriter.CreateFree(ctx, u.ID, free.ID); serr != nil {
				return SocialResult{}, serr
			}
		} else {
			return SocialResult{}, e
		}
	}
	if !u.IsActive {
		return SocialResult{}, unauthorized(msgAccountDisabled)
	}
	toks, err := s.generateTokens(ctx, u)
	if err != nil { return SocialResult{}, err }
	return SocialResult{Tokens: toks, User: UserPayload{ID: u.ID, Email: u.Email, Name: u.Name}}, nil
}

func emailLocalPart(e string) string {
	if i := strings.IndexByte(e, '@'); i >= 0 { return e[:i] }
	return e
}
```
> Note the link path's `Update` sets `EmailVerified:true` AND the social column — the repo's `linkSocial` query (`LinkSocial*`) already forces `email_verified=true`, so the `UserPatch.EmailVerified` is informational; the SQL is the source of truth. Routing in `repo.Update`: `SocialProvider != ""` wins the switch → `linkSocial`.

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run Social -race -v` → PASS. Full `go test ./internal/auth/ -race`.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): auth.SocialLogin usecase (link/create/takeover guard)"
```

---

## Task B8: Social handler

**Files:**
- Modify: `internal/auth/handler.go`
- Test: `internal/auth/handler_social_test.go`

- [ ] **Step 1: Write failing test**:
```go
func TestHandler_Social_201(t *testing.T) {
	users := newStubUsers()
	h := NewHandler(newSvcSocial(t, users, fakeSocial{profile: SocialProfile{SocialID: "g1", Email: "g@b.com", Name: "G", EmailVerified: true}}))
	req := httptest.NewRequest(http.MethodPost, "/auth/social",
		strings.NewReader(`{"provider":"google","id_token":"tok"}`))
	rr := httptest.NewRecorder()
	h.Social(rr, req)
	if rr.Code != http.StatusCreated { t.Fatalf("status %d body %s", rr.Code, rr.Body) }
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if _, ok := body["access_token"]; !ok { t.Fatal("no access_token") }
}

func TestHandler_Social_UnsupportedProvider_400(t *testing.T) {
	h := NewHandler(newSvcSocial(t, newStubUsers(), fakeSocial{}))
	req := httptest.NewRequest(http.MethodPost, "/auth/social", strings.NewReader(`{"provider":"x","id_token":"t"}`))
	rr := httptest.NewRecorder()
	h.Social(rr, req)
	if rr.Code != http.StatusBadRequest { t.Fatalf("status %d", rr.Code) }
}
```

- [ ] **Step 2: Run, verify fail** — FAIL.

- [ ] **Step 3: Implement** in `handler.go`:
```go
func (h *Handler) Social(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider  string `json:"provider"`
		IDToken   string `json:"id_token"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if !decodeJSON(w, r, &body) { return }
	res, err := h.svc.SocialLogin(r.Context(), body.Provider, body.IDToken, InboundProfile{
		Name: body.Name, Email: body.Email, AvatarURL: body.AvatarURL,
	})
	if err != nil {
		if writeDomainErr(w, r, err) { return }
		shared.WriteError(w, r, "internal", "social login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token": res.AccessToken, "refresh_token": res.RefreshToken,
		"user": map[string]any{"id": res.User.ID, "email": res.User.Email, "name": res.User.Name},
	})
}
```

- [ ] **Step 4: Run** — `go test ./internal/auth/ -run "Handler_Social" -race -v` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/auth/
git commit -m "feat(go-port): POST /auth/social handler"
```

---

## Task B9: Wire social into main.go + router

**Files:**
- Modify: `cmd/server/main.go`, `internal/shared/router.go`

- [ ] **Step 1: Router field + mount**

Add to `Handlers`:
```go
	Social http.Handler // POST /auth/social (public)
```
Mount (public, by Login/Refresh):
```go
	if r.Social != nil { mux.Method(http.MethodPost, "/auth/social", r.Social) }
```

- [ ] **Step 2: main.go — real verifier + handler field**

Replace the Part-A `nil` social verifier with the real one (it's already referenced as `socialVer` in Task A14 step 2; ensure it's `authpkg.NewHTTPSocialVerifier(cfg.AppleAudiences)`), and add:
```go
	core.Social = http.HandlerFunc(authHandler.Social)
```

- [ ] **Step 3: Full build + test** — `go test ./... -race` → PASS. `gofmt -l .` empty. `go vet ./...` clean.

- [ ] **Step 4: Commit**
```bash
git add cmd/server/main.go internal/shared/router.go
git commit -m "feat(go-port): wire POST /auth/social into router + composition root"
```

---

## Final: whole-implementation review + finish

- [ ] **Step 1: Full suite** — `go test ./... -race`, `gofmt -l .` (empty), `go vet ./...` (clean), `sqlc generate` (no diff).
- [ ] **Step 2: Dispatch a final code-reviewer** over the whole Phase 1b diff (`git diff develop...HEAD`) — focus: parity (error strings, status codes, check-order vs `auth.service.ts`), security (no SQL interpolation of provider, clockTolerance 0, plaintext-code parity intentional), Rule #1 (no duplicated error envelopes, shared helpers reused).
- [ ] **Step 3:** Address any findings (fix subagent per finding, re-review).
- [ ] **Step 4:** Use **superpowers:finishing-a-development-branch** → merge `feature/go-backend-phase1b-20260614` → `develop` `--no-ff`. Ships nothing to prod (no Caddy flip; live shadow gate operator-pending).

---

## Self-review notes (plan author)

- **Spec coverage:** all 6 endpoints (A9–A11, A13, B7–B8), email module (A6–A7), schema patch (A0), config (A1), user writes (A2–A3, B1), plans (A4), sub writer (A5), validation (A12), social verifiers (B3–B6), wiring (A14, B9). ✓
- **Type consistency:** `UserPatch`/`NewUser`/`AuthState`/`NullableString`/`NullableTime` defined in A2, used A3/A10/A11/B7. `SocialProfile`/`InboundProfile`/`SocialVerifier` defined A8/B2, used B3–B8. `RegisterResult`/`SocialResult`/`UserPayload` consistent across usecase+handler. ✓
- **Known parity risks (gate-pinned, flagged in spec §12):** class-validator exotic-input ordering (A12 covers common cases); password length byte-vs-UTF16 (A11 note); email HTML not gate-checked. ✓
- **sqlc generated names:** several steps say "confirm against generated output" — implementer must read the `sqlc generate` result for exact param/field/enum-type names (`AvatarUrl`, `UsersAuthProviderEnum`, etc.) and the nullable→pointer mapping. This is the one place the plan can't be 100% literal (codegen owns the names). ✓
