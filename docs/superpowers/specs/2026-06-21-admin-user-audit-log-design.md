# Admin-User Deactivation Audit Log (#51) — Design

**Status:** Approved (brainstorm), pending implementation plan.
**Issue:** [#51](https://github.com/tannpv/draftright/issues/51) — go-port hardening, spec §9 follow-up to #32.
**Date:** 2026-06-21
**Scope decision:** **Go-only** (net-new feature, not a port-parity item — see §6).

---

## Goal

Record an append-only audit trail every time an admin deactivates another admin
account via `DELETE /admin/admin-users/:id` (the surface #32 guarded), and surface
it as a read-only page in the admin portal. Answers "who deactivated whom, when."

## Non-goals

- Auditing other soft-deletes (AI provider, plan, customer-user) — explicitly out
  of scope; this feature covers admin-user deactivation only.
- Editing/deleting audit rows — the log is append-only and immutable.
- Auditing *re-activation* (PATCH `is_active=true`) — only the deactivation path.
- Touching the Node backend — see §6.

---

## 1. Data model — `admin_user_audit_log`

New table, append-only:

| column | type | notes |
|---|---|---|
| `id` | `uuid` PK, default `gen_random_uuid()` | |
| `actor_admin_id` | `uuid NOT NULL` | who performed the deactivation (JWT `sub`) |
| `actor_email` | `text NOT NULL` | snapshot at action time |
| `target_admin_id` | `uuid NOT NULL` | the deactivated admin |
| `target_email` | `text NOT NULL` | snapshot at action time |
| `created_at` | `timestamptz NOT NULL DEFAULT now()` | |

**Design rules:**
- **No `action` column** — scope is deactivation-only (YAGNI). If scope ever widens,
  add the column then.
- **No foreign keys** to `admin_users`. The log must survive a later hard-delete or
  rename of either party; that's the whole point of snapshotting the emails.
- **Index** `idx_admin_user_audit_log_created_at` on `created_at DESC` — the read
  endpoint orders newest-first.

**Migration files (all three, since the Go service owns its own schema + the shadow
harness seeds from a frozen template):**
- `backend/sql/2026-06-21-admin-user-audit-log.sql` — canonical prod migration.
- `backend-rewrite-go/internal/platform/db/schema.sql` — append the table (Go service
  schema-of-record).
- `backend-rewrite-go/deploy/shadow/augment.sql` — so shadow-gate fixtures can seed
  rows.

---

## 2. Write path — atomic with the soft-delete

The audit row is written in the **same DB transaction** as the soft-delete. A
deactivation must never exist without a trail, and a trail must never exist without
a deactivation. This is a security record, not telemetry — **not** fire-and-forget
(contrast `rewrite_logs`).

**Location:** `internal/adminauth` — `AdminUsersHandler.Delete`
(`admin_users_handler.go`) already runs the two #32 guards (self-delete,
last-active-admin) *before* the soft-delete. The audit write goes **after** the
guards pass, inside the delete transaction — so a rejected deactivation writes **no**
audit row.

**Mechanism:** extend the usecase method the handler calls. Today:
`svc.SoftDelete(ctx, id)`. New: `svc.SoftDeleteWithAudit(ctx, actorID, targetID)`
which, in one `pgxpool` transaction:
1. Read `target_email` from `admin_users` WHERE id = targetID (also confirms the
   target exists).
2. Read `actor_email` from `admin_users` WHERE id = actorID.
3. `UPDATE admin_users SET is_active=false WHERE id=targetID` (the existing
   soft-delete).
4. `INSERT INTO admin_user_audit_log (actor_admin_id, actor_email, target_admin_id,
   target_email) VALUES (...)`.
5. Commit. Any step error → rollback → handler returns the existing
   `500 internal "admin-users failed"` envelope (unchanged).

**Actor email source:** read by `actorID` (= `claims.Sub`) inside the tx — robust,
no dependency on whether the admin JWT carries an `email` claim.

**Response unchanged:** `DELETE` still returns `200 { "success": true }`
byte-for-byte. The audit write is a pure side effect.

---

## 3. Read endpoint — `GET /admin/admin-user-audit`

- **Auth:** admin-JWT guarded (same guard chain as the other `/admin` routes).
- **Order:** newest-first (`created_at DESC`).
- **Pagination:** mirror the existing admin-users list endpoint — `?limit=` (default
  50, max 100) + `?offset=`, response shape **`{ "rows": [...], "total": <int> }`**
  (matches `adminUsersPaginatedResponse` field order rows, total).
- **Row shape:**
  ```json
  {
    "id": "<uuid>",
    "actor_admin_id": "<uuid>",
    "actor_email": "admin@draftright.info",
    "target_admin_id": "<uuid>",
    "target_email": "ops@draftright.info",
    "created_at": "2026-06-21T12:00:00.000Z"
  }
  ```
- **Module wiring (per `backend-rewrite-go/CLAUDE.md` recipe):** new query in
  `internal/shared/pg/queries_adminauth.sql` (`ListAdminUserAudit`,
  `CountAdminUserAudit`, `InsertAdminUserAudit`) → `sqlc generate`; repo method on
  the existing adminauth pg repo; usecase `ListAudit`; handler `ListAudit`; route
  field on `coreHandlers` + mount in `shared/router.go`.

---

## 4. Admin portal — dedicated "Admin Audit Log" page

- New route + nav item under the admin section (alongside Admin Users).
- Read-only paginated table, columns: **When · Actor · Target**
  (`created_at` formatted · `actor_email` · `target_email`).
- Calls `GET /admin/admin-user-audit` through the existing admin API client.
- Follows the Modernize dark-theme list-page pattern already used by other admin
  list pages (reuse the existing table + pagination components — do not fork).
- Empty state: "No admin deactivations recorded."

---

## 5. Testing

**Go (fakes satisfy ports, no DB):**
- Usecase: audit row written on successful soft-delete; **not** written when either
  #32 guard rejects (self-delete, last-admin); rollback on insert failure leaves
  `is_active` unchanged.
- Handler `Delete`: guard-reject paths still return their existing 400s and trigger
  no audit write.
- Handler `ListAudit`: returns `{rows,total}`, newest-first, respects limit/offset,
  admin-auth required.

**Portal:** component renders rows, pagination, and empty state (follow existing
admin-page test pattern, if any; otherwise a render smoke test).

---

## 6. Why Go-only (no Node)

Node serves **zero** prod traffic since the 2026-06-19 Phase-5 cutover; it is kept
only as (a) the shadow-gate **parity oracle** and (b) a **rollback anchor**. The
shadow gate exists to prove the *port* is byte-faithful to Node — it has nothing to
assert about a feature Node never had. Building the audit log in Node too would be
throwaway TypeScript on a backend slated for retirement (~mid-July 2026, after the
prod bake).

Therefore:
- Implement in **Go + admin portal only**.
- The new `GET /admin/admin-user-audit` route is **out of shadow-gate scope** (Node
  returns 404; that is expected, not a parity failure). Add it to the gate's
  skip/allow list so the gate does not flag the Go-only route. The `DELETE`
  response is unchanged, so existing admin-users delete fixtures stay green.

This is the **first net-new Go-only feature** — it sets the precedent that
post-cutover features target Go alone.

---

## 7. Deployment

Standard go-port batch pipeline:
1. Branch from `develop`, implement, full local Go gate
   (build/vet/gofmt/test-race/sqlc).
2. Run the prod migration `2026-06-21-admin-user-audit-log.sql` **before** the new
   image starts (the table must exist before the binary queries it).
3. Live shadow gate on dev VPS must stay green (the new route is gate-skipped;
   existing surface unchanged).
4. Merge feat→develop→main (`--no-ff`), push.
5. **Prod deploy requires explicit per-batch authorization** (ask): apply migration
   on prod DB → rebuild `draftright-backend-go:latest` from main → `compose up -d
   backend-go` @ `/opt/draftright`; admin portal `npm run build` + sudo-rsync `dist/`
   to `/var/www/admin.draftright/`.
6. Label issue `status: deployed to production` + add `## ✅ How to Verify` comment.
   Leave open.

---

## 8. Open items resolved during planning

- Actor-email source → read by `claims.Sub` inside the tx (no JWT-claim dependency).
- Pagination shape → `{rows,total}` + `limit`/`offset`, mirroring admin-users list.
