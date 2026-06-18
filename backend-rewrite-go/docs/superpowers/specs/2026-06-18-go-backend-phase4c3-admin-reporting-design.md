# Phase 4c-3 — Admin Reporting/Analytics Design

> Go backend port (`backend-rewrite-go`), byte-identical NestJS drop-in.
> **Node is parity authority.** Every route here reads/writes the same
> Postgres that Node uses; during the shadow phase the Go responses are
> compared byte-for-byte (JSON keys in order, status codes, error envelopes)
> against Node. Ship nothing to prod before the live shadow gate passes.

**Goal:** Port the 11 admin reporting/analytics endpoints — stats, analytics,
transactions, training-data (4 routes, new module), payments admin (4 routes) —
as a byte-identical drop-in.

**Parity authority sources (NestJS):**
- `src/admin/admin.controller.ts` lines 468–711 (the 11 route handlers)
- `src/subscriptions/subscriptions.service.ts` (countActive, getPlansBreakdown,
  getMonthlyStats, findAllPaginated, findLatestStripeForUserPlan, cancelByStripeSubId)
- `src/usage/usage.service.ts` (global countToday, countThisMonth)
- `src/rewrite/rewrite-log.service.ts` + `entities/rewrite-log.entity.ts`
- `src/payment/payment.service.ts` (getStats, findAll, adminConfirm, refund)

---

## 1. Scope — 11 routes

All mount inside the existing admin group in `internal/shared/router.go`
(`jwtMW → RequireAdmin`), via nil-guarded `http.Handler` fields on `Router`
(same pattern as the 25 routes added in 4c-2). All paths prefixed `/admin`.

| # | Route | Method → Status | Module |
|---|---|---|---|
| 1 | `/admin/stats` | GET → 200 | adminstats (new) |
| 2 | `/admin/analytics` | GET → 200 | adminstats (new) |
| 3 | `/admin/transactions` | GET → 200 | subscription (extend) + handler |
| 4 | `/admin/training-data/stats` | GET → 200 | rewritelog (new) |
| 5 | `/admin/training-data` | GET → 200 | rewritelog (new) |
| 6 | `/admin/training-data/:id` | PATCH → 200 `{success:true}` | rewritelog (new) |
| 7 | `/admin/training-data/export` | GET → 200 (non-JSON) | rewritelog (new) |
| 8 | `/admin/payments/stats` | GET → 200 | payment (extend) |
| 9 | `/admin/payments` | GET → 200 | payment (extend) |
| 10 | `/admin/payments/:id/confirm` | POST → 201 | payment (extend) |
| 11 | `/admin/payments/:id/refund` | POST → 201 | payment (extend) |

**Route-ordering note (chi):** mount `/admin/training-data/stats`,
`/admin/training-data/export` and `/admin/payments/stats` BEFORE the `:id`
variants are not a concern (distinct literal segments vs `{id}`), but
`/admin/training-data/export` and `/admin/training-data/stats` are literal
siblings of the `:id` PATCH — chi routes static segments before wildcards, so
ordering is automatic. Verify in the router test that `GET
/admin/training-data/stats` does not match the PATCH `:id` handler.

---

## 2. NEW module `internal/rewritelog/` (flat)

Flat module (single repo, single transport) per the CLAUDE.md flat-first rule:
`domain.go`, `usecase.go`, `repo_pg.go`, `handler.go`, `*_test.go`.

### 2.1 Domain (`rewrite_logs` table — read-only here; writes are issue #36)

Columns (from `rewrite-log.entity.ts`): `id uuid`, `tone varchar(20)`,
`input_text text`, `output_text text`, `model varchar(100)`,
`provider_type varchar(20)`, `response_time_ms int`,
`quality varchar(20) default 'pending'` (pending|approved|rejected),
`created_at timestamptz`.

```go
type RewriteLog struct {
    ID             string    `json:"id"`
    Tone           string    `json:"tone"`
    InputText      string    `json:"input_text"`
    OutputText     string    `json:"output_text"`
    Model          string    `json:"model"`
    ProviderType   string    `json:"provider_type"`
    ResponseTimeMs int       `json:"response_time_ms"`
    Quality        string    `json:"quality"`
    CreatedAt      time.Time `json:"created_at"`
}
```

JSON field order matches the TypeORM entity column order (the entity serialises
in declaration order). Confirm against a live Node `GET /admin/training-data`
response during shadow.

### 2.2 Use case ports + methods

```go
type Repo interface {
    Count(ctx) (int, error)
    CountByQuality(ctx) (pending, approved, rejected int, err error)
    FindPending(ctx, page, limit int) (logs []RewriteLog, total int, err error)
    UpdateQuality(ctx, id, quality string) error
    FindApprovedAsc(ctx) ([]RewriteLog, error)   // for export
}
```

- **Count** — `SELECT COUNT(*) FROM rewrite_logs`.
- **CountByQuality** — 3 counts (`quality='pending'|'approved'|'rejected'`).
  Node runs them as `Promise.all`; Go can run one grouped query OR three —
  result identical. Prefer a single `SELECT quality, COUNT(*) ... GROUP BY
  quality` mapped into the three buckets (default 0).
- **FindPending** — `WHERE quality='pending' ORDER BY created_at DESC
  LIMIT $limit OFFSET ($page-1)*$limit` + a `COUNT(*) WHERE quality='pending'`.
  Returns `{logs, total}`.
- **UpdateQuality** — `UPDATE rewrite_logs SET quality=$2 WHERE id=$1`.
- **FindApprovedAsc** — `WHERE quality='approved' ORDER BY created_at ASC`.

### 2.3 Responses (exact)

- **Route 4 `training-data/stats`** → `{ total, pending, approved, rejected }`.
  Node: `{ total, ...byQuality }` where byQuality = `{pending,approved,rejected}`.
  Key order: `total, pending, approved, rejected`.
- **Route 5 `training-data`** → `{ logs: [...], total }` (RewriteLog array).
  Node `findPending` returns `{ logs, total }`.
- **Route 6 `training-data/:id`** PATCH body `{ quality: 'approved'|'rejected' }`
  → `{ success: true }` (200). **No DTO validation in Node** (plain inline
  body type) → no whitelist/forbidNonWhitelisted. Go reads `quality` field;
  any value is passed straight to `UpdateQuality` (Node does the same — it does
  not validate the enum). Match Node: do NOT reject unknown/extra keys, do NOT
  validate the enum value.
- **Route 7 `training-data/export`** GET → **non-JSON**. Headers:
  `Content-Type: application/jsonl`,
  `Content-Disposition: attachment; filename=draftright-training-data.jsonl`.
  Body = approved logs (ASC) each as one JSON line joined by `\n`:
  ```json
  {"messages":[{"role":"system","content":"Rewrite the following text in a <tone> tone. Return only the rewritten text."},{"role":"user","content":"<input_text>"},{"role":"assistant","content":"<output_text>"}]}
  ```
  System content template (verbatim): ``Rewrite the following text in a ${tone}
  tone. Return only the rewritten text.``. Empty dataset → empty body (200,
  zero bytes). The handler bypasses `shared.WriteJSON` and writes the raw
  string. Status 200 (NestJS `res.send` default).

---

## 3. EXTEND `internal/adminstats/` (NEW flat module — routes 1, 2)

Stats + analytics are cross-module aggregations (user + subscription + usage +
plans). They own no table; they compose sibling read ports. Per CLAUDE.md
"declare a small port interface in your OWN usecase.go; main.go injects the
sibling's concrete." Put them in a new `internal/adminstats/` flat module
(usecase + handler + tests; no repo_pg — it has no table of its own).

### 3.1 Ports (consumer-side, small)

```go
type userCounter interface { Count(ctx) (int, error) }
type subStats interface {
    CountActive(ctx) (int, error)
    GetPlansBreakdown(ctx) ([]PlanBreakdown, error)
    GetMonthlyStats(ctx, months int) ([]MonthStat, error)
}
type usageCounter interface {
    CountTodayAll(ctx) (int, error)
    CountThisMonthAll(ctx) (int, error)
}
type planLister interface { FindAll(ctx) ([]plans.Plan, error) }
```

### 3.2 Route 1 `GET /admin/stats` → 200

```json
{ "total_users": N, "active_subscriptions": N, "rewrites_today": N, "rewrites_this_month": N }
```

Node runs 4 counts in `Promise.all` (`usersService.count`,
`subscriptionsService.countActive`, `usageService.countToday`,
`usageService.countThisMonth`). Go runs them sequentially (order irrelevant —
independent counts). Key order: `total_users, active_subscriptions,
rewrites_today, rewrites_this_month`.

### 3.3 Route 2 `GET /admin/analytics` → 200

```json
{ "mrr": N, "total_revenue": N, "plans_breakdown": [...], "monthly_stats": [...] }
```

- `plans_breakdown` (from `getPlansBreakdown`): array of
  `{ plan_name, active_count, price_cents }` — `WHERE status='active'`,
  `GROUP BY plan.name, plan.price_cents`, `COUNT(*)` as active_count. Field
  order: `plan_name, active_count, price_cents`. (Node maps raw rows; note the
  output key order is plan_name, active_count, price_cents.)
- `monthly_stats` (from `getMonthlyStats(12)`): array of
  `{ month, new_subscriptions, revenue_cents, churned }`, oldest→newest
  (`i = months-1 … 0`). Per month: `month` = `"YYYY-MM"`;
  `new_subscriptions` = COUNT subs with `started_at` in [monthStart,monthEnd);
  `revenue_cents` = `COALESCE(SUM(plan.price_cents),0)` over those subs;
  `churned` = COUNT subs `status IN ('cancelled','expired')` with `updated_at`
  in [monthStart,monthEnd).
- **MRR** (computed in the handler/usecase, matching the controller): start 0;
  load `plansService.findAll()`; for each breakdown row find plan by
  `plan.name == b.plan_name`; if `plan.price_cents > 0`: monthly →
  `mrr += active_count * price_cents`; yearly →
  `mrr += active_count * round(price_cents / 12)`. `round` = `Math.round`
  (round half away from zero for positive ints → `(price_cents + 6) / 12`
  is wrong; use `math.Round(float64(price_cents)/12)`).
- `total_revenue` = sum of `monthly_stats[].revenue_cents`.

### 3.4 TIMEZONE PARITY INVARIANT (critical)

Node builds month/day boundaries with the **server-local** timezone:
`new Date(y, m, 1)`, `setHours(0,0,0,0)`, `setDate(1)`. Go must compute the
identical instants. **The prod Node container TZ must be confirmed (expected
UTC)** and the Go process must use the same `time.Location`. Implement boundary
math with an injected `*time.Location` (default `time.UTC`, override via `TZ`)
so a misconfigured TZ shifts both identically. Document the confirmed prod TZ in
the plan. If prod Node runs UTC, Go uses `time.UTC` and month buckets align.

`getMonthlyStats` uses `now` (current time) — the Go usecase takes a
`clock func() time.Time` injected for test determinism (prod = `time.Now`).

---

## 4. EXTEND `internal/subscription/` (routes 3, supports 2 + 11)

New read methods on the subscription reader (all net-new):

- **CountActive** — `COUNT(*) WHERE status='active'`.
- **GetPlansBreakdown** — §3.3.
- **GetMonthlyStats(months)** — §3.3 (the 3 per-month sub-queries; loop in Go).
- **FindAllPaginated({search,page,limit})** — limit default **20**,
  `LEFT JOIN user, plan`, `ORDER BY sub.created_at DESC`,
  `LIMIT/OFFSET`; if `search` non-empty:
  `WHERE user.email ILIKE %s OR user.name ILIKE %s`. Returns
  `{subscriptions, total}` (getManyAndCount = 2 queries).
- **FindLatestStripeForUserPlan(userID, planID)** — `WHERE user_id, plan_id,
  store_type='stripe' ORDER BY created_at DESC LIMIT 1` (any status). For refund.
- **CancelByStripeSubId(subID)** — already specified as `cancelByStoreRef`
  (`store_type='stripe', store_transaction_id=subID, status='active'` →
  `status='cancelled'`); returns rows affected. **Check Go subscription module
  — a `CancelByStoreRef` likely already exists from the webhook port; reuse it,
  do not duplicate.**

### Route 3 `GET /admin/transactions` → 200

Bespoke query parse (NOT listquery): `page=parseInt|1`, `limit=parseInt|20`,
`search` passthrough. Maps each subscription to:

```json
{ "id","user_email","user_name","user_id","plan_name","price_cents","store_type","store_transaction_id","status","started_at","expires_at","created_at" }
```

`user_email`/`user_name`/`plan_name` fall back to `"—"` (em dash, U+2014) when
the relation is null; `price_cents` falls back to 0. Response
`{ transactions: [...], total }`.

---

## 5. EXTEND `internal/usage/` (supports route 1)

Add **global** counters (Node `countToday()`/`countThisMonth()` take no user):

- **CountTodayAll** — `COUNT(*) WHERE created_at >= todayStart` where
  todayStart = local midnight today.
- **CountThisMonthAll** — `COUNT(*) WHERE created_at >= monthStart` where
  monthStart = local first-of-month 00:00.

Same TZ invariant as §3.4 — share the injected `*time.Location`. The existing
`CountToday(userID)` (per-user, used by 4c-2 getUser) stays untouched.

---

## 6. EXTEND `internal/payment/` (routes 8–11)

### Route 8 `GET /admin/payments/stats` → 200

```json
{ "total","completed","pending","revenue" }
```

`total` = COUNT(*); `completed` = COUNT status='completed'; `pending` = COUNT
status='pending'; `revenue` = `COALESCE(SUM(amount),0) WHERE status='completed'`
(parsed to int). Key order: total, completed, pending, revenue.

### Route 9 `GET /admin/payments` → 200

Bespoke parse: `page|1`, `limit|20`, `status`, `search`, `sort_by`,
`sort_order` (`ASC` iff upper=='ASC' else `DESC`). Query: `LEFT JOIN user,
plan`; if `status` → `WHERE payment.status=$status`; if `search.trim()` →
ILIKE over `(reference_code, user.email, user.name, plan.name, method)`.
sortMap (6 allowed; default `payment.created_at`):
`reference_code→payment.reference_code, amount→payment.amount,
method→payment.method, status→payment.status, created_at→payment.created_at,
user.email→user.email`. `ORDER BY <field> <dir>`, LIMIT/OFFSET,
getManyAndCount. Response `{ payments: [...], total }` (the Payment entity
serialised with `user` + `plan` relations — confirm exact key order against
the Go payment domain + a live Node response during shadow).

### Route 10 `POST /admin/payments/:id/confirm` → 201

Body `{ notes?: string }`. Load payment + plan; not found → 404 `not-found`
("Payment not found"); status != pending → 400 `invalid-input` ("Payment is not
pending"). Else: status→completed, completed_at=now, notes = `notes ||
"Manually confirmed by admin"`, save; then **activateSubscription(payment)**.
Returns the updated payment (201). `activateSubscription` — locate the Go
equivalent (the webhook activation path already implements this; reuse the same
internal activation so a manual confirm and a webhook confirm produce identical
subscription rows). If no shared helper exists, extract one — do NOT fork
activation logic.

### Route 11 `POST /admin/payments/:id/refund` → 201 (HIGH COMPLEXITY)

Body `{ reason?: string }`. Guards (in order): not found → 404 `not-found`
("Payment not found"); already `refunded` → return the row (201, no-op);
status != completed → 400 ("Only completed payments can be refunded"); method
!= stripe → 400 (``Refund not supported for method '<method>'``); no Stripe
secret key (settings.stripe_secret_key || env STRIPE_SECRET_KEY) → 400
("Stripe secret_key is not configured").

Then **live Stripe REST** (net-new Go Stripe client — Go currently has only the
webhook strategy, no outbound Stripe calls):
1. `findLatestStripeForUserPlan(user_id, plan_id)` → `stripeSubId =
   sub.store_transaction_id`.
2. If `stripeSubId`: `GET /v1/subscriptions/{id}?expand[]=latest_invoice` →
   read `latest_invoice.charge` (fallback
   `latest_invoice.payments.data[0].payment.charge`); if charge →
   `POST /v1/refunds {charge, reason}` (capture refund id); then
   `DELETE /v1/subscriptions/{id}` (cancel; failures logged, not fatal).
3. status→refunded; append to `notes`:
   `Refunded by admin( (<reason>))?( — Stripe refund <refundId>)?` joined to
   prior notes with `" | "` (filter empties); save.
4. If `stripeSubId`: `cancelByStripeSubId(stripeSubId)` (DB sub → cancelled).
Returns the payment (201).

**Design for testability:** define a `stripeRefunder` port
(`RetrieveSubscription`, `CreateRefund`, `CancelSubscription`) with a real
HTTP impl (api.stripe.com, Bearer secret key, form-encoded) and a fake for
unit tests. The usecase never imports net/http for Stripe directly. Live
Stripe behaviour is verified manually in the shadow phase, not in CI.

---

## 7. Router wiring

Add 11 `http.Handler` fields to `Router` (4c-2 pattern), nil-guarded, mounted
in the existing admin group. `cmd/server/main.go` constructs the new modules
inside `if pool != nil` and assigns the fields. Reuse existing shared
collaborators (one `usage.Counter`, one `plans` reader, one `subscription`
reader, one `payment` service) — extend, do not fork.

---

## 8. Parity invariants (summary)

- POST confirm/refund → **201** (no `@HttpCode`); all GET + the PATCH → 200.
- Bespoke list parses (transactions, payments) default limit **20**, not
  listquery's 10. `parseInt` semantics (Go `strconv.Atoi`, fallback on error).
- Error envelope `{error,code,request_id}`: 404→`not-found`, 400→`invalid-input`,
  403→`forbidden`, 500→`internal`. Messages byte-exact (§6 routes 10/11).
- `monthly_stats` + global usage counts + month boundaries computed in the
  **same timezone as the Node prod container** (§3.4) — hard parity gate.
- training-data PATCH does NOT validate (no DTO) — match Node, accept any body.
- JSONL export = the only non-JSON response; exact Content-Type +
  Content-Disposition; lines joined by `\n`, no trailing newline.
- em-dash `"—"` (U+2014) fallbacks in transactions mapping.

---

## 9. Testing

- Each module: table-driven unit tests, fakes satisfy the consumer ports, no
  DB. rewritelog: count/quality/findPending/updateQuality/export (incl. empty
  dataset → empty body, system-prompt template exactness). adminstats: stats 4
  counts, analytics MRR (monthly+yearly+price_cents=0 skip), monthly_stats TZ
  boundaries with injected clock + location. payment: getStats, findAll
  (sortMap default + each field, ILIKE, status filter), adminConfirm (404/400/
  happy + activation called), refund (every guard + fake stripeRefunder happy
  path + already-refunded no-op).
- Router auth-guard integration test (extend the 4c-2 `router_admin*_test.go`
  table): all 11 routes — no-JWT → 401, non-admin token → 403.
- Full gate before finishing: `go build ./...`, `go vet ./...`,
  `gofmt -l .` (empty), `go test ./... -race`, `sqlc generate` (no drift).

---

## 10. Out of scope / follow-ups

- **Issue #36** — Go rewrite `callAI` never writes `rewrite_logs`. Training-data
  reads here are shadow-safe (same DB as Node) but at cutover Go-only stops
  populating the table. Fix in the rewrite module pre-cutover. NOT in 4c-3.
- `exportAll` (Ollama format) — Node has it but NO route exposes it. Skip.
- `findByUser` / `expireStalePayments` — not admin reporting routes. Skip.
- Carry-forward 4c-2 follow-ups #29–#35 (unchanged; fix post-cutover).

---

## 11. Decomposition

One spec → one implementation plan (~28–32 TDD bite-sized tasks; refund spans
3–4: stripeRefunder port, HTTP impl, usecase guards, wiring) → subagent-driven
execution (fresh implementer per task, spec review then quality review).
Branch: `feature/go-backend-phase4c3-admin-reporting-20260618` (from develop).
