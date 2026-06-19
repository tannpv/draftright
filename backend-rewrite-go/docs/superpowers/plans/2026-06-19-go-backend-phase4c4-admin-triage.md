# Phase 4c-4 — Admin Triage Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the final 19 NestJS admin routes (errors, bug-reports, inbox, releases, grant) + 2 hourly AI fix-proposal crons to Go as a byte-identical drop-in.

**Architecture:** Extend existing modules (`errreport`, `bugreports`, `updates`, `subscription`, `aiprovider`); add one new flat module (`inbox`) and an hourly scheduler (`scheduler/hourly.go`). A shared `aiprovider.Service.Propose` facade resolves the default provider and runs the completion for both suggest-fix routes and both crons. Routes mount in the existing `shared/router.go` admin group (RequireAuth → RequireAdmin), 19 nil-guarded `http.Handler` fields. `cmd/server/main.go` wires repos→services→handlers and starts the 2 crons gated on `DISABLE_FIX_PROPOSAL_CRON`.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc v1.31, slog, testify.

**Spec:** `docs/superpowers/specs/2026-06-19-go-backend-phase4c4-admin-triage-design.md` (read §4–§10 for the verbatim contracts each task ports).

---

## Standing rules for every task

- **Byte-parity is the bar.** Status codes, JSON keys, key order, error strings, and validation messages must match the NestJS source verbatim. The shadow gate compares bytes.
- **TDD:** write the failing test, run red, implement minimal, run green, commit.
- **Stage ONLY this task's files. NEVER `git add -A`.** Each commit lists exact paths.
- **Clean-arch invariants:** use cases import NO pgx/chi/net/http. Consumer-side ports, 1–3 methods. `inbox` imports sibling **exported data types** only.
- **Go struct field order = JSON key order.** Response structs below list fields in Node's exact order.
- After SQL changes: `sqlc generate`, stage `internal/shared/pg/sqlc`, confirm no drift.
- Full gate (run in the final task, and whenever in doubt): `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./... -race`, `sqlc generate` (no drift).

## Parity landmines (keep open while implementing — spec §10)

1. Not-found: **errors → 400 "not found"**; **bugs → 404 "bug report not found"**.
2. List wrappers: errors `{items,total}` (bespoke parse); bugs `{rows,total}` (parseListQuery).
3. DELETE: errors `{id, deleted}` (idempotent); bugs `{success:true}` (404 if absent).
4. PATCH: errors inline body, 400 `"status required"`; bugs DTO with verbatim messages.
5. POST = 201; GET/PATCH/DELETE = 200.
6. Screenshot content-type by `.png` suffix (case-insensitive) else jpeg; disposition filename non-word→`_`.
7. suggest-fix side effects: error → `status=3`; bug → `ai_fix_proposed_at=now` + `status='fix_proposed'` **only if was 'new'**.
8. `error_reports` has **NO `ai_fix_proposed_at`** column.
9. Inbox shapes verbatim (`errorStatusLabel` array; `created_at = last_seen_at ?? first_seen_at`; `(no message)`/`(no description)` fallbacks).
10. AI prompts hardcoded verbatim; user text bounded **8000 chars**.
11. Grant query must be `:one RETURNING` (existing `:exec` returns nothing).
12. Releases GET key order = PLATFORMS order (`mac, windows, linux, android, ios`), each `{policy, channels:{direct, store}}`.
13. Crons: shared toggle gates BOTH; never throw; **1000ms** between calls; log `"X succeeded, Y failed"`; LIMIT **10** errors / **5** bugs.
14. **`findDefault()` filters `is_default=true AND is_active=true`** — not just is_default.

---

# GROUP A — Shared AI fix-proposal foundation

> Both `suggest-fix` routes and both crons converge on: resolve default provider → if none, 400 "No default AI provider configured" → `callProvider(provider, system, user)`. Build this once in `aiprovider` and inject it as a consumer port into `errreport` + `bugreports`. The provider→completer bridge (`Factory.For`) and the blocking `Completer.Complete` already exist (`internal/aiprovider/completer_factory.go`, `internal/aicall/completer.go`).

### Task A1: `GetDefaultAiProvider` query + `GetDefault` repo method

**Files:**
- Modify: `internal/shared/pg/queries_aiprovider.sql` (or wherever ai_providers queries live — grep `name: .*AiProvider`; if none, create `queries_aiprovider.sql`)
- Regenerate: `internal/shared/pg/sqlc/*`
- Modify: `internal/aiprovider/repo_pg.go`, `internal/aiprovider/usecase.go` (add to `Repo` interface)
- Test: `internal/aiprovider/repo_pg_test.go` (if it has a fake querier) — otherwise assert at usecase level in A2

- [ ] **Step 1: Add SQL.** Mirror the existing single-row provider query in that file. The Node query is `findOne({ where: { is_default: true, is_active: true } })`:

```sql
-- name: GetDefaultAiProvider :one
-- Node AiProvidersService.findDefault(): the active default provider.
SELECT * FROM ai_providers
WHERE is_default = true AND is_active = true
LIMIT 1;
```

Run `sqlc generate`.

- [ ] **Step 2: Add `GetDefault` to the `Repo` interface** in `internal/aiprovider/usecase.go`:

```go
GetDefault(ctx context.Context) (AiProvider, error)
```

- [ ] **Step 3: Implement on `*PgRepo`** in `internal/aiprovider/repo_pg.go`, mapping the sqlc row to `AiProvider` exactly like the existing `GetByID` does, and translating sqlc `ErrNoRows` → `ErrNotFound`:

```go
// GetDefault returns the active default provider, or ErrNotFound when none is
// configured (Node findDefault() → 400 "No default AI provider configured").
func (r *PgRepo) GetDefault(ctx context.Context) (AiProvider, error) {
	row, err := r.q.GetDefaultAiProvider(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AiProvider{}, ErrNotFound
		}
		return AiProvider{}, err
	}
	return mapRow(row), nil  // reuse the same mapper GetByID uses
}
```

- [ ] **Step 4: Build green.** `go build ./internal/aiprovider/... && go test ./internal/aiprovider/... -race` → PASS (existing tests still green; the fake querier may need the new method stubbed — add it returning a zero row + nil).
- [ ] **Step 5: Commit.** `git add internal/shared/pg/queries_aiprovider.sql internal/shared/pg/sqlc internal/aiprovider/repo_pg.go internal/aiprovider/usecase.go internal/aiprovider/repo_pg_test.go` →
`feat(aiprovider): GetDefault repo method + GetDefaultAiProvider query (Node findDefault parity)`

### Task A2: `aiprovider.Service.Propose` facade + `ErrNoDefaultProvider`

**Files:**
- Modify: `internal/aiprovider/usecase.go`
- Test: `internal/aiprovider/usecase_test.go`

- [ ] **Step 1: Write failing tests** in `internal/aiprovider/usecase_test.go`. Use the existing fake repo + fake factory/completer the `Test()` tests already use. Three cases:

```go
func TestPropose_NoDefault(t *testing.T) {
	svc := aiprovider.NewService(&fakeRepo{getDefaultErr: aiprovider.ErrNotFound}, fakeFactory{})
	_, err := svc.Propose(context.Background(), "sys", "user")
	require.ErrorIs(t, err, aiprovider.ErrNoDefaultProvider)
}

func TestPropose_OK(t *testing.T) {
	svc := aiprovider.NewService(&fakeRepo{def: aiprovider.AiProvider{Type: "openai"}}, fakeFactory{text: "ANALYSIS"})
	got, err := svc.Propose(context.Background(), "sys", "user")
	require.NoError(t, err)
	require.Equal(t, "ANALYSIS", got)
}

func TestPropose_CompleterError(t *testing.T) {
	svc := aiprovider.NewService(&fakeRepo{def: aiprovider.AiProvider{Type: "openai"}}, fakeFactory{err: errors.New("boom")})
	_, err := svc.Propose(context.Background(), "sys", "user")
	require.Error(t, err)
	require.NotErrorIs(t, err, aiprovider.ErrNoDefaultProvider)
}
```

(Add `getDefaultErr`/`def` fields + a `GetDefault` method to the existing fake repo; `fakeFactory{text,err}` likely already exists for `Test()` — extend if needed.)

- [ ] **Step 2: Run red.** `go test ./internal/aiprovider/... -race` → FAIL (Propose/ErrNoDefaultProvider undefined).

- [ ] **Step 3: Implement** in `internal/aiprovider/usecase.go`:

```go
// ErrNoDefaultProvider maps to Node's BadRequestException('No default AI
// provider configured') thrown by findDefault() when no active default exists.
// The HTTP edge translates this to 400 with that exact message.
var ErrNoDefaultProvider = errors.New("No default AI provider configured")

// Propose resolves the active default provider and runs one blocking
// completion. Mirrors Node: aiProviders.findDefault() then callProvider(
// provider, system, user). Used by errreport/bugreports suggest-fix + both
// hourly crons. Returns ErrNoDefaultProvider when no default is configured.
func (s *Service) Propose(ctx context.Context, system, user string) (string, error) {
	p, err := s.repo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrNoDefaultProvider
		}
		return "", err
	}
	c, err := s.factory.For(p)
	if err != nil {
		return "", err
	}
	text, _, err := c.Complete(ctx, system, user)
	if err != nil {
		return "", err
	}
	return text, nil
}
```

- [ ] **Step 4: Run green.** `go test ./internal/aiprovider/... -race` → PASS.
- [ ] **Step 5: Commit.** `git add internal/aiprovider/usecase.go internal/aiprovider/usecase_test.go` →
`feat(aiprovider): Propose facade (default-provider lookup + completion) for suggest-fix + crons`

---

# GROUP B — Errors admin (routes E1–E6 + errors cron)

> Spec §4 + §9.2. `internal/errreport` is ingest-only today (see `usecase.go`, `domain.go`). Add admin repo methods, an admin entity, an admin use case, an admin handler, and the errors cron. Not-found → **400 "not found"**.

### Task B1: `ErrorReportEntity` + admin sqlc queries + repo methods

**Files:**
- Modify: `internal/errreport/domain.go` (add `ErrorReportEntity`)
- Create: `internal/shared/pg/queries_errreport.sql` (if admin queries don't already share a file — grep `name: .*Error`; reuse the existing errreport query file if present)
- Regenerate: `internal/shared/pg/sqlc/*`
- Modify: `internal/errreport/repo_pg.go`
- Test: `internal/errreport/repo_pg_test.go` (create if absent; otherwise cover at usecase level)

- [ ] **Step 1: Add the admin entity** to `internal/errreport/domain.go`. Field order = Node ErrorReport row key order (spec §4), so JSON marshals identically:

```go
// ErrorReportEntity is the full error_reports row returned by the admin
// read/list/patch/delete routes. JSON key order mirrors the Node entity
// (src/errors/entities/error-report.entity.ts) exactly.
type ErrorReportEntity struct {
	ID            string          `json:"id"`
	DisplayNo     *int            `json:"display_no"`
	Platform      string          `json:"platform"`
	AppVersion    *string         `json:"app_version"`
	Severity      string          `json:"severity"`
	ErrorType     *string         `json:"error_type"`
	Message       *string         `json:"message"`
	StackTrace    *string         `json:"stack_trace"`
	Context       json.RawMessage `json:"context"`
	UserID        *string         `json:"user_id"`
	DeviceID      *string         `json:"device_id"`
	Fingerprint   string          `json:"fingerprint"`
	Count         int             `json:"count"`
	Status        int             `json:"status"`
	AiFixProposal *string         `json:"ai_fix_proposal"`
	ResolvedBy    *string         `json:"resolved_by"`
	ResolvedAt    *time.Time      `json:"resolved_at"`
	FirstSeenAt   time.Time       `json:"first_seen_at"`
	LastSeenAt    time.Time       `json:"last_seen_at"`
}
```

> Confirm each column's nullability + Go type against the sqlc `ErrorReport` model before finalizing pointers vs values. `display_no` may be a serial that is non-null — match the sqlc model. `context` is jsonb (marshal raw so `null`/`{}` round-trips byte-identically; if the column is null Node returns `null`).

- [ ] **Step 2: Add SQL.** The admin list filters are dynamic (optional platform/status/severity + limit/offset, ORDER BY `last_seen_at DESC`). Use a single query with COALESCE-style optional filters, OR build it via pgxpool like `listquery` does. Simplest parity-safe approach mirroring Node's QueryBuilder: a `:many` with nullable filter params.

```sql
-- name: AdminListErrors :many
-- Node ErrorsService.list(): optional platform/status/severity filters,
-- ORDER BY last_seen_at DESC, LIMIT/OFFSET. NULL filter param = no filter.
SELECT * FROM error_reports
WHERE (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform'))
  AND (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'))
ORDER BY last_seen_at DESC
LIMIT $1 OFFSET $2;

-- name: AdminCountErrors :one
SELECT COUNT(*) FROM error_reports
WHERE (sqlc.narg('platform')::text IS NULL OR platform = sqlc.narg('platform'))
  AND (sqlc.narg('status')::int IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'));

-- name: AdminGetError :one
SELECT * FROM error_reports WHERE id = $1;

-- name: AdminDeleteError :execrows
DELETE FROM error_reports WHERE id = $1;

-- name: AdminSetErrorStatus :one
UPDATE error_reports
SET status = $2,
    resolved_at = $3,
    resolved_by = $4
WHERE id = $1
RETURNING *;

-- name: AdminSetErrorFixProposal :one
UPDATE error_reports
SET ai_fix_proposal = $2, status = $3
WHERE id = $1
RETURNING *;

-- name: AdminErrorFixCandidates :many
-- Cron: status=0 AND ai_fix_proposal IS NULL AND count >= 2,
-- ORDER BY count DESC, last_seen_at DESC, LIMIT 10.
SELECT * FROM error_reports
WHERE status = 0 AND ai_fix_proposal IS NULL AND count >= 2
ORDER BY count DESC, last_seen_at DESC
LIMIT $1;
```

Run `sqlc generate`.

> `AdminSetErrorStatus` takes resolved_at/resolved_by computed in the use case (B3), NOT in SQL — the use case decides whether to set them based on `status ∈ {4,5}`. Pass `nil`/zero when not resolving.

- [ ] **Step 3: Add repo methods** to `internal/errreport/repo_pg.go`, each mapping the sqlc row → `ErrorReportEntity` via a shared `mapEntity(sqlc.ErrorReport) ErrorReportEntity` helper:

```go
func (r *PgRepo) AdminList(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error)
func (r *PgRepo) AdminGet(ctx context.Context, id string) (ErrorReportEntity, error) // ErrNotFound on no rows
func (r *PgRepo) AdminDelete(ctx context.Context, id string) (bool, error)           // affected>0
func (r *PgRepo) AdminSetStatus(ctx context.Context, id string, status int, resolvedAt *time.Time, resolvedBy *string) (ErrorReportEntity, error)
func (r *PgRepo) AdminSetFixProposal(ctx context.Context, id string, proposal string, status int) (ErrorReportEntity, error)
func (r *PgRepo) AdminFixCandidates(ctx context.Context, limit int32) ([]string, error) // ids only
```

Where `AdminListFilter struct { Platform, Severity *string; Status *int; Limit, Offset int }`. Add `var ErrNotFound = errors.New("error report not found")` to `domain.go` (used internally; the HTTP message is "not found" per the use case, see B3).

- [ ] **Step 4: Test** the mapper + at least the filter pass-through with a fake querier in `internal/errreport/repo_pg_test.go` (mirror the bugreports `repo_pg_test.go` style). Run `go test ./internal/errreport/... -race` → PASS.
- [ ] **Step 5: Commit.** `git add internal/errreport/domain.go internal/shared/pg/queries_errreport.sql internal/shared/pg/sqlc internal/errreport/repo_pg.go internal/errreport/repo_pg_test.go` →
`feat(errreport): admin entity + list/get/delete/setstatus/fixproposal/candidates repo methods`

### Task B2: errreport admin use case (E1–E5 logic, no HTTP)

**Files:**
- Create: `internal/errreport/admin_usecase.go`
- Test: `internal/errreport/admin_usecase_test.go`

- [ ] **Step 1: Write failing tests.** Fake an `adminRepo` (the B1 methods) + a `fixProposer` port. Cover: not-found → `ErrNotFound`; `SetStatus` with status 4 sets resolved_at/resolved_by="admin" (and resolved_by passthrough); `SetStatus` status 2 leaves resolved_* nil; `SuggestFix` builds the §3.2 prompt, calls Propose, persists with status=3; `SuggestFix` no-default → `ErrNoDefaultProvider` (propagated).

```go
func TestAdminSetStatus_ResolvedFields(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, func() time.Time { return now })
	_, err := svc.SetStatus(context.Background(), "e1", 4, "")
	require.NoError(t, err)
	require.Equal(t, &now, repo.gotResolvedAt)
	require.Equal(t, "admin", *repo.gotResolvedBy)
}

func TestAdminSetStatus_NonResolvedLeavesNil(t *testing.T) {
	repo := &fakeAdminRepo{}
	svc := errreport.NewAdminService(repo, fakeProposer{}, time.Now)
	_, _ = svc.SetStatus(context.Background(), "e1", 2, "")
	require.Nil(t, repo.gotResolvedAt)
	require.Nil(t, repo.gotResolvedBy)
}

func TestAdminSuggestFix_SetsStatus3(t *testing.T) {
	repo := &fakeAdminRepo{getEntity: errreport.ErrorReportEntity{ID: "e1", Platform: "ios", Count: 3}}
	svc := errreport.NewAdminService(repo, fakeProposer{text: "ROOT CAUSE\n..."}, time.Now)
	_, err := svc.SuggestFix(context.Background(), "e1")
	require.NoError(t, err)
	require.Equal(t, "ROOT CAUSE\n...", repo.gotProposal)
	require.Equal(t, 3, repo.gotProposalStatus)
}
```

- [ ] **Step 2: Run red.** `go test ./internal/errreport/... -race` → FAIL.

- [ ] **Step 3: Implement** `internal/errreport/admin_usecase.go`. Declare the consumer ports here:

```go
// adminRepo is the persistence port for admin error operations (B1).
type adminRepo interface {
	AdminList(ctx context.Context, f AdminListFilter) ([]ErrorReportEntity, int, error)
	AdminGet(ctx context.Context, id string) (ErrorReportEntity, error)
	AdminDelete(ctx context.Context, id string) (bool, error)
	AdminSetStatus(ctx context.Context, id string, status int, resolvedAt *time.Time, resolvedBy *string) (ErrorReportEntity, error)
	AdminSetFixProposal(ctx context.Context, id string, proposal string, status int) (ErrorReportEntity, error)
	AdminFixCandidates(ctx context.Context, limit int32) ([]string, error)
}

// fixProposer resolves the default AI provider and runs a completion.
// Satisfied by *aiprovider.Service.Propose.
type fixProposer interface {
	Propose(ctx context.Context, system, user string) (string, error)
}

// ErrNotFoundMsg is the exact Node message for a missing error (400, NOT 404).
const ErrNotFoundMsg = "not found"

type AdminService struct {
	repo     adminRepo
	proposer fixProposer
	now      func() time.Time
}

func NewAdminService(repo adminRepo, proposer fixProposer, now func() time.Time) *AdminService {
	return &AdminService{repo: repo, proposer: proposer, now: now}
}
```

`SetStatus(ctx, id, status, resolvedBy)`: load via AdminGet (404→ return `ErrNotFound`); if `status==4 || status==5` compute `resolvedAt=&now`, `rb := resolvedBy; if rb=="" { rb="admin" }`; call `AdminSetStatus`. `SuggestFix(ctx, id)`: AdminGet (ErrNotFound passthrough); build the §3.2 prompts (next step); `text, err := proposer.Propose(...)`; on err return it (incl. ErrNoDefaultProvider); `AdminSetFixProposal(id, text, 3)`.

- [ ] **Step 4: Add the verbatim §3.2 prompt builder** (transcribe byte-for-byte from `src/errors/errors.service.ts` `suggestFix` — the `systemPrompt` literal and the `userText` array, joined `\n`, `.slice(0,8000)`). Reuse the existing `sliceUTF16(s, 8000)` from `domain.go` for the bound:

```go
func buildErrorPrompt(e ErrorReportEntity) (system, user string) {
	system = `You are a senior software engineer reviewing a production crash report. Analyze the error and propose a concrete fix.
...` // EXACT bytes from errors.service.ts
	user = strings.Join([]string{
		"Platform: " + e.Platform,
		"App version: " + derefOr(e.AppVersion, "unknown"),
		// ... exact lines incl. blank "" separators, "Stack trace:", JSON context ...
	}, "\n")
	return system, sliceUTF16(user, 8000)
}
```

> The implementer MUST copy the system + user literals byte-for-byte from `src/errors/errors.service.ts` (captured in the spec §3.2 + the controller recon). `first_seen_at`/`last_seen_at` → `.toISOString()`; context → `JSON.stringify(ctx, null, 2)` (2-space indent). Match JS `Date.toISOString()` (`2026-06-19T12:00:00.000Z`, millisecond precision) via `e.FirstSeenAt.UTC().Format("2006-01-02T15:04:05.000Z07:00")`.

- [ ] **Step 5: Run green + commit.** `go test ./internal/errreport/... -race` → PASS.
`git add internal/errreport/admin_usecase.go internal/errreport/admin_usecase_test.go` →
`feat(errreport): admin use case — list/get/delete/setstatus/suggestfix (status=3, 400 not-found)`

### Task B3: errors cron

**Files:**
- Create: `internal/errreport/cron.go`
- Test: `internal/errreport/cron_test.go`

- [ ] **Step 1: Write failing tests.** Cover: toggle on → no candidate calls + no error; N candidates → `SuggestFix` called per id, sleep invoked between, returns success/fail tally; a `SuggestFix` error increments failed and does NOT abort the loop. Inject a fake sleeper + a `now` is not needed (no scheduling here).

```go
func TestErrorCron_RunsCandidates(t *testing.T) {
	svc := &fakeSuggest{ids: []string{"a", "b"}, failOn: map[string]bool{"b": true}}
	var sleeps int
	cron := errreport.NewCron(svc, func() bool { return false }, func(time.Duration) { sleeps++ })
	res := cron.RunOnce(context.Background())
	require.Equal(t, 1, res.Success)
	require.Equal(t, 1, res.Failed)
	require.Equal(t, []string{"a", "b"}, svc.calledIDs)
}

func TestErrorCron_DisabledSkips(t *testing.T) {
	svc := &fakeSuggest{ids: []string{"a"}}
	cron := errreport.NewCron(svc, func() bool { return true }, func(time.Duration) {})
	res := cron.RunOnce(context.Background())
	require.Equal(t, 0, res.Success)
	require.Empty(t, svc.calledIDs)
}
```

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/errreport/cron.go`. Define a `cronRepo`/`cronSvc` consumer port (`AdminFixCandidates` + `SuggestFix`). `disabled func() bool` lets main.go close over the env toggle; `sleep func(time.Duration)` is injected (`time.Sleep` in prod) so tests don't wait. LIMIT 10. Catch each `SuggestFix` error (never propagate); sleep 1000ms after each call. Return `CronResult{Success, Failed int}`. Log `"X succeeded, Y failed"` via an injected logger or return the tally for main.go to log.

```go
type CronResult struct{ Success, Failed int }

func (c *Cron) RunOnce(ctx context.Context) CronResult {
	if c.disabled() {
		return CronResult{}
	}
	ids, err := c.svc.AdminFixCandidates(ctx, 10)
	if err != nil || len(ids) == 0 {
		return CronResult{}
	}
	var res CronResult
	for _, id := range ids {
		if _, err := c.svc.SuggestFix(ctx, id); err != nil {
			res.Failed++
		} else {
			res.Success++
		}
		c.sleep(1000 * time.Millisecond)
	}
	return res
}
```

> Node sleeps 1000ms only AFTER a success (inside the try, before catch). Match exactly: in Node the `await sleep` is inside `try` after `success++`, so on failure there is NO sleep. Adjust: call `c.sleep` only in the success branch to be byte-faithful to the cron's timing. Re-check `fix-proposal.cron.ts` and mirror its control flow precisely.

- [ ] **Step 4: Run green + commit.** `go test ./internal/errreport/... -race` → PASS.
`git add internal/errreport/cron.go internal/errreport/cron_test.go` →
`feat(errreport): hourly fix-proposal cron (status=0, count>=2, limit 10, toggle-gated)`

### Task B4: errors admin handler (E1–E6)

**Files:**
- Create: `internal/errreport/admin_handler.go`
- Test: `internal/errreport/admin_handler_test.go`

- [ ] **Step 1: Write failing tests** for each route. Use `httptest`, a fake `AdminService` + fake `Cron`. Assert status codes + bodies + error envelopes:
  - E1 `GET /admin/errors?limit=5&status=0` → 200 `{items,total}`; verify limit cap 200, default 50, offset default 0 parsed via `strconv` with Node `Number()` semantics.
  - E2 `GET /admin/errors/{id}` not found → **400** `{"error":"not found","code":"invalid-input",...}` (confirm the code `inferCode(400)` maps to — match 4c-2/4c-3 400 envelopes; likely `invalid-input`).
  - E3 `PATCH` missing status → 400 `"status required"`; valid → 200 entity.
  - E4 `DELETE` → 200 `{id, deleted}` (deleted false when absent — idempotent, NOT 404).
  - E5 `POST .../suggest-fix` → **201** entity; no-default → 400 "No default AI provider configured".
  - E6 `POST /admin/errors/run-ai-cron` → **201** `{ok:true}`.

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/errreport/admin_handler.go`. One handler struct holding `*AdminService` + the `*Cron` (for E6). Methods: `List`, `Get`, `Patch`, `Delete`, `SuggestFix`, `RunCron`. Map errors: `ErrNotFound` → `shared.WriteError(w, r, "<400 code>", "not found")`; `aiprovider.ErrNoDefaultProvider` → 400 with its message; other errors → 500. Parse E1 query with `Number()` parity (`strconv.Atoi`, ignore parse failures → undefined→default, matching `Number(x)` only when present). Parse E3 inline body `{status *int, resolved_by string}` via `json.Decode`; `status==nil` → 400 "status required".

> 400 envelope `code`: check what `shared.WriteError` produces for the 400 cases in 4c-2/4c-3 (`inferCode(400)`). Use the SAME code string those phases used for BadRequestException 400s so the envelope matches. Grep `internal/shared` for the 400 code constant; do not invent one.

- [ ] **Step 4: Run green + commit.** `go test ./internal/errreport/... -race` → PASS.
`git add internal/errreport/admin_handler.go internal/errreport/admin_handler_test.go` →
`feat(errreport): admin handler — E1-E6 (errors list/get/patch/delete/suggest-fix/run-cron)`

---

# GROUP C — Bug-reports admin (routes B1–B6 + bugs cron)

> Spec §5 + §9.3. `internal/bugreports` has ingest + storage already (`storage.go`, `repo_pg.go`). Add admin entity, admin repo methods, admin use case (incl. screenshot path + suggestFix), admin handler, bugs cron. Not-found → **404 "bug report not found"**. List uses `listquery` (def 10, cap 100).

### Task C1: `BugReportEntity` + admin sqlc queries + repo methods

**Files:**
- Modify: `internal/bugreports/domain.go`
- Modify/Create: `internal/shared/pg/queries_bugreports.sql`
- Regenerate: `internal/shared/pg/sqlc/*`
- Modify: `internal/bugreports/repo_pg.go`
- Test: `internal/bugreports/repo_pg_test.go`

- [ ] **Step 1: Add `BugReportEntity`** to `domain.go`, field order = Node BugReport row key order (spec §5):

```go
type BugReportEntity struct {
	ID                 string          `json:"id"`
	DisplayNo          *int            `json:"display_no"`
	Source             string          `json:"source"`
	Description        string          `json:"description"`
	ScreenshotPath     *string         `json:"screenshot_path"`
	ScreenshotFilename *string         `json:"screenshot_filename"`
	AppVersion         *string         `json:"app_version"`
	OsInfo             *string         `json:"os_info"`
	UserID             *string         `json:"user_id"`
	UserEmail          *string         `json:"user_email"`
	Context            json.RawMessage `json:"context"`
	Status             string          `json:"status"`
	Kind               string          `json:"kind"`
	Title              *string         `json:"title"`
	TargetPlatform     *string         `json:"target_platform"`
	VoteCount          int             `json:"vote_count"`
	IsPublic           bool            `json:"is_public"`
	AdminNotes         *string         `json:"admin_notes"`
	AiFixProposal      *string         `json:"ai_fix_proposal"`
	AiFixProposedAt    *time.Time      `json:"ai_fix_proposed_at"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}
```

> Confirm nullability vs the sqlc `BugReport` model. The public-feed/ingest path may already map a subset — reuse its mapper if it covers these columns; otherwise add `mapBugEntity`.

- [ ] **Step 2: Add SQL.** The admin list is a `listquery` consumer (dynamic sort/search) → run it via pgxpool like the 4c-2/4c-3 listquery consumers, NOT a static sqlc query. Add static sqlc queries for the rest:

```sql
-- name: AdminGetBugReport :one
SELECT * FROM bug_reports WHERE id = $1;

-- name: AdminDeleteBugReport :execrows
DELETE FROM bug_reports WHERE id = $1;

-- name: AdminUpdateBugReport :one
UPDATE bug_reports
SET status = $2, admin_notes = $3, title = $4, target_platform = $5, is_public = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AdminSetBugFixProposal :one
UPDATE bug_reports
SET ai_fix_proposal = $2, ai_fix_proposed_at = $3, status = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: AdminBugFixCandidates :many
-- Cron: status='new' AND kind='bug' AND ai_fix_proposal IS NULL,
-- ORDER BY created_at DESC, LIMIT 5.
SELECT * FROM bug_reports
WHERE status = 'new' AND kind = 'bug' AND ai_fix_proposal IS NULL
ORDER BY created_at DESC
LIMIT $1;
```

> `AdminUpdateBugReport` writes ALL patchable columns; the use case (C2) does partial-merge in Go (load → apply only provided fields → write the merged values), mirroring Node's per-field `if (dto.x !== undefined)`. Don't try to do COALESCE partial-update in SQL — the validation + merge must happen in the use case for byte-parity of messages.

Run `sqlc generate`.

- [ ] **Step 3: Add repo methods** (`AdminList` via pgxpool listquery, `AdminGet`→ErrNotFound, `AdminDelete`→bool, `AdminUpdate`, `AdminSetFixProposal`, `AdminFixCandidates`). `AdminList` filter struct carries the listquery `Built` + `status/kind/target_platform` custom filters; build the WHERE exactly like Node `findAllPaginated` (status in ALLOWED or 'all', kind in bug/feature, target_platform in TARGET_PLATFORMS), search cols `br.description, br.title, br.user_email, br.source`, sort allow-list `created_at/updated_at/status/source/kind/vote_count`→`br.*`, default `br.created_at DESC`. Response wrapper key = `{rows, total}`.

- [ ] **Step 4: Test mapper + filter SQL** with the fake querier / a thin integration if the repo test uses pgxmock. Run `go test ./internal/bugreports/... -race` → PASS.
- [ ] **Step 5: Commit.** `git add internal/bugreports/domain.go internal/shared/pg/queries_bugreports.sql internal/shared/pg/sqlc internal/bugreports/repo_pg.go internal/bugreports/repo_pg_test.go` →
`feat(bugreports): admin entity + list/get/update/delete/fixproposal/candidates repo methods`

### Task C2: bugreports admin use case (B2, B4, B6 + screenshot path)

**Files:**
- Create: `internal/bugreports/admin_usecase.go`
- Test: `internal/bugreports/admin_usecase_test.go`

- [ ] **Step 1: Write failing tests.** Cover: `FindByID` not-found → `ErrNotFound`; `Update` validation messages verbatim (status not in ALLOWED → `"status must be one of: new, reviewing, fix_proposed, resolved, wont_fix"`; title empty/over-80 → `"title is required (1-80 characters)"`; bad target_platform → `"target_platform must be one of: playground, mobile, windows, mac, linux"`); merge order status→admin_notes→title→target_platform→is_public; `SuggestFix` sets ai_fix_proposal + ai_fix_proposed_at, status→'fix_proposed' ONLY if was 'new' (test a 'reviewing' row stays 'reviewing'); `GetScreenshot` returns path+filename or nil.

```go
func TestBugUpdate_BadStatusMessage(t *testing.T) {
	repo := &fakeBugAdminRepo{get: bugreports.BugReportEntity{ID: "b1", Status: "new"}}
	svc := bugreports.NewAdminService(repo, fakeProposer{}, time.Now)
	_, err := svc.Update(context.Background(), "b1", bugreports.BugPatch{Status: ptr("bogus")})
	require.EqualError(t, err, "status must be one of: new, reviewing, fix_proposed, resolved, wont_fix")
}

func TestBugSuggestFix_OnlyNewBecomesFixProposed(t *testing.T) {
	now := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	repo := &fakeBugAdminRepo{get: bugreports.BugReportEntity{ID: "b1", Status: "reviewing", Description: "x"}}
	svc := bugreports.NewAdminService(repo, fakeProposer{text: "LIKELY ROOT CAUSE..."}, func() time.Time { return now })
	_, _ = svc.SuggestFix(context.Background(), "b1")
	require.Equal(t, "reviewing", repo.gotStatus) // unchanged (was not 'new')
	require.Equal(t, &now, repo.gotProposedAt)
}
```

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/bugreports/admin_usecase.go`. Constants:

```go
var bugAllowedStatuses = []string{"new", "reviewing", "fix_proposed", "resolved", "wont_fix"}
var bugTargetPlatforms = []string{"playground", "mobile", "windows", "mac", "linux"}
var ErrNotFound = errors.New("bug report not found")
```

`BugPatch struct { Status, AdminNotes, Title, TargetPlatform *string; IsPublic *bool }`. `Update`: AdminGet (ErrNotFound); validate+merge in Node's exact order (status, admin_notes, title[trim, 1-80], target_platform, is_public), each with its verbatim BadRequestException message returned as a Go error; call `AdminUpdate` with the merged values. `SuggestFix`: AdminGet; build §3.3 prompts verbatim from `src/bug-reports/bug-reports.service.ts`; `text, err := proposer.Propose`; `status := row.Status; if status=="new" { status = "fix_proposed" }`; `AdminSetFixProposal(id, text, &now, status)`. `GetScreenshot(id)`: AdminGet; if `ScreenshotPath==nil` return `(nil, nil)`; else return path+filename (filename falls back to `filepath.Base(path)` when `ScreenshotFilename` empty — mirror Node).

- [ ] **Step 4: Transcribe the §3.3 prompt** byte-for-byte (system has NO stack trace; sections LIKELY ROOT CAUSE / LIKELY AREA OF CODE / PROPOSED FIX OR INVESTIGATION / CONFIDENCE / REPRODUCTION QUESTIONS; user lines Source / Platform·OS / App version / Submitted / User email / Has screenshot / description / context). Bound `sliceUTF16(user, 8000)`. `created_at.toISOString()` via the `.000Z` format helper.

- [ ] **Step 5: Run green + commit.** `go test ./internal/bugreports/... -race` → PASS.
`git add internal/bugreports/admin_usecase.go internal/bugreports/admin_usecase_test.go` →
`feat(bugreports): admin use case — get/update(validate)/delete/suggestfix/screenshot (404 not-found)`

### Task C3: bugs cron

**Files:**
- Create: `internal/bugreports/cron.go`
- Test: `internal/bugreports/cron_test.go`

- [ ] **Step 1: Write failing tests** mirroring B3's cron tests but LIMIT 5 and candidates by id. Toggle-gated; per-candidate `SuggestFix`; failure does not abort; sleep 1000ms (match Node control flow — sleep only after success, see B3 note).
- [ ] **Step 2: Run red.** FAIL.
- [ ] **Step 3: Implement** `internal/bugreports/cron.go` — same shape as errreport `Cron`, candidate port `AdminBugFixCandidates(ctx, 5)` returning ids + `SuggestFix`. Return `CronResult{Success, Failed}`.
- [ ] **Step 4: Run green + commit.** `go test ./internal/bugreports/... -race` → PASS.
`git add internal/bugreports/cron.go internal/bugreports/cron_test.go` →
`feat(bugreports): hourly fix-proposal cron (status=new,kind=bug, limit 5, toggle-gated)`

### Task C4: bugreports admin handler (B1–B6)

**Files:**
- Create: `internal/bugreports/admin_handler.go`
- Test: `internal/bugreports/admin_handler_test.go`

- [ ] **Step 1: Write failing tests.** Per route:
  - B1 `GET /admin/bug-reports` → 200 `{rows,total}`; verify parseListQuery defaults (limit 10 cap 100) + custom filters parsed.
  - B2 `GET .../{id}` not found → **404** `{"error":"bug report not found","code":"<404 code>",...}`.
  - B3 `GET .../{id}/screenshot`: png path → `Content-Type: image/png` + `Content-Disposition: inline; filename="..._..."` (non-word→_); null path → 400 "no screenshot for this report"; missing file → 400 "screenshot file missing on disk". Use a temp file on disk for the happy path; assert the streamed bytes.
  - B4 `PATCH` validation messages verbatim + happy 200.
  - B5 `DELETE` → 200 `{success:true}`; 404 if absent.
  - B6 `POST .../{id}/fix-proposal` → **201** entity; no-default → 400.

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/bugreports/admin_handler.go`. The screenshot route streams from disk: after `GetScreenshot` returns a path, `os.Stat` it (missing → 400 "screenshot file missing on disk"), set headers (content-type by `strings.HasSuffix(strings.ToLower(path), ".png")`; disposition filename via `regexp.MustCompile(\`[^\w.\-]\`).ReplaceAllString(name, "_")`), then `io.Copy` an opened `*os.File`. Map `ErrNotFound` → 404; `ErrNoDefaultProvider` → 400; validation errors → 400 with the message. parseListQuery is the shared `listquery.Parse`.

> Confirm the 404 envelope `code` (`inferCode(404)`) matches what 4c-2/4c-3 produced for NotFoundException (grep shared for the 404 code; bugreports/feedback already 404 somewhere — reuse it).

- [ ] **Step 4: Run green + commit.** `go test ./internal/bugreports/... -race` → PASS.
`git add internal/bugreports/admin_handler.go internal/bugreports/admin_handler_test.go` →
`feat(bugreports): admin handler — B1-B6 (list/get/screenshot/patch/delete/fix-proposal)`

---

# GROUP D — Inbox (routes I1–I2)

> Spec §6. NEW flat module `internal/inbox`. Composes errreport + bugreports list services through 2 consumer ports. No DB of its own.

### Task D1: inbox domain + use case (I1 counts + I2 feed)

**Files:**
- Create: `internal/inbox/domain.go`, `internal/inbox/usecase.go`
- Test: `internal/inbox/usecase_test.go`

- [ ] **Step 1: Write failing tests.** Fake the 2 ports. Cover:
  - `Counts`: bug new/bug, bug new/feature, error status0 totals → `{new_bugs, new_features, new_errors, total=sum}`.
  - `Feed`: errorItem mapping (title `[type,msg].join(': ').slice(200) || '(no message)'`; status via errorStatusLabel array incl. idx 2&4 → 'resolved'; created_at = last_seen ?? first_seen); bugItem mapping (title `description.slice(200) || '(no description)'`; platform=os_info; status raw; has_screenshot = path!=nil); merge + sort created_at DESC + slice(limit); `kind=='bug'` filters out errors; `kind=='error'` filters out bugs; `status=='open'` passes status0/'new' filters; counts `{errors, bugs, returned}`.

```go
func TestErrorStatusLabel(t *testing.T) {
	cases := map[int]string{0: "new", 1: "reviewing", 2: "resolved", 3: "fix_proposed", 4: "resolved", 5: "wont_fix", 9: "new"}
	for in, want := range cases {
		require.Equal(t, want, inbox.ErrorStatusLabel(in))
	}
}

func TestFeed_MergeSortSlice(t *testing.T) {
	errs := errLister{items: []errreport.ErrorReportEntity{{ID: "e1", ErrorType: ptr("TypeError"), Message: ptr("boom"), LastSeenAt: t2(10)}}, total: 1}
	bugs := bugLister{rows: []bugreports.BugReportEntity{{ID: "b1", Description: "crash", OsInfo: ptr("iOS"), CreatedAt: t2(20)}}, total: 1}
	svc := inbox.NewService(errs, bugs)
	out, _ := svc.Feed(context.Background(), inbox.FeedQuery{Limit: 50})
	require.Equal(t, "b1", out.Items[0].ID) // newer first
	require.Equal(t, "TypeError: boom", out.Items[1].Title)
	require.Equal(t, 1, out.Counts.Errors)
	require.Equal(t, 1, out.Counts.Bugs)
	require.Equal(t, 2, out.Counts.Returned)
}
```

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/inbox/domain.go` (`InboxItem` with key order `kind, id, title, platform, app_version, status, created_at, ai_fix_proposal` + optional error extras `error_type, severity, occurrence_count` and bug extras `user_email, has_screenshot` — use `omitempty` carefully so the per-kind extras only appear for their kind; match Node which sets only the relevant extras). Note: Node always includes the base 8 keys; error items add 3 error keys, bug items add 2 bug keys. To get exact key sets per kind, use TWO structs (errorItem/bugItem) sharing the base via embedding, OR a single struct where the irrelevant pointers are nil + `omitempty`. **Verify against Node:** Node's object literal includes `error_type/severity/occurrence_count` only in the errorItems map and `user_email/has_screenshot` only in bugItems — so a bug item's JSON has NO `error_type` key. Use two distinct structs to guarantee the key sets differ.

`ErrorStatusLabel(s int) string` = `[]string{"new","reviewing","resolved","fix_proposed","resolved","wont_fix"}` indexed, default "new".

`usecase.go` declares the 2 consumer ports:

```go
// errorLister is errreport's admin list, as inbox needs it.
type errorLister interface {
	AdminList(ctx context.Context, f errreport.AdminListFilter) ([]errreport.ErrorReportEntity, int, error)
}
// bugLister is bugreports' admin list.
type bugLister interface {
	AdminList(ctx context.Context, f bugreports.AdminListFilter) ([]bugreports.BugReportEntity, int, error)
}
```

`Counts` issues 3 list calls with limit 1 (bug new/bug, bug new/feature, error status0) and reads totals. `Feed(q)`: `wantErrors = q.Kind != "bug"`, `wantBugs = q.Kind != "error"`, `openOnly = q.Status == "open"`; call each lister with the right filter (errors status `0` when openOnly else nil, limit, offset 0; bugs page1, limit, status 'new' when openOnly else "", sort created_at DESC); map → items; merge; sort by created_at desc; slice(limit). Sort must be stable and match JS `.sort((a,b)=>b-a)` on `getTime()`.

> Limit parse for I2 lives in the handler (D2): `min(parseInt(limit||'50')||50, 100)`.

- [ ] **Step 4: Run green + commit.** `go test ./internal/inbox/... -race` → PASS.
`git add internal/inbox/domain.go internal/inbox/usecase.go internal/inbox/usecase_test.go` →
`feat(inbox): use case — counts + merged time-sorted feed (errreport + bugreports)`

### Task D2: inbox handler (I1, I2)

**Files:**
- Create: `internal/inbox/handler.go`
- Test: `internal/inbox/handler_test.go`

- [ ] **Step 1: Write failing tests.** `GET /admin/inbox/counts` → 200 `{new_bugs,new_features,new_errors,total}`. `GET /admin/inbox?kind=bug&status=open&limit=5` → 200 `{items, counts}`; assert limit parse cap 100, default 50; assert JSON key order of one item (golden string).
- [ ] **Step 2: Run red.** FAIL.
- [ ] **Step 3: Implement** `internal/inbox/handler.go` — `Counts` + `Feed` methods. Parse `limit` with `min(atoiOr(limit,50), 100)`; pass kind/status through. `shared.WriteJSON(w, 200, ...)`.
- [ ] **Step 4: Run green + commit.** `go test ./internal/inbox/... -race` → PASS.
`git add internal/inbox/handler.go internal/inbox/handler_test.go` →
`feat(inbox): handler — GET /admin/inbox/counts + GET /admin/inbox`

---

# GROUP E — Releases (routes R1–R4)

> Spec §7. Extend `internal/updates`. Existing public reads stay. Add admin queries + admin use case + admin handler. GET key order = PLATFORMS (`mac, windows, linux, android, ios`).

### Task E1: release admin sqlc queries + repo methods

**Files:**
- Modify: `internal/shared/pg/queries_updates.sql` (grep for existing release queries; reuse the file)
- Regenerate: `internal/shared/pg/sqlc/*`
- Modify: `internal/updates/repo_pg.go`
- Test: `internal/updates/repo_pg_test.go`

- [ ] **Step 1: Add SQL** (find-all + upsert + delete for releases; find-all + upsert for policies):

```sql
-- name: ListAllReleases :many
SELECT * FROM app_releases;

-- name: ListAllReleasePolicies :many
SELECT * FROM app_release_policies;

-- name: UpsertReleaseChannel :one
INSERT INTO app_releases (platform, channel, version, download_url, sha256, release_notes, required, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (platform, channel) DO UPDATE SET
  version = EXCLUDED.version,
  download_url = EXCLUDED.download_url,
  sha256 = EXCLUDED.sha256,
  release_notes = EXCLUDED.release_notes,
  required = EXCLUDED.required,
  enabled = EXCLUDED.enabled,
  updated_at = now()
RETURNING *;

-- name: DeleteReleaseChannel :execrows
DELETE FROM app_releases WHERE platform = $1 AND channel = $2;

-- name: UpsertReleasePolicy :one
INSERT INTO app_release_policies (platform, preferred, store_status, notes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (platform) DO UPDATE SET
  preferred = EXCLUDED.preferred,
  store_status = EXCLUDED.store_status,
  notes = EXCLUDED.notes,
  updated_at = now()
RETURNING *;
```

> Confirm the unique constraints exist: `app_releases (platform, channel)` and `app_release_policies (platform)`. Grep the migrations / `\d` the tables. If a UNIQUE constraint is missing, the ON CONFLICT target is invalid — fall back to the Node load-then-update/insert pattern in the use case (SELECT existing → UPDATE or INSERT). **The Node service does load-then-branch (not upsert)** — to be safe and byte-faithful to its column-merge semantics (it only overwrites release_notes/required/enabled when provided), prefer replicating the load-then-update in the USE CASE (E2) and keep SQL as plain `GetReleaseChannel`/`UpdateReleaseChannel`/`InsertReleaseChannel`. Decide in Step 1 after inspecting the schema; default to load-then-branch to match Node's partial-overwrite of optional fields.

Run `sqlc generate`.

- [ ] **Step 2: Add repo methods** returning `AppRelease`/`AppReleasePolicy` domain structs (define these in `updates/domain.go` with field order: AppRelease `platform, channel, version, download_url, sha256, release_notes, required, enabled, updated_at`; AppReleasePolicy `platform, preferred, store_status, notes, updated_at`). Methods: `ListAllReleases`, `ListAllPolicies`, `GetReleaseChannel(platform,channel) (*AppRelease, error)`, `UpsertRelease(...)`, `DeleteRelease(platform,channel) (affected int, err)`, `GetPolicy`, `UpsertPolicy`.

- [ ] **Step 3: Test** mapper + delete-affected with fake querier. Run `go test ./internal/updates/... -race` → PASS.
- [ ] **Step 4: Commit.** `git add internal/updates/domain.go internal/shared/pg/queries_updates.sql internal/shared/pg/sqlc internal/updates/repo_pg.go internal/updates/repo_pg_test.go` →
`feat(updates): admin release/policy repo methods (list-all, upsert, delete)`

### Task E2: releases admin use case (R1–R4 logic)

**Files:**
- Create: `internal/updates/admin_usecase.go`
- Test: `internal/updates/admin_usecase_test.go`

- [ ] **Step 1: Write failing tests.** Cover:
  - `ListAll`: seeds all 5 PLATFORMS in order, fills channels/policy, skips unknown-platform rows → nested map.
  - `UpsertChannel` validation (verbatim): bad platform → `"platform must be one of: mac, windows, linux, android, ios"`; bad channel → `"channel must be one of: direct, store"`; missing version/download_url → `"version and download_url are required"`; bad sha256 → `"sha256 must be a 64-char hex string (or empty)"`; sha256 lowercased; defaults (channel direct, sha256 '', notes '', required false, enabled true).
  - `DeleteChannel` affected=0 → `ErrReleaseNotFound` with message `"No release row for {platform}/{channel}"`.
  - `UpsertPolicy` validation (verbatim): platform, preferred (`"preferred must be one of: direct, store"`), store_status (`"store_status must be one of: not_submitted, in_review, approved, rejected, n/a"`); defaults preferred direct, store_status not_submitted, notes ''.

```go
func TestUpsertChannel_BadSha(t *testing.T) {
	svc := updates.NewAdminService(&fakeRelRepo{})
	_, err := svc.UpsertChannel(context.Background(), updates.UpsertChannelInput{Platform: "mac", Channel: "direct", Version: "1", DownloadURL: "u", Sha256: "xyz"})
	require.EqualError(t, err, "sha256 must be a 64-char hex string (or empty)")
}
```

- [ ] **Step 2: Run red.** FAIL.

- [ ] **Step 3: Implement** `internal/updates/admin_usecase.go`. Constants `platforms = []string{"mac","windows","linux","android","ios"}`, `channels = []string{"direct","store"}`, `storeStatuses = []string{"not_submitted","in_review","approved","rejected","n/a"}`. `ListAll`: build `map[string]PlatformReleases` but RETURN an ordered structure — since Go maps don't preserve order and JSON key order must = PLATFORMS, model the response as a struct with explicit fields:

```go
// ReleasesView is the GET /admin/releases body. Field order = PLATFORMS order
// so JSON keys serialize mac, windows, linux, android, ios.
type ReleasesView struct {
	Mac     PlatformReleases `json:"mac"`
	Windows PlatformReleases `json:"windows"`
	Linux   PlatformReleases `json:"linux"`
	Android PlatformReleases `json:"android"`
	Ios     PlatformReleases `json:"ios"`
}
type PlatformReleases struct {
	Policy   *AppReleasePolicy `json:"policy"`
	Channels Channels          `json:"channels"`
}
type Channels struct {
	Direct *AppRelease `json:"direct"`
	Store  *AppRelease `json:"store"`
}
```

`UpsertChannel`: validate (verbatim messages → Go errors), default channel/sha/etc, lowercase sha, then load-then-branch via repo (overwrite optional fields only when provided — mirror Node). `DeleteChannel`: `affected==0` → `fmt.Errorf("No release row for %s/%s", platform, channel)` wrapped as `ErrReleaseNotFound` (handler maps to 404). `UpsertPolicy`: validate + load-then-branch.

- [ ] **Step 4: Run green + commit.** `go test ./internal/updates/... -race` → PASS.
`git add internal/updates/admin_usecase.go internal/updates/admin_usecase_test.go` →
`feat(updates): admin use case — releases listAll/upsert/delete + policy upsert (verbatim validation)`

### Task E3: releases admin handler (R1–R4)

**Files:**
- Create: `internal/updates/admin_handler.go`
- Test: `internal/updates/admin_handler_test.go`

- [ ] **Step 1: Write failing tests.**
  - R1 `GET /admin/releases` → 200; assert key order `mac,windows,linux,android,ios` each `{policy,channels:{direct,store}}` (golden string).
  - R2 `POST /admin/releases` → **201** upserted release; validation 400 with verbatim messages.
  - R3 `DELETE /admin/releases/{platform}/{channel}` → 200 `{ok:true}`; affected 0 → **404** `"No release row for windows/store"`.
  - R4 `POST /admin/release-policies` → **201** policy; validation 400.
- [ ] **Step 2: Run red.** FAIL.
- [ ] **Step 3: Implement** `internal/updates/admin_handler.go` — `ListReleases`, `UpsertRelease`, `DeleteRelease`, `UpsertPolicy`. Decode bodies; default `channel` to "direct" before calling the use case (matches Node controller `body.channel ?? 'direct'`). Map `ErrReleaseNotFound` → 404; validation errors → 400.
- [ ] **Step 4: Run green + commit.** `go test ./internal/updates/... -race` → PASS.
`git add internal/updates/admin_handler.go internal/updates/admin_handler_test.go` →
`feat(updates): admin handler — R1-R4 (releases get/post/delete + release-policies post)`

---

# GROUP F — Subscriptions grant (route G1)

> Spec §8. Extend `internal/subscription`. Existing `CancelActiveSubsByUser` (`:exec`) is reused; change `InsertGrantedSubscription` to `:one RETURNING` so the response carries the full row.

### Task F1: grant query `:one RETURNING` + Grant repo method

**Files:**
- Modify: `internal/shared/pg/queries_auth.sql` (line ~210, the existing `InsertGrantedSubscription`)
- Regenerate: `internal/shared/pg/sqlc/*`
- Modify: `internal/subscription/writer.go` (or wherever the grant write belongs — grep `InsertGrantedSubscription`)
- Test: corresponding `_test.go`

- [ ] **Step 1: Change the query** in `queries_auth.sql` from `:exec` to `:one` with RETURNING (full row, Node returns the saved entity):

```sql
-- name: InsertGrantedSubscription :one
INSERT INTO subscriptions (user_id, plan_id, status, store_type, started_at, expires_at)
VALUES ($1, $2, 'active'::subscriptions_status_enum, $3, now(), $4)
RETURNING id, user_id, plan_id, status, store_type, store_transaction_id, started_at, expires_at, created_at, updated_at;
```

Run `sqlc generate`. (This changes the generated method's signature from `error` to `(row, error)` — fix the one existing caller, if any, to consume/ignore the returned row.)

- [ ] **Step 2: Add a `GrantedSub` domain struct** (field order = Subscription row key order, spec §8) + a `Grant` repo method that runs CancelActive then InsertGranted and maps the returned row:

```go
// GrantedSub is the POST /admin/subscriptions/grant response. Key order
// mirrors the Node Subscription entity.
type GrantedSub struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status"`
	StoreType          string     `json:"store_type"`
	StoreTransactionID *string    `json:"store_transaction_id"`
	StartedAt          time.Time  `json:"started_at"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
```

- [ ] **Step 3: Test** the repo method with the fake querier: asserts CancelActive called for user, Insert called with `store_type='admin_granted'`, `expires_at` nil when not provided, and the returned row maps to `GrantedSub`.
- [ ] **Step 4: Build + green + commit.** `go test ./internal/subscription/... -race` → PASS.
`git add internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc internal/subscription/*.go` →
`feat(subscription): grant returns full row (:one RETURNING) — admin grant repo method`

### Task F2: grant use case + admin handler (G1)

**Files:**
- Modify: `internal/subscription/admin_usecase.go` (create if the admin path has only a handler today) + `internal/subscription/admin_handler.go`
- Test: `internal/subscription/admin_handler_test.go`

- [ ] **Step 1: Write failing tests.** `POST /admin/subscriptions/grant` with `{user_id, plan_id}` (UUIDs) → **201** GrantedSub; `expires_at` provided (ISO date) → parsed and stored; invalid UUID → 400 (class-validator `@IsUUID` → `invalid-input`, message like `"user_id must be a UUID"` — confirm the exact humanized message the 4c-2 DTO validation produced for `@IsUUID` and match it); bad `expires_at` (not a date string) → 400 `@IsDateString` message.

> Match the existing DTO-validation envelope from 4c-2 (UpdateUserDto used `forbidNonWhitelisted` + `humanizeValidation`). `GrantSubscriptionDto` has `@IsUUID user_id`, `@IsUUID plan_id`, `@IsOptional @IsDateString expires_at`. Replicate the message text + property order (user_id, plan_id, expires_at) the same way the 4c-2 validator does. Grep `internal/user` / `internal/shared` for the humanize helper and reuse it.

- [ ] **Step 2: Run red.** FAIL.
- [ ] **Step 3: Implement** the grant route. Validate the DTO (UUID + optional date-string) producing verbatim messages; parse `expires_at` (`time.Parse` RFC3339 / date) → `*time.Time`; call the repo `Grant`. 201 on success.
- [ ] **Step 4: Run green + commit.** `go test ./internal/subscription/... -race` → PASS.
`git add internal/subscription/admin_usecase.go internal/subscription/admin_handler.go internal/subscription/admin_handler_test.go` →
`feat(subscription): admin grant route — POST /admin/subscriptions/grant (201, full row)`

---

# GROUP G — Hourly scheduler

### Task G1: `scheduler/hourly.go` — `NextHourlyAt` + `RunHourly`

**Files:**
- Create: `internal/platform/scheduler/hourly.go`
- Test: `internal/platform/scheduler/hourly_test.go`

- [ ] **Step 1: Write failing tests** mirroring `daily_test.go`:

```go
func TestNextHourlyAt(t *testing.T) {
	loc := time.UTC
	// 12:30 → next :00 is 13:00
	from := time.Date(2026, 6, 19, 12, 30, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 19, 13, 0, 0, 0, loc), scheduler.NextHourlyAt(from, loc))
	// exactly on :00 rolls to next hour
	on := time.Date(2026, 6, 19, 13, 0, 0, 0, loc)
	require.Equal(t, time.Date(2026, 6, 19, 14, 0, 0, 0, loc), scheduler.NextHourlyAt(on, loc))
}
```

- [ ] **Step 2: Run red.** `go test ./internal/platform/scheduler/... -race` → FAIL.
- [ ] **Step 3: Implement** `internal/platform/scheduler/hourly.go`, mirroring `daily.go`:

```go
// NextHourlyAt returns the next top-of-hour strictly after `from` in loc.
func NextHourlyAt(from time.Time, loc *time.Location) time.Time {
	f := from.In(loc)
	next := time.Date(f.Year(), f.Month(), f.Day(), f.Hour(), 0, 0, 0, loc)
	if !next.After(f) {
		next = next.Add(time.Hour)
	}
	return next
}

// RunHourly blocks, firing job(ctx) at every top-of-hour local until ctx is
// cancelled. `now` is injected for testability (prod passes time.Now).
func RunHourly(ctx context.Context, loc *time.Location, now func() time.Time, job func(context.Context)) {
	for {
		next := NextHourlyAt(now(), loc)
		timer := time.NewTimer(next.Sub(now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			job(ctx)
		}
	}
}
```

- [ ] **Step 4: Run green + commit.** PASS.
`git add internal/platform/scheduler/hourly.go internal/platform/scheduler/hourly_test.go` →
`feat(scheduler): NextHourlyAt + RunHourly (top-of-hour cron loop)`

---

# GROUP H — Router wiring + guard test

### Task H1: router fields + admin mounts (19 routes)

**Files:**
- Modify: `internal/shared/router.go`
- Test: `internal/shared/router_admin4c4_test.go` (Task H2)

- [ ] **Step 1: Add 19 nil-guarded `http.Handler` fields** to the `Router` struct, after the 4c-3 fields (`AdminPaymentRefund`, ~line 179), grouped + commented like the 4c-3 block:

```go
	// ── Phase 4c-4 admin triage (19 routes) ──────────────────────────────
	AdminErrorsList     http.Handler // GET    /admin/errors
	AdminErrorGet       http.Handler // GET    /admin/errors/{id}
	AdminErrorPatch     http.Handler // PATCH  /admin/errors/{id}
	AdminErrorDelete    http.Handler // DELETE /admin/errors/{id}
	AdminErrorSuggest   http.Handler // POST   /admin/errors/{id}/suggest-fix
	AdminErrorsRunCron  http.Handler // POST   /admin/errors/run-ai-cron
	AdminBugList        http.Handler // GET    /admin/bug-reports
	AdminBugGet         http.Handler // GET    /admin/bug-reports/{id}
	AdminBugScreenshot  http.Handler // GET    /admin/bug-reports/{id}/screenshot
	AdminBugPatch       http.Handler // PATCH  /admin/bug-reports/{id}
	AdminBugDelete      http.Handler // DELETE /admin/bug-reports/{id}
	AdminBugFixProposal http.Handler // POST   /admin/bug-reports/{id}/fix-proposal
	AdminInboxCounts    http.Handler // GET    /admin/inbox/counts
	AdminInbox          http.Handler // GET    /admin/inbox
	AdminReleasesList   http.Handler // GET    /admin/releases
	AdminReleaseUpsert  http.Handler // POST   /admin/releases
	AdminReleaseDelete  http.Handler // DELETE /admin/releases/{platform}/{channel}
	AdminPolicyUpsert   http.Handler // POST   /admin/release-policies
	AdminGrantSub       http.Handler // POST   /admin/subscriptions/grant
```

- [ ] **Step 2: Mount them** inside the admin group (the `admin.Method(...)` block near line 482). **Order matters — static before wildcard (chi prioritizes static, but be explicit):**

```go
			// Phase 4c-4 — errors. Static run-ai-cron before {id} wildcards.
			admin.Method(http.MethodPost, "/admin/errors/run-ai-cron", r.AdminErrorsRunCron)
			admin.Method(http.MethodGet, "/admin/errors", r.AdminErrorsList)
			admin.Method(http.MethodGet, "/admin/errors/{id}", r.AdminErrorGet)
			admin.Method(http.MethodPatch, "/admin/errors/{id}", r.AdminErrorPatch)
			admin.Method(http.MethodDelete, "/admin/errors/{id}", r.AdminErrorDelete)
			admin.Method(http.MethodPost, "/admin/errors/{id}/suggest-fix", r.AdminErrorSuggest)
			// bug-reports
			admin.Method(http.MethodGet, "/admin/bug-reports", r.AdminBugList)
			admin.Method(http.MethodGet, "/admin/bug-reports/{id}", r.AdminBugGet)
			admin.Method(http.MethodGet, "/admin/bug-reports/{id}/screenshot", r.AdminBugScreenshot)
			admin.Method(http.MethodPatch, "/admin/bug-reports/{id}", r.AdminBugPatch)
			admin.Method(http.MethodDelete, "/admin/bug-reports/{id}", r.AdminBugDelete)
			admin.Method(http.MethodPost, "/admin/bug-reports/{id}/fix-proposal", r.AdminBugFixProposal)
			// inbox — static counts before /admin/inbox
			admin.Method(http.MethodGet, "/admin/inbox/counts", r.AdminInboxCounts)
			admin.Method(http.MethodGet, "/admin/inbox", r.AdminInbox)
			// releases
			admin.Method(http.MethodGet, "/admin/releases", r.AdminReleasesList)
			admin.Method(http.MethodPost, "/admin/releases", r.AdminReleaseUpsert)
			admin.Method(http.MethodDelete, "/admin/releases/{platform}/{channel}", r.AdminReleaseDelete)
			admin.Method(http.MethodPost, "/admin/release-policies", r.AdminPolicyUpsert)
			// grant
			admin.Method(http.MethodPost, "/admin/subscriptions/grant", r.AdminGrantSub)
```

- [ ] **Step 3: Build.** `go build ./...` → OK (handlers nil for now; guard test in H2 sets them). Commit.
`git add internal/shared/router.go` →
`feat(router): mount 19 Phase 4c-4 admin routes (errors/bug-reports/inbox/releases/grant)`

### Task H2: router auth-guard integration test (19 routes)

**Files:**
- Create: `internal/shared/router_admin4c4_test.go`

- [ ] **Step 1: Write the test** mirroring `router_admin4c3_test.go` exactly: build a `Router` with all 19 fields = `adminOKHandler(200)`, table of `{method, path}` for all 19, assert no-auth → 401 and valid non-admin JWT → 403. Use the existing `testJWTSecret`, `adminOKHandler`, `signTestToken` helpers from the 4c-3 test file (same package `shared_test`).
- [ ] **Step 2: Run.** `go test ./internal/shared/... -race -run Admin4c4` → PASS (401 + 403 for every route; a 404 would mean an unmounted route).
- [ ] **Step 3: Commit.** `git add internal/shared/router_admin4c4_test.go` →
`test(router): 4c-4 admin routes are auth+admin guarded (401/403)`

---

# GROUP I — Composition + crons + finish

### Task I1: main.go composition wiring + 2 hourly crons

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/shared/router.go` is NOT touched here (fields exist from H1)

- [ ] **Step 1: Wire the services + handlers** in the admin section of `main.go` (after the 4c-3 block, ~line 630). Build:
  - `errAdminRepo` (extend the existing errreport repo) → `errAdminSvc := errreport.NewAdminService(errAdminRepo, aiProviderSvc, time.Now)` (reuse the existing `aiProviderSvc` from 4c-2 as the `fixProposer` — it now has `Propose`). → `errCron := errreport.NewCron(errAdminSvc, errDisabled, time.Sleep)` → `errAdminHandler := errreport.NewAdminHandler(errAdminSvc, errCron)`.
  - Same for bugreports: `bugAdminSvc`, `bugCron`, `bugAdminHandler`.
  - `inboxSvc := inbox.NewService(errAdminRepo, bugAdminRepo)` → `inboxHandler`.
  - `relAdminSvc := updates.NewAdminService(updatesRepo)` (extend existing) → `relAdminHandler`.
  - `subAdminGrantHandler` (extend the existing `subAdminHandler` from 4c-3 or add a grant handler reusing `subReader`/writer).
  - Assign all 19 `core.adminXxx = http.HandlerFunc(handler.Method)`.

> `errDisabled`/`bugDisabled` close over the env toggle: `disabled := func() bool { v := cfg.DisableFixProposalCron; return v == "1" || v == "true" }` — add the env field if not present (grep `DISABLE_FIX_PROPOSAL_CRON`; add to the config struct + loader if missing). The toggle gates BOTH crons (shared).

- [ ] **Step 2: Start the 2 hourly crons** mirroring the daily-cron block (~line 612):

```go
		// Hourly AI fix-proposal crons (ports NestJS @Cron(EVERY_HOUR) ×2).
		// Shared toggle DISABLE_FIX_PROPOSAL_CRON gates both. Background
		// daemons scoped to ctx; exit with the process.
		go scheduler.RunHourly(ctx, time.Local, time.Now, func(c context.Context) {
			res := errCron.RunOnce(c)
			log.Info("error fix-proposal cron", "succeeded", res.Success, "failed", res.Failed)
		})
		go scheduler.RunHourly(ctx, time.Local, time.Now, func(c context.Context) {
			res := bugCron.RunOnce(c)
			log.Info("bug fix-proposal cron", "succeeded", res.Success, "failed", res.Failed)
		})
		log.Info("scheduler started", "job", "ai-fix-proposal", "at", "hourly local")
```

- [ ] **Step 3: Build + vet.** `go build ./... && go vet ./...` → clean.
- [ ] **Step 4: Commit.** `git add cmd/server/main.go internal/platform/config/*` (config only if the toggle field was added) →
`feat(main): wire 4c-4 admin handlers + start 2 hourly fix-proposal crons (toggle-gated)`

### Task I2: full gate + finish

**Files:** none (verification only)

- [ ] **Step 1: Run the full gate.**
  - `go build ./...` → OK
  - `go vet ./...` → clean
  - `gofmt -l .` → empty
  - `go test ./... -race` → all PASS
  - `sqlc generate` → no diff (`git status` clean for `internal/shared/pg/sqlc`)
- [ ] **Step 2: Final whole-branch review** (subagent-driven final reviewer — the skill dispatches this). Address any 🔴 parity findings before finishing.
- [ ] **Step 3:** Invoke `superpowers:finishing-a-development-branch`. Branch is NOT pushed; push/merge only when the user asks.

---

## Self-review notes (author checklist — done)

**Spec coverage:** E1–E6 → B1–B4; B1–B6 → C1–C4; I1–I2 → D1–D2; R1–R4 → E1–E3; G1 → F1–F2; crons → B3/C3 + scheduler G1 + wiring I1; shared AI path → A1–A2; router → H1–H2; gate → I2. All 19 routes + 2 crons + scheduler covered.

**Placeholder scan:** No "TBD/implement later". Two deliberate verbatim-transcription steps (B2 §3.2 prompt, C2 §3.3 prompt) point the implementer at the exact Node source files because the literals are long; the spec carries the section structure and the controller recon carries the exact bytes. Several "confirm against sqlc model / schema" steps are verification instructions, not placeholders — the parity fact (column list, nullability) must be checked against generated code, not guessed.

**Type consistency:** `ErrorReportEntity`/`BugReportEntity`/`GrantedSub`/`AppRelease`/`AppReleasePolicy`/`ReleasesView` field names + JSON tags fixed and reused across tasks. `fixProposer.Propose(ctx, system, user) (string, error)` is the single shared signature (A2) consumed by errreport (B2), bugreports (C2). `CronResult{Success, Failed}` shared shape (B3/C3). `AdminListFilter` named consistently per module. `NextHourlyAt(from, loc)` / `RunHourly(ctx, loc, now, job)` match the daily.go idiom.

**Known verification gates for the implementer (not guesses to encode blindly):** 400/404 envelope `code` strings (reuse the 4c-2/4c-3 constants), sqlc column nullability, UNIQUE constraints for ON CONFLICT (fall back to load-then-branch — which also matches Node's partial-overwrite semantics), DTO-validation humanized messages for `@IsUUID`/`@IsDateString` (reuse the 4c-2 humanize helper), and the cron sleep-only-after-success control flow (mirror `fix-proposal.cron.ts`).
