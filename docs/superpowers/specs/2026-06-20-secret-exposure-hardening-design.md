# Secret-Exposure Hardening (#29–#32) — Design

**Date:** 2026-06-20
**Issues:** #29, #30, #31, #32
**Surfaces:** NestJS backend (`backend/`, parity authority) · Go port (`backend-rewrite-go/`) · admin portal (`admin/`)
**Goal:** Stop the admin API from returning secrets in plaintext and stop an admin from deleting themselves or the last admin — coordinated across all three surfaces so the byte-parity shadow gate stays green.

---

## 1. Problem

The admin API serializes raw TypeORM entities with zero `@Exclude`, leaking secrets to any authenticated admin client:

| Issue | Route(s) | Leak |
|---|---|---|
| #29 | `GET /admin/ai-providers`, `/paginated`, create/update echo | `ai_providers.api_key` in plaintext |
| #30 | `GET /admin/settings`, PATCH echo | 11 secret columns in `app_settings` plaintext (2 Apple ID columns are identifiers, left as-is) |
| #31 | `GET /admin/users/:id` (+ list) | `password_hash`, reset code/expiry/attempts, email-verification code/expiry |
| #32 | `DELETE /admin/admin-users/:id` | no guard — admin can deactivate self or the last active admin |

The Go port is a **byte-identical drop-in**. The shadow gate (`cmd/shadowdiff`) diffs Go vs Node response bodies on a fresh DB per fixture. Therefore every fix MUST land in **Node and Go simultaneously** — fixing one side breaks parity, and a Node-only fix reopens on Go cutover (and vice-versa).

## 2. Core thesis — why the gate stays green

The gate compares **Go ⇄ Node**, not new-code ⇄ old-code. If both backends mask/drop identically, both emit identical bytes → diff is empty → **PASS, with no new gate mechanism**. The shadowdiff fixture format (`ignore_value_of`, `status_only`) is untouched.

The only change that can move a status code is #32's new guard. It is handled with two **new** fixtures (below); the existing delete fixture does not trip the guard.

## 3. The mask function (byte-identical TS + Go)

A single pure function, implemented identically in both backends:

```
maskSecret(s) =
  s == ""        → ""                       # unset stays empty
  runeLen(s) < 16 → "…"                     # short/weak secret fully hidden
  else           → first3(s) + "…" + last4(s)
```

- `…` is **U+2026 HORIZONTAL ELLIPSIS** — the single marker the write-path keys on. Real API keys/secrets are ASCII and never contain it.
- API keys are ASCII, so rune / UTF-16 / byte indexing all agree; spec pins **rune-based** indexing to be explicit. Go: `[]rune(s)`. TS: `Array.from(s)` (code-point iteration), not `s.slice` on UTF-16 units — pinned so a future non-ASCII secret can't drift Go vs Node.
- Reveal threshold = **16**. Below it, reveal nothing (just the `…` marker so the portal can tell "set" from "unset"). At/above, reveal first-3 + last-4.
- Examples: `"sk-proj-abcd1234wxyz"` → `"sk-…wxyz"`; `"short"` → `"…"`; `""` → `""`.

### Where it applies (read path)

| Issue | Fields masked |
|---|---|
| #29 | `ai_providers.api_key` — list, paginated, detail, **create echo, update echo** |
| #30 | `app_settings`: `stripe_secret_key`, `stripe_webhook_secret`, `paypal_client_secret`, `momo_access_key`, `momo_secret_key`, `casso_api_key`, `sepay_api_key`, `resend_api_key`, `lemonsqueezy_api_key`, `lemonsqueezy_webhook_secret`, `google_client_secret` — GET + PATCH echo |

**Not masked:** `apple_team_id`, `apple_key_id` — these are identifiers, not secrets (confirmed). Apple's actual secret is the `.p8` private key, which is not stored in `app_settings`.

### #31 — drop, not mask

`GET /admin/users/:id` (and the list endpoint if it serializes the same entity) **omit** these columns entirely:

- `password_hash`
- `password_reset_code`, `password_reset_expires`, `password_reset_attempts`
- `email_verification_code`, `email_verification_expires`

Pure removal — these are never sent back by any client, so there is no write-path risk. Both backends drop them → byte-identical → gate green.

## 4. Write path — masked-tail consequence

Masked tail is **not** a fixed sentinel, so the write guard keys on the **marker char**, not value equality. Per secret field, on create/update:

```
if value contains "…" (U+2026)  → omit this field from the persisted patch (keep stored value)
```

- Applies to `app_settings` PATCH and `ai_providers` create/update, both backends.
- Empty string still means **explicit clear** — unchanged semantics, not affected by this guard.
- Rationale: defense in depth. Protects the stored secret regardless of which client (current portal, future client, curl) echoes a masked value back.

## 5. Portal (belt-and-suspenders)

| Page | Field(s) | Change |
|---|---|---|
| `SettingsPage.tsx` | all `app_settings` secrets | Build the PATCH body from **only edited secret fields**; never send an untouched (masked) secret. Track per-secret "dirty" state. |
| `ProvidersPage.tsx` | `api_key` | **No change** — already empties on edit + conditional send (`if (form.api_key)`). |
| `AdminUsersPage.tsx` | `password` | **No change** — same safe pattern. |

The backend write guard (§4) is the safety net; the SettingsPage fix is the clean-UX layer. Both ship.

## 6. #32 — admin delete guard

`DELETE /admin/admin-users/:id` gains two guards **before** the `is_active = false` write:

1. **Self-delete:** acting admin id (from JWT) == target id → reject.
2. **Last-active-admin:** target is the only remaining `is_active = true` admin → reject.

Both reject with **HTTP 400** and a fixed message. **Node is the parity authority** — Node defines the exact message strings and the error envelope (`AllExceptionsFilter` shape + `code`), Go mirrors byte-for-byte.

Proposed messages (finalize in plan against Node's existing exception style):
- self: `"You cannot deactivate your own admin account"`
- last admin: `"Cannot deactivate the last active admin"`

The acting admin id must be available in the handler. Node: from the admin JWT guard (`req.user`/`@CurrentUser`). Go: from the admin auth middleware context. If the route currently does not capture the caller, add it (both sides) — this is part of the fix.

### Gate impact

- Existing `admin_crud/admin_users_delete.json` deletes a placeholder id with the seeded admin still active and acting-admin ≠ target → trips neither guard → unchanged 2xx → green.
- **Add 2 new fixtures** so the guard is itself gated:
  - `admin_users_delete_self.json` — acting admin deletes own id → 400 + message.
  - `admin_users_delete_last_admin.json` — delete the sole active admin → 400 + message.
  - Both rely on the standard `{{admin_token}}` bootstrap; the last-admin case may need the fixture/DB-template to ensure exactly one active admin at reset time (note for the plan: verify the frozen template seeds a single active admin, or the fixture sequence deactivates others first).

## 7. Testing

- **Mask function:** table tests both backends — empty → empty, `<16` → `…`, `≥16` → `first3…last4`, ASCII boundary.
- **Write guard:** unit — a field containing `…` is omitted from the patch; a real new value is persisted; empty string clears.
- **#31 drop:** assert the six columns are absent from the serialized user.
- **#32 guard:** unit — self-delete 400, last-admin 400, normal delete still succeeds.
- **Gate:** full `go test ./... -race`, `gofmt -l .` empty, `go vet`, `sqlc generate` no drift; then the live shadow gate (`deploy/shadow/run-gate.sh`) — green except the 2 new #32 fixtures asserting the new 400s.

## 8. Rollout order (one plan, sequenced)

1. **#31** — user field-drop. Pure removal, no portal/write risk. Lowest risk, highest severity (password_hash + live reset codes). Ship first.
2. **#29** — `ai_providers.api_key` mask (read + create/update echo) + write guard. Portal already safe.
3. **#30** — `app_settings` secret mask (read + echo) + write guard + SettingsPage dirty-tracking.
4. **#32** — admin delete guard + 2 new fixtures.

Each step changes Node + Go together; commit per issue; run the gate after each. GitFlow: branch from `develop`, `--no-ff`, never direct to `main`/`develop`, push/merge only when asked.

## 9. Out of scope

- Encrypting secrets at rest (DB-level) — separate hardening, not required to close these issues.
- Rotating any currently-exposed secret — operational follow-up; note in issue comments that exposed keys *should* be rotated since they were readable by every admin session.
- Audit logging of admin deletes — separate feature.

## 10. File map (for the plan)

**Node (`backend/`):**
- `src/ai-providers/entities/ai-provider.entity.ts`, `ai-providers.service.ts` (findAll/findAllPaginated/findById/create/update)
- `src/admin/admin.controller.ts` — ai-providers routes (~549–565), settings routes (~432–457), users `:id` (~510–517), admin-users delete (~775–779)
- `src/admin/entities/app-settings.entity.ts`
- `src/users/entities/user.entity.ts`, `users.service.ts` findById
- new shared `maskSecret` util + secret-field lists
- admin JWT/current-user wiring for #32 if not already present

**Go (`backend-rewrite-go/`):**
- `internal/aiprovider/` — domain mask on serialize, write-guard in service, create/update echo
- `internal/appsettings/` — mask on GET/PATCH serialize, write-guard
- `internal/user/admin_handler.go` — drop fields on serialize
- admin-users delete handler/service — self + last-admin guard, caller id from auth context
- shared `maskSecret` (mirror TS), secret-field lists
- `cmd/shadowdiff/fixtures/admin_crud/` — 2 new delete fixtures

**Portal (`admin/`):**
- `src/pages/SettingsPage.tsx` — per-secret dirty tracking, send only edited secrets
