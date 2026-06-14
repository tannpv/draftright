# Go Backend Phase 3a — Payment Core (read endpoints + foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the byte-deterministic, provider-secret-free slice of the NestJS payment subsystem to Go — the `payments` persistence layer, the payment-method/status enums, the reference-code generator, the store-type mapping, the enabled-methods registry, and the three read endpoints (`GET /payment/methods`, `GET /payment/status/:ref`, `GET /payment/history`).

**Architecture:** New flat Clean-Architecture module `internal/payment/` mirroring the established `subscription`/`exttoken`/`plans` pattern (domain → usecase → repo_pg → handler, ports on the consumer side, sqlc for SQL). NO concrete payment strategies, NO provider HTTP calls, NO webhooks, NO checkout-creation here — those land in Phase 3b/3c/3d. This phase ships the pure, shadow-gate-comparable foundation everything else plugs into. The strategy *interface* and the registered-method *key set* are defined now (so `GET /payment/methods` is byte-identical from day one); the concrete strategies fill them in 3b.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + pgxpool, sqlc v1.31, crypto/rand, slog. Same toolchain as Phases 0–2.

---

## Phase 3 decomposition (context for the worker)

Phase 3 (Payment, highest risk) is split into four shippable sub-plans. **This document is 3a only.**

| Sub-plan | Scope | Byte-comparable? |
|---|---|---|
| **3a (this)** | `payments` table repo, enums, reference gen, store-type map, enabled-methods registry, Strategy port interface, `GET /payment/{methods,status/:ref,history}` | ✅ fully (no provider nondeterminism) |
| 3b | `POST /payment/checkout` + 3 concrete strategies (Stripe, LemonSqueezy, VietQR/bank-transfer) + `CheckoutResult` wiring | partial (redirect URLs/QR vary) |
| 3c | 5 webhook routes + signature verification + idempotency + `handleWebhook` dispatch + `completePayment`/`activateSubscription` | the 401/verify paths only |
| 3d | `GET /payment/portal`, `DELETE /payment/subscription`, admin payment mgmt (`/admin/payments*`), `expireStalePayments` cron | mixed |

## Reference: Node source being ported (read these before starting)

- `backend/src/payment/entities/payment.entity.ts` — `Payment` entity, `PaymentMethod` + `PaymentStatus` enums, table `payments`, indexes `(user_id, created_at)`, `(status, created_at)`.
- `backend/src/payment/payment-reference.ts` — `PAYMENT_REF_PREFIX='DR-PRO-'`, `PAYMENT_REF_REGEX=/DR-[A-Z]+-[A-Z0-9]+/`, `generatePaymentReference()` (`randomBytes(4).hex().toUpperCase()`), `extractPaymentReference()`.
- `backend/src/payment/store-type-mapping.ts` — `storeTypeForMethod()` switch.
- `backend/src/payment/payment.service.ts:73-114` — `getEnabledMethods()`, `registeredMethods()`, `assertMethodsRegisterable()`, `assertEnabled()`.
- `backend/src/payment/payment.service.ts:575-580,645-651` — `getStatus()`, `findByUser()`.
- `backend/src/payment/payment.controller.ts:17-61` — `GET /payment/methods`, `GET /payment/status/:ref`, `GET /payment/history` response shapes.

## Parity invariants specific to this phase

1. **Enabled-methods ordering is insertion-order.** Node builds `new Set(csv.split(',')…)` then `[...set]`; a JS `Set` preserves first-insertion order. `bank_transfer` is appended **only if** `vietqr` is present **and** `bank_transfer` wasn't already in the CSV. Go MUST replicate exactly — use an ordered set (slice + seen-map), never a Go `map` (random order) for the output.
2. **`getStatus` not-found returns `{"status":"not_found"}`** (200, not 404). Found returns a hand-built object with `plan_name` from the joined plan and `completed_at`/`expires_at` as ms-precision ISO or `null`.
3. **`getEnabledMethods` reads fresh each call** (admin toggles take effect without restart) — no caching.
4. **Default method is `stripe`** when neither `app_settings.payment_methods_enabled` nor env `PAYMENT_ENABLED_METHODS` is set.
5. **Registered methods set** (what `registeredMethods()`/`assertMethodsRegisterable` validate against) = exactly `{stripe, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay}` — the keys of Node's strategy `Map`. `paypal` and `momo` are enum values with NO strategy and must NOT be in the registered set.
6. **Timestamps** use `shared.ISOMillis` (Node `Date.toISOString()`, ms precision, `Z`).
7. **Money is raw integer minor units** (`amount` = VND đồng or USD cents), never divided.
8. Error envelope via `shared.WriteError(w, r, code, message)`; success via `shared.WriteJSON(w, status, body)`. Codes: `not-found`→404, `invalid-input`→400, default→500 (see `internal/shared/errenvelope.go`).

## File structure (what this phase creates/modifies)

- Create: `internal/payment/domain.go` — `PaymentMethod`/`PaymentStatus` string consts, `StoreType` consts, `StoreTypeForMethod`, `RegisteredMethods`, the `Strategy` port interface + `CheckoutResult`/`WebhookAction` type definitions (defined now, implemented in 3b).
- Create: `internal/payment/reference.go` — reference generate/extract + consts.
- Create: `internal/payment/methods.go` — `EnabledMethods` (ordered set), `AssertMethodsRegisterable`.
- Create: `internal/payment/reader.go` — sqlc `Querier` port + `Repo` (`GetByReference`, `ListByUser`) + projections.
- Create: `internal/payment/usecase.go` — `Service` (methods/status/history use cases) + consumer-side ports.
- Create: `internal/payment/handler.go` — `Methods`, `Status`, `History` HTTP edges.
- Create: `internal/shared/pg/queries_payment.sql` — sqlc source (then `sqlc generate`).
- Modify: `internal/shared/router.go` — add `PaymentMethods`, `PaymentStatus`, `PaymentHistory` fields + mounts.
- Modify: `cmd/server/main.go` — wire repo→service→handler, add `coreHandlers` fields + router assignments.
- Create (tests): `internal/payment/*_test.go` beside each file (no DB; fake the ports).

---

### Task 1: PaymentMethod / PaymentStatus / StoreType enums + StoreTypeForMethod

**Files:**
- Create: `internal/payment/domain.go`
- Test: `internal/payment/domain_test.go`

- [ ] **Step 1: Write the failing test**

```go
package payment

import "testing"

func TestStoreTypeForMethod(t *testing.T) {
	cases := map[PaymentMethod]StoreType{
		MethodStripe:       StoreStripe,
		MethodLemonSqueezy: StoreLemonSqueezy,
		MethodVietQR:       StoreVietQR,
		MethodBankTransfer: StoreBankTransfer,
		MethodPayPal:       StorePayPal,
		MethodApplePay:     StoreStripe, // wallets run on Stripe
		MethodGooglePay:    StoreStripe,
		MethodMoMo:         StoreAdminGranted,
		PaymentMethod("???"): StoreAdminGranted, // default arm
	}
	for in, want := range cases {
		if got := StoreTypeForMethod(in); got != want {
			t.Errorf("StoreTypeForMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnumWireValues(t *testing.T) {
	// Byte-parity: the string values must equal the Node enum values exactly.
	pairs := []struct{ got, want string }{
		{string(MethodStripe), "stripe"},
		{string(MethodPayPal), "paypal"},
		{string(MethodMoMo), "momo"},
		{string(MethodVietQR), "vietqr"},
		{string(MethodBankTransfer), "bank_transfer"},
		{string(MethodLemonSqueezy), "lemonsqueezy"},
		{string(MethodApplePay), "apple_pay"},
		{string(MethodGooglePay), "google_pay"},
		{string(StatusPending), "pending"},
		{string(StatusCompleted), "completed"},
		{string(StatusFailed), "failed"},
		{string(StatusExpired), "expired"},
		{string(StatusRefunded), "refunded"},
		{string(StoreGooglePlay), "google_play"},
		{string(StoreAppleIAP), "apple_iap"},
		{string(StoreAdminGranted), "admin_granted"},
		{string(StoreLemonSqueezy), "lemonsqueezy"},
		{string(StoreStripe), "stripe"},
		{string(StoreVietQR), "vietqr"},
		{string(StoreBankTransfer), "bank_transfer"},
		{string(StorePayPal), "paypal"},
	}
	for _, p := range pairs {
		if p.got != p.want {
			t.Errorf("wire value = %q, want %q", p.got, p.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'TestStoreTypeForMethod|TestEnumWireValues' -v`
Expected: FAIL — `undefined: PaymentMethod` (package doesn't compile yet).

- [ ] **Step 3: Write minimal implementation**

```go
// Package payment ports the NestJS payment subsystem. Phase 3a is the
// byte-deterministic foundation: enums, reference codes, store-type
// mapping, enabled-methods registry, and the read endpoints. Concrete
// strategies, checkout, and webhooks land in Phases 3b/3c.
package payment

// PaymentMethod mirrors backend/src/payment/entities/payment.entity.ts
// PaymentMethod. The string values are the DB/wire names — keep them
// byte-identical to Node (they appear in payments.method and in
// GET /payment/methods).
type PaymentMethod string

const (
	MethodStripe       PaymentMethod = "stripe"
	MethodPayPal       PaymentMethod = "paypal"
	MethodMoMo         PaymentMethod = "momo"
	MethodVietQR       PaymentMethod = "vietqr"
	MethodBankTransfer PaymentMethod = "bank_transfer"
	MethodLemonSqueezy PaymentMethod = "lemonsqueezy"
	MethodApplePay     PaymentMethod = "apple_pay"
	MethodGooglePay    PaymentMethod = "google_pay"
)

// PaymentStatus mirrors the Node PaymentStatus enum.
type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusCompleted PaymentStatus = "completed"
	StatusFailed    PaymentStatus = "failed"
	StatusExpired   PaymentStatus = "expired"
	StatusRefunded  PaymentStatus = "refunded"
)

// StoreType mirrors backend/src/subscriptions/entities/subscription.entity.ts
// StoreType — the audit field stamped on the subscription a payment grants.
type StoreType string

const (
	StoreGooglePlay   StoreType = "google_play"
	StoreAppleIAP     StoreType = "apple_iap"
	StoreAdminGranted StoreType = "admin_granted"
	StoreLemonSqueezy StoreType = "lemonsqueezy"
	StoreStripe       StoreType = "stripe"
	StoreVietQR       StoreType = "vietqr"
	StoreBankTransfer StoreType = "bank_transfer"
	StorePayPal       StoreType = "paypal"
)

// StoreTypeForMethod maps a payment method to the StoreType its resulting
// subscription row carries. Ports store-type-mapping.ts verbatim. Apple/Google
// Pay run on Stripe → StoreStripe; MoMo + any unknown → StoreAdminGranted.
func StoreTypeForMethod(m PaymentMethod) StoreType {
	switch m {
	case MethodStripe:
		return StoreStripe
	case MethodLemonSqueezy:
		return StoreLemonSqueezy
	case MethodVietQR:
		return StoreVietQR
	case MethodBankTransfer:
		return StoreBankTransfer
	case MethodPayPal:
		return StorePayPal
	case MethodApplePay, MethodGooglePay:
		return StoreStripe
	case MethodMoMo:
		return StoreAdminGranted
	default:
		return StoreAdminGranted
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -run 'TestStoreTypeForMethod|TestEnumWireValues' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/payment/domain.go internal/payment/domain_test.go
git commit -m "feat(go-payment): payment method/status/store-type enums + store-type mapping"
```

---

### Task 2: Payment reference generator + extractor

**Files:**
- Create: `internal/payment/reference.go`
- Test: `internal/payment/reference_test.go`

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"regexp"
	"testing"
)

func TestGeneratePaymentReference_Format(t *testing.T) {
	re := regexp.MustCompile(`^DR-PRO-[0-9A-F]{8}$`) // 4 random bytes → 8 upper hex
	for i := 0; i < 50; i++ {
		ref := GeneratePaymentReference()
		if !re.MatchString(ref) {
			t.Fatalf("ref %q does not match DR-PRO-<8 upper hex>", ref)
		}
	}
}

func TestGeneratePaymentReference_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		ref := GeneratePaymentReference()
		if seen[ref] {
			t.Fatalf("duplicate ref %q within 200 draws", ref)
		}
		seen[ref] = true
	}
}

func TestExtractPaymentReference(t *testing.T) {
	cases := map[string]string{
		"Payment for dr-pro-a1b2c3d4 thanks": "DR-PRO-A1B2C3D4", // case-insensitive, upper-normalised
		"NoRefHere":                          "",
		"":                                   "",
		"prefix DR-PRO-DEADBEEF suffix":      "DR-PRO-DEADBEEF",
	}
	for in, want := range cases {
		if got := ExtractPaymentReference(in); got != want {
			t.Errorf("ExtractPaymentReference(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'Reference' -v`
Expected: FAIL — `undefined: GeneratePaymentReference`.

- [ ] **Step 3: Write minimal implementation**

```go
package payment

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// PaymentRefPrefix is the literal prefix on every reference code. Customers
// put the full code in the bank-transfer memo; the SePay/Casso/MB webhook
// (Phase 3c) matches it back. Generator + matcher MUST agree — both live here.
// Ports payment-reference.ts PAYMENT_REF_PREFIX.
const PaymentRefPrefix = "DR-PRO-"

// paymentRefRegex matches any DraftRight reference (DR-<SECTION>-<ALNUM>) in a
// transfer memo. Ports PAYMENT_REF_REGEX. Applied to upper-cased text.
var paymentRefRegex = regexp.MustCompile(`DR-[A-Z]+-[A-Z0-9]+`)

// GeneratePaymentReference returns DR-PRO-XXXXXXXX where XXXXXXXX is 4 random
// bytes as upper-case hex. Ports generatePaymentReference()
// (randomBytes(4).toString('hex').toUpperCase()).
func GeneratePaymentReference() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is catastrophic and unrecoverable; mirror Node,
		// which would throw. Panicking here is correct — a payment without a
		// unique reference must never be created.
		panic("payment: crypto/rand failed: " + err.Error())
	}
	return PaymentRefPrefix + strings.ToUpper(hex.EncodeToString(b[:]))
}

// ExtractPaymentReference pulls the first DraftRight reference out of free-text
// transfer content (case-insensitive). Returns "" when none is present.
// Ports extractPaymentReference().
func ExtractPaymentReference(text string) string {
	return paymentRefRegex.FindString(strings.ToUpper(text))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -run 'Reference' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/payment/reference.go internal/payment/reference_test.go
git commit -m "feat(go-payment): payment reference generate + extract (DR-PRO-XXXXXXXX)"
```

---

### Task 3: Registered methods + enabled-methods ordered-set logic

**Files:**
- Create: `internal/payment/methods.go`
- Test: `internal/payment/methods_test.go`

Node refs: `payment.service.ts` `registeredMethods()` (strategy-map keys), `getEnabledMethods()`, `assertMethodsRegisterable()`. `getEnabledMethods` here takes the raw CSV (settings or env or default) as an argument so the SQL/env read stays in the repo/service layer and this stays pure + unit-testable.

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"reflect"
	"testing"
)

func TestRegisteredMethods(t *testing.T) {
	// Exactly the keys of Node's strategy Map — paypal + momo excluded.
	got := RegisteredMethods()
	want := []string{"stripe", "vietqr", "bank_transfer", "lemonsqueezy", "apple_pay", "google_pay"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RegisteredMethods() = %v, want %v", got, want)
	}
}

func TestEnabledMethods_OrderingAndVietQRImplication(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"default when blank", "", []string{"stripe"}},
		{"single", "lemonsqueezy", []string{"lemonsqueezy"}},
		{"vietqr implies bank_transfer appended last", "vietqr", []string{"vietqr", "bank_transfer"}},
		{"insertion order preserved", "lemonsqueezy,stripe", []string{"lemonsqueezy", "stripe"}},
		{"bank_transfer already present keeps its position", "bank_transfer,vietqr", []string{"bank_transfer", "vietqr"}},
		{"dedupe + trim + lowercase", " Stripe , stripe ,LEMONSQUEEZY", []string{"stripe", "lemonsqueezy"}},
		{"vietqr mid-list still appends bank_transfer at end", "stripe,vietqr,lemonsqueezy", []string{"stripe", "vietqr", "lemonsqueezy", "bank_transfer"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EnabledMethods(c.raw)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("EnabledMethods(%q) = %v, want %v", c.raw, got, c.want)
			}
		})
	}
}

func TestAssertMethodsRegisterable(t *testing.T) {
	if err := AssertMethodsRegisterable("stripe,vietqr"); err != nil {
		t.Fatalf("registerable CSV should pass, got %v", err)
	}
	if err := AssertMethodsRegisterable(""); err != nil {
		t.Fatalf("blank CSV is allowed (falls back to default), got %v", err)
	}
	err := AssertMethodsRegisterable("stripe,paypal,momo")
	if err == nil {
		t.Fatal("paypal+momo have no strategy → must error")
	}
	want := "Cannot enable payment method(s) with no backend strategy: paypal, momo. " +
		"Registered methods: stripe, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay."
	if err.Error() != want {
		t.Fatalf("error =\n%q\nwant\n%q", err.Error(), want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'Methods|Registerable' -v`
Expected: FAIL — `undefined: RegisteredMethods`.

- [ ] **Step 3: Write minimal implementation**

```go
package payment

import (
	"fmt"
	"strings"
)

// DefaultPaymentMethod is used when neither app_settings.payment_methods_enabled
// nor env PAYMENT_ENABLED_METHODS is set. Ports DEFAULT_PAYMENT_METHOD.
const DefaultPaymentMethod = string(MethodStripe)

// registeredMethods is the ordered key set of Node's strategy Map
// (payment.service.ts constructor). These are the methods that actually have a
// backend strategy and can therefore check out. paypal + momo are enum values
// with NO strategy and are deliberately absent. 3b's strategies must cover
// EXACTLY this set; keep it the single source of truth.
var registeredMethods = []string{
	string(MethodStripe),
	string(MethodVietQR),
	string(MethodBankTransfer),
	string(MethodLemonSqueezy),
	string(MethodApplePay),
	string(MethodGooglePay),
}

// RegisteredMethods returns the registerable method keys (a fresh copy so
// callers can't mutate the package state).
func RegisteredMethods() []string {
	out := make([]string, len(registeredMethods))
	copy(out, registeredMethods)
	return out
}

// EnabledMethods resolves the storefront's visible methods from a raw CSV
// (already chosen by the caller: settings ?? env ?? default). Ports
// getEnabledMethods() body: lowercase, split, trim, drop blanks, dedupe in
// first-seen order, then — iff vietqr is present — ensure bank_transfer is in
// the list (appended last only when not already present). Output order is
// byte-identical to Node's `[...set]`.
func EnabledMethods(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultPaymentMethod
	}
	raw = strings.ToLower(raw)
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		m := strings.TrimSpace(part)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if seen[string(MethodVietQR)] && !seen[string(MethodBankTransfer)] {
		seen[string(MethodBankTransfer)] = true
		out = append(out, string(MethodBankTransfer))
	}
	return out
}

// AssertMethodsRegisterable validates a payment_methods_enabled CSV before it
// is saved (Phase 4 admin write will call this): every listed method must have
// a registered strategy. Blank input is allowed (falls back to default).
// Ports assertMethodsRegisterable() including the exact error message.
func AssertMethodsRegisterable(csv string) error {
	registered := map[string]bool{}
	for _, m := range registeredMethods {
		registered[m] = true
	}
	var unknown []string
	for _, part := range strings.Split(strings.ToLower(csv), ",") {
		m := strings.TrimSpace(part)
		if m == "" || registered[m] {
			continue
		}
		unknown = append(unknown, m)
	}
	if len(unknown) > 0 {
		return fmt.Errorf(
			"Cannot enable payment method(s) with no backend strategy: %s. Registered methods: %s.",
			strings.Join(unknown, ", "), strings.Join(registeredMethods, ", "),
		)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -run 'Methods|Registerable' -v`
Expected: PASS.

> **Note on `assertMethodsRegisterable` duplicate handling:** Node does NOT dedupe the `unknown` list, so `"paypal,paypal"` would yield `"paypal, paypal"`. The Go version above also does not dedupe `unknown` (it appends every non-registered occurrence), matching Node. The `registered` lookup is the only set; `unknown` is a plain slice. Verified parity.

- [ ] **Step 5: Commit**

```bash
git add internal/payment/methods.go internal/payment/methods_test.go
git commit -m "feat(go-payment): enabled-methods ordered set + registry + registerable assertion"
```

---

### Task 4: sqlc queries for payments (status + history)

**Files:**
- Create: `internal/shared/pg/queries_payment.sql`
- Modify (generated): `internal/shared/pg/sqlc/*` via `sqlc generate`
- No standalone test (generated code; exercised by Task 5/6 fakes + the live shadow gate).

The `payments` table is owned by the live NestJS migrations — read it as-is, do NOT emit DDL. Columns (from `payment.entity.ts`): `id uuid pk`, `user_id uuid`, `plan_id uuid`, `amount int`, `currency varchar(10)`, `method varchar(20)`, `status varchar(20)`, `provider_ref varchar(255) null`, `reference_code varchar(50) unique`, `qr_data text null`, `notes text null`, `expires_at timestamp null`, `completed_at timestamp null`, `created_at timestamp`, `updated_at timestamp`. Plans table join gives `plan.name`.

- [ ] **Step 1: Write the sqlc source**

```sql
-- internal/shared/pg/queries_payment.sql
-- Payment persistence (read paths for Phase 3a). Table public.payments is
-- owned by the live NestJS-managed schema; mirror PaymentService read
-- semantics exactly. Timestamps are "without time zone".

-- name: GetPaymentByReference :one
-- getStatus(referenceCode): the pending/settled payment plus the plan name the
-- controller surfaces as plan_name. LEFT JOIN so a payment whose plan row was
-- deleted still returns (plan_name NULL), matching TypeORM relations:['plan']
-- with a possibly-null relation.
SELECT
  p.id, p.user_id, p.plan_id, p.amount, p.currency, p.method, p.status,
  p.provider_ref, p.reference_code, p.qr_data, p.notes,
  p.expires_at, p.completed_at, p.created_at, p.updated_at,
  pl.name AS plan_name
FROM payments p
LEFT JOIN plans pl ON pl.id = p.plan_id
WHERE p.reference_code = $1
LIMIT 1;

-- name: ListPaymentsByUser :many
-- findByUser(userId): the user's 20 most-recent payments, newest first, each
-- with its plan (TypeORM relations:['plan'] order:{created_at:'DESC'} take:20).
SELECT
  p.id, p.user_id, p.plan_id, p.amount, p.currency, p.method, p.status,
  p.provider_ref, p.reference_code, p.qr_data, p.notes,
  p.expires_at, p.completed_at, p.created_at, p.updated_at,
  pl.id AS plan_pk, pl.name AS plan_name, pl.daily_limit AS plan_daily_limit,
  pl.price_cents AS plan_price_cents, pl.currency AS plan_currency,
  pl.stripe_price_id AS plan_stripe_price_id, pl.trial_days AS plan_trial_days,
  pl.billing_period AS plan_billing_period, pl.is_active AS plan_is_active,
  pl.created_at AS plan_created_at, pl.updated_at AS plan_updated_at
FROM payments p
LEFT JOIN plans pl ON pl.id = p.plan_id
WHERE p.user_id = $1
ORDER BY p.created_at DESC
LIMIT 20;
```

- [ ] **Step 2: Generate + verify it compiles**

Run: `cd internal/shared/pg && sqlc generate && cd ../../.. && go build ./internal/shared/pg/...`
Expected: sqlc emits `GetPaymentByReference`, `ListPaymentsByUser` + their `*Row`/`*Params` types into `internal/shared/pg/sqlc/`; build clean. If `sqlc generate` errors on an unknown column, re-read the live `payments`/`plans` schema (`\d payments` against dev DB) and reconcile — do NOT invent columns.

- [ ] **Step 3: Commit**

```bash
git add internal/shared/pg/queries_payment.sql internal/shared/pg/sqlc/
git commit -m "feat(go-payment): sqlc queries for payment status + history reads"
```

---

### Task 5: Payment repo (Querier port + Repo projections)

**Files:**
- Create: `internal/payment/reader.go`
- Test: `internal/payment/reader_test.go`

Mirrors `exttoken/repo_pg.go`: a consumer-side `Querier` interface (the sqlc subset), a `Repo` that maps sqlc rows → domain projections so callers never see `pgtype`. Two read methods: `GetByReference` and `ListByUser`.

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

type fakeQ struct {
	byRef    sqlc.GetPaymentByReferenceRow
	byRefErr error
	list     []sqlc.ListPaymentsByUserRow
	listErr  error
}

func (f fakeQ) GetPaymentByReference(ctx context.Context, ref string) (sqlc.GetPaymentByReferenceRow, error) {
	return f.byRef, f.byRefErr
}
func (f fakeQ) ListPaymentsByUser(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListPaymentsByUserRow, error) {
	return f.list, f.listErr
}

func uuidV(s string) pgtype.UUID { var u pgtype.UUID; _ = u.Scan(s); return u }

func TestRepo_GetByReference_NotFound(t *testing.T) {
	r := NewRepo(fakeQ{byRefErr: pgx.ErrNoRows})
	got, err := r.GetByReference(context.Background(), "DR-PRO-NOPE")
	if err != nil {
		t.Fatalf("ErrNoRows must map to (nil,nil), got err %v", err)
	}
	if got != nil {
		t.Fatalf("not-found must be nil, got %+v", got)
	}
}

func TestRepo_GetByReference_Maps(t *testing.T) {
	name := "Pro Monthly"
	r := NewRepo(fakeQ{byRef: sqlc.GetPaymentByReferenceRow{
		ID: uuidV("11111111-1111-1111-1111-111111111111"), Amount: 900, Currency: "USD",
		Method: "stripe", Status: "completed", ReferenceCode: "DR-PRO-ABCD1234",
		CompletedAt: pgtype.Timestamp{Time: time.Unix(1700000000, 0).UTC(), Valid: true},
		PlanName:    pgtype.Text{String: name, Valid: true},
	}})
	got, err := r.GetByReference(context.Background(), "DR-PRO-ABCD1234")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.Amount != 900 || got.Currency != "USD" || got.Method != "stripe" {
		t.Fatalf("scalar mapping wrong: %+v", got)
	}
	if got.PlanName == nil || *got.PlanName != name {
		t.Fatalf("plan_name should be %q, got %v", name, got.PlanName)
	}
	if got.CompletedAt == nil {
		t.Fatal("completed_at should be set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'Repo' -v`
Expected: FAIL — `undefined: NewRepo`.

- [ ] **Step 3: Write minimal implementation**

> The exact sqlc field types (`pgtype.Text` vs `*string` for nullable columns) depend on your `sqlc.yaml` config. Inspect the generated `GetPaymentByReferenceRow` in `internal/shared/pg/sqlc/queries_payment.sql.go` after Task 4 and adjust the field reads below to match (the existing modules use `pgtype.Timestamp` for nullable timestamps and `*string` for nullable text — see `plans/reader.go` `Currency *string`). The structure below assumes that convention; reconcile if sqlc emits differently.

```go
package payment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// Querier is the sqlc subset the payment repo needs (consumer-side port so
// tests fake it without a DB).
type Querier interface {
	GetPaymentByReference(ctx context.Context, referenceCode string) (sqlc.GetPaymentByReferenceRow, error)
	ListPaymentsByUser(ctx context.Context, userID pgtype.UUID) ([]sqlc.ListPaymentsByUserRow, error)
}

// StatusRow is the GetByReference projection feeding the status endpoint.
// PlanName/CompletedAt/ExpiresAt are nil when the underlying column is NULL.
type StatusRow struct {
	Status        string
	Method        string
	Amount        int
	Currency      string
	ReferenceCode string
	PlanName      *string
	CompletedAt   *time.Time
	ExpiresAt     *time.Time
}

// PaymentRow is the full ListByUser projection (every payments column + the
// joined plan), serialised by the history endpoint to match TypeORM's entity
// JSON. Nullable columns are pointers.
type PaymentRow struct {
	ID            string
	UserID        string
	PlanID        string
	Amount        int
	Currency      string
	Method        string
	Status        string
	ProviderRef   *string
	ReferenceCode string
	QRData        *string
	Notes         *string
	ExpiresAt     *time.Time
	CompletedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Plan          *PlanBrief
}

// PlanBrief is the nested plan object on a history row, field-for-field with
// plans.PlanEntity (same column set TypeORM loads via relations:['plan']).
type PlanBrief struct {
	ID            string
	Name          string
	DailyLimit    int
	PriceCents    int
	Currency      *string
	StripePriceID *string
	TrialDays     int
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repo is the concrete persistence adapter over the sqlc Querier.
type Repo struct{ q Querier }

// NewRepo wires the querier (accept interface, return struct).
func NewRepo(q Querier) *Repo { return &Repo{q: q} }

// GetByReference returns the payment for a reference code, or (nil,nil) when
// none exists (Node returns null → controller emits {status:"not_found"}).
func (r *Repo) GetByReference(ctx context.Context, ref string) (*StatusRow, error) {
	row, err := r.q.GetPaymentByReference(ctx, ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &StatusRow{
		Status:        row.Status,
		Method:        row.Method,
		Amount:        int(row.Amount),
		Currency:      row.Currency,
		ReferenceCode: row.ReferenceCode,
		PlanName:      textPtr(row.PlanName),
		CompletedAt:   tsPtr(row.CompletedAt),
		ExpiresAt:     tsPtr(row.ExpiresAt),
	}, nil
}

// ListByUser returns the user's 20 newest payments, each with its plan. Always
// returns a non-nil slice ([] not null).
func (r *Repo) ListByUser(ctx context.Context, userID string) ([]PaymentRow, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return []PaymentRow{}, nil // malformed id → empty history
	}
	rows, err := r.q.ListPaymentsByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentRow, 0, len(rows))
	for _, row := range rows {
		pr := PaymentRow{
			ID:            uuid.UUID(row.ID.Bytes).String(),
			UserID:        uuid.UUID(row.UserID.Bytes).String(),
			PlanID:        uuid.UUID(row.PlanID.Bytes).String(),
			Amount:        int(row.Amount),
			Currency:      row.Currency,
			Method:        row.Method,
			Status:        row.Status,
			ProviderRef:   textPtr(row.ProviderRef),
			ReferenceCode: row.ReferenceCode,
			QRData:        textPtr(row.QrData),
			Notes:         textPtr(row.Notes),
			ExpiresAt:     tsPtr(row.ExpiresAt),
			CompletedAt:   tsPtr(row.CompletedAt),
			CreatedAt:     row.CreatedAt.Time,
			UpdatedAt:     row.UpdatedAt.Time,
		}
		if row.PlanPk.Valid {
			pr.Plan = &PlanBrief{
				ID:            uuid.UUID(row.PlanPk.Bytes).String(),
				Name:          textOr(row.PlanName),
				DailyLimit:    int(row.PlanDailyLimit.Int32),
				PriceCents:    int(row.PlanPriceCents.Int32),
				Currency:      row.PlanCurrency,
				StripePriceID: row.PlanStripePriceID,
				TrialDays:     int(row.PlanTrialDays.Int32),
				BillingPeriod: string(row.PlanBillingPeriod),
				IsActive:      row.PlanIsActive.Bool,
				CreatedAt:     row.PlanCreatedAt.Time,
				UpdatedAt:     row.PlanUpdatedAt.Time,
			}
		}
		out = append(out, pr)
	}
	return out, nil
}

// --- pgtype helpers (mirror exttoken/repo_pg.go tsPtr) -------------------

func tsPtr(ts pgtype.Timestamp) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

// textPtr returns nil for a NULL/absent string. If sqlc emits *string for the
// nullable column, replace the body with `return s` and the param type with
// `*string` — inspect the generated row first (see Task 5 note).
func textPtr(s *string) *string { return s }

func textOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

> **Important — reconcile generated types:** the field accesses above (`row.PlanName`, `row.PlanDailyLimit.Int32`, `row.PlanCurrency`, etc.) assume sqlc's emitted types. Open `internal/shared/pg/sqlc/queries_payment.sql.go` and adjust each read to the actual emitted Go type. Nullable `int4` → `pgtype.Int4` (`.Int32`/`.Valid`); nullable `text` → `*string` per repo convention; nullable `bool` → `pgtype.Bool` (`.Bool`). Keep the projection field types (`*string`, `*time.Time`, `int`) stable — only the row-read expressions change. If `row.PlanName` is emitted as `*string`, change `textPtr(row.PlanName)` to `row.PlanName` and `textOr(row.PlanName)` accordingly.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -run 'Repo' -v`
Expected: PASS. (If the fake's `sqlc.GetPaymentByReferenceRow` field names/types don't match generated, fix the test literals to match the generated struct.)

- [ ] **Step 5: Commit**

```bash
git add internal/payment/reader.go internal/payment/reader_test.go
git commit -m "feat(go-payment): payment repo — status + history projections over sqlc"
```

---

### Task 6: Service + consumer-side ports (methods/status/history use cases)

**Files:**
- Create: `internal/payment/usecase.go`
- Test: `internal/payment/usecase_test.go`

The `Service` owns the three read use cases. It needs: the `Repo` (status/history), and a `SettingsReader` port for the raw `payment_methods_enabled` CSV with the env/default fallback resolved by the caller. To keep the env/default precedence identical to Node (`settings ?? env ?? default`), the Service takes the env value at construction and reads settings per-call.

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"context"
	"testing"
	"time"
)

type fakeSettings struct {
	csv   string
	found bool
	err   error
}

func (f fakeSettings) PaymentMethodsEnabled(ctx context.Context) (string, bool, error) {
	return f.csv, f.found, f.err
}

type fakeRepo struct {
	status *StatusRow
	hist   []PaymentRow
}

func (f fakeRepo) GetByReference(ctx context.Context, ref string) (*StatusRow, error) { return f.status, nil }
func (f fakeRepo) ListByUser(ctx context.Context, uid string) ([]PaymentRow, error)   { return f.hist, nil }

func TestService_EnabledMethods_SettingsWins(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{csv: "vietqr", found: true}, "lemonsqueezy")
	got, err := svc.EnabledMethods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vietqr", "bank_transfer"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("settings CSV must win → %v, got %v", want, got)
	}
}

func TestService_EnabledMethods_EnvFallback(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{found: false}, "lemonsqueezy")
	got, _ := svc.EnabledMethods(context.Background())
	if len(got) != 1 || got[0] != "lemonsqueezy" {
		t.Fatalf("env fallback → [lemonsqueezy], got %v", got)
	}
}

func TestService_EnabledMethods_DefaultStripe(t *testing.T) {
	svc := NewService(fakeRepo{}, fakeSettings{found: false}, "")
	got, _ := svc.EnabledMethods(context.Background())
	if len(got) != 1 || got[0] != "stripe" {
		t.Fatalf("no settings + no env → [stripe], got %v", got)
	}
}

func TestService_Status_NotFound(t *testing.T) {
	svc := NewService(fakeRepo{status: nil}, fakeSettings{}, "")
	v, err := svc.Status(context.Background(), "DR-PRO-NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if v.NotFound() == false {
		t.Fatal("nil status row → NotFound view")
	}
}

func TestService_History_PassesThrough(t *testing.T) {
	rows := []PaymentRow{{ReferenceCode: "DR-PRO-1", CreatedAt: time.Now()}}
	svc := NewService(fakeRepo{hist: rows}, fakeSettings{}, "")
	got, err := svc.History(context.Background(), "u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("history passthrough failed: %v %v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'Service' -v`
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: Write minimal implementation**

```go
package payment

import "context"

// PaymentRepo is the consumer-side port for the read paths (satisfied by *Repo).
type PaymentRepo interface {
	GetByReference(ctx context.Context, ref string) (*StatusRow, error)
	ListByUser(ctx context.Context, userID string) ([]PaymentRow, error)
}

// SettingsReader yields the admin-configured payment_methods_enabled CSV.
// found=false means the app_settings row (or column) is absent → caller falls
// back to env then default. Satisfied by the platform settings reader extended
// in main.go (or a thin sqlc-backed adapter).
type SettingsReader interface {
	PaymentMethodsEnabled(ctx context.Context) (string, bool, error)
}

// Service is the payment read use case (methods/status/history).
type Service struct {
	repo     PaymentRepo
	settings SettingsReader
	envCSV   string // PAYMENT_ENABLED_METHODS, used when settings absent
}

// NewService wires the repo, settings reader, and env fallback CSV.
func NewService(repo PaymentRepo, settings SettingsReader, envCSV string) *Service {
	return &Service{repo: repo, settings: settings, envCSV: envCSV}
}

// EnabledMethods resolves the storefront's visible methods. Precedence mirrors
// getEnabledMethods(): app_settings.payment_methods_enabled ?? env ?? default,
// then EnabledMethods() applies lowercase/dedupe/ordering + vietqr→bank_transfer.
func (s *Service) EnabledMethods(ctx context.Context) ([]string, error) {
	csv, found, err := s.settings.PaymentMethodsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	raw := ""
	switch {
	case found && csv != "":
		raw = csv
	case s.envCSV != "":
		raw = s.envCSV
	default:
		raw = DefaultPaymentMethod
	}
	return EnabledMethods(raw), nil
}

// Status returns the status view for a reference code (NotFound when absent).
func (s *Service) Status(ctx context.Context, ref string) (StatusView, error) {
	row, err := s.repo.GetByReference(ctx, ref)
	if err != nil {
		return StatusView{}, err
	}
	return statusViewFrom(row), nil
}

// History returns the user's 20 newest payments (Node findByUser passthrough).
func (s *Service) History(ctx context.Context, userID string) ([]PaymentRow, error) {
	return s.repo.ListByUser(ctx, userID)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -run 'Service' -v`
Expected: PASS (after Task 7 adds `StatusView`/`statusViewFrom`; if running before Task 7, this task's compile depends on them — implement Task 7's view type in the same commit or move the view definition here). **To keep this task self-contained, define `StatusView` + `statusViewFrom` now in `usecase.go`** (Task 7 only adds the handler that serialises it):

```go
// StatusView is GET /payment/status/:ref's response. When NotFound is true the
// handler emits {"status":"not_found"} and nothing else; otherwise it emits the
// full object. Field order + names match the Node controller exactly.
type StatusView struct {
	notFound      bool
	Status        string
	Method        string
	Amount        int
	Currency      string
	ReferenceCode string
	PlanName      *string
	CompletedAt   *string // ISOMillis or nil
	ExpiresAt     *string // ISOMillis or nil
}

// NotFound reports whether the reference matched no payment.
func (v StatusView) NotFound() bool { return v.notFound }

func statusViewFrom(row *StatusRow) StatusView {
	if row == nil {
		return StatusView{notFound: true}
	}
	v := StatusView{
		Status:        row.Status,
		Method:        row.Method,
		Amount:        row.Amount,
		Currency:      row.Currency,
		ReferenceCode: row.ReferenceCode,
		PlanName:      row.PlanName,
	}
	if row.CompletedAt != nil {
		iso := shared.ISOMillis(*row.CompletedAt)
		v.CompletedAt = &iso
	}
	if row.ExpiresAt != nil {
		iso := shared.ISOMillis(*row.ExpiresAt)
		v.ExpiresAt = &iso
	}
	return v
}
```

Add the import `"github.com/tannpv/draftright-rewrite/internal/shared"` to `usecase.go`.

- [ ] **Step 5: Commit**

```bash
git add internal/payment/usecase.go internal/payment/usecase_test.go
git commit -m "feat(go-payment): read service — enabled-methods precedence + status + history"
```

---

### Task 7: HTTP handlers + JSON marshalers (methods/status/history)

**Files:**
- Create: `internal/payment/handler.go`
- Test: `internal/payment/handler_test.go`

The status response is the subtle one: not-found is `{"status":"not_found"}` (one key), found is the full object. Use a custom `MarshalJSON` on `StatusView` so a single handler emits the right shape. History serialises `[]PaymentRow` with a `MarshalJSON` on `PaymentRow` + `PlanBrief` pinning field order + ms timestamps (mirroring `plans.PlanEntity.MarshalJSON`).

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

func TestStatusView_NotFoundJSON(t *testing.T) {
	b, _ := json.Marshal(StatusView{notFound: true})
	if string(b) != `{"status":"not_found"}` {
		t.Fatalf("not-found JSON = %s", b)
	}
}

func TestStatusView_FoundJSON_FieldOrder(t *testing.T) {
	name := "Pro"
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	iso := shared.ISOMillis(ts)
	v := StatusView{
		Status: "completed", Method: "stripe", Amount: 900, Currency: "USD",
		ReferenceCode: "DR-PRO-ABCD1234", PlanName: &name, CompletedAt: &iso, ExpiresAt: nil,
	}
	b, _ := json.Marshal(v)
	want := `{"status":"completed","method":"stripe","amount":900,"currency":"USD","reference_code":"DR-PRO-ABCD1234","plan_name":"Pro","completed_at":"` + iso + `","expires_at":null}`
	if string(b) != want {
		t.Fatalf("found JSON =\n%s\nwant\n%s", b, want)
	}
}

func TestMethodsHandler(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{}, fakeSettings{csv: "stripe", found: true}, ""))
	rec := httptest.NewRecorder()
	h.Methods(rec, httptest.NewRequest(http.MethodGet, "/payment/methods", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"methods\":[\"stripe\"]}\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestStatusHandler_NotFound(t *testing.T) {
	h := NewHandler(NewService(fakeRepo{status: nil}, fakeSettings{}, ""))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/payment/status/DR-PRO-NOPE", nil)
	// chi URLParam needs a route context; inject ref directly via a sub-test
	// helper or use chi.NewRouteContext. Simplest: call the service path.
	_ = req
	v, _ := h.svc.Status(context.Background(), "DR-PRO-NOPE")
	if !v.NotFound() {
		t.Fatal("expected not-found")
	}
	_ = rec
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'StatusView|MethodsHandler|StatusHandler' -v`
Expected: FAIL — `undefined: NewHandler`.

- [ ] **Step 3: Write minimal implementation**

```go
package payment

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// Handler is the HTTP edge for /payment read routes. Calls the Service only.
//
//	GET /payment/methods      → Methods (200, public)
//	GET /payment/status/:ref  → Status  (200, public)
//	GET /payment/history      → History (200, JWT)
type Handler struct{ svc *Service }

// NewHandler wires the service.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// MarshalJSON pins the status response shape. Not-found collapses to the single
// {status:"not_found"} key; otherwise the full object in Node controller order.
func (v StatusView) MarshalJSON() ([]byte, error) {
	if v.notFound {
		return []byte(`{"status":"not_found"}`), nil
	}
	return json.Marshal(struct {
		Status        string  `json:"status"`
		Method        string  `json:"method"`
		Amount        int     `json:"amount"`
		Currency      string  `json:"currency"`
		ReferenceCode string  `json:"reference_code"`
		PlanName      *string `json:"plan_name"`
		CompletedAt   *string `json:"completed_at"`
		ExpiresAt     *string `json:"expires_at"`
	}{
		Status: v.Status, Method: v.Method, Amount: v.Amount, Currency: v.Currency,
		ReferenceCode: v.ReferenceCode, PlanName: v.PlanName,
		CompletedAt: v.CompletedAt, ExpiresAt: v.ExpiresAt,
	})
}

// Methods: GET /payment/methods (public) → {methods:[...]}.
func (h *Handler) Methods(w http.ResponseWriter, r *http.Request) {
	methods, err := h.svc.EnabledMethods(r.Context())
	if err != nil {
		shared.WriteError(w, r, "internal", "methods failed")
		return
	}
	if methods == nil {
		methods = []string{}
	}
	shared.WriteJSON(w, http.StatusOK, map[string][]string{"methods": methods})
}

// Status: GET /payment/status/{ref} (public) → status view (or not_found).
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ref := chi.URLParam(r, "ref")
	v, err := h.svc.Status(r.Context(), ref)
	if err != nil {
		shared.WriteError(w, r, "internal", "status failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, v)
}

// History: GET /payment/history (JWT) → bare JSON array of payments. Empty → [].
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	rows, err := h.svc.History(r.Context(), claims.Sub)
	if err != nil {
		shared.WriteError(w, r, "internal", "history failed")
		return
	}
	if rows == nil {
		rows = []PaymentRow{}
	}
	shared.WriteJSON(w, http.StatusOK, rows)
}
```

Add `PaymentRow` + `PlanBrief` `MarshalJSON` methods in `reader.go` pinning field order to the TypeORM entity declaration order (id, user_id, plan_id, amount, currency, method, status, provider_ref, reference_code, qr_data, notes, expires_at, completed_at, created_at, updated_at, then nested `plan`):

```go
// MarshalJSON pins PaymentRow to the TypeORM Payment entity JSON: every column
// in declaration order, ms-precision timestamps, nested plan last. The `user`
// relation is NOT loaded by findByUser (relations:['plan'] only), so it is
// absent here too.
func (p PaymentRow) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string     `json:"id"`
		UserID        string     `json:"user_id"`
		PlanID        string     `json:"plan_id"`
		Amount        int        `json:"amount"`
		Currency      string     `json:"currency"`
		Method        string     `json:"method"`
		Status        string     `json:"status"`
		ProviderRef   *string    `json:"provider_ref"`
		ReferenceCode string     `json:"reference_code"`
		QRData        *string    `json:"qr_data"`
		Notes         *string    `json:"notes"`
		ExpiresAt     *string    `json:"expires_at"`
		CompletedAt   *string    `json:"completed_at"`
		CreatedAt     string     `json:"created_at"`
		UpdatedAt     string     `json:"updated_at"`
		Plan          *PlanBrief `json:"plan"`
	}{
		ID: p.ID, UserID: p.UserID, PlanID: p.PlanID, Amount: p.Amount,
		Currency: p.Currency, Method: p.Method, Status: p.Status,
		ProviderRef: p.ProviderRef, ReferenceCode: p.ReferenceCode, QRData: p.QRData,
		Notes: p.Notes, ExpiresAt: isoPtr(p.ExpiresAt), CompletedAt: isoPtr(p.CompletedAt),
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
		Plan: p.Plan,
	})
}

// MarshalJSON pins PlanBrief to plans.PlanEntity's shape (same column order).
func (p PlanBrief) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            string  `json:"id"`
		Name          string  `json:"name"`
		DailyLimit    int     `json:"daily_limit"`
		PriceCents    int     `json:"price_cents"`
		Currency      *string `json:"currency"`
		StripePriceID *string `json:"stripe_price_id"`
		TrialDays     int     `json:"trial_days"`
		BillingPeriod string  `json:"billing_period"`
		IsActive      bool    `json:"is_active"`
		CreatedAt     string  `json:"created_at"`
		UpdatedAt     string  `json:"updated_at"`
	}{
		ID: p.ID, Name: p.Name, DailyLimit: p.DailyLimit, PriceCents: p.PriceCents,
		Currency: p.Currency, StripePriceID: p.StripePriceID, TrialDays: p.TrialDays,
		BillingPeriod: p.BillingPeriod, IsActive: p.IsActive,
		CreatedAt: shared.ISOMillis(p.CreatedAt), UpdatedAt: shared.ISOMillis(p.UpdatedAt),
	})
}

func isoPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := shared.ISOMillis(*t)
	return &s
}
```

Add imports `"encoding/json"`, `"time"`, and the shared package to `reader.go`.

> **History parity caveat:** TypeORM serialises a `Date` column that is NULL as JSON `null`, and omits a relation that wasn't loaded. The marshaler above emits `null` for null timestamps (correct) and includes `plan` always (loaded). If the live Node response turns out to include extra TypeORM metadata or a different null-vs-absent treatment, the shadow gate (history fixture, Phase 3a deliverable below) will surface the exact diff — reconcile then. Until pinned, the history shadow fixture should `ignore_value_of` the per-row `id`, `created_at`, `updated_at`, `expires_at`, `completed_at`, and nested `plan.created_at`/`plan.updated_at`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/payment/ -v`
Expected: PASS (whole package).

- [ ] **Step 5: Commit**

```bash
git add internal/payment/handler.go internal/payment/handler_test.go internal/payment/reader.go
git commit -m "feat(go-payment): read handlers + status/history JSON marshalers (field-order pinned)"
```

---

### Task 8: Settings adapter — payment_methods_enabled

**Files:**
- Modify: `internal/shared/pg/queries_core.sql` (add query) → `sqlc generate`
- Create: `internal/payment/settings_pg.go` (thin adapter implementing `SettingsReader`)
- Test: `internal/payment/settings_pg_test.go`

Node reads `settingsRepo.findOne({})` and takes `payment_methods_enabled` (a nullable text column on `app_settings`). Add a one-column sqlc query + an adapter that maps NULL/no-row → `found=false`.

- [ ] **Step 1: Add the sqlc query**

Append to `internal/shared/pg/queries_core.sql`:

```sql
-- name: GetPaymentMethodsEnabled :one
-- payment_methods_enabled CSV from the singleton app_settings row. Nullable;
-- absent row or NULL column → caller falls back to env then default.
SELECT payment_methods_enabled FROM app_settings LIMIT 1;
```

Run: `cd internal/shared/pg && sqlc generate && cd ../../..`
Expected: `GetPaymentMethodsEnabled` added; emitted return type is `*string` (nullable text) — confirm in `queries_core.sql.go`.

- [ ] **Step 2: Write the failing test**

```go
package payment

import (
	"context"
	"testing"
)

type fakeCoreQ struct {
	val *string
	err error
}

func (f fakeCoreQ) GetPaymentMethodsEnabled(ctx context.Context) (*string, error) {
	return f.val, f.err
}

func TestSettingsAdapter(t *testing.T) {
	csv := "stripe,vietqr"
	a := NewSettingsAdapter(fakeCoreQ{val: &csv})
	got, found, err := a.PaymentMethodsEnabled(context.Background())
	if err != nil || !found || got != csv {
		t.Fatalf("present CSV → (%q,true,nil), got (%q,%v,%v)", csv, got, found, err)
	}

	a2 := NewSettingsAdapter(fakeCoreQ{val: nil})
	_, found2, _ := a2.PaymentMethodsEnabled(context.Background())
	if found2 {
		t.Fatal("NULL column → found=false")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/payment/ -run 'SettingsAdapter' -v`
Expected: FAIL — `undefined: NewSettingsAdapter`.

- [ ] **Step 4: Write minimal implementation**

```go
package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	sqlc "github.com/tannpv/draftright-rewrite/internal/shared/pg/sqlc"
)

// coreSettingsQuerier is the one-method sqlc subset for the settings read.
type coreSettingsQuerier interface {
	GetPaymentMethodsEnabled(ctx context.Context) (*string, error)
}

// SettingsAdapter maps the app_settings.payment_methods_enabled column to the
// Service's SettingsReader port. NULL column / no row → found=false.
type SettingsAdapter struct{ q coreSettingsQuerier }

// NewSettingsAdapter wires the querier (the live *sqlc.Queries satisfies it).
func NewSettingsAdapter(q coreSettingsQuerier) *SettingsAdapter { return &SettingsAdapter{q: q} }

// PaymentMethodsEnabled returns (csv, found, err). found=false on NULL column
// or absent row so the Service degrades to env → default.
func (a *SettingsAdapter) PaymentMethodsEnabled(ctx context.Context) (string, bool, error) {
	v, err := a.q.GetPaymentMethodsEnabled(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if v == nil {
		return "", false, nil
	}
	return *v, true, nil
}

var _ = sqlc.Queries{} // keep the sqlc import meaningful if unused above
```

> If the generated `GetPaymentMethodsEnabled` returns `pgtype.Text` instead of `*string`, change `coreSettingsQuerier` + the fake to that type and read `.Valid`/`.String`. Remove the `var _ =` line if it causes an unused-import or vet complaint — it's only there to anchor the import; delete it if the file already imports `sqlc` elsewhere (it doesn't here, so drop the import + the anchor line and the `coreSettingsQuerier` interface alone suffices).

- [ ] **Step 5: Run + commit**

Run: `go test ./internal/payment/ -v`
Expected: PASS.

```bash
git add internal/shared/pg/queries_core.sql internal/shared/pg/sqlc/ internal/payment/settings_pg.go internal/payment/settings_pg_test.go
git commit -m "feat(go-payment): app_settings payment_methods_enabled adapter"
```

---

### Task 9: Wire into router + main.go composition root

**Files:**
- Modify: `internal/shared/router.go` (3 new fields + 3 mounts)
- Modify: `cmd/server/main.go` (`coreHandlers` fields + composeDeps wiring + Router assignments)
- Test: extend `internal/shared/router_test.go` if present (else rely on build + live gate)

- [ ] **Step 1: Add router fields**

In `internal/shared/router.go`, add to the `Router` struct (near the other public-route fields, after `Plans`):

```go
	PaymentMethods http.Handler // GET /payment/methods       (public)
	PaymentStatus  http.Handler // GET /payment/status/{ref}   (public)
	PaymentHistory http.Handler // GET /payment/history        (auth)
```

- [ ] **Step 2: Mount them**

In `Build()`, mount the two public routes beside the other public mounts (after the `r.Plans` block):

```go
	if r.PaymentMethods != nil {
		mux.Method(http.MethodGet, "/payment/methods", r.PaymentMethods)
	}
	if r.PaymentStatus != nil {
		mux.Method(http.MethodGet, "/payment/status/{ref}", r.PaymentStatus)
	}
```

And mount the authed history route inside the JWT `mux.Group` (beside `r.Subscription`):

```go
		if r.PaymentHistory != nil {
			api.Method(http.MethodGet, "/payment/history", r.PaymentHistory)
		}
```

- [ ] **Step 3: Run build to verify it compiles**

Run: `go build ./internal/shared/...`
Expected: clean.

- [ ] **Step 4: Wire in main.go**

In `cmd/server/main.go`, add to `coreHandlers` (beside `subscription`/`verifyReceipt`):

```go
	paymentMethods http.Handler // GET /payment/methods       (set when pool != nil)
	paymentStatus  http.Handler // GET /payment/status/{ref}   (set when pool != nil)
	paymentHistory http.Handler // GET /payment/history        (set when pool != nil)
```

In `composeDeps` (inside the `pool != nil` branch, beside the subscription/exttoken wiring), build the module. Use the env `PAYMENT_ENABLED_METHODS` from config (add the field to `internal/platform/config/config.go` if absent — default `""`):

```go
		paymentRepo := paymentpkg.NewRepo(q)
		paymentSettings := paymentpkg.NewSettingsAdapter(q)
		paymentSvc := paymentpkg.NewService(paymentRepo, paymentSettings, cfg.PaymentEnabledMethods)
		paymentHandler := paymentpkg.NewHandler(paymentSvc)
		core.paymentMethods = http.HandlerFunc(paymentHandler.Methods)
		core.paymentStatus = http.HandlerFunc(paymentHandler.Status)
		core.paymentHistory = http.HandlerFunc(paymentHandler.History)
```

Add the import `paymentpkg "github.com/tannpv/draftright-rewrite/internal/payment"` to main.go's import block (matching the `subpkg`/`exttokenpkg` aliasing style).

Then assign on the `Router` literal (beside `Subscription`):

```go
		PaymentMethods: core.paymentMethods,
		PaymentStatus:  core.paymentStatus,
		PaymentHistory: core.paymentHistory,
```

- [ ] **Step 5: Config field (if needed)**

If `cfg.PaymentEnabledMethods` doesn't exist, add to `internal/platform/config/config.go`: a `PaymentEnabledMethods string` field read from env `PAYMENT_ENABLED_METHODS` (default `""`), mirroring how other optional env strings are read there.

- [ ] **Step 6: Build + full test + gates**

Run:
```bash
go build ./... && go test ./... -race && gofmt -l . && go vet ./...
```
Expected: build clean, all tests pass, gofmt prints nothing, vet clean.

- [ ] **Step 7: Commit**

```bash
git add internal/shared/router.go cmd/server/main.go internal/platform/config/config.go
git commit -m "feat(go-payment): mount /payment/{methods,status,history} + wire composition root"
```

---

### Task 10: Shadow-gate fixtures for the three read endpoints

**Files:**
- Create: `fixtures/payment_methods.json`, `fixtures/payment_status.json`, `fixtures/payment_history.json`

These extend the existing shadow gate (`cmd/shadowdiff`). Follow the `REPLACE_WITH_DEV_*` placeholder convention + `_note` discipline established in the Phase 0–2 fixtures.

- [ ] **Step 1: Author the fixtures**

`fixtures/payment_methods.json`:
```json
{
  "name": "payment-methods",
  "method": "GET",
  "path": "/payment/methods",
  "headers": {},
  "ignore_value_of": [],
  "_note": "Public. {methods:[...]} — ORDER matters (insertion order; vietqr implies bank_transfer appended last). Deterministic from app_settings.payment_methods_enabled; nothing ignored. If Go and Node disagree on order the gate flags a real ordering-parity bug — do NOT add to ignore."
}
```

`fixtures/payment_status.json`:
```json
{
  "name": "payment-status",
  "method": "GET",
  "path": "/payment/status/REPLACE_WITH_DEV_PAYMENT_REFERENCE",
  "headers": {},
  "ignore_value_of": ["completed_at", "expires_at"],
  "_note": "Public. Substitute a real DR-PRO-* reference from a seeded payment row. For a non-existent ref both return {status:'not_found'} (also a valid parity check — point the path at DR-PRO-NOPE to test that branch). completed_at/expires_at ignored until ISOMillis format is pinned byte-for-byte (same timestamp risk as account.json); then REMOVE them. status/method/amount/currency/reference_code/plan_name are deterministic and must match."
}
```

`fixtures/payment_history.json`:
```json
{
  "name": "payment-history",
  "method": "GET",
  "path": "/payment/history",
  "headers": { "Authorization": "Bearer REPLACE_WITH_DEV_NODE_TOKEN" },
  "ignore_value_of": ["id", "created_at", "updated_at", "expires_at", "completed_at"],
  "_note": "Authed. Bare JSON array, newest first, max 20, each row = full Payment entity + nested plan (user relation NOT loaded). Per-row id + timestamps ignored (presence+non-empty, recursively — also covers nested plan.created_at/updated_at). Deterministic per row: user_id, plan_id, amount, currency, method, status, provider_ref, reference_code, qr_data, notes, and the nested plan's name/daily_limit/price_cents/currency/billing_period/is_active. If Go emits a `user` key (it must NOT — findByUser loads only plan) the gate flags it. Seed a fixed set of payments for a stable compare."
}
```

- [ ] **Step 2: Validate JSON + run the existing gate's unit tests**

Run:
```bash
for f in fixtures/payment_*.json; do python3 -c "import json;json.load(open('$f'))" && echo "ok $f"; done
go test ./cmd/shadowdiff/
```
Expected: all `ok`, shadowdiff tests pass (the recursive comparator from the prior commit already handles bare arrays + nested ignored keys).

- [ ] **Step 3: Commit**

```bash
git add fixtures/payment_methods.json fixtures/payment_status.json fixtures/payment_history.json
git commit -m "feat(go-payment): shadow-gate fixtures for /payment read endpoints"
```

> **Live run is operator-pending** (same constraints as Phase 2): needs dev seed creds, a real `DR-PRO-*` reference, a dev node token, Go on `:3001` with dev DB, network to `api.dev.draftright.info`. Do NOT run live with invented creds. Phase 3a ships nothing to prod — no Caddy change.

---

## Self-Review

**1. Spec coverage** (against the Node read slice):
- `GET /payment/methods` → Tasks 3, 6, 7, 9 ✅
- `GET /payment/status/:ref` (incl. not_found branch) → Tasks 5, 6, 7, 9 ✅
- `GET /payment/history` → Tasks 5, 6, 7, 9 ✅
- `getEnabledMethods` precedence + ordering + vietqr→bank_transfer → Task 3 + Task 6 ✅
- `registeredMethods` / `assertMethodsRegisterable` → Task 3 ✅ (assertion is exposed for Phase 4 admin write; not mounted as a route here — correct, Node only calls it from the settings-save path)
- `generatePaymentReference` / `extractPaymentReference` → Task 2 ✅ (used by 3b checkout + 3c webhooks; built now as foundation)
- `storeTypeForMethod` → Task 1 ✅ (used by 3b/3c; built now)
- enums → Task 1 ✅
- **Deliberately deferred (NOT in 3a):** `POST /payment/checkout`, concrete strategies, `CheckoutResult`/`WebhookAction` *implementations*, all 5 webhooks, `getCustomerPortalUrl`, `cancelActiveSubscription`, admin `/admin/payments*`, `expireStalePayments` cron, `completePayment`/`activateSubscription`. Tracked in the Phase 3 decomposition table. ✅ (gap is intentional + documented)

**2. Placeholder scan:** No "TBD"/"handle errors"/"similar to Task N". Every code step has complete code. The two reconcile-with-generated-sqlc notes (Task 5, Task 8) are explicit, bounded instructions ("open the generated file, match the emitted type"), not hand-waves — sqlc output is environment-determined and cannot be hard-coded blind. ✅

**3. Type consistency:** `PaymentMethod`/`PaymentStatus`/`StoreType` consts (Task 1) reused everywhere. `StatusRow`/`PaymentRow`/`PlanBrief` (Task 5) consumed by Task 6/7 marshalers. `StatusView` + `statusViewFrom` defined in Task 6, marshaled in Task 7 — same type, no drift. `Querier` (Task 5) vs `coreSettingsQuerier` (Task 8) are distinct ports for distinct sqlc subsets — intentional, no collision. `PaymentRepo`/`SettingsReader` ports (Task 6) satisfied by `*Repo` (Task 5) / `*SettingsAdapter` (Task 8). `EnabledMethods(raw string)` signature stable across Tasks 3 + 6. ✅

**Note for the implementer:** the `subscription` module already defines its own `PlanBrief` (in `view.go`). The `payment.PlanBrief` here is a *different type in a different package* — no conflict (Go package scoping). Do not attempt to share them; their JSON shapes differ (subscription's is 3 fields, payment's is the full plan entity).
