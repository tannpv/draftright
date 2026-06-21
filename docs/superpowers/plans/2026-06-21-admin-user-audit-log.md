# Admin-User Deactivation Audit Log Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record an append-only audit row (who deactivated whom, when) every time an admin deactivates another admin via `DELETE /admin/admin-users/:id`, written atomically with the soft-delete, and surface it as a read-only admin-portal page.

**Architecture:** Go-only net-new feature on the retiring Node backend (Node serves zero prod traffic). New `admin_user_audit_log` table; the audit INSERT shares the soft-delete transaction in `internal/adminauth`. A new `GET /admin/admin-user-audit` read endpoint returns `{rows,total}` and is deliberately **omitted from the shadow-gate route inventory** (Node would 404 — not a parity break). A dedicated React admin-portal page renders the log.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc v1.31, golang-jwt v5; admin portal = React 18 + Vite + TS.

**Spec:** `docs/superpowers/specs/2026-06-21-admin-user-audit-log-design.md`

**Parity contract (non-negotiable):** the `DELETE` response stays byte-identical (`200 {"success":true}`). The audit write is a pure side effect inside the existing transaction-free guard flow → now a transaction. Existing admin-users fixtures must stay green.

---

## File Structure

| File | Responsibility | New/Modify |
|---|---|---|
| `backend/sql/2026-06-21-admin-user-audit-log.sql` | Canonical prod migration (CREATE TABLE + index) | Create |
| `backend-rewrite-go/internal/platform/db/schema.sql` | Schema mirror sqlc validates against — append the table | Modify |
| `backend-rewrite-go/internal/shared/pg/queries_adminusers.sql` | Append 4 queries (email lookup, insert audit, list, count) | Modify |
| `backend-rewrite-go/internal/shared/pg/sqlc/*` | sqlc-generated Go (regenerated) | Modify (generated) |
| `backend-rewrite-go/internal/adminauth/audit.go` | `AdminUserAuditOut` domain + `MarshalJSON` | Create |
| `backend-rewrite-go/internal/adminauth/audit.go` test → `audit_test.go` | MarshalJSON field-order/timestamp test | Create |
| `backend-rewrite-go/internal/adminauth/admin_users_repo_pg.go` | Add `SoftDeleteWithAudit` tx method | Modify |
| `backend-rewrite-go/internal/adminauth/admin_users_usecase.go` | Replace `SoftDelete` → `SoftDeleteWithAudit` | Modify |
| `backend-rewrite-go/internal/adminauth/admin_users_handler.go` | `Delete` calls `SoftDeleteWithAudit(actor, target)` | Modify |
| `backend-rewrite-go/internal/adminauth/admin_users_handler_test.go` | Update fake + assert audit-call wiring | Modify |
| `backend-rewrite-go/internal/adminauth/audit_read.go` | `AdminAuditService` + `AdminAuditRepo` + `AdminAuditHandler` (read path) | Create |
| `backend-rewrite-go/internal/adminauth/audit_read_test.go` | Read handler/service tests with fake | Create |
| `backend-rewrite-go/cmd/server/main.go` | Wire read handler + `coreHandlers` field | Modify |
| `backend-rewrite-go/internal/shared/router.go` | New `AdminAuditList` field + mount | Modify |
| `backend-rewrite-go/deploy/shadow/routes.txt` | Comment documenting the intentional omission | Modify |
| `admin/src/pages/AdminAuditLogPage.tsx` | Read-only log page | Create |
| `admin/src/App.tsx` | Route `/admin-audit` | Modify |
| `admin/src/components/Layout.tsx` | Nav item "Admin Audit Log" | Modify |

**Branch:** already on `feature/admin-audit-log-51-20260621` (off develop). Do NOT create a new branch.

---

### Task 1: Migration SQL + schema mirror

The table must exist in `schema.sql` before sqlc can compile any query that references it (sqlc type-checks against that file). The prod migration is the deployable artifact.

**Files:**
- Create: `backend/sql/2026-06-21-admin-user-audit-log.sql`
- Modify: `backend-rewrite-go/internal/platform/db/schema.sql` (append after the `admin_users` table block, ~line 127)

- [ ] **Step 1: Write the prod migration**

Create `backend/sql/2026-06-21-admin-user-audit-log.sql`:

```sql
-- #51 admin-user deactivation audit log. Append-only. No FKs (snapshot must
-- survive a later hard-delete/rename of either party). No `action` column
-- (scope = deactivation only). Run BEFORE deploying the Go image that queries
-- it. Idempotent: safe to re-run.
CREATE TABLE IF NOT EXISTS public.admin_user_audit_log (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    actor_admin_id uuid NOT NULL,
    actor_email text NOT NULL,
    target_admin_id uuid NOT NULL,
    target_email text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT admin_user_audit_log_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_admin_user_audit_log_created_at
    ON public.admin_user_audit_log (created_at DESC);
```

- [ ] **Step 2: Mirror the table into the Go schema dump**

In `backend-rewrite-go/internal/platform/db/schema.sql`, immediately after the `CREATE TABLE public.admin_users (...)` block (ends ~line 122, blank line at 123), insert:

```sql
--
-- Name: admin_user_audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_user_audit_log (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    actor_admin_id uuid NOT NULL,
    actor_email text NOT NULL,
    target_admin_id uuid NOT NULL,
    target_email text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

```

- [ ] **Step 3: Verify schema still parses (build is unaffected, no queries yet)**

Run: `cd backend-rewrite-go && go build ./...`
Expected: success (no code references the table yet).

- [ ] **Step 4: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend/sql/2026-06-21-admin-user-audit-log.sql backend-rewrite-go/internal/platform/db/schema.sql
git commit -m "feat(audit-log): add admin_user_audit_log table (migration + schema mirror) (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: sqlc queries + generate

Add the four queries the write tx and read path need, then regenerate. `GetAdminUserEmailByID` + `InsertAdminUserAudit` feed the transaction; `ListAdminUserAudit` + `CountAdminUserAudit` feed the read endpoint. `SoftDeleteAdminUser` already exists and is reused inside the tx via `WithTx`.

**Files:**
- Modify: `backend-rewrite-go/internal/shared/pg/queries_adminusers.sql`
- Modify (generated): `backend-rewrite-go/internal/shared/pg/sqlc/*`

- [ ] **Step 1: Append the queries**

At the end of `backend-rewrite-go/internal/shared/pg/queries_adminusers.sql`, add:

```sql
-- #51 audit log. GetAdminUserEmailByID snapshots actor/target email inside the
-- soft-delete tx. InsertAdminUserAudit writes the append-only row in that same
-- tx. List/Count back GET /admin/admin-user-audit (Go-only, newest-first).

-- name: GetAdminUserEmailByID :one
SELECT email FROM admin_users WHERE id = $1;

-- name: InsertAdminUserAudit :exec
INSERT INTO admin_user_audit_log (actor_admin_id, actor_email, target_admin_id, target_email)
VALUES ($1, $2, $3, $4);

-- name: ListAdminUserAudit :many
SELECT id, actor_admin_id, actor_email, target_admin_id, target_email, created_at
FROM admin_user_audit_log
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAdminUserAudit :one
SELECT COUNT(*) FROM admin_user_audit_log;
```

- [ ] **Step 2: Regenerate**

Run: `cd backend-rewrite-go && sqlc generate`
Expected: no error; `git status` shows modified files under `internal/shared/pg/sqlc/`.

- [ ] **Step 3: Verify the generated functions exist and compile**

Run: `cd backend-rewrite-go && go build ./... && grep -l "InsertAdminUserAudit\|ListAdminUserAudit\|GetAdminUserEmailByID\|CountAdminUserAudit" internal/shared/pg/sqlc/*.go`
Expected: build succeeds; grep prints at least one filename.

- [ ] **Step 4: Verify sqlc-check is clean (no drift)**

Run: `cd backend-rewrite-go && ./scripts/sqlc-check.sh`
Expected: exits 0 (generated output matches committed).

- [ ] **Step 5: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend-rewrite-go/internal/shared/pg/queries_adminusers.sql backend-rewrite-go/internal/shared/pg/sqlc
git commit -m "feat(audit-log): sqlc queries for audit insert + email lookup + list/count (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Audit domain type + JSON marshalling

`AdminUserAuditOut` is the serialized row. Field order and the ISO-millis timestamp must be pinned via `MarshalJSON` exactly like `AdminUserOut` (other admin responses) so the JSON is stable.

**Files:**
- Create: `backend-rewrite-go/internal/adminauth/audit.go`
- Create: `backend-rewrite-go/internal/adminauth/audit_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend-rewrite-go/internal/adminauth/audit_test.go`:

```go
package adminauth

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAdminUserAuditOut_MarshalJSON_FieldOrderAndTimestamp(t *testing.T) {
	row := AdminUserAuditOut{
		ID:            "11111111-1111-1111-1111-111111111111",
		ActorAdminID:  "22222222-2222-2222-2222-222222222222",
		ActorEmail:    "admin@draftright.info",
		TargetAdminID: "33333333-3333-3333-3333-333333333333",
		TargetEmail:   "ops@draftright.info",
		CreatedAt:     time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"id":"11111111-1111-1111-1111-111111111111","actor_admin_id":"22222222-2222-2222-2222-222222222222","actor_email":"admin@draftright.info","target_admin_id":"33333333-3333-3333-3333-333333333333","target_email":"ops@draftright.info","created_at":"2026-06-21T12:00:00.000Z"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestAdminUserAuditOut -v`
Expected: FAIL — `undefined: AdminUserAuditOut`.

- [ ] **Step 3: Write the domain type**

Create `backend-rewrite-go/internal/adminauth/audit.go`:

```go
package adminauth

import (
	"encoding/json"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// AdminUserAuditOut is one admin_user_audit_log row as GET /admin/admin-user-audit
// serializes it: snake_case, fixed key order, created_at as ISO-8601 millis (UTC)
// — matching the timestamp format every other admin response uses (shared.ISOMillis).
type AdminUserAuditOut struct {
	ID            string
	ActorAdminID  string
	ActorEmail    string
	TargetAdminID string
	TargetEmail   string
	CreatedAt     time.Time
}

func (a AdminUserAuditOut) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string `json:"id"`
		ActorAdminID  string `json:"actor_admin_id"`
		ActorEmail    string `json:"actor_email"`
		TargetAdminID string `json:"target_admin_id"`
		TargetEmail   string `json:"target_email"`
		CreatedAt     string `json:"created_at"`
	}{
		ID:            a.ID,
		ActorAdminID:  a.ActorAdminID,
		ActorEmail:    a.ActorEmail,
		TargetAdminID: a.TargetAdminID,
		TargetEmail:   a.TargetEmail,
		CreatedAt:     shared.ISOMillis(a.CreatedAt),
	})
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestAdminUserAuditOut -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend-rewrite-go/internal/adminauth/audit.go backend-rewrite-go/internal/adminauth/audit_test.go
git commit -m "feat(audit-log): AdminUserAuditOut domain type with pinned JSON (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Write path — atomic soft-delete + audit insert

Replace the non-transactional `SoftDelete` with `SoftDeleteWithAudit(actorID, targetID)`. The repo opens a pgx transaction: snapshot both emails, soft-delete the target, insert the audit row, commit. A target that doesn't exist is a no-op success with **no** audit row (Node's `deleteAdminUser` returns `{success:true}` unconditionally — preserving byte-parity). The handler's two #32 guards are unchanged and still run *before* this call, so a rejected deactivation writes nothing.

**Files:**
- Modify: `backend-rewrite-go/internal/adminauth/admin_users_repo_pg.go`
- Modify: `backend-rewrite-go/internal/adminauth/admin_users_usecase.go`
- Modify: `backend-rewrite-go/internal/adminauth/admin_users_handler.go`
- Modify: `backend-rewrite-go/internal/adminauth/admin_users_handler_test.go`

- [ ] **Step 1: Update the handler test fake + expectations (RED)**

In `admin_users_handler_test.go`, find the fake service type that satisfies `adminUsersService`. Rename its `SoftDelete(ctx, id)` method/field to `SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error`, recording `actorID`+`targetID` and a call count. Then update the Delete tests:

- Add/confirm a success case: valid non-self target that is NOT the last admin → assert response is `200` with body `{"success":true}` AND `fake.softDeleteCalls == 1` AND the recorded `actorID == <claims.Sub>` and `targetID == <url id>`.
- Confirm the self-delete case (`id == claims.Sub`) → `400` AND `fake.softDeleteCalls == 0`.
- Confirm the last-active-admin case (active target, count ≤ 1) → `400` AND `fake.softDeleteCalls == 0`.

Example fake shape (adapt to the file's existing fake):

```go
type fakeAdminUsersSvc struct {
	// ... existing fields ...
	softDeleteCalls int
	gotActorID      string
	gotTargetID     string
	isActive        bool
	activeCount     int
}

func (f *fakeAdminUsersSvc) SoftDeleteWithAudit(_ context.Context, actorID, targetID string) error {
	f.softDeleteCalls++
	f.gotActorID = actorID
	f.gotTargetID = targetID
	return nil
}
func (f *fakeAdminUsersSvc) IsActiveAdmin(context.Context, string) (bool, error) { return f.isActive, nil }
func (f *fakeAdminUsersSvc) CountActiveAdmins(context.Context) (int, error)       { return f.activeCount, nil }
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestAdminUsersHandler -v` (or the Delete test name in this file)
Expected: FAIL/compile error — handler still calls `SoftDelete`, interface mismatch.

- [ ] **Step 3: Update the handler interface + Delete call**

In `admin_users_handler.go`, in the `adminUsersService` interface, replace:

```go
	SoftDelete(ctx context.Context, id string) error
```

with:

```go
	SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error
```

In `Delete`, replace the soft-delete call:

```go
	if err := h.svc.SoftDelete(r.Context(), id); err != nil {
```

with:

```go
	if err := h.svc.SoftDeleteWithAudit(r.Context(), claims.Sub, id); err != nil {
```

(`claims` is already in scope — captured at the top of `Delete` for the self-check.)

- [ ] **Step 4: Update the usecase**

In `admin_users_usecase.go`, in the `adminUsersRepo` interface replace:

```go
	SoftDelete(ctx context.Context, id string) error
```

with:

```go
	SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error
```

Replace the `SoftDelete` method:

```go
// SoftDelete clears is_active (Node deleteAdminUser).
func (s *AdminUsersService) SoftDelete(ctx context.Context, id string) error {
	return s.repo.SoftDelete(ctx, id)
}
```

with:

```go
// SoftDeleteWithAudit clears is_active AND records an append-only audit row in
// the same transaction (#51). actorID is the deactivating admin (JWT sub);
// targetID is the deactivated admin. The #32 guards run upstream in the handler,
// so a rejected deactivation never reaches here and never writes a row.
func (s *AdminUsersService) SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error {
	return s.repo.SoftDeleteWithAudit(ctx, actorID, targetID)
}
```

- [ ] **Step 5: Implement the repo transaction**

In `admin_users_repo_pg.go`, replace the `SoftDelete` method:

```go
// SoftDelete clears is_active. ALWAYS returns nil on success even if 0 rows
// matched (Node returns { success: true } unconditionally, no existence check).
func (r *AdminUsersRepo) SoftDelete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.q.SoftDeleteAdminUser(ctx, uid)
}
```

with:

```go
// SoftDeleteWithAudit clears is_active on the target AND appends an audit row in
// ONE transaction (#51) — a deactivation and its trail are all-or-nothing. Both
// emails are snapshotted by id inside the tx (no dependency on JWT claim shape).
//
// A missing target is a no-op success with NO audit row: Node's deleteAdminUser
// returns { success: true } unconditionally even when the row is absent, so the
// DELETE response stays byte-identical and we don't fabricate a trail for a
// deactivation that didn't happen.
func (r *AdminUsersRepo) SoftDeleteWithAudit(ctx context.Context, actorID, targetID string) error {
	actorUUID, err := parseUUID(actorID)
	if err != nil {
		return err
	}
	targetUUID, err := parseUUID(targetID)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := r.q.WithTx(tx)

	targetEmail, err := qtx.GetAdminUserEmailByID(ctx, targetUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // missing target → Node success, no audit row
	}
	if err != nil {
		return err
	}
	actorEmail, err := qtx.GetAdminUserEmailByID(ctx, actorUUID)
	if err != nil {
		return err // actor is the authenticated caller; row must exist
	}

	if err := qtx.SoftDeleteAdminUser(ctx, targetUUID); err != nil {
		return err
	}
	if err := qtx.InsertAdminUserAudit(ctx, sqlc.InsertAdminUserAuditParams{
		ActorAdminID:  actorUUID,
		ActorEmail:    actorEmail,
		TargetAdminID: targetUUID,
		TargetEmail:   targetEmail,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```

If `admin_users_repo_pg_test.go` references `SoftDelete`, update those references to `SoftDeleteWithAudit` (passing an actor id) or delete the obsolete case if it was a pure DB test that no longer applies — keep the file compiling.

- [ ] **Step 6: Run to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -race -v`
Expected: PASS (handler Delete tests green; package compiles).

- [ ] **Step 7: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend-rewrite-go/internal/adminauth/admin_users_repo_pg.go backend-rewrite-go/internal/adminauth/admin_users_usecase.go backend-rewrite-go/internal/adminauth/admin_users_handler.go backend-rewrite-go/internal/adminauth/admin_users_handler_test.go backend-rewrite-go/internal/adminauth/admin_users_repo_pg_test.go
git commit -m "feat(audit-log): write audit row atomically with admin soft-delete (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Read path — service, repo, handler

`GET /admin/admin-user-audit` returns `{rows,total}` newest-first with `?limit=` (default 50, max 100) + `?offset=` (default 0). A small self-contained trio in the `adminauth` package: handler → service → repo, each with its own consumer-side port.

**Files:**
- Create: `backend-rewrite-go/internal/adminauth/audit_read.go`
- Create: `backend-rewrite-go/internal/adminauth/audit_read_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend-rewrite-go/internal/adminauth/audit_read_test.go`:

```go
package adminauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeAuditRepo struct {
	rows       []AdminUserAuditOut
	total      int
	gotLimit   int
	gotOffset  int
}

func (f *fakeAuditRepo) ListAudit(_ context.Context, limit, offset int) ([]AdminUserAuditOut, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.rows, nil
}
func (f *fakeAuditRepo) CountAudit(context.Context) (int, error) { return f.total, nil }

func TestAdminAuditHandler_List_ReturnsRowsTotal(t *testing.T) {
	repo := &fakeAuditRepo{
		rows: []AdminUserAuditOut{{
			ID: "a", ActorAdminID: "b", ActorEmail: "x@y.z",
			TargetAdminID: "c", TargetEmail: "p@q.r",
			CreatedAt: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}
	h := NewAdminAuditHandler(NewAdminAuditService(repo))

	req := httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit?limit=10&offset=5", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	var body struct {
		Rows  []map[string]any `json:"rows"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	if body.Total != 1 || len(body.Rows) != 1 {
		t.Fatalf("got total=%d rows=%d", body.Total, len(body.Rows))
	}
	if repo.gotLimit != 10 || repo.gotOffset != 5 {
		t.Errorf("limit/offset: got %d/%d want 10/5", repo.gotLimit, repo.gotOffset)
	}
}

func TestAdminAuditHandler_List_DefaultsAndCap(t *testing.T) {
	repo := &fakeAuditRepo{}
	h := NewAdminAuditHandler(NewAdminAuditService(repo))

	// no params → default limit 50, offset 0
	h.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit", nil))
	if repo.gotLimit != 50 || repo.gotOffset != 0 {
		t.Errorf("defaults: got %d/%d want 50/0", repo.gotLimit, repo.gotOffset)
	}
	// over-cap → clamped to 100
	h.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/admin-user-audit?limit=9999", nil))
	if repo.gotLimit != 100 {
		t.Errorf("cap: got %d want 100", repo.gotLimit)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestAdminAuditHandler -v`
Expected: FAIL — `undefined: NewAdminAuditHandler` / `NewAdminAuditService`.

- [ ] **Step 3: Implement the read path**

Create `backend-rewrite-go/internal/adminauth/audit_read.go`:

```go
package adminauth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tannpv/draftright-rewrite/internal/shared"
	"github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

const (
	auditDefaultLimit = 50
	auditMaxLimit     = 100
)

// --- repo (adapter) ---------------------------------------------------------

// AdminAuditRepo reads admin_user_audit_log rows. Both queries are static →
// sqlc. The write side lives in AdminUsersRepo (it owns the soft-delete tx).
type AdminAuditRepo struct {
	q *sqlc.Queries
}

// NewAdminAuditRepo wires the sqlc querier. The pool is unused here (static
// queries only) but accepted for wiring symmetry with the other admin repos.
func NewAdminAuditRepo(q *sqlc.Queries, _ *pgxpool.Pool) *AdminAuditRepo {
	return &AdminAuditRepo{q: q}
}

// ListAudit returns rows newest-first, paginated. Non-nil empty slice → JSON [].
func (r *AdminAuditRepo) ListAudit(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, error) {
	rows, err := r.q.ListAdminUserAudit(ctx, sqlc.ListAdminUserAuditParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AdminUserAuditOut, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminUserAuditOut{
			ID:            uuidStr(row.ID),
			ActorAdminID:  uuidStr(row.ActorAdminID),
			ActorEmail:    row.ActorEmail,
			TargetAdminID: uuidStr(row.TargetAdminID),
			TargetEmail:   row.TargetEmail,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return out, nil
}

// CountAudit returns the total audit-row count (for pagination).
func (r *AdminAuditRepo) CountAudit(ctx context.Context) (int, error) {
	n, err := r.q.CountAdminUserAudit(ctx)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// --- service (usecase) ------------------------------------------------------

// adminAuditRepo is the service's consumer-side port; *AdminAuditRepo satisfies it.
type adminAuditRepo interface {
	ListAudit(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, error)
	CountAudit(ctx context.Context) (int, error)
}

// AdminAuditService backs the read endpoint.
type AdminAuditService struct {
	repo adminAuditRepo
}

// NewAdminAuditService wires the repo.
func NewAdminAuditService(repo adminAuditRepo) *AdminAuditService {
	return &AdminAuditService{repo: repo}
}

// List returns one page of audit rows plus the total count.
func (s *AdminAuditService) List(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, int, error) {
	rows, err := s.repo.ListAudit(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountAudit(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// --- handler (transport) ----------------------------------------------------

// adminAuditLister is the handler's consumer-side port; *AdminAuditService satisfies it.
type adminAuditLister interface {
	List(ctx context.Context, limit, offset int) ([]AdminUserAuditOut, int, error)
}

// AdminAuditHandler serves GET /admin/admin-user-audit (admin group). Go-only:
// Node has no equivalent route, so this endpoint is intentionally absent from
// the shadow-gate route inventory (deploy/shadow/routes.txt).
type AdminAuditHandler struct {
	svc adminAuditLister
}

// NewAdminAuditHandler wires the service.
func NewAdminAuditHandler(svc *AdminAuditService) *AdminAuditHandler {
	return &AdminAuditHandler{svc: svc}
}

// adminAuditPaginatedResponse is the { rows, total } body (field order matches
// the admin-users list endpoint).
type adminAuditPaginatedResponse struct {
	Rows  []AdminUserAuditOut `json:"rows"`
	Total int                 `json:"total"`
}

// List parses limit/offset (limit default 50, max 100; offset default 0) and
// returns the page newest-first.
func (h *AdminAuditHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := auditDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > auditMaxLimit {
		limit = auditMaxLimit
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}

	rows, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-user-audit failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, adminAuditPaginatedResponse{Rows: rows, Total: total})
}
```

> Note: `uuidStr` already exists in the `adminauth` package (used by `admin_users_repo_pg.go`). Reuse it — do not redefine.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -race -v`
Expected: PASS (read tests green).

- [ ] **Step 5: Verify the whole module + sqlc param names compile**

Run: `cd backend-rewrite-go && go build ./... && go vet ./internal/adminauth/`
Expected: clean. (If `ListAdminUserAuditParams.Limit/Offset` or `row.ActorAdminID` field names differ from sqlc's generation, fix the references to match the generated struct — inspect `internal/shared/pg/sqlc/` for the exact names.)

- [ ] **Step 6: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend-rewrite-go/internal/adminauth/audit_read.go backend-rewrite-go/internal/adminauth/audit_read_test.go
git commit -m "feat(audit-log): GET /admin/admin-user-audit read path (service/repo/handler) (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Wire the read endpoint into main.go + router + document the gate omission

**Files:**
- Modify: `backend-rewrite-go/internal/shared/router.go`
- Modify: `backend-rewrite-go/cmd/server/main.go`
- Modify: `backend-rewrite-go/deploy/shadow/routes.txt`

- [ ] **Step 1: Add the router field + mount**

In `internal/shared/router.go`, after the `AdminAccountDelete` field (line ~163), add:

```go
	AdminAuditList     http.Handler // GET    /admin/admin-user-audit (admin, Go-only)
```

After the `AdminAccountDelete` mount block (line ~504), add:

```go
		if r.AdminAuditList != nil {
			admin.Method(http.MethodGet, "/admin/admin-user-audit", r.AdminAuditList)
		}
```

- [ ] **Step 2: Add the coreHandlers field**

In `cmd/server/main.go`, in the `coreHandlers` struct, after `adminAccountDelete http.Handler` (line ~940), add:

```go
	adminAuditList     http.Handler // GET    /admin/admin-user-audit (admin, Go-only)
```

- [ ] **Step 3: Build + assign the handler**

In `cmd/server/main.go`, right after the admin-accounts wiring block (after line ~599, `core.adminAccountDelete = ...`), add:

```go
		// Admin-user audit log (#51, Go-only read endpoint).
		adminAuditSvc := adminauth.NewAdminAuditService(adminauth.NewAdminAuditRepo(q, pool))
		core.adminAuditList = http.HandlerFunc(adminauth.NewAdminAuditHandler(adminAuditSvc).List)
```

In the `coreHandlers{...}` → router struct literal (where `AdminAccountDelete: core.adminAccountDelete,` is, line ~231), add:

```go
		AdminAuditList:     core.adminAuditList,
```

- [ ] **Step 4: Document the intentional gate omission**

In `deploy/shadow/routes.txt`, add this comment block at the end of the file (it is a comment — `loadRoutes` ignores `#` lines, so it adds NO probe and NO coverage requirement):

```
# ── Go-only routes (NO Node parity) — intentionally NOT probed ───────────────
# GET /admin/admin-user-audit  — #51 audit log. Node returns 404 (route never
#   existed); listing it here would make the shadow gate diff 200 vs 404 and
#   fail. Net-new post-cutover Go-only feature; keep it out of the inventory.
```

- [ ] **Step 5: Build + full gate**

Run:
```bash
cd backend-rewrite-go && go build ./... && go vet ./... && gofmt -l . && go test ./... -race 2>&1 | tail -20
```
Expected: build clean, `gofmt -l .` prints nothing, all tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /opt/openAi/DraftRight
git add backend-rewrite-go/internal/shared/router.go backend-rewrite-go/cmd/server/main.go backend-rewrite-go/deploy/shadow/routes.txt
git commit -m "feat(audit-log): mount GET /admin/admin-user-audit + note gate omission (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Admin portal — Admin Audit Log page

Read-only paginated table (When · Actor · Target). Reuses the existing `DataTable` + `apiFetch` + page chrome from `AdminUsersPage.tsx`. Admin portal has no unit-test harness (Playwright e2e only), so verification is `tsc` + manual; no TDD unit test.

**Files:**
- Create: `admin/src/pages/AdminAuditLogPage.tsx`
- Modify: `admin/src/App.tsx`
- Modify: `admin/src/components/Layout.tsx`

- [ ] **Step 1: Create the page**

Create `admin/src/pages/AdminAuditLogPage.tsx`:

```tsx
import { useState, useEffect, useCallback } from 'react';
import DataTable from '../components/DataTable';
import { apiFetch } from '../api';

interface AuditRow {
  id: string;
  actor_admin_id: string;
  actor_email: string;
  target_admin_id: string;
  target_email: string;
  created_at: string;
  [key: string]: unknown;
}

export default function AdminAuditLogPage() {
  const [rows, setRows] = useState<AuditRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [total, setTotal] = useState(0);

  const fetchRows = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        limit: String(pageSize),
        offset: String((page - 1) * pageSize),
      });
      const data = await apiFetch(`/admin/admin-user-audit?${params}`) as { rows: AuditRow[]; total: number };
      setRows(data.rows ?? []);
      setTotal(data.total ?? 0);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit log');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize]);

  useEffect(() => { fetchRows(); }, [fetchRows]);

  const columns = [
    {
      header: 'When',
      key: 'created_at',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--muted)' }}>{new Date(row.created_at).toLocaleString()}</span>
      ),
    },
    {
      header: 'Actor',
      key: 'actor_email',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--text)', fontWeight: 600 }}>{row.actor_email}</span>
      ),
    },
    {
      header: 'Deactivated',
      key: 'target_email',
      render: (row: AuditRow) => (
        <span style={{ color: 'var(--text)' }}>{row.target_email}</span>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: 'var(--text)', margin: 0 }}>Admin Audit Log</h1>
        <p style={{ color: 'var(--muted)', marginTop: 4 }}>Admin-account deactivations, newest first.</p>
      </div>

      {error && <div style={{ color: 'var(--danger, #f44)', marginBottom: 16 }}>{error}</div>}

      <DataTable
        columns={columns}
        rows={rows}
        loading={loading}
        page={page}
        totalPages={Math.max(1, Math.ceil(total / pageSize))}
        onPageChange={setPage}
        total={total}
        pageSize={pageSize}
        onPageSizeChange={(s) => { setPageSize(s); setPage(1); }}
        emptyMessage="No admin deactivations recorded."
      />
    </div>
  );
}
```

- [ ] **Step 2: Register the route**

In `admin/src/App.tsx`, add the import after the `AdminUsersPage` import (line ~17):

```tsx
import AdminAuditLogPage from './pages/AdminAuditLogPage';
```

Add the route after the `/admin-users` route (line ~49):

```tsx
            <Route path="/admin-audit" element={<AdminAuditLogPage />} />
```

- [ ] **Step 3: Add the nav item**

In `admin/src/components/Layout.tsx`, add a nav entry immediately after the `/admin-users` item (line ~155). Reuse the existing `IconAdminUsers` icon (no new icon needed):

```tsx
  { path: '/admin-audit',   label: 'Admin Audit Log', icon: <IconAdminUsers />,    exact: false },
```

- [ ] **Step 4: Type-check + build**

Run:
```bash
cd admin && npx tsc --noEmit && npm run build 2>&1 | tail -5
```
Expected: no TS errors; Vite build succeeds.

- [ ] **Step 5: Commit**

```bash
cd /opt/openAi/DraftRight
git add admin/src/pages/AdminAuditLogPage.tsx admin/src/App.tsx admin/src/components/Layout.tsx
git commit -m "feat(audit-log): admin portal Admin Audit Log page (#51)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Final Verification (after all tasks)

- [ ] Full Go gate:
```bash
cd backend-rewrite-go && go build ./... && go vet ./... && gofmt -l . && go test ./... -race 2>&1 | tail -20 && ./scripts/sqlc-check.sh
```
Expected: build clean, vet clean, `gofmt -l .` empty, all tests PASS, sqlc-check exits 0.

- [ ] Admin portal: `cd admin && npx tsc --noEmit && npm run build` → clean.

- [ ] Live shadow gate (on dev VPS, before any prod deploy): apply `backend/sql/2026-06-21-admin-user-audit-log.sql` to `draftright_dev` FIRST (so the template clone has the table), then `./deploy/shadow/run-gate.sh`. Expected: all fixtures green, no coverage gap (the Go-only route is not listed, so it is neither probed nor required).

- [ ] Manual parity spot-check on dev: `DELETE /admin/admin-users/:id` for a non-self, non-last target → still `200 {"success":true}`; then `GET /admin/admin-user-audit` → `{rows:[…one new row…], total}` with correct actor/target emails and a `created_at`.

## Deployment (requires explicit per-batch authorization — do NOT deploy without asking)

Per spec §7: merge feat→develop→main (`--no-ff`); apply the migration on prod DB BEFORE the new image starts; rebuild `draftright-backend-go:latest` from main + `compose up -d backend-go` @ `/opt/draftright`; admin `npm run build` + sudo-rsync `dist/` to the admin web root. Then label `#51` `status: deployed to production` + add the mandatory `## ✅ How to Verify` comment. Leave the issue OPEN.

---

## Self-Review

**1. Spec coverage:**
- §1 table → Task 1 (migration + schema mirror). ✅
- §2 atomic write → Task 4 (tx, missing-target no-op, guards unchanged). ✅
- §3 read endpoint `{rows,total}` + limit/offset → Task 5 + Task 6 mount. ✅
- §4 portal page → Task 7. ✅
- §5 testing (audit on success, not on guard-reject; handler returns/limit/offset) → Task 4 handler tests + Task 5 read tests. ✅
- §6 Go-only + gate-skip → Task 6 Step 4 (routes.txt comment, no probe line). ✅
- §7 deployment → Deployment section. ✅
- §8 opens (actor email in-tx, `{rows,total}`) → Task 4 repo tx reads both emails by id; Task 5 response shape. ✅
- **Intentional deviation from spec §1:** the spec listed `deploy/shadow/augment.sql` as a third migration target to seed fixture rows. Omitted by design — the read route is gate-skipped, so the gate never reads audit rows; seeding them buys nothing (YAGNI) and an INSERT against a not-yet-migrated shadow template would break the whole augment file. The Final Verification step instead applies the migration to `draftright_dev` before the template clone.

**2. Placeholder scan:** No TBD/TODO; every code step shows full code; commands have expected output. ✅

**3. Type consistency:** `AdminUserAuditOut` fields (ID, ActorAdminID, ActorEmail, TargetAdminID, TargetEmail, CreatedAt) identical across Task 3 (def), Task 4 (insert params via sqlc), Task 5 (repo map + handler). `SoftDeleteWithAudit(ctx, actorID, targetID)` signature identical across handler interface, usecase, repo, and the test fake. `adminAuditPaginatedResponse{Rows,Total}` matches `adminUsersPaginatedResponse`. Task 5 Step 5 explicitly flags verifying sqlc-generated field/param names (`ListAdminUserAuditParams`, `row.ActorAdminID`) against the actual generated structs. ✅
