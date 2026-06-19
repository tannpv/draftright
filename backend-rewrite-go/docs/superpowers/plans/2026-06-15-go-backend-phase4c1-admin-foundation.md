# Phase 4c-1 — Admin Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the NestJS `admin/auth` foundation (3 endpoints), the admin-role gate, and the shared dynamic list-query helper to Go as a byte-identical drop-in.

**Architecture:** New flat clean-arch slice `internal/adminauth/` (domain → usecase → repo_pg → handler), a pure helper package `internal/shared/listquery/`, a `RequireAdmin` middleware in `internal/shared/`, and a one-field widening of `auth.Claims`. Composed in `cmd/server/main.go`. Admin tokens are signed with the same `JWT_SECRET` as customer access tokens, so the existing `auth.Verifier` validates them; 4c-1 only adds the `isAdmin` claim and the role gate.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + sqlc, golang-jwt/jwt/v5 (HS256, zero clock tolerance), x/crypto/bcrypt (cost 10), slog.

**Spec:** `docs/superpowers/specs/2026-06-15-go-backend-phase4c1-admin-foundation-design.md`

**Parity authority:** NestJS `/opt/openAi/DraftRight/backend/src`. Node wins every disagreement.

## Deviations from spec (intentional, more idiomatic)

1. **`RequireAdmin` lives in `internal/shared/admin_middleware.go`**, not `internal/platform/auth/admin.go`. It reads `shared.ClaimsFromContext` and writes via `shared.WriteError` — both unexported-keyed / package-local to `shared`. Placing it beside `RequireAuth` (`shared/auth_middleware.go`) keeps the context wiring in one package.
2. **No `Minter` port.** The `adminauth.Service` holds two `*platauth.Signer` + an `isProd bool` directly and picks the TTL inline — mirroring the existing `internal/auth` `Service` (which holds `access`/`refresh` `*platauth.Signer`). A single-impl port with no fake would be ceremony (YAGNI); signing is deterministic and tests verify it with a real `platauth.Verifier`.

## File structure

```
internal/shared/listquery/listquery.go        NEW  Query, Parse, jsParseInt, Built, Build
internal/shared/listquery/listquery_test.go    NEW  table tests (Tasks 1-2)
internal/platform/auth/jwt.go                  MOD  add IsAdminFlag claim field (Task 3)
internal/platform/auth/jwt_test.go             MOD  decode-isAdmin test (Task 3)
internal/shared/admin_middleware.go            NEW  RequireAdmin middleware (Task 4)
internal/shared/admin_middleware_test.go       NEW  guard tests (Task 4)
internal/adminauth/domain.go                   NEW  AdminUser + Error sentinels (Task 5)
internal/shared/pg/queries_adminauth.sql       NEW  3 sqlc queries (Task 5)
internal/shared/pg/sqlc/*                       GEN  sqlc generate output (Task 5)
internal/adminauth/repo_pg.go                  NEW  Querier port + PgRepo (Task 6)
internal/adminauth/repo_pg_test.go             NEW  fake-Querier mapping tests (Task 6)
internal/adminauth/usecase.go                  NEW  Service: Login (Task 7) + ChangePassword/GetProfile (Task 8)
internal/adminauth/usecase_test.go             NEW  service tests (Tasks 7-8)
internal/adminauth/handler.go                  NEW  3 HTTP handlers (Task 9)
internal/adminauth/handler_test.go             NEW  status/shape/key-order tests (Task 9)
internal/shared/router.go                      MOD  3 route fields + mount (Task 10)
internal/shared/router_phase4c_test.go         NEW  router mount tests (Task 10)
cmd/server/main.go                             MOD  compose adminauth (Task 11)
```

Implementer note for EVERY task: **stage ONLY the files named in that task's `git add`. NEVER `git add -A`.**

---

### Task 1: listquery — `Parse` + `jsParseInt`

**Files:**
- Create: `internal/shared/listquery/listquery.go`
- Create: `internal/shared/listquery/listquery_test.go`

`parseListQuery` (Node `backend/src/common/list-query.ts`) coerces raw query-string values. `page`/`limit` use JS `parseInt(str,10)` semantics — leading optional sign + digits, stop at first non-digit (`"12abc"`→12). `strconv.Atoi` is WRONG (errors on `"12abc"`). Absent/empty/non-numeric → nil pointer (Node stores `NaN`, which is falsy in the later `|| 1`/`|| 10`, behaviourally identical to "absent").

- [ ] **Step 1: Write the failing test**

```go
package listquery

import (
	"net/url"
	"reflect"
	"testing"
)

func intp(i int) *int { return &i }

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Query
	}{
		{"empty", "", Query{}},
		{"search", "search=hi", Query{Search: "hi"}},
		{"status active", "status=active", Query{Status: "active"}},
		{"status bogus dropped", "status=bogus", Query{}},
		{"sort_order lowercased", "sort_order=asc", Query{SortOrder: "ASC"}},
		{"sort_order bogus dropped", "sort_order=sideways", Query{}},
		{"sort_by", "sort_by=name", Query{SortBy: "name"}},
		{"page+limit", "page=3&limit=25", Query{Page: intp(3), Limit: intp(25)}},
		{"page jsParseInt 12abc", "page=12abc", Query{Page: intp(12)}},
		{"page non-numeric absent", "page=abc", Query{}},
		{"page float truncates", "limit=2.5", Query{Limit: intp(2)}},
		{"page negative", "page=-5", Query{Page: intp(-5)}},
		{"page empty absent", "page=", Query{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := url.ParseQuery(tt.raw)
			got := Parse(v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestJsParseInt(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"12", 12, true}, {"12abc", 12, true}, {"  3", 3, true},
		{"2.5", 2, true}, {"-5", -5, true}, {"+7", 7, true},
		{"abc", 0, false}, {"", 0, false}, {"-", 0, false}, {"   ", 0, false},
	}
	for _, tt := range tests {
		got, ok := jsParseInt(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("jsParseInt(%q) = (%d,%v), want (%d,%v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/listquery/ -run 'TestParse|TestJsParseInt' -v`
Expected: FAIL — `undefined: Query` / `undefined: Parse` / `undefined: jsParseInt`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package listquery ports backend/src/common/list-query.ts — the query
// coercion + dynamic WHERE/ORDER/LIMIT builder shared by every admin list
// endpoint. Pure (no DB): Build returns SQL fragments + args the caller runs
// via pgxpool. The allow-listed sort map is the SQL-injection guard.
package listquery

import "strings"

// Query is the coerced shape of the admin frontend's list params. Page/Limit
// are *int so "absent" is distinct from a parsed 0 (matches Node's
// undefined-vs-0 distinction before the `|| 1` / `|| 10` defaults).
type Query struct {
	Search    string // "" when absent
	Status    string // "active" | "inactive" | "all" | "" (unset)
	SortBy    string
	SortOrder string // "ASC" | "DESC" | "" (unset)
	Page      *int
	Limit     *int
}

// Parse mirrors parseListQuery: status restricted to the allow-list,
// sort_order uppercased then allow-listed, page/limit via jsParseInt.
func Parse(v map[string][]string) Query {
	get := func(k string) string {
		if vs := v[k]; len(vs) > 0 {
			return vs[0]
		}
		return ""
	}
	var q Query
	q.Search = get("search")
	if s := get("status"); s == "active" || s == "inactive" || s == "all" {
		q.Status = s
	}
	q.SortBy = get("sort_by")
	if so := strings.ToUpper(get("sort_order")); so == "ASC" || so == "DESC" {
		q.SortOrder = so
	}
	if n, ok := jsParseInt(get("page")); ok {
		q.Page = &n
	}
	if n, ok := jsParseInt(get("limit")); ok {
		q.Limit = &n
	}
	return q
}

// jsParseInt replicates JavaScript parseInt(s, 10): skip leading ASCII
// whitespace, accept one optional +/- sign, consume base-10 digits, and STOP
// at the first non-digit (so "12abc"→12). Returns ok=false when no digit is
// consumed (JS returns NaN, which the caller treats as absent).
func jsParseInt(s string) (int, bool) {
	i, n := 0, len(s)
	for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	neg := false
	if i < n && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	start := i
	val := 0
	for i < n && s[i] >= '0' && s[i] <= '9' {
		val = val*10 + int(s[i]-'0')
		i++
	}
	if i == start {
		return 0, false
	}
	if neg {
		val = -val
	}
	return val, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/shared/listquery/ -run 'TestParse|TestJsParseInt' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shared/listquery/listquery.go internal/shared/listquery/listquery_test.go
git commit -m "feat(adminfoundation): listquery Parse + jsParseInt (Node parseListQuery parity)"
```

---

### Task 2: listquery — `Build`

**Files:**
- Modify: `internal/shared/listquery/listquery.go`
- Modify: `internal/shared/listquery/listquery_test.go`

`Build` mirrors `applyListQuery`: ILIKE OR-clause across caller-supplied columns (one reused placeholder), `is_active` bool status filter, allow-listed sort (default DESC), page default 1, limit default 10 cap 100. Sort field comes ONLY from the allow-list map → never parameterized, injection-safe. Caller runs two queries sharing `Built.Where`+`Built.Args` (rows with `Order`/`Limit`/`Offset`, count without) = TypeORM `getManyAndCount`.

- [ ] **Step 1: Write the failing test**

```go
func TestBuild(t *testing.T) {
	cols := []string{"u.email", "u.name"}
	allow := map[string]string{"name": "u.name", "created": "u.created_at"}

	t.Run("defaults: no search/status/sort", func(t *testing.T) {
		b := Build(Query{}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty", b.Where)
		}
		if len(b.Args) != 0 {
			t.Errorf("Args = %v, want none", b.Args)
		}
		if b.Order != "ORDER BY u.created_at DESC" {
			t.Errorf("Order = %q", b.Order)
		}
		if b.Limit != 10 || b.Offset != 0 {
			t.Errorf("Limit=%d Offset=%d, want 10/0", b.Limit, b.Offset)
		}
	})

	t.Run("search OR-clause, single arg", func(t *testing.T) {
		b := Build(Query{Search: "  bob  "}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE (u.email ILIKE $1 OR u.name ILIKE $1)" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 1 || b.Args[0] != "%bob%" {
			t.Errorf("Args = %v, want [%%bob%%]", b.Args)
		}
	})

	t.Run("status active → is_active true", func(t *testing.T) {
		b := Build(Query{Status: "active"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE u.is_active = $1" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 1 || b.Args[0] != true {
			t.Errorf("Args = %v, want [true]", b.Args)
		}
	})

	t.Run("status all → no filter", func(t *testing.T) {
		b := Build(Query{Status: "all"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty", b.Where)
		}
	})

	t.Run("search + status combine, placeholders increment", func(t *testing.T) {
		b := Build(Query{Search: "x", Status: "inactive"}, cols, allow, "u.created_at", "u.is_active")
		if b.Where != "WHERE (u.email ILIKE $1 OR u.name ILIKE $1) AND u.is_active = $2" {
			t.Errorf("Where = %q", b.Where)
		}
		if len(b.Args) != 2 || b.Args[0] != "%x%" || b.Args[1] != false {
			t.Errorf("Args = %v", b.Args)
		}
	})

	t.Run("sort allow-list + ASC", func(t *testing.T) {
		b := Build(Query{SortBy: "name", SortOrder: "ASC"}, cols, allow, "u.created_at", "u.is_active")
		if b.Order != "ORDER BY u.name ASC" {
			t.Errorf("Order = %q", b.Order)
		}
	})

	t.Run("sort_by not in allow-list → default DESC", func(t *testing.T) {
		b := Build(Query{SortBy: "password; DROP TABLE", SortOrder: "ASC"}, cols, allow, "u.created_at", "u.is_active")
		if b.Order != "ORDER BY u.created_at ASC" {
			t.Errorf("Order = %q, injection must fall to default field (order still honored)", b.Order)
		}
	})

	t.Run("limit cap 100, page offset math", func(t *testing.T) {
		b := Build(Query{Page: intp(3), Limit: intp(250)}, cols, allow, "u.created_at", "u.is_active")
		if b.Limit != 100 {
			t.Errorf("Limit = %d, want 100", b.Limit)
		}
		if b.Offset != 200 {
			t.Errorf("Offset = %d, want (3-1)*100=200", b.Offset)
		}
	})

	t.Run("page/limit zero floor to 1", func(t *testing.T) {
		b := Build(Query{Page: intp(0), Limit: intp(0)}, cols, allow, "u.created_at", "u.is_active")
		if b.Limit != 1 || b.Offset != 0 {
			t.Errorf("Limit=%d Offset=%d, want 1/0", b.Limit, b.Offset)
		}
	})

	t.Run("status disabled when statusCol empty", func(t *testing.T) {
		b := Build(Query{Status: "active"}, cols, allow, "u.created_at", "")
		if b.Where != "" {
			t.Errorf("Where = %q, want empty when statusCol disabled", b.Where)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/listquery/ -run TestBuild -v`
Expected: FAIL — `undefined: Build` / `undefined: Built`.

- [ ] **Step 3: Write minimal implementation** (append to `listquery.go`)

```go
import (
	"fmt"
	"strings"
)
// NOTE: merge this import block with the existing one — keep a single
// `import (...)` at the top of listquery.go ("strings" is already imported).

// Built is the SQL fragments Build produces. The caller composes:
//
//	rows : SELECT <cols> FROM <base> {Where} {Order} LIMIT {Limit} OFFSET {Offset}
//	count: SELECT count(*) FROM <base> {Where}
//
// both with Args. Where is "" or starts with "WHERE "; Order always set.
type Built struct {
	Where  string
	Args   []any
	Order  string
	Limit  int
	Offset int
}

// Build mirrors applyListQuery. searchCols + sortAllow values are
// caller-supplied literals (alias.field), never user input. statusCol == ""
// disables the status filter (Node's statusColumn = null).
func Build(q Query, searchCols []string, sortAllow map[string]string, defaultSort, statusCol string) Built {
	var clauses []string
	var args []any

	if term := strings.TrimSpace(q.Search); term != "" && len(searchCols) > 0 {
		args = append(args, "%"+term+"%")
		ph := fmt.Sprintf("$%d", len(args))
		ors := make([]string, len(searchCols))
		for i, c := range searchCols {
			ors[i] = c + " ILIKE " + ph
		}
		clauses = append(clauses, "("+strings.Join(ors, " OR ")+")")
	}

	if statusCol != "" && q.Status != "" && q.Status != "all" {
		args = append(args, q.Status == "active")
		clauses = append(clauses, fmt.Sprintf("%s = $%d", statusCol, len(args)))
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	field := defaultSort
	if mapped := sortAllow[q.SortBy]; mapped != "" {
		field = mapped
	}
	order := "DESC"
	if q.SortOrder == "ASC" {
		order = "ASC"
	}

	page := 1
	if q.Page != nil && *q.Page > 1 {
		page = *q.Page
	}
	limit := 10
	if q.Limit != nil {
		limit = *q.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	return Built{
		Where:  where,
		Args:   args,
		Order:  "ORDER BY " + field + " " + order,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/shared/listquery/ -v`
Expected: PASS (all of Tasks 1-2).

- [ ] **Step 5: Commit**

```bash
git add internal/shared/listquery/listquery.go internal/shared/listquery/listquery_test.go
git commit -m "feat(adminfoundation): listquery Build (applyListQuery parity, injection-safe sort)"
```

---

### Task 3: Widen `auth.Claims` with `isAdmin`

**Files:**
- Modify: `internal/platform/auth/jwt.go:27-32`
- Modify: `internal/platform/auth/jwt_test.go`

Node's admin payload is `{sub, email, role:'admin', isAdmin:true}`. The Go `Claims` struct must decode the `isAdmin` claim so `RequireAdmin` can read it. Customer tokens lack the key → decodes `false`.

- [ ] **Step 1: Write the failing test** (append to `internal/platform/auth/jwt_test.go`)

```go
func TestVerify_DecodesIsAdminFlag(t *testing.T) {
	const secret = "test-secret-at-least-16-chars-long"
	signer := NewSigner(secret)
	tok, err := signer.Sign(Claims{Sub: "admin-1", Email: "a@b.c", Role: "admin", IsAdminFlag: true}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !claims.IsAdminFlag {
		t.Errorf("IsAdminFlag = false, want true (isAdmin claim must round-trip)")
	}
}

func TestVerify_IsAdminFlagDefaultsFalse(t *testing.T) {
	const secret = "test-secret-at-least-16-chars-long"
	tok, _ := NewSigner(secret).Sign(Claims{Sub: "user-1", Role: "user"}, time.Hour)
	claims, err := NewVerifier(secret).Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.IsAdminFlag {
		t.Errorf("IsAdminFlag = true, want false for a customer token")
	}
}
```

(If `time` is not yet imported in `jwt_test.go`, add it to the import block.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/auth/ -run TestVerify_DecodesIsAdminFlag -v`
Expected: FAIL — `unknown field 'IsAdminFlag' in struct literal`.

- [ ] **Step 3: Add the field** — replace the `Claims` struct (`jwt.go:27-32`):

```go
type Claims struct {
	Sub         string `json:"sub"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role,omitempty"`
	IsAdminFlag bool   `json:"isAdmin,omitempty"`
	jwt.RegisteredClaims
}
```

Leave the existing `UserID()` and `IsAdmin()` (role-only) methods unchanged.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/platform/auth/ -v`
Expected: PASS (new tests + existing suite).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/auth/jwt.go internal/platform/auth/jwt_test.go
git commit -m "feat(adminfoundation): decode isAdmin claim (admin JWT payload parity)"
```

---

### Task 4: `RequireAdmin` middleware

**Files:**
- Create: `internal/shared/admin_middleware.go`
- Create: `internal/shared/admin_middleware_test.go`

Mirrors Node `RolesGuard` for `@Roles('admin')`: allow iff `user.isAdmin || user.role === 'admin'`, else `ForbiddenException('Admin access required')` → 403 code `forbidden`. Runs AFTER `RequireAuth` (which stamps claims + handles the no-token 401), so it only reads `ClaimsFromContext`. A missing claim (router misconfig) → treat as 403 (defensive; never reached in wired routes).

- [ ] **Step 1: Write the failing test**

```go
package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
)

func TestRequireAdmin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })

	cases := []struct {
		name   string
		claims *auth.Claims
		status int
	}{
		{"isAdmin flag passes", &auth.Claims{Sub: "a", IsAdminFlag: true}, 204},
		{"role admin passes", &auth.Claims{Sub: "a", Role: "admin"}, 204},
		{"customer rejected", &auth.Claims{Sub: "u", Role: "user"}, http.StatusForbidden},
		{"no role rejected", &auth.Claims{Sub: "u"}, http.StatusForbidden},
		{"missing claims rejected", nil, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
			if tc.claims != nil {
				req = req.WithContext(ContextWithClaims(req.Context(), tc.claims))
			}
			RequireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestRequireAdmin_ForbiddenEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(ContextWithClaims(req.Context(), &auth.Claims{Sub: "u", Role: "user"}))
	RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !contains(body, `"error":"Admin access required"`) || !contains(body, `"code":"forbidden"`) {
		t.Errorf("body = %s, want Admin access required / forbidden", body)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/ -run TestRequireAdmin -v`
Expected: FAIL — `undefined: RequireAdmin`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package shared — admin role gate. This file: RequireAdmin, the Go port of
// the NestJS RolesGuard('admin'). Mounted AFTER RequireAuth, which stamps the
// verified claims and owns the no-token 401; RequireAdmin only checks the role.
package shared

import "net/http"

// RequireAdmin allows the request iff the verified claims carry isAdmin=true OR
// role=="admin" (Node RolesGuard: `user.isAdmin || user.role === 'admin'`).
// Otherwise it writes the 403 envelope {error:"Admin access required",
// code:"forbidden", request_id} — byte-identical to the global filter's output
// for a bare ForbiddenException('Admin access required') (inferCode(403) =
// ERROR_CODES.forbidden). A missing claim (route not wrapped by RequireAuth)
// is treated as non-admin; wired admin routes always sit behind RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		if !ok || !(c.IsAdminFlag || c.Role == "admin") {
			WriteError(w, r, "forbidden", "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/shared/ -run TestRequireAdmin -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shared/admin_middleware.go internal/shared/admin_middleware_test.go
git commit -m "feat(adminfoundation): RequireAdmin middleware (RolesGuard parity, 403 forbidden)"
```

---

### Task 5: adminauth domain + sqlc queries

**Files:**
- Create: `internal/adminauth/domain.go`
- Create: `internal/shared/pg/queries_adminauth.sql`
- Generated: `internal/shared/pg/sqlc/*` (via `sqlc generate`)

Domain types + sentinel errors (the 4 admin-auth 401 messages) + the 3 sqlc queries. `admin_users` is already in `schema.sql:112-121` (all columns NOT NULL; `id` uuid, timestamps `timestamp without time zone`). sqlc emits `id`→`pgtype.UUID`, timestamps→`pgtype.Timestamp`, rest as `string`/`bool`.

- [ ] **Step 1: Write `domain.go`**

```go
// Package adminauth ports the NestJS AdminAuthService (admin/auth) — login,
// change-password, getProfile — as a byte-identical drop-in. Admin tokens are
// signed with the SAME JWT_SECRET as customer access tokens (so the existing
// auth.Verifier validates them) but carry {role:'admin', isAdmin:true} and use
// hardcoded TTLs (access 15m prod / 24h dev, refresh 7d) instead of the
// app_settings token_expiry values. The RolesGuard equivalent is
// shared.RequireAdmin.
package adminauth

import "time"

// AdminUser mirrors the admin_users row. password_hash is carried for
// verification and stripped by the handler before serialization.
type AdminUser struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	IsActive     bool
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Error is a 401-class admin-auth failure carrying the exact Node message. The
// handler maps it to {error: Message, code: "invalid-token"} → 401, matching
// the NestJS UnauthorizedException → inferCode(401)="invalid-token".
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// The four admin-auth 401 messages, byte-identical to admin-auth.service.ts.
var (
	ErrInvalidCredentials = &Error{Message: "Invalid credentials"}        // bad email/password
	ErrAccountDisabled    = &Error{Message: "Account disabled"}           // !is_active
	ErrUnauthorized       = &Error{Message: "Unauthorized"}               // bare UnauthorizedException() (admin row gone)
	ErrCurrentPwIncorrect = &Error{Message: "Current password is incorrect"}
)
```

- [ ] **Step 2: Write `queries_adminauth.sql`**

```sql
-- internal/shared/pg/queries_adminauth.sql
-- Admin authentication (POST /admin/auth/login, change-password, GET me).
-- admin_users is the portal-admin table, separate from `users` (customers).

-- name: FindAdminByEmailLower :one
SELECT * FROM admin_users WHERE LOWER(email) = LOWER($1);

-- name: FindAdminByID :one
SELECT * FROM admin_users WHERE id = $1;

-- name: UpdateAdminPasswordHash :exec
UPDATE admin_users SET password_hash = $2, updated_at = now() WHERE id = $1;
```

- [ ] **Step 3: Generate sqlc + verify build**

Run: `sqlc generate && go build ./...`
Expected: no output (clean). New symbols exist: `sqlc.AdminUser`, `sqlc.Querier.FindAdminByEmailLower(ctx, string) (AdminUser, error)`, `FindAdminByID(ctx, pgtype.UUID) (AdminUser, error)`, `UpdateAdminPasswordHash(ctx, UpdateAdminPasswordHashParams) error` where the params struct is `{ID pgtype.UUID; PasswordHash string}`.

Verify the generated signatures before continuing:

Run: `grep -n "FindAdminByEmailLower\|FindAdminByID\|UpdateAdminPasswordHash" internal/shared/pg/sqlc/*.go`
Expected: the three methods on `Querier` + their concrete impls + `UpdateAdminPasswordHashParams`.

- [ ] **Step 4: Confirm domain compiles**

Run: `go build ./internal/adminauth/ ./internal/shared/pg/...`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/adminauth/domain.go internal/shared/pg/queries_adminauth.sql internal/shared/pg/sqlc/
git commit -m "feat(adminfoundation): adminauth domain + admin_users sqlc queries"
```

---

### Task 6: adminauth repo

**Files:**
- Create: `internal/adminauth/repo_pg.go`
- Create: `internal/adminauth/repo_pg_test.go`

`PgRepo` adapts the sqlc `Querier` to the `adminauth.Repo` port (defined here on the consumer side as a small interface). Maps `sqlc.AdminUser` → `domain.AdminUser`; `pgx.ErrNoRows` → `(nil, nil)` so the service treats "not found" without importing pgx error types.

- [ ] **Step 1: Write the failing test**

```go
package adminauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	byEmail map[string]sqlc.AdminUser
	byID    map[string]sqlc.AdminUser
	updated map[string]string // id -> new hash
}

func (f *fakeQ) FindAdminByEmailLower(_ context.Context, email string) (sqlc.AdminUser, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return sqlc.AdminUser{}, pgx.ErrNoRows
}
func (f *fakeQ) FindAdminByID(_ context.Context, id pgtype.UUID) (sqlc.AdminUser, error) {
	if u, ok := f.byID[uuidStr(id)]; ok {
		return u, nil
	}
	return sqlc.AdminUser{}, pgx.ErrNoRows
}
func (f *fakeQ) UpdateAdminPasswordHash(_ context.Context, arg sqlc.UpdateAdminPasswordHashParams) error {
	if f.updated == nil {
		f.updated = map[string]string{}
	}
	f.updated[uuidStr(arg.ID)] = arg.PasswordHash
	return nil
}

func mkUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func sampleRow() sqlc.AdminUser {
	return sqlc.AdminUser{
		ID:           mkUUID("11111111-1111-1111-1111-111111111111"),
		Email:        "Admin@DraftRight.com",
		PasswordHash: "$2a$10$hash",
		Name:         "Root",
		IsActive:     true,
		Role:         "admin",
		CreatedAt:    pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		UpdatedAt:    pgtype.Timestamp{Time: time.Unix(1700000001, 0).UTC(), Valid: true},
	}
}

func TestRepo_FindByEmailLower(t *testing.T) {
	row := sampleRow()
	r := NewPgRepo(&fakeQ{byEmail: map[string]sqlc.AdminUser{"admin@draftright.com": row}})

	got, err := r.FindByEmailLower(context.Background(), "admin@draftright.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || got.ID != "11111111-1111-1111-1111-111111111111" || got.Email != "Admin@DraftRight.com" ||
		got.Name != "Root" || !got.IsActive || got.Role != "admin" || got.PasswordHash != "$2a$10$hash" {
		t.Errorf("mapped = %+v", got)
	}
	if !got.CreatedAt.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Errorf("CreatedAt = %v", got.CreatedAt)
	}
}

func TestRepo_FindByEmailLower_NotFound(t *testing.T) {
	r := NewPgRepo(&fakeQ{})
	got, err := r.FindByEmailLower(context.Background(), "nobody@x.com")
	if err != nil || got != nil {
		t.Errorf("got (%v,%v), want (nil,nil)", got, err)
	}
}

func TestRepo_FindByID_NotFound(t *testing.T) {
	r := NewPgRepo(&fakeQ{})
	got, err := r.FindByID(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil || got != nil {
		t.Errorf("got (%v,%v), want (nil,nil)", got, err)
	}
}

func TestRepo_UpdatePasswordHash(t *testing.T) {
	q := &fakeQ{}
	r := NewPgRepo(q)
	if err := r.UpdatePasswordHash(context.Background(), "11111111-1111-1111-1111-111111111111", "$2b$10$new"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if q.updated["11111111-1111-1111-1111-111111111111"] != "$2b$10$new" {
		t.Errorf("updated = %v", q.updated)
	}
}

var _ = errors.Is // keep errors import if unused after edits
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adminauth/ -run TestRepo -v`
Expected: FAIL — `undefined: NewPgRepo` / `undefined: uuidStr`.

- [ ] **Step 3: Write minimal implementation**

```go
package adminauth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the admin-auth repo needs (consumer-side port).
type Querier interface {
	FindAdminByEmailLower(ctx context.Context, email string) (sqlc.AdminUser, error)
	FindAdminByID(ctx context.Context, id pgtype.UUID) (sqlc.AdminUser, error)
	UpdateAdminPasswordHash(ctx context.Context, arg sqlc.UpdateAdminPasswordHashParams) error
}

// PgRepo adapts the admin_users sqlc queries to the Repo port.
type PgRepo struct{ q Querier }

// NewPgRepo wires the querier.
func NewPgRepo(q Querier) *PgRepo { return &PgRepo{q: q} }

// FindByEmailLower returns the admin matching LOWER(email), or (nil, nil) when
// none exists.
func (r *PgRepo) FindByEmailLower(ctx context.Context, email string) (*AdminUser, error) {
	row, err := r.q.FindAdminByEmailLower(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a := mapRow(row)
	return &a, nil
}

// FindByID returns the admin by id, or (nil, nil) when none exists.
func (r *PgRepo) FindByID(ctx context.Context, id string) (*AdminUser, error) {
	row, err := r.q.FindAdminByID(ctx, toUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a := mapRow(row)
	return &a, nil
}

// UpdatePasswordHash sets a new bcrypt hash (and bumps updated_at via the query).
func (r *PgRepo) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	return r.q.UpdateAdminPasswordHash(ctx, sqlc.UpdateAdminPasswordHashParams{
		ID:           toUUID(id),
		PasswordHash: hash,
	})
}

func mapRow(row sqlc.AdminUser) AdminUser {
	return AdminUser{
		ID:           uuidStr(row.ID),
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		IsActive:     row.IsActive,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func toUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if s == "" {
		return u
	}
	parsed, err := uuid.Parse(s)
	if err != nil {
		return u
	}
	u.Bytes = parsed
	u.Valid = true
	return u
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adminauth/ -run TestRepo -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adminauth/repo_pg.go internal/adminauth/repo_pg_test.go
git commit -m "feat(adminfoundation): adminauth PgRepo (sqlc→domain mapping)"
```

---

### Task 7: adminauth Service — Login

**Files:**
- Create: `internal/adminauth/usecase.go`
- Create: `internal/adminauth/usecase_test.go`

`Service.Login` mirrors `admin-auth.service.ts login()`: trim email, LOWER match, not-found → `Invalid credentials`, bad bcrypt → `Invalid credentials`, `!is_active` → `Account disabled` (in that order). Mints `{sub,email,role,isAdmin:true}` — access via `JWT_SECRET` (TTL 15m prod / 24h dev), refresh via `JWT_REFRESH_SECRET` (TTL 7d). Holds two `*platauth.Signer` + `isProd` directly (mirrors `internal/auth` Service; no Minter port).

- [ ] **Step 1: Write the failing test**

```go
package adminauth

import (
	"context"
	"testing"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

const (
	accSecret = "access-secret-at-least-16-chars"
	refSecret = "refresh-secret-at-least-16-char"
)

type fakeRepo struct {
	byEmail map[string]*AdminUser
	byID    map[string]*AdminUser
	newHash string
}

func (f *fakeRepo) FindByEmailLower(_ context.Context, email string) (*AdminUser, error) {
	return f.byEmail[email], nil
}
func (f *fakeRepo) FindByID(_ context.Context, id string) (*AdminUser, error) { return f.byID[id], nil }
func (f *fakeRepo) UpdatePasswordHash(_ context.Context, _, hash string) error {
	f.newHash = hash
	return nil
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := shared.HashPassword(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}

func newSvc(repo Repo, isProd bool) *Service {
	return NewService(repo, accSecret, refSecret, isProd)
}

func TestLogin_Success(t *testing.T) {
	admin := &AdminUser{ID: "a1", Email: "Admin@DraftRight.com", PasswordHash: mustHash(t, "pw"), Name: "Root", IsActive: true, Role: "admin"}
	svc := newSvc(&fakeRepo{byEmail: map[string]*AdminUser{"admin@draftright.com": admin}}, false)

	res, err := svc.Login(context.Background(), "  Admin@DraftRight.com  ", "pw")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.User.ID != "a1" || res.User.Email != "Admin@DraftRight.com" || res.User.Name != "Root" || res.User.Role != "admin" {
		t.Errorf("user = %+v", res.User)
	}
	// access token must verify with the access secret and carry isAdmin + role.
	claims, err := platauth.NewVerifier(accSecret).Verify(res.AccessToken)
	if err != nil {
		t.Fatalf("verify access: %v", err)
	}
	if claims.Sub != "a1" || claims.Role != "admin" || !claims.IsAdminFlag {
		t.Errorf("access claims = %+v", claims)
	}
	// refresh token must verify with the refresh secret, NOT the access secret.
	if _, err := platauth.NewVerifier(refSecret).Verify(res.RefreshToken); err != nil {
		t.Fatalf("verify refresh: %v", err)
	}
	if _, err := platauth.NewVerifier(accSecret).Verify(res.RefreshToken); err == nil {
		t.Error("refresh token verified with access secret — secrets not separated")
	}
}

func TestLogin_Errors(t *testing.T) {
	good := &AdminUser{ID: "a1", Email: "a@b.c", PasswordHash: mustHash(t, "pw"), IsActive: true, Role: "admin"}
	disabled := &AdminUser{ID: "a2", Email: "d@b.c", PasswordHash: mustHash(t, "pw"), IsActive: false, Role: "admin"}
	svc := newSvc(&fakeRepo{byEmail: map[string]*AdminUser{"a@b.c": good, "d@b.c": disabled}}, false)

	cases := []struct {
		name, email, pw string
		want            *Error
	}{
		{"no user", "ghost@b.c", "pw", ErrInvalidCredentials},
		{"bad password", "a@b.c", "wrong", ErrInvalidCredentials},
		{"disabled", "d@b.c", "pw", ErrAccountDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Login(context.Background(), tc.email, tc.pw)
			if err != tc.want {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adminauth/ -run TestLogin -v`
Expected: FAIL — `undefined: Repo` / `undefined: NewService` / `undefined: Service`.

- [ ] **Step 3: Write minimal implementation**

```go
package adminauth

import (
	"context"
	"strings"
	"time"

	platauth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Repo is the persistence port (consumer-side). Satisfied by *PgRepo.
type Repo interface {
	FindByEmailLower(ctx context.Context, email string) (*AdminUser, error) // nil,nil if none
	FindByID(ctx context.Context, id string) (*AdminUser, error)            // nil,nil if none
	UpdatePasswordHash(ctx context.Context, id, hash string) error
}

const (
	accessTTLProd = 15 * time.Minute
	accessTTLDev  = 24 * time.Hour
	refreshTTL    = 7 * 24 * time.Hour
)

// Service orchestrates admin-auth flows. Two signers (access/refresh secrets)
// + isProd select the access TTL. Mirrors internal/auth.Service, minus the
// settings-backed TTLReader (admin TTLs are hardcoded by environment).
type Service struct {
	repo    Repo
	access  *platauth.Signer
	refresh *platauth.Signer
	isProd  bool
}

// NewService wires the repo + the two signing secrets. accessSecret signs admin
// access tokens (same JWT_SECRET as customers); refreshSecret signs refresh.
func NewService(repo Repo, accessSecret, refreshSecret string, isProd bool) *Service {
	return &Service{
		repo:    repo,
		access:  platauth.NewSigner(accessSecret),
		refresh: platauth.NewSigner(refreshSecret),
		isProd:  isProd,
	}
}

// LoginResult is the {access_token, refresh_token, user} the handler shapes.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         AdminUser
}

// Login authenticates an admin and mints the token pair.
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	admin, err := s.repo.FindByEmailLower(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return LoginResult{}, err
	}
	if admin == nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	ok, err := shared.VerifyPassword(password, admin.PasswordHash)
	if err != nil {
		return LoginResult{}, err
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !admin.IsActive {
		return LoginResult{}, ErrAccountDisabled
	}
	access, refresh, err := s.generateTokens(admin)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccessToken: access, RefreshToken: refresh, User: *admin}, nil
}

// generateTokens mirrors Node generateTokens: payload {sub,email,role,isAdmin:true};
// access signed with JWT_SECRET (15m prod / 24h dev), refresh with
// JWT_REFRESH_SECRET (7d).
func (s *Service) generateTokens(a *AdminUser) (access, refresh string, err error) {
	claims := platauth.Claims{Sub: a.ID, Email: a.Email, Role: a.Role, IsAdminFlag: true}
	accTTL := accessTTLDev
	if s.isProd {
		accTTL = accessTTLProd
	}
	access, err = s.access.Sign(claims, accTTL)
	if err != nil {
		return "", "", err
	}
	refresh, err = s.refresh.Sign(claims, refreshTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}
```

NOTE on the `FindByEmailLower` arg: the service lower-cases+trims the email and the SQL also wraps both sides in `LOWER()`, so the query's `LOWER($1)` is applied to an already-lowercased string — idempotent, byte-identical result to Node's `LOWER(a.email)=LOWER(:email)` with `email.trim()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adminauth/ -run TestLogin -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/adminauth/usecase.go internal/adminauth/usecase_test.go
git commit -m "feat(adminfoundation): adminauth Service.Login (admin-auth.service parity)"
```

---

### Task 8: adminauth Service — ChangePassword + GetProfile

**Files:**
- Modify: `internal/adminauth/usecase.go`
- Modify: `internal/adminauth/usecase_test.go`

`changePassword`: load by id → missing → bare `Unauthorized`; wrong current → `Current password is incorrect`; else hash + update. `getProfile`: load by id → missing → bare `Unauthorized`; else return the admin (handler strips `password_hash`).

- [ ] **Step 1: Write the failing test** (append to `usecase_test.go`)

```go
func TestChangePassword(t *testing.T) {
	admin := &AdminUser{ID: "a1", PasswordHash: mustHash(t, "old"), Role: "admin", IsActive: true}
	repo := &fakeRepo{byID: map[string]*AdminUser{"a1": admin}}
	svc := newSvc(repo, false)

	t.Run("missing admin → Unauthorized", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), "ghost", "old", "new")
		if err != ErrUnauthorized {
			t.Errorf("err = %v, want Unauthorized", err)
		}
	})
	t.Run("wrong current → message", func(t *testing.T) {
		err := svc.ChangePassword(context.Background(), "a1", "WRONG", "new")
		if err != ErrCurrentPwIncorrect {
			t.Errorf("err = %v, want Current password is incorrect", err)
		}
	})
	t.Run("success hashes + persists new", func(t *testing.T) {
		if err := svc.ChangePassword(context.Background(), "a1", "old", "brand-new-pw"); err != nil {
			t.Fatalf("err: %v", err)
		}
		ok, _ := shared.VerifyPassword("brand-new-pw", repo.newHash)
		if !ok {
			t.Errorf("stored hash does not verify against new password")
		}
	})
}

func TestGetProfile(t *testing.T) {
	admin := &AdminUser{ID: "a1", Email: "a@b.c", Name: "Root", IsActive: true, Role: "admin", PasswordHash: "$2a$10$x"}
	svc := newSvc(&fakeRepo{byID: map[string]*AdminUser{"a1": admin}}, false)

	t.Run("missing → Unauthorized", func(t *testing.T) {
		_, err := svc.GetProfile(context.Background(), "ghost")
		if err != ErrUnauthorized {
			t.Errorf("err = %v, want Unauthorized", err)
		}
	})
	t.Run("success returns admin", func(t *testing.T) {
		got, err := svc.GetProfile(context.Background(), "a1")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got.ID != "a1" || got.Email != "a@b.c" || got.Name != "Root" || got.Role != "admin" {
			t.Errorf("profile = %+v", got)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adminauth/ -run 'TestChangePassword|TestGetProfile' -v`
Expected: FAIL — `svc.ChangePassword undefined` / `svc.GetProfile undefined`.

- [ ] **Step 3: Append the methods to `usecase.go`**

```go
// ChangePassword verifies the current password and stores a new bcrypt hash.
// Missing admin → ErrUnauthorized (bare Node UnauthorizedException()); wrong
// current → ErrCurrentPwIncorrect.
func (s *Service) ChangePassword(ctx context.Context, adminID, current, next string) error {
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrUnauthorized
	}
	ok, err := shared.VerifyPassword(current, admin.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCurrentPwIncorrect
	}
	hash, err := shared.HashPassword(next)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordHash(ctx, adminID, hash)
}

// GetProfile loads the admin by id. Missing → ErrUnauthorized. The handler
// strips password_hash before serializing.
func (s *Service) GetProfile(ctx context.Context, adminID string) (AdminUser, error) {
	admin, err := s.repo.FindByID(ctx, adminID)
	if err != nil {
		return AdminUser{}, err
	}
	if admin == nil {
		return AdminUser{}, ErrUnauthorized
	}
	return *admin, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adminauth/ -v`
Expected: PASS (all adminauth tests so far).

- [ ] **Step 5: Commit**

```bash
git add internal/adminauth/usecase.go internal/adminauth/usecase_test.go
git commit -m "feat(adminfoundation): adminauth Service.ChangePassword + GetProfile"
```

---

### Task 9: adminauth HTTP handlers

**Files:**
- Create: `internal/adminauth/handler.go`
- Create: `internal/adminauth/handler_test.go`

Three handlers. Login: 201, body `{access_token, refresh_token, user:{id,email,name,role}}`. ChangePassword: 201, `{success:true}`. Me: 200, `{id,email,name,is_active,role,created_at,updated_at}` (timestamps via `shared.ISOMillis`). `*Error` → `invalid-token` 401; unexpected → `internal` 500. ChangePassword/Me read the admin id from `shared.ClaimsFromContext` (routes sit behind RequireAuth). Login reads `{email,password}` with plain `json.Unmarshal` (no `DisallowUnknownFields` — Node's inline body type isn't a validated DTO).

- [ ] **Step 1: Write the failing test**

```go
package adminauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"
	"github.com/tannpv/draftright-rewrite/internal/shared"
)

type fakeSvc struct {
	login   LoginResult
	loginEr error
	cpEr    error
	prof    AdminUser
	profEr  error
}

func (f *fakeSvc) Login(context.Context, string, string) (LoginResult, error) {
	return f.login, f.loginEr
}
func (f *fakeSvc) ChangePassword(context.Context, string, string, string) error { return f.cpEr }
func (f *fakeSvc) GetProfile(context.Context, string) (AdminUser, error)        { return f.prof, f.profEr }

func TestLoginHandler_Success(t *testing.T) {
	h := &Handler{svc: &fakeSvc{login: LoginResult{
		AccessToken: "acc", RefreshToken: "ref",
		User: AdminUser{ID: "a1", Email: "a@b.c", Name: "Root", Role: "admin"},
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"a@b.c","password":"pw"}`))
	h.Login(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	// key order must be access_token, refresh_token, user{id,email,name,role}
	want := `{"access_token":"acc","refresh_token":"ref","user":{"id":"a1","email":"a@b.c","name":"Root","role":"admin"}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestLoginHandler_BadCreds(t *testing.T) {
	h := &Handler{svc: &fakeSvc{loginEr: ErrInvalidCredentials}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(`{"email":"x","password":"y"}`))
	h.Login(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if b := rec.Body.String(); !strings.Contains(b, `"error":"Invalid credentials"`) || !strings.Contains(b, `"code":"invalid-token"`) {
		t.Errorf("body = %s", b)
	}
}

func TestChangePasswordHandler_Success(t *testing.T) {
	h := &Handler{svc: &fakeSvc{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/change-password", strings.NewReader(`{"current_password":"a","new_password":"b"}`))
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.ChangePassword(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"success":true}` {
		t.Errorf("body = %s, want {\"success\":true}", got)
	}
}

func TestMeHandler_Success(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 678000000, time.UTC)
	updated := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)
	h := &Handler{svc: &fakeSvc{prof: AdminUser{
		ID: "a1", Email: "a@b.c", Name: "Root", IsActive: true, Role: "admin",
		PasswordHash: "$2a$10$secret", CreatedAt: created, UpdatedAt: updated,
	}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.Me(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") || strings.Contains(body, "secret") {
		t.Fatalf("password_hash leaked: %s", body)
	}
	want := `{"id":"a1","email":"a@b.c","name":"Root","is_active":true,"role":"admin","created_at":"2026-01-02T03:04:05.678Z","updated_at":"2026-01-02T03:04:06.000Z"}`
	if got := strings.TrimSpace(body); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
	// sanity: key order is fixed (ordered struct, not a map)
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestMeHandler_AdminGone(t *testing.T) {
	h := &Handler{svc: &fakeSvc{profEr: ErrUnauthorized}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/auth/me", nil)
	req = req.WithContext(shared.ContextWithClaims(req.Context(), &auth.Claims{Sub: "a1", IsAdminFlag: true}))
	h.Me(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if b := rec.Body.String(); !strings.Contains(b, `"error":"Unauthorized"`) || !strings.Contains(b, `"code":"invalid-token"`) {
		t.Errorf("body = %s", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adminauth/ -run 'Handler' -v`
Expected: FAIL — `undefined: Handler`.

- [ ] **Step 3: Write minimal implementation**

```go
package adminauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// adminService is the handler's consumer-side port (Service satisfies it).
type adminService interface {
	Login(ctx context.Context, email, password string) (LoginResult, error)
	ChangePassword(ctx context.Context, adminID, current, next string) error
	GetProfile(ctx context.Context, adminID string) (AdminUser, error)
}

// Handler serves /admin/auth/{login,change-password,me}. Login is public;
// change-password + me sit behind shared.RequireAuth + shared.RequireAdmin, so
// they read the admin id from the verified claims.
type Handler struct{ svc adminService }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// --- request bodies (plain decode, no DisallowUnknownFields: Node's inline
// body type is not a validated DTO) ---

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePwBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// --- response bodies (ordered structs = fixed Node key order) ---

type loginUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type loginResp struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	User         loginUser `json:"user"`
}

type successResp struct {
	Success bool `json:"success"`
}

type meResp struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Login → POST /admin/auth/login (public). 201 on success.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	res, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeAdminErr(w, r, err, "login failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, loginResp{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		User:         loginUser{ID: res.User.ID, Email: res.User.Email, Name: res.User.Name, Role: res.User.Role},
	})
}

// ChangePassword → POST /admin/auth/change-password (admin). 201 {success:true}.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminIDFromCtx(r)
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body changePwBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), adminID, body.CurrentPassword, body.NewPassword); err != nil {
		writeAdminErr(w, r, err, "change-password failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, successResp{Success: true})
}

// Me → GET /admin/auth/me (admin). 200, password_hash stripped.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	adminID, ok := adminIDFromCtx(r)
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	a, err := h.svc.GetProfile(r.Context(), adminID)
	if err != nil {
		writeAdminErr(w, r, err, "profile failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, meResp{
		ID:        a.ID,
		Email:     a.Email,
		Name:      a.Name,
		IsActive:  a.IsActive,
		Role:      a.Role,
		CreatedAt: shared.ISOMillis(a.CreatedAt),
		UpdatedAt: shared.ISOMillis(a.UpdatedAt),
	})
}

// adminIDFromCtx pulls the verified admin id (sub claim) stamped by RequireAuth.
func adminIDFromCtx(r *http.Request) (string, bool) {
	c, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		return "", false
	}
	return c.Sub, true
}

// writeAdminErr maps a service error to its envelope. *Error → invalid-token
// 401 (the four admin-auth messages); anything else → internal 500.
func writeAdminErr(w http.ResponseWriter, r *http.Request, err error, internalMsg string) {
	var ae *Error
	if errors.As(err, &ae) {
		shared.WriteError(w, r, "invalid-token", ae.Message)
		return
	}
	shared.WriteError(w, r, "internal", internalMsg)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adminauth/ -v`
Expected: PASS (full adminauth suite).

- [ ] **Step 5: Commit**

```bash
git add internal/adminauth/handler.go internal/adminauth/handler_test.go
git commit -m "feat(adminfoundation): adminauth HTTP handlers (status/shape/key-order parity)"
```

---

### Task 10: Router wiring

**Files:**
- Modify: `internal/shared/router.go` (add 3 fields + mounts)
- Create: `internal/shared/router_phase4c_test.go`

Add `AdminLogin` (public, mounted before the auth group) and `AdminChangePassword` + `AdminMe` (behind a new group using `jwtMW` then `RequireAdmin`). All nil-guarded.

- [ ] **Step 1: Write the failing test**

```go
package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
}

func TestRouter_AdminLoginPublic(t *testing.T) {
	r := &Router{
		Verifier:  auth.NewVerifier("test-secret-at-least-16-chars-x"),
		AdminLogin: okHandler(201),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/admin/auth/login", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Errorf("login status = %d, want 201 (public, no JWT)", resp.StatusCode)
	}
}

func TestRouter_AdminRoutesGuarded(t *testing.T) {
	r := &Router{
		Verifier:            auth.NewVerifier("test-secret-at-least-16-chars-x"),
		AdminChangePassword: okHandler(201),
		AdminMe:             okHandler(200),
	}
	srv := httptest.NewServer(r.Build())
	defer srv.Close()

	// No JWT → RequireAuth rejects with 401 (never reaches the handler).
	for _, p := range []struct {
		method, path string
	}{
		{http.MethodGet, "/admin/auth/me"},
		{http.MethodPost, "/admin/auth/change-password"},
	} {
		req, _ := http.NewRequest(p.method, srv.URL+p.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", p.method, p.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s %s status = %d, want 401 without JWT", p.method, p.path, resp.StatusCode)
		}
	}
}
```

(`auth` is already imported in the `shared` package's router.go; the test file is in `package shared`, so reference `auth.NewVerifier` directly. If the test file needs the import, add `auth "github.com/tannpv/draftright-rewrite/internal/platform/auth"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shared/ -run 'TestRouter_Admin' -v`
Expected: FAIL — `unknown field 'AdminLogin' in struct literal of type Router`.

- [ ] **Step 3: Add the three fields** to the `Router` struct (after the Phase 4b block, before `EnableTracing`, `router.go:124`):

```go
	// Phase 4c-1 admin foundation. AdminLogin is PUBLIC (mounted before the
	// auth group). AdminChangePassword + AdminMe sit behind RequireAuth THEN
	// RequireAdmin (the RolesGuard('admin') equivalent). All nil-guarded.
	AdminLogin          http.Handler // POST   /admin/auth/login           (public)
	AdminChangePassword http.Handler // POST   /admin/auth/change-password (admin)
	AdminMe             http.Handler // GET    /admin/auth/me              (admin)
```

- [ ] **Step 4: Mount the public login** — add inside `Build`, in the public block (e.g. after the feedback mounts, `router.go:252`):

```go
	if r.AdminLogin != nil {
		mux.Method(http.MethodPost, "/admin/auth/login", r.AdminLogin)
	}
```

- [ ] **Step 5: Mount the guarded admin group** — add a new group AFTER the existing JWT group (`router.go:317`, after the closing `})` of `mux.Group`):

```go
	// Admin group: RequireAuth (401 on no/invalid token) THEN RequireAdmin
	// (403 'Admin access required' for non-admin tokens).
	mux.Group(func(admin chi.Router) {
		admin.Use(jwtMW)
		admin.Use(RequireAdmin)
		if r.AdminChangePassword != nil {
			admin.Method(http.MethodPost, "/admin/auth/change-password", r.AdminChangePassword)
		}
		if r.AdminMe != nil {
			admin.Method(http.MethodGet, "/admin/auth/me", r.AdminMe)
		}
	})
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/shared/ -run 'TestRouter_Admin' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/shared/router.go internal/shared/router_phase4c_test.go
git commit -m "feat(adminfoundation): mount /admin/auth routes (login public, me+change-password guarded)"
```

---

### Task 11: Compose in `main.go`

**Files:**
- Modify: `cmd/server/main.go` (inside `composeDeps`, the `pool != nil` block; assign Router fields)

Build the adminauth repo → service → handler and assign `AdminLogin`/`AdminChangePassword`/`AdminMe`. Reuse the existing sqlc `Queries` (the `pool`-backed querier other modules use) + `cfg.JWTSecret` / `cfg.JWTRefreshSecret` / `cfg.IsProduction()`.

- [ ] **Step 1: Locate the wiring site**

Run: `grep -n "feedback.NewHandler\|feedback.NewService\|FeedbackCreate\|queries :=\|sqlc.New\|pool != nil\|JWTSecret\|router\.\|rt\.\|\.Login =" cmd/server/main.go`
Expected: shows the sqlc querier variable name (e.g. `queries`/`q`), the `Router`/`rt` variable, and how a recent module (feedback) is built + assigned. Match those exact names.

- [ ] **Step 2: Add the adminauth wiring**

Place beside the other feature wiring inside the `pool != nil` branch, using the SAME querier + router variable names found in Step 1 (shown here as `queries` and `rt`):

```go
	// Phase 4c-1 admin foundation: admin-auth endpoints.
	adminAuthSvc := adminauth.NewService(
		adminauth.NewPgRepo(queries),
		cfg.JWTSecret,
		cfg.JWTRefreshSecret,
		cfg.IsProduction(),
	)
	adminAuthHandler := adminauth.NewHandler(adminAuthSvc)
	rt.AdminLogin = http.HandlerFunc(adminAuthHandler.Login)
	rt.AdminChangePassword = http.HandlerFunc(adminAuthHandler.ChangePassword)
	rt.AdminMe = http.HandlerFunc(adminAuthHandler.Me)
```

Add the import `"github.com/tannpv/draftright-rewrite/internal/adminauth"` to `main.go`'s import block (gofmt will order it).

NOTE: `adminauth.NewPgRepo(queries)` requires `queries` to satisfy `adminauth.Querier` (the 3 admin methods). After Task 5's `sqlc generate`, the concrete `*sqlc.Queries` has them. If main wires a hand-narrowed querier interface instead of `*sqlc.Queries`, pass the concrete `*sqlc.Queries` to `NewPgRepo` (it accepts the small `Querier` port).

- [ ] **Step 3: Build + vet + fmt + full suite**

Run:
```bash
go build ./... && go vet ./... && gofmt -l . && go test ./... -race
```
Expected: `go build` clean; `go vet` clean; `gofmt -l .` prints nothing; all tests PASS.

- [ ] **Step 4: Smoke-check the routes are wired** (optional, if a dev DB is up — do NOT hit prod)

Run: `grep -n "AdminLogin\|AdminMe\|AdminChangePassword" cmd/server/main.go internal/shared/router.go`
Expected: fields assigned in main.go AND mounted in router.go.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(adminfoundation): wire adminauth into composition root"
```

---

## Self-Review

**Spec coverage:**
- 3 admin/auth endpoints (login 201 / change-pw 201 / me 200) → Tasks 7-9. ✓
- Error parity (Invalid credentials / Account disabled / Unauthorized / Current password is incorrect, all invalid-token 401) → Tasks 7-9. ✓
- Admin token payload `{sub,email,role,isAdmin:true}`, access 15m-prod/24h-dev JWT_SECRET, refresh 7d JWT_REFRESH_SECRET → Task 7. ✓
- login resp `{access_token,refresh_token,user{id,email,name,role}}`; me `{id,email,name,is_active,role,created_at,updated_at}` (password_hash stripped) → Task 9. ✓
- RequireAdmin (isAdmin||role==admin, else 403 forbidden "Admin access required") → Task 4. ✓
- isAdmin claim widening → Task 3. ✓
- listquery Parse+Build (jsParseInt, status allow-list, sort allow-list+DESC default, page 1 / limit 10 cap 100) → Tasks 1-2. ✓
- sqlc 3 queries + admin_users mapping → Tasks 5-6. ✓
- Router mount (login public, 2 guarded) → Task 10. ✓
- Composition → Task 11. ✓
- Gate (build/test/gofmt/vet/sqlc) → Task 11 + Task 5. ✓

**Type consistency:** `Repo`/`Querier`/`Service`/`Handler`/`AdminUser`/`Error`/`LoginResult` names consistent across Tasks 5-9. `NewService(repo, accessSecret, refreshSecret, isProd)` defined Task 7, called Task 11. `NewPgRepo(Querier)` defined Task 6, called Task 11. `NewHandler(*Service)` defined Task 9, called Task 11. `shared.RequireAdmin` defined Task 4, used Task 10. `auth.Claims.IsAdminFlag` defined Task 3, used Tasks 4 & 7. Router fields `AdminLogin/AdminChangePassword/AdminMe` defined Task 10, assigned Task 11. ✓

**Placeholder scan:** none — every code step is complete. `queries`/`rt` in Task 11 are explicitly "use the names from Step 1's grep" (real codebase variables), not placeholders.

## Execution note (parity caveat carried from spec)

Known malformed-input divergence: Node admin login on a missing `email` throws `TypeError` → 500; Go treats missing email as `""` → 401 `Invalid credentials`. Adversarial-only path; the live shadow gate (well-formed traffic) never exercises it. Documented, not fixed.
