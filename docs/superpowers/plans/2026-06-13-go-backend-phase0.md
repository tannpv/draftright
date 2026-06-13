# Go Backend Port — Phase 0 (Parity Foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the existing layer-first Go `rewrite` service into a modular package-by-feature layout, close the measured parity gaps versus the Node backend, and build a shadow-compare replay tool that proves `GET /health` and `GET /auth/me` are byte-identical to Node before any path is flipped.

**Architecture:** Modular monolith with clean architecture per module. Top level is sliced by feature (`internal/rewrite/`, `internal/core/`), not by layer. Shared infrastructure lives in `internal/platform/` (config, DB, JWT, logger, metrics, tracing) and cross-cutting app concerns in `internal/shared/` (error envelope, request-id middleware, auth middleware, sqlc-generated DB types). `cmd/server/main.go` is the single composition root. A standalone `cmd/shadowdiff` replays fixtures against Node + Go and diffs the responses.

**Tech Stack:** Go 1.25, chi/v5 router, pgx/v5 + sqlc, golang-jwt/v5 (HS256), Go stdlib `testing` + testify, slog. Module path `github.com/tannpv/draftright-rewrite`, working dir `backend-rewrite-go/`.

---

## Working directory

Every command below runs from `/opt/openAi/DraftRight/backend-rewrite-go` unless stated otherwise. Branch: `feature/go-backend-phase0-spec-20260613` (continue on it, or branch a `feature/go-phase0-impl-<date>` from it).

## File Structure (target after Phase 0)

```
backend-rewrite-go/
├── cmd/
│   ├── server/main.go              # composition root — wires rewrite + core handlers
│   └── shadowdiff/main.go          # NEW — fixture replay + diff tool
├── internal/
│   ├── rewrite/                    # MOVED from layer-first — the module template
│   │   ├── domain/                 #   (was internal/domain) tone, rewrite_request, user, ports, errors, metrics
│   │   ├── usecase/                #   (was internal/usecase) rewrite.go
│   │   ├── transport/              #   (was internal/http handler_rewrite.go, sse.go)
│   │   └── adapter/                #   (was internal/adapter) anthropic openai ollama chain memory pg
│   ├── core/                       # NEW — Phase 0 proof endpoints (health + me)
│   │   ├── health.go               #   GET /health handler
│   │   ├── health_test.go
│   │   ├── me.go                   #   GET /auth/me handler
│   │   └── me_test.go
│   ├── shared/                     # NEW — cross-module app concerns
│   │   ├── errenvelope.go          #   {error, code, request_id} writer + status map
│   │   ├── errenvelope_test.go
│   │   ├── requestid.go            #   request-id middleware (+ getter)
│   │   ├── requestid_test.go
│   │   ├── auth_middleware.go      #   (moved from internal/http/middleware_auth.go) RequireAuth + ClaimsFromContext
│   │   ├── router.go               #   (moved from internal/http/router.go) mux build + access log
│   │   └── pg/sqlc/                #   (moved from internal/adapter/pg/sqlc) generated DB types — shared
│   └── platform/                   # UNCHANGED — config db auth logger metrics tracing redis
│       ├── auth/jwt.go             #   + Email claim + Signer (Task 4)
│       └── ...
├── internal/adapter/pg/queries.sql # + Phase 0 queries (FindUserByID, GetClientLogLevel) (Task 2)
└── fixtures/                       # NEW — shadowdiff request fixtures
    ├── health.json
    └── auth_me.json
```

**Note on the rewrite move:** the existing `internal/adapter/pg/sqlc` (generated DB types) is shared across modules (the master design mandates one sqlc package over the whole schema), so it moves to `internal/shared/pg/sqlc`, NOT under `internal/rewrite/`. The rewrite-specific Postgres adapter (`user_repo.go`) stays in the rewrite module and imports the shared sqlc package.

---

## Task 1: Restructure to modular layout (mechanical, behavior-preserving)

The existing `rewrite` service has full test coverage. The move is pure — no logic changes. **The existing test suite passing unchanged after the move is the proof of correctness.** Do the move in small, independently-buildable commits so a mistake is easy to bisect.

**Files:**
- Move (git mv): `internal/domain/*` → `internal/rewrite/domain/`, `internal/usecase/*` → `internal/rewrite/usecase/`, `internal/http/{handler_rewrite*.go,sse.go,log_context.go}` → `internal/rewrite/transport/`, `internal/adapter/{anthropic,openai,ollama,chain,memory}` → `internal/rewrite/adapter/`, `internal/adapter/pg/user_repo.go` → `internal/rewrite/adapter/pg/user_repo.go`
- Move (git mv): `internal/adapter/pg/sqlc` → `internal/shared/pg/sqlc`, `internal/http/middleware_auth.go` → `internal/shared/auth_middleware.go`, `internal/http/router.go` → `internal/shared/router.go`
- Modify: every moved file's `package` clause + every importer's import paths; `cmd/server/main.go`; `sqlc.yaml` (out path)

- [ ] **Step 1: Confirm green baseline before touching anything**

Run: `go build ./... && go test ./...`
Expected: build OK, all tests PASS. Record the test count (e.g. "ok ... rewrite/usecase"). If anything is red here, STOP and fix before restructuring — you need a known-green starting point.

- [ ] **Step 2: Move the shared sqlc package first (lowest fan-in to fix)**

```bash
git mv internal/adapter/pg/sqlc internal/shared/pg/sqlc
# fix the generated package's own import references (none expected) + all importers
grep -rl 'internal/adapter/pg/sqlc' --include='*.go' . \
  | xargs sed -i '' 's#internal/adapter/pg/sqlc#internal/shared/pg/sqlc#g'
# point sqlc generation at the new location
sed -i '' 's#out: "internal/adapter/pg/sqlc"#out: "internal/shared/pg/sqlc"#' sqlc.yaml
```

- [ ] **Step 3: Build to confirm the sqlc move compiles**

Run: `go build ./...`
Expected: build OK. (Only import paths changed; the `sqlc` package name is unchanged.)

- [ ] **Step 4: Commit the sqlc move**

```bash
git add -A && git commit -m "refactor(go): move generated sqlc to internal/shared/pg/sqlc

The sqlc-generated DB types are shared across modules (one package over
the whole schema), so they belong in shared/, not under any single
feature module. Pure move + import-path update; no behavior change."
```

- [ ] **Step 5: Move auth middleware + router into shared, rename package**

```bash
git mv internal/http/middleware_auth.go internal/shared/auth_middleware.go
git mv internal/http/router.go internal/shared/router.go
# rename the package clause on the two moved files: package http -> package shared
sed -i '' 's/^package http$/package shared/' internal/shared/auth_middleware.go internal/shared/router.go
```

The other files still in `internal/http/` (`handler_rewrite.go`, `sse.go`, `log_context.go`, `errors.go`, `*_test.go`) move with the rewrite module in Step 8 — leave them for now; the package won't build until then, which is fine (we commit per logical move only after each builds; this step's build is deferred to Step 9).

- [ ] **Step 6: Move the rewrite domain/usecase/adapter trees**

```bash
mkdir -p internal/rewrite
git mv internal/domain internal/rewrite/domain
git mv internal/usecase internal/rewrite/usecase
mkdir -p internal/rewrite/adapter
git mv internal/adapter/anthropic internal/rewrite/adapter/anthropic
git mv internal/adapter/openai    internal/rewrite/adapter/openai
git mv internal/adapter/ollama    internal/rewrite/adapter/ollama
git mv internal/adapter/chain     internal/rewrite/adapter/chain
git mv internal/adapter/memory    internal/rewrite/adapter/memory
mkdir -p internal/rewrite/adapter/pg
git mv internal/adapter/pg/user_repo.go internal/rewrite/adapter/pg/user_repo.go
git mv internal/adapter/pg/queries.sql  internal/rewrite/adapter/pg/queries.sql
```

- [ ] **Step 7: Move the rewrite transport files**

```bash
mkdir -p internal/rewrite/transport
git mv internal/http/handler_rewrite.go      internal/rewrite/transport/handler_rewrite.go
git mv internal/http/handler_rewrite_test.go internal/rewrite/transport/handler_rewrite_test.go
git mv internal/http/sse.go                  internal/rewrite/transport/sse.go
git mv internal/http/log_context.go          internal/rewrite/transport/log_context.go
git mv internal/http/errors.go               internal/rewrite/transport/errors.go
# router observability test references router; it moves to shared
git mv internal/http/router_observability_test.go internal/shared/router_observability_test.go
# the internal/http dir should now be empty
rmdir internal/http 2>/dev/null || true
```

- [ ] **Step 8: Rewrite import paths + package clauses across the moved trees**

```bash
# import-path rewrites (longest-prefix first to avoid partial overlaps)
grep -rl --include='*.go' 'internal/domain'  . | xargs sed -i '' 's#internal/domain#internal/rewrite/domain#g'
grep -rl --include='*.go' 'internal/usecase' . | xargs sed -i '' 's#internal/usecase#internal/rewrite/usecase#g'
grep -rl --include='*.go' 'internal/adapter/anthropic' . | xargs sed -i '' 's#internal/adapter/anthropic#internal/rewrite/adapter/anthropic#g'
grep -rl --include='*.go' 'internal/adapter/openai'    . | xargs sed -i '' 's#internal/adapter/openai#internal/rewrite/adapter/openai#g'
grep -rl --include='*.go' 'internal/adapter/ollama'    . | xargs sed -i '' 's#internal/adapter/ollama#internal/rewrite/adapter/ollama#g'
grep -rl --include='*.go' 'internal/adapter/chain'     . | xargs sed -i '' 's#internal/adapter/chain#internal/rewrite/adapter/chain#g'
grep -rl --include='*.go' 'internal/adapter/memory'    . | xargs sed -i '' 's#internal/adapter/memory#internal/rewrite/adapter/memory#g'
grep -rl --include='*.go' '"github.com/tannpv/draftright-rewrite/internal/adapter/pg"' . | xargs sed -i '' 's#internal/adapter/pg"#internal/rewrite/adapter/pg"#g'
# package clauses for the moved transport files: package http -> package transport
sed -i '' 's/^package http$/package transport/' internal/rewrite/transport/*.go
```

The transport files reference unexported identifiers that lived in the old `package http` (e.g. `writeJSON`, `withRequestLogger`, `structuredLogger`, `RequireAuth`, `ClaimsFromContext`). After the split, `RequireAuth`/`ClaimsFromContext`/`structuredLogger`/`withRequestLogger` live in `package shared` and `writeJSON` is needed by both. Resolve in the next step.

- [ ] **Step 9: Fix cross-package references exposed by the split**

The split moved `RequireAuth`, `ClaimsFromContext`, `structuredLogger`, `withRequestLogger` to `package shared` but the rewrite transport handler used them as same-package identifiers. Two concrete fixes:

1. **`writeJSON` helper** — it was in the old `package http`. Move it to `internal/shared/render.go` as an exported `shared.WriteJSON`, then update both shared and transport callers.

Create `internal/shared/render.go`:

```go
package shared

import (
	"encoding/json"
	"net/http"
)

// WriteJSON serialises v as the response body with the given status.
// Single helper so every handler — across modules — emits the same
// content type + encoding (Rule #1: one place owns the wire write).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

2. **`ClaimsFromContext`** — the rewrite transport handler reads claims via `ClaimsFromContext`. It now lives in `package shared`. Update the rewrite handler to call `shared.ClaimsFromContext(...)` and import `internal/shared`.

```bash
# repoint writeJSON(...) -> shared.WriteJSON(...) in transport files, add shared import via goimports
grep -rl --include='*.go' 'writeJSON(' internal/rewrite/transport internal/shared \
  | xargs sed -i '' 's/\bwriteJSON(/shared.WriteJSON(/g'
# in shared package files, the call is same-package: undo the prefix there
sed -i '' 's/shared\.WriteJSON(/WriteJSON(/g' internal/shared/*.go
```

Then delete the now-duplicate `writeJSON` definition wherever it still exists (search: `grep -rn 'func writeJSON' internal/`), and add the missing imports with `goimports -w internal/`.

- [ ] **Step 10: Update the composition root (`cmd/server/main.go`)**

The router type moved from `internalhttp.Router` to `shared.Router`, and the rewrite handler from `internalhttp.RewriteHandler` to `transport.RewriteHandler`. Update imports + references:

```bash
sed -i '' \
  -e 's#internalhttp "github.com/tannpv/draftright-rewrite/internal/http"#"github.com/tannpv/draftright-rewrite/internal/shared"#' \
  -e 's#internalhttp\.Router#shared.Router#g' \
  -e 's#internalhttp\.RewriteHandler#transport.RewriteHandler#g' \
  cmd/server/main.go
```

Then add the rewrite transport import (`"github.com/tannpv/draftright-rewrite/internal/rewrite/transport"`) and the other moved package paths via `goimports -w cmd/server/main.go`. The `shared.Router` struct field that holds the rewrite handler is `Rewrite *transport.RewriteHandler` — update `internal/shared/router.go`'s import + field type accordingly.

- [ ] **Step 11: Compile the whole tree**

Run: `goimports -w ./... && go build ./...`
Expected: build OK. If unresolved identifiers remain, they are almost always (a) a missed import path, or (b) an unexported identifier that crossed a package boundary in the split — export it (capitalise) in `shared` and prefix call sites with `shared.`.

- [ ] **Step 12: Run the full test suite — must match the Step 1 baseline**

Run: `go test ./...`
Expected: same PASS set as Step 1 (same number of `ok` packages, now under their new paths). No test logic changed. If a test fails, the move broke something — bisect against the per-move commits.

- [ ] **Step 13: Regenerate sqlc to confirm config still resolves**

Run: `sqlc generate && git diff --stat`
Expected: no diff (generated output is identical; only its location moved, already committed). If `sqlc` is not installed, skip and note it — CI gate `scripts/sqlc-check.sh` covers it.

- [ ] **Step 14: Commit the restructure**

```bash
git add -A && git commit -m "refactor(go): restructure layer-first -> modular package-by-feature

Move the rewrite service into internal/rewrite/ (domain/usecase/
transport/adapter), and cross-cutting concerns into internal/shared/
(router, auth middleware, JSON render). platform/ unchanged. Pure move
+ import-path update; full test suite green, unchanged from baseline.
Establishes internal/rewrite/ as the template every later module copies."
```

---

## Task 2: sqlc queries for the Phase 0 reads (users, app_settings)

Phase 0's `/auth/me` reads one user row; `/health` reads `app_settings.client_log_level`. Add two queries to the shared schema's query file. (The rewrite module's `queries.sql` is feature-specific; Phase 0's core reads get their own query file under the shared sqlc input so `core/` doesn't import the rewrite module.)

**Files:**
- Create: `internal/shared/pg/queries_core.sql`
- Modify: `sqlc.yaml` (add the new query file to the `queries` list)
- Generated (by sqlc): `internal/shared/pg/sqlc/*` (new `GetUserByID`, `GetClientLogLevel` funcs)

- [ ] **Step 1: Confirm the schema has the columns we read**

Run: `grep -nE 'CREATE TABLE public.(users|app_settings)' internal/platform/db/schema.sql`
Then inspect the `users` columns (need `id, email, role`) and `app_settings` (need `client_log_level`):
Run: `sed -n '/CREATE TABLE public.users /,/);/p' internal/platform/db/schema.sql` and the same for `app_settings`.
Expected: `users.id` (uuid), `users.email` (varchar), `users.role` (varchar/enum); `app_settings.client_log_level` (varchar). If `client_log_level` is absent from the dumped schema, refresh it: `scripts/dump-prod-schema.sh` (or dump from dev) — the Node app added the column.

- [ ] **Step 2: Write the two queries**

Create `internal/shared/pg/queries_core.sql`:

```sql
-- Phase 0 core-endpoint queries (health + /auth/me). Kept separate
-- from the rewrite module's queries.sql so the core package depends on
-- the shared sqlc types only, never on a feature module.

-- name: GetUserByID :one
-- Minimal user projection for GET /auth/me — id, email, role. Mirrors
-- the fields Node's /auth/me returns from the JWT-resolved user.
SELECT id, email, role
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetClientLogLevel :one
-- The single app_settings row's client_log_level, surfaced by
-- GET /health. There is exactly one settings row (Node does
-- `findOne({ where: {} })`); LIMIT 1 matches that.
SELECT client_log_level
FROM app_settings
LIMIT 1;
```

- [ ] **Step 3: Register the new query file with sqlc**

In `sqlc.yaml`, change the single `queries:` entry to a list including both files. Replace:

```yaml
    queries: "internal/adapter/pg/queries.sql"
```

with:

```yaml
    queries:
      - "internal/rewrite/adapter/pg/queries.sql"
      - "internal/shared/pg/queries_core.sql"
```

(Note: the rewrite queries path already moved in Task 1 Step 6 — confirm it points at the new location.)

- [ ] **Step 4: Generate**

Run: `sqlc generate`
Expected: success; `git status` shows modified `internal/shared/pg/sqlc/*` with new `GetUserByID` + `GetClientLogLevel` methods on the `Querier` interface and `Queries` struct.

- [ ] **Step 5: Verify the generated signatures compile**

Run: `go build ./internal/shared/...`
Expected: build OK. Note the generated row types (likely `GetUserByIDRow{ID, Email, Role}` and a `string`/`pgtype.Text` return for `GetClientLogLevel`) — Tasks 5 and 6 reference them.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat(go): add Phase 0 core sqlc queries (GetUserByID, GetClientLogLevel)

Separate query file under shared/pg so the core package depends only on
the shared sqlc types, not on any feature module."
```

---

## Task 3: Shared error envelope + request-id middleware (parity gap: request_id)

Node emits `{error, code, request_id}` on every error; the Go `errors.go` (now `internal/rewrite/transport/errors.go`) emits only `{error, code}` and is rewrite-scoped. Phase 0 promotes this to a shared, request-id-aware envelope with a status map reconciled to Node's `httpStatusForCode`.

**Files:**
- Create: `internal/shared/requestid.go`, `internal/shared/requestid_test.go`
- Create: `internal/shared/errenvelope.go`, `internal/shared/errenvelope_test.go`

- [ ] **Step 1: Write the request-id middleware test**

Create `internal/shared/requestid_test.go`:

```go
package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_MintsAndStampsResponseHeader(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if seen == "" {
		t.Fatal("handler saw empty request id in context")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Fatalf("response header X-Request-Id = %q, want %q (the context value)", got, seen)
	}
}

func TestRequestID_HonoursInboundHeader(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "inbound-123")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "inbound-123" {
		t.Fatalf("request id = %q, want inbound-123 (existing header reused)", seen)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

Run: `go test ./internal/shared/ -run TestRequestID -v`
Expected: FAIL — `undefined: RequestID`, `undefined: RequestIDFromContext`.

- [ ] **Step 3: Implement the middleware**

Create `internal/shared/requestid.go`:

```go
package shared

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDKey is unexported so only this package can write the value;
// handlers read it via RequestIDFromContext.
type requestIDCtxKey struct{}

// requestIDHeader is the canonical header both backends use. Matches
// the Node filter's `req.requestId` source so a request traced through
// Caddy keeps one id end to end.
const requestIDHeader = "X-Request-Id"

// RequestID middleware mints a UUID per request (or reuses an inbound
// X-Request-Id), stamps it on the response header, and threads it into
// the context. The error envelope reads it so every error body carries
// `request_id` — parity with the Node AllExceptionsFilter.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the id stamped by RequestID, or "" when
// the middleware did not run (e.g. a unit test calling a handler
// directly). Callers tolerate "" — an empty request_id is still valid
// JSON, just less useful for support.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey{}).(string)
	return id
}
```

- [ ] **Step 4: Run the request-id test to confirm it passes**

Run: `go test ./internal/shared/ -run TestRequestID -v`
Expected: PASS (both cases).

- [ ] **Step 5: Write the error-envelope test (status map parity)**

Create `internal/shared/errenvelope_test.go`. The table asserts the exact Node `httpStatusForCode` mapping:

```go
package shared

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusForCode_MatchesNodeTable(t *testing.T) {
	cases := map[string]int{
		"invalid-input":        http.StatusBadRequest,          // 400
		"invalid-token":        http.StatusUnauthorized,        // 401
		"user-not-found":       http.StatusUnauthorized,        // 401
		"quota-exceeded":       http.StatusPaymentRequired,     // 402
		"forbidden":            http.StatusForbidden,           // 403
		"not-found":            http.StatusNotFound,            // 404
		"conflict":             http.StatusConflict,            // 409
		"rate-limited":         http.StatusTooManyRequests,     // 429
		"provider-failed":      http.StatusBadGateway,          // 502
		"provider-unavailable": http.StatusServiceUnavailable,  // 503
		"internal":             http.StatusInternalServerError, // 500
		"something-unmapped":   http.StatusInternalServerError, // default 500
	}
	for code, want := range cases {
		if got := StatusForCode(code); got != want {
			t.Errorf("StatusForCode(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestWriteError_EnvelopeShapeWithRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := context.WithValue(context.Background(), requestIDCtxKey{}, "req-77")
	req := httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(ctx)

	WriteError(rec, req, "quota-exceeded", "daily limit reached")

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rec.Code)
	}
	var body struct {
		Error     string `json:"error"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.Error != "daily limit reached" || body.Code != "quota-exceeded" || body.RequestID != "req-77" {
		t.Fatalf("envelope = %+v, want {error:daily limit reached, code:quota-exceeded, request_id:req-77}", body)
	}
}
```

- [ ] **Step 6: Run the envelope test to confirm it fails**

Run: `go test ./internal/shared/ -run 'TestStatusForCode|TestWriteError' -v`
Expected: FAIL — `undefined: StatusForCode`, `undefined: WriteError`.

- [ ] **Step 7: Implement the shared envelope**

Create `internal/shared/errenvelope.go`:

```go
package shared

import "net/http"

// httpError is the wire shape every error response uses, identical to
// the Node AllExceptionsFilter envelope: { error, code, request_id }.
// One struct so error bodies never drift between handlers or backends
// (Rule #1 — one place owns the contract).
type httpError struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

// StatusForCode maps a kebab-case error code to its HTTP status,
// reconciled byte-for-byte with the Node backend's
// httpStatusForCode (backend/src/common/error-codes.ts). Codes not
// listed default to 500 — same as Node.
func StatusForCode(code string) int {
	switch code {
	case "invalid-input":
		return http.StatusBadRequest // 400
	case "invalid-token", "user-not-found":
		return http.StatusUnauthorized // 401
	case "quota-exceeded":
		return http.StatusPaymentRequired // 402
	case "forbidden":
		return http.StatusForbidden // 403
	case "not-found":
		return http.StatusNotFound // 404
	case "conflict":
		return http.StatusConflict // 409
	case "rate-limited":
		return http.StatusTooManyRequests // 429
	case "provider-failed":
		return http.StatusBadGateway // 502
	case "provider-unavailable":
		return http.StatusServiceUnavailable // 503
	default:
		return http.StatusInternalServerError // 500
	}
}

// WriteError writes the canonical error envelope. Status is derived
// from the code via StatusForCode, and request_id is pulled from the
// request context (set by the RequestID middleware). Every handler —
// current and future — emits errors through this single function.
func WriteError(w http.ResponseWriter, r *http.Request, code, message string) {
	WriteJSON(w, StatusForCode(code), httpError{
		Error:     message,
		Code:      code,
		RequestID: RequestIDFromContext(r.Context()),
	})
}
```

- [ ] **Step 8: Run the full shared test suite**

Run: `go test ./internal/shared/ -v`
Expected: PASS — request-id (2) + status map + envelope shape.

- [ ] **Step 9: Wire RequestID into the router before the access logger**

In `internal/shared/router.go`'s `Build()`, add `mux.Use(RequestID)` as the FIRST middleware (before `middleware.RealIP`), so every downstream layer — including error responses and the access log — sees the id. (chi's own `middleware.RequestID` can stay or be removed; our `RequestID` is the one the envelope reads. To avoid two ids, remove the `mux.Use(middleware.RequestID)` line and update `structuredLogger` to read `RequestIDFromContext(req.Context())` instead of `middleware.GetReqID`.)

Run: `go build ./... && go test ./internal/shared/ -v`
Expected: build OK, tests PASS.

- [ ] **Step 10: Commit**

```bash
git add -A && git commit -m "feat(go): shared error envelope with request_id + reconciled status map

Promote the rewrite-scoped {error,code} body to a shared
{error,code,request_id} envelope matching the Node AllExceptionsFilter.
Add a RequestID middleware (mint/reuse X-Request-Id, stamp response +
context) wired first in the chain. StatusForCode mirrors Node's
httpStatusForCode byte-for-byte, asserted by a table-driven test."
```

---

## Task 4: JWT email claim + signer util (parity gap: email claim)

Node's access-token payload is `{sub, email, role}`; the Go `Claims` has only `{sub, role}`. Add `Email`, and add a `Signer` that issues Node-identical tokens so Phase 1 login is plug-in. Phase 0 only verifies on the request path; the signer is unit-tested, not yet wired to an endpoint.

**Files:**
- Modify: `internal/platform/auth/jwt.go` (add `Email` field + `Signer`)
- Modify/Create: `internal/platform/auth/jwt_test.go` (add email round-trip + parity tests)

- [ ] **Step 1: Write the failing tests for email claim + signer round-trip**

Add to `internal/platform/auth/jwt_test.go`:

```go
func TestVerify_ParsesEmailClaim(t *testing.T) {
	secret := "test-secret-at-least-16-chars"
	signer := NewSigner(secret)
	tok, err := signer.Sign(Claims{Sub: "u-1", Email: "a@b.com", Role: "user"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Email != "a@b.com" {
		t.Fatalf("email = %q, want a@b.com", claims.Email)
	}
	if claims.Sub != "u-1" || claims.Role != "user" {
		t.Fatalf("claims = %+v, want sub=u-1 role=user", claims)
	}
}

func TestSign_PayloadMatchesNodeShape(t *testing.T) {
	// Node signs { sub, email, role, iat, exp } with HS256. Assert the
	// Go-issued token decodes to exactly those claim keys so a Node
	// verifier (and our own) accept it interchangeably.
	secret := "test-secret-at-least-16-chars"
	tok, err := NewSigner(secret).Sign(Claims{Sub: "u-9", Email: "x@y.z", Role: "admin"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	parsed, _ := jwt.Parse(tok, func(*jwt.Token) (any, error) { return []byte(secret), nil })
	m, _ := parsed.Claims.(jwt.MapClaims)
	for _, k := range []string{"sub", "email", "role", "iat", "exp"} {
		if _, ok := m[k]; !ok {
			t.Errorf("issued token missing claim %q", k)
		}
	}
	if m["sub"] != "u-9" || m["email"] != "x@y.z" || m["role"] != "admin" {
		t.Errorf("claim values = %v, want sub=u-9 email=x@y.z role=admin", m)
	}
}
```

(Ensure the test file imports `time` and `github.com/golang-jwt/jwt/v5` as `jwt`.)

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/platform/auth/ -run 'TestVerify_ParsesEmail|TestSign_Payload' -v`
Expected: FAIL — `Claims has no field Email`, `undefined: NewSigner`.

- [ ] **Step 3: Add the Email claim**

In `internal/platform/auth/jwt.go`, extend `Claims`:

```go
type Claims struct {
	Sub   string `json:"sub"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
	jwt.RegisteredClaims
}
```

- [ ] **Step 4: Add the Signer**

Append to `internal/platform/auth/jwt.go`:

```go
// Signer issues HS256 tokens whose payload is byte-compatible with the
// NestJS backend ({sub, email, role, iat, exp}). Phase 0 does not mint
// tokens on any request path — this exists so Phase 1's login handler
// drops in without re-deriving the claim shape. Same secret as the
// Verifier; a Go-issued token verifies on Node and vice versa.
type Signer struct {
	secret []byte
}

// NewSigner stores the shared secret as bytes (mirrors NewVerifier).
func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

// Sign returns a signed token for the given claims, expiring ttl from
// now. iat/exp are stamped here so callers pass only the identity
// fields. The caller chooses ttl (Node sources it from
// app_settings.token_expiry_minutes).
func (s *Signer) Sign(c Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}
```

Add `"time"` to the file's imports if absent.

- [ ] **Step 5: Run the auth tests**

Run: `go test ./internal/platform/auth/ -v`
Expected: PASS — existing verifier tests + the 2 new ones. (The existing `validate`/middleware path that reads `Email` off claims now compiles.)

- [ ] **Step 6: Surface email through the auth middleware (if it builds a user view)**

The `RequireAuth` middleware (now `internal/shared/auth_middleware.go`) stores `*auth.Claims` in context; `claims.Email` is now available to handlers. No change needed unless a handler needs it — `/auth/me` (Task 6) reads it from the DB, not the token, for parity. Confirm no compile breakage:

Run: `go build ./...`
Expected: build OK.

- [ ] **Step 7: Commit**

```bash
git add -A && git commit -m "feat(go): add email JWT claim + Signer for Node-identical token issuance

Claims gains Email (Node payload is {sub,email,role}). NewSigner issues
HS256 tokens byte-compatible with NestJS so Phase 1 login plugs in and
tokens are mutually valid across backends during the parallel window.
Round-trip + payload-shape parity tests added."
```

---

## Task 5: GET /health parity endpoint

Match Node's `{app:"draftright", version, status:"ok", client_log_level}` with the same DB-tolerant, 30s-cached `client_log_level` read. Lives in `internal/core/`.

**Files:**
- Create: `internal/core/health.go`, `internal/core/health_test.go`

- [ ] **Step 1: Write the health handler test**

Create `internal/core/health_test.go`:

```go
package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubLogLevel implements the LogLevelReader the health handler needs.
type stubLogLevel struct {
	level string
	err   error
}

func (s stubLogLevel) ClientLogLevel(context.Context) (string, error) { return s.level, s.err }

func TestHealth_ShapeMatchesNode(t *testing.T) {
	h := NewHealthHandler(stubLogLevel{level: "warnings"}, "2.0.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	want := map[string]any{"app": "draftright", "version": "2.0.0", "status": "ok", "client_log_level": "warnings"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %v, want %v", k, body[k], v)
		}
	}
}

func TestHealth_DBErrorFallsBackToInfo(t *testing.T) {
	h := NewHealthHandler(stubLogLevel{err: context.DeadlineExceeded}, "2.0.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("a DB hiccup must not fail /health; status = %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["client_log_level"] != "info" {
		t.Errorf("fallback level = %v, want info", body["client_log_level"])
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/core/ -run TestHealth -v`
Expected: FAIL — `undefined: NewHealthHandler`.

- [ ] **Step 3: Implement the health handler**

Create `internal/core/health.go`:

```go
// Package core hosts the Phase 0 proof endpoints (GET /health,
// GET /auth/me). These are deliberately thin — they exist to exercise
// the foundation (config, DB, JWT, envelope, shadow-diff) end to end.
// /auth/me moves into a real internal/auth module in Phase 1.
package core

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// LogLevelReader is the seam over app_settings.client_log_level. The
// Postgres implementation lives in this package (adapter on the shared
// sqlc Queries); tests pass a stub. Interface, not a concrete, so the
// handler is unit-testable without a database (Rule #1).
type LogLevelReader interface {
	ClientLogLevel(ctx context.Context) (string, error)
}

// HealthHandler serves GET /health with the same body shape as the
// Node backend: { app, version, status, client_log_level }. The log
// level is cached briefly and DB-tolerant — a DB hiccup never makes
// the liveness probe fail.
type HealthHandler struct {
	reader  LogLevelReader
	version string

	mu        sync.Mutex
	cached    string
	cachedAt  time.Time
}

const (
	healthApp        = "draftright"
	defaultLogLevel  = "info"
	logLevelCacheTTL = 30 * time.Second
)

// NewHealthHandler wires the log-level source + the version string the
// endpoint reports (mirror the Node "2.0.0").
func NewHealthHandler(reader LogLevelReader, version string) *HealthHandler {
	return &HealthHandler{reader: reader, version: version, cached: defaultLogLevel}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	shared.WriteJSON(w, http.StatusOK, map[string]string{
		"app":              healthApp,
		"version":          h.version,
		"status":           "ok",
		"client_log_level": h.clientLogLevel(r.Context()),
	})
}

// clientLogLevel returns the cached level, refreshing at most every
// TTL. On a read error it keeps serving the last-known (or default)
// level — identical tolerance to the Node controller.
func (h *HealthHandler) clientLogLevel(ctx context.Context) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if time.Since(h.cachedAt) < logLevelCacheTTL && h.cached != "" {
		return h.cached
	}
	level, err := h.reader.ClientLogLevel(ctx)
	if err != nil || level == "" {
		if h.cached == "" {
			h.cached = defaultLogLevel
		}
		// keep h.cached as-is (last known), refresh the timer so we
		// don't hammer a struggling DB on every probe
		h.cachedAt = time.Now()
		return h.cached
	}
	h.cached = level
	h.cachedAt = time.Now()
	return level
}
```

- [ ] **Step 4: Run the health tests**

Run: `go test ./internal/core/ -run TestHealth -v`
Expected: PASS (both cases).

- [ ] **Step 5: Implement the Postgres LogLevelReader adapter**

Append to `internal/core/health.go` (or a sibling `pg.go`) the concrete reader over shared sqlc:

```go
import sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"

// pgLogLevel reads app_settings.client_log_level via the shared sqlc
// Queries. Returns ("", err) on no-row / db error so the handler falls
// back to its cached/default level.
type pgLogLevel struct{ q *sqlc.Queries }

// NewPgLogLevel adapts a sqlc.Queries (built from the shared pool) to
// the LogLevelReader seam.
func NewPgLogLevel(q *sqlc.Queries) LogLevelReader { return pgLogLevel{q: q} }

func (p pgLogLevel) ClientLogLevel(ctx context.Context) (string, error) {
	return p.q.GetClientLogLevel(ctx)
}
```

Adjust the return-type handling to whatever sqlc generated in Task 2 Step 5 (e.g. if `GetClientLogLevel` returns `pgtype.Text`, convert: `if v.Valid { return v.String, nil }; return "", nil`). Put the import at the file top, not inline.

Run: `go build ./internal/core/`
Expected: build OK.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat(go): GET /health parity endpoint (app/version/status/client_log_level)

Matches the Node health body incl. DB-tolerant 30s-cached
client_log_level. LogLevelReader seam keeps the handler unit-testable
without Postgres; pg adapter reads app_settings via shared sqlc."
```

---

## Task 6: GET /auth/me parity endpoint

Match Node's `{id, email, role, flags:{use_go_backend}}`. Reads the user via shared sqlc, computes `use_go_backend` from the `GO_BACKEND_RAMP_PERCENT` bucket. Lives in `internal/core/`.

**Files:**
- Create: `internal/core/me.go`, `internal/core/me_test.go`
- Create: `internal/core/featureflags.go`, `internal/core/featureflags_test.go`

- [ ] **Step 1: Confirm the Node bucket algorithm**

Read `backend/src/.../feature-flags*.ts` (the `FeatureFlagsService.useGoBackend` bucket). Run from repo root:
Run: `grep -rn "useGoBackend\|GO_BACKEND_RAMP\|bucket" backend/src --include='*.ts'`
Record the exact algorithm (almost certainly: hash the user id → integer in [0,100) → `< ramp_percent`). The Go implementation MUST reproduce it bit-for-bit or the same user gets different answers from the two backends. Note the hash function used (e.g. first bytes of a sha/`crc`, or `parseInt` over a substring).

- [ ] **Step 2: Write the feature-flag bucket test (mirrors Node)**

Create `internal/core/featureflags_test.go` — fill the expected values from the Node algorithm recorded in Step 1:

```go
package core

import "testing"

func TestUseGoBackend_RampBoundaries(t *testing.T) {
	// ramp 0 => nobody; ramp 100 => everybody. These hold regardless
	// of the hash function.
	if UseGoBackend("any-user-id", 0) {
		t.Error("ramp 0 must bucket everyone OUT")
	}
	if !UseGoBackend("any-user-id", 100) {
		t.Error("ramp 100 must bucket everyone IN")
	}
}

func TestUseGoBackend_DeterministicPerUser(t *testing.T) {
	a := UseGoBackend("user-abc", 50)
	b := UseGoBackend("user-abc", 50)
	if a != b {
		t.Error("same user + same ramp must be stable")
	}
}
```

If Step 1 revealed concrete known vectors (a specific user id that lands at bucket N), add an exact-value case asserting parity with Node. (Capture one by calling the dev Node `/auth/me` with a known token at a known ramp.)

- [ ] **Step 3: Run to confirm failure**

Run: `go test ./internal/core/ -run TestUseGoBackend -v`
Expected: FAIL — `undefined: UseGoBackend`.

- [ ] **Step 4: Implement the bucket function**

Create `internal/core/featureflags.go` reproducing the Node algorithm from Step 1. Example shape (REPLACE the hash body with whatever Node does):

```go
package core

import (
	"crypto/sha256"
	"encoding/binary"
)

// UseGoBackend reproduces the Node FeatureFlagsService bucket exactly:
// a stable hash of the user id mapped into [0,100), compared against
// rampPercent. MUST match Node bit-for-bit (backend/src/.../
// feature-flags.service.ts) or a user gets contradictory answers from
// the two backends mid-migration.
//
// NOTE: confirm the hash matches Node in Task 6 Step 1 before trusting
// this. If Node uses a different hash, swap the body — the signature
// and boundary behavior stay.
func UseGoBackend(userID string, rampPercent int) bool {
	if rampPercent <= 0 {
		return false
	}
	if rampPercent >= 100 {
		return true
	}
	sum := sha256.Sum256([]byte(userID))
	bucket := binary.BigEndian.Uint32(sum[:4]) % 100
	return int(bucket) < rampPercent
}
```

- [ ] **Step 5: Run the bucket test**

Run: `go test ./internal/core/ -run TestUseGoBackend -v`
Expected: PASS. If an exact-vector case from Step 2 fails, the hash differs from Node — fix the body until the dev Node answer matches.

- [ ] **Step 6: Write the /auth/me handler test**

Create `internal/core/me_test.go`:

```go
package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

type stubUser struct {
	id, email, role string
	err             error
}

func (s stubUser) ByID(context.Context, string) (UserRow, error) {
	return UserRow{ID: s.id, Email: s.email, Role: s.role}, s.err
}

func TestMe_ShapeMatchesNode(t *testing.T) {
	h := NewMeHandler(stubUser{id: "u-1", email: "a@b.com", role: "user"}, 0)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	// inject verified claims the way RequireAuth would
	ctx := shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "u-1", Email: "a@b.com", Role: "user"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
		Flags struct {
			UseGoBackend bool `json:"use_go_backend"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body.ID != "u-1" || body.Email != "a@b.com" || body.Role != "user" {
		t.Fatalf("body = %+v, want id/email/role of u-1", body)
	}
	if body.Flags.UseGoBackend != false {
		t.Fatalf("use_go_backend = true, want false at ramp 0")
	}
}
```

This references a `UserRow`, a `UserReader` seam, `NewMeHandler`, and `shared.ContextWithClaims`. The last is a small exported helper so tests (and `RequireAuth`) inject claims — add it in Step 7.

- [ ] **Step 7: Add `ContextWithClaims` to shared auth middleware**

The middleware currently stores claims under an unexported key. Expose a constructor + keep the existing getter. In `internal/shared/auth_middleware.go`, add:

```go
// ContextWithClaims stores verified claims under the package key. Used
// by RequireAuth and by handler tests that need to simulate an authed
// request without a real token.
func ContextWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}
```

(Ensure `RequireAuth` uses this same constructor instead of inlining `context.WithValue`, so there is one writer — Rule #1. Update `ClaimsFromContext` if its key type changed during the Task 1 move.)

- [ ] **Step 8: Run the me test to confirm it fails for the right reason**

Run: `go test ./internal/core/ -run TestMe -v`
Expected: FAIL — `undefined: NewMeHandler`, `undefined: UserRow`, `undefined: UserReader`.

- [ ] **Step 9: Implement the /auth/me handler**

Create `internal/core/me.go`:

```go
package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// UserRow is the minimal user projection /auth/me returns. Decoupled
// from the sqlc row type so the handler + its test don't depend on the
// generated package.
type UserRow struct {
	ID    string
	Email string
	Role  string
}

// UserReader is the seam over the user lookup. pg implementation below;
// tests stub it.
type UserReader interface {
	ByID(ctx context.Context, id string) (UserRow, error)
}

// MeHandler serves GET /auth/me with Node's body shape:
// { id, email, role, flags: { use_go_backend } }. Identity comes from
// the verified JWT claims (RequireAuth ran first); the user row is
// reloaded from the DB so a stale token can't report stale email/role.
type MeHandler struct {
	users       UserReader
	rampPercent int
}

// NewMeHandler wires the user source + the GO_BACKEND_RAMP_PERCENT in
// effect (from config). ramp drives the use_go_backend flag.
func NewMeHandler(users UserReader, rampPercent int) *MeHandler {
	return &MeHandler{users: users, rampPercent: rampPercent}
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		// RequireAuth must run before this handler; reaching here is a
		// router misconfiguration, not a client error.
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}

	row, err := h.users.ByID(r.Context(), claims.Sub)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			shared.WriteError(w, r, "user-not-found", "user not found")
			return
		}
		shared.WriteError(w, r, "internal", "failed to load user")
		return
	}

	shared.WriteJSON(w, http.StatusOK, map[string]any{
		"id":    row.ID,
		"email": row.Email,
		"role":  row.Role,
		"flags": map[string]any{
			"use_go_backend": UseGoBackend(row.ID, h.rampPercent),
		},
	})
}

// errUserNotFound is the sentinel the pg adapter returns when the JWT
// sub matches no row (deleted user with a still-valid token).
var errUserNotFound = errors.New("core: user not found")

// pgUserReader adapts shared sqlc to UserReader.
type pgUserReader struct{ q *sqlc.Queries }

// NewPgUserReader builds the production user source over shared sqlc.
func NewPgUserReader(q *sqlc.Queries) UserReader { return pgUserReader{q: q} }

func (p pgUserReader) ByID(ctx context.Context, id string) (UserRow, error) {
	// id is the JWT sub (uuid string). GetUserByID takes the sqlc uuid
	// param type — convert per whatever sqlc generated in Task 2.
	row, err := p.q.GetUserByID(ctx, parseUUIDParam(id))
	if err != nil {
		return UserRow{}, mapUserErr(err)
	}
	return UserRow{ID: id, Email: row.Email, Role: row.Role}, nil
}
```

Implement the two small helpers `parseUUIDParam` (string → the sqlc param type, e.g. `pgtype.UUID`) and `mapUserErr` (`pgx.ErrNoRows` → `errUserNotFound`, else passthrough) in the same file, matching the generated `GetUserByID` signature from Task 2 Step 5. If `GetUserByID` returns the `id` already, prefer `row.ID` over echoing the input.

- [ ] **Step 10: Run the core test suite**

Run: `go test ./internal/core/ -v`
Expected: PASS — health (2) + bucket (2) + me (1).

- [ ] **Step 11: Commit**

```bash
git add -A && git commit -m "feat(go): GET /auth/me parity endpoint + use_go_backend bucket

Returns {id,email,role,flags:{use_go_backend}} matching Node. Identity
from verified JWT, user reloaded via shared sqlc. UseGoBackend
reproduces the Node GO_BACKEND_RAMP_PERCENT bucket; UserReader/
LogLevelReader seams keep handlers DB-free in tests."
```

---

## Task 7: Wire /health + /auth/me into the router + composition root

**Files:**
- Modify: `internal/shared/router.go` (add core handler fields + mounts)
- Modify: `cmd/server/main.go` (build core handlers, pass to router)

- [ ] **Step 1: Add core handler fields + mounts to the router**

In `internal/shared/router.go`, add fields to `Router`:

```go
	// Core Phase 0 endpoints. Health is public; Me is auth-gated.
	Health http.Handler // GET /health
	Me     http.Handler // GET /auth/me (mounted inside the auth group)
```

In `Build()`, replace the old `mux.Get("/health", handleHealth)` with `mux.Method(http.MethodGet, "/health", r.Health)` (guard nil for tests), and inside the auth `mux.Group`, add `api.Method(http.MethodGet, "/auth/me", r.Me)`. Delete the now-unused stub `handleHealth`.

- [ ] **Step 2: Build core deps in main + pass to router**

In `cmd/server/main.go`, after the Postgres pool is built in `composeDeps`, build a shared `sqlc.Queries` over the same pool and construct the core handlers. Since `composeDeps` currently returns only `usecase.RewriteDeps`, add the pool (or a `*sqlc.Queries`) to its return so main can wire core. Minimal change: have `composeDeps` also return the `*pgxpool.Pool` (or nil in dev fallback); in main:

```go
var health, me http.Handler
if pool != nil {
	q := sqlc.New(pool)
	health = core.NewHealthHandler(core.NewPgLogLevel(q), "2.0.0")
	me = core.NewMeHandler(core.NewPgUserReader(q), cfg.GoBackendRampPercent)
} else {
	// dev fallback: health with a static "info" reader, me disabled
	health = core.NewHealthHandler(staticInfoReader{}, "2.0.0")
	me = nil
}
```

Add `GoBackendRampPercent int` to `config.Config` + parse `GO_BACKEND_RAMP_PERCENT` (mirror the Node default 0) in `config.Load`. Add the `staticInfoReader` tiny stub in main (returns `"info", nil`) so dev `/health` works without a DB.

- [ ] **Step 3: Set the router's core fields in main**

In the `(&shared.Router{...})` literal, add `Health: health, Me: me,`.

- [ ] **Step 4: Build + test everything**

Run: `goimports -w ./... && go build ./... && go test ./...`
Expected: build OK; all tests PASS.

- [ ] **Step 5: Smoke-test locally against dev Postgres**

```bash
# point at dev DB (read-only creds fine); JWT_SECRET must match dev Node
DATABASE_URL='postgres://...dev...' JWT_SECRET='<dev secret>' \
  GO_BACKEND_RAMP_PERCENT=0 LISTEN_ADDR=':3001' go run ./cmd/server &
curl -s localhost:3001/health | jq .
# expect {app:"draftright", version:"2.0.0", status:"ok", client_log_level:"..."}
curl -s -H "Authorization: Bearer <dev-node-issued-token>" localhost:3001/auth/me | jq .
# expect {id, email, role, flags:{use_go_backend:false}}
```

Expected: both bodies match the Node shapes. Kill the server after.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat(go): mount /health + /auth/me; wire core handlers in composition root

config gains GoBackendRampPercent (GO_BACKEND_RAMP_PERCENT, default 0).
Router mounts health public + me inside the auth group. Dev fallback
serves /health with a static info reader when DATABASE_URL is unset."
```

---

## Task 8: Shadow-compare replay tool (`cmd/shadowdiff`) + fixtures

The Phase 0 done-gate: a standalone tool that replays fixtures against Node + Go and diffs status + JSON.

**Files:**
- Create: `cmd/shadowdiff/main.go`, `cmd/shadowdiff/diff.go`, `cmd/shadowdiff/diff_test.go`
- Create: `fixtures/health.json`, `fixtures/auth_me.json`

- [ ] **Step 1: Write the diff-logic test (pure, no network)**

Create `cmd/shadowdiff/diff_test.go`:

```go
package main

import "testing"

func TestDiffJSON_IgnoresRequestIDValue(t *testing.T) {
	node := []byte(`{"error":"x","code":"invalid-input","request_id":"aaaa"}`)
	goB := []byte(`{"error":"x","code":"invalid-input","request_id":"bbbb"}`)
	// request_id legitimately differs per request — present+non-empty
	// in both is parity; the value must NOT count as a mismatch.
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) != 0 {
		t.Fatalf("request_id value diff should be ignored, got %v", diffs)
	}
}

func TestDiffJSON_FlagsRealMismatch(t *testing.T) {
	node := []byte(`{"role":"user"}`)
	goB := []byte(`{"role":"admin"}`)
	diffs := diffJSON(node, goB, nil)
	if len(diffs) == 0 {
		t.Fatal("role mismatch must be reported")
	}
}

func TestDiffJSON_RequestIDMustBePresentInBoth(t *testing.T) {
	node := []byte(`{"request_id":"aaaa"}`)
	goB := []byte(`{}`)
	diffs := diffJSON(node, goB, []string{"request_id"})
	if len(diffs) == 0 {
		t.Fatal("missing request_id on one side must be reported even when value is ignored")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./cmd/shadowdiff/ -v`
Expected: FAIL — `undefined: diffJSON`.

- [ ] **Step 3: Implement the diff logic**

Create `cmd/shadowdiff/diff.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// diffJSON deep-compares two JSON bodies. Keys in ignoreValueOf are
// compared for presence + non-emptiness only (their values may
// legitimately differ per request, e.g. request_id). Returns a sorted
// list of human-readable diff lines; empty means parity.
func diffJSON(a, b []byte, ignoreValueOf []string) []string {
	var ma, mb map[string]any
	if err := json.Unmarshal(a, &ma); err != nil {
		return []string{fmt.Sprintf("node body not JSON: %v", err)}
	}
	if err := json.Unmarshal(b, &mb); err != nil {
		return []string{fmt.Sprintf("go body not JSON: %v", err)}
	}
	ignore := map[string]bool{}
	for _, k := range ignoreValueOf {
		ignore[k] = true
	}
	var diffs []string
	for _, k := range unionKeys(ma, mb) {
		av, aok := ma[k]
		bv, bok := mb[k]
		if ignore[k] {
			if !aok || !bok || isEmpty(av) || isEmpty(bv) {
				diffs = append(diffs, fmt.Sprintf("%s: must be present+non-empty on both (node=%v go=%v)", k, av, bv))
			}
			continue
		}
		if !reflect.DeepEqual(av, bv) {
			diffs = append(diffs, fmt.Sprintf("%s: node=%v go=%v", k, av, bv))
		}
	}
	sort.Strings(diffs)
	return diffs
}

func unionKeys(a, b map[string]any) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isEmpty(v any) bool {
	s, ok := v.(string)
	return v == nil || (ok && s == "")
}
```

- [ ] **Step 4: Run the diff test**

Run: `go test ./cmd/shadowdiff/ -v`
Expected: PASS (all three).

- [ ] **Step 5: Implement the replay main**

Create `cmd/shadowdiff/main.go`:

```go
// Command shadowdiff replays fixture requests against the Node backend
// and the Go backend and reports any status/JSON differences. It is the
// Phase 0 parity gate: every fixture must report PASS before a path is
// flipped in Caddy. No live traffic is tapped — fixtures are authored +
// checked in (no production launch yet, so no real stream exists).
//
// Usage:
//   shadowdiff --node=https://api.dev.draftright.info \
//              --go=http://localhost:3001 --fixtures=./fixtures
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// fixture is one replayable request. headers may carry an Authorization
// bearer (a real Node-issued token for authed endpoints).
type fixture struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	// IgnoreValueOf lists response keys whose value may differ per
	// request (compared for presence only). request_id is the usual one.
	IgnoreValueOf []string `json:"ignore_value_of"`
}

func main() {
	nodeBase := flag.String("node", "", "Node backend base URL")
	goBase := flag.String("go", "", "Go backend base URL")
	dir := flag.String("fixtures", "./fixtures", "fixtures directory")
	flag.Parse()
	if *nodeBase == "" || *goBase == "" {
		fmt.Fprintln(os.Stderr, "both --node and --go are required")
		os.Exit(2)
	}

	fixtures, err := loadFixtures(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load fixtures: %v\n", err)
		os.Exit(2)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	failed := 0
	for _, f := range fixtures {
		nStatus, nBody, err := send(client, *nodeBase, f)
		if err != nil {
			fmt.Printf("FAIL %s: node request error: %v\n", f.Name, err)
			failed++
			continue
		}
		gStatus, gBody, err := send(client, *goBase, f)
		if err != nil {
			fmt.Printf("FAIL %s: go request error: %v\n", f.Name, err)
			failed++
			continue
		}
		var problems []string
		if nStatus != gStatus {
			problems = append(problems, fmt.Sprintf("status: node=%d go=%d", nStatus, gStatus))
		}
		problems = append(problems, diffJSON(nBody, gBody, f.IgnoreValueOf)...)
		if len(problems) > 0 {
			failed++
			fmt.Printf("FAIL %s\n", f.Name)
			for _, p := range problems {
				fmt.Printf("   - %s\n", p)
			}
			continue
		}
		fmt.Printf("PASS %s\n", f.Name)
	}

	fmt.Printf("\n%d/%d fixtures passed\n", len(fixtures)-failed, len(fixtures))
	if failed > 0 {
		os.Exit(1)
	}
}

func loadFixtures(dir string) ([]fixture, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []fixture
	for _, p := range entries {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if f.Name == "" {
			f.Name = filepath.Base(p)
		}
		out = append(out, f)
	}
	return out, nil
}

func send(c *http.Client, base string, f fixture) (int, []byte, error) {
	var body io.Reader
	if len(f.Body) > 0 {
		body = bytes.NewReader(f.Body)
	}
	req, err := http.NewRequest(f.Method, base+f.Path, body)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range f.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}
```

- [ ] **Step 6: Build the tool**

Run: `go build ./cmd/shadowdiff/`
Expected: build OK.

- [ ] **Step 7: Write the fixtures**

Create `fixtures/health.json`:

```json
{
  "name": "health",
  "method": "GET",
  "path": "/health",
  "ignore_value_of": []
}
```

Create `fixtures/auth_me.json` (replace the bearer with a real, unexpired dev-Node-issued access token at run time — do NOT commit a long-lived token; use a short-lived one or a placeholder the operator fills):

```json
{
  "name": "auth_me",
  "method": "GET",
  "path": "/auth/me",
  "headers": { "Authorization": "Bearer REPLACE_WITH_DEV_NODE_TOKEN" },
  "ignore_value_of": ["request_id"]
}
```

- [ ] **Step 8: Run the parity gate against dev Node + local Go**

Start the Go server (Task 7 Step 5), then:

```bash
# mint a fresh dev token (login to dev Node), paste into fixtures/auth_me.json
go run ./cmd/shadowdiff \
  --node=https://api.dev.draftright.info \
  --go=http://localhost:3001 \
  --fixtures=./fixtures
```

Expected: `PASS health`, `PASS auth_me`, `2/2 fixtures passed`, exit 0. If `auth_me` fails on a field, fix the Go handler until the diff is empty (that's the whole point of the gate). `/health` `version` may differ if Node reports a different version string — align the Go `version` constant to whatever dev Node returns, or add `version` to `ignore_value_of` if the two backends legitimately report their own build versions (decide per parity intent; default is to match).

- [ ] **Step 9: Commit**

```bash
git add -A && git commit -m "feat(go): cmd/shadowdiff parity gate + health/auth_me fixtures

Standalone replay tool: fires fixtures at Node + Go, deep-diffs status +
JSON, treats request_id as present-only. Exits non-zero on any diff —
the Phase 0 done-gate. Ships /health + /auth/me fixtures (authed fixture
token filled at run time, never committed long-lived)."
```

---

## Task 9: Final verification + housekeeping

- [ ] **Step 1: Full build + vet + test**

Run: `go vet ./... && go build ./... && go test ./...`
Expected: clean vet, build OK, all PASS.

- [ ] **Step 2: Confirm no cross-module internal imports**

Run: `grep -rn '"github.com/tannpv/draftright-rewrite/internal/rewrite' internal/core/ ; grep -rn '"github.com/tannpv/draftright-rewrite/internal/core' internal/rewrite/`
Expected: no output. `core` and `rewrite` must not import each other's `internal` — only `platform` + `shared`. If anything prints, refactor the shared piece into `internal/shared/`.

- [ ] **Step 3: Confirm sqlc is drift-free**

Run: `sqlc generate && git diff --quiet && echo "sqlc clean" || echo "sqlc DRIFT — commit generated changes"`
Expected: `sqlc clean`.

- [ ] **Step 4: Update the master design doc status**

In `docs/superpowers/specs/2026-06-13-go-backend-port-design.md`, change the Phase 0 bullet to note it is implemented + the proof endpoints are shadow-green on dev. Commit:

```bash
git add docs/superpowers/specs/2026-06-13-go-backend-port-design.md
git commit -m "docs(go-port): mark Phase 0 implemented (health + me shadow-green on dev)"
```

- [ ] **Step 5: Merge to develop (GitFlow — no direct commits to develop/main)**

```bash
git checkout develop && git pull origin develop
git merge --no-ff feature/go-backend-phase0-spec-20260613 -m "Merge Phase 0 parity foundation (Go backend port)"
git push origin develop
```

Do NOT deploy or flip any Caddy route — Phase 0 ships nothing to production. The Go service stays dev-only until parity is green across enough modules to flip a path (Phase 5).

---

## Self-Review notes (coverage check vs spec)

- Spec §3 parity gaps → Task 3 (request_id + status map), Task 4 (email claim), Task 5 (/health), Task 6 (/auth/me). ✓
- Spec §4 restructure-in-Phase-0 → Task 1. ✓ (sqlc → shared, not under rewrite, as §5 note requires.)
- Spec §5 deliverables 1–7 → Tasks 1,2,3,4,5,6,8. ✓
- Spec §6 shadow harness (presence-only request_id, fixtures, exit non-zero) → Task 8. ✓
- Spec §7 testing (restructure green, unit per piece, shadow gate) → Tasks 1.12, 3, 4, 5, 6, 8.8. ✓
- Spec §9 done criteria (tests green, modular, shadow PASS, no migration/prod change) → Task 9. ✓
- Spec §10 out-of-scope (no login, me moves in Phase 1, healthcheck #28) → respected; `core/` holds `me` temporarily per §4. ✓
- Open risk carried into execution: Task 6 Step 1 MUST confirm the Node feature-flag hash before trusting `UseGoBackend` — flagged inline.
