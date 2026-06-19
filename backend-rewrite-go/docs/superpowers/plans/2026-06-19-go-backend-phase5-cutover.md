# Go Backend Phase 5 Cutover — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the Go backend byte-identical to Node across all ~100 routes under real DB writes, then cut production over to Go with instant rollback.

**Architecture:** A `shadowdiff` harness resets two isolated DBs (one per backend) from a frozen `draftright_dev` clone before every fixture, replays each fixture against both backends, and diffs status + JSON. Once every route's fixture is green on dev, an operator flips one Caddy block to Go, soaks 7 days, then decommissions Node.

**Tech Stack:** Go 1.26 (`cmd/shadowdiff`), Postgres 16 (`TEMPLATE` clones + `pg_terminate_backend`), Docker Compose, Caddy.

**Spec:** `docs/superpowers/specs/2026-06-19-go-backend-phase5-cutover-design.md`

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `cmd/shadowdiff/subst.go` | expand `{{user_token}}`/`{{admin_token}}`/`{{ext_token}}` in a fixture | create |
| `cmd/shadowdiff/subst_test.go` | substitution unit tests | create |
| `cmd/shadowdiff/reset.go` | reset-SQL generation + maintenance-conn executor (terminate/drop/create) | create |
| `cmd/shadowdiff/reset_test.go` | reset-SQL generation unit tests | create |
| `cmd/shadowdiff/bootstrap.go` | mint user/admin/ext tokens from a live backend at run start | create |
| `cmd/shadowdiff/bootstrap_test.go` | bootstrap response-parsing unit tests | create |
| `cmd/shadowdiff/coverage.go` | load canonical route list, assert every route has a fixture | create |
| `cmd/shadowdiff/coverage_test.go` | coverage-gap detection unit tests | create |
| `cmd/shadowdiff/main.go` | wire reset/subst/bootstrap/coverage + new flags into the run loop | modify |
| `cmd/shadowdiff/fixtures/<module>/*.json` | one fixture per route | create (~100) |
| `deploy/shadow/routes.txt` | canonical `METHOD /path` list from `router.go` | create |
| `deploy/shadow/augment.sql` | idempotent known creds + representative rows | create |
| `deploy/shadow/make-template.sh` | clone dev DB → augment → freeze template | create |
| `deploy/shadow/docker-compose.shadow.yml` | `backend-node-shadow` + `backend-go-shadow` | create |
| `deploy/shadow/run-gate.sh` | template → up shadows → run shadowdiff | create |
| `deploy/phase5-cutover-runbook.md` | manual gated prod flip + soak + teardown | create |

The `pg` driver is already a dependency (`github.com/jackc/pgx/v5`). `reset.go` uses `pgx` directly on a maintenance connection.

---

## PART 1 — Harness extensions (no infra, pure Go + TDD)

### Task 1: Token substitution

**Files:**
- Create: `cmd/shadowdiff/subst.go`
- Test: `cmd/shadowdiff/subst_test.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import "testing"

func TestSubstitute_HeaderAndBody(t *testing.T) {
	f := fixture{
		Headers: map[string]string{"Authorization": "Bearer {{user_token}}"},
		Body:    []byte(`{"token":"{{ext_token}}"}`),
	}
	vars := map[string]string{"user_token": "AAA", "ext_token": "BBB"}
	out := substitute(f, vars)
	if out.Headers["Authorization"] != "Bearer AAA" {
		t.Fatalf("header = %q", out.Headers["Authorization"])
	}
	if string(out.Body) != `{"token":"BBB"}` {
		t.Fatalf("body = %q", out.Body)
	}
}

func TestSubstitute_UnknownPlaceholderLeftIntact(t *testing.T) {
	f := fixture{Headers: map[string]string{"X": "{{nope}}"}}
	out := substitute(f, map[string]string{"user_token": "AAA"})
	if out.Headers["X"] != "{{nope}}" {
		t.Fatalf("unknown placeholder should be left as-is, got %q", out.Headers["X"])
	}
}

func TestSubstitute_DoesNotMutateInput(t *testing.T) {
	f := fixture{Headers: map[string]string{"Authorization": "Bearer {{user_token}}"}}
	_ = substitute(f, map[string]string{"user_token": "AAA"})
	if f.Headers["Authorization"] != "Bearer {{user_token}}" {
		t.Fatal("input fixture must not be mutated")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/shadowdiff/ -run TestSubstitute -v`
Expected: FAIL — `undefined: substitute`

- [ ] **Step 3: Write minimal implementation**

```go
package main

import "strings"

// substitute returns a copy of f with every {{key}} in headers and body
// replaced by vars[key]. Unknown placeholders are left untouched so a
// missing token surfaces as a real request error, not a silent empty value.
// The input fixture is never mutated (headers map is copied).
func substitute(f fixture, vars map[string]string) fixture {
	repl := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{{"+k+"}}", v)
		}
		return s
	}
	out := f
	if f.Headers != nil {
		out.Headers = make(map[string]string, len(f.Headers))
		for k, v := range f.Headers {
			out.Headers[k] = repl(v)
		}
	}
	if len(f.Body) > 0 {
		out.Body = []byte(repl(string(f.Body)))
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/shadowdiff/ -run TestSubstitute -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/shadowdiff/subst.go cmd/shadowdiff/subst_test.go
git commit -m "feat(shadowdiff): fixture token substitution"
```

---

### Task 2: Reset-SQL generation

**Files:**
- Create: `cmd/shadowdiff/reset.go`
- Test: `cmd/shadowdiff/reset_test.go`

This task only covers the **pure SQL-string generation** (unit-testable). The executor that opens a maintenance connection is added in Step 3 below but exercised live in Part 2.

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"strings"
	"testing"
)

func TestResetStatements_Order(t *testing.T) {
	stmts := resetStatements("draftright_shadow_go", "draftright_shadow_tmpl")
	if len(stmts) != 3 {
		t.Fatalf("want 3 statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "pg_terminate_backend") ||
		!strings.Contains(stmts[0], "'draftright_shadow_go'") {
		t.Fatalf("stmt[0] must terminate conns to target db: %q", stmts[0])
	}
	if stmts[1] != `DROP DATABASE IF EXISTS "draftright_shadow_go"` {
		t.Fatalf("stmt[1] = %q", stmts[1])
	}
	if stmts[2] != `CREATE DATABASE "draftright_shadow_go" TEMPLATE "draftright_shadow_tmpl"` {
		t.Fatalf("stmt[2] = %q", stmts[2])
	}
}

func TestResetStatements_QuotesIdentifiers(t *testing.T) {
	// Identifiers are double-quoted so a hyphen or mixed case can't break the SQL.
	stmts := resetStatements("db-go", "tmpl")
	if !strings.Contains(stmts[1], `"db-go"`) {
		t.Fatalf("target not quoted: %q", stmts[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/shadowdiff/ -run TestResetStatements -v`
Expected: FAIL — `undefined: resetStatements`

- [ ] **Step 3: Write minimal implementation**

```go
package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// resetStatements returns the ordered SQL that drops `target` and recreates it
// from `template`. Statement order is load-bearing: terminate live sessions
// FIRST (DROP DATABASE refuses to run while connections exist), then drop, then
// clone. Identifiers are double-quoted; the datname literal in the terminate
// filter is single-quoted (it is a string, not an identifier).
func resetStatements(target, template string) []string {
	return []string{
		fmt.Sprintf(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()",
			target),
		fmt.Sprintf(`DROP DATABASE IF EXISTS %q`, target),
		fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, target, template),
	}
}

// resetDB runs resetStatements against a maintenance connection (which MUST be
// connected to a different database than `target`, e.g. "postgres"). Used live
// in Part 2; not unit-tested (needs a real server).
func resetDB(ctx context.Context, maint *pgx.Conn, target, template string) error {
	for _, s := range resetStatements(target, template) {
		if _, err := maint.Exec(ctx, s); err != nil {
			return fmt.Errorf("reset %s: %q: %w", target, s, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/shadowdiff/ -run TestResetStatements -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/shadowdiff/reset.go cmd/shadowdiff/reset_test.go
git commit -m "feat(shadowdiff): per-fixture DB reset SQL (terminate/drop/create from template)"
```

---

### Task 3: Token bootstrap (response parsing)

**Files:**
- Create: `cmd/shadowdiff/bootstrap.go`
- Test: `cmd/shadowdiff/bootstrap_test.go`

The network calls run live; the unit-testable seam is parsing each backend response into a token. Split parsing out so it can be tested without a server.

- [ ] **Step 1: Write the failing test**

```go
package main

import "testing"

func TestParseAccessToken_OK(t *testing.T) {
	tok, err := parseAccessToken([]byte(`{"access_token":"abc.def.ghi","refresh_token":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok != "abc.def.ghi" {
		t.Fatalf("tok = %q", tok)
	}
}

func TestParseAccessToken_Missing(t *testing.T) {
	if _, err := parseAccessToken([]byte(`{"refresh_token":"x"}`)); err == nil {
		t.Fatal("missing access_token must error")
	}
}

func TestParseExtToken_OK(t *testing.T) {
	// POST /auth/extension-tokens returns {"token":"dr_ext_..."}.
	tok, err := parseExtToken([]byte(`{"token":"dr_ext_abc","id":"1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if tok != "dr_ext_abc" {
		t.Fatalf("tok = %q", tok)
	}
}
```

> **Implementer note:** confirm the ext-token response key against
> `internal/extauth` (or wherever `MintExtToken` builds its body) before
> trusting `"token"`. If the real key differs, fix the test + impl to match —
> Node is the parity authority.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/shadowdiff/ -run 'TestParse(Access|Ext)Token' -v`
Expected: FAIL — `undefined: parseAccessToken`

- [ ] **Step 3: Write minimal implementation**

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func parseAccessToken(body []byte) (string, error) {
	var r struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("no access_token in response: %s", body)
	}
	return r.AccessToken, nil
}

func parseExtToken(body []byte) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.Token == "" {
		return "", fmt.Errorf("no token in ext-token response: %s", body)
	}
	return r.Token, nil
}

// bootstrapTokens logs in as the augment user + admin against `base` and mints
// an extension token, returning the substitution map. Run once per gate, before
// fixtures. Same JWT secret means these verify on both backends.
func bootstrapTokens(c *http.Client, base, userEmail, userPass, adminEmail, adminPass string) (map[string]string, error) {
	post := func(path string, payload any, hdr map[string]string) ([]byte, error) {
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s -> %d: %s", path, resp.StatusCode, body)
		}
		return body, nil
	}

	vars := map[string]string{}
	ub, err := post("/auth/login", map[string]string{"email": userEmail, "password": userPass}, nil)
	if err != nil {
		return nil, err
	}
	if vars["user_token"], err = parseAccessToken(ub); err != nil {
		return nil, err
	}
	ab, err := post("/admin/auth/login", map[string]string{"email": adminEmail, "password": adminPass}, nil)
	if err != nil {
		return nil, err
	}
	if vars["admin_token"], err = parseAccessToken(ab); err != nil {
		return nil, err
	}
	eb, err := post("/auth/extension-tokens", map[string]string{"device_name": "shadow"},
		map[string]string{"Authorization": "Bearer " + vars["user_token"]})
	if err != nil {
		return nil, err
	}
	if vars["ext_token"], err = parseExtToken(eb); err != nil {
		return nil, err
	}
	_ = time.Second
	return vars, nil
}
```

> **Implementer note:** verify the `/auth/extension-tokens` request body field
> (`device_name`?) against the Go handler `r.MintExtToken` before trusting it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/shadowdiff/ -run 'TestParse(Access|Ext)Token' -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/shadowdiff/bootstrap.go cmd/shadowdiff/bootstrap_test.go
git commit -m "feat(shadowdiff): token bootstrap (user/admin/ext) for authed fixtures"
```

---

### Task 4: Route-coverage assertion

**Files:**
- Create: `cmd/shadowdiff/coverage.go`
- Test: `cmd/shadowdiff/coverage_test.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import "testing"

func TestMissingRoutes_ReportsGaps(t *testing.T) {
	routes := []string{"POST /auth/login", "GET /plans", "GET /health"}
	fixtures := []fixture{
		{Method: "POST", Path: "/auth/login"},
		{Method: "GET", Path: "/health"},
	}
	missing := missingRoutes(routes, fixtures)
	if len(missing) != 1 || missing[0] != "GET /plans" {
		t.Fatalf("missing = %v, want [GET /plans]", missing)
	}
}

func TestMissingRoutes_PathParamsMatchByPattern(t *testing.T) {
	// A fixture hitting a concrete id must satisfy the {id} route template.
	routes := []string{"GET /admin/users/{id}"}
	fixtures := []fixture{{Method: "GET", Path: "/admin/users/abc-123"}}
	if m := missingRoutes(routes, fixtures); len(m) != 0 {
		t.Fatalf("param route should be covered, got missing %v", m)
	}
}

func TestMissingRoutes_None(t *testing.T) {
	routes := []string{"GET /health"}
	fixtures := []fixture{{Method: "GET", Path: "/health"}}
	if m := missingRoutes(routes, fixtures); len(m) != 0 {
		t.Fatalf("want none, got %v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/shadowdiff/ -run TestMissingRoutes -v`
Expected: FAIL — `undefined: missingRoutes`

- [ ] **Step 3: Write minimal implementation**

```go
package main

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
)

// loadRoutes reads "METHOD /path" lines (blank lines + #comments ignored) from
// deploy/shadow/routes.txt — the canonical route inventory.
func loadRoutes(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

// routeToRegexp turns "GET /admin/users/{id}" into a matcher that accepts any
// concrete segment in place of {id}.
func routeToRegexp(route string) *regexp.Regexp {
	parts := strings.SplitN(route, " ", 2)
	method, path := parts[0], parts[1]
	esc := regexp.QuoteMeta(path)
	esc = regexp.MustCompile(`\\\{[^/}]+\\\}`).ReplaceAllString(esc, `[^/]+`)
	return regexp.MustCompile("^" + method + " " + esc + "$")
}

// missingRoutes returns the canonical routes that no fixture exercises. A
// fixture's "METHOD /concrete/path" must match a route template (params allowed).
func missingRoutes(routes []string, fixtures []fixture) []string {
	var missing []string
	for _, route := range routes {
		re := routeToRegexp(route)
		covered := false
		for _, f := range fixtures {
			if re.MatchString(f.Method + " " + f.Path) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	return missing
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/shadowdiff/ -run TestMissingRoutes -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/shadowdiff/coverage.go cmd/shadowdiff/coverage_test.go
git commit -m "feat(shadowdiff): route-coverage assertion (fail on uncovered routes)"
```

---

### Task 5: Wire new pieces + flags into main.go

**Files:**
- Modify: `cmd/shadowdiff/main.go`

- [ ] **Step 1: Add flags + run-loop wiring**

Replace the flag block and run loop in `main()`. Add these flags after the existing three:

```go
	maintDSN := flag.String("maint-dsn", "", "maintenance Postgres DSN (postgres db) — enables per-fixture reset")
	template := flag.String("template", "draftright_shadow_tmpl", "frozen template DB name")
	dbNode := flag.String("db-node", "draftright_shadow_node", "Node backend's DB name (reset target)")
	dbGo := flag.String("db-go", "draftright_shadow_go", "Go backend's DB name (reset target)")
	routesFile := flag.String("routes", "", "canonical routes.txt — enables coverage assertion")
	userEmail := flag.String("user-email", "", "bootstrap login email")
	userPass := flag.String("user-pass", "", "bootstrap login password")
	adminEmail := flag.String("admin-email", "", "bootstrap admin email")
	adminPass := flag.String("admin-pass", "", "bootstrap admin password")
```

- [ ] **Step 2: Coverage gate (after `loadFixtures`)**

```go
	if *routesFile != "" {
		routes, err := loadRoutes(*routesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load routes: %v\n", err)
			os.Exit(2)
		}
		if miss := missingRoutes(routes, fixtures); len(miss) > 0 {
			fmt.Fprintf(os.Stderr, "coverage gap — %d routes lack a fixture:\n", len(miss))
			for _, m := range miss {
				fmt.Fprintf(os.Stderr, "   - %s\n", m)
			}
			os.Exit(2)
		}
	}
```

- [ ] **Step 3: Bootstrap tokens (before the run loop)**

```go
	vars := map[string]string{}
	if *userEmail != "" {
		vars, err = bootstrapTokens(client, *nodeBase, *userEmail, *userPass, *adminEmail, *adminPass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
			os.Exit(2)
		}
	}
```

- [ ] **Step 4: Reset + substitute inside the loop**

Open a maintenance connection once before the loop (only when `--maint-dsn` set), and reset both DBs at the top of each iteration; substitute tokens before `send`:

```go
	var maint *pgx.Conn
	if *maintDSN != "" {
		maint, err = pgx.Connect(context.Background(), *maintDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "maint connect: %v\n", err)
			os.Exit(2)
		}
		defer maint.Close(context.Background())
	}
	...
	for _, f := range fixtures {
		if maint != nil {
			if err := resetDB(context.Background(), maint, *dbNode, *template); err != nil {
				fmt.Printf("FAIL %s: reset node db: %v\n", f.Name, err)
				failed++
				continue
			}
			if err := resetDB(context.Background(), maint, *dbGo, *template); err != nil {
				fmt.Printf("FAIL %s: reset go db: %v\n", f.Name, err)
				failed++
				continue
			}
		}
		ff := substitute(f, vars)
		nStatus, nBody, err := send(client, *nodeBase, ff)
		// ... unchanged, but use ff instead of f for both sends ...
```

Add `"context"` and `"github.com/jackc/pgx/v5"` to the imports.

- [ ] **Step 5: Verify build + existing tests still pass**

Run: `go build ./cmd/shadowdiff/ && go test ./cmd/shadowdiff/ -v`
Expected: build OK, all unit tests PASS (diff, token, subst, reset, bootstrap-parse, coverage)

- [ ] **Step 6: Commit**

```bash
git add cmd/shadowdiff/main.go
git commit -m "feat(shadowdiff): wire reset/subst/bootstrap/coverage + flags into run loop"
```

---

## PART 2 — Dev shadow infra

### Task 6: Canonical route inventory

**Files:**
- Create: `deploy/shadow/routes.txt`

- [ ] **Step 1: Generate the route list from the router**

Run this and paste the cleaned output into `deploy/shadow/routes.txt`, one `METHOD /path` per line. Collapse the two `/v1/rewrite` registrations into a single `POST /v1/rewrite` line.

```bash
cd /opt/openAi/DraftRight/backend-rewrite-go
grep -oE 'http\.Method(Get|Post|Patch|Put|Delete), "[^"]*"' internal/shared/router.go \
  | sed -E 's/http\.Method([A-Za-z]+), "(.*)"/\1 \2/' | tr 'a-z' 'A-Z' \
  | sed -E 's/^([A-Z]+) /\1 /' | sort -u
```

The file begins with a comment header:

```
# Canonical route inventory — keep in sync with internal/shared/router.go.
# Format: METHOD /path  (path params as {name}). Drives shadowdiff coverage.
GET /health
POST /auth/login
...
```

- [ ] **Step 2: Verify count**

Run: `grep -vcE '^#|^$' deploy/shadow/routes.txt`
Expected: ~100 (the distinct routes; `/v1/rewrite` counted once).

- [ ] **Step 3: Commit**

```bash
git add deploy/shadow/routes.txt
git commit -m "chore(shadow): canonical route inventory for coverage assertion"
```

---

### Task 7: Augment SQL (known creds + representative rows)

**Files:**
- Create: `deploy/shadow/augment.sql`

**Goal:** make the cloned dev DB deterministic for fixtures — guarantee a known
login user, a known admin, and ≥1 row in every table a read fixture asserts
against. Idempotent (re-runnable) via `ON CONFLICT DO NOTHING` / `WHERE NOT EXISTS`.

- [ ] **Step 1: Inspect the live dev schema to get exact column names**

Run (on the VPS, against `draftright_dev`):

```bash
docker exec -i $(docker compose ps -q postgres) \
  psql -U draftright -d draftright_dev -c '\d users' -c '\d admin_users' \
  -c '\d subscriptions' -c '\d error_reports' -c '\d bug_reports' \
  -c '\d app_releases' -c '\d app_release_policies' -c '\d payment_transactions'
```

- [ ] **Step 2: Write `augment.sql`**

Author idempotent inserts. The user/admin password hashes must be real bcrypt
hashes of the shadow passwords (generate them, do NOT invent). Generate with:

```bash
cd /opt/openAi/DraftRight/backend-rewrite-go
go run ./cmd/server --hash 'ShadowPass123'   # if a hash subcommand exists; else:
# node -e "console.log(require('bcryptjs').hashSync('ShadowPass123',10))"
```

Skeleton (fill real columns/hashes from Step 1):

```sql
-- Idempotent augment applied to the cloned dev DB before freezing the template.
-- Guarantees deterministic fixtures: a known customer, a known admin, and at
-- least one representative row per entity asserted by read fixtures.
-- Re-runnable: every insert is guarded.

INSERT INTO users (id, email, password_hash, name, is_active, email_verified, created_at, updated_at)
VALUES ('00000000-0000-4000-8000-000000000001',
        'shadow-user@draftright.info', '$2a$10$REPLACE_WITH_REAL_BCRYPT',
        'Shadow User', true, true, now(), now())
ON CONFLICT (email) DO NOTHING;

INSERT INTO admin_users (id, email, password_hash, name, is_active, role, created_at, updated_at)
VALUES ('00000000-0000-4000-8000-000000000002',
        'shadow-admin@draftright.info', '$2a$10$REPLACE_WITH_REAL_BCRYPT',
        'Shadow Admin', true, 'admin', now(), now())
ON CONFLICT (email) DO NOTHING;

-- … one guarded INSERT per: subscriptions, error_reports, bug_reports,
--    app_releases (one per platform/channel a fixture reads), app_release_policies,
--    payment_transactions, ai_providers (if not already present in the dev clone).
```

> Use stable UUIDs (`…0001`, `…0002`, …) so fixtures can reference concrete ids.

- [ ] **Step 3: Commit**

```bash
git add deploy/shadow/augment.sql
git commit -m "chore(shadow): idempotent augment SQL (known creds + representative rows)"
```

---

### Task 8: Template builder

**Files:**
- Create: `deploy/shadow/make-template.sh`

- [ ] **Step 1: Write the script**

```bash
#!/usr/bin/env bash
# Builds the frozen shadow template DB from the live dev DB:
#   draftright_dev --(clone)--> draftright_shadow_tmpl --(augment.sql)-->
# Run on the VPS. Idempotent: drops + rebuilds the template each run.
#
#   ./make-template.sh
set -euo pipefail

PGC=$(docker compose ps -q postgres)
SUPER="psql -U draftright"
TMPL=draftright_shadow_tmpl
SRC=draftright_dev

run() { docker exec -i "$PGC" $SUPER -v ON_ERROR_STOP=1 "$@"; }

echo "[1/3] terminating connections to $SRC + $TMPL"
run -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN ('$SRC','$TMPL') AND pid <> pg_backend_pid();"

echo "[2/3] cloning $SRC -> $TMPL"
run -d postgres -c "DROP DATABASE IF EXISTS $TMPL;"
run -d postgres -c "CREATE DATABASE $TMPL TEMPLATE $SRC;"

echo "[3/3] applying augment.sql"
docker exec -i "$PGC" $SUPER -v ON_ERROR_STOP=1 -d "$TMPL" < "$(dirname "$0")/augment.sql"

echo "template $TMPL ready"
```

> Cloning with `TEMPLATE draftright_dev` requires no active connections to
> `draftright_dev` — step 1 terminates them. Run the gate against the shadows,
> never against `draftright_dev` itself.

- [ ] **Step 2: Make executable + commit**

```bash
chmod +x deploy/shadow/make-template.sh
git add deploy/shadow/make-template.sh
git commit -m "chore(shadow): template builder (clone dev DB + apply augment)"
```

---

### Task 9: Shadow compose (two backends, own DBs)

**Files:**
- Create: `deploy/shadow/docker-compose.shadow.yml`

- [ ] **Step 1: Write the overlay**

```yaml
# Shadow rig: Node + Go backends each on their OWN database, both cloned from
# draftright_shadow_tmpl by shadowdiff before every fixture. Runs on the dev
# VPS alongside the existing dev/prod stacks. Reuses the prod Postgres+Redis.
#
#   docker compose -f docker-compose.yml -f deploy/shadow/docker-compose.shadow.yml \
#     up -d backend-node-shadow backend-go-shadow

services:
  backend-node-shadow:
    extends:
      file: ../../docker-compose.yml
      service: backend           # the existing NestJS service
    container_name: dr-backend-node-shadow
    ports:
      - "3200:3000"
    environment:
      DATABASE_URL: "postgres://draftright:draftright@postgres:5432/draftright_shadow_node"
      NODE_ENV: "production"
    restart: "no"

  backend-go-shadow:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: dr-backend-go-shadow
    ports:
      - "3201:3001"
    environment:
      LISTEN_ADDR: ":3001"
      DATABASE_URL: "postgres://draftright:draftright@postgres:5432/draftright_shadow_go"
      # JWT_SECRET / JWT_REFRESH_SECRET / provider keys: pulled from --env-file at
      # `up` time (same values as backend-dev) so both backends share the secret.
    restart: "no"
```

> **Implementer note:** verify the NestJS service name + env var names against
> the real `docker-compose.yml` (`backend`? `backend-dev`?) and align the
> `extends`/ports. Both backends MUST receive the identical `JWT_SECRET` —
> bring them up with the same `--env-file` the dev backend uses.

- [ ] **Step 2: Commit**

```bash
git add deploy/shadow/docker-compose.shadow.yml
git commit -m "chore(shadow): two-backend compose (node+go, isolated DBs)"
```

---

### Task 10: Reset-while-connected smoke verification (the key risk)

**Files:**
- Create: `deploy/shadow/run-gate.sh` (smoke portion first)

This task proves the single riskiest assumption: a backend's pool survives its
DB being dropped + recreated under it.

- [ ] **Step 1: Write `run-gate.sh`**

```bash
#!/usr/bin/env bash
# End-to-end dev gate: build template, bring up both shadow backends, run
# shadowdiff with per-fixture reset + coverage + bootstrap. Run on the VPS.
#
#   ./run-gate.sh --env-file /opt/draftright/.env.dev
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
ENVFILE="${2:-/opt/draftright/.env.dev}"

DC="docker compose -f $ROOT/docker-compose.yml -f $HERE/docker-compose.shadow.yml --env-file $ENVFILE"
PGC=$(docker compose ps -q postgres)

echo "== build template =="
"$HERE/make-template.sh"

echo "== create initial shadow DBs from template =="
for db in draftright_shadow_node draftright_shadow_go; do
  docker exec -i "$PGC" psql -U draftright -d postgres -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS $db;" -c "CREATE DATABASE $db TEMPLATE draftright_shadow_tmpl;"
done

echo "== up shadow backends =="
$DC up -d backend-node-shadow backend-go-shadow
sleep 8   # health

echo "== run shadowdiff =="
go run "$ROOT/cmd/shadowdiff" \
  --node=http://localhost:3200 --go=http://localhost:3201 \
  --fixtures="$ROOT/cmd/shadowdiff/fixtures" \
  --routes="$HERE/routes.txt" \
  --maint-dsn="postgres://draftright:draftright@localhost:5432/postgres" \
  --template=draftright_shadow_tmpl \
  --db-node=draftright_shadow_node --db-go=draftright_shadow_go \
  --user-email=shadow-user@draftright.info  --user-pass=ShadowPass123 \
  --admin-email=shadow-admin@draftright.info --admin-pass=ShadowPass123
```

- [ ] **Step 2: Smoke-test the reset assumption (manual, on VPS)**

Bring up only `backend-go-shadow`, then from a maintenance psql run the three
reset statements against `draftright_shadow_go`, then curl an authed Go endpoint.

```bash
docker exec -i $(docker compose ps -q postgres) psql -U draftright -d postgres \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='draftright_shadow_go' AND pid<>pg_backend_pid();" \
  -c "DROP DATABASE IF EXISTS draftright_shadow_go;" \
  -c "CREATE DATABASE draftright_shadow_go TEMPLATE draftright_shadow_tmpl;"
sleep 1
curl -fsS http://localhost:3201/health   # must 200 after reset
curl -fsS http://localhost:3201/plans     # must return data from the fresh clone
```

Expected: both succeed — pgxpool re-dials the recreated DB. If the pool wedges,
add `--reset-settle=1s` handling or a pool `MaxConnLifetime` note here and
re-test before proceeding. **Do not author fixtures until this passes.**

- [ ] **Step 3: Commit**

```bash
chmod +x deploy/shadow/run-gate.sh
git add deploy/shadow/run-gate.sh
git commit -m "chore(shadow): dev gate runner + reset-while-connected smoke proof"
```

---

## PART 3 — Fixture authoring (~100 routes, per-module tasks)

**Authoring recipe (applies to every Part-3 task):**

1. One JSON file per route under `cmd/shadowdiff/fixtures/<module>/`. Filename =
   `<short_name>.json`; set `"name"` to a unique label.
2. Fields: `method`, `path`, `headers`, `body`, `ignore_value_of`.
3. **Auth:** add `"Authorization": "Bearer {{user_token}}"` (customer routes),
   `{{admin_token}}` (admin routes), or `{{ext_token}}` (keyboard rewrite). Add
   `"Content-Type": "application/json"` for JSON bodies.
4. **`ignore_value_of`:** always include `"request_id"`. Add any field whose
   value is generated at request time: `access_token`, `refresh_token`,
   `created_at`, `updated_at`, `id` (on create responses), `expires_at` when set
   to now-relative, `response_time_ms`, etc. Presence is still checked.
5. **Concrete ids/paths:** reference the augment rows' stable UUIDs
   (`…0001` user, etc.) so path-param routes resolve against seeded data.
6. **Verify each fixture** by running the gate filtered to the module dir (see
   each task's verify step) — Node and Go must both respond and diff clean.
   Mismatches are real parity bugs; record them, do NOT loosen `ignore_value_of`
   to hide a genuine value difference.

> Each Part-3 task is independent (separate fixture files) and may be dispatched
> in parallel. Stage ONLY that module's fixture dir.

### Task 11: Public auth fixtures

**Files:** Create `cmd/shadowdiff/fixtures/auth/*.json`

Routes: `POST /auth/login`, `/auth/refresh`, `/auth/register`,
`/auth/verify-email`, `/auth/resend-verification`, `/auth/forgot-password`,
`/auth/reset-password`, `/auth/social`.

- [ ] **Step 1:** Author one fixture per route. For each, author BOTH a success
  body (valid augment creds / valid payload) AND, where Node returns a specific
  error envelope, an error fixture (e.g. `auth_login_bad_creds` → 401
  `{"error":"Invalid credentials","code":"invalid-token",...}`). `/auth/refresh`
  needs a valid refresh token — capture one in bootstrap or use a login-then-refresh
  fixture pair. `ignore_value_of`: `request_id`, `access_token`, `refresh_token`.
- [ ] **Step 2: Verify** — `./deploy/shadow/run-gate.sh` then read the `auth_*`
  lines; all PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/auth && git commit -m "test(shadow): auth fixtures"`

### Task 12: Plans + public payment fixtures

**Files:** Create `cmd/shadowdiff/fixtures/payment_public/*.json`

Routes: `GET /plans`, `GET /payment/methods`, `GET /payment/status/{ref}`, and the
5 webhooks `POST /payment/webhook/{stripe,vietqr,casso,sepay,lemonsqueezy}`.

- [ ] **Step 1:** Author read fixtures (plans, methods, status using a seeded
  transaction ref). For each webhook, author a fixture with a **valid signed
  payload** for that provider (signature header computed from the dev webhook
  secret; reference each provider strategy in `internal/payment` for the exact
  header name + signing scheme). Also author one invalid-signature fixture per
  webhook asserting the rejection status/envelope Node returns.
- [ ] **Step 2: Verify** — gate; `payment_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/payment_public && git commit -m "test(shadow): plans + public payment + webhook fixtures"`

### Task 13: Misc public ingest fixtures

**Files:** Create `cmd/shadowdiff/fixtures/public_misc/*.json`

Routes: `GET /ime-packs/manifest`, `GET /updates/latest`, `POST /errors`,
`POST /webhooks/resend`, `POST /bug-reports`, `POST /feedback`, `GET /feedback`,
`POST /feedback/{id}/vote`, `GET /health`, `GET /metrics`.

- [ ] **Step 1:** Author one fixture per route. `POST /bug-reports` is multipart —
  if the harness only sends JSON bodies, note that bug-report ingest is multipart
  and author the simplest non-screenshot variant; if multipart isn't supported by
  `send`, record it as a gap to handle (extend `send` or mark verified-by-unit-test
  in the runbook). `/metrics` is Prometheus text not JSON — `ignore_value_of`
  won't apply; verify status + non-empty body only (note in fixture name).
- [ ] **Step 2: Verify** — gate; `public_misc_*` PASS (or documented exception).
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/public_misc && git commit -m "test(shadow): public ingest fixtures"`

### Task 14: Rewrite + extract fixtures

**Files:** Create `cmd/shadowdiff/fixtures/rewrite/*.json`

Routes: `POST /v1/rewrite` (both `{{ext_token}}` and `{{user_token}}` auth paths),
`POST /extract`.

- [ ] **Step 1:** Author a JWT-auth rewrite fixture and an ext-token rewrite
  fixture (same body, different auth) plus an extract fixture. **AI provider
  determinism:** rewrite/extract call an LLM — for a deterministic diff the
  augment must point the default provider at a stub/echo provider, OR
  `ignore_value_of` the model-generated text field and assert only the response
  envelope shape + status. Choose the latter (simpler): ignore the `result`/
  `rewritten` text value, check presence + surrounding keys. Document the choice
  in the fixture comment.
- [ ] **Step 2: Verify** — gate; `rewrite_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/rewrite && git commit -m "test(shadow): rewrite + extract fixtures"`

### Task 15: Authed account + subscription + ext-token fixtures

**Files:** Create `cmd/shadowdiff/fixtures/account/*.json`

Routes: `GET /auth/me`, `POST /auth/change-password`, `GET /auth/account`,
`DELETE /auth/account`, `GET /subscription`, `POST /subscription/verify-receipt`,
`POST /auth/extension-tokens`, `GET /auth/extension-tokens`,
`DELETE /auth/extension-tokens/{id}`, `GET /payment/history`,
`POST /payment/checkout`, `GET /payment/portal`, `DELETE /payment/subscription`.

- [ ] **Step 1:** Author with `{{user_token}}`. `DELETE /auth/account` and
  `DELETE /payment/subscription` mutate — fine, the per-fixture reset isolates
  them. `DELETE /auth/extension-tokens/{id}` needs a seeded ext-token row id
  (add one to augment). `verify-receipt`/`checkout`/`portal` may call external
  providers — assert the envelope Node returns for the dev config (likely an
  error/stub path); `ignore_value_of` provider-side urls/ids.
- [ ] **Step 2: Verify** — gate; `account_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/account && git commit -m "test(shadow): authed account/subscription/ext-token fixtures"`

### Task 16: Admin auth + ai-providers + settings fixtures

**Files:** Create `cmd/shadowdiff/fixtures/admin_core/*.json`

Routes: `POST /admin/auth/login` (already in bootstrap — add an explicit fixture
too), `POST /admin/auth/change-password`, `GET /admin/auth/me`,
`GET /admin/ai-providers`, `/admin/ai-providers/paginated`,
`POST /admin/ai-providers`, `PATCH /admin/ai-providers/{id}`,
`DELETE /admin/ai-providers/{id}`, `POST /admin/ai-providers/{id}/test`,
`GET /admin/settings`, `PATCH /admin/settings`, `POST /admin/settings/test-email`.

- [ ] **Step 1:** `{{admin_token}}`. Seed an ai-provider row in augment for the
  GET/PATCH/DELETE/test ids. `/test` and `test-email` hit external services —
  assert Node's dev-config envelope; `ignore_value_of` `response_time_ms`.
- [ ] **Step 2: Verify** — gate; `admin_core_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/admin_core && git commit -m "test(shadow): admin auth/ai-providers/settings fixtures"`

### Task 17: Admin plans + users + admin-users + email fixtures

**Files:** Create `cmd/shadowdiff/fixtures/admin_crud/*.json`

Routes: `GET/POST /admin/plans`, `PATCH/DELETE /admin/plans/{id}`,
`GET /admin/users`, `GET/PATCH /admin/users/{id}`,
`GET/POST /admin/admin-users`, `PATCH/DELETE /admin/admin-users/{id}`,
`GET /admin/email-logs`, `GET /admin/email-templates`,
`PATCH/DELETE /admin/email-templates/{key}`, `GET /admin/email-templates/{key}/preview`.

- [ ] **Step 1:** `{{admin_token}}`. Use seeded plan/user/admin-user/template-key
  ids. `GET /admin/users` dual-mode + default limit 20 — author both no-query and
  query variants. `PATCH /admin/users/{id}` with `{"role":"admin"}` → assert the
  400 validation envelope (the C-fix parity). Template `{key}` = a real builtin key.
- [ ] **Step 2: Verify** — gate; `admin_crud_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/admin_crud && git commit -m "test(shadow): admin plans/users/admin-users/email fixtures"`

### Task 18: Admin reporting fixtures

**Files:** Create `cmd/shadowdiff/fixtures/admin_reporting/*.json`

Routes: `GET /admin/stats`, `/admin/analytics`, `/admin/transactions`,
`/admin/training-data/stats`, `/admin/training-data/export`, `/admin/training-data`,
`PATCH /admin/training-data/{id}`, `GET /admin/payments/stats`, `GET /admin/payments`,
`POST /admin/payments/{id}/confirm`, `POST /admin/payments/{id}/refund`.

- [ ] **Step 1:** `{{admin_token}}`. Seed a payment_transaction + a rewrite_log
  (training-data) row. `refund` calls Stripe — assert Node's dev envelope;
  `ignore_value_of` provider refund ids. Reporting responses carry computed
  numbers from seeded rows — assert exact values (frozen template = deterministic).
- [ ] **Step 2: Verify** — gate; `admin_reporting_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/admin_reporting && git commit -m "test(shadow): admin reporting fixtures"`

### Task 19: Admin triage fixtures (errors, bug-reports, inbox, releases, grant)

**Files:** Create `cmd/shadowdiff/fixtures/admin_triage/*.json`

Routes: `POST /admin/errors/run-ai-cron`, `GET /admin/errors`,
`GET/PATCH/DELETE /admin/errors/{id}`, `POST /admin/errors/{id}/suggest-fix`,
`GET /admin/bug-reports`, `GET /admin/bug-reports/{id}`,
`GET /admin/bug-reports/{id}/screenshot`, `PATCH/DELETE /admin/bug-reports/{id}`,
`POST /admin/bug-reports/{id}/fix-proposal`, `GET /admin/inbox/counts`,
`GET /admin/inbox`, `GET /admin/releases`, `POST /admin/releases`,
`DELETE /admin/releases/{platform}/{channel}`, `POST /admin/release-policies`,
`POST /admin/subscriptions/grant`.

- [ ] **Step 1:** `{{admin_token}}`. Seed error_report + bug_report + a
  bug-report-with-screenshot + app_release rows. `suggest-fix`/`fix-proposal`/
  `run-ai-cron` call the LLM — assert envelope, `ignore_value_of` the proposal
  text. `screenshot` returns `image/png` not JSON — verify status + content-type
  + non-empty body (note in fixture). `grant` mutates — reset isolates it; assert
  the GrantedSub key order (the C-fix parity).
- [ ] **Step 2: Verify** — gate; `admin_triage_*` PASS.
- [ ] **Step 3: Commit** — `git add cmd/shadowdiff/fixtures/admin_triage && git commit -m "test(shadow): admin triage fixtures"`

---

## PART 4 — Run the full gate to green

### Task 20: Full gate + coverage green

**Files:** none (may touch fixtures / `ignore_value_of` to fix authoring bugs; any Go parity fix is a separate branch — Node is authority)

- [ ] **Step 1: Run the full gate on the VPS**

Run: `./deploy/shadow/run-gate.sh --env-file /opt/draftright/.env.dev`
Expected: `N/N fixtures passed`, coverage assertion silent (no gap), exit 0.

- [ ] **Step 2: Triage diffs**

For each FAIL: if it's a fixture-authoring error (wrong path, missing
`ignore_value_of` for a genuinely request-time value), fix the fixture. If it's a
**real Go≠Node divergence**, STOP — file a parity issue and fix it on a separate
fix branch off `develop` (do not paper over it by loosening the diff). Re-run.

- [ ] **Step 3: Record the green run**

Append the passing summary (counts + date) to `deploy/phase5-cutover-runbook.md`
§"Gate evidence" (created next task). Commit any fixture fixes:

```bash
git add cmd/shadowdiff/fixtures
git commit -m "test(shadow): fixture corrections — full gate green (N/N)"
```

---

## PART 5 — Prod cutover runbook (manual, gated)

### Task 21: Write the cutover runbook

**Files:** Create `deploy/phase5-cutover-runbook.md`

- [ ] **Step 1: Write the runbook**

Document the operator steps from spec §8 verbatim as a numbered checklist with
exact commands, plus a "Gate evidence" section and a "Rollback" section:

```markdown
# Phase 5 — Production Cutover Runbook (manual, gated)

PRECONDITIONS (all must hold):
- [ ] `./deploy/shadow/run-gate.sh` reports N/N green on dev (paste summary below).
- [ ] Coverage assertion passes (no route lacks a fixture).
- [ ] Human go-ahead from Tan.

## Gate evidence
<paste latest green run summary + date>

## 1. Build + ship Go prod image
cd /opt/draftright && git fetch origin && git checkout main && git pull --ff-only
# add backend-go service to prod compose pointing at PROD db + secrets via --env-file
docker compose build backend-go
docker compose --env-file /opt/draftright/.env up -d backend-go    # alongside Node
curl -fsS http://localhost:<go-port>/health     # smoke

## 2. Flip Caddy (instant, reversible)
# edit api.draftright.info block: reverse_proxy localhost:<node-port> -> <go-port>
systemctl reload caddy
# verify
curl -fsS https://api.draftright.info/health

## 3. Payment webhooks
# confirm provider dashboards target /payment/webhook/* (path-only -> auto re-pointed)

## 4. Soak (Node stays warm)
docker logs -f dr-backend-go   # watch errors; check /health; container status

## Rollback (seconds)
# revert the Caddy block to <node-port>; systemctl reload caddy

## 5. Teardown (after 7 clean days)
docker compose stop backend           # Node
# confirm Go steady, then remove backend service from prod compose + prune image
docker image prune -a
```

- [ ] **Step 2: Post-flip housekeeping note**

Add a final section referencing `~/.claude/CLAUDE.md`: apply GitHub
issue-lifecycle label (`status: deployed to production`) + a `## ✅ How to Verify`
comment on the Phase 5 tracking issue.

- [ ] **Step 3: Commit**

```bash
git add deploy/phase5-cutover-runbook.md
git commit -m "docs(phase5): production cutover runbook (gated, manual)"
```

> **Tasks 1–21 leave production untouched.** The runbook is executed by a human
> on explicit go-ahead — not by the implementer, not autonomously.

---

## Full-gate verification (after all tasks)

- `go test ./cmd/shadowdiff/ -v` — all harness unit tests PASS.
- `go build ./...` — clean.
- `./deploy/shadow/run-gate.sh` on the VPS — N/N fixtures, exit 0, coverage clean.
- Manual parity spot-check of 2 representative routes against Node remains as a
  runbook precondition before the flip.
