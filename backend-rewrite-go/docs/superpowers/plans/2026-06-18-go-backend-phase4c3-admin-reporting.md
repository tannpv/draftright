# Phase 4c-3 — Admin Reporting/Analytics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the 11 admin reporting/analytics endpoints (stats, analytics, transactions, training-data, payments admin) from NestJS to Go as a byte-identical drop-in.

**Architecture:** Two new flat modules — `internal/rewritelog` (training-data table) and `internal/adminstats` (no table; composes user/subscription/usage/plans read ports for stats+analytics). Extend `internal/subscription` (analytics + paginated reads), `internal/usage` (global counters), `internal/payment` (admin getStats/findAll/confirm/refund). All routes mount in the existing admin group in `internal/shared/router.go` via nil-guarded `http.Handler` fields, exactly mirroring the 25 routes added in 4c-2.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc v1.31, golang-jwt/jwt/v5 (HS256 zero tolerance), stripe-go/v82 (refund — reuse the existing SDK client pattern from `internal/payment/strategy/stripe/stripe.go`), slog.

**Spec:** `docs/superpowers/specs/2026-06-18-go-backend-phase4c3-admin-reporting-design.md`. Node is parity authority. Branch: `feature/go-backend-phase4c3-admin-reporting-20260618` (already created from develop, spec committed `2a04e1f4`).

---

## Standing rules for every task

- **TDD:** write the failing test FIRST, run it red, implement minimal, run green, commit.
- **Stage ONLY the task's files. NEVER `git add -A`.** List exact paths in the commit.
- Node is the parity authority — when in doubt, read the cited Node source and match byte-for-byte (JSON keys IN ORDER, status codes, error envelope `{error,code,request_id}`, messages).
- After each code change run `go test ./<pkg>/... -race`. Before the final task run the full gate: `go build ./...`, `go vet ./...`, `gofmt -l .` (must print nothing), `go test ./... -race`, `sqlc generate` (no drift).
- New sqlc queries: add to the relevant `internal/shared/pg/queries_*.sql`, run `sqlc generate`, commit the regenerated `internal/shared/pg/sqlc/*` together with the `.sql`.
- Consumer-side ports, small interfaces, accept interfaces / return structs. Use cases never import `pgx`/`chi`/`net/http`.

**Shared helpers (verbatim signatures):**
- `shared.WriteJSON(w http.ResponseWriter, status int, v any)` — `internal/shared/render.go`.
- `shared.WriteError(w http.ResponseWriter, r *http.Request, code, message string)` — codes: `invalid-input`→400, `not-found`→404, `forbidden`→403, `conflict`→409, default→500. `internal/shared/errenvelope.go`.
- JSON entity field-order + millisecond timestamps are pinned via a custom `MarshalJSON` — copy the pattern from `internal/plans/domain.go` `PlanEntity.MarshalJSON` for every new entity that serializes to the wire.

---

## TIMEZONE PARITY (read before Tasks 1–8)

Node builds time boundaries in the **server-local** timezone: `new Date(y, m, 1)`, `setHours(0,0,0,0)`, `setDate(1)`. The existing Go `usage.Counter.CountToday` already mirrors this with `time.Date(...now.Location())` (`internal/usage/counter.go:33`). Every boundary in this plan uses the **same convention**: derive from a `now time.Time` and use `now.Location()`. The prod Node container TZ is assumed UTC; verify before cutover (out of scope here — both Node and Go read the same wall clock, so the shadow gate catches a mismatch). Inject `now func() time.Time` (default `time.Now`) into the use cases that compute boundaries (`adminstats`, the subscription `getMonthlyStats`) for deterministic tests.

---

# GROUP 1 — usage global counters (supports route 1)

### Task 1: Global usage SQL queries + Counter methods

**Files:**
- Modify: `internal/shared/pg/queries_auth.sql` (add 2 queries near `CountUsageToday`, line ~57)
- Regenerate: `internal/shared/pg/sqlc/*` (via `sqlc generate`)
- Modify: `internal/usage/counter.go`
- Test: `internal/usage/counter_test.go`

- [ ] **Step 1: Write failing tests** in `internal/usage/counter_test.go`. Add a fake querier implementing the new methods and assert `CountTodayAll`/`CountThisMonthAll` pass the correct boundary (local midnight; local first-of-month 00:00) and return the count. Mirror the existing `CountToday` test style in that file (inject a fixed `now` if the method takes one — see Step 3; otherwise assert the boundary the fake receives).

```go
func TestCountTodayAll(t *testing.T) {
	now := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
	want := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	f := &fakeAllQ{today: 42}
	c := usage.NewCounter(f)
	got, err := c.CountTodayAllAt(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 42, got)
	require.Equal(t, want, f.gotTodayBoundary)
}

func TestCountThisMonthAll(t *testing.T) {
	now := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeAllQ{month: 99}
	c := usage.NewCounter(f)
	got, err := c.CountThisMonthAllAt(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, 99, got)
	require.Equal(t, want, f.gotMonthBoundary)
}
```

- [ ] **Step 2: Run red.** `go test ./internal/usage/... -race` → FAIL (methods undefined).

- [ ] **Step 3: Add SQL** to `internal/shared/pg/queries_auth.sql`:

```sql
-- name: CountUsageTodayAll :one
-- Global rewrite count since local midnight (Node usageService.countToday()).
SELECT COUNT(*) FROM usage_logs WHERE created_at >= $1;

-- name: CountUsageThisMonthAll :one
-- Global rewrite count since local first-of-month (Node usageService.countThisMonth()).
SELECT COUNT(*) FROM usage_logs WHERE created_at >= $1;
```

Run `sqlc generate`. Extend the `usage.Querier` interface in `internal/usage/counter.go` with:

```go
CountUsageTodayAll(ctx context.Context, createdAt pgtype.Timestamp) (int64, error)
CountUsageThisMonthAll(ctx context.Context, createdAt pgtype.Timestamp) (int64, error)
```

Add the methods (boundary helpers mirror `CountToday`):

```go
// CountTodayAllAt mirrors Node usageService.countToday(): all users' rewrites
// since local midnight of `now`.
func (c *Counter) CountTodayAllAt(ctx context.Context, now time.Time) (int, error) {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n, err := c.q.CountUsageTodayAll(ctx, pgtype.Timestamp{Time: midnight, Valid: true})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// CountThisMonthAllAt mirrors Node usageService.countThisMonth(): all users'
// rewrites since local first-of-month of `now`.
func (c *Counter) CountThisMonthAllAt(ctx context.Context, now time.Time) (int, error) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	n, err := c.q.CountUsageThisMonthAll(ctx, pgtype.Timestamp{Time: monthStart, Valid: true})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
```

- [ ] **Step 4: Run green.** `go test ./internal/usage/... -race` → PASS.
- [ ] **Step 5: Commit.** `git add internal/shared/pg/queries_auth.sql internal/shared/pg/sqlc internal/usage/counter.go internal/usage/counter_test.go` →
`feat(usage): global CountTodayAll/CountThisMonthAll for admin stats (Node parity)`

---

# GROUP 2 — subscription analytics (supports routes 2, 3, 11)

> Add these to the subscription read path. `CancelByStoreRef` ALREADY EXISTS on `*WebhookWriter` (`internal/subscription/webhook_writer.go:87`) — REUSE it for refund, do not duplicate. The new analytics reads belong on `*Reader` (`internal/subscription/reader.go`). Add a new sqlc query file `internal/shared/pg/queries_subscription.sql` if none exists (check first; if subscription queries live in `queries_auth.sql`, add there).

### Task 2: `CountActive` + `GetPlansBreakdown`

**Files:**
- Modify: subscription queries `.sql` + regenerate sqlc
- Modify: `internal/subscription/reader.go`
- Test: `internal/subscription/reader_test.go`

- [ ] **Step 1: Failing tests.** Fake querier returns canned rows; assert:
  - `CountActive` returns the int from `CountActiveSubscriptions`.
  - `GetPlansBreakdown` maps rows to `[]PlanBreakdown{PlanName, ActiveCount, PriceCents}` in row order; empty → non-nil empty slice.

```go
func TestGetPlansBreakdown(t *testing.T) {
	f := &fakeAnalyticsQ{breakdown: []sqlc.PlansBreakdownRow{
		{PlanName: "Pro", ActiveCount: 3, PriceCents: 999},
	}}
	r := subscription.NewReader(f)
	got, err := r.GetPlansBreakdown(context.Background())
	require.NoError(t, err)
	require.Equal(t, []subscription.PlanBreakdown{{PlanName: "Pro", ActiveCount: 3, PriceCents: 999}}, got)
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: SQL + types.** Add:

```sql
-- name: CountActiveSubscriptions :one
SELECT COUNT(*) FROM subscriptions WHERE status = 'active';

-- name: PlansBreakdown :many
-- Node getPlansBreakdown: active subs grouped by plan.
SELECT p.name AS plan_name, p.price_cents AS price_cents, COUNT(*) AS active_count
FROM subscriptions s
LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.status = 'active'
GROUP BY p.name, p.price_cents;
```

`sqlc generate`. Add to `reader.go`:

```go
// PlanBreakdown is one row of GET /admin/analytics plans_breakdown.
// JSON key order (analytics serializes via the handler): plan_name, active_count, price_cents.
type PlanBreakdown struct {
	PlanName    string `json:"plan_name"`
	ActiveCount int    `json:"active_count"`
	PriceCents  int    `json:"price_cents"`
}

func (r *Reader) CountActive(ctx context.Context) (int, error) {
	n, err := r.q.CountActiveSubscriptions(ctx)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *Reader) GetPlansBreakdown(ctx context.Context) ([]PlanBreakdown, error) {
	rows, err := r.q.PlansBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanBreakdown, 0, len(rows))
	for _, row := range rows {
		out = append(out, PlanBreakdown{
			PlanName:    derefName(row.PlanName), // plan name may be NULL on a dangling join → ""
			ActiveCount: int(row.ActiveCount),
			PriceCents:  int(row.PriceCents),
		})
	}
	return out, nil
}
```

Add `CountActiveSubscriptions` + `PlansBreakdown` to the `Querier` interface. Handle the nullable `plan_name`/`price_cents` from the LEFT JOIN using whatever pgtype the generated row uses (e.g. `pgtype.Text`/`*string`); `NULL` name → `""`, `NULL` price → `0`. Match Node: `r.plan_name`, `parseInt(r.active_count)`, `parseInt(r.price_cents) || 0`.

- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit.** Stage the `.sql`, sqlc, reader.go, reader_test.go →
`feat(subscription): CountActive + GetPlansBreakdown for admin analytics`

### Task 3: `GetMonthlyStats(months)`

**Files:** subscription `.sql` + sqlc, `internal/subscription/reader.go`, `reader_test.go`

- [ ] **Step 1: Failing test.** `GetMonthlyStatsAt(ctx, now, 12)` returns 12 entries oldest→newest; each `{Month "YYYY-MM", NewSubscriptions, RevenueCents, Churned}`. Inject `now` for determinism. Fake querier returns canned per-window values; assert window count = 12 and `Month` strings (e.g. for `now=2026-06-18`, last entry `"2026-06"`, first `"2025-07"`).

```go
func TestGetMonthlyStats_WindowsAndOrder(t *testing.T) {
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	f := &fakeMonthlyQ{} // returns newSubs=1, revenue=500, churned=0 for any window
	r := subscription.NewReader(f)
	got, err := r.GetMonthlyStatsAt(context.Background(), now, 12)
	require.NoError(t, err)
	require.Len(t, got, 12)
	require.Equal(t, "2025-07", got[0].Month)
	require.Equal(t, "2026-06", got[11].Month)
	require.Equal(t, subscription.MonthStat{Month: "2026-06", NewSubscriptions: 1, RevenueCents: 500, Churned: 0}, got[11])
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: SQL + impl.** Three windowed queries (parameters: monthStart, monthEnd):

```sql
-- name: CountNewSubsInWindow :one
SELECT COUNT(*) FROM subscriptions WHERE started_at >= $1 AND started_at < $2;

-- name: SumRevenueInWindow :one
SELECT COALESCE(SUM(p.price_cents), 0)::bigint AS total
FROM subscriptions s LEFT JOIN plans p ON p.id = s.plan_id
WHERE s.started_at >= $1 AND s.started_at < $2;

-- name: CountChurnedInWindow :one
SELECT COUNT(*) FROM subscriptions
WHERE status IN ('cancelled','expired') AND updated_at >= $1 AND updated_at < $2;
```

`sqlc generate`. Add to `reader.go`:

```go
// MonthStat is one row of GET /admin/analytics monthly_stats.
type MonthStat struct {
	Month            string `json:"month"`
	NewSubscriptions int    `json:"new_subscriptions"`
	RevenueCents     int    `json:"revenue_cents"`
	Churned          int    `json:"churned"`
}

// GetMonthlyStatsAt mirrors Node getMonthlyStats: `months` windows oldest→newest,
// boundaries in now's timezone (new Date(y, m, 1)).
func (r *Reader) GetMonthlyStatsAt(ctx context.Context, now time.Time, months int) ([]MonthStat, error) {
	out := make([]MonthStat, 0, months)
	for i := months - 1; i >= 0; i-- {
		mStart := time.Date(now.Year(), int(now.Month())-i, 1, 0, 0, 0, 0, now.Location())
		mEnd := time.Date(mStart.Year(), int(mStart.Month())+1, 1, 0, 0, 0, 0, now.Location())
		monthStr := mStart.Format("2006-01")
		// ... call the 3 queries with pgtype.Timestamp{mStart}, {mEnd} ...
	}
	return out, nil
}
```

Note: `time.Date` normalizes overflow months (e.g. `Month(-2)` → prior year) exactly like Node's `new Date(year, month-i, 1)`. `Format("2006-01")` yields `YYYY-MM` matching Node's `${y}-${String(m+1).padStart(2,'0')}`.

- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(subscription): GetMonthlyStats (12-month windows) for admin analytics`

### Task 4: `FindAllPaginated` (transactions) + `FindLatestStripeForUserPlan` (refund)

**Files:** subscription `.sql` + sqlc, `internal/subscription/reader.go`, `reader_test.go`

- [ ] **Step 1: Failing tests.**
  - `FindAllPaginated({search,page,limit})`: limit default 20; returns `{Subscriptions []TransactionRow, Total int}`; with search, ILIKE over user.email/name. Assert mapping incl. nil-relation `"—"` fallbacks and `price_cents` 0 fallback.
  - `FindLatestStripeForUserPlan(userID, planID)` returns the newest stripe sub's `store_transaction_id` (or nil when none).

```go
func TestFindAllPaginated_MapsAndFallbacks(t *testing.T) {
	f := &fakeTxQ{rows: []sqlc.ListSubscriptionsPaginatedRow{ /* one row, NULL user+plan */ }, total: 1}
	r := subscription.NewReader(f)
	res, err := r.FindAllPaginated(context.Background(), subscription.TxQuery{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.Equal(t, 1, res.Total)
	require.Equal(t, "—", res.Transactions[0].UserEmail)
	require.Equal(t, 0, res.Transactions[0].PriceCents)
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: SQL + impl.** Two-query getManyAndCount + the lookup:

```sql
-- name: ListSubscriptionsPaginated :many
SELECT s.id, u.email AS user_email, u.name AS user_name, s.user_id,
       p.name AS plan_name, p.price_cents AS price_cents,
       s.store_type, s.store_transaction_id, s.status,
       s.started_at, s.expires_at, s.created_at
FROM subscriptions s
LEFT JOIN users u ON u.id = s.user_id
LEFT JOIN plans p ON p.id = s.plan_id
WHERE (sqlc.narg('search')::text IS NULL
       OR u.email ILIKE '%' || sqlc.narg('search') || '%'
       OR u.name  ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY s.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSubscriptionsPaginated :one
SELECT COUNT(*) FROM subscriptions s
LEFT JOIN users u ON u.id = s.user_id
WHERE (sqlc.narg('search')::text IS NULL
       OR u.email ILIKE '%' || sqlc.narg('search') || '%'
       OR u.name  ILIKE '%' || sqlc.narg('search') || '%');

-- name: FindLatestStripeForUserPlan :one
SELECT id, store_transaction_id FROM subscriptions
WHERE user_id = $1 AND plan_id = $2 AND store_type = 'stripe'
ORDER BY created_at DESC LIMIT 1;
```

Note: Node passes `search` only when truthy and uses `%term%`. With sqlc `narg`, pass `nil` when search is empty (Node: no WHERE) — so when `search==""` send NULL → the `IS NULL` branch matches all. This is parity-equivalent to Node omitting the clause.

Add `TxQuery{Search string; Page, Limit int}`, `TransactionRow` (12 fields, json tags in the spec §4 order), `Paginated[TransactionRow]`-style return `{Transactions []TransactionRow; Total int}`. Offset = `(page-1)*limit`. Map NULL email/name/plan_name → `"—"` (U+2014), NULL price_cents → 0. `FindLatestStripeForUserPlan` returns `(storeTxID string, found bool, err error)`; `pgx.ErrNoRows` → `found=false`.

- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(subscription): FindAllPaginated (transactions) + FindLatestStripeForUserPlan`

---

# GROUP 3 — rewritelog module (routes 4–7, NEW flat module)

### Task 5: `internal/rewritelog/domain.go` — RewriteLog entity

**Files:**
- Create: `internal/rewritelog/domain.go`
- Test: `internal/rewritelog/domain_test.go`

- [ ] **Step 1: Failing test.** Assert `json.Marshal(RewriteLog{...})` produces keys in entity order `id,tone,input_text,output_text,model,provider_type,response_time_ms,quality,created_at` with a millisecond-precision ISO timestamp for `created_at` (match Node TypeORM Date serialization). Copy the timestamp-format approach from `internal/plans/domain.go` `PlanEntity.MarshalJSON`.

```go
func TestRewriteLog_JSONFieldOrder(t *testing.T) {
	l := rewritelog.RewriteLog{
		ID: "id1", Tone: "polished", InputText: "in", OutputText: "out",
		Model: "llama3.2", ProviderType: "ollama", ResponseTimeMs: 12,
		Quality: "pending", CreatedAt: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	}
	b, _ := json.Marshal(l)
	require.Equal(t, `{"id":"id1","tone":"polished","input_text":"in","output_text":"out","model":"llama3.2","provider_type":"ollama","response_time_ms":12,"quality":"pending","created_at":"2026-06-18T12:00:00.000Z"}`, string(b))
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `RewriteLog` with explicit json tags + a `MarshalJSON` pinning order and ms-ISO timestamp (mirror `PlanEntity.MarshalJSON`). Verify the exact created_at format against a live Node `GET /admin/training-data` row during shadow; the `.000Z` ms form is the TypeORM default.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(rewritelog): RewriteLog domain entity with pinned JSON order`

### Task 6: `internal/rewritelog/repo_pg.go` + queries

**Files:**
- Create: `internal/shared/pg/queries_rewritelog.sql`, regenerate sqlc
- Create: `internal/rewritelog/repo_pg.go`
- Test: `internal/rewritelog/repo_pg_test.go` (compile-only / fake-querier mapping tests — no DB)

- [ ] **Step 1: Failing test.** Fake sqlc querier; assert repo maps rows → `[]RewriteLog`, `CountByQuality` buckets a grouped result into `(pending,approved,rejected)` with 0 defaults, `FindPending` returns `(logs, total)` and computes offset `(page-1)*limit`.

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: SQL:**

```sql
-- name: CountRewriteLogs :one
SELECT COUNT(*) FROM rewrite_logs;

-- name: CountRewriteLogsByQuality :many
SELECT quality, COUNT(*) AS n FROM rewrite_logs GROUP BY quality;

-- name: ListPendingRewriteLogs :many
SELECT id, tone, input_text, output_text, model, provider_type, response_time_ms, quality, created_at
FROM rewrite_logs WHERE quality = 'pending'
ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountPendingRewriteLogs :one
SELECT COUNT(*) FROM rewrite_logs WHERE quality = 'pending';

-- name: UpdateRewriteLogQuality :exec
UPDATE rewrite_logs SET quality = $2 WHERE id = $1;

-- name: ListApprovedRewriteLogsAsc :many
SELECT id, tone, input_text, output_text, model, provider_type, response_time_ms, quality, created_at
FROM rewrite_logs WHERE quality = 'approved' ORDER BY created_at ASC;
```

`sqlc generate`. Implement `PgRepo` (struct `{q *sqlc.Queries}`, `NewPgRepo`), methods `Count`, `CountByQuality` (map grouped rows → 3 ints), `FindPending(page,limit)`, `UpdateQuality(id,quality)`, `FindApprovedAsc`. `id` is uuid — accept string, `pgtype.UUID.Scan`; bad id in `UpdateQuality` → return nil (no-op, matching Node `update(id,...)` which silently affects 0 rows). Mirror the sqlc-call idiom from `internal/appsettings/repo_pg.go`.

- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(rewritelog): pg repo + queries (count/quality/pending/update/approved)`

### Task 7: `internal/rewritelog/usecase.go`

**Files:** Create `internal/rewritelog/usecase.go`, `usecase_test.go`

- [ ] **Step 1: Failing test.** Fake repo; assert:
  - `Stats(ctx)` → `StatsResult{Total, Pending, Approved, Rejected}`.
  - `ListPending(page,limit)` → `{Logs, Total}`.
  - `Review(id, quality)` calls repo.UpdateQuality (no validation of the quality value — Node doesn't validate).
  - `ExportJSONL(ctx)` returns the exact JSONL string for approved logs (system-prompt template verbatim), lines joined by `\n`, empty dataset → `""`.

```go
func TestExportJSONL_Format(t *testing.T) {
	f := &fakeRepo{approved: []rewritelog.RewriteLog{
		{Tone: "polished", InputText: "hi", OutputText: "Hello."},
	}}
	s := rewritelog.NewService(f)
	out, err := s.ExportJSONL(context.Background())
	require.NoError(t, err)
	require.Equal(t, `{"messages":[{"role":"system","content":"Rewrite the following text in a polished tone. Return only the rewritten text."},{"role":"user","content":"hi"},{"role":"assistant","content":"Hello."}]}`, out)
}

func TestExportJSONL_Empty(t *testing.T) {
	s := rewritelog.NewService(&fakeRepo{})
	out, _ := s.ExportJSONL(context.Background())
	require.Equal(t, "", out)
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `Service` + consumer `Repo` port (the 5 repo methods), `NewService`. `Stats` returns `StatsResult{Total,Pending,Approved,Rejected int}`. `ExportJSONL`: build each line with `encoding/json` Marshal of an ordered struct `{Messages []msg}` where `msg{Role,Content string}` json `role,content`; system content = `fmt.Sprintf("Rewrite the following text in a %s tone. Return only the rewritten text.", log.Tone)`; `strings.Join(lines, "\n")`. Verify Marshal does not HTML-escape `<`/`>`/`&` differently from Node `JSON.stringify` — use a `json.Encoder` with `SetEscapeHTML(false)` (Node does NOT escape these). Add a test with input containing `<` & `&` asserting no `<`.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(rewritelog): service (stats/listPending/review/exportJSONL)`

### Task 8: `internal/rewritelog/handler.go` — routes 4–7

**Files:** Create `internal/rewritelog/handler.go`, `handler_test.go`

- [ ] **Step 1: Failing tests** (httptest, fake service via consumer port):
  - `Stats` GET → 200 body `{"total":N,"pending":N,"approved":N,"rejected":N}` (assert key order with a raw-bytes check helper like 4c-2's `assertKeyOrder`).
  - `List` GET → 200 `{"logs":[...],"total":N}`; empty logs → `[]` not null.
  - `Review` PATCH (chi `{id}`) body `{"quality":"approved"}` → 200 `{"success":true}`. NO validation — any body that has a `quality` string is accepted; missing/extra keys ignored. Malformed JSON → mirror Node: `body.quality` would be undefined → still calls update with empty → returns `{success:true}` (assert 200; confirm against Node — Node has no DTO, `@Body() body` is untyped, a non-JSON body throws body-parser 400. Match Node: decode error → 400 `invalid-input` "Invalid request body"; valid JSON missing quality → passes `""`/undefined through → `{success:true}`).
  - `Export` GET → 200, headers `Content-Type: application/jsonl` and `Content-Disposition: attachment; filename=draftright-training-data.jsonl`, body = raw JSONL (NOT via WriteJSON). Empty dataset → 200, zero-length body.

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `Handler{svc}` + `NewHandler`, consumer-side `service` interface (4 methods). `Stats`/`List` use `shared.WriteJSON`. `List` parses `page`/`limit` via `strconv.Atoi` fallback (1 / 20). `Review` reads chi `{id}`, decodes `{quality string}` (decode error → 400 `invalid-input` "Invalid request body"), calls `svc.Review`, writes `{"success":true}`. `Export` sets the two headers then `w.Write([]byte(jsonl))` (200 default). Define a small ordered response struct for stats `{Total,Pending,Approved,Rejected int}` json `total,pending,approved,rejected`.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(rewritelog): handlers for training-data stats/list/review/export`

---

# GROUP 4 — adminstats module (routes 1–2, NEW flat module)

### Task 9: `internal/adminstats/usecase.go` — Stats (route 1)

**Files:** Create `internal/adminstats/usecase.go`, `usecase_test.go`

- [ ] **Step 1: Failing test.** Fakes for the 4 ports; `Stats(ctx, now)` → `StatsResult{TotalUsers, ActiveSubscriptions, RewritesToday, RewritesThisMonth int}`. Assert it calls userCount, sub.CountActive, usage.CountTodayAllAt(now), usage.CountThisMonthAllAt(now).

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement.** Define consumer ports (small):

```go
type userCounter interface { Count(ctx context.Context) (int, error) }
type subStatsPort interface {
	CountActive(ctx context.Context) (int, error)
	GetPlansBreakdown(ctx context.Context) ([]subscription.PlanBreakdown, error)
	GetMonthlyStatsAt(ctx context.Context, now time.Time, months int) ([]subscription.MonthStat, error)
}
type usageGlobal interface {
	CountTodayAllAt(ctx context.Context, now time.Time) (int, error)
	CountThisMonthAllAt(ctx context.Context, now time.Time) (int, error)
}
type planLister interface { ListAll(ctx context.Context) ([]plans.PlanEntity, error) }
```

(Importing `subscription`/`plans` exported TYPES across modules is allowed by CLAUDE.md rule 2.) `Service{users, subs, usage, plans, now func() time.Time}`, `NewService(...)`. `Stats` runs the 4 reads (sequential, order irrelevant). **Check the Go `user` module for an existing global `Count` method** (4c-2 stats used `usersService.count`); if `*user.AdminRepo` or a reader already exposes a total-user count, reuse it via the port — do not add a duplicate. If none exists, add `CountUsers` (a `SELECT COUNT(*) FROM users`) to the user module in this task.

- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(adminstats): Stats usecase (4 counts) for GET /admin/stats`

### Task 10: `internal/adminstats/usecase.go` — Analytics + MRR (route 2)

**Files:** Modify `internal/adminstats/usecase.go`, `usecase_test.go`

- [ ] **Step 1: Failing tests** for `Analytics(ctx, now)` → `AnalyticsResult{MRR, TotalRevenue int; PlansBreakdown []PlanBreakdown; MonthlyStats []MonthStat}`:
  - MRR: monthly plan → `active_count * price_cents`; yearly → `active_count * round(price_cents/12)`; `price_cents <= 0` → skip; plan matched to breakdown by `Name == plan_name`.
  - `total_revenue` = sum of monthly_stats revenue.
  - Assert a mixed case: Pro monthly 999×2 + Pro-Year yearly 9990×1 (`round(9990/12)=833`) → mrr = 1998 + 833 = 2831.

```go
func TestAnalytics_MRR(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	subs := &fakeSubStats{
		breakdown: []subscription.PlanBreakdown{
			{PlanName: "Pro", ActiveCount: 2, PriceCents: 999},
			{PlanName: "ProYear", ActiveCount: 1, PriceCents: 9990},
		},
		monthly: []subscription.MonthStat{{Month: "2026-06", RevenueCents: 5000}},
	}
	pl := &fakePlans{all: []plans.PlanEntity{
		{Name: "Pro", PriceCents: 999, BillingPeriod: "monthly"},
		{Name: "ProYear", PriceCents: 9990, BillingPeriod: "yearly"},
	}}
	s := adminstats.NewService(nil, subs, nil, pl, func() time.Time { return now })
	res, err := s.Analytics(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2831, res.MRR)
	require.Equal(t, 5000, res.TotalRevenue)
}
```

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement.** `round(price/12)` = `int(math.Round(float64(price)/12.0))`. Loop breakdown, find plan by name, branch on `BillingPeriod` (`"monthly"`/`"yearly"`). `monthly_stats` from `subs.GetMonthlyStatsAt(now, 12)`. Result struct json keys: `mrr,total_revenue,plans_breakdown,monthly_stats`.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(adminstats): Analytics + MRR for GET /admin/analytics`

### Task 11: `internal/adminstats/handler.go` — routes 1, 2

**Files:** Create `internal/adminstats/handler.go`, `handler_test.go`

- [ ] **Step 1: Failing tests.** `Stats` GET → 200 `{"total_users":..,"active_subscriptions":..,"rewrites_today":..,"rewrites_this_month":..}` (key order). `Analytics` GET → 200 `{"mrr":..,"total_revenue":..,"plans_breakdown":[...],"monthly_stats":[...]}`; empty breakdown/monthly → `[]` not null.
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `Handler{svc}` + consumer port (2 methods), ordered response structs with the exact json tags, nil slices → empty. Repo/usecase error → `shared.WriteError(w,r,"internal","stats failed")` / `"analytics failed"`.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(adminstats): handlers for GET /admin/stats + /admin/analytics`

---

# GROUP 5 — transactions handler (route 3)

### Task 12: transactions handler

> Belongs with the subscription read added in Task 4. Put the handler in a new `internal/subscription/admin_handler.go` (the subscription module owns the data) OR in `adminstats` — choose `internal/subscription/admin_handler.go` to keep data + transport together. Confirm no existing subscription admin handler; if one exists, extend it.

**Files:** Create `internal/subscription/admin_handler.go`, `admin_handler_test.go`

- [ ] **Step 1: Failing test.** GET `/admin/transactions?page=&limit=&search=` → 200 `{"transactions":[...],"total":N}`. Bespoke parse: page default 1, limit default 20 (NOT listquery). Assert mapping order matches spec §4 (id,user_email,user_name,user_id,plan_name,price_cents,store_type,store_transaction_id,status,started_at,expires_at,created_at) and `"—"` fallbacks. Empty → `[]`.
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `AdminHandler{svc}` + consumer port `List(ctx, TxQuery) (TxPage, error)`. Parse `page`/`limit` via `strconv.Atoi` fallback. `TransactionRow` json field order pinned (MarshalJSON or struct tags + ms timestamps for started_at/expires_at/created_at — expires_at nullable → `*time.Time`/`null`). Response `{transactions, total}`.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(subscription): GET /admin/transactions handler (bespoke parse, limit 20)`

---

# GROUP 6 — payment admin (routes 8–11)

> Add admin reads/writes to `internal/payment`. New sqlc queries in `internal/shared/pg/queries_payment.sql`. The Payment entity wire shape for route 9 must match Node's serialized Payment (with `user`+`plan` relations) — copy the existing payment row→JSON projection if `internal/payment` already has one; otherwise define `AdminPaymentRow` with json tags verified against a live Node `GET /admin/payments` response in shadow.

### Task 13: payment `GetStats` (route 8)

**Files:** `internal/shared/pg/queries_payment.sql` + sqlc, `internal/payment/reader.go` (or a new `admin_reader.go`), `internal/payment/usecase.go`, tests

- [ ] **Step 1: Failing test.** `svc.GetStats(ctx)` → `{Total,Completed,Pending,Revenue int}`; revenue = SUM(amount) where completed.
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: SQL** (one grouped query or four — match Node's 4 reads):

```sql
-- name: PaymentStats :one
SELECT
  COUNT(*) AS total,
  COUNT(*) FILTER (WHERE status = 'completed') AS completed,
  COUNT(*) FILTER (WHERE status = 'pending') AS pending,
  COALESCE(SUM(amount) FILTER (WHERE status = 'completed'), 0)::bigint AS revenue
FROM payments;
```

`sqlc generate`. Add `GetStats` to the repo + a service method returning `PaymentStats{Total,Completed,Pending,Revenue int}` (json `total,completed,pending,revenue`).
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): admin GetStats (total/completed/pending/revenue)`

### Task 14: payment `FindAll` paginated (route 9)

**Files:** `queries_payment.sql` + sqlc, payment repo, usecase, tests

- [ ] **Step 1: Failing tests.** `svc.FindAll({page,limit,status,search,sortBy,sortOrder})` → `{Payments, Total}`; default limit 20; sortMap of 6 fields with `payment.created_at` default; ILIKE over (reference_code,user.email,user.name,plan.name,method); status filter optional; sort_order ASC iff "ASC". Because sort field is dynamic, this read uses **pgxpool directly** (like listquery's Build), NOT a static sqlc query — mirror `internal/shared/listquery` Build's injection-safe allow-list approach. Assert the sort allow-list rejects an unknown `sort_by` → falls back to created_at.
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** a dynamic query builder in the payment repo: base SELECT with LEFT JOIN users + plans; append `status` filter, `search` ILIKE, `ORDER BY <allow-listed col> <ASC|DESC>`, LIMIT/OFFSET; run via `pool.Query`. sortMap literal: `{reference_code:"p.reference_code", amount:"p.amount", method:"p.method", status:"p.status", created_at:"p.created_at", "user.email":"u.email"}`, default `p.created_at`. Response `{payments:[AdminPaymentRow], total}`. `AdminPaymentRow` field order = Node Payment entity serialization (verify in shadow).
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): admin FindAll paginated (6-field sortMap, ILIKE search)`

### Task 15: payment `AdminConfirm` (route 10)

**Files:** `queries_payment.sql` + sqlc, payment repo, usecase, tests

> Reuse the webhook subscription-activation path. The webhook completes a payment + grants/extends a sub via `SubsWriter` (`internal/payment/usecase.go` WebhookRepo.MarkPaymentCompleted + subsWriter.Grant/ExtendByStoreRef). `adminConfirm` must produce the SAME subscription side effects as a webhook completion. Find the internal activation helper invoked by `HandleWebhook` and call it from `AdminConfirm` — do NOT fork activation logic. If the activation is inlined in the webhook handler, extract it into a private `activate(ctx, payment)` reused by both.

- [ ] **Step 1: Failing tests.** `AdminConfirm(id, notes)`:
  - payment not found → error mapped to 404 `not-found` "Payment not found" at the handler.
  - status != pending → 400 `invalid-input` "Payment is not pending".
  - happy: status→completed, completed_at set, notes = `notes || "Manually confirmed by admin"`, activation called once; returns updated payment.
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement.** Add repo methods: load payment+plan by id, update (status/completed_at/notes). Sentinel errors `ErrPaymentNotFound`, `ErrPaymentNotPending` for the handler to map. Call the shared activation. Return the payment.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): admin AdminConfirm (→completed + activate subscription)`

### Task 16: `stripeRefunder` port + stripe-go SDK impl

**Files:** Create `internal/payment/refund_stripe.go`, `refund_stripe_test.go`

> Go already depends on **stripe-go/v82** (`internal/payment/strategy/stripe/stripe.go`). Reuse the SDK — do NOT write a raw HTTP client. Build a thin port + a real impl that constructs an SDK client from the secret key.

- [ ] **Step 1: Failing test** for the port shape + a fake (the real SDK impl is verified manually in shadow, not unit-tested):

```go
type stripeRefunder interface {
	// LatestInvoiceCharge retrieves the subscription (expand latest_invoice) and
	// returns the charge id on its latest invoice ("" when none).
	LatestInvoiceCharge(ctx context.Context, secretKey, subID string) (chargeID string, err error)
	CreateRefund(ctx context.Context, secretKey, chargeID, reason string) (refundID string, err error)
	CancelSubscription(ctx context.Context, secretKey, subID string) error
}
```

Test only asserts a fake satisfies it and the usecase (Task 17) calls it in order. The real `stripeSDKRefunder` (using `client.New(secretKey)` / `sc.Subscriptions.Get(subID, &params{Expand:["latest_invoice"]})`, `sc.Refunds.New`, `sc.Subscriptions.Cancel`) has a build-only smoke test (constructs it, no network).

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** the interface + `stripeSDKRefunder` real impl mirroring the SDK usage in `strategy/stripe/stripe.go` (same import, same client construction). Charge extraction: `latest_invoice.charge`, fallback `latest_invoice.payments.data[0].payment.charge` (best-effort; "" when absent → skip refund, still cancel sub). Cancel failure is logged, not fatal.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): stripeRefunder port + stripe-go SDK impl`

### Task 17: payment `Refund` usecase (route 11)

**Files:** payment repo (load/update + secret-key read), `internal/payment/usecase.go` (or `refund.go`), tests

- [ ] **Step 1: Failing tests** with a fake `stripeRefunder` + fake subscription reader/canceller:
  - not found → 404 `not-found` "Payment not found".
  - already `refunded` → returns the row, no Stripe calls, 201 no-op.
  - status != completed → 400 "Only completed payments can be refunded".
  - method != stripe → 400 ``Refund not supported for method '<method>'``.
  - no secret key (settings.stripe_secret_key empty AND env empty) → 400 "Stripe secret_key is not configured".
  - happy: finds stripe sub via `FindLatestStripeForUserPlan`; LatestInvoiceCharge→CreateRefund→CancelSubscription called; status→refunded; notes appended (`Refunded by admin( (<reason>))?( — Stripe refund <id>)?` joined with `" | "`, empties filtered); `CancelByStripeSubId` (the existing `WebhookWriter.CancelByStoreRef("stripe", subID)`) called; returns payment.

- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement.** Guards in the exact order above (sentinel errors for the handler). Secret key = `settings.stripe_secret_key || env STRIPE_SECRET_KEY` (read via the existing settings reader + config). Note string built like Node's `[notes, "Refunded by admin..."].filter(Boolean).join(" | ")`. When no stripe sub linked → skip Stripe calls, still mark refunded (Node logs a warning and proceeds). Inject the `stripeRefunder`, the sub-lookup port (`FindLatestStripeForUserPlan`), and the sub-canceller port (`CancelByStoreRef`).
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): admin Refund (Stripe SDK refund + cancel sub)`

### Task 18: payment admin handler (routes 8–11)

**Files:** Create `internal/payment/admin_handler.go`, `admin_handler_test.go`

- [ ] **Step 1: Failing tests** (httptest, fake admin service):
  - `Stats` GET → 200 `{total,completed,pending,revenue}`.
  - `FindAll` GET → 200 `{payments,total}`; bespoke parse (page1/limit20/status/search/sort_by/sort_order).
  - `Confirm` POST `{notes?}` → **201** updated payment; not-found→404 `not-found`; not-pending→400 `invalid-input`.
  - `Refund` POST `{reason?}` → **201** payment; all guard errors mapped (404/400 with exact messages).
- [ ] **Step 2: Run red** → FAIL.
- [ ] **Step 3: Implement** `AdminHandler{svc}` + consumer port (4 methods). Confirm/Refund decode optional body (empty body OK), read chi `{id}`, map sentinel errors → `shared.WriteError`, success → `shared.WriteJSON(w, http.StatusCreated, payment)`. Stats/FindAll → 200.
- [ ] **Step 4: Run green** → PASS.
- [ ] **Step 5: Commit** → `feat(payment): admin handlers (stats/findAll/confirm/refund)`

---

# GROUP 7 — router wiring + integration test

### Task 19: Router fields + mount (11 routes)

**Files:** Modify `internal/shared/router.go`

- [ ] **Step 1:** Add 11 `http.Handler` fields to `Router` (after the 4c-2 block), commented with method/path:

```go
// Phase 4c-3 admin reporting. All http.Handler, nil-guarded.
AdminStats        http.Handler // GET  /admin/stats
AdminAnalytics    http.Handler // GET  /admin/analytics
AdminTransactions http.Handler // GET  /admin/transactions
TrainingDataStats   http.Handler // GET   /admin/training-data/stats
TrainingDataList    http.Handler // GET   /admin/training-data
TrainingDataReview  http.Handler // PATCH /admin/training-data/{id}
TrainingDataExport  http.Handler // GET   /admin/training-data/export
AdminPaymentsStats  http.Handler // GET  /admin/payments/stats
AdminPaymentsList   http.Handler // GET  /admin/payments
AdminPaymentConfirm http.Handler // POST /admin/payments/{id}/confirm
AdminPaymentRefund  http.Handler // POST /admin/payments/{id}/refund
```

- [ ] **Step 2:** Mount inside the existing admin group (after the 4c-2 mounts), nil-guarded, same `admin.Method(...)` pattern. **Order the static `/admin/training-data/stats` and `/admin/training-data/export` BEFORE `/admin/training-data/{id}`** is not required (chi prioritizes static over wildcard), but mount `/admin/payments/stats` and `/admin/payments` as distinct paths. Use chi `{id}` for the two payment POSTs and the training-data PATCH.

```go
if r.AdminStats != nil { admin.Method(http.MethodGet, "/admin/stats", r.AdminStats) }
if r.AdminAnalytics != nil { admin.Method(http.MethodGet, "/admin/analytics", r.AdminAnalytics) }
if r.AdminTransactions != nil { admin.Method(http.MethodGet, "/admin/transactions", r.AdminTransactions) }
if r.TrainingDataStats != nil { admin.Method(http.MethodGet, "/admin/training-data/stats", r.TrainingDataStats) }
if r.TrainingDataExport != nil { admin.Method(http.MethodGet, "/admin/training-data/export", r.TrainingDataExport) }
if r.TrainingDataList != nil { admin.Method(http.MethodGet, "/admin/training-data", r.TrainingDataList) }
if r.TrainingDataReview != nil { admin.Method(http.MethodPatch, "/admin/training-data/{id}", r.TrainingDataReview) }
if r.AdminPaymentsStats != nil { admin.Method(http.MethodGet, "/admin/payments/stats", r.AdminPaymentsStats) }
if r.AdminPaymentsList != nil { admin.Method(http.MethodGet, "/admin/payments", r.AdminPaymentsList) }
if r.AdminPaymentConfirm != nil { admin.Method(http.MethodPost, "/admin/payments/{id}/confirm", r.AdminPaymentConfirm) }
if r.AdminPaymentRefund != nil { admin.Method(http.MethodPost, "/admin/payments/{id}/refund", r.AdminPaymentRefund) }
```

- [ ] **Step 3:** `go build ./...` → OK.
- [ ] **Step 4: Commit** `git add internal/shared/router.go` → `feat(server): add 11 Phase 4c-3 reporting route fields + mounts`

### Task 20: Router auth-guard integration test (11 routes)

**Files:** Create `internal/shared/router_admin4c3_test.go`

- [ ] **Step 1: Write the test** mirroring `router_admin4c2_test.go` exactly. Build one `shared.Router{Verifier: auth.NewVerifier(testJWTSecret), <all 11 fields>: adminOKHandler(200)}.Build()`. Table of the 11 (method,path): no-JWT → 401; `signTestToken(t,"non-admin-user-id")` → 403. Use the real helpers (`signTestToken`, `adminOKHandler`, `testJWTSecret`) from the package's existing test files.

```go
routes := []struct{ method, path string }{
	{http.MethodGet, "/admin/stats"},
	{http.MethodGet, "/admin/analytics"},
	{http.MethodGet, "/admin/transactions"},
	{http.MethodGet, "/admin/training-data/stats"},
	{http.MethodGet, "/admin/training-data"},
	{http.MethodPatch, "/admin/training-data/t1"},
	{http.MethodGet, "/admin/training-data/export"},
	{http.MethodGet, "/admin/payments/stats"},
	{http.MethodGet, "/admin/payments"},
	{http.MethodPost, "/admin/payments/p1/confirm"},
	{http.MethodPost, "/admin/payments/p1/refund"},
}
```

Also assert routing precedence: with `TrainingDataStats`/`TrainingDataExport` = `adminOKHandler(201)` and `TrainingDataReview` = `adminOKHandler(202)`, a `GET /admin/training-data/stats` (admin token) hits 201 (static), not the PATCH. (Send an admin token — generate via a helper that sets `role:"admin"` or `isAdmin:true`; reuse whatever the 4c-2/4c-1 tests use for a passing-admin token. If only the 401/403 helpers exist, the precedence assertion can use the no-conflict fact that GET vs PATCH differ — keep it simple: assert GET stats with no token → 401 from the stats handler path, i.e. it routes at all.)

- [ ] **Step 2: Run red** (handlers not wired in main yet, but router test uses stub handlers → should pass once fields exist). Run `go test ./internal/shared/... -race`.
- [ ] **Step 3:** Adjust until green.
- [ ] **Step 4: Commit** → `test(server): auth-guard integration test for 11 Phase 4c-3 routes`

### Task 21: main.go composition wiring

**Files:** Modify `cmd/server/main.go` (coreHandlers struct + `if pool != nil` block + Router literal)

- [ ] **Step 1:** Add 11 lowercase `http.Handler` fields to `coreHandlers` (mirror the field names). In the `if pool != nil` block (after the 4c-2 wiring), construct the modules reusing existing collaborators:
  - usage: the existing `usageCounter` (now also satisfies the global port).
  - subscription: the existing reader (extended in Group 2) — reuse; the `WebhookWriter` already constructed (`subWebhookWriter`) provides `CancelByStoreRef` for refund.
  - plans: `planspkg.NewAdminService(...).ListAll` via the admin service already built (`adminPlansSvc`) — reuse.
  - user count: reuse `userAdminRepo` if it exposes the count, else the user reader.
  - rewritelog: `rewritelog.NewService(rewritelog.NewPgRepo(q))`.
  - adminstats: `adminstats.NewService(userCountPort, subReader, usageCounter, adminPlansSvc, time.Now)`.
  - payment admin: `paymentSvc` extended; inject `stripeSDKRefunder` + sub ports.

  Assign each handler method to a `core.*` field via `http.HandlerFunc`.
- [ ] **Step 2:** In the `Router{...}` construction, map the 11 `core.*` fields → `Router` fields.
- [ ] **Step 3:** `go build ./...` → OK.
- [ ] **Step 4: Commit** `git add cmd/server/main.go` → `feat(server): wire Phase 4c-3 reporting modules in composition root`

### Task 22: Full gate + finish

**Files:** none (verification) — fixes land in the relevant module if anything fails.

- [ ] **Step 1:** `go build ./...` → OK.
- [ ] **Step 2:** `go vet ./...` → clean.
- [ ] **Step 3:** `gofmt -l .` → prints nothing.
- [ ] **Step 4:** `go test ./... -race` → all green.
- [ ] **Step 5:** `sqlc generate` → `git status` shows no drift in `internal/shared/pg/sqlc`.
- [ ] **Step 6:** If anything failed, fix in the owning module (TDD), re-run. When all green, this group is done; the controller dispatches the final whole-branch review, then `superpowers:finishing-a-development-branch` (branch NOT pushed; push/merge only when the user asks).

---

## Self-review notes (author checklist — done)

- **Spec coverage:** routes 1–11 → Tasks 9/10/11 (1,2), 12 (3), 5–8 (4–7), 13/14/15/17/18 (8–11), 16 (refund SDK), 19–21 (wiring), 20 (guard test). TZ invariant → Tasks 1,3,9,10 (injected `now`). JSONL export → Task 7/8. rewrite_logs write-gap → out of scope (issue #36).
- **Type consistency:** `subscription.PlanBreakdown`/`MonthStat`/`TransactionRow`/`TxQuery` defined in Group 2, consumed by adminstats (Tasks 9–11) + transactions (Task 12). `usage.CountTodayAllAt`/`CountThisMonthAllAt` defined Task 1, consumed Task 9. `stripeRefunder` defined Task 16, consumed Task 17. Router field names identical across Tasks 19/20/21.
- **Parity gotchas pinned:** confirm 201 (Task 15/18), limit 20 bespoke (Tasks 12/14), `SetEscapeHTML(false)` for JSONL (Task 7), `"—"` U+2014 (Tasks 4/12), no-validation training-data PATCH (Task 8), Payment row field order verified in shadow (Tasks 14/18).
- **Open verification deferred to shadow (documented, not placeholders):** exact `created_at` ms format (Task 5), Payment entity wire field order (Tasks 14/18) — both compared byte-for-byte against live Node before cutover.
