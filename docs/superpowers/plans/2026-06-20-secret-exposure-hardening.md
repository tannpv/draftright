# Secret-Exposure Hardening (#29–#32) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop the admin API from returning secrets in plaintext (#29 ai_provider api_key, #30 app_settings payment/SMTP keys, #31 user password_hash + reset/verification codes) and guard admin self/last-admin soft-delete (#32) — coordinated across NestJS, the Go port, and the admin portal so the byte-parity shadow gate stays green.

**Architecture:** Every read-path change masks/drops the same fields in **both** backends in lockstep → identical bytes → gate green with no new fixture mechanism. A shared `maskSecret` (mirrored TS + Go) renders `first3…last4` (U+2026 marker, hidden below 16 chars). The same marker drives a write-path guard: any incoming secret containing `…` is dropped on save so a masked echo never overwrites the real value. #32 adds two delete guards (both backends, Node defines the 400 message, Go mirrors byte-for-byte).

**Tech Stack:** NestJS 10 / TypeScript / TypeORM (`backend/`); Go 1.26 / chi / pgx / sqlc (`backend-rewrite-go/`); React / Vite (`admin/`); shadowdiff parity harness.

**Parity invariant (read before every task):** The shadow gate (`cmd/shadowdiff`) diffs Go vs Node response bodies byte-for-byte on a fresh DB. A change to one backend without the matching change to the other = gate failure. Every issue below changes Node AND Go together. The masked value must be byte-identical on both sides → `maskSecret` must be implemented identically (rune/code-point indexing, same 16-char threshold, same U+2026 marker).

**Branch:** `feature/secret-exposure-hardening-20260620` (already created; spec committed there).

---

## File Structure

**New files:**
- `backend-rewrite-go/internal/shared/secrets.go` — `MaskSecret`, `MaskMarker`, `ContainsMaskMarker`
- `backend-rewrite-go/internal/shared/secrets_test.go`
- `backend/src/common/mask-secret.util.ts` — `maskSecret`, `MASK_MARKER`, `containsMaskMarker`
- `backend/src/common/mask-secret.util.spec.ts`
- `backend/src/users/sanitize-user.util.ts` — `stripUserSecrets`
- `backend/src/ai-providers/mask-provider.util.ts` — `maskProvider`
- `backend/src/admin/mask-settings.util.ts` — `maskSettings`, `stripMaskedSecretsFromBody`
- `backend-rewrite-go/cmd/shadowdiff/fixtures/admin_crud/admin_users_delete_self.json`

**Modified files:**
- Go: `internal/aiprovider/domain.go` (mask in MarshalJSON), `internal/aiprovider/handler.go` (write guard), `internal/appsettings/domain.go` (mask in MarshalJSON), `internal/appsettings/handler.go` (write guard), `internal/user/admin_domain.go` (drop fields in MarshalJSON), `internal/adminauth/admin_users_handler.go` + its service/repo (#32 guard), `cmd/shadowdiff/bootstrap.go` (`{{admin_id}}` substitution), `internal/shared/pg/queries_*.sql` (+ `CountActiveAdminUsers`)
- Node: `src/admin/admin.controller.ts` (apply mask/strip + write guards + #32 guard), test specs
- Portal: `admin/src/pages/SettingsPage.tsx` (per-secret dirty tracking)

---

## ISSUE #31 — Drop user secrets (ships first: pure removal, zero write risk)

### Task 1: Go — drop 6 secret columns from `UserDetail` JSON

**Files:**
- Modify: `backend-rewrite-go/internal/user/admin_domain.go` (the `MarshalJSON` wire struct + literal, ~lines 66–114)
- Test: `backend-rewrite-go/internal/user/admin_domain_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

```go
package user

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUserDetail_OmitsSecretColumns(t *testing.T) {
	code := "123456"
	hash := "$2b$10$abcdefghijklmnopqrstuv"
	exp := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	u := UserDetail{
		ID: "u1", Email: "a@b.com", PasswordHash: &hash, Name: "A",
		EmailVerificationCode: &code, EmailVerificationExpires: &exp,
		PasswordResetCode: &code, PasswordResetExpires: &exp, PasswordResetAttempts: 3,
		CreatedAt: exp, UpdatedAt: exp,
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, k := range []string{
		"password_hash", "email_verification_code", "email_verification_expires",
		"password_reset_code", "password_reset_expires", "password_reset_attempts",
	} {
		if strings.Contains(s, k) {
			t.Errorf("response leaks %q: %s", k, s)
		}
	}
	// Non-secret columns must remain.
	for _, k := range []string{"id", "email", "name", "is_active", "auth_provider", "created_at"} {
		if !strings.Contains(s, k) {
			t.Errorf("missing expected key %q", k)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/user/ -run TestUserDetail_OmitsSecretColumns -v`
Expected: FAIL (response still contains `password_hash` etc.)

- [ ] **Step 3: Remove the 6 fields from the `MarshalJSON` anonymous wire struct AND its literal**

In `admin_domain.go` `func (u UserDetail) MarshalJSON()`, delete these struct-tag lines:
```go
		PasswordHash             *string `json:"password_hash"`
		EmailVerificationCode    *string `json:"email_verification_code"`
		EmailVerificationExpires *string `json:"email_verification_expires"`
		PasswordResetCode        *string `json:"password_reset_code"`
		PasswordResetExpires     *string `json:"password_reset_expires"`
		PasswordResetAttempts    int     `json:"password_reset_attempts"`
```
and the matching assignments in the struct literal:
```go
		PasswordHash:             u.PasswordHash,
		EmailVerificationCode:    u.EmailVerificationCode,
		EmailVerificationExpires: isoPtr(u.EmailVerificationExpires),
		PasswordResetCode:        u.PasswordResetCode,
		PasswordResetExpires:     isoPtr(u.PasswordResetExpires),
		PasswordResetAttempts:    u.PasswordResetAttempts,
```
Leave the `UserDetail` struct fields themselves intact (data still loads from DB; only the JSON projection drops them). If `isoPtr` becomes unused after the deletion, keep it — it is still used by other nullable timestamps (`created_at`/`updated_at` use `shared.ISOMillis`; verify `isoPtr` has another caller, else remove it to satisfy `go vet`).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/user/ -run TestUserDetail_OmitsSecretColumns -v`
Expected: PASS

- [ ] **Step 5: Verify the admin users LIST path uses the same projection**

Run: `cd backend-rewrite-go && grep -rn "UserDetail\|password_hash" internal/user/`
If `GET /admin/users` (list) serializes a different struct that still emits these columns, drop them there too (same edit). If it reuses `UserDetail`, it is already fixed. Note findings in the commit body.

- [ ] **Step 6: Full Go gate + commit**

Run: `cd backend-rewrite-go && gofmt -w internal/user/ && go vet ./internal/user/ && go test ./internal/user/ -race`
Expected: PASS, gofmt clean.

```bash
git add backend-rewrite-go/internal/user/admin_domain.go backend-rewrite-go/internal/user/admin_domain_test.go
git commit -m "fix(go/user): drop password_hash + reset/verification codes from admin user JSON (#31)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Node — strip the same 6 columns from admin user responses

**Files:**
- Create: `backend/src/users/sanitize-user.util.ts`
- Create: `backend/src/users/sanitize-user.util.spec.ts`
- Modify: `backend/src/admin/admin.controller.ts` (`getUser` ~510–517, plus any other handler returning a raw user entity)

- [ ] **Step 1: Write the failing test**

```typescript
// backend/src/users/sanitize-user.util.spec.ts
import { stripUserSecrets } from './sanitize-user.util';

describe('stripUserSecrets', () => {
  it('removes secret columns but keeps profile columns', () => {
    const user: any = {
      id: 'u1', email: 'a@b.com', name: 'A', is_active: true, role: 'user',
      password_hash: '$2b$10$x', email_verification_code: '123456',
      email_verification_expires: new Date(), password_reset_code: '654321',
      password_reset_expires: new Date(), password_reset_attempts: 2,
    };
    const out = stripUserSecrets(user);
    for (const k of [
      'password_hash', 'email_verification_code', 'email_verification_expires',
      'password_reset_code', 'password_reset_expires', 'password_reset_attempts',
    ]) {
      expect(out).not.toHaveProperty(k);
    }
    expect(out).toMatchObject({ id: 'u1', email: 'a@b.com', name: 'A', is_active: true });
  });

  it('returns null for null input (missing user is 200 with user:null)', () => {
    expect(stripUserSecrets(null)).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/users/sanitize-user.util.spec.ts`
Expected: FAIL ("Cannot find module './sanitize-user.util'")

- [ ] **Step 3: Implement the util**

```typescript
// backend/src/users/sanitize-user.util.ts
// Columns that must never reach an admin client. Dropped (not masked) because
// no client ever writes them back — see spec §3 (#31). Mirrored by the Go
// port: internal/user/admin_domain.go MarshalJSON omits the same keys, so both
// backends emit byte-identical JSON and the shadow gate stays green.
const USER_SECRET_COLUMNS = [
  'password_hash',
  'email_verification_code',
  'email_verification_expires',
  'password_reset_code',
  'password_reset_expires',
  'password_reset_attempts',
] as const;

export function stripUserSecrets<T extends Record<string, unknown> | null>(user: T): T {
  if (!user) return user;
  const copy: Record<string, unknown> = { ...user };
  for (const k of USER_SECRET_COLUMNS) delete copy[k];
  return copy as T;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/users/sanitize-user.util.spec.ts`
Expected: PASS

- [ ] **Step 5: Apply in the controller**

In `admin.controller.ts`, add the import near the other `../users` import:
```typescript
import { stripUserSecrets } from '../users/sanitize-user.util';
```
Change `getUser` (line ~516) from:
```typescript
    return { user, subscription: sub, usage_today: usageToday, recent_usage: recentUsage };
```
to:
```typescript
    return { user: stripUserSecrets(user), subscription: sub, usage_today: usageToday, recent_usage: recentUsage };
```

- [ ] **Step 6: Find and fix every other admin handler returning a raw user entity**

Run: `cd backend && grep -n "usersService.findById\|usersService.update\|this.usersService" src/admin/admin.controller.ts`
For each handler that returns a `user` entity to the client (e.g. the `PATCH /admin/users/:id` updateUser echo and any `GET /admin/users` list mapping), wrap the user object in `stripUserSecrets(...)`. For a list, map each row: `users.map(stripUserSecrets)`. Match exactly what the Go side emits (Task 1 Step 5). Record each site touched in the commit body.

- [ ] **Step 7: Build + commit**

Run: `cd backend && npx tsc --noEmit && npx jest src/users/sanitize-user.util.spec.ts`
Expected: tsc clean, tests PASS.

```bash
git add backend/src/users/sanitize-user.util.ts backend/src/users/sanitize-user.util.spec.ts backend/src/admin/admin.controller.ts
git commit -m "fix(node/admin): strip password_hash + reset/verification codes from admin user responses (#31)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## ISSUE #29 — Mask ai_provider api_key (read echo + write guard)

### Task 3: Go — shared `MaskSecret` helper

**Files:**
- Create: `backend-rewrite-go/internal/shared/secrets.go`
- Create: `backend-rewrite-go/internal/shared/secrets_test.go`

- [ ] **Step 1: Write the failing test**

```go
package shared

import "testing"

func TestMaskSecret(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},                                  // unset stays empty
		{"short", "…"},                            // <16 → marker only
		{"0123456789abcde", "…"},                  // 15 runes → marker only
		{"0123456789abcdef", "012…cdef"},          // 16 runes → first3+marker+last4
		{"sk-proj-abcd1234wxyz", "sk-…wxyz"},      // typical key
	}
	for _, c := range cases {
		if got := MaskSecret(c.in); got != c.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestContainsMaskMarker(t *testing.T) {
	if !ContainsMaskMarker("sk-…wxyz") {
		t.Error("expected marker detected")
	}
	if ContainsMaskMarker("sk-proj-realkey-no-marker") {
		t.Error("false positive on real key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/shared/ -run "TestMaskSecret|TestContainsMaskMarker" -v`
Expected: FAIL (undefined: MaskSecret)

- [ ] **Step 3: Implement the helper**

```go
// backend-rewrite-go/internal/shared/secrets.go
package shared

import "strings"

// MaskMarker is U+2026 HORIZONTAL ELLIPSIS — the single marker MaskSecret
// emits and the write path keys on. Real API keys/secrets are ASCII and never
// contain it.
const MaskMarker = "…"

// MaskSecret renders a stored secret for an admin API response. Empty stays
// empty (unset). Secrets shorter than 16 runes reveal nothing but the marker;
// longer secrets reveal first 3 + marker + last 4. Implemented identically in
// the Node backend (src/common/mask-secret.util.ts) — both sides MUST agree
// byte-for-byte or the Go-vs-Node shadow gate fails. Rune-based indexing (Node
// uses code-point iteration) keeps a future non-ASCII secret from drifting.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) < 16 {
		return MaskMarker
	}
	return string(r[:3]) + MaskMarker + string(r[len(r)-4:])
}

// ContainsMaskMarker reports whether a secret value being written is a masked
// echo (contains U+2026) and must be ignored on save (keep the stored value).
func ContainsMaskMarker(s string) bool {
	return strings.Contains(s, MaskMarker)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/shared/ -run "TestMaskSecret|TestContainsMaskMarker" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/shared/secrets.go backend-rewrite-go/internal/shared/secrets_test.go
git commit -m "feat(go/shared): MaskSecret helper (first3…last4, U+2026 marker) (#29)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Go — mask `api_key` in `AiProvider` JSON + drop masked echo on write

**Files:**
- Modify: `backend-rewrite-go/internal/aiprovider/domain.go` (`MarshalJSON`, the `APIKey` field assignment)
- Modify: `backend-rewrite-go/internal/aiprovider/handler.go` (`Create` + `Update` write guard)
- Test: `backend-rewrite-go/internal/aiprovider/domain_test.go` (create/extend), `backend-rewrite-go/internal/aiprovider/handler_test.go` (extend)

- [ ] **Step 1: Write the failing serialize test**

```go
// backend-rewrite-go/internal/aiprovider/domain_test.go
package aiprovider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAiProvider_MasksAPIKey(t *testing.T) {
	p := AiProvider{ID: "p1", Name: "OpenAI", Type: "openai", APIKey: "sk-proj-abcd1234wxyz", Model: "gpt-4o"}
	b, _ := json.Marshal(p)
	s := string(b)
	if strings.Contains(s, "sk-proj-abcd1234wxyz") {
		t.Errorf("leaks raw api_key: %s", s)
	}
	if !strings.Contains(s, `"api_key":"sk-…wxyz"`) {
		t.Errorf("expected masked api_key, got: %s", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/aiprovider/ -run TestAiProvider_MasksAPIKey -v`
Expected: FAIL (raw key present)

- [ ] **Step 3: Mask in `MarshalJSON`**

In `domain.go` `MarshalJSON`, change the `APIKey` assignment in the struct literal from `APIKey: p.APIKey,` to:
```go
		APIKey: shared.MaskSecret(p.APIKey), Model: p.Model, Temperature: p.Temperature,
```
(`shared` is already imported.) `MarshalJSON` is the HTTP projection only — the provider-call factory reads the `APIKey` Go field directly, so masking the JSON does not affect live provider calls.

- [ ] **Step 4: Run serialize test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/aiprovider/ -run TestAiProvider_MasksAPIKey -v`
Expected: PASS

- [ ] **Step 5: Write the failing write-guard test**

```go
// add to backend-rewrite-go/internal/aiprovider/handler_test.go
func TestUpdate_DropsMaskedAPIKeyEcho(t *testing.T) {
	var captured ProviderPatch
	svc := &fakeProviderSvc{updateFn: func(_ context.Context, _ string, p ProviderPatch) (AiProvider, error) {
		captured = p
		return AiProvider{ID: "p1", APIKey: "stored-key-unchanged-1234567890"}, nil
	}}
	h := NewHandler(svc)
	body := `{"api_key":"sk-…wxyz","model":"gpt-4o"}`
	req := httptest.NewRequest("PATCH", "/admin/ai-providers/p1", strings.NewReader(body))
	req = withURLParam(req, "id", "p1")
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if captured.APIKey != nil {
		t.Errorf("masked api_key echo must be dropped, got %v", *captured.APIKey)
	}
	if captured.Model == nil || *captured.Model != "gpt-4o" {
		t.Errorf("non-secret field must still be applied")
	}
}
```
(Use the existing fake/service-double and URL-param helper in `handler_test.go`; if the file fakes the service via an interface, add `updateFn`/`createFn` hooks to that fake. If the handler currently depends on a concrete `*Service`, introduce a small consumer-side interface `providerService` with `Create`/`Update`/`List`/`ListPaginated`/`SoftDelete` in the handler file and have the fake satisfy it — minimal, matches the "interfaces on the consumer side" rule.)

- [ ] **Step 6: Run write-guard test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/aiprovider/ -run TestUpdate_DropsMaskedAPIKeyEcho -v`
Expected: FAIL (masked echo persisted)

- [ ] **Step 7: Add the write guard in `handler.go`**

In `Update`, after building `patch` and before `h.svc.Update`, insert:
```go
	// Drop a masked echo so the portal re-saving a masked secret can't
	// overwrite the real key (spec §4). The marker is ASCII-absent in real keys.
	if patch.APIKey != nil && shared.ContainsMaskMarker(*patch.APIKey) {
		patch.APIKey = nil
	}
```
In `Create`, after building `in` and before `h.svc.Create`, insert:
```go
	if shared.ContainsMaskMarker(in.APIKey) {
		in.APIKey = "" // ignore a masked echo on create (keep DB default empty)
	}
```
Add `"github.com/tannpv/draftright-rewrite/internal/shared"` to the handler imports if not present.

- [ ] **Step 8: Run write-guard test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/aiprovider/ -run TestUpdate_DropsMaskedAPIKeyEcho -v`
Expected: PASS

- [ ] **Step 9: Full module gate + commit**

Run: `cd backend-rewrite-go && gofmt -w internal/aiprovider/ && go vet ./internal/aiprovider/ && go test ./internal/aiprovider/ -race`
Expected: PASS.

```bash
git add backend-rewrite-go/internal/aiprovider/
git commit -m "fix(go/aiprovider): mask api_key in responses + drop masked echo on write (#29)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Node — `maskSecret` util + mask provider responses + write guard

**Files:**
- Create: `backend/src/common/mask-secret.util.ts`
- Create: `backend/src/common/mask-secret.util.spec.ts`
- Create: `backend/src/ai-providers/mask-provider.util.ts`
- Modify: `backend/src/admin/admin.controller.ts` (`listProviders`, `listProvidersPaginated`, `createProvider`, `updateProvider`)

- [ ] **Step 1: Write the failing mask-util test (parity with Go)**

```typescript
// backend/src/common/mask-secret.util.spec.ts
import { maskSecret, containsMaskMarker, MASK_MARKER } from './mask-secret.util';

describe('maskSecret (must match Go internal/shared.MaskSecret byte-for-byte)', () => {
  it.each([
    ['', ''],
    ['short', '…'],
    ['0123456789abcde', '…'],        // 15 chars
    ['0123456789abcdef', '012…cdef'], // 16 chars
    ['sk-proj-abcd1234wxyz', 'sk-…wxyz'],
  ])('maskSecret(%j) = %j', (input, want) => {
    expect(maskSecret(input)).toBe(want);
  });
  it('marker is U+2026', () => expect(MASK_MARKER).toBe('…'));
  it('containsMaskMarker detects echoes', () => {
    expect(containsMaskMarker('sk-…wxyz')).toBe(true);
    expect(containsMaskMarker('sk-realkey')).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/common/mask-secret.util.spec.ts`
Expected: FAIL (module not found)

- [ ] **Step 3: Implement the util**

```typescript
// backend/src/common/mask-secret.util.ts
// U+2026 HORIZONTAL ELLIPSIS — the single marker maskSecret emits and the write
// path keys on. Real API keys/secrets are ASCII and never contain it.
export const MASK_MARKER = '…';

// Mirror of Go internal/shared.MaskSecret. Both MUST agree byte-for-byte or the
// Go-vs-Node shadow gate fails. Empty stays empty; <16 code points reveal only
// the marker; longer reveal first3 + marker + last4. Array.from gives
// code-point iteration (not UTF-16 units) to match Go []rune.
export function maskSecret(s: string): string {
  if (!s) return '';
  const r = Array.from(s);
  if (r.length < 16) return MASK_MARKER;
  return r.slice(0, 3).join('') + MASK_MARKER + r.slice(-4).join('');
}

export function containsMaskMarker(s: unknown): boolean {
  return typeof s === 'string' && s.includes(MASK_MARKER);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/common/mask-secret.util.spec.ts`
Expected: PASS

- [ ] **Step 5: Implement `maskProvider` (returns a masked COPY — never mutate the entity)**

```typescript
// backend/src/ai-providers/mask-provider.util.ts
import { maskSecret } from '../common/mask-secret.util';

// Returns a shallow copy with api_key masked. NEVER mutate the loaded entity —
// the provider-strategy registry reads the real api_key off the same object to
// call the upstream API. Mirrors Go AiProvider.MarshalJSON (masks on serialize).
export function maskProvider<T extends { api_key?: string }>(p: T): T {
  if (!p) return p;
  return { ...p, api_key: maskSecret(p.api_key ?? '') };
}
```

- [ ] **Step 6: Apply in the four controller handlers**

Add import: `import { maskProvider } from '../ai-providers/mask-provider.util';` and `import { containsMaskMarker } from '../common/mask-secret.util';`

`listProviders` (line ~555):
```typescript
  @Get('ai-providers')
  async listProviders() { return (await this.aiProvidersService.findAll()).map(maskProvider); }
```
`listProvidersPaginated` (line ~549) — mask each row of the `ListResult`:
```typescript
  @Get('ai-providers/paginated')
  async listProvidersPaginated(@Query() q: Record<string, unknown>) {
    const res = await this.aiProvidersService.findAllPaginated(parseListQuery(q));
    return { ...res, rows: res.rows.map(maskProvider) };
  }
```
(Confirm the `ListResult` field name — `rows` vs `data` — by reading `src/common/list-query.ts`; match the Go `paginatedResponse{Rows,Total}` which serializes `rows`/`total`. Use the actual TS field name.)
`createProvider` (line ~557) — guard then mask echo:
```typescript
  @Post('ai-providers')
  async createProvider(@Body() body: { name: string; type: string; endpoint_url: string; api_key?: string; model: string; temperature?: number }) {
    if (containsMaskMarker(body.api_key)) delete body.api_key;
    return maskProvider(await this.aiProvidersService.create(body as any));
  }
```
`updateProvider` (line ~562):
```typescript
  @Patch('ai-providers/:id')
  async updateProvider(@Param('id') id: string, @Body() body: Partial<{ name: string; type: string; endpoint_url: string; api_key: string; model: string; temperature: number; is_default: boolean; is_active: boolean }>) {
    if (containsMaskMarker(body.api_key)) delete body.api_key;
    return maskProvider(await this.aiProvidersService.update(id, body as any));
  }
```

- [ ] **Step 7: Build + commit**

Run: `cd backend && npx tsc --noEmit && npx jest src/common/mask-secret.util.spec.ts`
Expected: tsc clean, tests PASS.

```bash
git add backend/src/common/mask-secret.util.ts backend/src/common/mask-secret.util.spec.ts backend/src/ai-providers/mask-provider.util.ts backend/src/admin/admin.controller.ts
git commit -m "fix(node/admin): mask ai_provider api_key + drop masked echo on write (#29)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## ISSUE #30 — Mask app_settings secrets (read echo + write guard) + portal

### Task 6: Go — mask 11 secret fields in `AppSettings` JSON

**Files:**
- Modify: `backend-rewrite-go/internal/appsettings/domain.go` (`MarshalJSON`, 11 field assignments)
- Test: `backend-rewrite-go/internal/appsettings/domain_test.go` (create/extend)

The 11 masked fields (Apple `apple_team_id`/`apple_key_id` are IDs → NOT masked; `*_mode`, `*_client_id`, `*_partner_code`, `vietqr_*`, `email_from`, `lemonsqueezy_store_id`/`_variant_*` are NOT secrets):
`stripe_secret_key`, `stripe_webhook_secret`, `paypal_client_secret`, `momo_access_key`, `momo_secret_key`, `casso_api_key`, `sepay_api_key`, `resend_api_key`, `google_client_secret`, `lemonsqueezy_api_key`, `lemonsqueezy_webhook_secret`.

- [ ] **Step 1: Write the failing test**

```go
// backend-rewrite-go/internal/appsettings/domain_test.go
package appsettings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAppSettings_MasksSecrets(t *testing.T) {
	s := AppSettings{
		StripeSecretKey: "sk_live_abcdefghijklmnop", ResendAPIKey: "re_abcdefghijklmnop",
		GoogleClientSecret: "gocspx-abcdefghijklmnop", AppleTeamID: "ABCDE12345",
		AppleKeyID: "KEY1234567", PaypalClientID: "paypal-public-id-value",
	}
	b, _ := json.Marshal(s)
	body := string(b)
	for _, raw := range []string{"sk_live_abcdefghijklmnop", "re_abcdefghijklmnop", "gocspx-abcdefghijklmnop"} {
		if strings.Contains(body, raw) {
			t.Errorf("leaks secret %q", raw)
		}
	}
	// Apple IDs and public client IDs are NOT masked.
	if !strings.Contains(body, "ABCDE12345") || !strings.Contains(body, "paypal-public-id-value") {
		t.Errorf("non-secret identifier wrongly masked: %s", body)
	}
	if !strings.Contains(body, `"stripe_secret_key":"sk_…mnop"`) {
		t.Errorf("expected masked stripe key: %s", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/appsettings/ -run TestAppSettings_MasksSecrets -v`
Expected: FAIL

- [ ] **Step 3: Mask the 11 fields in `MarshalJSON`**

In `domain.go` `MarshalJSON`, wrap exactly these 11 struct-literal assignments with `shared.MaskSecret(...)` (leave all other fields untouched):
```go
		StripeSecretKey:            shared.MaskSecret(s.StripeSecretKey),
		StripeWebhookSecret:        shared.MaskSecret(s.StripeWebhookSecret),
		PaypalClientSecret:         shared.MaskSecret(s.PaypalClientSecret),
		MomoAccessKey:              shared.MaskSecret(s.MomoAccessKey),
		MomoSecretKey:              shared.MaskSecret(s.MomoSecretKey),
		CassoAPIKey:                shared.MaskSecret(s.CassoAPIKey),
		SepayAPIKey:                shared.MaskSecret(s.SepayAPIKey),
		ResendAPIKey:               shared.MaskSecret(s.ResendAPIKey),
		GoogleClientSecret:         shared.MaskSecret(s.GoogleClientSecret),
		LemonsqueezyAPIKey:         shared.MaskSecret(s.LemonsqueezyAPIKey),
		LemonsqueezyWebhookSecret:  shared.MaskSecret(s.LemonsqueezyWebhookSecret),
```
Confirm `shared` is imported (it is — `shared.ISOMillis` already used).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/appsettings/ -run TestAppSettings_MasksSecrets -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/appsettings/domain.go backend-rewrite-go/internal/appsettings/domain_test.go
git commit -m "fix(go/appsettings): mask 11 payment/SMTP secret columns in GET/PATCH response (#30)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Go — drop masked secret echoes on settings PATCH

**Files:**
- Modify: `backend-rewrite-go/internal/appsettings/handler.go` (`Patch`, before building `Patch{}`)
- Test: `backend-rewrite-go/internal/appsettings/handler_test.go` (extend)

- [ ] **Step 1: Write the failing test**

```go
func TestPatch_DropsMaskedSecretEchoes(t *testing.T) {
	var captured Patch
	svc := &fakeSettingsSvc{patchFn: func(_ context.Context, p Patch) (AppSettings, error) {
		captured = p
		return AppSettings{}, nil
	}}
	h := NewHandler(svc)
	// stripe = masked echo (must be dropped); resend = real new value (must persist).
	body := `{"stripe_secret_key":"sk_…mnop","resend_api_key":"re_brandnewkey12345","email_from":"x@y.com"}`
	req := httptest.NewRequest("PATCH", "/admin/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Patch(rec, req)
	if captured.StripeSecretKey != nil {
		t.Errorf("masked stripe echo must be dropped")
	}
	if captured.ResendAPIKey == nil || *captured.ResendAPIKey != "re_brandnewkey12345" {
		t.Errorf("real new secret must persist")
	}
	if captured.EmailFrom == nil || *captured.EmailFrom != "x@y.com" {
		t.Errorf("non-secret field must persist")
	}
}
```
(Reuse/extend the existing settings handler fake; if none, add a `patchFn`-backed fake satisfying the handler's service interface.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/appsettings/ -run TestPatch_DropsMaskedSecretEchoes -v`
Expected: FAIL

- [ ] **Step 3: Add the write guard in `Patch`**

In `handler.go` `Patch`, immediately after decoding `body` and before constructing `p := Patch{...}`, insert:
```go
	// Drop masked echoes so a portal re-save can't overwrite the stored secret
	// with its mask (spec §4). Applies to the 11 secret-bearing columns only.
	for _, f := range []**string{
		&body.StripeSecretKey, &body.StripeWebhookSecret, &body.PaypalClientSecret,
		&body.MomoAccessKey, &body.MomoSecretKey, &body.CassoAPIKey, &body.SepayAPIKey,
		&body.ResendAPIKey, &body.GoogleClientSecret, &body.LemonsqueezyAPIKey,
		&body.LemonsqueezyWebhookSecret,
	} {
		if *f != nil && shared.ContainsMaskMarker(**f) {
			*f = nil
		}
	}
```
Add `"github.com/tannpv/draftright-rewrite/internal/shared"` to handler imports if not present. (This relies on `patchBody`'s secret fields being `*string`; confirm from the file — the mapping at `p := Patch{StripeSecretKey: body.StripeSecretKey, ...}` shows they are pointers.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/appsettings/ -run TestPatch_DropsMaskedSecretEchoes -v`
Expected: PASS

- [ ] **Step 5: Module gate + commit**

Run: `cd backend-rewrite-go && gofmt -w internal/appsettings/ && go vet ./internal/appsettings/ && go test ./internal/appsettings/ -race`
Expected: PASS.

```bash
git add backend-rewrite-go/internal/appsettings/handler.go backend-rewrite-go/internal/appsettings/handler_test.go
git commit -m "fix(go/appsettings): drop masked secret echoes on PATCH (#30)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Node — mask settings response + write guard

**Files:**
- Create: `backend/src/admin/mask-settings.util.ts`
- Create: `backend/src/admin/mask-settings.util.spec.ts`
- Modify: `backend/src/admin/admin.controller.ts` (`getSettings` ~432, `updateSettings` ~442)

- [ ] **Step 1: Write the failing test**

```typescript
// backend/src/admin/mask-settings.util.spec.ts
import { maskSettings, stripMaskedSecretsFromBody } from './mask-settings.util';

describe('maskSettings', () => {
  it('masks the 11 secret columns, leaves IDs/modes alone', () => {
    const s: any = {
      stripe_secret_key: 'sk_live_abcdefghijklmnop', resend_api_key: 're_abcdefghijklmnop',
      apple_team_id: 'ABCDE12345', paypal_client_id: 'paypal-public', stripe_mode: 'live',
    };
    const out = maskSettings(s);
    expect(out.stripe_secret_key).toBe('sk_…mnop');
    expect(out.resend_api_key).toBe('re_…mnop');
    expect(out.apple_team_id).toBe('ABCDE12345');
    expect(out.paypal_client_id).toBe('paypal-public');
    expect(out.stripe_mode).toBe('live');
  });
});

describe('stripMaskedSecretsFromBody', () => {
  it('deletes only masked secret keys, keeps real ones + non-secrets', () => {
    const body: any = {
      stripe_secret_key: 'sk_…mnop',           // masked echo → drop
      resend_api_key: 're_brandnewkey12345',   // real → keep
      email_from: 'x@y.com',                   // non-secret → keep
    };
    stripMaskedSecretsFromBody(body);
    expect(body).not.toHaveProperty('stripe_secret_key');
    expect(body.resend_api_key).toBe('re_brandnewkey12345');
    expect(body.email_from).toBe('x@y.com');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/admin/mask-settings.util.spec.ts`
Expected: FAIL (module not found)

- [ ] **Step 3: Implement the util**

```typescript
// backend/src/admin/mask-settings.util.ts
import { maskSecret, containsMaskMarker } from '../common/mask-secret.util';

// The 11 secret-bearing app_settings columns. Apple team/key IDs, public
// client IDs, *_mode, vietqr_*, email_from, lemonsqueezy store/variant are NOT
// secrets. Mirrors the Go set masked in AppSettings.MarshalJSON (spec §3).
export const SETTINGS_SECRET_COLUMNS = [
  'stripe_secret_key', 'stripe_webhook_secret', 'paypal_client_secret',
  'momo_access_key', 'momo_secret_key', 'casso_api_key', 'sepay_api_key',
  'resend_api_key', 'google_client_secret', 'lemonsqueezy_api_key',
  'lemonsqueezy_webhook_secret',
] as const;

// Returns a masked COPY for the response. Never mutate the entity (payment
// strategies read the real keys off the loaded settings object).
export function maskSettings<T extends Record<string, any>>(s: T): T {
  if (!s) return s;
  const copy: Record<string, any> = { ...s };
  for (const k of SETTINGS_SECRET_COLUMNS) {
    if (k in copy) copy[k] = maskSecret(copy[k] ?? '');
  }
  return copy as T;
}

// Mutates the inbound PATCH body in place: drops any secret key whose value is
// a masked echo, so a portal re-save can't overwrite the stored secret.
export function stripMaskedSecretsFromBody(body: Record<string, any>): void {
  if (!body) return;
  for (const k of SETTINGS_SECRET_COLUMNS) {
    if (k in body && containsMaskMarker(body[k])) delete body[k];
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/admin/mask-settings.util.spec.ts`
Expected: PASS

- [ ] **Step 5: Apply in the controller**

Add import: `import { maskSettings, stripMaskedSecretsFromBody } from './mask-settings.util';`

`getSettings` — change `return settings;` (line ~439) to `return maskSettings(settings);`

`updateSettings` — add the guard right after the method opens (before the `payment_methods_enabled` block, line ~444) :
```typescript
    stripMaskedSecretsFromBody(body);
```
and change the final `return this.settingsRepo.findOne({ where: { id: settings.id } });` (line ~456) to:
```typescript
    return maskSettings(await this.settingsRepo.findOne({ where: { id: settings.id } }));
```

- [ ] **Step 6: Build + commit**

Run: `cd backend && npx tsc --noEmit && npx jest src/admin/mask-settings.util.spec.ts`
Expected: tsc clean, tests PASS.

```bash
git add backend/src/admin/mask-settings.util.ts backend/src/admin/mask-settings.util.spec.ts backend/src/admin/admin.controller.ts
git commit -m "fix(node/admin): mask app_settings secrets + drop masked echoes on PATCH (#30)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Portal — SettingsPage sends only edited secrets

**Files:**
- Modify: `admin/src/pages/SettingsPage.tsx`

Context: `saveSettings` (line ~170) PATCHes the entire `settings` object; masked secrets would round-trip and (absent the backend guard) overwrite the real value. Backend guard now protects this, but we also stop sending untouched secrets for clean UX and defense in depth.

- [ ] **Step 1: Track which secret fields the admin edited**

After the `settings` state declaration, add a dirty-set state:
```tsx
const [dirtySecrets, setDirtySecrets] = useState<Set<string>>(new Set());
```
Define the secret key list (mirror backend `SETTINGS_SECRET_COLUMNS`):
```tsx
const SECRET_KEYS = [
  'stripe_secret_key', 'stripe_webhook_secret', 'paypal_client_secret',
  'momo_access_key', 'momo_secret_key', 'casso_api_key', 'sepay_api_key',
  'resend_api_key', 'google_client_secret', 'lemonsqueezy_api_key',
  'lemonsqueezy_webhook_secret',
];
```

- [ ] **Step 2: Mark a secret dirty when its input changes**

Change the `set` helper (line ~177) so editing a secret records it as dirty:
```tsx
const set = (key: string) => (val: string | number | boolean) => {
  if (SECRET_KEYS.includes(key)) {
    setDirtySecrets((prev) => new Set(prev).add(key));
  }
  setSettings({ ...settings, [key]: val });
};
```

- [ ] **Step 3: Strip untouched secrets from the PATCH body**

In `saveSettings` (line ~167), build the payload from a copy minus untouched secrets:
```tsx
const payload: Record<string, unknown> = { ...settings };
for (const k of SECRET_KEYS) {
  if (!dirtySecrets.has(k)) delete payload[k];
}
await apiFetch('/admin/settings', { method: 'PATCH', body: JSON.stringify(payload) });
setDirtySecrets(new Set());
```

- [ ] **Step 4: Reset dirty tracking on (re)load**

In the GET effect that calls `setSettings({ ...defaults, ...data })` (line ~162), also reset: `setDirtySecrets(new Set());`

- [ ] **Step 5: Type-check + build + commit**

Run: `cd admin && npx tsc --noEmit && npm run build`
Expected: tsc clean, build OK.

```bash
git add admin/src/pages/SettingsPage.tsx
git commit -m "fix(admin): SettingsPage sends only edited secrets (masked values never round-trip) (#30)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## ISSUE #32 — Admin self/last-admin delete guard

### Task 10: Node — guard `DELETE /admin/admin-users/:id`

**Files:**
- Modify: `backend/src/admin/admin.controller.ts` (`deleteAdminUser` ~775–779; add `@Request()` import from `@nestjs/common`)
- Test: `backend/src/admin/admin.controller.spec.ts` (create/extend a focused unit spec for this handler)

Guard order (Node is the parity authority — these exact strings + 400 are what Go mirrors):
1. self-delete → `BadRequestException('You cannot deactivate your own admin account')`
2. last-active-admin → `BadRequestException('Cannot deactivate the last active admin')`

- [ ] **Step 1: Write the failing test**

```typescript
// in backend/src/admin/admin.controller.spec.ts (mock adminUserRepo)
describe('deleteAdminUser guards (#32)', () => {
  const acting = { user: { id: 'admin-self' } } as any;
  function makeController(repo: any) {
    const c: any = new AdminController(/* ...other deps as mocks... */);
    c.adminUserRepo = repo;
    return c;
  }
  it('rejects self-deletion with 400', async () => {
    const repo = { findOne: jest.fn(), count: jest.fn(), update: jest.fn() };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('admin-self', acting)).rejects.toThrow('You cannot deactivate your own admin account');
    expect(repo.update).not.toHaveBeenCalled();
  });
  it('rejects deleting the last active admin with 400', async () => {
    const repo = {
      findOne: jest.fn().mockResolvedValue({ id: 'other', is_active: true }),
      count: jest.fn().mockResolvedValue(1),
      update: jest.fn(),
    };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('other', acting)).rejects.toThrow('Cannot deactivate the last active admin');
    expect(repo.update).not.toHaveBeenCalled();
  });
  it('allows a normal delete (other admins remain active)', async () => {
    const repo = {
      findOne: jest.fn().mockResolvedValue({ id: 'other', is_active: true }),
      count: jest.fn().mockResolvedValue(3),
      update: jest.fn().mockResolvedValue({}),
    };
    const c = makeController(repo);
    await expect(c.deleteAdminUser('other', acting)).resolves.toEqual({ success: true });
    expect(repo.update).toHaveBeenCalledWith('other', { is_active: false });
  });
});
```
(Construct the controller with the constructor's mocks; only `adminUserRepo` matters here. If direct construction is awkward, instantiate a partial via `Object.create(AdminController.prototype)` and assign `adminUserRepo`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && npx jest src/admin/admin.controller.spec.ts -t "deleteAdminUser guards"`
Expected: FAIL (no guard; current handler takes only `id`)

- [ ] **Step 3: Implement the guard**

Ensure `Request` is imported from `@nestjs/common` (the file already imports `BadRequestException`). Replace `deleteAdminUser` (lines ~775–779):
```typescript
  @Delete('admin-users/:id')
  async deleteAdminUser(@Param('id') id: string, @Request() req: any) {
    if (id === req.user.id) {
      throw new BadRequestException('You cannot deactivate your own admin account');
    }
    const target = await this.adminUserRepo.findOne({ where: { id } });
    if (target && target.is_active) {
      const activeCount = await this.adminUserRepo.count({ where: { is_active: true } });
      if (activeCount <= 1) {
        throw new BadRequestException('Cannot deactivate the last active admin');
      }
    }
    await this.adminUserRepo.update(id, { is_active: false });
    return { success: true };
  }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && npx jest src/admin/admin.controller.spec.ts -t "deleteAdminUser guards"`
Expected: PASS

- [ ] **Step 5: Build + commit**

Run: `cd backend && npx tsc --noEmit && npx jest src/admin/admin.controller.spec.ts`
Expected: tsc clean, tests PASS.

```bash
git add backend/src/admin/admin.controller.ts backend/src/admin/admin.controller.spec.ts
git commit -m "fix(node/admin): guard admin self/last-admin soft-delete (400) (#32)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Go — mirror the delete guard

**Files:**
- Modify: `backend-rewrite-go/internal/shared/pg/queries_*.sql` (add `CountActiveAdminUsers`; run `sqlc generate`)
- Modify: `backend-rewrite-go/internal/adminauth/` service + repo (add `GetByID`/`CountActiveAdmins` if absent), `admin_users_handler.go` (`Delete`)
- Test: `backend-rewrite-go/internal/adminauth/admin_users_handler_test.go` (extend)

The Go 400 must equal Node byte-for-byte. Node `BadRequestException(msg)` → `AllExceptionsFilter` → 400, `code: 'invalid-input'`. So Go emits `shared.WriteError(w, r, "invalid-input", msg)` with the identical message strings. Confirm `shared.StatusForCode("invalid-input") == 400` (read `errenvelope.go`); if the code name differs, use whatever Node's filter produces for a `BadRequestException` (match the existing Go convention already used for 400s, e.g. the ai-providers/appsettings handlers' `"invalid-input"`).

- [ ] **Step 1: Write the failing test**

```go
func TestDelete_SelfAndLastAdminGuards(t *testing.T) {
	// self-delete → 400, no mutation
	{
		svc := &fakeAdminUsersSvc{ /* SoftDelete should NOT be called */ }
		h := NewAdminUsersHandler(svc)
		req := withAdminClaims(httptest.NewRequest("DELETE", "/admin/admin-users/admin-self", nil), "admin-self")
		req = withURLParam(req, "id", "admin-self")
		rec := httptest.NewRecorder()
		h.Delete(rec, req)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "You cannot deactivate your own admin account") {
			t.Errorf("self-delete: got %d %s", rec.Code, rec.Body.String())
		}
		if svc.softDeleteCalled {
			t.Error("self-delete must not mutate")
		}
	}
	// last-active-admin → 400
	{
		svc := &fakeAdminUsersSvc{targetActive: true, activeCount: 1}
		h := NewAdminUsersHandler(svc)
		req := withAdminClaims(httptest.NewRequest("DELETE", "/admin/admin-users/other", nil), "admin-self")
		req = withURLParam(req, "id", "other")
		rec := httptest.NewRecorder()
		h.Delete(rec, req)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "Cannot deactivate the last active admin") {
			t.Errorf("last-admin: got %d %s", rec.Code, rec.Body.String())
		}
	}
	// normal delete → 200
	{
		svc := &fakeAdminUsersSvc{targetActive: true, activeCount: 3}
		h := NewAdminUsersHandler(svc)
		req := withAdminClaims(httptest.NewRequest("DELETE", "/admin/admin-users/other", nil), "admin-self")
		req = withURLParam(req, "id", "other")
		rec := httptest.NewRecorder()
		h.Delete(rec, req)
		if rec.Code != 200 || !svc.softDeleteCalled {
			t.Errorf("normal delete: got %d, softDeleteCalled=%v", rec.Code, svc.softDeleteCalled)
		}
	}
}
```
(Extend the existing `fakeAdminUsersSvc` with `targetActive bool`, `activeCount int`, `softDeleteCalled bool`, and the new service methods `IsActiveAdmin(ctx,id)(bool,error)` + `CountActiveAdmins(ctx)(int,error)`. `withAdminClaims` must stamp `*auth.Claims{Sub: actingID}` into the request context via the same `claimsKey` the real middleware uses — reuse the helper from `internal/shared/admin_middleware_test.go` if present, else add one.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestDelete_SelfAndLastAdminGuards -v`
Expected: FAIL

- [ ] **Step 3: Add the count query + regen sqlc**

Add to the admin-users SQL file (e.g. `internal/shared/pg/queries_adminusers.sql`):
```sql
-- name: CountActiveAdminUsers :one
SELECT COUNT(*) FROM admin_users WHERE is_active = true;
```
Confirm a by-id lookup exists (for `targetActive`); if not, add:
```sql
-- name: GetAdminUserByID :one
SELECT id, email, password_hash, name, is_active, role, created_at, updated_at
FROM admin_users WHERE id = $1;
```
Run: `cd backend-rewrite-go && sqlc generate` and verify no unrelated drift (`git diff --stat internal/shared/pg/sqlc`).

- [ ] **Step 4: Add service methods**

In the adminauth service, add:
```go
// CountActiveAdmins returns the number of is_active=true admin_users.
func (s *AdminUsersService) CountActiveAdmins(ctx context.Context) (int, error) {
	n, err := s.q.CountActiveAdminUsers(ctx)
	return int(n), err
}

// IsActiveAdmin reports whether the admin id exists and is currently active.
// A missing/malformed id returns (false, nil) — the guard then no-ops like Node
// (target null → falls through to the update, which 0-row no-ops).
func (s *AdminUsersService) IsActiveAdmin(ctx context.Context, id string) (bool, error) {
	row, err := s.repo.GetByID(ctx, id) // add a thin repo.GetByID if absent
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.IsActive, nil
}
```
(Match the module's existing repo/querier wiring; add `GetByID` to the repo + its querier interface if it does not already exist. Keep the consumer-side interface in the handler small — only the methods `Delete` needs.)

- [ ] **Step 5: Implement the handler guard**

Replace `Delete` in `admin_users_handler.go`:
```go
// Delete handles DELETE /admin/admin-users/:id → 200 { success:true }, with
// two guards mirroring Node (parity authority): an admin cannot deactivate
// their own account or the last active admin → 400 invalid-input.
func (h *AdminUsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	if id == claims.Sub {
		shared.WriteError(w, r, "invalid-input", "You cannot deactivate your own admin account")
		return
	}
	active, err := h.svc.IsActiveAdmin(r.Context(), id)
	if err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	if active {
		count, err := h.svc.CountActiveAdmins(r.Context())
		if err != nil {
			shared.WriteError(w, r, "internal", "admin-users failed")
			return
		}
		if count <= 1 {
			shared.WriteError(w, r, "invalid-input", "Cannot deactivate the last active admin")
			return
		}
	}
	if err := h.svc.SoftDelete(r.Context(), id); err != nil {
		shared.WriteError(w, r, "internal", "admin-users failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, struct {
		Success bool `json:"success"`
	}{true})
}
```
Add the handler's consumer-side service interface methods (`IsActiveAdmin`, `CountActiveAdmins`) alongside the existing `SoftDelete`. Import `shared` if not already.

- [ ] **Step 6: Run test to verify it passes**

Run: `cd backend-rewrite-go && go test ./internal/adminauth/ -run TestDelete_SelfAndLastAdminGuards -v`
Expected: PASS

- [ ] **Step 7: Module gate + commit**

Run: `cd backend-rewrite-go && gofmt -w internal/adminauth/ internal/shared/pg/ && go vet ./internal/adminauth/ && go test ./internal/adminauth/ -race`
Expected: PASS.

```bash
git add backend-rewrite-go/internal/adminauth/ backend-rewrite-go/internal/shared/pg/
git commit -m "fix(go/adminauth): mirror admin self/last-admin delete guard (400) (#32)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Shadow fixture — self-delete returns 400 on both backends

**Files:**
- Modify: `backend-rewrite-go/cmd/shadowdiff/bootstrap.go` (expose `{{admin_id}}`)
- Create: `backend-rewrite-go/cmd/shadowdiff/fixtures/admin_crud/admin_users_delete_self.json`

The self-delete fixture needs the acting admin's own id in the path. The harness substitutes `{{admin_token}}`; add `{{admin_id}}` for its subject. The last-admin case needs awkward single-admin DB seeding and is covered by unit tests on both backends (Tasks 10–11) — do NOT attempt a fixture for it; note this in the commit body.

- [ ] **Step 1: Add `{{admin_id}}` substitution in bootstrap**

In `bootstrap.go` `bootstrapTokens`, after the admin login obtains the admin token, also record the admin's id. Decode the JWT `sub` claim from the admin access token (the harness already has the token string) or read it from the admin login response body if it returns the admin object. Set `tokens["admin_id"] = <id>`. Match the existing map/struct the harness uses for `{{...}}` substitution (same place `admin_token` is registered).

Run to confirm wiring compiles: `cd backend-rewrite-go && go build ./cmd/shadowdiff/`
Expected: builds clean.

- [ ] **Step 2: Author the fixture**

```json
{
  "name": "admin_crud_admin_users_delete_self",
  "method": "DELETE",
  "path": "/admin/admin-users/{{admin_id}}",
  "headers": { "Authorization": "Bearer {{admin_token}}" },
  "ignore_value_of": ["request_id"]
}
```
Both backends must return 400 with `"You cannot deactivate your own admin account"` → byte-identical → PASS. The shadowdiff diff compares status + body; the message is identical on both sides.

- [ ] **Step 3: Verify the existing delete fixture still passes**

The existing `admin_users_delete.json` targets a non-acting placeholder id; with multiple seeded admins it stays 200. If the frozen template seeds only ONE admin, that placeholder id will not exist → `target` null → guard no-ops → still 200 on both sides (Node: `findOne` null → falls through; Go: `IsActiveAdmin` false → falls through). Confirm by reading `deploy/shadow/` seed/template setup; if the template seeds exactly one admin AND the placeholder equals that admin, adjust the fixture id to a non-existent uuid so it stays a 200 no-op. Document the template's admin seed count in the commit body.

- [ ] **Step 4: Commit**

```bash
git add backend-rewrite-go/cmd/shadowdiff/bootstrap.go backend-rewrite-go/cmd/shadowdiff/fixtures/admin_crud/admin_users_delete_self.json
git commit -m "test(shadow): {{admin_id}} substitution + self-delete 400 parity fixture (#32)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## ISSUE-WIDE — Full verification

### Task 13: Whole-suite gate + live shadow gate

- [ ] **Step 1: Go full suite**

Run: `cd backend-rewrite-go && gofmt -l . && go vet ./... && go test ./... -race && sqlc generate && git diff --exit-code internal/shared/pg/sqlc`
Expected: gofmt prints nothing; vet clean; tests PASS; no sqlc drift.

- [ ] **Step 2: Node build + targeted tests**

Run: `cd backend && npx tsc --noEmit && npx jest src/common/mask-secret.util.spec.ts src/users/sanitize-user.util.spec.ts src/admin/mask-settings.util.spec.ts src/admin/admin.controller.spec.ts`
Expected: tsc clean; all PASS.

- [ ] **Step 3: Portal build**

Run: `cd admin && npx tsc --noEmit && npm run build`
Expected: clean.

- [ ] **Step 4: Live shadow gate (the parity proof)**

Run: `cd backend-rewrite-go && ./deploy/shadow/run-gate.sh`
Expected: every fixture PASS, including:
- masked-secret routes (`ai_providers_list/create/update`, `settings_get/patch`) — byte-identical masks on both sides → PASS
- `users_get` — both backends drop the 6 columns → PASS
- `admin_users_delete` — still 200 → PASS
- `admin_users_delete_self` — 400 with the identical self-delete message on both sides → PASS

If any masked route FAILS, the two `maskSecret` implementations diverged (threshold, marker, or indexing) — diff the offending field's value Node vs Go and reconcile. If `users_get` FAILS, Node and Go dropped a different set of columns — align the lists.

- [ ] **Step 5: Final review + finish**

Dispatch a final whole-branch review, then invoke `superpowers:finishing-a-development-branch`. Branch is NOT pushed; per workflow, push/merge only when the user asks.

---

## Self-Review (author check against spec)

**Spec coverage:**
- §3 mask function → Tasks 3 (Go) + 5 Step 3 (Node). ✓
- §3 #29 api_key mask (read + echo) → Tasks 4, 5. ✓
- §3 #30 11 settings secrets (read + echo) → Tasks 6, 8. ✓
- §3 #31 drop 6 user columns → Tasks 1, 2. ✓
- §4 write guard (marker-based) → Tasks 4 (Go provider), 7 (Go settings), 5/8 (Node). ✓
- §5 portal SettingsPage dirty-tracking; Providers/AdminUsers untouched → Task 9 (+ explicit no-change note). ✓
- §6 #32 self + last-admin guards, Node-defines-message → Tasks 10 (Node), 11 (Go). ✓
- §6 2 fixtures → Task 12 authors the self-delete fixture; last-admin covered by unit tests both sides (deviation documented: fixture needs awkward single-admin seeding). ✓
- §7 tests + gate → Task 13. ✓

**Placeholder scan:** mask threshold (16), marker (U+2026), field lists, exact messages, exact file lines all concrete. The few "confirm X in the file" steps (ListResult field name, patchBody pointer-ness, template admin seed count, claims helper name) are verification steps with a stated default, not deferred logic.

**Type consistency:** `maskSecret`/`MaskSecret`, `containsMaskMarker`/`ContainsMaskMarker`, `MASK_MARKER`/`MaskMarker`, `stripUserSecrets`, `maskProvider`, `maskSettings`/`stripMaskedSecretsFromBody`, `SETTINGS_SECRET_COLUMNS` used consistently across tasks. 11-secret list identical in Go (Task 6) and Node (Task 8) and portal (Task 9).

**Deviation from spec:** spec §6 said "add 2 new fixtures"; plan adds 1 (self-delete) + unit-test coverage for last-admin, because the last-admin fixture requires single-active-admin DB seeding the frozen template does not provide. Documented in Task 12.
