# Phase 4c-2 — Admin Content/Ops CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the 25 NestJS `AdminController` content/ops CRUD endpoints to Go as a byte-identical drop-in — identical routes, request shapes, response JSON (keys IN ORDER), status codes, and error envelopes — and make these the first real consumers of the `listquery` helper from 4c-1.

**Architecture:** Two NEW flat clean-arch modules (`internal/aiprovider`, `internal/appsettings`) following the `domain.go → usecase.go → repo_pg.go → handler.go` recipe, plus admin extensions to four existing modules (`user`, `plans`, `adminauth`, `email`). Cross-module needs (usage count, active subscription, payment-method validation, email send, building a Completer from a stored provider row) are expressed as small consumer-side port interfaces injected from `cmd/server/main.go` — the only composition root. Handler tests use fakes; no DB. Node is the parity authority; ship nothing to prod.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc v1.31, golang-jwt/jwt/v5 (HS256, zero clock tolerance), x/crypto/bcrypt (cost 10), slog.

---

## Conventions used by every task

These are established in the codebase (4c-1 and earlier). Reuse them verbatim; do not reinvent.

**Error envelope** — `shared.WriteError(w, r, code, message)`. Codes → status (from `internal/shared/errenvelope.go`): `invalid-input`→400, `invalid-token`/`user-not-found`→401, `forbidden`→403, `not-found`→404, `conflict`→409, default→500. Body is `{error,code,request_id}`; `request_id` is dynamic, so assert it by presence, never by exact value.

**JSON writer** — `shared.WriteJSON(w, status, v)` sets `Content-Type: application/json` and encodes `v`.

**Ordered JSON** — Go marshals struct fields in declaration order. For every response object, define a domain struct with a `MarshalJSON` that wraps an anonymous struct carrying the exact `json:"…"` tags IN NODE ORDER (the `plans.PlanEntity` pattern in `internal/plans/domain.go`). Nullable columns → pointer fields (`*string`, `*time.Time`) so `null` serializes identically. Timestamps render through `shared.ISOMillis(t)` (`2006-01-02T15:04:05.000Z07:00`).

**listquery** — `listquery.Parse(r.URL.Query()) Query`; `listquery.Build(q Query, searchCols []string, sortAllow map[string]string, defaultSort, statusCol string) Built`. `Built{Where, Args, Order, Limit, Offset}`. `searchCols` are SQL column literals (never user input); `sortAllow` maps the public `sort_by` value → SQL column literal (injection-safe allow-list, includes Node aliases like `price`→`price_cents`); `statusCol` empty disables status filtering.

**Test helpers** — each new test file needs these (copy from `internal/feedback/handler_test.go`): `decodeBody(t, raw) map[string]any`, `assertKeyOrder(t, raw, keys...)`, and for routed params `routeWithID(r, id)`. The FIRST test task of each new package defines them; later test tasks in the same package reuse them.

**Admin guard** — all 25 routes mount inside the existing admin group (`admin.Use(jwtMW); admin.Use(RequireAdmin)`) in `internal/shared/router.go`. Handlers themselves never re-check admin.

**Staging rule (subagent-driven):** every implementer stages ONLY the files named in its task. NEVER `git add -A`. **Any task that adds a NEW `queries_*.sql` file MUST also stage `sqlc.yaml`** — `sqlc.yaml` lists each query file explicitly under `sql.queries`, so a new query file requires a one-line registration there or `sqlc generate` won't pick it up and the CI `sqlc-check.sh` gate fails. Applies to Tasks 6, 9, 12, 14, 16.

**Full gate** (run before the final review): `go build ./...`, `go vet ./...`, `gofmt -l .` prints nothing, `go test ./... -race` green.

---

## Module / file map

| Module | New / Modify | Files |
|--------|--------------|-------|
| `internal/aiprovider` | NEW | `domain.go`, `usecase.go`, `repo_pg.go`, `handler.go`, `*_test.go` |
| `internal/appsettings` | NEW | `domain.go`, `usecase.go`, `repo_pg.go`, `handler.go`, `*_test.go` |
| `internal/plans` | MODIFY | `domain.go` (+writer methods), `admin_repo_pg.go` (new), `admin_usecase.go`, `admin_handler.go`, `*_test.go` |
| `internal/user` | MODIFY | `admin_domain.go`, `admin_usecase.go`, `admin_repo_pg.go`, `admin_handler.go`, `*_test.go` |
| `internal/adminauth` | MODIFY | `admin_users_usecase.go`, `admin_users_repo_pg.go`, `admin_users_handler.go`, `*_test.go` |
| `internal/email` | MODIFY | `admin_logs_{repo_pg,usecase,handler}.go`, `admin_templates_{repo_pg,usecase,handler}.go`, `*_test.go` |
| `internal/aiprovider` (factory) | NEW | `completer_factory.go`, `*_test.go` |
| `internal/shared/pg/queries_*.sql` | MODIFY | new queries per module; `sqlc generate` |
| `cmd/server/main.go` | MODIFY | wire all handlers + cross-module ports |
| `internal/shared/router.go` | MODIFY | add Router fields + mount 25 routes |

**Task order** (dependency-aware): aiprovider → appsettings → plans → user → adminauth → email → main.go wiring → router mount → integration tests → gate. Final whole-branch review is run by the subagent-driven controller after the last task.

---

## Group A — `internal/aiprovider` (NEW module, routes 8–13)

### Task 1: aiprovider domain struct + ordered JSON

**Files:**
- Create: `internal/aiprovider/domain.go`
- Test: `internal/aiprovider/domain_test.go`

Node field order (`src/ai-providers/entities/ai-provider.entity.ts`): `id, name, type, endpoint_url, api_key, model, temperature, is_default, is_active, created_at, updated_at`. **`api_key` is returned in plaintext** (parity; follow-up §8). `temperature` is `decimal(3,2)`.

- [ ] **Step 1: Determine the `temperature` JSON type (parity)**

`decimal` columns in TypeORM come back as a JS `string` UNLESS the entity declares a `ColumnNumericTransformer`. Check the Node entity: `grep -n "transformer\|temperature" /opt/openAi/DraftRight/backend/src/ai-providers/entities/ai-provider.entity.ts`. If a numeric transformer is present → `temperature` serializes as a JSON **number** (Go field `float64`, tag `json:"temperature"`). If absent → it serializes as a JSON **string** like `"0.30"` (Go field `string`). Record the finding in a code comment. The struct below assumes **number**; switch the field to `string` if the grep shows no transformer.

- [ ] **Step 2: Write the failing test**

```go
package aiprovider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAiProvider_JSONKeyOrder(t *testing.T) {
	key := "sk-test"
	p := AiProvider{
		ID: "11111111-1111-1111-1111-111111111111", Name: "OpenAI", Type: "openai",
		EndpointURL: "https://api.openai.com/v1", APIKey: key, Model: "gpt-4o-mini",
		Temperature: 0.3, IsDefault: true, IsActive: true,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{"id", "name", "type", "endpoint_url", "api_key", "model",
		"temperature", "is_default", "is_active", "created_at", "updated_at"}
	prev := -1
	for _, k := range order {
		idx := strings.Index(string(raw), `"`+k+`"`)
		if idx <= prev {
			t.Fatalf("key %q out of order: %s", k, raw)
		}
		prev = idx
	}
	// api_key must be plaintext (parity, not masked).
	if !strings.Contains(string(raw), `"api_key":"sk-test"`) {
		t.Fatalf("api_key must be plaintext: %s", raw)
	}
	if !strings.Contains(string(raw), `"created_at":"1970-01-01T00:00:00.000Z"`) {
		t.Fatalf("timestamp not ISO-millis: %s", raw)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && go test ./internal/aiprovider/ -run TestAiProvider_JSONKeyOrder`
Expected: FAIL — `undefined: AiProvider`.

- [ ] **Step 4: Write the implementation**

```go
package aiprovider

import (
	"encoding/json"
	"errors"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"time"
)

// ErrNotFound is returned when an ai_provider row does not exist.
var ErrNotFound = errors.New("ai provider not found")

// AiProvider mirrors the ai_providers row. api_key is returned in plaintext
// on every read (Node parity — hardening tracked in spec §8). Field order
// matches src/ai-providers/entities/ai-provider.entity.ts exactly.
type AiProvider struct {
	ID          string
	Name        string
	Type        string
	EndpointURL string
	APIKey      string
	Model       string
	Temperature float64 // JSON number; see Task 1 Step 1 (switch to string if Node has no numeric transformer)
	IsDefault   bool
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p AiProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		EndpointURL string  `json:"endpoint_url"`
		APIKey      string  `json:"api_key"`
		Model       string  `json:"model"`
		Temperature float64 `json:"temperature"`
		IsDefault   bool    `json:"is_default"`
		IsActive    bool    `json:"is_active"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, Type: p.Type, EndpointURL: p.EndpointURL,
		APIKey: p.APIKey, Model: p.Model, Temperature: p.Temperature,
		IsDefault: p.IsDefault, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/aiprovider/ -run TestAiProvider_JSONKeyOrder`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/aiprovider/domain.go internal/aiprovider/domain_test.go
git commit -m "feat(aiprovider): domain struct with Node-ordered JSON"
```

---

### Task 2: aiprovider SQL queries + repo

**Files:**
- Create: `internal/shared/pg/queries_aiprovider.sql`
- Create: `internal/aiprovider/repo_pg.go`
- Modify: regenerate `internal/shared/pg/sqlc` via `sqlc generate`

The repo needs: list all (created_at ASC), paginated rows (dynamic WHERE/ORDER/LIMIT/OFFSET → run via pgxpool, NOT sqlc, because sort is dynamic — same approach 4c-1 specced for listquery), count for pagination, get by id, create, update (partial), soft-delete, and demote-all-defaults (single-default invariant). Static queries go through sqlc; the dynamic paginated SELECT is built from `listquery.Built`.

- [ ] **Step 1: Write the static sqlc queries**

```sql
-- name: ListAiProviders :many
SELECT id, name, type, endpoint_url, api_key, model, temperature,
       is_default, is_active, created_at, updated_at
FROM ai_providers
ORDER BY created_at ASC;

-- name: GetAiProviderByID :one
SELECT id, name, type, endpoint_url, api_key, model, temperature,
       is_default, is_active, created_at, updated_at
FROM ai_providers WHERE id = $1;

-- name: DemoteDefaultAiProviders :exec
UPDATE ai_providers SET is_default = false WHERE is_default = true;

-- name: InsertAiProvider :one
INSERT INTO ai_providers (name, type, endpoint_url, api_key, model, temperature, is_default, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, name, type, endpoint_url, api_key, model, temperature,
          is_default, is_active, created_at, updated_at;

-- name: SoftDeleteAiProvider :exec
UPDATE ai_providers SET is_default = false, is_active = false WHERE id = $1;
```

- [ ] **Step 2: Run `sqlc generate`, verify build**

Run: `cd /opt/openAi/DraftRight/backend-rewrite-go && sqlc generate && go build ./internal/shared/pg/...`
Expected: no error; new methods on `*sqlc.Queries`.

- [ ] **Step 3: Write the failing repo test (mapping + dynamic paginated SQL shape)**

```go
package aiprovider

import "testing"

func TestPaginatedSQL_UsesListqueryBuilt(t *testing.T) {
	// paginatedSQL composes the SELECT with the listquery WHERE/ORDER.
	where := "WHERE (name ILIKE $1)"
	order := "ORDER BY created_at DESC"
	got := paginatedSQL(where, order)
	want := "SELECT id, name, type, endpoint_url, api_key, model, temperature, " +
		"is_default, is_active, created_at, updated_at FROM ai_providers " +
		"WHERE (name ILIKE $1) ORDER BY created_at DESC LIMIT $%d OFFSET $%d"
	if got != want {
		t.Fatalf("paginatedSQL mismatch:\n got=%q\nwant=%q", got, want)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/aiprovider/ -run TestPaginatedSQL`
Expected: FAIL — `undefined: paginatedSQL`.

- [ ] **Step 5: Write `repo_pg.go`**

```go
package aiprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the repo needs.
type Querier interface {
	ListAiProviders(ctx context.Context) ([]sqlc.AiProvider, error)
	GetAiProviderByID(ctx context.Context, id pgtypeUUID) (sqlc.AiProvider, error)
	DemoteDefaultAiProviders(ctx context.Context) error
	InsertAiProvider(ctx context.Context, arg sqlc.InsertAiProviderParams) (sqlc.AiProvider, error)
	SoftDeleteAiProvider(ctx context.Context, id pgtypeUUID) error
}

// NOTE: pgtypeUUID is an alias documented here for the plan — in code use the
// real pgtype.UUID from "github.com/jackc/pgx/v5/pgtype". Map string id →
// pgtype.UUID with uid.Scan(id); on scan failure return ErrNotFound.

// PgRepo runs static queries via sqlc and the dynamic paginated SELECT via pool.
type PgRepo struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

func NewPgRepo(q *sqlc.Queries, pool *pgxpool.Pool) *PgRepo { return &PgRepo{q: q, pool: pool} }

// paginatedSQL composes the rows query from a listquery WHERE + ORDER. The two
// trailing positional params are LIMIT and OFFSET; the caller supplies their
// numbers (len(args)+1, +2).
func paginatedSQL(where, order string) string {
	return "SELECT id, name, type, endpoint_url, api_key, model, temperature, " +
		"is_default, is_active, created_at, updated_at FROM ai_providers " +
		fmt.Sprintf("%s %s LIMIT $%%d OFFSET $%%d", where, order)
}

func countSQL(where string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM ai_providers %s", where)
}

// ListPaginated runs the rows + count queries from a listquery.Built.
func (r *PgRepo) ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error) {
	limPos := len(b.Args) + 1
	offPos := len(b.Args) + 2
	rowsSQL := fmt.Sprintf(paginatedSQL(b.Where, b.Order), limPos, offPos)
	args := append(append([]any{}, b.Args...), b.Limit, b.Offset)
	rows, err := r.pool.Query(ctx, rowsSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AiProvider
	for rows.Next() {
		p, err := scanRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.pool.QueryRow(ctx, countSQL(b.Where), b.Args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []AiProvider{}
	}
	return out, total, nil
}
```

Add: `List(ctx) ([]AiProvider, error)` (maps `ListAiProviders`), `GetByID(ctx, id) (AiProvider, error)` (maps `GetAiProviderByID`; `pgx.ErrNoRows`/scan failure → `ErrNotFound`), `DemoteDefaults(ctx) error`, `Insert(ctx, NewProvider) (AiProvider, error)`, `Update(ctx, id, ProviderPatch) (AiProvider, error)` (build a dynamic `UPDATE … SET …` from the non-nil patch fields via pool, RETURNING all columns; reuse `scanRow`), `SoftDelete(ctx, id) error`. Define `scanRow(row pgx.Row) (AiProvider, error)` mapping the 11 columns (temperature via `pgtype.Numeric` → `float64`, or string per Task 1). Define `NewProvider` and `ProviderPatch` input structs in `domain.go` (pointers for optional patch fields).

- [ ] **Step 6: Run test, verify pass + build**

Run: `go test ./internal/aiprovider/ -run TestPaginatedSQL && go build ./...`
Expected: PASS, build clean.

- [ ] **Step 7: Commit**

```bash
git add internal/shared/pg/queries_aiprovider.sql internal/shared/pg/sqlc internal/aiprovider/repo_pg.go internal/aiprovider/repo_pg_test.go internal/aiprovider/domain.go
git commit -m "feat(aiprovider): sqlc queries + repo (static + dynamic paginated)"
```

---

### Task 3: aiprovider usecase (Service + ports)

**Files:**
- Create: `internal/aiprovider/usecase.go`
- Test: `internal/aiprovider/usecase_test.go`

The Service owns its ports: `Repo` (the methods from Task 2) and `CompleterFactory` (builds an `aicall.Completer` from a stored provider row — injected from main.go; tested with a fake). Single-default invariant: Create/Update with `is_default=true` calls `Repo.DemoteDefaults` first (Node demotes all then inserts/updates).

- [ ] **Step 1: Write the failing test**

```go
package aiprovider

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	demoted  bool
	inserted NewProvider
	provider *AiProvider
}

func (f *fakeRepo) List(context.Context) ([]AiProvider, error)                  { return nil, nil }
func (f *fakeRepo) ListPaginated(context.Context, any) ([]AiProvider, int, error) { return nil, 0, nil }
func (f *fakeRepo) GetByID(_ context.Context, id string) (AiProvider, error) {
	if f.provider == nil {
		return AiProvider{}, ErrNotFound
	}
	return *f.provider, nil
}
func (f *fakeRepo) DemoteDefaults(context.Context) error { f.demoted = true; return nil }
func (f *fakeRepo) Insert(_ context.Context, in NewProvider) (AiProvider, error) {
	f.inserted = in
	return AiProvider{ID: "new", Name: in.Name, IsDefault: in.IsDefault}, nil
}
func (f *fakeRepo) Update(context.Context, string, ProviderPatch) (AiProvider, error) {
	return AiProvider{ID: "u"}, nil
}
func (f *fakeRepo) SoftDelete(context.Context, string) error { return nil }

type fakeCompleter struct{ text string; err error }
func (f fakeCompleter) Complete(context.Context, string, string) (string, int64, error) {
	return f.text, 42, f.err
}
func (f fakeCompleter) Name() string { return "fake" }

type fakeFactory struct{ c Completer; err error }
func (f fakeFactory) For(AiProvider) (Completer, error) { return f.c, f.err }

func TestCreate_DemotesWhenDefault(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, fakeFactory{})
	_, err := svc.Create(context.Background(), NewProvider{Name: "X", Type: "openai", Model: "m", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.demoted {
		t.Fatal("is_default=true must demote prior defaults before insert")
	}
}

func TestTest_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{provider: nil}, fakeFactory{})
	res := svc.Test(context.Background(), "missing")
	if res.Success || res.Error != "Provider not found" {
		t.Fatalf("not-found result mismatch: %+v", res)
	}
}

func TestTest_Success(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{text: "ok"}})
	res := svc.Test(context.Background(), "1")
	if !res.Success || res.Response != "ok" || res.ResponseTimeMs != 42 {
		t.Fatalf("success result mismatch: %+v", res)
	}
}

func TestTest_CompleterError(t *testing.T) {
	p := AiProvider{ID: "1", Type: "openai"}
	svc := NewService(&fakeRepo{provider: &p}, fakeFactory{c: fakeCompleter{err: errors.New("boom")}})
	res := svc.Test(context.Background(), "1")
	if res.Success || res.Error != "boom" {
		t.Fatalf("error result mismatch: %+v", res)
	}
}
```

(Adjust `ListPaginated` fake signature to the real `listquery.Built` type once written; the test above stubs it as `any` only to compile the fake — use the real type.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/aiprovider/ -run 'TestCreate_Demotes|TestTest_'`
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: Write `usecase.go`**

```go
package aiprovider

import (
	"context"

	"github.com/tannpv/draftright-rewrite/internal/shared/listquery"
)

// Completer is the blocking provider call (satisfied by aicall.Completer).
type Completer interface {
	Complete(ctx context.Context, system, user string) (text string, ms int64, err error)
	Name() string
}

// CompleterFactory builds a Completer from a stored provider row. Injected
// from main.go; lets the test route call the real adapter without this module
// importing rewrite/adapter concretes.
type CompleterFactory interface {
	For(p AiProvider) (Completer, error)
}

// Repo is the persistence port.
type Repo interface {
	List(ctx context.Context) ([]AiProvider, error)
	ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error)
	GetByID(ctx context.Context, id string) (AiProvider, error)
	DemoteDefaults(ctx context.Context) error
	Insert(ctx context.Context, in NewProvider) (AiProvider, error)
	Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error)
	SoftDelete(ctx context.Context, id string) error
}

// TestResult is the POST /:id/test response.
type TestResult struct {
	Success        bool
	Response       string
	ResponseTimeMs int64
	Error          string
}

type Service struct {
	repo    Repo
	factory CompleterFactory
}

func NewService(repo Repo, factory CompleterFactory) *Service {
	return &Service{repo: repo, factory: factory}
}

const (
	testSystemPrompt = "Rewrite this text to be more concise."
	testUserPrompt   = "This is a test sentence to verify the connection works properly."
)

func (s *Service) List(ctx context.Context) ([]AiProvider, error) { return s.repo.List(ctx) }

func (s *Service) ListPaginated(ctx context.Context, b listquery.Built) ([]AiProvider, int, error) {
	return s.repo.ListPaginated(ctx, b)
}

func (s *Service) Create(ctx context.Context, in NewProvider) (AiProvider, error) {
	if in.IsDefault {
		if err := s.repo.DemoteDefaults(ctx); err != nil {
			return AiProvider{}, err
		}
	}
	return s.repo.Insert(ctx, in)
}

func (s *Service) Update(ctx context.Context, id string, p ProviderPatch) (AiProvider, error) {
	if p.IsDefault != nil && *p.IsDefault {
		if err := s.repo.DemoteDefaults(ctx); err != nil {
			return AiProvider{}, err
		}
	}
	return s.repo.Update(ctx, id, p)
}

func (s *Service) SoftDelete(ctx context.Context, id string) error { return s.repo.SoftDelete(ctx, id) }

// Test loads the provider, builds a completer, and runs the fixed prompts.
func (s *Service) Test(ctx context.Context, id string) TestResult {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return TestResult{Success: false, Error: "Provider not found"}
	}
	c, err := s.factory.For(p)
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	text, ms, err := c.Complete(ctx, testSystemPrompt, testUserPrompt)
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	return TestResult{Success: true, Response: text, ResponseTimeMs: ms}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/aiprovider/ -run 'TestCreate_Demotes|TestTest_'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/aiprovider/usecase.go internal/aiprovider/usecase_test.go
git commit -m "feat(aiprovider): service + ports (single-default invariant, test route)"
```

---

### Task 4: aiprovider handler (6 routes) + listquery config

**Files:**
- Create: `internal/aiprovider/handler.go`
- Test: `internal/aiprovider/handler_test.go` (also defines the package test helpers `decodeBody`, `assertKeyOrder`, `routeWithID`)

Routes: `GET /admin/ai-providers` (`[]AiProvider`), `GET /admin/ai-providers/paginated` (`{rows,total}`), `POST` (full provider, 200 — Node `@Post` default is 200 here unless the controller sets 201; VERIFY: check `admin.controller.ts` create handler for `@HttpCode` — default Nest POST is **201**; AI-provider create has no `@HttpCode` override → **201**. Confirm and pin in the test), `PATCH /:id` (200), `DELETE /:id` (200 `{success:true}`), `POST /:id/test` (`{success,...}`).

paginated listquery config: `searchCols=[]string{"name","type","model"}`, `sortAllow=map[string]string{"name":"name","type":"type","model":"model","is_default":"is_default","is_active":"is_active","created_at":"created_at"}`, `defaultSort="created_at"`, `statusCol="is_active"`.

- [ ] **Step 1: Confirm create status code against Node**

`grep -n "ai-providers" -A3 /opt/openAi/DraftRight/backend/src/admin/admin.controller.ts | grep -i "post\|httpcode"`. Nest `@Post()` with no `@HttpCode` → **201**. Pin the create test to the value Node actually returns.

- [ ] **Step 2: Write the failing handler test**

```go
package aiprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestList_ReturnsArray(t *testing.T) {
	svc := NewService(&fakeRepo{}, fakeFactory{})
	h := NewHandler(svc)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/ai-providers", nil))
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list must serialize as []: %s", rec.Body.String())
	}
}

func TestTestRoute_NotFound(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{provider: nil}, fakeFactory{}))
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodPost, "/admin/ai-providers/x/test", nil), "x")
	h.Test(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "success", "error")
	body := decodeBody(t, raw)
	if body["success"] != false || body["error"] != "Provider not found" {
		t.Fatalf("not-found body=%s", raw)
	}
}

// package test helpers (copied from internal/feedback/handler_test.go)
func decodeBody(t *testing.T, raw string) map[string]any { /* json.Unmarshal */ return nil }
func assertKeyOrder(t *testing.T, raw string, keys ...string) { /* index walk */ }
func routeWithID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
```

(Write the helper bodies in full from the feedback test file — the stubs above are placeholders to show structure; the real file must contain working helpers.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/aiprovider/ -run 'TestList_Returns|TestTestRoute'`
Expected: FAIL — `undefined: NewHandler`.

- [ ] **Step 4: Write `handler.go`**

Implement `Handler{svc *Service}`, `NewHandler(svc)`. Methods:
- `List` → `svc.List`, `WriteJSON(w,200,providers)` (ensure non-nil → `[]`).
- `Paginated` → `q := listquery.Parse(r.URL.Query())`; `b := listquery.Build(q, searchCols, sortAllow, "created_at", "is_active")`; `rows,total := svc.ListPaginated`; `WriteJSON(w,200, paginatedResponse{Rows: rows, Total: total})` where `paginatedResponse` is an ordered struct `{Rows []AiProvider json:"rows"; Total int json:"total"}`.
- `Create` → decode `{name,type,endpoint_url,api_key?,model,temperature?}` into `NewProvider` (default `temperature=0.3` when absent); `svc.Create`; `WriteJSON(w,201,p)` (pin to Node value from Step 1).
- `Update` → `id := chi.URLParam(r,"id")`; decode `ProviderPatch`; `svc.Update`; 200.
- `Delete` → `svc.SoftDelete`; `WriteJSON(w,200, map-free ordered struct{Success bool json:"success"}{true})`.
- `Test` → `svc.Test`; marshal `TestResult` with a `MarshalJSON` that emits `{success,response,response_time_ms}` on success and `{success,error}` on failure (two shapes, key order per spec §4.3).

Define the listquery config as package vars:
```go
var aiSearchCols = []string{"name", "type", "model"}
var aiSortAllow = map[string]string{
	"name": "name", "type": "type", "model": "model",
	"is_default": "is_default", "is_active": "is_active", "created_at": "created_at",
}
```

- [ ] **Step 5: Run test to verify it passes + build**

Run: `go test ./internal/aiprovider/ && go build ./...`
Expected: PASS.

- [ ] **Step 6: Add a listquery-config unit test**

```go
func TestPaginatedConfig_SortAliasAndStatus(t *testing.T) {
	q := listquery.Parse(map[string][]string{"sort_by": {"name"}, "status": {"inactive"}, "search": {"gpt"}})
	b := listquery.Build(q, aiSearchCols, aiSortAllow, "created_at", "is_active")
	if !strings.Contains(b.Order, "name") {
		t.Fatalf("sort_by=name must order by name: %q", b.Order)
	}
	if !strings.Contains(b.Where, "is_active") {
		t.Fatalf("status=inactive must filter is_active: %q", b.Where)
	}
}
```
Run: `go test ./internal/aiprovider/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/aiprovider/handler.go internal/aiprovider/handler_test.go
git commit -m "feat(aiprovider): 6 admin routes + listquery paginated config"
```

---

## Group B — `internal/appsettings` (NEW module, routes 18–20)

### Task 5: appsettings domain struct (full ordered row)

**Files:**
- Create: `internal/appsettings/domain.go`
- Test: `internal/appsettings/domain_test.go`

Node field order (`src/admin/entities/app-settings.entity.ts`), 40 columns IN ORDER: `id, environment, trial_limit, token_expiry_minutes, refresh_token_expiry_days, max_input_length, supported_languages, payment_methods_enabled, stripe_secret_key, stripe_webhook_secret, stripe_mode, paypal_client_id, paypal_client_secret, paypal_mode, momo_partner_code, momo_access_key, momo_secret_key, momo_mode, vietqr_bank_id, vietqr_account_number, vietqr_account_name, casso_api_key, sepay_api_key, sepay_mode, resend_api_key, email_from, google_client_id, google_client_secret, apple_client_id, apple_team_id, apple_key_id, lemonsqueezy_api_key, lemonsqueezy_store_id, lemonsqueezy_webhook_secret, lemonsqueezy_variant_monthly, lemonsqueezy_variant_yearly, client_log_level, updated_at`.

**All secrets returned unmasked** (parity; follow-up §8). All string columns are NOT NULL (have defaults) → plain `string`; only `updated_at` is a timestamp.

- [ ] **Step 1: Write the failing test (full key order)**

```go
package appsettings

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAppSettings_JSONKeyOrder(t *testing.T) {
	s := AppSettings{ID: "1", Environment: "testing", StripeSecretKey: "sk_secret", UpdatedAt: time.Unix(0, 0).UTC()}
	raw, _ := json.Marshal(s)
	order := []string{"id", "environment", "trial_limit", "token_expiry_minutes",
		"refresh_token_expiry_days", "max_input_length", "supported_languages",
		"payment_methods_enabled", "stripe_secret_key", "stripe_webhook_secret",
		"stripe_mode", "paypal_client_id", "paypal_client_secret", "paypal_mode",
		"momo_partner_code", "momo_access_key", "momo_secret_key", "momo_mode",
		"vietqr_bank_id", "vietqr_account_number", "vietqr_account_name",
		"casso_api_key", "sepay_api_key", "sepay_mode", "resend_api_key", "email_from",
		"google_client_id", "google_client_secret", "apple_client_id", "apple_team_id",
		"apple_key_id", "lemonsqueezy_api_key", "lemonsqueezy_store_id",
		"lemonsqueezy_webhook_secret", "lemonsqueezy_variant_monthly",
		"lemonsqueezy_variant_yearly", "client_log_level", "updated_at"}
	prev := -1
	for _, k := range order {
		idx := strings.Index(string(raw), `"`+k+`"`)
		if idx <= prev {
			t.Fatalf("key %q out of order: %s", k, raw)
		}
		prev = idx
	}
	if !strings.Contains(string(raw), `"stripe_secret_key":"sk_secret"`) {
		t.Fatalf("secrets must be unmasked: %s", raw)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL** (`undefined: AppSettings`).

Run: `go test ./internal/appsettings/ -run TestAppSettings_JSONKeyOrder`

- [ ] **Step 3: Write `domain.go`**

Define `AppSettings` with the 38 fields above (int for `trial_limit, token_expiry_minutes, refresh_token_expiry_days, max_input_length`; `string` for the rest; `time.Time` for `UpdatedAt`) and a `MarshalJSON` wrapping an anonymous struct with all 38 `json:"…"` tags IN ORDER, `UpdatedAt` via `shared.ISOMillis`. Also define `Patch` (all pointer fields, same names) for PATCH. Confirm each column's int-vs-string and default by re-reading the Node entity at implementation time; the entity is the authority.

- [ ] **Step 4: Run test, verify PASS.**

- [ ] **Step 5: Commit**

```bash
git add internal/appsettings/domain.go internal/appsettings/domain_test.go
git commit -m "feat(appsettings): full ordered settings struct (secrets unmasked, parity)"
```

---

### Task 6: appsettings repo (get-or-create + patch)

**Files:**
- Create: `internal/shared/pg/queries_appsettings.sql`
- Create: `internal/appsettings/repo_pg.go`
- Test: `internal/appsettings/repo_pg_test.go`

Single-row table. Node `GET` returns the row, creating it with defaults if missing. The DB defaults already populate every column → `INSERT … DEFAULT VALUES RETURNING *` when absent. PATCH builds a dynamic `UPDATE … SET … RETURNING *` from non-nil patch fields (run via pool, like aiprovider).

- [ ] **Step 1: Write the static queries**

```sql
-- name: GetAppSettings :one
SELECT * FROM app_settings ORDER BY updated_at ASC LIMIT 1;

-- name: InsertDefaultAppSettings :one
INSERT INTO app_settings DEFAULT VALUES RETURNING *;
```

(VERIFY the Node `getSettings` selection order against the entity — `SELECT *` returns physical column order, which should equal declaration order; if the captured fixture disagrees, list columns explicitly.)

- [ ] **Step 2: `sqlc generate`, build.**

Run: `sqlc generate && go build ./internal/shared/pg/...`

- [ ] **Step 3: Write the failing repo test** — a `paginated`-free unit test asserting `patchSQL` builds the right `SET` clause:

```go
func TestPatchSQL_OnlyNonNilFields(t *testing.T) {
	set, args := patchSQL(Patch{Environment: ptr("prod"), TrialLimit: ptrInt(5)})
	if set != "environment = $1, trial_limit = $2, updated_at = now()" {
		t.Fatalf("set=%q", set)
	}
	if len(args) != 2 {
		t.Fatalf("args=%v", args)
	}
}
```

- [ ] **Step 4: Run test, verify FAIL.**

- [ ] **Step 5: Write `repo_pg.go`** — `PgRepo{q, pool}`, `GetOrCreate(ctx) (AppSettings, error)` (GetAppSettings; on `pgx.ErrNoRows` → InsertDefaultAppSettings), `Patch(ctx, Patch) (AppSettings, error)` (build dynamic UPDATE from non-nil fields + `updated_at = now()`, RETURNING *, scan via shared `scanSettings`). Implement `patchSQL(p Patch) (set string, args []any)` and `scanSettings(row pgx.Row) (AppSettings, error)`.

- [ ] **Step 6: Run test, verify PASS + build.**

- [ ] **Step 7: Commit**

```bash
git add internal/shared/pg/queries_appsettings.sql internal/shared/pg/sqlc internal/appsettings/repo_pg.go internal/appsettings/repo_pg_test.go
git commit -m "feat(appsettings): get-or-create + dynamic patch repo"
```

---

### Task 7: appsettings usecase (Get / Patch+validation / TestEmail)

**Files:**
- Create: `internal/appsettings/usecase.go`
- Test: `internal/appsettings/usecase_test.go`

Ports: `Repo` (Task 6), `MethodValidator` (`AssertMethodsRegisterable(csv string) error` — satisfied by a thin wrapper over `payment.AssertMethodsRegisterable`), `EmailSender` (`SendRaw(ctx, to, subject, html, label string)` — satisfied by `*email.Service`). PATCH must run `MethodValidator` on `payment_methods_enabled` BEFORE persisting (Node order). TestEmail validates `to` contains `@` else returns the sentinel error.

- [ ] **Step 1: Write the failing test**

```go
package appsettings

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct{ patched Patch }
func (f *fakeRepo) GetOrCreate(context.Context) (AppSettings, error) { return AppSettings{ID: "1"}, nil }
func (f *fakeRepo) Patch(_ context.Context, p Patch) (AppSettings, error) { f.patched = p; return AppSettings{ID: "1"}, nil }

type fakeValidator struct{ err error }
func (f fakeValidator) AssertMethodsRegisterable(string) error { return f.err }

type fakeSender struct{ to, subject string }
func (f *fakeSender) SendRaw(_ context.Context, to, subject, _ , _ string) { f.to = to; f.subject = subject }

func TestPatch_RejectsBadPaymentMethods(t *testing.T) {
	svc := NewService(&fakeRepo{}, fakeValidator{err: errors.New("Cannot enable payment method(s)...")}, &fakeSender{})
	_, err := svc.Patch(context.Background(), Patch{PaymentMethodsEnabled: ptr("bogus")})
	if err == nil {
		t.Fatal("invalid payment_methods_enabled must error before persist")
	}
}

func TestTestEmail_RequiresAt(t *testing.T) {
	svc := NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{})
	if err := svc.SendTestEmail(context.Background(), "no-at-sign"); err == nil || err.Error() != "Valid recipient email required" {
		t.Fatalf("got %v", err)
	}
}

func TestTestEmail_Sends(t *testing.T) {
	snd := &fakeSender{}
	svc := NewService(&fakeRepo{}, fakeValidator{}, snd)
	if err := svc.SendTestEmail(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if snd.to != "a@b.com" || snd.subject != "DraftRight test email" {
		t.Fatalf("sender got to=%q subject=%q", snd.to, snd.subject)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write `usecase.go`**

```go
package appsettings

import (
	"context"
	"errors"
	"strings"
)

type Repo interface {
	GetOrCreate(ctx context.Context) (AppSettings, error)
	Patch(ctx context.Context, p Patch) (AppSettings, error)
}

type MethodValidator interface {
	AssertMethodsRegisterable(csv string) error
}

type EmailSender interface {
	SendRaw(ctx context.Context, to, subject, html, label string)
}

type Service struct {
	repo      Repo
	validator MethodValidator
	sender    EmailSender
}

func NewService(repo Repo, v MethodValidator, s EmailSender) *Service {
	return &Service{repo: repo, validator: v, sender: s}
}

func (s *Service) Get(ctx context.Context) (AppSettings, error) { return s.repo.GetOrCreate(ctx) }

func (s *Service) Patch(ctx context.Context, p Patch) (AppSettings, error) {
	if p.PaymentMethodsEnabled != nil {
		if err := s.validator.AssertMethodsRegisterable(*p.PaymentMethodsEnabled); err != nil {
			return AppSettings{}, err
		}
	}
	return s.repo.Patch(ctx, p)
}

// SendTestEmail mirrors admin.controller.ts sendTestEmail: '@' check then send.
// The HTML body matches email.service.ts sendTestEmail (built by the sender);
// here we pass subject "DraftRight test email" and label "test email".
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	if to == "" || !strings.Contains(to, "@") {
		return errors.New("Valid recipient email required")
	}
	s.sender.SendRaw(ctx, to, "DraftRight test email", testEmailHTML(), "test email")
	return nil
}
```

Implement `testEmailHTML()` to return the exact HTML from `email.service.ts` `sendTestEmail` (the "It works." template). **Parity note:** Node's body interpolates `new Date().toISOString()` at send time — that timestamp is non-deterministic and email is out-of-band (NOT shadow-gated), so an exact-byte match is not required; replicate the structure. If you prefer, add a `SendTest(ctx, to)` method to `email.Service` that owns the exact body and have `EmailSender` expose that instead — cleaner reuse. Decide during implementation; the handler/usecase contract (`@` check, subject, `{sent:true,to}`) is what matters.

- [ ] **Step 4: Run test, verify PASS.**

- [ ] **Step 5: Commit**

```bash
git add internal/appsettings/usecase.go internal/appsettings/usecase_test.go
git commit -m "feat(appsettings): get/patch(validate)/test-email service"
```

---

### Task 8: appsettings handler (3 routes)

**Files:**
- Create: `internal/appsettings/handler.go`
- Test: `internal/appsettings/handler_test.go` (defines package helpers)

Routes: `GET /admin/settings` (200 full row), `PATCH /admin/settings` (200 full row; bad payment methods → 400 `invalid-input` with Node's exact message), `POST /admin/settings/test-email` (body `{to}`; bad → 400 `invalid-input` "Valid recipient email required"; ok → 200 `{sent:true,to:<recipient>}`).

- [ ] **Step 1: Write the failing handler test**

```go
func TestTestEmail_BadRecipient400(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.TestEmail(rec, httptest.NewRequest(http.MethodPost, "/admin/settings/test-email", strings.NewReader(`{"to":"bad"}`)))
	if rec.Code != 400 {
		t.Fatalf("status=%d", rec.Code)
	}
	body := decodeBody(t, rec.Body.String())
	if body["code"] != "invalid-input" || body["error"] != "Valid recipient email required" {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestTestEmail_OK200(t *testing.T) {
	h := NewHandler(NewService(&fakeRepo{}, fakeValidator{}, &fakeSender{}))
	rec := httptest.NewRecorder()
	h.TestEmail(rec, httptest.NewRequest(http.MethodPost, "/admin/settings/test-email", strings.NewReader(`{"to":"a@b.com"}`)))
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "sent", "to")
	body := decodeBody(t, raw)
	if body["sent"] != true || body["to"] != "a@b.com" {
		t.Fatalf("body=%s", raw)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write `handler.go`** — `Handler{svc}`, `NewHandler`. `Get` → `svc.Get`, 200. `Patch` → decode `Patch`, `svc.Patch`; on validator error `WriteError(w,r,"invalid-input", err.Error())`. `TestEmail` → decode `{to}`; `svc.SendTestEmail`; on error `WriteError(w,r,"invalid-input",err.Error())`; on ok `WriteJSON(w,200, struct{Sent bool json:"sent"; To string json:"to"}{true, to})`. Copy the package test helpers into the test file.

- [ ] **Step 4: Run test, verify PASS + build.**

- [ ] **Step 5: Commit**

```bash
git add internal/appsettings/handler.go internal/appsettings/handler_test.go
git commit -m "feat(appsettings): get/patch/test-email handlers"
```

---

## Group C — `internal/plans` admin CRUD (routes 4–7)

### Task 9: plans writer methods (Create/Update/SoftDelete) + paginated repo

**Files:**
- Create: `internal/shared/pg/queries_plans_admin.sql`
- Create: `internal/plans/admin_repo_pg.go`
- Test: `internal/plans/admin_repo_pg_test.go`

`PlanEntity` already exists with correct ordered JSON. Add a paginated lister (dynamic, via pool) + count + create + update + soft-delete.

- [ ] **Step 1: Static queries**

```sql
-- name: ListAllPlans :many
SELECT id, name, daily_limit, price_cents, currency, stripe_price_id,
       trial_days, billing_period, is_active, created_at, updated_at
FROM plans ORDER BY price_cents ASC;

-- name: InsertPlan :one
INSERT INTO plans (name, daily_limit, price_cents, billing_period, currency, trial_days, stripe_price_id)
VALUES ($1,$2,$3,$4,$5,$6,$7)
RETURNING id, name, daily_limit, price_cents, currency, stripe_price_id,
          trial_days, billing_period, is_active, created_at, updated_at;

-- name: SoftDeletePlan :exec
UPDATE plans SET is_active = false WHERE id = $1;
```

(`ListAllPlans` order: VERIFY Node `plansService.findAll()` ORDER BY — match it. The Go `Reader.ListActive` uses cheapest-first; `findAll()` for admin may differ. Pin against the Node service.)

- [ ] **Step 2: `sqlc generate`, build.**

- [ ] **Step 3: Write the failing test** for `plansPaginatedSQL(where, order)` + `plansPatchSQL(PlanPatch)` (same pattern as aiprovider/appsettings).

- [ ] **Step 4: Run test, verify FAIL.**

- [ ] **Step 5: Write `admin_repo_pg.go`** — `AdminRepo{q, pool}` with `ListAll(ctx) ([]PlanEntity, error)`, `ListPaginated(ctx, listquery.Built) ([]PlanEntity, int, error)`, `Create(ctx, NewPlan) (PlanEntity, error)`, `Update(ctx, id, PlanPatch) (PlanEntity, error)` (dynamic UPDATE RETURNING all 11 cols), `SoftDelete(ctx, id) error`. Reuse `shared.ISOMillis` via `PlanEntity.MarshalJSON` (already present). Define `NewPlan`/`PlanPatch` in `domain.go`.

- [ ] **Step 6: Run test, verify PASS + build.**

- [ ] **Step 7: Commit**

```bash
git add internal/shared/pg/queries_plans_admin.sql internal/shared/pg/sqlc internal/plans/admin_repo_pg.go internal/plans/admin_repo_pg_test.go internal/plans/domain.go
git commit -m "feat(plans): admin CRUD repo (list-all/paginated/create/update/soft-delete)"
```

---

### Task 10: plans admin usecase + handler (dual-mode list)

**Files:**
- Create: `internal/plans/admin_usecase.go`
- Create: `internal/plans/admin_handler.go`
- Test: `internal/plans/admin_handler_test.go` (package helpers)

Dual-mode `GET /admin/plans`: unpaginated (`[]PlanEntity`) when the query is empty OR all of `page`,`search`,`status`,`sort_by` are absent; else paginated `{rows,total}`. listquery config: `searchCols=[]string{"name","currency","billing_period"}` (VERIFY Node searchable cols), `sortAllow=map[string]string{"name":"name","price":"price_cents","currency":"currency","billing_period":"billing_period","trial_days":"trial_days","is_active":"is_active","created_at":"created_at"}` (note `price`→`price_cents` alias), `defaultSort="created_at"`, `statusCol="is_active"`. `POST` → **201** full PlanEntity. `PATCH /:id` → 200. `DELETE /:id` → 200 `{success:true}`.

- [ ] **Step 1: Write the failing dual-mode test**

```go
func TestList_UnpaginatedWhenNoParams(t *testing.T) {
	h := NewAdminHandler(NewAdminService(&fakeAdminRepo{all: []PlanEntity{{ID: "p1"}}}))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/plans", nil))
	raw := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(raw, "[") {
		t.Fatalf("no params must return a bare array: %s", raw)
	}
}

func TestList_PaginatedWhenParam(t *testing.T) {
	h := NewAdminHandler(NewAdminService(&fakeAdminRepo{}))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/plans?page=1", nil))
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "rows", "total")
}

func TestCreate_Returns201(t *testing.T) {
	h := NewAdminHandler(NewAdminService(&fakeAdminRepo{}))
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodPost, "/admin/plans",
		strings.NewReader(`{"name":"Pro","daily_limit":100,"price_cents":900,"billing_period":"monthly"}`)))
	if rec.Code != 201 {
		t.Fatalf("create must be 201, got %d", rec.Code)
	}
}
```

(`fakeAdminRepo` satisfies the usecase `AdminRepo` port.)

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write `admin_usecase.go`** — `AdminService` over an `AdminRepo` port mirroring Task 9's methods. Dual-mode decision lives in the handler (it inspects raw query params); the service exposes `ListAll`, `ListPaginated`, `Create`, `Update`, `SoftDelete`.

- [ ] **Step 4: Write `admin_handler.go`** — `List` inspects `r.URL.Query()`: if `page`,`search`,`status`,`sort_by` are ALL absent → `ListAll` → bare array; else parse+build+`ListPaginated` → `{rows,total}` ordered struct. `Create` decodes `NewPlan` (defaults: `currency` nil, `trial_days` from Node default 30 when absent — VERIFY), `WriteJSON(w,201,plan)`. `Update`/`Delete` per spec. Define the listquery config vars `planSearchCols`/`planSortAllow`.

- [ ] **Step 5: Run test, verify PASS + build.**

- [ ] **Step 6: Add the listquery-config test** asserting `sort_by=price` orders by `price_cents`:

```go
func TestPlanConfig_PriceAlias(t *testing.T) {
	b := listquery.Build(listquery.Parse(map[string][]string{"sort_by": {"price"}}), planSearchCols, planSortAllow, "created_at", "is_active")
	if !strings.Contains(b.Order, "price_cents") {
		t.Fatalf("price must alias to price_cents: %q", b.Order)
	}
}
```
Run: `go test ./internal/plans/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/plans/admin_usecase.go internal/plans/admin_handler.go internal/plans/admin_handler_test.go
git commit -m "feat(plans): admin dual-mode list + create(201)/update/soft-delete"
```

---

## Group D — `internal/user` admin (routes 1–3)

### Task 11: full-detail user struct (getUser/update response)

**Files:**
- Create: `internal/user/admin_domain.go`
- Test: `internal/user/admin_domain_test.go`

The slim `user.User` (10 fields) is insufficient — getUser returns the FULL Node user entity. Node order (`src/users/entities/user.entity.ts`), 22 columns: `id, email, password_hash, name, is_active, role, auth_provider, google_id, facebook_id, tiktok_id, apple_id, avatar_url, stripe_customer_id, email_verified, email_verification_code, email_verification_expires, password_reset_code, password_reset_expires, password_reset_attempts, lemonsqueezy_customer_id, created_at, updated_at`.

Nullable columns (`*string`/`*time.Time`): `google_id, facebook_id, tiktok_id, apple_id, avatar_url, stripe_customer_id, email_verification_code, email_verification_expires, password_reset_code, password_reset_expires, lemonsqueezy_customer_id`. **`password_hash` + reset/verification codes are exposed** (parity; follow-up §8).

- [ ] **Step 1: Write the failing test** asserting the 22-key order + that nullable absent fields render `null` + `password_hash` present.

```go
func TestUserDetail_JSONKeyOrder(t *testing.T) {
	d := UserDetail{ID: "1", Email: "a@b.com", PasswordHash: "$2b$10$x", Name: "Tan",
		IsActive: true, Role: "user", AuthProvider: "local", EmailVerified: true,
		PasswordResetAttempts: 0, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC()}
	raw, _ := json.Marshal(d)
	order := []string{"id", "email", "password_hash", "name", "is_active", "role",
		"auth_provider", "google_id", "facebook_id", "tiktok_id", "apple_id",
		"avatar_url", "stripe_customer_id", "email_verified", "email_verification_code",
		"email_verification_expires", "password_reset_code", "password_reset_expires",
		"password_reset_attempts", "lemonsqueezy_customer_id", "created_at", "updated_at"}
	prev := -1
	for _, k := range order {
		idx := strings.Index(string(raw), `"`+k+`"`)
		if idx <= prev {
			t.Fatalf("key %q out of order: %s", k, raw)
		}
		prev = idx
	}
	if !strings.Contains(string(raw), `"google_id":null`) {
		t.Fatalf("absent nullable must be null: %s", raw)
	}
}
```

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write `admin_domain.go`** — `UserDetail` struct with the 22 fields (pointer types for the nullables above) + `MarshalJSON` wrapping an ordered anonymous struct. Timestamps via `shared.ISOMillis`; nullable timestamps via a helper that emits `null` or the ISO string (a `*time.Time` → `*string` conversion in MarshalJSON). Also define `UserPatchAdmin` (`IsActive *bool`, `Role *string`, `Name *string`) for PATCH.

- [ ] **Step 4: Run test, verify PASS.**

- [ ] **Step 5: Commit**

```bash
git add internal/user/admin_domain.go internal/user/admin_domain_test.go
git commit -m "feat(user): full-detail admin user struct (22 cols, parity)"
```

---

### Task 12: user admin repo (listUsers bespoke, getFull, update)

**Files:**
- Create: `internal/shared/pg/queries_users_admin.sql`
- Create: `internal/user/admin_repo_pg.go`
- Test: `internal/user/admin_repo_pg_test.go`

`GET /admin/users` is bespoke (NOT listquery): `page` def 1, `limit` def **20**, `status` (`active|inactive|all`), `sort_by` allow `email,name,role,is_active,created_at` (def `created_at`), `sort_order` def `DESC`. Per-row admin list shape is small (`id,email,name,role,is_active,created_at`) + `plan`/`usage_today` computed by the usecase via ports. getFull selects all 22 columns. Update applies the optional patch.

- [ ] **Step 1: Static + dynamic queries** — `GetUserFull` (`SELECT <22 cols> FROM users WHERE id=$1`), `UpdateUserAdmin` is dynamic (build from patch), and the bespoke list is dynamic (status WHERE + allow-listed ORDER + LIMIT/OFFSET, run via pool). Add a sqlc `GetUserFull :one` and `CountUsers`/`CountUsersByActive` as needed; the list rows query is built in Go.

- [ ] **Step 2: `sqlc generate`, build.**

- [ ] **Step 3: Write the failing test** for `userListSQL(status, sortCol, sortOrder)` asserting: default limit 20 path, `status=active` → `WHERE is_active = true`, `status=all` → no status filter, `sort_by` outside the allow-list falls back to `created_at`, and the allow-list prevents injection (an arbitrary `sort_by` never reaches the SQL).

- [ ] **Step 4: Run test, verify FAIL.**

- [ ] **Step 5: Write `admin_repo_pg.go`** — `AdminRepo{q, pool}`:
  - `ListUsers(ctx, ListUsersParams) ([]UserListRow, int, error)` — `ListUsersParams{Search, Status, SortBy, SortOrder string; Page, Limit int}`; build WHERE from status + ILIKE search (`email`/`name`), ORDER from the allow-list (map `sort_by`→column, default `created_at`), LIMIT/OFFSET. Return rows (`id,email,name,role,is_active,created_at`) + total count.
  - `GetFull(ctx, id) (UserDetail, error)` — maps the 22 cols; `pgx.ErrNoRows` → a `not-found` sentinel.
  - `Update(ctx, id, UserPatchAdmin) error` — dynamic UPDATE.
  Define `UserListRow{ID,Email,Name,Role string; IsActive bool; CreatedAt time.Time}`.

- [ ] **Step 6: Run test, verify PASS + build.**

- [ ] **Step 7: Commit**

```bash
git add internal/shared/pg/queries_users_admin.sql internal/shared/pg/sqlc internal/user/admin_repo_pg.go internal/user/admin_repo_pg_test.go
git commit -m "feat(user): admin repo (bespoke list def-20, get-full, update)"
```

---

### Task 13: user admin usecase + handler (list N+1, composite getUser, update)

**Files:**
- Create: `internal/user/admin_usecase.go`
- Create: `internal/user/admin_handler.go`
- Test: `internal/user/admin_handler_test.go` (package helpers)

Ports (consumer-side, injected from main.go):
- `UsageCounter interface { CountToday(ctx, userID string) (int, error) }` (→ `*usage.Counter`).
- `SubReader interface { ActiveByUser(ctx, userID string) (*AdminSubView, error) }` — for getUser, the FULL nested subscription; for list, only the plan name. Define `AdminSubView` to carry the full Subscription entity fields + nested plan.

**getUser composite** (`{user, subscription, usage_today, recent_usage}`):
- `user` = `UserDetail` (Task 11).
- `subscription` = FULL Node Subscription entity with `plan` nested, `user` relation OMITTED (Node loads `relations:['plan']`). Field order: `id, user_id, plan_id, plan{…PlanEntity…}, status, store_type, store_transaction_id, started_at, expires_at, created_at, updated_at`. `null` when no active sub.
- `usage_today` = `CountToday`.
- `recent_usage` = up to 20 rows `created_at DESC`. Per-row order (UsageLog entity, relations NOT loaded → omitted): `id, user_id, tone, input_length, output_length, ai_provider_id, response_time_ms, created_at`.

**listUsers** per-row output order: `id, email, name, role, is_active, plan, usage_today, created_at`; `plan` = active sub plan name else `"None"`; response `{users:[…],total}`.

- [ ] **Step 1: Add the recent_usage + admin-sub queries** (`internal/shared/pg/queries_users_admin.sql` or a usage admin query): `RecentUsageByUser :many` (`SELECT id,user_id,tone,input_length,output_length,ai_provider_id,response_time_ms,created_at FROM usage_logs WHERE user_id=$1 ORDER BY created_at DESC LIMIT 20`) and `GetActiveSubFullByUser :one` (full subscription cols + a join to plan, or two queries). `sqlc generate`, build.

- [ ] **Step 2: Write the failing handler test** — getUser asserts top-level key order `user, subscription, usage_today, recent_usage`, nested subscription key order, and `subscription:null` when no active sub:

```go
func TestGetUser_CompositeShape(t *testing.T) {
	repo := &fakeAdminUserRepo{detail: UserDetail{ID: "u1", Email: "a@b.com"}}
	svc := NewAdminService(repo, fakeUsage{n: 3}, fakeSub{view: nil}, fakeRecent{rows: nil})
	h := NewAdminHandler(svc)
	rec := httptest.NewRecorder()
	req := routeWithID(httptest.NewRequest(http.MethodGet, "/admin/users/u1", nil), "u1")
	h.GetUser(rec, req)
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "user", "subscription", "usage_today", "recent_usage")
	body := decodeBody(t, raw)
	if body["subscription"] != nil {
		t.Fatalf("no active sub must be null: %s", raw)
	}
	if body["usage_today"] != float64(3) {
		t.Fatalf("usage_today=%v", body["usage_today"])
	}
}

func TestListUsers_Shape(t *testing.T) {
	repo := &fakeAdminUserRepo{list: []UserListRow{{ID: "u1", Email: "a@b.com", Name: "A", Role: "user", IsActive: true}}, total: 1}
	svc := NewAdminService(repo, fakeUsage{n: 5}, fakeSub{planName: ""}, fakeRecent{})
	h := NewAdminHandler(svc)
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "users", "total")
	// per-row plan defaults to "None"
	if !strings.Contains(raw, `"plan":"None"`) {
		t.Fatalf("absent sub → plan None: %s", raw)
	}
}
```

- [ ] **Step 3: Run test, verify FAIL.**

- [ ] **Step 4: Write `admin_usecase.go` + `admin_handler.go`**:
  - usecase `AdminService` with ports above + `AdminRepo` (Task 12) + a `RecentUsageReader` port. Methods: `List(ctx, params) (users []AdminUserRow, total int, err error)` (per row: fetch sub plan name + usage), `Get(ctx, id) (GetUserResponse, error)`, `Update(ctx, id, patch) (UserDetail, error)`.
  - Define ordered response structs: `AdminUserRow` (`id,email,name,role,is_active,plan,usage_today,created_at`), `GetUserResponse` (`User UserDetail json:"user"; Subscription *AdminSubView json:"subscription"; UsageToday int json:"usage_today"; RecentUsage []RecentUsageRow json:"recent_usage"`), `AdminSubView` with ordered MarshalJSON (nested `plan` = `plans.PlanEntity`), `RecentUsageRow` with ordered MarshalJSON.
  - handler `List` (bespoke parse: page def 1, limit def 20, status filter, sort_by allow-list, sort_order def DESC) → `{users,total}`; `GetUser` → composite; `UpdateUser` → decode `UserPatchAdmin` (role only "user"; VERIFY Node UpdateUserDto), `svc.Update`, 200 full `UserDetail`.
  - `recent_usage` empty → `[]` (non-nil slice).

- [ ] **Step 5: Run test, verify PASS + build.**

- [ ] **Step 6: Capture-fixture parity check** — against a seeded local Node instance (or a captured fixture committed under `internal/user/testdata/`), diff one real `GET /admin/users/:id` body byte-for-byte to confirm the nested subscription + recent_usage ordering. Add a `//go:embed testdata/getuser_node.json` golden test if a fixture is available; otherwise document the manual diff in the commit message.

- [ ] **Step 7: Commit**

```bash
git add internal/user/admin_usecase.go internal/user/admin_handler.go internal/user/admin_handler_test.go internal/shared/pg/queries_users_admin.sql internal/shared/pg/sqlc
git commit -m "feat(user): admin list/getUser(composite)/update handlers"
```

---

## Group E — `internal/adminauth` admin-users CRUD (routes 14–17)

### Task 14: admin-users repo (list dual-mode, create, update, soft-delete, dup-check)

**Files:**
- Create: `internal/shared/pg/queries_adminusers.sql`
- Create: `internal/adminauth/admin_users_repo_pg.go`
- Test: `internal/adminauth/admin_users_repo_pg_test.go`

Reuse `adminauth.AdminUser` (8 fields) + `shared.HashPassword`/`VerifyPassword` (bcrypt 10). Response order EXCLUDES `password_hash`: `id, email, name, is_active, role, created_at, updated_at`.

- [ ] **Step 1: Static queries** — `ListAdminUsers :many` (`SELECT id,email,name,is_active,role,created_at,updated_at FROM admin_users ORDER BY created_at DESC` — VERIFY order), `InsertAdminUser :one` (`email,password_hash,name,role` → RETURNING the 7 non-secret cols), `SoftDeleteAdminUser :exec` (`UPDATE … SET is_active=false WHERE id=$1`), `FindAdminByEmailLower` already exists (dup-check). Paginated rows via pool (dynamic).

- [ ] **Step 2: `sqlc generate`, build.**

- [ ] **Step 3: Write the failing test** for `adminUsersPaginatedSQL` + a mapping test ensuring `password_hash` is never selected into the response struct.

- [ ] **Step 4: Run test, verify FAIL.**

- [ ] **Step 5: Write `admin_users_repo_pg.go`** — extend the existing `PgRepo` (or a sibling `AdminUsersRepo`) with `ListAll(ctx) ([]AdminUserOut, error)`, `ListPaginated(ctx, listquery.Built) ([]AdminUserOut, int, error)`, `EmailExists(ctx, emailLower string) (bool, error)`, `Insert(ctx, NewAdminUser) (AdminUserOut, error)`, `Update(ctx, id, AdminUserPatch) (AdminUserOut, error)`, `SoftDelete(ctx, id) error`. Define `AdminUserOut` with ordered MarshalJSON (7 keys, no password_hash) + input structs.

- [ ] **Step 6: Run test, verify PASS + build.**

- [ ] **Step 7: Commit**

```bash
git add internal/shared/pg/queries_adminusers.sql internal/shared/pg/sqlc internal/adminauth/admin_users_repo_pg.go internal/adminauth/admin_users_repo_pg_test.go
git commit -m "feat(adminauth): admin-users repo (list/paginated/create/update/soft-delete)"
```

---

### Task 15: admin-users usecase + handler (dual-mode, 201, bcrypt, dup→400)

**Files:**
- Create: `internal/adminauth/admin_users_usecase.go`
- Create: `internal/adminauth/admin_users_handler.go`
- Test: `internal/adminauth/admin_users_handler_test.go`

`GET /admin/admin-users` dual-mode (legacy array when no params; else `{rows,total}`); listquery config `searchCols=[]string{"name","email","role"}`, `sortAllow=map[string]string{"name":"name","email":"email","role":"role","is_active":"is_active","created_at":"created_at"}`, `defaultSort="created_at"`, `statusCol="is_active"`. `POST` body `{email,password,name,role?(def "admin")}`, dup email → 400 `invalid-input`, bcrypt hash, **201** stripped. `PATCH /:id` `{name?,email?,role?,is_active?,password?}` (re-hash only if password present), 200 stripped. `DELETE /:id` soft → 200 `{success:true}` (no self/last-admin guard — parity).

- [ ] **Step 1: Write the failing test**

```go
func TestCreateAdminUser_201_DupEmail400(t *testing.T) {
	repo := &fakeAdminUsersRepo{}
	h := NewAdminUsersHandler(NewAdminUsersService(repo))
	// happy → 201
	rec := httptest.NewRecorder()
	h.Create(rec, httptest.NewRequest(http.MethodPost, "/admin/admin-users",
		strings.NewReader(`{"email":"x@y.com","password":"pw","name":"X"}`)))
	if rec.Code != 201 {
		t.Fatalf("create must be 201, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Fatalf("password_hash must be stripped: %s", rec.Body.String())
	}
	// dup → 400
	repo.emailExists = true
	rec2 := httptest.NewRecorder()
	h.Create(rec2, httptest.NewRequest(http.MethodPost, "/admin/admin-users",
		strings.NewReader(`{"email":"x@y.com","password":"pw","name":"X"}`)))
	if rec2.Code != 400 || decodeBody(t, rec2.Body.String())["code"] != "invalid-input" {
		t.Fatalf("dup email must be 400 invalid-input: %s", rec2.Body.String())
	}
}

func TestListAdminUsers_DualMode(t *testing.T) {
	h := NewAdminUsersHandler(NewAdminUsersService(&fakeAdminUsersRepo{all: []AdminUserOut{{ID: "a1"}}}))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/admin-users", nil))
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Fatalf("no params → array: %s", rec.Body.String())
	}
	rec2 := httptest.NewRecorder()
	h.List(rec2, httptest.NewRequest(http.MethodGet, "/admin/admin-users?page=1", nil))
	assertKeyOrder(t, rec2.Body.String(), "rows", "total")
}
```

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write usecase + handler** — `AdminUsersService` over the repo port: `Create` checks `EmailExists` (→ `ErrDuplicateEmail` sentinel), hashes via `shared.HashPassword`, defaults `role="admin"`, inserts; `Update` re-hashes only when password provided; `SoftDelete`. Handler maps `ErrDuplicateEmail` → `WriteError(w,r,"invalid-input","Email already exists")` (VERIFY Node's exact message), `Create` → 201, dual-mode `List` (inspect raw params like plans). Strip happens by returning `AdminUserOut` (no password_hash field). Copy package helpers.

- [ ] **Step 4: Run test, verify PASS + build.**

- [ ] **Step 5: listquery-config test** (search/sort/status). Run `go test ./internal/adminauth/`, PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adminauth/admin_users_usecase.go internal/adminauth/admin_users_handler.go internal/adminauth/admin_users_handler_test.go
git commit -m "feat(adminauth): admin-users dual-mode list + create(201)/update/soft-delete"
```

---

## Group F — `internal/email` admin (routes 21–25)

Existing `internal/email/templates.go` already has 6 builtin templates + `renderTemplate`. Reuse them. Builtin keys (VERIFY against `templates.go`): `welcome, password_reset, email_verification, subscription_expiring, subscription_expired, payment_failed` — confirm the exact set + each `label`/`variables`/`default_subject`/`default_html` before coding. The DB table `email_templates` stores customizations keyed by `key`.

### Task 16: email-logs list (bespoke def-50/max-200)

**Files:**
- Create: `internal/shared/pg/queries_emaillogs.sql`
- Create: `internal/email/admin_logs_repo_pg.go`
- Create: `internal/email/admin_logs_usecase.go` + `admin_logs_handler.go`
- Test: `internal/email/admin_logs_handler_test.go`

`GET /admin/email-logs` is bespoke (NOT listquery): `page` def 1, `limit` def **50** cap **200** (jsParseInt), optional `status` + `email_type` filters (VERIFY which filters Node accepts). Row fields IN ORDER: `id, to_email, email_type, subject, status, provider_id, error, created_at`. `provider_id` + `error` nullable. Response `{logs:[…],total}` (VERIFY wrapper key — could be `{logs,total}` or `{rows,total}`; read Node `getEmailLogs`).

- [ ] **Step 1: Queries** — dynamic paginated SELECT via pool (filters + LIMIT/OFFSET) + a count query. Define `EmailLogRow` with ordered MarshalJSON (8 keys, nullable `provider_id`/`error`).

- [ ] **Step 2: Write the failing test** for `emailLogsLimit(raw)` clamp: missing→50, `"10"`→10, `"500"`→200 (cap), `"0"`/`"-1"`→50 (VERIFY Node's floor behavior), `"abc"`→50.

- [ ] **Step 3: Run test, verify FAIL.**

- [ ] **Step 4: Write repo + usecase + handler** — `AdminLogsRepo.List(ctx, params)`, `AdminLogsService.List`, handler parses page/limit/filters → `{logs,total}`. `logs` empty → `[]`.

- [ ] **Step 5: Run test, verify PASS + build.**

- [ ] **Step 6: Commit**

```bash
git add internal/shared/pg/queries_emaillogs.sql internal/shared/pg/sqlc internal/email/admin_logs_repo_pg.go internal/email/admin_logs_usecase.go internal/email/admin_logs_handler.go internal/email/admin_logs_handler_test.go
git commit -m "feat(email): admin email-logs list (bespoke def-50/max-200)"
```

---

### Task 17: email-templates merged list

**Files:**
- Create: `internal/email/admin_templates_repo_pg.go`
- Create: `internal/email/admin_templates_usecase.go` + `admin_templates_handler.go`
- Test: `internal/email/admin_templates_handler_test.go`

`GET /admin/email-templates` merges the 6 builtins with DB customizations. Per-row fields IN ORDER: `key, label, variables, subject, html, customized, default_subject, default_html`. `subject`/`html` = customized value if a DB row exists else the builtin default; `customized` = bool (DB row exists); `default_subject`/`default_html` = builtin. `variables` = builtin's variable list (`[]string`). Response = a bare array (VERIFY: array vs `{templates:[…]}`).

- [ ] **Step 1: Query** — `ListEmailTemplates :many` (`SELECT key, subject, html FROM email_templates` — VERIFY columns). Map into a `map[string]dbTemplate`.

- [ ] **Step 2: `sqlc generate`, build.**

- [ ] **Step 3: Write the failing test** — seed a fake repo with one customization for `welcome`; assert merged output: `welcome.customized==true` + `welcome.subject==<custom>`, and an un-customized key has `customized==false` + `subject==default_subject`. Assert per-row key order.

```go
func TestListTemplates_MergeAndOrder(t *testing.T) {
	repo := &fakeTemplatesRepo{custom: map[string]DBTemplate{"welcome": {Subject: "Hi!", HTML: "<b>hi</b>"}}}
	h := NewAdminTemplatesHandler(NewAdminTemplatesService(repo))
	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/admin/email-templates", nil))
	raw := rec.Body.String()
	assertKeyOrder(t, raw, "key", "label", "variables", "subject", "html", "customized", "default_subject", "default_html")
	if !strings.Contains(raw, `"subject":"Hi!"`) || !strings.Contains(raw, `"customized":true`) {
		t.Fatalf("welcome must reflect customization: %s", raw)
	}
}
```

- [ ] **Step 4: Run test, verify FAIL.**

- [ ] **Step 5: Write repo + usecase + handler** — `AdminTemplatesService.List` iterates the 6 builtins (stable order — VERIFY Node's iteration order, likely the builtin map/array order), overlays DB customizations, builds ordered `TemplateRow` structs.

- [ ] **Step 6: Run test, verify PASS + build.**

- [ ] **Step 7: Commit**

```bash
git add internal/email/admin_templates_repo_pg.go internal/email/admin_templates_usecase.go internal/email/admin_templates_handler.go internal/email/admin_templates_handler_test.go internal/shared/pg/queries_emaillogs.sql internal/shared/pg/sqlc
git commit -m "feat(email): admin email-templates merged list (builtins + customizations)"
```

---

### Task 18: email-template update + reset + preview

**Files:**
- Modify: `internal/shared/pg/queries_emaillogs.sql` (add upsert/delete)
- Modify: `internal/email/admin_templates_repo_pg.go`, `admin_templates_usecase.go`, `admin_templates_handler.go`
- Test: `internal/email/admin_templates_handler_test.go`

- `PATCH /admin/email-templates/:key` — body `{subject?,html?}`; unknown builtin key → **404**; else UPSERT into `email_templates` → `{ok:true}`. VERIFY Node: does it 404 on unknown key or accept any key?
- `DELETE /admin/email-templates/:key` — reset = DELETE the DB row → `{ok:true}` (idempotent; unknown key → VERIFY 404 vs ok).
- `GET /admin/email-templates/:key/preview` — render the (possibly customized) template with sample vars → `{subject,html}`. Sample vars MUST match Node's exactly. Known sample: `expires` = `new Date(Date.now()+30*86400000).toDateString()` → Go must reproduce `toDateString()` format (`"Wed Jul 18 2026"`). This is PRODUCTION code (not a workflow script), so `time.Now()` is allowed — VERIFY which other sample vars Node injects (name, url, code, etc.) by reading `previewTemplate`. Unknown key → 404.

- [ ] **Step 1: Queries** — `UpsertEmailTemplate :exec` (`INSERT … ON CONFLICT (key) DO UPDATE`), `DeleteEmailTemplate :exec`. `sqlc generate`, build.

- [ ] **Step 2: Write the failing tests**

```go
func TestUpdateTemplate_UnknownKey404(t *testing.T) {
	h := NewAdminTemplatesHandler(NewAdminTemplatesService(&fakeTemplatesRepo{}))
	rec := httptest.NewRecorder()
	req := routeWithKey(httptest.NewRequest(http.MethodPatch, "/admin/email-templates/nope",
		strings.NewReader(`{"subject":"x"}`)), "nope")
	h.Update(rec, req)
	if rec.Code != 404 {
		t.Fatalf("unknown key must 404, got %d", rec.Code)
	}
}

func TestUpdateTemplate_OK(t *testing.T) {
	repo := &fakeTemplatesRepo{}
	h := NewAdminTemplatesHandler(NewAdminTemplatesService(repo))
	rec := httptest.NewRecorder()
	req := routeWithKey(httptest.NewRequest(http.MethodPatch, "/admin/email-templates/welcome",
		strings.NewReader(`{"subject":"Hi","html":"<b>x</b>"}`)), "welcome")
	h.Update(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("update welcome → 200 {ok:true}: %d %s", rec.Code, rec.Body.String())
	}
	if !repo.upserted {
		t.Fatalf("must upsert")
	}
}

func TestPreviewTemplate_RendersSubjectHTML(t *testing.T) {
	h := NewAdminTemplatesHandler(NewAdminTemplatesService(&fakeTemplatesRepo{}))
	rec := httptest.NewRecorder()
	req := routeWithKey(httptest.NewRequest(http.MethodGet, "/admin/email-templates/welcome/preview", nil), "welcome")
	h.Preview(rec, req)
	assertKeyOrder(t, rec.Body.String(), "subject", "html")
}
```

- [ ] **Step 3: Run tests, verify FAIL.**

- [ ] **Step 4: Implement** — `Update`/`Reset`/`Preview` on service + handler; validate key against the builtin set (404 sentinel → `WriteError(w,r,"not-found",…)` — VERIFY Node's exact code/message); preview reuses `renderTemplate` with the sample-var map.

- [ ] **Step 5: Run tests, verify PASS + build.**

- [ ] **Step 6: Commit**

```bash
git add internal/email/admin_templates_repo_pg.go internal/email/admin_templates_usecase.go internal/email/admin_templates_handler.go internal/email/admin_templates_handler_test.go internal/shared/pg/queries_emaillogs.sql internal/shared/pg/sqlc
git commit -m "feat(email): admin template update(404-guard)/reset/preview"
```

---

## Group G — Wiring, router, integration, gate, final review

### Task 19: completer factory (provider → aicall.Completer)

**Files:**
- Create: `internal/aiprovider/completer_factory.go`
- Test: `internal/aiprovider/completer_factory_test.go`

The ai-provider `:id/test` route needs a live `aicall.Completer` built from a stored `AiProvider` row. Mirror Node `callProvider` dispatch: `type` → adapter. Constructors confirmed: `openai.New(id,key)`, `anthropic.New(id,key)`, `ollama.New(id, ollama.WithEndpoint(url))`. Define a factory satisfying aiprovider's `CompleterFactory` port (Task 3).

- [ ] **Step 1: Write the failing test** — `BuildCompleter(AiProvider{Type:"ollama",EndpointURL:"http://x"})` returns non-nil + no error; unknown type → a sentinel error (VERIFY Node's behavior for unknown type — error vs default).

- [ ] **Step 2: Run test, verify FAIL.**

- [ ] **Step 3: Write `completer_factory.go`** — `type Factory struct{}`; `func (Factory) Build(p AiProvider) (aicall.Completer, error)` switching on `p.Type` ∈ `{openai,anthropic,ollama,custom}`. `custom` → VERIFY Node (likely OpenAI-compatible via the openai adapter with an endpoint override — confirm the openai adapter supports a custom endpoint; if not, document the gap). model/temperature passed through per adapter API.

- [ ] **Step 4: Run test, verify PASS + build.**

- [ ] **Step 5: Commit**

```bash
git add internal/aiprovider/completer_factory.go internal/aiprovider/completer_factory_test.go
git commit -m "feat(aiprovider): completer factory (type → aicall adapter, parity callProvider)"
```

---

### Task 20: wire all handlers in `cmd/server/main.go`

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/shared/router.go` (add handler fields)

Construct every new repo→service→handler and inject cross-module ports, REUSING existing shared collaborators (one `usage.Counter`, one `subscription` reader, one `payment` registry, one `email` sender — do NOT fork). main.go is the only place naming concrete constructors.

Wiring checklist (VERIFY each constructor signature against the existing modules before writing):
- `aiprovider`: repo(pool,q) → service(repo, `aiprovider.Factory{}`) → handler. Factory from Task 19.
- `appsettings`: repo(pool,q) → service(repo, `payment.MethodValidator` adapter wrapping `payment.AssertMethodsRegisterable`, `email` sender adapter wrapping `email.SendRaw`) → handler.
- `user` admin: adminRepo(pool,q) → adminService(adminRepo, `usage.Counter`, subReader adapter over `subscription.Reader`, recentUsageReader) → adminHandler.
- `adminauth` admin-users: reuse/extend existing adminauth repo → service → handler.
- `email` admin: logs repo+service+handler; templates repo+service+handler (inject builtins from the `email` package).
- `plans` admin: adminRepo → adminService → adminHandler.

- [ ] **Step 1: Add handler fields** to the router struct in `internal/shared/router.go` (e.g. `AiProvider *aiprovider.Handler`, `AppSettings *appsettings.Handler`, `UserAdmin *user.AdminHandler`, `AdminUsers *adminauth.AdminUsersHandler`, `EmailLogs`, `EmailTemplates`, `PlansAdmin`). Match the existing `coreHandlers`/router field naming convention exactly.

- [ ] **Step 2: Construct + assign** all of the above in `main.go`, after the existing module construction, reusing the already-built `usage.Counter`, `subscription` reader, `payment` registry, `email` sender, pool, and sqlc `q`.

- [ ] **Step 3: Build** — `go build ./...`. Expected: compiles. Struct-field assignment leaves no unused locals; if a local is flagged, assign it to a field this task (mounting happens in Task 21).

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go internal/shared/router.go
git commit -m "chore(server): construct + inject all 4c-2 admin handlers"
```

---

### Task 21: mount all 25 routes + router integration tests

**Files:**
- Modify: `internal/shared/router.go` (mount routes in the admin group)
- Test: `internal/shared/router_admin4c2_test.go`

Mount all 25 routes inside the existing admin group (`jwtMW → RequireAdmin`) created in 4c-1. Exact paths + methods from the spec §2 inventory (routes 1–25). `:id`/`:key` are chi URL params.

- [ ] **Step 1: Write the failing test** — table-driven over all 25 (method,path): no JWT → 401; valid non-admin JWT → 403. Reuse the 4c-1 admin-group test helpers (mint a test JWT).

```go
func TestAdmin4c2Routes_AuthGuards(t *testing.T) {
	r := NewRouter(testRouterDeps(t)) // same helper 4c-1 used
	cases := []struct{ method, path string }{
		{"GET", "/admin/users"}, {"GET", "/admin/users/u1"}, {"PATCH", "/admin/users/u1"},
		{"GET", "/admin/plans"}, {"POST", "/admin/plans"}, {"PATCH", "/admin/plans/p1"}, {"DELETE", "/admin/plans/p1"},
		{"GET", "/admin/ai-providers"}, {"GET", "/admin/ai-providers/paginated"}, {"POST", "/admin/ai-providers"},
		{"PATCH", "/admin/ai-providers/a1"}, {"DELETE", "/admin/ai-providers/a1"}, {"POST", "/admin/ai-providers/a1/test"},
		{"GET", "/admin/admin-users"}, {"POST", "/admin/admin-users"}, {"PATCH", "/admin/admin-users/a1"}, {"DELETE", "/admin/admin-users/a1"},
		{"GET", "/admin/settings"}, {"PATCH", "/admin/settings"}, {"POST", "/admin/settings/test-email"},
		{"GET", "/admin/email-logs"}, {"GET", "/admin/email-templates"}, {"PATCH", "/admin/email-templates/welcome"},
		{"DELETE", "/admin/email-templates/welcome"}, {"GET", "/admin/email-templates/welcome/preview"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != 401 {
			t.Errorf("%s %s no-JWT want 401 got %d", c.method, c.path, rec.Code)
		}
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(c.method, c.path, nil)
		req2.Header.Set("Authorization", "Bearer "+nonAdminToken(t))
		r.ServeHTTP(rec2, req2)
		if rec2.Code != 403 {
			t.Errorf("%s %s non-admin want 403 got %d", c.method, c.path, rec2.Code)
		}
	}
}
```

- [ ] **Step 2: Run test, verify FAIL** (routes 404, not 401/403, until mounted).

- [ ] **Step 3: Mount all 25 routes** in the admin group, calling the handler methods wired in Task 20.

- [ ] **Step 4: Run test, verify PASS + build.**

- [ ] **Step 5: Commit**

```bash
git add internal/shared/router.go internal/shared/router_admin4c2_test.go
git commit -m "feat(server): mount 25 admin 4c-2 routes + auth-guard integration tests"
```

---

### Task 22: full gate

**Files:** none (verification only).

- [ ] **Step 1: Build** — `go build ./...`. Expected: no output, exit 0.
- [ ] **Step 2: Vet** — `go vet ./...`. Expected: clean.
- [ ] **Step 3: Format** — `gofmt -l .`. Expected: prints nothing. If any file listed → `gofmt -w` it + amend the owning commit.
- [ ] **Step 4: Test+race** — `go test ./... -race`. Expected: all PASS.
- [ ] **Step 5: sqlc drift** — `sqlc generate` then `git status --porcelain internal/shared/pg/sqlc`. Expected: no diff.
- [ ] **Step 6:** If anything fails, fix in the owning module's task scope, re-run. Do NOT leave a broken gate.

(No commit — verification gate.)

---

### Task 23: final whole-branch review

**Files:** none (review only).

- [ ] **Step 1: Parity diff sweep** — for each of the 25 routes, open the Node `AdminController` handler + the Go handler side by side; confirm route, method, status code, request body fields, response JSON key order, and error envelope code/message match byte-for-byte. Record any drift.
- [ ] **Step 2: Secret-exposure parity confirm** — verify (NOT fix) that `ai_providers.api_key`, settings secrets, and getUser `password_hash` are returned UNMASKED, matching Node. These 4 §8 follow-ups STAY unmasked in 4c-2 (masking breaks byte-parity). Note them for separate GitHub issues.
- [ ] **Step 3: listquery consumer check** — confirm plans/ai-providers-paginated/admin-users use `listquery.Build` with correct searchCols/sortAllow/statusCol, and users + email-logs do NOT (bespoke). Confirm `{rows,total}` wrapper on the paginated branches.
- [ ] **Step 4: Placeholder scan** — grep new code for `TODO`/`FIXME`/stub returns. Resolve or document.
- [ ] **Step 5:** Dispatch a final code-reviewer subagent over the whole branch diff (`git diff develop...HEAD`) for a fresh-eyes pass. Address findings.

(No commit — review gate. After approval → `superpowers:finishing-a-development-branch`.)

---

## Post-plan follow-ups (file as GitHub issues when asked — do NOT fix in 4c-2)

Per spec §8, these 4 break byte-parity if fixed now, so they ride a later hardening pass:
1. `ai_providers.api_key` returned plaintext on READ.
2. `app_settings` secret fields (payment keys, SMTP password) returned unmasked.
3. `GET /admin/users/:id` exposes `password_hash` + reset/verification codes.
4. admin-user soft-delete has no self-delete / last-admin guard.
