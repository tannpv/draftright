# Go Backend Phase 2 — Entitlements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the NestJS `plans`, `subscriptions`, and `extension-tokens` modules to Go byte-identically, and make `/v1/rewrite` accept `dr_ext_*` bearer tokens.

**Architecture:** Three flat clean-arch feature modules under `internal/` (`plans` extended, `subscription` extended, `exttoken` new). Each: `domain.go` (types + pure logic, zero deps) → `usecase.go` (Service + consumer-side ports) → `repo_pg.go` (sqlc-backed). HTTP edges in `handler.go`, wired in `cmd/server/main.go`, mounted in `shared/router.go`. Reuse existing `usage.Counter`. A `time.Timer`-based daily scheduler runs the subscription expiry cron. A dual-auth shim on the rewrite route resolves `dr_ext_` before JWT.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + sqlc, golang-jwt/jwt/v5 (HS256), crypto/rand + crypto/sha256, slog. `go test ./... -race` is the source of truth.

**Spec:** `docs/superpowers/specs/2026-06-14-go-backend-phase2-entitlements-design.md`

---

## Conventions every task follows

- TDD: failing test → run red → minimal impl → run green → commit.
- Commit prefix `feat(go-port):`. Co-author trailer:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Never commit to develop/main — we are on `feature/go-backend-phase2-20260611`.
- After any `*.sql` query change: run `sqlc generate` from `backend-rewrite-go/`,
  commit the regenerated `internal/shared/pg/sqlc/*` too.
- `gofmt -w` touched files before commit; `go vet ./...` clean.
- All `cd` into `backend-rewrite-go/` for go/sqlc commands.

### Shared ISO-millis formatter (used by plans + subscription)

Node serializes `timestamp` columns via `Date.toISOString()` → millisecond
precision, UTC `Z` (e.g. `2026-06-14T09:30:00.000Z`). Go's existing
`auth/usecase.go` uses `time.RFC3339` (no ms) for the account view — that is an
inherited Phase 1a concern, gate-pinned, DO NOT change it. New Phase 2
endpoints use the ms formatter below. It lives in `shared` so both new modules
share one copy.

---

### Task 1: Shared ISO-millis formatter

**Files:**
- Create: `backend-rewrite-go/internal/shared/isotime.go`
- Test: `backend-rewrite-go/internal/shared/isotime_test.go`

- [ ] **Step 1: Write the failing test**

```go
package shared

import (
	"testing"
	"time"
)

func TestISOMillis(t *testing.T) {
	tm := time.Date(2026, 6, 14, 9, 30, 0, 0, time.UTC)
	if got := ISOMillis(tm); got != "2026-06-14T09:30:00.000Z" {
		t.Fatalf("got %q", got)
	}
	// non-UTC input is converted to UTC Z
	loc := time.FixedZone("x", 3600)
	tm2 := time.Date(2026, 6, 14, 10, 30, 0, 0, loc)
	if got := ISOMillis(tm2); got != "2026-06-14T09:30:00.000Z" {
		t.Fatalf("tz convert got %q", got)
	}
	// sub-ms truncates to 3 decimals
	tm3 := time.Date(2026, 6, 14, 9, 30, 0, 123_456_789, time.UTC)
	if got := ISOMillis(tm3); got != "2026-06-14T09:30:00.123Z" {
		t.Fatalf("ms got %q", got)
	}
}
```

- [ ] **Step 2: Run red** — `cd backend-rewrite-go && go test ./internal/shared/ -run TestISOMillis` → FAIL (undefined ISOMillis).

- [ ] **Step 3: Implement**

```go
package shared

import "time"

// ISOMillis formats t as Node's Date.toISOString() does: UTC, exactly
// three fractional-second digits, trailing "Z". Matches the JSON that
// TypeORM emits for timestamp columns (created_at/updated_at/expires_at)
// on the Phase 2 entitlement endpoints.
func ISOMillis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}
```

- [ ] **Step 4: Run green** — same command → PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/shared/isotime.go internal/shared/isotime_test.go
git add internal/shared/isotime.go internal/shared/isotime_test.go
git commit -m "feat(go-port): shared ISOMillis formatter (toISOString parity)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: plans — query + repo projection

**Files:**
- Modify: `backend-rewrite-go/internal/shared/pg/queries_auth.sql` (append query)
- Modify: `backend-rewrite-go/internal/plans/reader.go` (add ListActivePlans path)
- Create: `backend-rewrite-go/internal/plans/domain.go`
- Test: `backend-rewrite-go/internal/plans/domain_test.go`

**Context:** `GET /plans` returns a JSON ARRAY of raw plan entities, snake_case,
in entity-declaration order (NOT DB column order). Order:
`id, name, daily_limit, price_cents, currency, stripe_price_id, trial_days,
billing_period, is_active, created_at, updated_at`. `price_cents` raw int (NOT
/100). `currency` char(3) and `stripe_price_id` nullable → `*string`.
`is_active` always true on this route. Query: `WHERE is_active=true ORDER BY
price_cents ASC`. Empty result must serialise `[]` not `null`.

- [ ] **Step 1: Write the failing test** (`domain_test.go`)

```go
package plans

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPlanEntity_JSONShapeAndOrder(t *testing.T) {
	cur := "USD"
	sp := "price_123"
	p := PlanEntity{
		ID: "11111111-1111-1111-1111-111111111111", Name: "Pro",
		DailyLimit: 100, PriceCents: 499, Currency: &cur, StripePriceID: &sp,
		TrialDays: 30, BillingPeriod: "monthly", IsActive: true,
		CreatedAt: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC),
	}
	b, _ := json.Marshal(p)
	want := `{"id":"11111111-1111-1111-1111-111111111111","name":"Pro","daily_limit":100,"price_cents":499,"currency":"USD","stripe_price_id":"price_123","trial_days":30,"billing_period":"monthly","is_active":true,"created_at":"2026-06-14T09:00:00.000Z","updated_at":"2026-06-14T09:00:00.000Z"}`
	if string(b) != want {
		t.Fatalf("\n got %s\nwant %s", b, want)
	}
}

func TestPlanEntity_NullCurrency(t *testing.T) {
	p := PlanEntity{ID: "x", BillingPeriod: "none"}
	b, _ := json.Marshal(p)
	// currency + stripe_price_id null when pointers nil
	if !contains(string(b), `"currency":null`) || !contains(string(b), `"stripe_price_id":null`) {
		t.Fatalf("got %s", b)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (func() bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub { return true }
	}
	return false
})() }
```

- [ ] **Step 2: Run red** — `go test ./internal/plans/ -run TestPlanEntity` → FAIL (undefined PlanEntity).

- [ ] **Step 3a: Create `domain.go`** with the entity + custom marshaller for ms timestamps. Because two fields need ms formatting, use a marshalling shadow struct:

```go
package plans

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// PlanEntity is the raw plan row as GET /plans serialises it: snake_case,
// in entity-declaration order, price_cents raw (not /100), nullable
// currency/stripe_price_id. MarshalJSON pins field order + ms timestamps.
type PlanEntity struct {
	ID            string
	Name          string
	DailyLimit    int
	PriceCents    int
	Currency      *string
	StripePriceID *string
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p PlanEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		DailyLimit    int     `json:"daily_limit"`
		PriceCents    int     `json:"price_cents"`
		Currency      *string `json:"currency"`
		StripePriceID *string `json:"stripe_price_id"`
		TrialDays     int     `json:"trial_days"`
		BillingPeriod string  `json:"billing_period"`
		IsActive      bool    `json:"is_active"`
		CreatedAt     string  `json:"created_at"`
		UpdatedAt     string  `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, DailyLimit: p.DailyLimit, PriceCents: p.PriceCents,
		Currency: p.Currency, StripePriceID: p.StripePriceID, TrialDays: p.TrialDays,
		BillingPeriod: p.BillingPeriod, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}
```

- [ ] **Step 3b: Append the query** to `internal/shared/pg/queries_auth.sql`:

```sql
-- name: ListActivePlans :many
-- Mirrors plansService.findAll(): every active plan, cheapest first.
-- No tiebreaker beyond price_cents (matches TypeORM order:{price_cents:'ASC'}).
SELECT id, name, daily_limit, price_cents, currency, stripe_price_id,
       trial_days, billing_period, is_active, created_at, updated_at
FROM plans
WHERE is_active = true
ORDER BY price_cents ASC;
```

- [ ] **Step 3c: Regenerate sqlc** — `cd backend-rewrite-go && sqlc generate`. Confirm `ListActivePlans` appears in `internal/shared/pg/sqlc/querier.go`.

- [ ] **Step 4: Run green** — `go test ./internal/plans/ -run TestPlanEntity` → PASS. `go build ./...` clean.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/plans/
git add internal/plans/domain.go internal/plans/domain_test.go internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc/
git commit -m "feat(go-port): plans entity JSON shape + ListActivePlans query

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: plans — Service.ListActive + repo wiring

**Files:**
- Modify: `backend-rewrite-go/internal/plans/reader.go` (add Querier method + mapping)
- Create: `backend-rewrite-go/internal/plans/usecase.go`
- Test: `backend-rewrite-go/internal/plans/usecase_test.go`

**Context:** Reuse the existing `Reader` (it already wraps a sqlc `Querier`).
Add a `ListActive` that maps `sqlc.ListActivePlansRow` → `[]PlanEntity`. Empty
result returns a non-nil empty slice so the handler emits `[]`.

- [ ] **Step 1: Write the failing test**

```go
package plans

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeListQ struct{ rows []sqlc.ListActivePlansRow }

func (f fakeListQ) FindFreePlan(ctx context.Context) (sqlc.FindFreePlanRow, error) {
	return sqlc.FindFreePlanRow{}, nil
}
func (f fakeListQ) ListActivePlans(ctx context.Context) ([]sqlc.ListActivePlansRow, error) {
	return f.rows, nil
}

func TestListActive_MapsAndEmpty(t *testing.T) {
	// empty → non-nil []
	r := NewReader(fakeListQ{})
	got, err := r.ListActive(context.Background())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty: got=%v err=%v", got, err)
	}
	// one row maps fields, raw price_cents
	cur := pgtype.Text{String: "USD", Valid: true}
	row := sqlc.ListActivePlansRow{
		ID: mustUUID(t, "11111111-1111-1111-1111-111111111111"), Name: "Pro",
		DailyLimit: 100, PriceCents: 499, Currency: &cur, TrialDays: 30,
		BillingPeriod: sqlc.PlansBillingPeriodEnumMonthly, IsActive: true,
		CreatedAt: pgtype.Timestamp{Time: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC), Valid: true},
		UpdatedAt: pgtype.Timestamp{Time: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC), Valid: true},
	}
	r2 := NewReader(fakeListQ{rows: []sqlc.ListActivePlansRow{row}})
	got2, _ := r2.ListActive(context.Background())
	if len(got2) != 1 || got2[0].PriceCents != 499 || got2[0].Currency == nil || *got2[0].Currency != "USD" {
		t.Fatalf("map: %+v", got2)
	}
}
```

> NOTE: the implementer MUST inspect the real generated `sqlc.ListActivePlansRow`
> field types (after Task 2's `sqlc generate`) — `Currency`/`StripePriceID` may
> be `*pgtype.Text` or `pgtype.Text` depending on `emit_pointers_for_null_types`.
> Add a `mustUUID(t, s)` helper that builds a `pgtype.UUID` via `.Scan(s)`.
> Adjust the test literal types to match the generated struct exactly.

- [ ] **Step 2: Run red** — `go test ./internal/plans/ -run TestListActive` → FAIL.

- [ ] **Step 3a: Extend the `Querier` interface** in `reader.go`:

```go
// Querier is the sqlc subset plans needs.
type Querier interface {
	FindFreePlan(ctx context.Context) (sqlc.FindFreePlanRow, error)
	ListActivePlans(ctx context.Context) ([]sqlc.ListActivePlansRow, error)
}
```

- [ ] **Step 3b: Add `ListActive`** to `reader.go` (map row→PlanEntity; convert
  `pgtype.Text`→`*string` only when `.Valid`; `pgtype.Timestamp.Time` for dates;
  raw `int(row.PriceCents)`). Allocate `out := make([]PlanEntity, 0, len(rows))`
  so empty → `[]`. Match the generated nullable types from Task 2.

```go
// ListActive returns every active plan, cheapest first, as raw entities
// for GET /plans. Empty → non-nil empty slice (serialises []).
func (r *Reader) ListActive(ctx context.Context) ([]PlanEntity, error) {
	rows, err := r.q.ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanEntity, 0, len(rows))
	for _, row := range rows {
		p := PlanEntity{
			ID: row.ID.String(), Name: row.Name, DailyLimit: int(row.DailyLimit),
			PriceCents: int(row.PriceCents), TrialDays: int(row.TrialDays),
			BillingPeriod: string(row.BillingPeriod), IsActive: row.IsActive,
			CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		}
		if c := textPtr(row.Currency); c != nil {
			p.Currency = c
		}
		if s := textPtr(row.StripePriceID); s != nil {
			p.StripePriceID = s
		}
		out = append(out, p)
	}
	return out, nil
}
```

> The implementer writes `textPtr` to match the generated type (a helper that
> returns `*string` from the nullable column, or `nil` when absent). If the
> generated type is already `*pgtype.Text`, nil-check then `.String`.

- [ ] **Step 3c: Create `usecase.go`** — a thin Service so the handler depends on
  a small port, not the concrete Reader:

```go
package plans

import "context"

// Lister is the consumer-side port the handler needs.
type Lister interface {
	ListActive(ctx context.Context) ([]PlanEntity, error)
}

// Service is the plans use case. Trivial today (one passthrough); it exists
// so the HTTP edge depends on a port, matching the module recipe.
type Service struct{ r Lister }

// NewService wires the lister.
func NewService(r Lister) *Service { return &Service{r: r} }

// ListActive returns active plans for GET /plans.
func (s *Service) ListActive(ctx context.Context) ([]PlanEntity, error) {
	return s.r.ListActive(ctx)
}
```

- [ ] **Step 4: Run green** — `go test ./internal/plans/ -race` → PASS; `go build ./...` clean.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/plans/
git add internal/plans/
git commit -m "feat(go-port): plans ListActive reader + service

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: plans — handler + router + main wiring

**Files:**
- Create: `backend-rewrite-go/internal/plans/handler.go`
- Test: `backend-rewrite-go/internal/plans/handler_test.go`
- Modify: `backend-rewrite-go/internal/shared/router.go` (add `Plans` field + mount)
- Modify: `backend-rewrite-go/cmd/server/main.go` (build service + assign route)

**Context:** Public route, no auth. 200. Body = JSON array. Use
`shared.WriteJSON(w, 200, plans)`.

- [ ] **Step 1: Write the failing handler test**

```go
package plans

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeLister struct{ out []PlanEntity }

func (f fakeLister) ListActive(ctx context.Context) ([]PlanEntity, error) { return f.out, nil }

func TestHandler_List_200Array(t *testing.T) {
	h := NewHandler(NewService(fakeLister{out: []PlanEntity{}}))
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/plans", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var arr []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &arr); err != nil {
		t.Fatalf("not an array: %s", rr.Body)
	}
}
```

- [ ] **Step 2: Run red** — `go test ./internal/plans/ -run TestHandler_List` → FAIL.

- [ ] **Step 3a: Create `handler.go`**

```go
package plans

import (
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for GET /plans.
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List: GET /plans → 200, JSON array of raw plan entities. Public.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListActive(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "plans failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, plans)
}
```

- [ ] **Step 3b: Add `Plans` field + mount** in `router.go`. Add field near
  the public handlers block:

```go
	Plans http.Handler // GET /plans (public)
```

  And mount it among the public routes (after the health/metrics block, before
  the auth group):

```go
	if r.Plans != nil {
		mux.Method(http.MethodGet, "/plans", r.Plans)
	}
```

- [ ] **Step 3c: Wire in `main.go`.** In the place where core handlers are built
  (mirror the auth wiring), add:

```go
	plansReader := plans.NewReader(queries)          // queries = the sqlc *Queries already built
	plansSvc := plans.NewService(plansReader)
	plansHandler := plans.NewHandler(plansSvc)
```

  Add a `plans http.Handler` field to the `coreHandlers` struct, set
  `core.plans = http.HandlerFunc(plansHandler.List)`, and in the `Router{...}`
  literal add `Plans: core.plans`.

> NOTE: the implementer reads `main.go` to find the existing `queries`/`*Queries`
> variable name and the `coreHandlers` struct, and matches them. If `plans.NewReader`
> already takes the shared querier elsewhere, reuse that instance — do not build a
> second pool.

- [ ] **Step 4: Run green** — `go test ./internal/plans/ -race` PASS; `go build ./...` clean; `go test ./internal/shared/ -race` (router still builds) PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/plans/ internal/shared/router.go cmd/server/main.go
git add internal/plans/handler.go internal/plans/handler_test.go internal/shared/router.go cmd/server/main.go
git commit -m "feat(go-port): GET /plans handler + route wiring

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: subscription — nudge domain + DeriveBanner (pure logic)

**Files:**
- Create: `backend-rewrite-go/internal/subscription/nudge.go`
- Test: `backend-rewrite-go/internal/subscription/nudge_test.go`

**Context:** Port `subscriptions/nudge.ts` exactly. Thresholds inclusive (`<=`),
floats unrounded. `_usageToday` is ignored by `deriveBanner`. `lastExpiredAt`
nil-safe.

- [ ] **Step 1: Write the failing test (truth table + boundaries)**

```go
package subscription

import (
	"testing"
	"time"
)

func TestDeriveBanner(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	mk := func(d float64) *time.Time { t2 := now.Add(time.Duration(d * float64(24*time.Hour))); return &t2 }

	cases := []struct {
		name          string
		tier          string
		expiresAt     *time.Time
		lastExpiredAt *time.Time
		want          NudgeBanner
	}{
		{"pro expiring in 2d", "pro", mk(2), nil, BannerProExpiring},
		{"pro expiring exactly 3d (inclusive)", "pro", mk(3), nil, BannerProExpiring},
		{"pro 4d away → none", "pro", mk(4), nil, BannerNone},
		{"pro no expiry → none", "pro", nil, nil, BannerNone},
		{"free just expired 2d ago", "free", nil, mkPast(now, 2), BannerJustExpired},
		{"free expired exactly 7d ago (inclusive)", "free", nil, mkPast(now, 7), BannerJustExpired},
		{"free expired 8d ago → counter", "free", nil, mkPast(now, 8), BannerFreeCounter},
		{"free never expired → counter", "free", nil, nil, BannerFreeCounter},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DeriveBanner(c.tier, c.expiresAt, c.lastExpiredAt, now)
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func mkPast(now time.Time, days float64) *time.Time {
	t := now.Add(-time.Duration(days * float64(24*time.Hour)))
	return &t
}
```

- [ ] **Step 2: Run red** — `go test ./internal/subscription/ -run TestDeriveBanner` → FAIL.

- [ ] **Step 3: Implement `nudge.go`**

```go
package subscription

import "time"

// NudgeBanner mirrors the NestJS NudgeBanner enum string values.
type NudgeBanner string

const (
	BannerNone        NudgeBanner = "none"
	BannerProExpiring NudgeBanner = "pro_expiring"
	BannerJustExpired NudgeBanner = "just_expired"
	BannerFreeCounter NudgeBanner = "free_counter"
)

// Banner-window thresholds from nudge.ts. Comparisons inclusive (<=),
// arithmetic on unrounded day-fraction floats (matches the TS).
const (
	dayMS             = float64(86_400_000)
	proExpiringDays   = 3.0
	justExpiredDays   = 7.0
	FreeDailyLimit    = 10 // free-tier dailyLimit fallback
)

// DeriveBanner ports subscriptions/nudge.ts deriveBanner. usageToday is
// intentionally absent — the TS signature ignores it. tier is "pro" or
// "free"; expiresAt is the active sub's expiry (pro path); lastExpiredAt
// is the updated_at of the most-recent expired row (free path).
func DeriveBanner(tier string, expiresAt, lastExpiredAt *time.Time, now time.Time) NudgeBanner {
	if tier == "pro" {
		if expiresAt != nil {
			daysLeft := float64(expiresAt.Sub(now).Milliseconds()) / dayMS
			if daysLeft <= proExpiringDays {
				return BannerProExpiring
			}
		}
		return BannerNone
	}
	if lastExpiredAt != nil {
		daysSince := float64(now.Sub(*lastExpiredAt).Milliseconds()) / dayMS
		if daysSince <= justExpiredDays {
			return BannerJustExpired
		}
	}
	return BannerFreeCounter
}
```

> Parity note: nudge.ts computes `(expiresAt - now) / DAY_MS` on JS millisecond
> timestamps. `.Sub().Milliseconds()` reproduces the integer-ms delta exactly,
> then float-divides — identical to the TS for any sub-second now/expiry.

- [ ] **Step 4: Run green** — `go test ./internal/subscription/ -run TestDeriveBanner -race` → PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/subscription/nudge.go internal/subscription/nudge_test.go
git add internal/subscription/nudge.go internal/subscription/nudge_test.go
git commit -m "feat(go-port): subscription DeriveBanner nudge logic

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: subscription — repo queries (active+plan view, last-expired)

**Files:**
- Modify: `backend-rewrite-go/internal/shared/pg/queries_auth.sql`
- Modify: `backend-rewrite-go/internal/subscription/reader.go`
- Test: `backend-rewrite-go/internal/subscription/reader_test.go` (extend)

**Context:** GET /subscription needs the active sub joined to plan
(`name, daily_limit, billing_period`) AND the `updated_at` of the newest
EXPIRED row (for the free just_expired banner). The existing
`GetActiveSubscriptionByUserID` returns status/store_type/started_at/expires_at/
plan_name/daily_limit but NOT billing_period — add billing_period to a NEW
query (do not mutate the Phase 1a one, it's gate-pinned for /auth/account).

- [ ] **Step 1: Append two queries** to `queries_auth.sql`:

```sql
-- name: GetActiveSubWithPlan :one
-- GET /subscription: newest active sub + plan fields incl billing_period.
-- Distinct from GetActiveSubscriptionByUserID (which omits billing_period
-- and is pinned for /auth/account).
SELECT s.status, s.expires_at,
       p.name AS plan_name, p.daily_limit, p.billing_period
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active'::subscriptions_status_enum
ORDER BY s.created_at DESC
LIMIT 1;

-- name: GetLastExpiredAt :one
-- updated_at of the user's most-recently expired subscription (free-tier
-- just_expired banner). No row → caller treats as nil.
SELECT updated_at FROM subscriptions
WHERE user_id = $1 AND status = 'expired'::subscriptions_status_enum
ORDER BY updated_at DESC
LIMIT 1;
```

- [ ] **Step 2: Regenerate** — `cd backend-rewrite-go && sqlc generate`. Confirm `GetActiveSubWithPlan` + `GetLastExpiredAt` in `querier.go`.

- [ ] **Step 3: Write the failing reader test** (extend `reader_test.go`) for two
  new Reader methods `ActiveWithPlan(ctx,userID) (*SubView, error)` (nil when no
  row) and `LastExpiredAt(ctx,userID) (*time.Time, error)` (nil when no row).
  Use a fake querier returning canned rows + `pgx.ErrNoRows`.

```go
func TestActiveWithPlan_NoRow_Nil(t *testing.T) {
	r := NewReader(fakeQ{activeErr: pgx.ErrNoRows})
	v, err := r.ActiveWithPlan(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || v != nil {
		t.Fatalf("want nil,nil got %v,%v", v, err)
	}
}
func TestLastExpiredAt_NoRow_Nil(t *testing.T) {
	r := NewReader(fakeQ{expiredErr: pgx.ErrNoRows})
	tm, err := r.LastExpiredAt(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || tm != nil {
		t.Fatalf("want nil,nil got %v,%v", tm, err)
	}
}
```

> The implementer extends the existing `Querier` interface in `reader.go` with
> `GetActiveSubWithPlan` + `GetLastExpiredAt`, and extends the test's fake
> querier (or adds one) to satisfy all methods. Inspect the generated row struct
> for `BillingPeriod` enum type → `string(row.BillingPeriod)`.

- [ ] **Step 4: Run red** then implement the two Reader methods:

```go
// SubView is the active-sub projection GET /subscription needs.
type SubView struct {
	Status        string
	ExpiresAt     *time.Time
	PlanName      string
	DailyLimit    int
	BillingPeriod string
}

// ActiveWithPlan returns the newest active sub + plan, or (nil,nil) when none.
func (r *Reader) ActiveWithPlan(ctx context.Context, userID string) (*SubView, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil
	}
	row, err := r.q.GetActiveSubWithPlan(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v := &SubView{
		Status: string(row.Status), PlanName: row.PlanName,
		DailyLimit: int(row.DailyLimit), BillingPeriod: string(row.BillingPeriod),
	}
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		v.ExpiresAt = &t
	}
	return v, nil
}

// LastExpiredAt returns updated_at of the newest expired sub, or (nil,nil).
func (r *Reader) LastExpiredAt(ctx context.Context, userID string) (*time.Time, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return nil, nil
	}
	ts, err := r.q.GetLastExpiredAt(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !ts.Valid {
		return nil, nil
	}
	t := ts.Time
	return &t, nil
}
```

- [ ] **Step 5: Run green** — `go test ./internal/subscription/ -race` PASS; `go build ./...` clean. **Commit:**

```bash
cd backend-rewrite-go && gofmt -w internal/subscription/ internal/shared/pg/queries_auth.sql
git add internal/subscription/reader.go internal/subscription/reader_test.go internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc/
git commit -m "feat(go-port): subscription active+plan view + last-expired queries

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: subscription — Service.GetSubscription + VerifyReceipt + view marshalling

**Files:**
- Create: `backend-rewrite-go/internal/subscription/usecase.go`
- Create: `backend-rewrite-go/internal/subscription/view.go`
- Test: `backend-rewrite-go/internal/subscription/usecase_test.go`

**Context:** GET /subscription builds the full view object. `tier` = "pro" when
`plan.billing_period != "none"` else "free". `dailyLimit` for free fallback =
`FreeDailyLimit` (10) — BUT only when there's no active plan; when a sub exists
use `plan.daily_limit`. `usage_today` via the existing `usage.Counter` (port).
Top-level `plan` is null when no active sub; `status`/`expires_at` null too.
`nudge` always present. VerifyReceipt ignores its body, returns
`{subscription:…|null}` with `plan` key OMITTED when the plan name is empty
(mirrors `sub.plan?.name` → undefined → JSON omits).

The view's exact shapes (camelCase nudge, snake_case top-level, ms timestamps):

```go
// SubscriptionView is GET /subscription's 200 body.
type SubscriptionView struct {
	Plan       *PlanBrief `json:"plan"`
	Status     *string    `json:"status"`
	ExpiresAt  *string    `json:"expires_at"`
	UsageToday int        `json:"usage_today"`
	Nudge      Nudge      `json:"nudge"`
}
type PlanBrief struct {
	Name          string `json:"name"`
	DailyLimit    int    `json:"daily_limit"`
	BillingPeriod string `json:"billing_period"`
}
type Nudge struct {
	Tier       string  `json:"tier"`
	UsageToday int     `json:"usageToday"`
	DailyLimit int     `json:"dailyLimit"`
	ExpiresAt  *string `json:"expiresAt"`
	Banner     NudgeBanner `json:"banner"`
}
// ReceiptView is POST verify-receipt's 201 body.
type ReceiptView struct {
	Subscription *ReceiptSub `json:"subscription"`
}
type ReceiptSub struct {
	Plan      string  `json:"plan,omitempty"` // omitted when "" (sub.plan?.name undefined)
	Status    string  `json:"status"`
	ExpiresAt *string `json:"expires_at"`
}
```

- [ ] **Step 1: Write failing usecase tests**

```go
func TestGetSubscription_Pro(t *testing.T) {
	exp := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) // 2d → pro_expiring
	svc := newSubSvc(t, subStub{
		active: &SubView{Status: "active", ExpiresAt: &exp, PlanName: "Pro Monthly", DailyLimit: 1000, BillingPeriod: "monthly"},
		usage:  7,
	})
	v, err := svc.GetSubscription(context.Background(), "u1", time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC))
	if err != nil { t.Fatal(err) }
	if v.Plan == nil || v.Plan.Name != "Pro Monthly" { t.Fatalf("plan %+v", v.Plan) }
	if v.Nudge.Tier != "pro" || v.Nudge.Banner != BannerProExpiring { t.Fatalf("nudge %+v", v.Nudge) }
	if v.UsageToday != 7 || v.Nudge.DailyLimit != 1000 { t.Fatalf("usage/limit %+v", v) }
}

func TestGetSubscription_NoSub_Free(t *testing.T) {
	svc := newSubSvc(t, subStub{active: nil, usage: 3, lastExpired: nil})
	v, _ := svc.GetSubscription(context.Background(), "u1", time.Now())
	if v.Plan != nil || v.Status != nil || v.ExpiresAt != nil { t.Fatalf("want nulls %+v", v) }
	if v.Nudge.Tier != "free" || v.Nudge.DailyLimit != FreeDailyLimit || v.Nudge.Banner != BannerFreeCounter {
		t.Fatalf("nudge %+v", v.Nudge)
	}
}

func TestVerifyReceipt_OmitsPlanWhenEmpty(t *testing.T) {
	svc := newSubSvc(t, subStub{active: &SubView{Status: "active", PlanName: ""}})
	rv, _ := svc.VerifyReceipt(context.Background(), "u1")
	b, _ := json.Marshal(rv)
	if contains(string(b), `"plan"`) { t.Fatalf("plan should be omitted: %s", b) }
}
```

> The implementer writes `subStub` satisfying the Service's ports
> (`ActiveWithPlan`, `LastExpiredAt`, and a `UsageCounter` port with
> `CountToday(ctx,userID)(int,error)` — declared in THIS module, satisfied by
> `usage.Counter`). `newSubSvc` builds the Service from the stub. `contains` is
> the helper from the plans test (re-declare locally).

- [ ] **Step 2: Run red.**

- [ ] **Step 3: Implement `view.go`** (the structs above) and `usecase.go`:

```go
package subscription

import (
	"context"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// SubReader is the consumer-side port for the active sub + last-expired.
type SubReader interface {
	ActiveWithPlan(ctx context.Context, userID string) (*SubView, error)
	LastExpiredAt(ctx context.Context, userID string) (*time.Time, error)
}

// UsageCounter is the consumer-side port for today's usage count.
// Satisfied by *usage.Counter (extend, don't fork).
type UsageCounter interface {
	CountToday(ctx context.Context, userID string) (int, error)
}

// Service is the subscription read use case (GET + verify-receipt).
type Service struct {
	r     SubReader
	usage UsageCounter
}

// NewService wires the reader + usage counter.
func NewService(r SubReader, usage UsageCounter) *Service {
	return &Service{r: r, usage: usage}
}

// GetSubscription builds GET /subscription's view. now is injected for tests.
func (s *Service) GetSubscription(ctx context.Context, userID string, now time.Time) (SubscriptionView, error) {
	sub, err := s.r.ActiveWithPlan(ctx, userID)
	if err != nil {
		return SubscriptionView{}, err
	}
	usageToday, err := s.usage.CountToday(ctx, userID)
	if err != nil {
		return SubscriptionView{}, err
	}
	view := SubscriptionView{UsageToday: usageToday}

	var tier string
	var expForBanner *time.Time
	if sub != nil {
		tier = "free"
		if sub.BillingPeriod != "none" {
			tier = "pro"
		}
		view.Plan = &PlanBrief{Name: sub.PlanName, DailyLimit: sub.DailyLimit, BillingPeriod: sub.BillingPeriod}
		st := sub.Status
		view.Status = &st
		if sub.ExpiresAt != nil {
			iso := shared.ISOMillis(*sub.ExpiresAt)
			view.ExpiresAt = &iso
			expForBanner = sub.ExpiresAt
		}
	} else {
		tier = "free"
	}

	dailyLimit := FreeDailyLimit
	if sub != nil {
		dailyLimit = sub.DailyLimit
	}

	var lastExpired *time.Time
	if tier == "free" {
		lastExpired, err = s.r.LastExpiredAt(ctx, userID)
		if err != nil {
			return SubscriptionView{}, err
		}
	}

	nudge := Nudge{
		Tier: tier, UsageToday: usageToday, DailyLimit: dailyLimit,
		Banner: DeriveBanner(tier, expForBanner, lastExpired, now),
	}
	if view.ExpiresAt != nil {
		e := *view.ExpiresAt
		nudge.ExpiresAt = &e
	}
	view.Nudge = nudge
	return view, nil
}

// VerifyReceipt mirrors POST /subscription/verify-receipt: ignores the body,
// returns {subscription:…|null} from the active sub.
func (s *Service) VerifyReceipt(ctx context.Context, userID string) (ReceiptView, error) {
	sub, err := s.r.ActiveWithPlan(ctx, userID)
	if err != nil {
		return ReceiptView{}, err
	}
	if sub == nil {
		return ReceiptView{Subscription: nil}, nil
	}
	rs := &ReceiptSub{Plan: sub.PlanName, Status: sub.Status}
	if sub.ExpiresAt != nil {
		iso := shared.ISOMillis(*sub.ExpiresAt)
		rs.ExpiresAt = &iso
	}
	return ReceiptView{Subscription: rs}, nil
}
```

> Parity caveat to verify against Node: when an active sub exists but its
> `expires_at` is NULL, Node's `nudge.expiresAt` is null and top-level
> `expires_at` is null — the code above does that. When NO active sub, Node's
> `findActiveByUserId` returns null so the nudge runs with tier=free,
> lastExpiredAt path. Confirmed by the map.

- [ ] **Step 4: Run green** — `go test ./internal/subscription/ -race` PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/subscription/
git add internal/subscription/usecase.go internal/subscription/view.go internal/subscription/usecase_test.go
git commit -m "feat(go-port): subscription GetSubscription + VerifyReceipt use case

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: subscription — handlers + router + main wiring

**Files:**
- Create: `backend-rewrite-go/internal/subscription/handler.go`
- Test: `backend-rewrite-go/internal/subscription/handler_test.go`
- Modify: `backend-rewrite-go/internal/shared/router.go`
- Modify: `backend-rewrite-go/cmd/server/main.go`

**Context:** Both routes JWT-guarded → live in the auth group. GET → 200,
POST verify-receipt → **201** (default POST status). Read `claims.Sub` from
context. Use real `time.Now()` in the handler (tests inject now at the service
layer; the handler passes `time.Now()`).

- [ ] **Step 1: Write failing handler tests** — build a Service over a stub,
  set claims in context via `shared.ContextWithClaims`, assert 200 / 201 +
  presence of `nudge` / `subscription` keys.

```go
func TestHandler_GetSubscription_200(t *testing.T) {
	h := NewHandler(newSubSvc(t, subStub{active: nil, usage: 0}))
	req := httptest.NewRequest(http.MethodGet, "/subscription", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u1"}))
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != 200 || !contains(rr.Body.String(), `"nudge"`) {
		t.Fatalf("code %d body %s", rr.Code, rr.Body)
	}
}
func TestHandler_VerifyReceipt_201(t *testing.T) {
	h := NewHandler(newSubSvc(t, subStub{active: nil}))
	req := httptest.NewRequest(http.MethodPost, "/subscription/verify-receipt", strings.NewReader(`{"anything":1}`))
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u1"}))
	rr := httptest.NewRecorder()
	h.VerifyReceipt(rr, req)
	if rr.Code != 201 || !contains(rr.Body.String(), `"subscription"`) {
		t.Fatalf("code %d body %s", rr.Code, rr.Body)
	}
}
```

> Import `auth` for `auth.Claims` and `shared` for context+WriteJSON. Reuse
> `newSubSvc`/`subStub` from Task 7's test file (same package).

- [ ] **Step 2: Run red.**

- [ ] **Step 3a: Create `handler.go`**

```go
package subscription

import (
	"net/http"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /subscription.
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Get: GET /subscription (JWT) → 200.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.GetSubscription(r.Context(), claims.Sub, time.Now())
	if err != nil {
		shared.WriteError(w, r, "internal", "subscription failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, view)
}

// VerifyReceipt: POST /subscription/verify-receipt (JWT) → 201. Body ignored.
func (h *Handler) VerifyReceipt(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	view, err := h.svc.VerifyReceipt(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "verify-receipt failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, view)
}
```

- [ ] **Step 3b: Router** — add fields + mount inside the JWT `mux.Group`:

```go
	Subscription       http.Handler // GET /subscription (auth)
	VerifyReceipt      http.Handler // POST /subscription/verify-receipt (auth)
```

```go
		if r.Subscription != nil {
			api.Method(http.MethodGet, "/subscription", r.Subscription)
		}
		if r.VerifyReceipt != nil {
			api.Method(http.MethodPost, "/subscription/verify-receipt", r.VerifyReceipt)
		}
```

- [ ] **Step 3c: main.go** — build the subscription read service reusing the
  existing `usage.Counter` instance (do NOT build a second one):

```go
	subReader := subscription.NewReader(queries)        // existing sqlc *Queries
	subSvc := subscription.NewService(subReader, usageCounter) // usageCounter already built for auth
	subHandler := subscription.NewHandler(subSvc)
```

  Add `subscription, verifyReceipt http.Handler` to `coreHandlers`, set
  `core.subscription = http.HandlerFunc(subHandler.Get)` +
  `core.verifyReceipt = http.HandlerFunc(subHandler.VerifyReceipt)`, and add
  `Subscription: core.subscription, VerifyReceipt: core.verifyReceipt` to the
  Router literal.

> NOTE: `subscription.NewReader` exists (Phase 1a) and already takes the shared
> querier; the new `ActiveWithPlan`/`LastExpiredAt` are methods on that same
> Reader, which satisfies the `SubReader` port. Reuse the `usageCounter`
> variable name from the auth wiring (read main.go to confirm it).

- [ ] **Step 4: Run green** — `go test ./internal/subscription/ -race` PASS; `go build ./...` clean.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/subscription/ internal/shared/router.go cmd/server/main.go
git add internal/subscription/handler.go internal/subscription/handler_test.go internal/shared/router.go cmd/server/main.go
git commit -m "feat(go-port): GET /subscription + POST verify-receipt routes

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: subscription — expiry cron (pure logic + repo)

**Files:**
- Modify: `backend-rewrite-go/internal/shared/pg/queries_auth.sql`
- Create: `backend-rewrite-go/internal/subscription/cron.go`
- Test: `backend-rewrite-go/internal/subscription/cron_test.go`

**Context:** Port `subscriptions.cron.ts`. Two passes. Pass 1 = renewal
reminders for ACTIVE subs with `expires_at` in `[now+2.5d, now+3.5d]` (email,
no mutation). Pass 2 = expire ACTIVE|CANCELLED subs with `expires_at < now`
(bulk `UPDATE status='expired'`, expires_at UNTOUCHED) then email each. Notifier
best-effort: errors swallowed + logged, never abort the flip.

- [ ] **Step 1: Append queries** to `queries_auth.sql`:

```sql
-- name: SubsDueForRenewal :many
-- Cron pass 1: active subs whose expiry falls in the reminder window.
-- Bounds passed by caller (now+2.5d, now+3.5d) to keep tz in the Go process.
SELECT s.user_id, u.email, s.expires_at, p.name AS plan_name
FROM subscriptions s
JOIN users u ON u.id = s.user_id
JOIN plans p ON p.id = s.plan_id
WHERE s.status = 'active'::subscriptions_status_enum
  AND s.expires_at >= $1 AND s.expires_at <= $2;

-- name: ExpireLapsedSubs :many
-- Cron pass 2: flip active|cancelled subs past expiry to expired, returning
-- the affected rows so the caller can email each. expires_at left untouched.
UPDATE subscriptions s
SET status = 'expired'::subscriptions_status_enum, updated_at = now()
FROM users u
WHERE u.id = s.user_id
  AND s.status IN ('active'::subscriptions_status_enum, 'cancelled'::subscriptions_status_enum)
  AND s.expires_at < $1
RETURNING s.user_id, u.email, s.expires_at;
```

> The implementer confirms the `users` table + `email` column names against
> `schema.sql` and adjusts. `updated_at = now()` is required so the just_expired
> banner's `LastExpiredAt` (which orders by updated_at) reflects the flip — this
> matches TypeORM's `@UpdateDateColumn` auto-bump on the bulk update.

- [ ] **Step 2: Regenerate** — `sqlc generate`. Confirm `SubsDueForRenewal` + `ExpireLapsedSubs`.

- [ ] **Step 3: Write failing cron test** — fake repo + fake notifier; assert
  the renewal window queries with the right bounds, expire emails fire per
  returned row, and a notifier that returns an error does NOT abort.

```go
func TestCron_RunOnce_RemindersAndExpiry(t *testing.T) {
	now := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC)
	repo := &fakeCronRepo{
		due:     []RenewalRow{{Email: "a@b.com", PlanName: "Pro", ExpiresAt: now.Add(3 * 24 * time.Hour)}},
		expired: []ExpiredRow{{Email: "c@d.com", ExpiresAt: now.Add(-time.Hour)}},
	}
	notif := &fakeNotifier{failExpired: true} // notifier error must not abort
	cron := NewExpiryCron(repo, notif, slog.Default())
	if err := cron.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("RunOnce err %v", err)
	}
	if repo.dueLo != now.Add(60*time.Hour) || repo.dueHi != now.Add(84*time.Hour) {
		t.Fatalf("window bounds: lo=%v hi=%v", repo.dueLo, repo.dueHi)
	}
	if notif.reminders != 1 || notif.expired != 1 {
		t.Fatalf("notif counts r=%d e=%d", notif.reminders, notif.expired)
	}
}
```

> 2.5d = 60h, 3.5d = 84h. The implementer writes `fakeCronRepo` (records the
> bounds it was queried with, returns canned rows) and `fakeNotifier` (counts
> calls, can be told to error).

- [ ] **Step 4: Run red**, then implement `cron.go`:

```go
package subscription

import (
	"context"
	"log/slog"
	"time"
)

// RenewalRow / ExpiredRow are the cron's domain projections.
type RenewalRow struct {
	UserID    string
	Email     string
	PlanName  string
	ExpiresAt time.Time
}
type ExpiredRow struct {
	UserID    string
	Email     string
	ExpiresAt time.Time
}

// CronRepo is the consumer-side port for the expiry cron.
type CronRepo interface {
	DueForRenewal(ctx context.Context, lo, hi time.Time) ([]RenewalRow, error)
	ExpireLapsed(ctx context.Context, now time.Time) ([]ExpiredRow, error)
}

// CronNotifier sends the two emails. Best-effort: RunOnce ignores its errors.
type CronNotifier interface {
	SendRenewalReminder(ctx context.Context, row RenewalRow) error
	SendExpired(ctx context.Context, row ExpiredRow) error
}

// ExpiryCron is the daily entitlement-expiry job (port of subscriptions.cron.ts).
type ExpiryCron struct {
	repo  CronRepo
	notif CronNotifier
	log   *slog.Logger
}

// NewExpiryCron wires the cron.
func NewExpiryCron(repo CronRepo, notif CronNotifier, log *slog.Logger) *ExpiryCron {
	return &ExpiryCron{repo: repo, notif: notif, log: log}
}

// Renewal-reminder window: [now+2.5d, now+3.5d].
const (
	renewalLoHours = 60 * time.Hour // 2.5 days
	renewalHiHours = 84 * time.Hour // 3.5 days
)

// RunOnce executes both passes for the given now. Pass 1: renewal reminders
// (no mutation). Pass 2: expire lapsed (flip status, then email). Notifier
// failures are logged, never fatal.
func (c *ExpiryCron) RunOnce(ctx context.Context, now time.Time) error {
	due, err := c.repo.DueForRenewal(ctx, now.Add(renewalLoHours), now.Add(renewalHiHours))
	if err != nil {
		return err
	}
	for _, row := range due {
		if err := c.notif.SendRenewalReminder(ctx, row); err != nil {
			c.log.Warn("renewal reminder failed", "email", row.Email, "err", err)
		}
	}
	expired, err := c.repo.ExpireLapsed(ctx, now)
	if err != nil {
		return err
	}
	for _, row := range expired {
		if err := c.notif.SendExpired(ctx, row); err != nil {
			c.log.Warn("expired notice failed", "email", row.Email, "err", err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Implement the repo methods** in `reader.go` (or a new
  `cron_repo.go`) mapping the two sqlc queries → `[]RenewalRow`/`[]ExpiredRow`,
  passing `pgtype.Timestamp` bounds. Extend the Reader's `Querier` interface
  with `SubsDueForRenewal` + `ExpireLapsedSubs`. Run green:
  `go test ./internal/subscription/ -race` PASS; `go build ./...` clean.

- [ ] **Step 6: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/subscription/ internal/shared/pg/queries_auth.sql
git add internal/subscription/cron.go internal/subscription/cron_test.go internal/subscription/reader.go internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc/
git commit -m "feat(go-port): subscription expiry cron (renewal + lapse passes)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: subscription — daily scheduler + cron notifier + main wiring

**Files:**
- Create: `backend-rewrite-go/internal/platform/scheduler/daily.go`
- Test: `backend-rewrite-go/internal/platform/scheduler/daily_test.go`
- Create: `backend-rewrite-go/internal/subscription/cron_notifier.go` (email adapter)
- Modify: `backend-rewrite-go/cmd/server/main.go`

**Context:** Node uses `@Cron(EVERY_DAY_AT_9AM)` = `"0 09 * * *"` server-local.
Implement a dependency-free daily scheduler: a goroutine that computes the next
09:00 local, `time.Timer`s to it, fires the job, re-arms. The job is not
shadow-gated. The notifier adapts the existing email module (best-effort).

- [ ] **Step 1: Write failing scheduler test** — test the PURE next-fire
  computation, not the goroutine timing:

```go
package scheduler

import (
	"testing"
	"time"
)

func TestNextDailyAt(t *testing.T) {
	loc := time.UTC
	// before 09:00 → same day 09:00
	from := time.Date(2026, 6, 14, 8, 0, 0, 0, loc)
	if got := NextDailyAt(from, 9, 0, loc); !got.Equal(time.Date(2026, 6, 14, 9, 0, 0, 0, loc)) {
		t.Fatalf("before: %v", got)
	}
	// exactly 09:00 → next day (strictly after)
	at := time.Date(2026, 6, 14, 9, 0, 0, 0, loc)
	if got := NextDailyAt(at, 9, 0, loc); !got.Equal(time.Date(2026, 6, 15, 9, 0, 0, 0, loc)) {
		t.Fatalf("at: %v", got)
	}
	// after 09:00 → next day
	after := time.Date(2026, 6, 14, 10, 0, 0, 0, loc)
	if got := NextDailyAt(after, 9, 0, loc); !got.Equal(time.Date(2026, 6, 15, 9, 0, 0, 0, loc)) {
		t.Fatalf("after: %v", got)
	}
}
```

- [ ] **Step 2: Run red**, then implement `daily.go`:

```go
// Package scheduler runs background jobs on a daily wall-clock cadence
// without a cron dependency (one job today — YAGNI).
package scheduler

import (
	"context"
	"time"
)

// NextDailyAt returns the next instant strictly after `from` whose local
// time is hh:mm in loc. At exactly hh:mm it rolls to the next day (matches
// a cron that already fired this minute).
func NextDailyAt(from time.Time, hh, mm int, loc *time.Location) time.Time {
	from = from.In(loc)
	next := time.Date(from.Year(), from.Month(), from.Day(), hh, mm, 0, 0, loc)
	if !next.After(from) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// RunDaily blocks until ctx is cancelled, invoking job at each hh:mm local.
// Job errors are the job's own concern (it logs); RunDaily just re-arms.
func RunDaily(ctx context.Context, hh, mm int, loc *time.Location, now func() time.Time, job func(context.Context, time.Time)) {
	for {
		fire := NextDailyAt(now(), hh, mm, loc)
		timer := time.NewTimer(fire.Sub(now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			job(ctx, now())
		}
	}
}
```

- [ ] **Step 3: Create `cron_notifier.go`** — adapt the existing email module to
  `CronNotifier`. Inspect `internal/email` for the available send method
  (Phase 1b used it for verification/welcome/reset emails). Implement
  `SendRenewalReminder` + `SendExpired` calling the email sender; return its
  error (RunOnce swallows it). If the email module lacks renewal/expired
  templates, use the closest generic send with a subject/body literal matching
  Node's `subscription-notifier` text (read `backend/src/subscriptions/` for
  the exact subject lines) — keep it minimal; this path is not shadow-gated.

> Because the cron is not shadow-gated, exact email-body parity is NOT required
> for Phase 2 sign-off. Wire a working notifier; a TODO referencing the Node
> templates is acceptable if the email module doesn't expose them yet. State
> this explicitly in the commit body.

- [ ] **Step 4: Wire in `main.go`** — after building `subReader` + the email
  sender, build the cron repo (the Reader already satisfies `CronRepo` via its
  new methods), the notifier, the `ExpiryCron`, and launch
  `go scheduler.RunDaily(ctx, 9, 0, time.Local, time.Now, expiryCron.RunOnce)`.
  Use the server's root context so shutdown cancels it.

> NOTE: `ExpiryCron.RunOnce` matches `func(context.Context, time.Time) error`
> but `RunDaily` wants `func(context.Context, time.Time)`. Wrap:
> `func(c context.Context, t time.Time) { if err := expiryCron.RunOnce(c, t); err != nil { log.Error(...) } }`.

- [ ] **Step 5: Run green** — `go test ./internal/platform/scheduler/ -race` PASS; `go build ./...` clean.

- [ ] **Step 6: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/platform/scheduler/ internal/subscription/ cmd/server/main.go
git add internal/platform/scheduler/ internal/subscription/cron_notifier.go cmd/server/main.go
git commit -m "feat(go-port): daily scheduler + expiry cron wiring (not shadow-gated)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: exttoken — domain + token format + validation

**Files:**
- Create: `backend-rewrite-go/internal/exttoken/domain.go`
- Create: `backend-rewrite-go/internal/exttoken/validate.go`
- Test: `backend-rewrite-go/internal/exttoken/domain_test.go`

**Context:** New module. Token = `dr_ext_` + base64url(32 rand bytes, no pad) =
43 + 7 = 50 chars. Storage = sha256(FULL token incl prefix) hex 64. Mint DTO:
`device_id` UUIDv4, `device_name` 1..64 + regex `^[A-Za-z0-9 _.\-]+$` (message
`device_name must be alphanumeric/space/_/./- only`). No TTL.

- [ ] **Step 1: Write failing tests**

```go
package exttoken

import (
	"strings"
	"testing"
)

func TestGenerateToken_Format(t *testing.T) {
	raw, hash, err := generateToken(func(b []byte) (int, error) {
		for i := range b { b[i] = 0 } // deterministic
		return len(b), nil
	})
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(raw, "dr_ext_") { t.Fatalf("prefix: %s", raw) }
	if len(raw) != 50 { t.Fatalf("len %d (%s)", len(raw), raw) }
	if len(hash) != 64 { t.Fatalf("hash len %d", len(hash)) }
	// 32 zero bytes base64url-nopad = 43 'A's
	if raw != "dr_ext_"+strings.Repeat("A", 43) { t.Fatalf("body: %s", raw) }
}

func TestValidateMint(t *testing.T) {
	valid := "11111111-1111-4111-8111-111111111111"
	if msg := ValidateMint(valid, "My Phone_1.2-3"); msg != "" { t.Fatalf("should pass: %q", msg) }
	if msg := ValidateMint("not-a-uuid", "ok"); msg == "" { t.Fatal("bad uuid should fail") }
	if msg := ValidateMint(valid, ""); msg == "" { t.Fatal("empty name should fail") }
	if msg := ValidateMint(valid, strings.Repeat("x", 65)); msg == "" { t.Fatal("too long should fail") }
	if msg := ValidateMint(valid, "bad*name"); msg != "device_name must be alphanumeric/space/_/./- only" {
		t.Fatalf("regex msg: %q", msg)
	}
}
```

- [ ] **Step 2: Run red**, then implement `domain.go`:

```go
package exttoken

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// TokenPrefix is the literal that marks an extension token (distinguishes
// it from a JWT on the rewrite path).
const TokenPrefix = "dr_ext_"

// Sentinel errors mapped to the three 401 messages by the rewrite guard.
var (
	ErrMissingToken = errors.New("Missing bearer token")
	ErrInvalidToken = errors.New("Invalid extension token")
	ErrMissingScope = errors.New("Token missing rewrite scope")
)

// MintResult is the only place the raw token is exposed.
type MintResult struct {
	Token string
	ID    string
}

// TokenRow is the list-view projection (token_hash + user_id stripped).
type TokenRow struct {
	ID         string     `json:"id"`
	Scopes     []string   `json:"scopes"`
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	LastUsedAt *string    `json:"last_used_at"`
	CreatedAt  string     `json:"created_at"`
	RevokedAt  *string    `json:"revoked_at"` // always null on active list
	_          struct{}   // keep field order locked
}

// generateToken builds (rawToken, sha256hex) from 32 CSPRNG bytes.
// randRead is injectable for deterministic tests (prod: crypto/rand.Read).
func generateToken(randRead func([]byte) (int, error)) (string, string, error) {
	buf := make([]byte, 32)
	if _, err := randRead(buf); err != nil {
		return "", "", err
	}
	raw := TokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

// hashToken returns sha256hex of a full raw token (verify path).
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

var _ = time.Now // (placeholder import anchor; remove if unused)
```

> The implementer removes the `time` import + anchor if `time` ends up unused in
> domain.go (it's used by repo/usecase, not necessarily here). The `_ struct{}`
> in TokenRow is unnecessary — drop it; field order is already fixed by
> declaration. (Cleaned during code-quality review.)

- [ ] **Step 3: Implement `validate.go`** — mirror the Phase 1a validate
  convention (each message its own trailing period EXCEPT the regex message,
  which is the exact Node literal with no trailing period). UUIDv4 check via
  regex `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`:

```go
package exttoken

import "regexp"

var (
	uuidV4Re   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	deviceName = regexp.MustCompile(`^[A-Za-z0-9 _.\-]+$`)
)

// ValidateMint reproduces MintExtensionTokenDto validation. Empty = valid.
// Returns the first applicable message in property order (device_id, device_name).
func ValidateMint(deviceID, name string) string {
	if !uuidV4Re.MatchString(deviceID) {
		return "device_id must be a UUID"
	}
	if len(name) < 1 || len(name) > 64 {
		return "device_name must be longer than or equal to 1 and shorter than or equal to 64 characters"
	}
	if !deviceName.MatchString(name) {
		return "device_name must be alphanumeric/space/_/./- only"
	}
	return ""
}
```

> The implementer MUST verify the exact class-validator default messages for
> `@IsUUID('4')` and `@Length(1,64)` against the Node humanizer (Phase 1a's
> `auth/validate.go` shows the established wording style). If the Node app
> customizes them, match the custom text. The regex message is confirmed exact.
> If parity of the uuid/length messages is uncertain, note it for the shadow
> gate rather than guessing silently.

- [ ] **Step 4: Run green** — `go test ./internal/exttoken/ -race` PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/exttoken/
git add internal/exttoken/domain.go internal/exttoken/validate.go internal/exttoken/domain_test.go
git commit -m "feat(go-port): exttoken domain (dr_ext_ format, sha256, mint validation)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: exttoken — repo queries + sqlc

**Files:**
- Create: `backend-rewrite-go/internal/shared/pg/queries_exttoken.sql`
- Modify: `backend-rewrite-go/sqlc.yaml` (register the new query file)
- Create: `backend-rewrite-go/internal/exttoken/repo_pg.go`
- Test: covered via Task 13 service tests (repo is thin; `go build ./...` gates it)

**Context:** Five queries: revoke-active-by-device (pre-mint rotation), insert,
list-active, revoke-by-id (scoped to user), find-by-hash (verify), touch-last-used.

- [ ] **Step 1: Create `queries_exttoken.sql`**

```sql
-- name: RevokeActiveTokensForDevice :exec
-- Pre-mint rotation: revoke any active token for this (user, device).
UPDATE extension_tokens SET revoked_at = now()
WHERE user_id = $1 AND device_id = $2 AND revoked_at IS NULL;

-- name: InsertExtensionToken :one
INSERT INTO extension_tokens (user_id, token_hash, device_id, device_name)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListActiveTokens :many
-- Active tokens for the user, newest first. Caller strips token_hash+user_id.
SELECT id, scopes, device_id, device_name, last_used_at, created_at, revoked_at
FROM extension_tokens
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeTokenByID :exec
-- DELETE /:id — scoped to the caller's own active rows. Idempotent (0 rows OK).
UPDATE extension_tokens SET revoked_at = now()
WHERE user_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: FindActiveTokenByHash :one
-- Verify path: active token by sha256 hash. Returns user_id + scopes.
SELECT id, user_id, scopes FROM extension_tokens
WHERE token_hash = $1 AND revoked_at IS NULL
LIMIT 1;

-- name: TouchTokenLastUsed :exec
-- Fire-and-forget last_used_at bump after a successful verify.
UPDATE extension_tokens SET last_used_at = now() WHERE id = $1;
```

- [ ] **Step 2: Register the file** in `sqlc.yaml` under `queries:`:

```yaml
      - "internal/shared/pg/queries_exttoken.sql"
```

- [ ] **Step 3: Regenerate** — `sqlc generate`. Confirm the six functions in `querier.go`. `go build ./...` clean (generated code compiles).

- [ ] **Step 4: Implement `repo_pg.go`** — `PgRepo` wrapping the sqlc querier,
  mapping rows. `device_id`/`user_id` are `pgtype.UUID` (Scan from string).
  `scopes` is `[]string`. `last_used_at` `pgtype.Timestamp` → `*string` via
  `shared.ISOMillis` when Valid. List returns `make([]TokenRow,0,len)` (empty →
  `[]`). RevokeByID + RevokeForDevice are `:exec` (no rows-affected check —
  idempotent). The repo satisfies the `Repo` port declared in Task 13's
  usecase.go.

```go
package exttoken

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// PgRepo is the Postgres-backed extension-token store.
type PgRepo struct{ q ExtQuerier }

// ExtQuerier is the sqlc subset this repo needs.
type ExtQuerier interface {
	RevokeActiveTokensForDevice(ctx context.Context, arg sqlc.RevokeActiveTokensForDeviceParams) error
	InsertExtensionToken(ctx context.Context, arg sqlc.InsertExtensionTokenParams) (pgtype.UUID, error)
	ListActiveTokens(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListActiveTokensRow, error)
	RevokeTokenByID(ctx context.Context, arg sqlc.RevokeTokenByIDParams) error
	FindActiveTokenByHash(ctx context.Context, tokenHash string) (sqlc.FindActiveTokenByHashRow, error)
	TouchTokenLastUsed(ctx context.Context, id pgtype.UUID) error
}

// NewPgRepo wires the querier.
func NewPgRepo(q ExtQuerier) *PgRepo { return &PgRepo{q: q} }
```

> The implementer fills in the method bodies (Scan UUIDs, map rows, convert
> timestamps via `shared.ISOMillis`). Match the generated param/row struct
> field names exactly (e.g. `token_hash` may map to `TokenHash`). `char(64)`
> hash binds as a plain `string` param. Confirm `FindActiveTokenByHash` returns
> `UserID pgtype.UUID` + `Scopes []string`.

- [ ] **Step 5: Run** — `go build ./...` clean. **Commit:**

```bash
cd backend-rewrite-go && gofmt -w internal/exttoken/ sqlc.yaml
git add internal/exttoken/repo_pg.go internal/shared/pg/queries_exttoken.sql sqlc.yaml internal/shared/pg/sqlc/
git commit -m "feat(go-port): exttoken pg repo + queries

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: exttoken — Service (Mint / List / Revoke / Verify)

**Files:**
- Create: `backend-rewrite-go/internal/exttoken/usecase.go`
- Test: `backend-rewrite-go/internal/exttoken/usecase_test.go`

**Context:** Service owns its `Repo` port (consumer-side). Mint: rotate device →
generate → insert → return raw+id. List: passthrough. Revoke: passthrough
(idempotent). Verify: hash → find-active → scope check (`rewrite` ∈ scopes) →
fire-and-forget touch → return userID. Errors map to the three sentinels.

- [ ] **Step 1: Write failing tests** with a fake repo:

```go
func TestMint_RotatesThenInserts(t *testing.T) {
	repo := &fakeRepo{insertID: "tok-id"}
	svc := NewService(repo, zeroRand, fixedNow)
	res, err := svc.Mint(context.Background(), "u1", "dev1", "My Phone")
	if err != nil { t.Fatal(err) }
	if !repo.rotated { t.Fatal("did not rotate device") }
	if res.ID != "tok-id" || !strings.HasPrefix(res.Token, "dr_ext_") { t.Fatalf("%+v", res) }
	// stored hash matches the returned token
	if repo.insertedHash != hashToken(res.Token) { t.Fatal("hash mismatch") }
}

func TestVerify_OK_TouchesLastUsed(t *testing.T) {
	raw := "dr_ext_" + strings.Repeat("A", 43)
	repo := &fakeRepo{byHash: map[string]findResult{hashToken(raw): {userID: "u9", scopes: []string{"rewrite"}}}}
	svc := NewService(repo, zeroRand, fixedNow)
	uid, err := svc.Verify(context.Background(), raw)
	if err != nil || uid != "u9" { t.Fatalf("uid=%s err=%v", uid, err) }
	if !repo.touched { t.Fatal("last_used not touched") }
}

func TestVerify_Errors(t *testing.T) {
	svc := NewService(&fakeRepo{}, zeroRand, fixedNow)
	if _, err := svc.Verify(context.Background(), ""); err != ErrMissingToken { t.Fatal("empty") }
	if _, err := svc.Verify(context.Background(), "dr_ext_zzz"); err != ErrInvalidToken { t.Fatal("unknown") }
	raw := "dr_ext_" + strings.Repeat("B", 43)
	scoped := &fakeRepo{byHash: map[string]findResult{hashToken(raw): {userID: "u", scopes: []string{"other"}}}}
	svc2 := NewService(scoped, zeroRand, fixedNow)
	if _, err := svc2.Verify(context.Background(), raw); err != ErrMissingScope { t.Fatal("scope") }
}
```

> `zeroRand` = `func(b []byte)(int,error){ for i:=range b {b[i]=0}; return len(b),nil }`.
> `fixedNow` = `func() time.Time { return time.Unix(0,0).UTC() }`. The implementer
> writes `fakeRepo` + `findResult` satisfying the `Repo` port below.

- [ ] **Step 2: Run red**, then implement `usecase.go`:

```go
package exttoken

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Repo is the consumer-side port (satisfied by *PgRepo + test fakes).
type Repo interface {
	RotateDevice(ctx context.Context, userID, deviceID string) error
	Insert(ctx context.Context, userID, tokenHash, deviceID, deviceName string) (string, error)
	ListActive(ctx context.Context, userID string) ([]TokenRow, error)
	RevokeByID(ctx context.Context, userID, id string) error
	FindActiveByHash(ctx context.Context, tokenHash string) (userID string, scopes []string, err error)
	TouchLastUsed(ctx context.Context, id string) error
	// FindActiveByHash also needs the row id for TouchLastUsed; return it too.
}

// Service is the extension-token use case.
type Service struct {
	repo Repo
	rand func([]byte) (int, error)
	now  func() time.Time
}

// NewService wires the repo + injectable CSPRNG/clock (prod: crypto/rand.Read, time.Now).
func NewService(repo Repo, randRead func([]byte) (int, error), now func() time.Time) *Service {
	return &Service{repo: repo, rand: randRead, now: now}
}

// Mint rotates the device's prior token, generates a new one, stores its hash.
func (s *Service) Mint(ctx context.Context, userID, deviceID, deviceName string) (MintResult, error) {
	if err := s.repo.RotateDevice(ctx, userID, deviceID); err != nil {
		return MintResult{}, err
	}
	raw, hash, err := generateToken(s.rand)
	if err != nil {
		return MintResult{}, err
	}
	id, err := s.repo.Insert(ctx, userID, hash, deviceID, deviceName)
	if err != nil {
		return MintResult{}, err
	}
	return MintResult{Token: raw, ID: id}, nil
}

// List returns the user's active tokens (token_hash+user_id already stripped).
func (s *Service) List(ctx context.Context, userID string) ([]TokenRow, error) {
	return s.repo.ListActive(ctx, userID)
}

// Revoke marks a token revoked. Idempotent — unknown/foreign id is not an error.
func (s *Service) Revoke(ctx context.Context, userID, id string) error {
	return s.repo.RevokeByID(ctx, userID, id)
}

// Verify resolves a dr_ext_ token to its user, enforcing the rewrite scope.
func (s *Service) Verify(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", ErrMissingToken
	}
	userID, scopes, err := s.repo.FindActiveByHash(ctx, hashToken(raw))
	if errors.Is(err, pgx.ErrNoRows) || userID == "" {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", err
	}
	if !hasScope(scopes, "rewrite") {
		return "", ErrMissingScope
	}
	return userID, nil
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}
```

> DESIGN FIX during implementation: `Verify` must fire-and-forget
> `TouchLastUsed(id)`. That needs the row `id` from the hash lookup. Change the
> `Repo.FindActiveByHash` signature to also return `id string`
> (`(id, userID string, scopes []string, err error)`), call
> `go s.repo.TouchLastUsed(context.Background(), id)` (detached context, error
> ignored) before returning userID. Update `PgRepo` + the fake + the test
> (`repo.touched`) accordingly. The skeleton above omits it for brevity — the
> implementer wires it so `TestVerify_OK_TouchesLastUsed` passes.

- [ ] **Step 3: Run green** — `go test ./internal/exttoken/ -race` PASS; `go build ./...` clean.

- [ ] **Step 4: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/exttoken/
git add internal/exttoken/usecase.go internal/exttoken/usecase_test.go internal/exttoken/repo_pg.go
git commit -m "feat(go-port): exttoken service (mint/list/revoke/verify)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 14: exttoken — handlers (mint 200 / list 200 / delete 204)

**Files:**
- Create: `backend-rewrite-go/internal/exttoken/handler.go`
- Test: `backend-rewrite-go/internal/exttoken/handler_test.go`

**Context:** All JWT-guarded (read `claims.Sub`). Mint → **200** with
`{token,id}`; validation fail → 400 invalid-input (the ValidateMint message).
List → 200 array (`[]` when empty). Delete → **204** always (idempotent, no
body). The `:id` path param read via chi `chi.URLParam(r,"id")`.

- [ ] **Step 1: Write failing handler tests**

```go
func TestHandler_Mint_200(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{insertID: "id1"}, zeroRand, fixedNow))
	body := `{"device_id":"11111111-1111-4111-8111-111111111111","device_name":"My Phone"}`
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", strings.NewReader(body)), "u1")
	rr := httptest.NewRecorder()
	h.Mint(rr, req)
	if rr.Code != 200 { t.Fatalf("code %d body %s", rr.Code, rr.Body) }
	var m map[string]any
	json.Unmarshal(rr.Body.Bytes(), &m)
	if m["token"] == nil || m["id"] == nil { t.Fatalf("body %s", rr.Body) }
}

func TestHandler_Mint_BadDevice_400(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, zeroRand, fixedNow))
	body := `{"device_id":"nope","device_name":"x"}`
	req := withClaims(httptest.NewRequest(http.MethodPost, "/auth/extension-tokens", strings.NewReader(body)), "u1")
	rr := httptest.NewRecorder()
	h.Mint(rr, req)
	if rr.Code != 400 { t.Fatalf("code %d", rr.Code) }
}

func TestHandler_Delete_204_Always(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, zeroRand, fixedNow))
	// chi URL param via context
	req := withClaims(httptest.NewRequest(http.MethodDelete, "/auth/extension-tokens/whatever", nil), "u1")
	rctx := chi.NewRouteContext(); rctx.URLParams.Add("id", "whatever")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)
	if rr.Code != 204 { t.Fatalf("code %d", rr.Code) }
}
```

> `withClaims` = helper stamping `shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: uid})`.

- [ ] **Step 2: Run red**, then implement `handler.go`:

```go
package exttoken

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /auth/extension-tokens (all JWT-guarded).
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Mint: POST /auth/extension-tokens → 200 {token,id}.
func (h *Handler) Mint(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body struct {
		DeviceID   string `json:"device_id"`
		DeviceName string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if msg := ValidateMint(body.DeviceID, body.DeviceName); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	res, err := h.svc.Mint(r.Context(), claims.Sub, body.DeviceID, body.DeviceName)
	if err != nil {
		shared.WriteError(w, r, "internal", "mint failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"token": res.Token, "id": res.ID})
}

// List: GET /auth/extension-tokens → 200 array.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	rows, err := h.svc.List(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "list failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, rows)
}

// Delete: DELETE /auth/extension-tokens/{id} → 204, silent + idempotent.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	id := chi.URLParam(r, "id")
	_ = h.svc.Revoke(r.Context(), claims.Sub, id) // idempotent; ignore not-found
	w.WriteHeader(http.StatusNoContent)
}
```

> Parity check: Node returns 204 for ANY id, even a malformed one. The Go
> `Revoke` path passes `id` to a parameterized UPDATE; a malformed UUID makes
> the pg Scan fail. The implementer ensures `RevokeByID` swallows a Scan error
> (returns nil) so the handler still emits 204 — NEVER 400/500 on delete. Add a
> test feeding `id="not-a-uuid"` expecting 204.

- [ ] **Step 3: Run green** — `go test ./internal/exttoken/ -race` PASS.

- [ ] **Step 4: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/exttoken/
git add internal/exttoken/handler.go internal/exttoken/handler_test.go
git commit -m "feat(go-port): exttoken handlers (mint 200, list 200, delete 204)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: exttoken — routes + main wiring (CRUD mounted under JWT group)

**Files:**
- Modify: `backend-rewrite-go/internal/shared/router.go`
- Modify: `backend-rewrite-go/cmd/server/main.go`
- Test: `backend-rewrite-go/internal/shared/router_test.go` (extend if present)

**Context:** Three routes in the JWT group. `POST` + `GET` on
`/auth/extension-tokens`, `DELETE` on `/auth/extension-tokens/{id}`.

- [ ] **Step 1: Add router fields**

```go
	ExtTokenMint   http.Handler // POST   /auth/extension-tokens (auth)
	ExtTokenList   http.Handler // GET    /auth/extension-tokens (auth)
	ExtTokenDelete http.Handler // DELETE /auth/extension-tokens/{id} (auth)
```

- [ ] **Step 2: Mount inside the JWT `mux.Group`**

```go
		if r.ExtTokenMint != nil {
			api.Method(http.MethodPost, "/auth/extension-tokens", r.ExtTokenMint)
		}
		if r.ExtTokenList != nil {
			api.Method(http.MethodGet, "/auth/extension-tokens", r.ExtTokenList)
		}
		if r.ExtTokenDelete != nil {
			api.Method(http.MethodDelete, "/auth/extension-tokens/{id}", r.ExtTokenDelete)
		}
```

- [ ] **Step 3: Wire main.go**

```go
	extRepo := exttoken.NewPgRepo(queries)
	extSvc := exttoken.NewService(extRepo, rand.Read, time.Now) // crypto/rand
	extHandler := exttoken.NewHandler(extSvc)
```

  Add `extMint, extList, extDelete http.Handler` to `coreHandlers`, assign
  `http.HandlerFunc(extHandler.Mint/.List/.Delete)`, add the three fields to the
  Router literal. Import `crypto/rand`.

> `rand.Read` has signature `func([]byte)(int,error)` — matches the Service's
> port. Keep the existing `crypto/rand` import name unaliased; if `math/rand` is
> already imported, alias appropriately.

- [ ] **Step 4: Run green** — `go build ./...` clean; `go test ./... -race` PASS.

- [ ] **Step 5: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/shared/router.go cmd/server/main.go
git add internal/shared/router.go cmd/server/main.go internal/shared/router_test.go
git commit -m "feat(go-port): mount extension-token CRUD routes

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 16: rewrite dual-auth — accept dr_ext_ tokens on /v1/rewrite

**Files:**
- Create: `backend-rewrite-go/internal/shared/exttoken_auth.go` (dual-auth middleware)
- Test: `backend-rewrite-go/internal/shared/exttoken_auth_test.go`
- Modify: `backend-rewrite-go/internal/shared/router.go` (apply to /v1/rewrite only)
- Modify: `backend-rewrite-go/cmd/server/main.go` (inject the verifier)

**Context:** This closes the parity gap: Node's `RewriteAuthGuard` accepts EITHER
a JWT OR a `dr_ext_` token on the rewrite route; Go currently only does JWT. The
rewrite route must, when the bearer starts with `dr_ext_`, verify via the
exttoken Service and inject the resolved user into the SAME context shape
`RequireAuth` produces (so the rewrite handler reads `claims.Sub` unchanged).
Otherwise fall through to the existing `RequireAuth`. All ext-token failures →
**401** with the exact messages. The JWT path stays byte-identical.

The rewrite handler reads the user id from `shared.ClaimsFromContext` →
`*auth.Claims`. The dual-auth shim must build a minimal `*auth.Claims{Sub:userID}`
for the ext-token case.

- [ ] **Step 1: Write failing test**

```go
package shared

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeExtVerifier struct {
	uid string
	err error
}

func (f fakeExtVerifier) Verify(ctx context.Context, raw string) (string, error) {
	return f.uid, f.err
}

func TestExtOrJWT_ExtTokenPath(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		c, ok := ClaimsFromContext(r.Context())
		if !ok || c.Sub != "u7" {
			t.Fatalf("claims not injected: %+v ok=%v", c, ok)
		}
		w.WriteHeader(200)
	})
	mw := ExtOrJWT(fakeExtVerifier{uid: "u7"}, jwtPassThrough(t), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/rewrite", nil)
	req.Header.Set("Authorization", "Bearer dr_ext_"+repeat("A", 43))
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if !called || rr.Code != 200 {
		t.Fatalf("ext path failed code=%d called=%v", rr.Code, called)
	}
}

func TestExtOrJWT_ExtInvalid_401(t *testing.T) {
	mw := ExtOrJWT(fakeExtVerifier{err: errInvalidExt()}, mustNotRunJWT(t), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/rewrite", nil)
	req.Header.Set("Authorization", "Bearer dr_ext_bad")
	rr := httptest.NewRecorder()
	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next should not run") })).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestExtOrJWT_NonExt_FallsThroughToJWT(t *testing.T) {
	jwtRan := false
	jwt := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jwtRan = true; next.ServeHTTP(w, r) })
	}
	mw := ExtOrJWT(fakeExtVerifier{}, jwt, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/rewrite", nil)
	req.Header.Set("Authorization", "Bearer eyJ.jwt.here")
	rr := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).ServeHTTP(rr, req)
	if !jwtRan {
		t.Fatal("JWT middleware did not run for non-ext token")
	}
}
```

> The implementer supplies the small test helpers (`repeat`, `jwtPassThrough`
> returning a middleware that injects a claims and calls next, `mustNotRunJWT`
> failing if invoked, `errInvalidExt` returning the exttoken sentinel). To map
> the exttoken sentinels without importing `exttoken` into `shared` (cycle
> risk), the verifier port returns errors whose `.Error()` strings are compared,
> OR the shim maps any non-nil verify error to the right 401 message by string.
> SIMPLEST: the `ExtVerifier` port lives in `shared`; the exttoken sentinels'
> `.Error()` text IS the 401 message, so the shim writes
> `WriteError(w,r,"invalid-token", err.Error())` directly. Confirm
> `shared` does not import `exttoken` (only an interface + error string).

- [ ] **Step 2: Run red**, then implement `exttoken_auth.go`:

```go
package shared

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

// extTokenPrefix marks an extension token vs a JWT on the rewrite path.
// Duplicated literal (not imported from exttoken) to keep shared dependency-free.
const extTokenPrefix = "dr_ext_"

// ExtVerifier resolves a raw dr_ext_ token to a user id. Its error's
// .Error() string IS the 401 message (matches NestJS RewriteAuthGuard).
type ExtVerifier interface {
	Verify(ctx context.Context, raw string) (string, error)
}

// ExtOrJWT guards the rewrite route with dual auth: a Bearer token starting
// with dr_ext_ is verified as an extension token (resolved user injected as
// claims); anything else falls through to the standard JWT middleware. This
// mirrors NestJS RewriteAuthGuard. Apply ONLY to /v1/rewrite.
func ExtOrJWT(v ExtVerifier, jwt func(http.Handler) http.Handler, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		jwtChain := jwt(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if !strings.HasPrefix(raw, extTokenPrefix) {
				jwtChain.ServeHTTP(w, r) // JWT path unchanged
				return
			}
			uid, err := v.Verify(r.Context(), raw)
			if err != nil {
				WriteError(w, r, "invalid-token", err.Error())
				return
			}
			ctx := ContextWithClaims(r.Context(), &auth.Claims{Sub: uid})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token after "Bearer ", or "" if absent.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return h[len(p):]
	}
	return ""
}
```

- [ ] **Step 3: Apply to /v1/rewrite only** in `router.go`. The current group
  wraps `/v1/rewrite` with `api.Use(RequireAuth(...))` alongside the other auth
  routes. Split the rewrite route out so ONLY it gets `ExtOrJWT`, while the other
  auth routes keep plain `RequireAuth`:

```go
	mux.Group(func(api chi.Router) {
		api.Use(RequireAuth(r.Verifier, r.Log))
		// rewrite route handled separately below (dual-auth)
		if r.Me != nil { api.Method(http.MethodGet, "/auth/me", r.Me) }
		// ... change-password, account, delete, subscription, ext-tokens ...
	})

	// /v1/rewrite: dr_ext_ OR JWT (NestJS RewriteAuthGuard parity).
	if r.ExtVerifier != nil {
		mux.With(ExtOrJWT(r.ExtVerifier, RequireAuth(r.Verifier, r.Log), r.Log)).
			Post("/v1/rewrite", r.Rewrite.ServeHTTP)
	} else {
		mux.With(RequireAuth(r.Verifier, r.Log)).Post("/v1/rewrite", r.Rewrite.ServeHTTP)
	}
```

  Add an `ExtVerifier ExtVerifier` field to the Router struct. Remove the
  `/v1/rewrite` line from inside the JWT group (it moves out). Keep all other
  auth routes in the group untouched.

> CRITICAL: do NOT change the JWT path's behavior for /v1/rewrite — when
> `ExtVerifier` is nil OR the token is a JWT, the exact same `RequireAuth`
> runs. Verify the existing rewrite tests (`handler_rewrite_test.go`) still pass
> unchanged — they send `Bearer <jwt>` and must keep working.

- [ ] **Step 4: Wire main.go** — pass the exttoken Service as the verifier:
  `Router{ ..., ExtVerifier: extSvc }`. `*exttoken.Service` satisfies
  `shared.ExtVerifier` (it has `Verify(ctx, raw) (string, error)`).

- [ ] **Step 5: Run green** — `go test ./... -race` PASS (especially the existing
  rewrite tests); `go build ./...` clean; `gofmt -l .` empty; `go vet ./...` clean.

- [ ] **Step 6: Commit**

```bash
cd backend-rewrite-go && gofmt -w internal/shared/ cmd/server/main.go
git add internal/shared/exttoken_auth.go internal/shared/exttoken_auth_test.go internal/shared/router.go cmd/server/main.go
git commit -m "feat(go-port): dual-auth on /v1/rewrite (dr_ext_ or JWT, RewriteAuthGuard parity)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final review (after Task 16)

- [ ] `cd backend-rewrite-go && go test ./... -race` — all green.
- [ ] `go build ./...` clean, `gofmt -l .` empty, `go vet ./...` clean.
- [ ] `sqlc generate` produces no diff (CI gate parity).
- [ ] Dispatch a final code-reviewer over the whole Phase 2 diff
  (`git diff develop...HEAD`) — focus: status-code parity (200/201/204), JSON
  key order + ms timestamps, the three 401 ext-token messages, JWT path on
  /v1/rewrite unchanged, no second DB pool / second usage.Counter built.
- [ ] Then superpowers:finishing-a-development-branch → merge to develop `--no-ff`.

## Spec coverage self-check

| Spec item | Task |
|---|---|
| GET /plans 200 array, entity order, raw price_cents, ms dates | 1,2,3,4 |
| GET /subscription 200, nudge, tier, free fallback 10 | 5,6,7,8 |
| DeriveBanner inclusive thresholds | 5 |
| POST verify-receipt 201, body ignored, plan omitempty | 7,8 |
| Expiry cron (renewal window, expire active+cancelled, best-effort notifier) | 9,10 |
| Daily 09:00 scheduler | 10 |
| exttoken mint 200, dr_ext_ format, sha256, rotation | 11,12,13,14 |
| exttoken list 200 stripped, delete 204 idempotent | 12,13,14 |
| exttoken validation (uuid4, name regex message) | 11 |
| dr_ext_ accepted on /v1/rewrite, 3×401 messages, JWT unchanged | 16 |

## Operator-pending (NOT in these tasks)

- Live shadow gate vs dev Node (dev token + Go on dev DB).
- No prod/Caddy change. Cron + scheduler run but are not shadow-compared.
