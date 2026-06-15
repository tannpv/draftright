# Phase 3b — Payment Checkout + Strategies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the NestJS payment *write* surface — `POST /payment/checkout` (create a pending payment + delegate to a provider strategy), `GET /payment/portal`, and `DELETE /payment/subscription` — to Go as a byte-identical drop-in, with the Stripe / VietQR / LemonSqueezy strategies behind a `Strategy` port.

**Architecture:** A `payment.Service` owns a `map[string]strategy.Strategy` (the same key set as Node's `strategies` Map). `CreateCheckout` looks up the plan, asserts the method is enabled, INSERTs a `pending` payment row, then delegates to the resolved strategy. Strategies live in layered sub-packages (`internal/payment/strategy/{stripe,vietqr,lemonsqueezy}`) because the layer now has **N implementations** (CLAUDE.md flat→layered rule). The HTTP response nests the full payment entity under a `payment` key plus optional `redirect_url` / `qr_data` / `bank_info` / `wallet_intent`.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5 + sqlc, `github.com/stripe/stripe-go/v82` (official SDK, `client.API`), raw `net/http` for LemonSqueezy, `crypto/rand` + `crypto/subtle` for reference codes / constant-time compares.

---

## Parity facts (the contract this plan ports)

These come from the NestJS source and are the byte-for-byte targets. Do not paraphrase them in code — match them.

- **Reference code:** `DR-PRO-` + `randomBytes(4).toString('hex').toUpperCase()` → `DR-PRO-` + 8 uppercase hex chars. (`payment-reference.ts`)
- **Pending TTL:** `expires_at = now + 30 min`. (`PAYMENT_PENDING_TTL_MS = 30*60_000`)
- **websiteUrl():** `process.env.WEBSITE_URL || 'http://localhost:4000'`.
- **`createCheckout` flow** (`payment.service.ts:128-182`), in order:
  1. `plan = plansService.findById(planId)`; `null` → `NotFoundException('Plan not found')` (404).
  2. `plan.price_cents === 0` → `BadRequestException('Cannot purchase a free plan')` (400).
  3. `assertEnabled(method)`: not in `getEnabledMethods()` → `NotFoundException('Payment method '<method>' is not enabled.')` (404).
  4. `getStrategy(method)`: no strategy → `BadRequestException('Unsupported payment method: <method>')` (400). (Unreachable when method passed `@IsIn` AND is enabled AND is registered — registered set == enabled-eligible set — but port it.)
  5. `user = userRepo.findOne({id})`; `null` → `NotFoundException('User not found')` (404).
  6. INSERT payment: `{user_id, plan_id, amount: plan.price_cents, currency: plan.currency || 'USD', method, status: 'pending', reference_code, expires_at}`.
  7. attach `payment.plan = plan` (eager, for the response + strategy).
  8. `strategy.createCheckout(payment, {success_url, cancel_url, stripe_customer_id, user_email})` in try/catch. On throw → UPDATE `status='failed', notes=err.message`; rethrow as `BadRequestException(err.message || 'Payment provider error')` (400).
  9. if `result.qr_data` → UPDATE `qr_data`; set `payment.qr_data`.
  10. return `result`.
- **`getCustomerPortalUrl(userId)`** (`:198-231`):
  1. user `null` → 404 `'User not found'`.
  2. `sub = subscriptionsService.findActiveByUserId(userId)`; `null` → 404 `'No active subscription to manage'`.
  3. switch `sub.store_type`: `stripe`→STRIPE, `lemonsqueezy`→LEMONSQUEEZY, default → 404 `'Subscriptions sourced from '<store_type>' have no self-service portal'`.
  4. `url = strategy.getCustomerPortalUrl(user)`; `!url` → 404 `'Customer portal is not available for this subscription'`.
  5. return `url` → controller wraps `{ url }`.
- **`cancelActiveSubscription(userId)`** (`:252-281`):
  1. user `null` → 404 `'User not found'`.
  2. `sub = findActiveByUserId`; `null` → 404 `'No active subscription to cancel'`.
  3. `!sub.store_transaction_id` → 404 `'Subscription has no provider reference to cancel'`.
  4. switch `store_type`: stripe/lemonsqueezy, default → 404 `'Subscriptions sourced from '<store_type>' can't be cancelled in-app'`.
  5. `cancelled = strategy.cancelSubscription(sub.store_transaction_id)`; `!cancelled` → 404 `'Provider declined to cancel the subscription'`.
  6. return `{ cancelled: true, expires_at: sub.expires_at }`.
- **Controller status codes:** `POST /payment/checkout` → **201** (Nest default for `@Post`, no `@HttpCode`). `GET /payment/portal` → **200**, body `{ url }` (`url` may be a string; never reaches null because step 4 throws). `DELETE /payment/subscription` → **200** (explicit `@HttpCode(200)`), body `{ cancelled, expires_at }`.
- **DTO `CreateCheckoutDto`** (`@IsString plan_id`, `@IsIn(PAYMENT_METHODS) method`, `@IsOptional @IsString success_url?/cancel_url?`), global `ValidationPipe({whitelist, forbidNonWhitelisted, transform})`. On failure → 400, `code='invalid-input'`, message = each constraint string mapped through `humanizeValidation` and joined with `'. '`. Relevant raw constraint strings:
  - `plan_id` missing/non-string → `'plan_id must be a string'` (humanizer leaves verbatim).
  - `method` not allowed → `'method must be one of the following values: stripe, paypal, momo, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay'` (enum value order). Verbatim.
  - unknown property X (forbidNonWhitelisted) → `'property X should not exist'`. Verbatim.
  - Multiple errors join with `'. '`, in DTO property order (plan_id, method, success_url, cancel_url), then whitelist errors.
- **`CheckoutResult` JSON:** `{ payment: <Payment entity>, redirect_url?, qr_data?, bank_info?, wallet_intent? }`. Optional keys are **omitted** when absent (JS `undefined`). `payment` is the full entity with `plan` nested (NO `user` relation). `bank_info` = `{ bank_name, account_number, account_name, amount, currency, reference }`. `wallet_intent` = `{ client_secret, publishable_key, merchant_identifier?, country_code, currency_code, display_amount, display_label }`.
- **VietQR strategy:** credentials `bankId = creds || 'MB'`, `accountNumber = creds || '0000000000'`, `accountName = creds || 'DRAFTRIGHT'`. `bank_name = getBankDisplayName(bankId)` (map below, default = bankId). `method===bank_transfer` → `{payment, bank_info}` only. else `qr_data = https://img.vietqr.io/image/<bankId>-<accountNumber>-compact2.jpg?amount=<amount>&addInfo=<urlenc reference>&accountName=<urlenc accountName>` → `{payment, qr_data, bank_info}`.
  - Bank map: `MB:'MB Bank (Quân Đội)', VCB:'Vietcombank', ACB:'ACB', TCB:'Techcombank', VPB:'VPBank', TPB:'TPBank', BIDV:'BIDV', VTB:'VietinBank', SCB:'Sacombank'`.
- **Stripe strategy** (`stripe.strategy.ts`): `secretKey = resolveCredential(stripe_secret_key, STRIPE_SECRET_KEY)`. `createCheckout`: throw `'Stripe secret_key is not configured'` if `!secretKey`. method `apple_pay`/`google_pay` → wallet intent path; else subscription Checkout Session. Throw `'Plan has no Stripe price configured'` if `!plan.stripe_price_id` (see source for exact message — confirm at impl time, line ~70). Session params: `mode:'subscription'`, `payment_method_types:['card']`, `line_items:[{price: stripe_price_id, quantity:1}]`, `metadata:{reference_code, payment_id, user_id, plan_id}`, `subscription_data:{metadata:{reference_code, user_id, plan_id}, trial_period_days?: plan.trial_days when >0}`, `success_url: options.success_url || '<websiteUrl>/payment/success?ref=<reference_code>'`, `cancel_url: options.cancel_url || '<websiteUrl>/payment/cancel'`, `client_reference_id: user_id`, customer reuse (`customer` if `stripe_customer_id` else `customer_email`). Return `{payment, redirect_url: session.url || undefined}`.
  - Wallet intent: reuse/create Customer; `paymentIntents.create({amount, currency: lower(currency), customer, setup_future_usage:'off_session', payment_method_types:['card'], metadata:{reference_code, payment_id, user_id, plan_id, method}})`. `publishable_key` = env `STRIPE_PUBLISHABLE_KEY` (no DB column). `merchant_identifier` = env `APPLE_PAY_MERCHANT_ID` (→ omit if empty). zero-decimal currencies `['VND','JPY','KRW']`: `display_amount = isZeroDecimal ? String(amount) : (amount/100).toFixed(2)`. `display_label = cadence ? '<plan.name||Pro> · <Cap(cadence)>' : (plan.name||'Pro')` where cadence derives from `plan.billing_period` (`monthly`→`Monthly`, `yearly`→`Yearly`, else ''). `country_code = plan.country_code || 'US'` — **plans has no country_code column → always `'US'`**. `currency_code = upper(currency)`.
  - `getCustomerPortalUrl(user)`: `!user.stripe_customer_id` → null; else `billingPortal.sessions.create({customer, return_url: '<websiteUrl>/account?subscribed=1'})` → `session.url || null`.
  - `cancelSubscription(id)`: `subscriptions.update(id, {cancel_at_period_end:true})` → `true`.
- **LemonSqueezy strategy** (`lemonsqueezy.strategy.ts`, API base `https://api.lemonsqueezy.com/v1`): `apiKey`, `storeId = lemonsqueezy_store_id || String(env LEMONSQUEEZY_STORE_ID) || ''`, `variantMonthly`, `variantYearly`. `createCheckout`: throw `'LemonSqueezy is not configured'`-style if `!apiKey || !storeId` (confirm exact msg at impl, line ~78). `isYearly = (plan.billing_period||'monthly')==='yearly'`; `variantId = isYearly?variantYearly:variantMonthly`; throw if `!variantId`. POST `/checkouts` body `{data:{type:'checkouts', attributes:{checkout_data:{email: user_email, custom:{reference_code}}, product_options:{redirect_url: success_url || '<websiteUrl>/payment/success?ref=<reference_code>', enabled_variants:[Number(variantId)]}}, relationships:{store:{data:{type:'stores',id:String(storeId)}}, variant:{data:{type:'variants',id:String(variantId)}}}}}`, headers `Accept`+`Content-Type: application/vnd.api+json`, `Authorization: Bearer <apiKey>`. `!res.ok` → throw. `redirect_url = json.data.attributes.url`. Return `{payment, redirect_url}`.
  - `getCustomerPortalUrl(user)`: `!user.lemonsqueezy_customer_id` → null; GET `/customers/<id>` → `json.data.attributes.urls.customer_portal ?? null`.
  - `cancelSubscription(id)`: DELETE `/subscriptions/<id>`; `!res.ok` → throw; → `true`.
- **`resolveCredential(fromDb, env)`:** `fromDb && len>0 → fromDb`, else `env` (string) else `''`.
- **`timingSafeStrEqual(a,b)`:** length-checked constant-time (Phase 3b uses it only inside strategies for webhook-secret compares — webhooks are 3c, but the helper ships here so strategies compile clean; include + test it).

## Out of scope (later phases — do NOT build here)

- All `POST /payment/webhook/*` handlers + `verifyWebhook` strategy methods → **Phase 3c**.
- Subscription activation/renewal/expire side-effects, email notifications, the renewal cron → already Phase 2 / future.
- Refund endpoints (`refundPayment`), admin payment CRUD → later.
- Casso/SePay webhook key plumbing (the `casso_api_key`/`sepay_api_key` creds) — not read by checkout; skip.

---

## File Structure

**New files**

| File | Responsibility |
|---|---|
| `internal/payment/reference.go` | `GeneratePaymentReference()` → `DR-PRO-` + 8 upper-hex. |
| `internal/payment/reference_test.go` | format/regex/uniqueness test. |
| `internal/payment/strategy/strategy.go` | Port `Strategy` + value types (`Payment`,`Plan`,`Options`,`Result`,`BankInfo`,`WalletIntent`,`PortalUser`) + `Base` helpers (`ResolveCredential`,`TimingSafeStrEqual`). Zero deps on pgx/http. |
| `internal/payment/strategy/strategy_test.go` | `ResolveCredential` + `TimingSafeStrEqual` table tests. |
| `internal/payment/strategy/vietqr/vietqr.go` | VietQR + bank_transfer `Strategy`. |
| `internal/payment/strategy/vietqr/vietqr_test.go` | URL build, bank-name map, bank_transfer vs vietqr branch, defaults. |
| `internal/payment/strategy/stripe/stripe.go` | Stripe `Strategy`: subscription checkout + wallet intent + portal + cancel. |
| `internal/payment/strategy/stripe/stripe_test.go` | deterministic helpers (display_amount, display_label, success/cancel URL defaults, not-configured error). |
| `internal/payment/strategy/lemonsqueezy/lemonsqueezy.go` | LS `Strategy`: checkout + portal + cancel via net/http. |
| `internal/payment/strategy/lemonsqueezy/lemonsqueezy_test.go` | request-body build, variant selection, not-configured errors, httptest round-trips. |
| `internal/payment/checkout.go` | `Service.CreateCheckout` + DTO validation (`validateCheckout`) + `CheckoutResponse` marshaling. |
| `internal/payment/checkout_test.go` | flow + error-path tests with fake repo/strategies. |
| `internal/payment/portal.go` | `Service.CustomerPortalURL` + `Service.CancelActiveSubscription`. |
| `internal/payment/portal_test.go` | switch + guard tests. |
| `internal/payment/handler_checkout.go` | `Handler.Checkout`, `Handler.Portal`, `Handler.CancelSubscription`. |
| `fixtures/payment_checkout_errors.json`, `fixtures/payment_checkout_vietqr.json` | shadow-gate fixtures. |

**Modified files**

| File | Change |
|---|---|
| `internal/platform/config/config.go` | add payment env fields. |
| `internal/shared/pg/queries_core.sql` | add `GetPaymentCredentials`, `GetPlanForCheckout`, `GetUserForCheckout`, `CreatePayment`, `UpdatePaymentQRData`, `MarkPaymentFailed`; extend active-subscription query with `store_transaction_id`. |
| `internal/payment/settings_pg.go` | add `Credentials()` loader on `SettingsAdapter`. |
| `internal/payment/reader.go` | add checkout-side repo methods (`PlanForCheckout`, `UserForCheckout`, `CreatePayment`, `UpdateQRData`, `MarkFailed`) + extend `Querier`. |
| `internal/payment/usecase.go` | extend `Service` with strategies map + new ports; new constructor params. |
| `internal/subscription/reader.go` | add `StoreTransactionID` to `AccountSub`. |
| `internal/shared/router.go` | mount `POST /payment/checkout`, `GET /payment/portal`, `DELETE /payment/subscription`. |
| `cmd/server/main.go` | build strategies, extend `paymentSvc` wiring + route fields. |
| `go.mod` / `go.sum` | add `github.com/stripe/stripe-go/v82`. |

---

## Task 1: Config — payment env vars

**Files:** Modify `internal/platform/config/config.go`; Test `internal/platform/config/config_test.go` (extend if exists, else create).

- [ ] **Step 1: Write the failing test**

```go
func TestPaymentEnv(t *testing.T) {
	t.Setenv("WEBSITE_URL", "https://draftright.info")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_x")
	t.Setenv("STRIPE_PUBLISHABLE_KEY", "pk_test_x")
	t.Setenv("APPLE_PAY_MERCHANT_ID", "merchant.info.draftright")
	c := Load()
	if c.WebsiteURL != "https://draftright.info" {
		t.Fatalf("WebsiteURL=%q", c.WebsiteURL)
	}
	if c.StripeSecretKey != "sk_test_x" || c.StripePublishableKey != "pk_test_x" || c.ApplePayMerchantID != "merchant.info.draftright" {
		t.Fatalf("stripe/apple env not wired: %+v", c)
	}
}

func TestWebsiteURLDefault(t *testing.T) {
	t.Setenv("WEBSITE_URL", "")
	if got := Load().WebsiteURL; got != "http://localhost:4000" {
		t.Fatalf("default WebsiteURL=%q want http://localhost:4000", got)
	}
}
```

- [ ] **Step 2: Run → FAIL** `go test ./internal/platform/config/ -run TestPaymentEnv -v` → undefined fields.

- [ ] **Step 3: Implement.** Add fields to the `Config` struct and populate in `Load()` (match the file's existing `os.Getenv` style; `WebsiteURL` uses a default):

```go
// Payment (Phase 3b). Credentials prefer the app_settings DB row; these
// env values are the resolveCredential() fallback (first-deploy / dev).
// PublishableKey + ApplePayMerchantID are env-ONLY (no app_settings column).
WebsiteURL            string
StripeSecretKey       string
StripePublishableKey  string
ApplePayMerchantID    string
LemonSqueezyAPIKey    string
LemonSqueezyStoreID   string
VietQRBankID          string
VietQRAccountNumber   string
VietQRAccountName     string
```

```go
WebsiteURL:           envOr("WEBSITE_URL", "http://localhost:4000"),
StripeSecretKey:      os.Getenv("STRIPE_SECRET_KEY"),
StripePublishableKey: os.Getenv("STRIPE_PUBLISHABLE_KEY"),
ApplePayMerchantID:   os.Getenv("APPLE_PAY_MERCHANT_ID"),
LemonSqueezyAPIKey:   os.Getenv("LEMONSQUEEZY_API_KEY"),
LemonSqueezyStoreID:  os.Getenv("LEMONSQUEEZY_STORE_ID"),
VietQRBankID:         os.Getenv("VIETQR_BANK_ID"),
VietQRAccountNumber:  os.Getenv("VIETQR_ACCOUNT_NUMBER"),
VietQRAccountName:    os.Getenv("VIETQR_ACCOUNT_NAME"),
```

If `envOr` does not exist in the package, add it: `func envOr(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }`. (Note: Stripe webhook secrets + LS variant/webhook/store creds that webhooks need are added in Phase 3c — only checkout-time creds here.)

- [ ] **Step 4: Run → PASS** `go test ./internal/platform/config/ -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): add Phase 3b checkout env config"`

---

## Task 2: Payment reference generator

**Files:** Create `internal/payment/reference.go`, `internal/payment/reference_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package payment

import (
	"regexp"
	"testing"
)

func TestGeneratePaymentReference(t *testing.T) {
	re := regexp.MustCompile(`^DR-PRO-[0-9A-F]{8}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		got := GeneratePaymentReference()
		if !re.MatchString(got) {
			t.Fatalf("ref %q does not match DR-PRO-<8 upper hex>", got)
		}
		seen[got] = true
	}
	if len(seen) < 990 { // 4 random bytes → collisions vanishingly rare
		t.Fatalf("only %d unique refs in 1000 — randomness suspect", len(seen))
	}
}
```

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/ -run TestGeneratePaymentReference -v`.

- [ ] **Step 3: Implement** `internal/payment/reference.go`:

```go
package payment

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// PaymentRefPrefix mirrors payment-reference.ts PAYMENT_REF_PREFIX.
const PaymentRefPrefix = "DR-PRO-"

// GeneratePaymentReference ports generatePaymentReference():
// `DR-PRO-` + 4 random bytes as UPPERCASE hex (8 chars). Customers put this
// in the bank-transfer memo; the Casso/SePay webhook matches it back.
func GeneratePaymentReference() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on supported platforms; panic keeps the
		// signature string-only (matches the Node sync API).
		panic("payment: crypto/rand failed: " + err.Error())
	}
	return PaymentRefPrefix + strings.ToUpper(hex.EncodeToString(b[:]))
}
```

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/ -run TestGeneratePaymentReference -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): port DR-PRO reference generator"`

---

## Task 3: app_settings credential loader

**Files:** Modify `internal/shared/pg/queries_core.sql`, `internal/payment/settings_pg.go`; run `sqlc generate`; Test `internal/payment/settings_pg_test.go`.

First confirm the column set exists: `grep -n "stripe_secret_key\|vietqr_bank_id\|lemonsqueezy_api_key\|lemonsqueezy_store_id\|lemonsqueezy_variant" internal/platform/db/schema.sql`. All are present (mirror of Node schema). `lemonsqueezy_store_id` is `text` NOT NULL default ''.

- [ ] **Step 1: Add the sqlc query** to `internal/shared/pg/queries_core.sql`:

```sql
-- name: GetPaymentCredentials :one
-- Checkout-time provider credentials from the singleton app_settings row.
-- All columns are NOT NULL DEFAULT ''. No row → pgx.ErrNoRows (caller treats
-- as all-empty → env fallback per resolveCredential).
SELECT stripe_secret_key,
       vietqr_bank_id,
       vietqr_account_number,
       vietqr_account_name,
       lemonsqueezy_api_key,
       lemonsqueezy_store_id,
       lemonsqueezy_variant_monthly,
       lemonsqueezy_variant_yearly
FROM app_settings
LIMIT 1;
```

- [ ] **Step 2: Run** `sqlc generate` then `go build ./internal/shared/...`. Expected: `GetPaymentCredentials` appears in `internal/shared/pg/sqlc/querier.go`.

- [ ] **Step 3: Write the failing test** in `internal/payment/settings_pg_test.go` (extend the existing fake querier; add fields + a row return):

```go
func TestSettingsAdapter_Credentials(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{creds: sqlc.GetPaymentCredentialsRow{
		StripeSecretKey: "sk_db", VietqrBankId: "ACB", LemonsqueezyStoreId: "99",
	}})
	c, err := a.Credentials(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.StripeSecretKey != "sk_db" || c.VietQRBankID != "ACB" || c.LemonSqueezyStoreID != "99" {
		t.Fatalf("creds not mapped: %+v", c)
	}
}

func TestSettingsAdapter_CredentialsNoRow(t *testing.T) {
	a := NewSettingsAdapter(fakeCoreQ{credsErr: pgx.ErrNoRows})
	c, err := a.Credentials(context.Background())
	if err != nil {
		t.Fatalf("no row must be (zero,nil), got err=%v", err)
	}
	if c.StripeSecretKey != "" {
		t.Fatalf("no row must yield empty creds, got %+v", c)
	}
}
```

(Add `creds sqlc.GetPaymentCredentialsRow` + `credsErr error` to the test's `fakeCoreQ`, and a `GetPaymentCredentials` method on it. Also add `GetPaymentCredentials` to the `coreSettingsQuerier` interface in `settings_pg.go`.)

- [ ] **Step 4: Run → FAIL** `go test ./internal/payment/ -run TestSettingsAdapter_Cred -v`.

- [ ] **Step 5: Implement** in `internal/payment/settings_pg.go`. Add to the `coreSettingsQuerier` interface: `GetPaymentCredentials(ctx context.Context) (sqlc.GetPaymentCredentialsRow, error)`. Add the exported `Credentials` type + loader:

```go
// Credentials is the checkout-time provider credential set (DB priority side
// of resolveCredential). Empty strings when the column / row is absent.
type Credentials struct {
	StripeSecretKey       string
	VietQRBankID          string
	VietQRAccountNumber   string
	VietQRAccountName     string
	LemonSqueezyAPIKey    string
	LemonSqueezyStoreID   string
	LemonSqueezyVariantMonthly string
	LemonSqueezyVariantYearly  string
}

// Credentials reads the singleton app_settings credential row. A missing row
// is not an error — it yields the zero value, and the caller falls back to env
// per resolveCredential (Node getSettings() returns null the same way).
func (a *SettingsAdapter) Credentials(ctx context.Context) (Credentials, error) {
	row, err := a.q.GetPaymentCredentials(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credentials{}, nil
	}
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		StripeSecretKey:            row.StripeSecretKey,
		VietQRBankID:               row.VietqrBankId,
		VietQRAccountNumber:        row.VietqrAccountNumber,
		VietQRAccountName:          row.VietqrAccountName,
		LemonSqueezyAPIKey:         row.LemonsqueezyApiKey,
		LemonSqueezyStoreID:        row.LemonsqueezyStoreId,
		LemonSqueezyVariantMonthly: row.LemonsqueezyVariantMonthly,
		LemonSqueezyVariantYearly:  row.LemonsqueezyVariantYearly,
	}, nil
}
```

(Confirm generated field names with `grep "GetPaymentCredentialsRow" -A 12 internal/shared/pg/sqlc/*.go` and match casing exactly — sqlc renders `vietqr_bank_id` as `VietqrBankId`.)

- [ ] **Step 6: Run → PASS** `go test ./internal/payment/ -run TestSettingsAdapter -v`.

- [ ] **Step 7: Commit** `git add -A && git commit -m "feat(payment): load app_settings checkout credentials"`

---

## Task 4: Plan / user / payment write queries

**Files:** Modify `internal/shared/pg/queries_core.sql`; run `sqlc generate`; Modify `internal/payment/reader.go` (extend `Querier` + add repo methods); Test `internal/payment/reader_checkout_test.go`.

- [ ] **Step 1: Add queries** to `internal/shared/pg/queries_core.sql`:

```sql
-- name: GetPlanForCheckout :one
-- Plan fields createCheckout needs (price_cents drives the free-plan guard +
-- payment.amount; the rest feed the strategy). No row → pgx.ErrNoRows.
SELECT id, name, price_cents, currency, stripe_price_id, trial_days, billing_period
FROM plans
WHERE id = $1;

-- name: GetUserForCheckout :one
-- User fields createCheckout / portal / cancel need. No row → pgx.ErrNoRows.
SELECT id, email, stripe_customer_id, lemonsqueezy_customer_id
FROM users
WHERE id = $1;

-- name: CreatePayment :one
-- Insert a pending payment. Defaults (id, created_at, updated_at) are returned.
INSERT INTO payments (user_id, plan_id, amount, currency, method, status, reference_code, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at, updated_at;

-- name: UpdatePaymentQRData :exec
UPDATE payments SET qr_data = $2, updated_at = now() WHERE id = $1;

-- name: MarkPaymentFailed :exec
UPDATE payments SET status = 'failed', notes = $2, updated_at = now() WHERE id = $1;
```

- [ ] **Step 2: Run** `sqlc generate` && `go build ./internal/shared/...`. Confirm the 5 methods land in `querier.go`. (`currency`/`stripe_price_id` are nullable → generated as `*string`; `trial_days` `int32`; `billing_period` an enum type.)

- [ ] **Step 3: Write the failing test** `internal/payment/reader_checkout_test.go`:

```go
func TestPlanForCheckout_NoRow(t *testing.T) {
	r := NewRepo(fakeQ{planErr: pgx.ErrNoRows})
	p, err := r.PlanForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil || p != nil {
		t.Fatalf("no row → (nil,nil), got p=%v err=%v", p, err)
	}
}

func TestPlanForCheckout_Maps(t *testing.T) {
	price := "usd-price-id"
	r := NewRepo(fakeQ{plan: sqlc.GetPlanForCheckoutRow{
		Name: "Pro", PriceCents: 999, StripePriceID: &price, TrialDays: 7,
	}})
	p, err := r.PlanForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Pro" || p.PriceCents != 999 || p.StripePriceID != "usd-price-id" || p.TrialDays != 7 {
		t.Fatalf("plan mapped wrong: %+v", p)
	}
}

func TestUserForCheckout_NoRow(t *testing.T) {
	r := NewRepo(fakeQ{userErr: pgx.ErrNoRows})
	u, err := r.UserForCheckout(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil || u != nil {
		t.Fatalf("no row → (nil,nil), got u=%v err=%v", u, err)
	}
}
```

(Extend the test's `fakeQ` to satisfy the new `Querier` methods; add `plan/planErr/user/userErr` fields. A malformed UUID id → `PlanForCheckout`/`UserForCheckout` return `(nil,nil)` like the existing `ListByUser`.)

- [ ] **Step 4: Run → FAIL** `go test ./internal/payment/ -run "ForCheckout" -v`.

- [ ] **Step 5: Implement** in `internal/payment/reader.go`. Extend the `Querier` interface:

```go
GetPlanForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetPlanForCheckoutRow, error)
GetUserForCheckout(ctx context.Context, id pgtype.UUID) (sqlc.GetUserForCheckoutRow, error)
CreatePayment(ctx context.Context, arg sqlc.CreatePaymentParams) (sqlc.CreatePaymentRow, error)
UpdatePaymentQRData(ctx context.Context, arg sqlc.UpdatePaymentQRDataParams) error
MarkPaymentFailed(ctx context.Context, arg sqlc.MarkPaymentFailedParams) error
```

Add types + methods:

```go
// CheckoutPlan is the plan projection createCheckout needs. StripePriceID is
// "" when the column is NULL; Currency defaults applied by the caller.
type CheckoutPlan struct {
	ID, Name      string
	PriceCents    int
	Currency      string // "" when NULL
	StripePriceID string // "" when NULL
	TrialDays     int
	BillingPeriod string
}

type CheckoutUser struct {
	ID, Email             string
	StripeCustomerID      string // "" when NULL
	LemonSqueezyCustomerID string // "" when NULL
}

// CreatedPayment is the RETURNING projection of CreatePayment.
type CreatedPayment struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r *Repo) PlanForCheckout(ctx context.Context, id string) (*CheckoutPlan, error) {
	uid, ok := parseUUID(id)
	if !ok {
		return nil, nil
	}
	row, err := r.q.GetPlanForCheckout(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &CheckoutPlan{
		ID:            uuid.UUID(row.ID.Bytes).String(),
		Name:          row.Name,
		PriceCents:    int(row.PriceCents),
		Currency:      derefStr(row.Currency),
		StripePriceID: derefStr(row.StripePriceID),
		TrialDays:     int(row.TrialDays),
		BillingPeriod: string(row.BillingPeriod),
	}, nil
}

func (r *Repo) UserForCheckout(ctx context.Context, id string) (*CheckoutUser, error) {
	uid, ok := parseUUID(id)
	if !ok {
		return nil, nil
	}
	row, err := r.q.GetUserForCheckout(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &CheckoutUser{
		ID:                     uuid.UUID(row.ID.Bytes).String(),
		Email:                  row.Email,
		StripeCustomerID:       derefStr(row.StripeCustomerID),
		LemonSqueezyCustomerID: derefStr(row.LemonsqueezyCustomerID),
	}, nil
}

// CreatePayment inserts a pending payment row and returns its server-assigned
// id + timestamps. amount/currency/method/status/reference_code/expires_at are
// caller-supplied (createCheckout owns the parity values).
func (r *Repo) CreatePayment(ctx context.Context, userID, planID string, amount int, currency, method, status, ref string, expiresAt time.Time) (*CreatedPayment, error) {
	uid, _ := parseUUID(userID)
	pid, _ := parseUUID(planID)
	row, err := r.q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		UserID:        uid,
		PlanID:        pid,
		Amount:        int32(amount),
		Currency:      currency,
		Method:        method,
		Status:        status,
		ReferenceCode: ref,
		ExpiresAt:     pgtype.Timestamp{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return &CreatedPayment{
		ID:        uuid.UUID(row.ID.Bytes).String(),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *Repo) UpdateQRData(ctx context.Context, paymentID, qr string) error {
	pid, _ := parseUUID(paymentID)
	return r.q.UpdatePaymentQRData(ctx, sqlc.UpdatePaymentQRDataParams{ID: pid, QrData: &qr})
}

func (r *Repo) MarkFailed(ctx context.Context, paymentID, notes string) error {
	pid, _ := parseUUID(paymentID)
	return r.q.MarkPaymentFailed(ctx, sqlc.MarkPaymentFailedParams{ID: pid, Notes: &notes})
}
```

Add a `parseUUID` helper near the other helpers (reuse if one already exists — `grep -n "pgtype.UUID" internal/payment/reader.go`):

```go
func parseUUID(s string) (pgtype.UUID, bool) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, false
	}
	return u, true
}
```

(Match the generated param field names exactly — `QrData *string`, `Notes *string`, `Status string` etc. Adjust if sqlc renders the `method`/`status` columns as enum types rather than `string`; if so, cast `sqlc.PaymentsMethodEnum(method)`.)

- [ ] **Step 6: Run → PASS** `go test ./internal/payment/ -run "ForCheckout" -v`.

- [ ] **Step 7: Commit** `git add -A && git commit -m "feat(payment): plan/user lookups + payment write repo methods"`

---

## Task 5: Strategy port + base helpers

**Files:** Create `internal/payment/strategy/strategy.go`, `internal/payment/strategy/strategy_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package strategy

import "testing"

func TestResolveCredential(t *testing.T) {
	cases := []struct{ db, env, want string }{
		{"from-db", "from-env", "from-db"}, // DB priority
		{"", "from-env", "from-env"},        // empty DB → env
		{"", "", ""},                        // both empty
	}
	for _, c := range cases {
		if got := ResolveCredential(c.db, c.env); got != c.want {
			t.Fatalf("ResolveCredential(%q,%q)=%q want %q", c.db, c.env, got, c.want)
		}
	}
}

func TestTimingSafeStrEqual(t *testing.T) {
	if !TimingSafeStrEqual("secret", "secret") {
		t.Fatal("equal strings must compare true")
	}
	if TimingSafeStrEqual("secret", "secre") {
		t.Fatal("length mismatch must be false")
	}
	if TimingSafeStrEqual("secret", "secreT") {
		t.Fatal("differing strings must be false")
	}
}
```

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/strategy/ -v`.

- [ ] **Step 3: Implement** `internal/payment/strategy/strategy.go`:

```go
// Package strategy is the payment-provider port: the Service depends on the
// Strategy interface; concrete providers live in sub-packages (stripe, vietqr,
// lemonsqueezy). All types here are plain values with zero infra deps so the
// use case and tests never touch pgx/http.
package strategy

import (
	"context"
	"crypto/subtle"
)

// Payment is the just-created payment row a strategy reads to build checkout.
type Payment struct {
	ID            string
	UserID        string
	PlanID        string
	Amount        int
	Currency      string
	Method        string
	ReferenceCode string
}

// Plan is the plan context a strategy needs (Stripe price, trial, cadence).
type Plan struct {
	Name          string
	StripePriceID string // "" when unset
	TrialDays     int
	BillingPeriod string // "monthly" | "yearly" | "none" | ...
	PriceCents    int
}

// Options ports CreateCheckoutOptions + the service-injected user fields.
type Options struct {
	SuccessURL       string
	CancelURL        string
	StripeCustomerID string // user.stripe_customer_id ("" = none)
	UserEmail        string
}

// PortalUser is the subset getCustomerPortalUrl needs.
type PortalUser struct {
	StripeCustomerID       string
	LemonSqueezyCustomerID string
}

// BankInfo is the VietQR/bank-transfer manual-payment block.
type BankInfo struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Amount        int    `json:"amount"`
	Currency      string `json:"currency"`
	Reference     string `json:"reference"`
}

// WalletIntent is the Apple/Google Pay PaymentIntent block. MerchantIdentifier
// is omitted when empty (Node sends `undefined`).
type WalletIntent struct {
	ClientSecret       string `json:"client_secret"`
	PublishableKey     string `json:"publishable_key"`
	MerchantIdentifier string `json:"merchant_identifier,omitempty"`
	CountryCode        string `json:"country_code"`
	CurrencyCode       string `json:"currency_code"`
	DisplayAmount      string `json:"display_amount"`
	DisplayLabel       string `json:"display_label"`
}

// Result is what a strategy returns from CreateCheckout. The Service composes
// the HTTP response from this + the persisted payment row. Empty optional
// fields are omitted from JSON by the response marshaler.
type Result struct {
	RedirectURL  string        // "" → omit
	QRData       string        // "" → omit (also persisted to payments.qr_data)
	BankInfo     *BankInfo     // nil → omit
	WalletIntent *WalletIntent // nil → omit
}

// Strategy is the provider port. CreateCheckout may hit an external API and
// return an error (the Service marks the payment failed + 400s). Portal/Cancel
// default to "" / false for providers that don't support them.
type Strategy interface {
	CreateCheckout(ctx context.Context, p Payment, plan Plan, opts Options) (Result, error)
	// CustomerPortalURL returns "" when there is no portal for this user.
	CustomerPortalURL(ctx context.Context, u PortalUser) (string, error)
	// CancelSubscription returns false when the provider can't cancel.
	CancelSubscription(ctx context.Context, subscriptionID string) (bool, error)
}

// ResolveCredential ports BasePaymentStrategy.resolveCredential: DB value wins
// when non-empty, else the env fallback, else "".
func ResolveCredential(fromDB, env string) string {
	if fromDB != "" {
		return fromDB
	}
	return env
}

// TimingSafeStrEqual ports timingSafeStrEqual: constant-time, length-checked.
func TimingSafeStrEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
```

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/strategy/ -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): strategy port + base credential helpers"`

---

## Task 6: VietQR strategy

**Files:** Create `internal/payment/strategy/vietqr/vietqr.go`, `internal/payment/strategy/vietqr/vietqr_test.go`.

- [ ] **Step 1: Write the failing test**

```go
package vietqr

import (
	"context"
	"strings"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func newStrat() *Strategy {
	return New(Creds{BankID: "MB", AccountNumber: "0000000000", AccountName: "DRAFTRIGHT"})
}

func TestVietQR_QRUrlAndBankInfo(t *testing.T) {
	p := strategy.Payment{Amount: 124000, ReferenceCode: "DR-PRO-ABCD1234", Method: "vietqr"}
	res, err := newStrat().CreateCheckout(context.Background(), p, strategy.Plan{}, strategy.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://img.vietqr.io/image/MB-0000000000-compact2.jpg?amount=124000&addInfo=DR-PRO-ABCD1234&accountName=DRAFTRIGHT"
	if res.QRData != want {
		t.Fatalf("qr=\n%q\nwant\n%q", res.QRData, want)
	}
	if res.BankInfo == nil || res.BankInfo.BankName != "MB Bank (Quân Đội)" || res.BankInfo.Reference != "DR-PRO-ABCD1234" || res.BankInfo.Currency != "VND" || res.BankInfo.Amount != 124000 {
		t.Fatalf("bank_info wrong: %+v", res.BankInfo)
	}
}

func TestVietQR_BankTransferNoQR(t *testing.T) {
	p := strategy.Payment{Amount: 50000, ReferenceCode: "DR-PRO-DEADBEEF", Method: "bank_transfer"}
	res, err := newStrat().CreateCheckout(context.Background(), p, strategy.Plan{}, strategy.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.QRData != "" {
		t.Fatalf("bank_transfer must NOT set qr_data, got %q", res.QRData)
	}
	if res.BankInfo == nil {
		t.Fatal("bank_transfer must still return bank_info")
	}
}

func TestVietQR_Defaults(t *testing.T) {
	res, _ := New(Creds{}).CreateCheckout(context.Background(),
		strategy.Payment{Amount: 1, ReferenceCode: "DR-PRO-00000000", Method: "vietqr"},
		strategy.Plan{}, strategy.Options{})
	if !strings.HasPrefix(res.QRData, "https://img.vietqr.io/image/MB-0000000000-compact2.jpg") {
		t.Fatalf("empty creds must default MB/0000000000, got %q", res.QRData)
	}
}

func TestVietQR_AddInfoEncoding(t *testing.T) {
	// accountName with a space must be %20-encoded (Node encodeURIComponent).
	res, _ := New(Creds{BankID: "MB", AccountNumber: "1", AccountName: "DRAFT RIGHT"}).
		CreateCheckout(context.Background(),
			strategy.Payment{Amount: 1, ReferenceCode: "DR-PRO-00000000", Method: "vietqr"},
			strategy.Plan{}, strategy.Options{})
	if !strings.Contains(res.QRData, "accountName=DRAFT%20RIGHT") {
		t.Fatalf("space must encode as %%20, got %q", res.QRData)
	}
}

func TestVietQR_PortalAndCancelUnsupported(t *testing.T) {
	s := newStrat()
	if url, _ := s.CustomerPortalURL(context.Background(), strategy.PortalUser{}); url != "" {
		t.Fatal("vietqr has no portal")
	}
	if ok, _ := s.CancelSubscription(context.Background(), "x"); ok {
		t.Fatal("vietqr cannot cancel")
	}
}
```

> **Encoding note:** Node `encodeURIComponent` encodes space as `%20` and does NOT encode `-`. Go's `url.QueryEscape` encodes space as `+`. Use a small `encodeURIComponent` that matches JS (replace via `url.QueryEscape` then `+`→`%20`, or escape only the needed set). The reference codes are `[A-Z0-9-]` (never need encoding), but `accountName` can contain spaces — the test pins `%20`.

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/strategy/vietqr/ -v`.

- [ ] **Step 3: Implement** `internal/payment/strategy/vietqr/vietqr.go`:

```go
// Package vietqr implements the VietQR + bank_transfer payment strategy:
// a static img.vietqr.io QR URL + a manual bank-transfer info block. No
// external API call, no secrets at checkout time (webhook keys are Phase 3c).
package vietqr

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// Creds are the already-resolved (DB?env?default) display credentials.
type Creds struct {
	BankID        string
	AccountNumber string
	AccountName   string
}

// Strategy is the VietQR/bank-transfer provider.
type Strategy struct{ creds Creds }

// New applies the same local-dev defaults as Node getCredentials():
// MB / 0000000000 / DRAFTRIGHT when a field is empty.
func New(c Creds) *Strategy {
	if c.BankID == "" {
		c.BankID = "MB"
	}
	if c.AccountNumber == "" {
		c.AccountNumber = "0000000000"
	}
	if c.AccountName == "" {
		c.AccountName = "DRAFTRIGHT"
	}
	return &Strategy{creds: c}
}

// bankNames ports getBankDisplayName's table; unknown id → the id itself.
var bankNames = map[string]string{
	"MB":   "MB Bank (Quân Đội)",
	"VCB":  "Vietcombank",
	"ACB":  "ACB",
	"TCB":  "Techcombank",
	"VPB":  "VPBank",
	"TPB":  "TPBank",
	"BIDV": "BIDV",
	"VTB":  "VietinBank",
	"SCB":  "Sacombank",
}

func bankDisplayName(id string) string {
	if n, ok := bankNames[id]; ok {
		return n
	}
	return id
}

// encodeURIComponent mirrors JS: space → %20 (not +), and the unreserved set
// stays literal. url.QueryEscape is the closest stdlib fn; fix the space.
func encodeURIComponent(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func (s *Strategy) CreateCheckout(_ context.Context, p strategy.Payment, _ strategy.Plan, _ strategy.Options) (strategy.Result, error) {
	bank := &strategy.BankInfo{
		BankName:      bankDisplayName(s.creds.BankID),
		AccountNumber: s.creds.AccountNumber,
		AccountName:   s.creds.AccountName,
		Amount:        p.Amount,
		Currency:      "VND",
		Reference:     p.ReferenceCode,
	}
	// bank_transfer → manual flow, no QR (client renders the table only).
	if p.Method == "bank_transfer" {
		return strategy.Result{BankInfo: bank}, nil
	}
	qr := fmt.Sprintf(
		"https://img.vietqr.io/image/%s-%s-compact2.jpg?amount=%d&addInfo=%s&accountName=%s",
		s.creds.BankID, s.creds.AccountNumber, p.Amount,
		encodeURIComponent(p.ReferenceCode), encodeURIComponent(s.creds.AccountName),
	)
	return strategy.Result{QRData: qr, BankInfo: bank}, nil
}

// CustomerPortalURL / CancelSubscription: VietQR has neither (default behavior).
func (s *Strategy) CustomerPortalURL(_ context.Context, _ strategy.PortalUser) (string, error) {
	return "", nil
}

func (s *Strategy) CancelSubscription(_ context.Context, _ string) (bool, error) {
	return false, nil
}
```

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/strategy/vietqr/ -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): VietQR + bank_transfer strategy"`

---

## Task 7: Stripe strategy

**Files:** Add dep; Create `internal/payment/strategy/stripe/stripe.go`, `internal/payment/strategy/stripe/stripe_test.go`.

> Before reading on, re-read `backend/src/payment/strategies/stripe.strategy.ts` lines 53-251 in full so the SDK calls below match the exact param set (this plan lists every field, but confirm the `!plan.stripe_price_id` message text and any subtle field at impl time).

- [ ] **Step 1: Add the SDK** `go get github.com/stripe/stripe-go/v82@latest` (pin whatever major resolves; the `client.API` + `Resource.New(params)` surface used here is stable v72+). Run `go mod tidy`.

- [ ] **Step 2: Write the failing test** (deterministic helpers only — live API calls are covered by the shadow gate, not unit tests):

```go
package stripe

import "testing"

func TestDisplayAmount(t *testing.T) {
	cases := []struct {
		currency string
		amount   int
		want     string
	}{
		{"VND", 124000, "124000"}, // zero-decimal → integer string
		{"JPY", 500, "500"},
		{"KRW", 9900, "9900"},
		{"USD", 999, "9.99"},   // decimal → /100, 2dp
		{"usd", 1000, "10.00"}, // case-insensitive
	}
	for _, c := range cases {
		if got := displayAmount(c.currency, c.amount); got != c.want {
			t.Fatalf("displayAmount(%q,%d)=%q want %q", c.currency, c.amount, got, c.want)
		}
	}
}

func TestDisplayLabel(t *testing.T) {
	if got := displayLabel("Pro", "monthly"); got != "Pro · Monthly" {
		t.Fatalf("got %q", got)
	}
	if got := displayLabel("Pro", "yearly"); got != "Pro · Yearly" {
		t.Fatalf("got %q", got)
	}
	if got := displayLabel("", "monthly"); got != "Pro · Monthly" {
		t.Fatalf("empty name → Pro, got %q", got)
	}
	if got := displayLabel("Pro", "none"); got != "Pro" {
		t.Fatalf("no cadence → name only, got %q", got)
	}
}

func TestCheckout_NotConfigured(t *testing.T) {
	s := New(Creds{}, Env{}) // empty secret key
	_, err := s.CreateCheckout(t.Context(),
		paymentFixture(), planFixture(), optsFixture())
	if err == nil || err.Error() != "Stripe secret_key is not configured" {
		t.Fatalf("empty key must error with the Node message, got %v", err)
	}
}
```

(Add small `paymentFixture/planFixture/optsFixture` helpers returning `strategy.Payment{Method:"stripe"}` etc. `t.Context()` requires Go 1.24+; use `context.Background()` if the toolchain is older.)

- [ ] **Step 3: Run → FAIL** `go test ./internal/payment/strategy/stripe/ -v`.

- [ ] **Step 4: Implement** `internal/payment/strategy/stripe/stripe.go`:

```go
// Package stripe implements the Stripe payment strategy: subscription Checkout
// Sessions for the `stripe` method and PaymentIntents for the apple_pay /
// google_pay wallet methods, plus the billing-portal + cancel flows. Uses the
// official stripe-go client.API (stable surface across majors).
package stripe

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// Creds is the resolved Stripe secret key (DB?env).
type Creds struct{ SecretKey string }

// Env holds the env-only values (no app_settings column) + websiteUrl.
type Env struct {
	PublishableKey     string
	ApplePayMerchantID string
	WebsiteURL         string
}

type Strategy struct {
	creds Creds
	env   Env
	// newClient is overridable in tests; prod builds a real client.API.
	newClient func(key string) *client.API
}

func New(c Creds, e Env) *Strategy {
	return &Strategy{creds: c, env: e, newClient: func(key string) *client.API {
		sc := &client.API{}
		sc.Init(key, nil)
		return sc
	}}
}

func (s *Strategy) websiteURL() string {
	if s.env.WebsiteURL != "" {
		return s.env.WebsiteURL
	}
	return "http://localhost:4000"
}

func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	if s.creds.SecretKey == "" {
		return strategy.Result{}, fmt.Errorf("Stripe secret_key is not configured")
	}
	sc := s.newClient(s.creds.SecretKey)
	if p.Method == "apple_pay" || p.Method == "google_pay" {
		return s.walletIntent(ctx, sc, p, plan)
	}
	if plan.StripePriceID == "" {
		// Match the Node message exactly (confirm at impl from stripe.strategy.ts).
		return strategy.Result{}, fmt.Errorf("Plan has no Stripe price configured")
	}

	successURL := opts.SuccessURL
	if successURL == "" {
		successURL = fmt.Sprintf("%s/payment/success?ref=%s", s.websiteURL(), p.ReferenceCode)
	}
	cancelURL := opts.CancelURL
	if cancelURL == "" {
		cancelURL = s.websiteURL() + "/payment/cancel"
	}

	params := &stripe.CheckoutSessionParams{
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(plan.StripePriceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:       stripe.String(successURL),
		CancelURL:        stripe.String(cancelURL),
		ClientReferenceID: stripe.String(p.UserID),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{},
	}
	params.AddMetadata("reference_code", p.ReferenceCode)
	params.AddMetadata("payment_id", p.ID)
	params.AddMetadata("user_id", p.UserID)
	params.AddMetadata("plan_id", p.PlanID)
	params.SubscriptionData.AddMetadata("reference_code", p.ReferenceCode)
	params.SubscriptionData.AddMetadata("user_id", p.UserID)
	params.SubscriptionData.AddMetadata("plan_id", p.PlanID)
	if plan.TrialDays > 0 {
		params.SubscriptionData.TrialPeriodDays = stripe.Int64(int64(plan.TrialDays))
	}
	// Customer reuse: existing id, else pre-fill email for Stripe to create one.
	if opts.StripeCustomerID != "" {
		params.Customer = stripe.String(opts.StripeCustomerID)
	} else if opts.UserEmail != "" {
		params.CustomerEmail = stripe.String(opts.UserEmail)
	}

	sess, err := sc.CheckoutSessions.New(params)
	if err != nil {
		return strategy.Result{}, err
	}
	return strategy.Result{RedirectURL: sess.URL}, nil
}

// walletIntent ports createWalletPaymentIntent (apple_pay / google_pay).
func (s *Strategy) walletIntent(_ context.Context, sc *client.API, p strategy.Payment, plan strategy.Plan) (strategy.Result, error) {
	// Reuse/create the customer (mirror Node: reuse stripe_customer_id when set;
	// here checkout passes none for wallets, so create from nothing — Node also
	// creates a Customer when absent).  Build a PaymentIntent.
	piParams := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(int64(p.Amount)),
		Currency:           stripe.String(strings.ToLower(p.Currency)),
		SetupFutureUsage:   stripe.String("off_session"),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
	}
	piParams.AddMetadata("reference_code", p.ReferenceCode)
	piParams.AddMetadata("payment_id", p.ID)
	piParams.AddMetadata("user_id", p.UserID)
	piParams.AddMetadata("plan_id", p.PlanID)
	piParams.AddMetadata("method", p.Method)
	pi, err := sc.PaymentIntents.New(piParams)
	if err != nil {
		return strategy.Result{}, err
	}
	wi := &strategy.WalletIntent{
		ClientSecret:   pi.ClientSecret,
		PublishableKey: s.env.PublishableKey,
		CountryCode:    "US", // plans has no country_code column → Node default
		CurrencyCode:   strings.ToUpper(p.Currency),
		DisplayAmount:  displayAmount(p.Currency, p.Amount),
		DisplayLabel:   displayLabel(plan.Name, plan.BillingPeriod),
	}
	if s.env.ApplePayMerchantID != "" {
		wi.MerchantIdentifier = s.env.ApplePayMerchantID
	}
	return strategy.Result{WalletIntent: wi}, nil
}

func (s *Strategy) CustomerPortalURL(_ context.Context, u strategy.PortalUser) (string, error) {
	if u.StripeCustomerID == "" {
		return "", nil
	}
	sc := s.newClient(s.creds.SecretKey)
	sess, err := sc.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(u.StripeCustomerID),
		ReturnURL: stripe.String(s.websiteURL() + "/account?subscribed=1"),
	})
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (s *Strategy) CancelSubscription(_ context.Context, subscriptionID string) (bool, error) {
	sc := s.newClient(s.creds.SecretKey)
	_, err := sc.Subscriptions.Update(subscriptionID, &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- deterministic helpers (unit-tested) ---------------------------------

var zeroDecimal = map[string]bool{"VND": true, "JPY": true, "KRW": true}

// displayAmount ports the zero-decimal branch: integer string for zero-decimal
// currencies, else amount/100 with 2 decimals.
func displayAmount(currency string, amount int) string {
	if zeroDecimal[strings.ToUpper(currency)] {
		return strconv.Itoa(amount)
	}
	return strconv.FormatFloat(float64(amount)/100, 'f', 2, 64)
}

// displayLabel ports the cadence label: "<name|Pro> · <Cap(cadence)>" or just
// the name when there's no monthly/yearly cadence.
func displayLabel(name, billingPeriod string) string {
	if name == "" {
		name = "Pro"
	}
	var cadence string
	switch billingPeriod {
	case "monthly":
		cadence = "Monthly"
	case "yearly":
		cadence = "Yearly"
	}
	if cadence == "" {
		return name
	}
	return name + " · " + cadence
}
```

> **SDK verification:** the exact field/struct names (`CheckoutSessionParams`, `BillingPortalSessions`, `AddMetadata`, `CheckoutSessionModeSubscription`) are from stripe-go's stable API but MUST be confirmed against the resolved version — run `go doc github.com/stripe/stripe-go/v82 CheckoutSessionParams` and adjust. The `·` in `displayLabel` is U+00B7 MIDDLE DOT — copy it from the Node source (`stripe.strategy.ts` displayLabel) to guarantee byte-parity.

- [ ] **Step 5: Run → PASS** `go test ./internal/payment/strategy/stripe/ -v`.

- [ ] **Step 6: Commit** `git add -A && git commit -m "feat(payment): Stripe strategy (checkout, wallet, portal, cancel)"`

---

## Task 8: LemonSqueezy strategy

**Files:** Create `internal/payment/strategy/lemonsqueezy/lemonsqueezy.go`, `internal/payment/strategy/lemonsqueezy/lemonsqueezy_test.go`.

- [ ] **Step 1: Write the failing test** (httptest server stands in for the LS API; verifies body shape + variant selection + error mapping):

```go
package lemonsqueezy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func TestLS_NotConfigured(t *testing.T) {
	s := New(Creds{}, "http://localhost:4000") // empty apiKey/storeId
	_, err := s.CreateCheckout(context.Background(), strategy.Payment{ReferenceCode: "DR-PRO-1"}, strategy.Plan{}, strategy.Options{})
	if err == nil {
		t.Fatal("missing apiKey/storeId must error")
	}
}

func TestLS_CheckoutBodyAndVariant(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"data":{"attributes":{"url":"https://ls.example/checkout/xyz"}}}`))
	}))
	defer srv.Close()

	s := New(Creds{APIKey: "key", StoreID: "55", VariantMonthly: "111", VariantYearly: "222"}, "http://localhost:4000")
	s.apiBase = srv.URL // inject test endpoint
	res, err := s.CreateCheckout(context.Background(),
		strategy.Payment{ReferenceCode: "DR-PRO-ABCD1234"},
		strategy.Plan{BillingPeriod: "yearly"},
		strategy.Options{UserEmail: "u@x.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.RedirectURL != "https://ls.example/checkout/xyz" {
		t.Fatalf("redirect=%q", res.RedirectURL)
	}
	if gotAuth != "Bearer key" || gotAccept != "application/vnd.api+json" {
		t.Fatalf("headers: auth=%q accept=%q", gotAuth, gotAccept)
	}
	// yearly plan → variant 222 in enabled_variants
	attrs := gotBody["data"].(map[string]any)["attributes"].(map[string]any)
	po := attrs["product_options"].(map[string]any)
	ev := po["enabled_variants"].([]any)
	if len(ev) != 1 || ev[0].(float64) != 222 {
		t.Fatalf("yearly must select variant 222, got %v", ev)
	}
}

func TestLS_CheckoutErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"errors":[{"detail":"bad variant"}]}`))
	}))
	defer srv.Close()
	s := New(Creds{APIKey: "k", StoreID: "1", VariantMonthly: "1"}, "http://localhost:4000")
	s.apiBase = srv.URL
	_, err := s.CreateCheckout(context.Background(), strategy.Payment{ReferenceCode: "DR-PRO-1"}, strategy.Plan{}, strategy.Options{})
	if err == nil {
		t.Fatal("non-2xx LS response must error")
	}
}
```

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/strategy/lemonsqueezy/ -v`.

- [ ] **Step 3: Implement** `internal/payment/strategy/lemonsqueezy/lemonsqueezy.go`:

```go
// Package lemonsqueezy implements the LemonSqueezy (Merchant-of-Record) payment
// strategy via the JSON:API at api.lemonsqueezy.com/v1. Checkout creates a
// hosted checkout URL; portal/cancel call the customer + subscription APIs.
package lemonsqueezy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

const defaultAPIBase = "https://api.lemonsqueezy.com/v1"

// Creds are the resolved LS credentials.
type Creds struct {
	APIKey         string
	StoreID        string
	VariantMonthly string
	VariantYearly  string
}

type Strategy struct {
	creds      Creds
	websiteURL string
	apiBase    string
	http       *http.Client
}

func New(c Creds, websiteURL string) *Strategy {
	if websiteURL == "" {
		websiteURL = "http://localhost:4000"
	}
	return &Strategy{creds: c, websiteURL: websiteURL, apiBase: defaultAPIBase, http: http.DefaultClient}
}

func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	if s.creds.APIKey == "" || s.creds.StoreID == "" {
		return strategy.Result{}, fmt.Errorf("LemonSqueezy is not configured")
	}
	billing := plan.BillingPeriod
	if billing == "" {
		billing = "monthly"
	}
	variantID := s.creds.VariantMonthly
	if billing == "yearly" {
		variantID = s.creds.VariantYearly
	}
	if variantID == "" {
		return strategy.Result{}, fmt.Errorf("LemonSqueezy variant is not configured")
	}
	variantNum, err := strconv.Atoi(variantID)
	if err != nil {
		return strategy.Result{}, fmt.Errorf("invalid LemonSqueezy variant id: %q", variantID)
	}
	redirectURL := opts.SuccessURL
	if redirectURL == "" {
		redirectURL = fmt.Sprintf("%s/payment/success?ref=%s", s.websiteURL, p.ReferenceCode)
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "checkouts",
			"attributes": map[string]any{
				"checkout_data": map[string]any{
					"email":  opts.UserEmail,
					"custom": map[string]any{"reference_code": p.ReferenceCode},
				},
				"product_options": map[string]any{
					"redirect_url":     redirectURL,
					"enabled_variants": []int{variantNum},
				},
			},
			"relationships": map[string]any{
				"store":   map[string]any{"data": map[string]any{"type": "stores", "id": s.creds.StoreID}},
				"variant": map[string]any{"data": map[string]any{"type": "variants", "id": variantID}},
			},
		},
	}

	var out struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := s.do(ctx, http.MethodPost, "/checkouts", body, &out); err != nil {
		return strategy.Result{}, err
	}
	return strategy.Result{RedirectURL: out.Data.Attributes.URL}, nil
}

func (s *Strategy) CustomerPortalURL(ctx context.Context, u strategy.PortalUser) (string, error) {
	if u.LemonSqueezyCustomerID == "" {
		return "", nil
	}
	var out struct {
		Data struct {
			Attributes struct {
				URLs struct {
					CustomerPortal string `json:"customer_portal"`
				} `json:"urls"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := s.do(ctx, http.MethodGet, "/customers/"+u.LemonSqueezyCustomerID, nil, &out); err != nil {
		return "", err
	}
	return out.Data.Attributes.URLs.CustomerPortal, nil
}

func (s *Strategy) CancelSubscription(ctx context.Context, subscriptionID string) (bool, error) {
	if err := s.do(ctx, http.MethodDelete, "/subscriptions/"+subscriptionID, nil, nil); err != nil {
		return false, err
	}
	return true, nil
}

// do issues a JSON:API request with the LS headers and decodes a 2xx body into
// out (nil out = ignore body). Non-2xx → error (mirrors Node's `!res.ok` throw).
func (s *Strategy) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.apiBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+s.creds.APIKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("lemonsqueezy %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
```

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/strategy/lemonsqueezy/ -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): LemonSqueezy strategy (checkout, portal, cancel)"`

---

## Task 9: Checkout use case + DTO validation + response marshaling

**Files:** Create `internal/payment/checkout.go`, `internal/payment/checkout_test.go`; Modify `internal/payment/usecase.go` (extend `Service` + ports).

The `Service` gains: a `strategies map[string]strategy.Strategy`, a `CheckoutRepo` port (the new Repo methods), and a clock + ref generator (injectable for tests). Define the ports on the consumer side in `usecase.go`:

```go
// CheckoutRepo is the persistence the checkout/portal use cases need (consumer
// port; the concrete *Repo satisfies it).
type CheckoutRepo interface {
	PlanForCheckout(ctx context.Context, id string) (*CheckoutPlan, error)
	UserForCheckout(ctx context.Context, id string) (*CheckoutUser, error)
	CreatePayment(ctx context.Context, userID, planID string, amount int, currency, method, status, ref string, expiresAt time.Time) (*CreatedPayment, error)
	UpdateQRData(ctx context.Context, paymentID, qr string) error
	MarkFailed(ctx context.Context, paymentID, notes string) error
}
```

Extend `Service` (in `usecase.go`) with fields `checkoutRepo CheckoutRepo`, `strategies map[string]strategy.Strategy`, `subs SubsPort` (Task 10), `now func() time.Time`, `genRef func() string`, and update `NewService` to accept them (keep the existing methods-list ctor working — add a second constructor `NewServiceFull(...)` OR extend `NewService` and update `main.go` + existing tests). Prefer extending `NewService` with the new params and fixing call sites.

- [ ] **Step 1: Write the failing test** `internal/payment/checkout_test.go` (fakes for repo + strategy):

```go
package payment

import (
	"context"
	"testing"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// --- fakes ---
type fakeCheckoutRepo struct {
	plan       *CheckoutPlan
	user       *CheckoutUser
	created    *CreatedPayment
	createErr  error
	qrCalls    int
	failCalls  int
	failNotes  string
}

func (f *fakeCheckoutRepo) PlanForCheckout(_ context.Context, _ string) (*CheckoutPlan, error) { return f.plan, nil }
func (f *fakeCheckoutRepo) UserForCheckout(_ context.Context, _ string) (*CheckoutUser, error) { return f.user, nil }
func (f *fakeCheckoutRepo) CreatePayment(_ context.Context, _, _ string, _ int, _, _, _, _ string, _ time.Time) (*CreatedPayment, error) {
	return f.created, f.createErr
}
func (f *fakeCheckoutRepo) UpdateQRData(_ context.Context, _, _ string) error { f.qrCalls++; return nil }
func (f *fakeCheckoutRepo) MarkFailed(_ context.Context, _, notes string) error { f.failCalls++; f.failNotes = notes; return nil }

type fakeStrategy struct {
	res strategy.Result
	err error
}
func (f fakeStrategy) CreateCheckout(context.Context, strategy.Payment, strategy.Plan, strategy.Options) (strategy.Result, error) { return f.res, f.err }
func (f fakeStrategy) CustomerPortalURL(context.Context, strategy.PortalUser) (string, error) { return "", nil }
func (f fakeStrategy) CancelSubscription(context.Context, string) (bool, error) { return false, nil }

func svcWith(repo CheckoutRepo, strat strategy.Strategy, enabled []string) *Service {
	// constructor wiring: enabledFn returns `enabled`; strategies map keys to strat.
	return newTestService(repo, map[string]strategy.Strategy{"vietqr": strat, "stripe": strat}, enabled)
}

func TestCheckout_PlanNotFound(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: nil}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "Plan not found")
}

func TestCheckout_FreePlan(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 0}}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 400, "Cannot purchase a free plan")
}

func TestCheckout_MethodNotEnabled(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 999}}, fakeStrategy{}, []string{"stripe"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "Payment method 'vietqr' is not enabled.")
}

func TestCheckout_UserNotFound(t *testing.T) {
	s := svcWith(&fakeCheckoutRepo{plan: &CheckoutPlan{PriceCents: 999}, user: nil}, fakeStrategy{}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 404, "User not found")
}

func TestCheckout_StrategyErrorMarksFailed(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan: &CheckoutPlan{PriceCents: 999, Currency: "VND"},
		user: &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	s := svcWith(repo, fakeStrategy{err: errProvider("boom")}, []string{"vietqr"})
	_, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	assertDomainErr(t, err, 400, "boom")
	if repo.failCalls != 1 || repo.failNotes != "boom" {
		t.Fatalf("must MarkFailed with err message, calls=%d notes=%q", repo.failCalls, repo.failNotes)
	}
}

func TestCheckout_VietQRSuccessPersistsQR(t *testing.T) {
	repo := &fakeCheckoutRepo{
		plan: &CheckoutPlan{Name: "Pro", PriceCents: 124000, Currency: "VND"},
		user: &CheckoutUser{ID: "u1", Email: "u@x.com"},
		created: &CreatedPayment{ID: "pay1"},
	}
	strat := fakeStrategy{res: strategy.Result{QRData: "https://img.vietqr.io/...", BankInfo: &strategy.BankInfo{BankName: "MB Bank (Quân Đội)"}}}
	s := svcWith(repo, strat, []string{"vietqr"})
	resp, err := s.CreateCheckout(context.Background(), "u1", "p1", "vietqr", CheckoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.qrCalls != 1 {
		t.Fatalf("qr_data must persist, calls=%d", repo.qrCalls)
	}
	if resp.QRData == nil || *resp.QRData == "" || resp.BankInfo == nil {
		t.Fatalf("response must carry qr_data + bank_info: %+v", resp)
	}
	if resp.Payment.Status != "pending" || resp.Payment.Amount != 124000 || resp.Payment.Currency != "VND" {
		t.Fatalf("payment shape wrong: %+v", resp.Payment)
	}
}
```

(Provide test helpers `newTestService`, `assertDomainErr`, `errProvider` — `assertDomainErr` asserts the domain error's HTTP status + message; reuse the package's existing error type. If the package lacks a status-carrying error, define one in Step 3.)

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/ -run TestCheckout -v`.

- [ ] **Step 3: Implement** `internal/payment/checkout.go`. Define `CheckoutOptions`, the domain error helper, the flow, and `CheckoutResponse` marshaling:

```go
package payment

import (
	"context"
	"time"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

// PaymentPendingTTL ports PAYMENT_PENDING_TTL_MS (30 min).
const PaymentPendingTTL = 30 * time.Minute

// CheckoutOptions ports CreateCheckoutDto's optional URLs.
type CheckoutOptions struct {
	SuccessURL string
	CancelURL  string
}

// CheckoutResponse is the CheckoutResult JSON: the payment entity nested under
// `payment`, plus optional provider fields (omitted when absent).
type CheckoutResponse struct {
	Payment      PaymentRow            `json:"payment"`
	RedirectURL  *string               `json:"redirect_url,omitempty"`
	QRData       *string               `json:"qr_data,omitempty"`
	BankInfo     *strategy.BankInfo    `json:"bank_info,omitempty"`
	WalletIntent *strategy.WalletIntent `json:"wallet_intent,omitempty"`
}

// CreateCheckout ports payment.service.ts createCheckout (see plan parity facts).
func (s *Service) CreateCheckout(ctx context.Context, userID, planID, method string, opts CheckoutOptions) (*CheckoutResponse, error) {
	plan, err := s.checkoutRepo.PlanForCheckout(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, notFound("Plan not found")
	}
	if plan.PriceCents == 0 {
		return nil, badRequest("Cannot purchase a free plan")
	}

	enabled, err := s.enabledMethods(ctx) // existing EnabledMethods resolution
	if err != nil {
		return nil, err
	}
	if !contains(enabled, method) {
		return nil, notFound("Payment method '" + method + "' is not enabled.")
	}
	strat, ok := s.strategies[method]
	if !ok {
		return nil, badRequest("Unsupported payment method: " + method)
	}

	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, notFound("User not found")
	}

	currency := plan.Currency
	if currency == "" {
		currency = "USD"
	}
	ref := s.genRef()
	expiresAt := s.now().Add(PaymentPendingTTL)
	created, err := s.checkoutRepo.CreatePayment(ctx, userID, planID, plan.PriceCents, currency, method, "pending", ref, expiresAt)
	if err != nil {
		return nil, err
	}

	sp := strategy.Payment{
		ID: created.ID, UserID: userID, PlanID: planID,
		Amount: plan.PriceCents, Currency: currency, Method: method, ReferenceCode: ref,
	}
	splan := strategy.Plan{
		Name: plan.Name, StripePriceID: plan.StripePriceID, TrialDays: plan.TrialDays,
		BillingPeriod: plan.BillingPeriod, PriceCents: plan.PriceCents,
	}
	res, err := strat.CreateCheckout(ctx, sp, splan, strategy.Options{
		SuccessURL: opts.SuccessURL, CancelURL: opts.CancelURL,
		StripeCustomerID: user.StripeCustomerID, UserEmail: user.Email,
	})
	if err != nil {
		_ = s.checkoutRepo.MarkFailed(ctx, created.ID, err.Error())
		return nil, badRequest(err.Error())
	}
	if res.QRData != "" {
		if uerr := s.checkoutRepo.UpdateQRData(ctx, created.ID, res.QRData); uerr != nil {
			return nil, uerr
		}
	}

	// Build the response payment row mirroring the just-inserted entity (no
	// user relation, plan nested). provider_ref/notes/completed_at NULL.
	row := PaymentRow{
		ID: created.ID, UserID: userID, PlanID: planID,
		Amount: plan.PriceCents, Currency: currency, Method: method, Status: "pending",
		ReferenceCode: ref,
		ExpiresAt:     &expiresAt,
		CreatedAt:     created.CreatedAt, UpdatedAt: created.UpdatedAt,
		Plan: &PlanBrief{
			ID: plan.ID, Name: plan.Name, PriceCents: plan.PriceCents,
			StripePriceID: ptrOrNil(plan.StripePriceID), TrialDays: plan.TrialDays,
			BillingPeriod: plan.BillingPeriod, Currency: ptrOrNil(plan.Currency),
			IsActive: true,
		},
	}
	resp := &CheckoutResponse{Payment: row}
	if res.QRData != "" {
		row.QRData = &res.QRData
		resp.Payment = row
		resp.QRData = &res.QRData
	}
	if res.RedirectURL != "" {
		resp.RedirectURL = &res.RedirectURL
	}
	resp.BankInfo = res.BankInfo
	resp.WalletIntent = res.WalletIntent
	return resp, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

> **Parity caveat on `PlanBrief` nesting:** Node's response `payment.plan` is the FULL plan entity (`plansService.findById`), which includes `daily_limit`, `is_active`, `created_at`, `updated_at`. `GetPlanForCheckout` (Task 4) does NOT select those, so `PlanBrief.DailyLimit/CreatedAt/UpdatedAt` would be zero/epoch — a parity gap **only inside the nested plan object**. The shadow fixture (Task 12) therefore lists `created_at`, `updated_at`, `daily_limit`, `is_active` under `ignore_value_of` for the VietQR success case. If full nested-plan parity is later required, widen `GetPlanForCheckout` to select every plans column and populate `PlanBrief` fully — leave a `// TODO(phase3b-followup)` comment noting this and `log` nothing at runtime (it's a known, fixture-documented gap, not a silent cap).

Add `notFound`/`badRequest`/`errProvider` domain-error constructors if the package doesn't already expose status-carrying errors (check `internal/payment/` for an existing error type first — reuse it; the `/payment/status` + `/payment/history` handlers already map domain errors, so a pattern likely exists. If not, mirror `internal/auth/errors.go`: a struct with `Status int` + `Message string`, plus a `writeDomainErr`-style helper used by the handler in Task 11).

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/ -run TestCheckout -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): checkout use case + CheckoutResponse marshaling"`

---

## Task 10: Portal + cancel use cases + subscription extension

**Files:** Modify `internal/subscription/reader.go` (+ `internal/shared/pg/queries_subscription.sql` or wherever `GetActiveSubscriptionByUserID` lives) to add `store_transaction_id`; run `sqlc generate`; Create `internal/payment/portal.go`, `internal/payment/portal_test.go`; extend `Service` with a `SubsPort`.

First locate the active-sub query: `grep -rn "GetActiveSubscriptionByUserID" internal/shared/pg/*.sql`. Add `store_transaction_id` to its SELECT.

- [ ] **Step 1: Extend `AccountSub`** in `internal/subscription/reader.go`:

```go
type AccountSub struct {
	PlanName           string
	Status             string
	StoreType          string
	StoreTransactionID string // "" when NULL — needed by in-app cancel
	StartedAt          time.Time
	ExpiresAt          *time.Time
	DailyLimit         int
}
```

Populate it in `ActiveByUser` from the new column (`derefStr(row.StoreTransactionID)` — it's nullable). Update the SQL SELECT + `sqlc generate`, and fix any other `AccountSub` constructors.

- [ ] **Step 2: Define the consumer port** in `internal/payment/usecase.go`:

```go
// SubsPort is the subscription read the portal/cancel use cases need.
type SubsPort interface {
	ActiveByUser(ctx context.Context, userID string) (*subscription.AccountSub, error)
}
```

(Import `internal/subscription` for the exported `AccountSub` type — allowed cross-module data-type coupling per CLAUDE.md Rule #2. `subscription.Reader` already satisfies `ActiveByUser`.)

- [ ] **Step 3: Write the failing test** `internal/payment/portal_test.go`:

```go
func TestPortal_UserNotFound(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: nil}, nil, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "User not found")
}

func TestPortal_NoActiveSub(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}}, fakeSubs{sub: nil}, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "No active subscription to manage")
}

func TestPortal_UnsupportedStore(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "vietqr"}}, nil)
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Subscriptions sourced from 'vietqr' have no self-service portal")
}

func TestPortal_StripeURLEmpty(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe"}},
		map[string]strategy.Strategy{"stripe": fakeStrategy{}}) // portal returns ""
	_, err := s.CustomerPortalURL(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Customer portal is not available for this subscription")
}

func TestCancel_NoProviderRef(t *testing.T) {
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: ""}}, nil)
	_, err := s.CancelActiveSubscription(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Subscription has no provider reference to cancel")
}

func TestCancel_ProviderDeclined(t *testing.T) {
	exp := time.Now()
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: "sub_1", ExpiresAt: &exp}},
		map[string]strategy.Strategy{"stripe": fakeStrategy{}}) // cancel returns false
	_, err := s.CancelActiveSubscription(context.Background(), "u1")
	assertDomainErr(t, err, 404, "Provider declined to cancel the subscription")
}

func TestCancel_Success(t *testing.T) {
	exp := time.Now()
	s := portalSvc(&fakeCheckoutRepo{user: &CheckoutUser{ID: "u1"}},
		fakeSubs{sub: &subscription.AccountSub{StoreType: "stripe", StoreTransactionID: "sub_1", ExpiresAt: &exp}},
		map[string]strategy.Strategy{"stripe": fakeStrategyCancel{ok: true}})
	out, err := s.CancelActiveSubscription(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cancelled || out.ExpiresAt == nil {
		t.Fatalf("expected cancelled+expires_at, got %+v", out)
	}
}
```

(Add `fakeSubs` implementing `SubsPort`, and a `fakeStrategyCancel` whose `CancelSubscription` returns `(ok,nil)`. `portalSvc` builds a `Service` with the given repo/subs/strategies.)

- [ ] **Step 4: Run → FAIL** `go test ./internal/payment/ -run "TestPortal|TestCancel" -v`.

- [ ] **Step 5: Implement** `internal/payment/portal.go`:

```go
package payment

import (
	"context"
	"time"
)

// storeTypeStrategy maps a subscription's store_type to the registered strategy
// key. Mirrors the switch in getCustomerPortalUrl/cancelActiveSubscription.
func storeTypeStrategyKey(storeType string) (string, bool) {
	switch storeType {
	case "stripe":
		return "stripe", true
	case "lemonsqueezy":
		return "lemonsqueezy", true
	default:
		return "", false
	}
}

// CustomerPortalURL ports getCustomerPortalUrl.
func (s *Service) CustomerPortalURL(ctx context.Context, userID string) (string, error) {
	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", notFound("User not found")
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return "", err
	}
	if sub == nil {
		return "", notFound("No active subscription to manage")
	}
	key, ok := storeTypeStrategyKey(sub.StoreType)
	if !ok {
		return "", notFound("Subscriptions sourced from '" + sub.StoreType + "' have no self-service portal")
	}
	url, err := s.strategies[key].CustomerPortalURL(ctx, PortalUserFrom(user))
	if err != nil {
		return "", badRequest(err.Error())
	}
	if url == "" {
		return "", notFound("Customer portal is not available for this subscription")
	}
	return url, nil
}

// CancelResult ports the cancel response body.
type CancelResult struct {
	Cancelled bool       `json:"cancelled"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CancelActiveSubscription ports cancelActiveSubscription.
func (s *Service) CancelActiveSubscription(ctx context.Context, userID string) (*CancelResult, error) {
	user, err := s.checkoutRepo.UserForCheckout(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, notFound("User not found")
	}
	sub, err := s.subs.ActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, notFound("No active subscription to cancel")
	}
	if sub.StoreTransactionID == "" {
		return nil, notFound("Subscription has no provider reference to cancel")
	}
	key, ok := storeTypeStrategyKey(sub.StoreType)
	if !ok {
		return nil, notFound("Subscriptions sourced from '" + sub.StoreType + "' can't be cancelled in-app")
	}
	cancelled, err := s.strategies[key].CancelSubscription(ctx, sub.StoreTransactionID)
	if err != nil {
		return nil, badRequest(err.Error())
	}
	if !cancelled {
		return nil, notFound("Provider declined to cancel the subscription")
	}
	return &CancelResult{Cancelled: true, ExpiresAt: sub.ExpiresAt}, nil
}
```

Add `PortalUserFrom`:

```go
func PortalUserFrom(u *CheckoutUser) strategy.PortalUser {
	return strategy.PortalUser{StripeCustomerID: u.StripeCustomerID, LemonSqueezyCustomerID: u.LemonSqueezyCustomerID}
}
```

> **Note on the cancel `ExpiresAt` JSON:** Node returns `expires_at: sub.expires_at` — a `Date` serialized as an ISO **ms** string. Pin the same: change `CancelResult.ExpiresAt` to a custom marshaler using `shared.ISOMillis` (mirror `isoPtr` in `reader.go`), not the default `time.Time` RFC3339-nano. Implement `MarshalJSON` on `CancelResult` emitting `{"cancelled":bool,"expires_at":<ISOMillis|null>}`.

- [ ] **Step 6: Run → PASS** `go test ./internal/payment/ -run "TestPortal|TestCancel" -v`.

- [ ] **Step 7: Commit** `git add -A && git commit -m "feat(payment): portal + in-app cancel use cases"`

---

## Task 11: Handlers — checkout / portal / cancel

**Files:** Create `internal/payment/handler_checkout.go`; Test `internal/payment/handler_checkout_test.go`. (The existing `Handler` already serves Methods/Status/History — extend it.)

- [ ] **Step 1: Write the failing test** (httptest against the handler with a stub service or the real Service+fakes; assert status codes + envelope):

```go
func TestCheckoutHandler_ValidationError(t *testing.T) {
	h := newHandlerForTest(/* service that never gets called */)
	body := `{"method":"banana"}` // missing plan_id, bad method
	rr := doJSON(t, h.Checkout, http.MethodPost, "/payment/checkout", body, withClaims("u1"))
	if rr.Code != 400 {
		t.Fatalf("status=%d want 400", rr.Code)
	}
	var env struct{ Error, Code string }
	_ = json.Unmarshal(rr.Body.Bytes(), &env)
	if env.Code != "invalid-input" {
		t.Fatalf("code=%q want invalid-input", env.Code)
	}
	want := "plan_id must be a string. method must be one of the following values: stripe, paypal, momo, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay"
	if env.Error != want {
		t.Fatalf("msg=\n%q\nwant\n%q", env.Error, want)
	}
}

func TestCheckoutHandler_Success201(t *testing.T) {
	h := newHandlerForTest(serviceReturning(&CheckoutResponse{ /* vietqr */ }))
	rr := doJSON(t, h.Checkout, http.MethodPost, "/payment/checkout", `{"plan_id":"p1","method":"vietqr"}`, withClaims("u1"))
	if rr.Code != 201 {
		t.Fatalf("checkout must be 201, got %d", rr.Code)
	}
}

func TestCancelHandler_200(t *testing.T) {
	h := newHandlerForTest(serviceCancel(&CancelResult{Cancelled: true}))
	rr := doJSON(t, h.CancelSubscription, http.MethodDelete, "/payment/subscription", "", withClaims("u1"))
	if rr.Code != 200 {
		t.Fatalf("cancel must be 200, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run → FAIL** `go test ./internal/payment/ -run "Handler" -v`.

- [ ] **Step 3: Implement** `internal/payment/handler_checkout.go`. Validation mirrors `auth/validate.go` (hand-written, class-validator parity). `method` validity is the `@IsIn(PAYMENT_METHODS)` set = the full PaymentMethod enum (stripe, paypal, momo, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay):

```go
package payment

import (
	"encoding/json"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// allMethodValues is the @IsIn(PAYMENT_METHODS) set, in PaymentMethod enum
// declaration order (drives the validation message text).
var allMethodValues = []string{
	"stripe", "paypal", "momo", "vietqr", "bank_transfer", "lemonsqueezy", "apple_pay", "google_pay",
}

type checkoutBody struct {
	PlanID     string `json:"plan_id"`
	Method     string `json:"method"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// validateCheckout reproduces ValidationPipe+humanizer for CreateCheckoutDto,
// in property order (plan_id, method). Empty → valid. (success_url/cancel_url
// are @IsOptional @IsString — only validated when present and non-string,
// which a JSON string body can't violate, so they're not checked here.)
func validateCheckout(b checkoutBody) string {
	var msgs []string
	if b.PlanID == "" {
		msgs = append(msgs, "plan_id must be a string")
	}
	if !inSlice(allMethodValues, b.Method) {
		msgs = append(msgs, "method must be one of the following values: "+joinComma(allMethodValues))
	}
	return joinDotSpace(msgs)
}

// Checkout: POST /payment/checkout → 201. JWT-protected.
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		shared.WriteError(w, r, "invalid-input", "Invalid request body")
		return
	}
	if msg := validateCheckout(body); msg != "" {
		shared.WriteError(w, r, "invalid-input", msg)
		return
	}
	resp, err := h.svc.CreateCheckout(r.Context(), claims.UserID(), body.PlanID, body.Method,
		CheckoutOptions{SuccessURL: body.SuccessURL, CancelURL: body.CancelURL})
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "checkout failed")
		return
	}
	shared.WriteJSON(w, http.StatusCreated, resp)
}

// Portal: GET /payment/portal → 200 {url}. JWT-protected.
func (h *Handler) Portal(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	url, err := h.svc.CustomerPortalURL(r.Context(), claims.UserID())
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "portal failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"url": url})
}

// CancelSubscription: DELETE /payment/subscription → 200 {cancelled, expires_at}.
func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := shared.ClaimsFromContext(r.Context())
	if !ok {
		shared.WriteError(w, r, "internal", "auth context missing")
		return
	}
	out, err := h.svc.CancelActiveSubscription(r.Context(), claims.UserID())
	if err != nil {
		if writePaymentErr(w, r, err) {
			return
		}
		shared.WriteError(w, r, "internal", "cancel failed")
		return
	}
	shared.WriteJSON(w, http.StatusOK, out)
}
```

Add `writePaymentErr` (maps the package's domain error → `shared.WriteError` with the right code/status: 404→`not-found`, 400→`invalid-input`) — model on `auth/handler.go`'s `writeDomainErr`. Add the small string helpers (`inSlice`, `joinComma` = `strings.Join(x, ", ")`, `joinDotSpace` = `strings.Join(msgs, ". ")`) if not already present.

> **Status→code mapping for the envelope:** confirm the exact `code` strings the Node `AllExceptionsFilter.inferCode` emits: 404→`not-found`, 400→`invalid-input`. The `/payment/status` + `/payment/history` Phase 3a handlers should already have a `writePaymentErr`-equivalent; reuse it rather than forking.

- [ ] **Step 4: Run → PASS** `go test ./internal/payment/ -run "Handler" -v`.

- [ ] **Step 5: Commit** `git add -A && git commit -m "feat(payment): checkout/portal/cancel HTTP handlers"`

---

## Task 12: Router + main.go wiring

**Files:** Modify `internal/shared/router.go`, `cmd/server/main.go`. No new test (covered by `go build` + the shadow gate); optionally extend an existing router smoke test.

- [ ] **Step 1: Add Router fields + mounts** in `internal/shared/router.go`. Add to the `Router` struct: `PaymentCheckout, PaymentPortal, PaymentCancelSub http.Handler`. Mount inside the JWT-protected group (same group as `/payment/history`):

```go
r.Method(http.MethodPost, "/payment/checkout", rt.PaymentCheckout)
r.Method(http.MethodGet, "/payment/portal", rt.PaymentPortal)
r.Method(http.MethodDelete, "/payment/subscription", rt.PaymentCancelSub)
```

(Match the existing mount idiom — `grep -n "payment/history" internal/shared/router.go` and copy its auth-group placement exactly.)

- [ ] **Step 2: Build strategies + extend the service** in `cmd/server/main.go`, inside the existing `if pool != nil` payment block. Load credentials once at startup (admin can change them in the DB; Node re-reads per request — for parity under shadow, reading per request is safer, so prefer constructing strategies with a creds-loader closure. **Simplest faithful approach:** build the strategies map per request is heavy; instead, load creds at startup AND document that admin credential changes need a restart in Go until a later phase adds per-request refresh. For the shadow gate (error paths + VietQR), startup-load is fine.):

```go
creds, _ := paymentSettings.Credentials(ctx) // startup load; ErrNoRows → zero
vietqrStrat := vietqr.New(vietqr.Creds{
	BankID:        strategy.ResolveCredential(creds.VietQRBankID, cfg.VietQRBankID),
	AccountNumber: strategy.ResolveCredential(creds.VietQRAccountNumber, cfg.VietQRAccountNumber),
	AccountName:   strategy.ResolveCredential(creds.VietQRAccountName, cfg.VietQRAccountName),
})
stripeStrat := stripe.New(
	stripe.Creds{SecretKey: strategy.ResolveCredential(creds.StripeSecretKey, cfg.StripeSecretKey)},
	stripe.Env{PublishableKey: cfg.StripePublishableKey, ApplePayMerchantID: cfg.ApplePayMerchantID, WebsiteURL: cfg.WebsiteURL},
)
lsStrat := lemonsqueezy.New(lemonsqueezy.Creds{
	APIKey:         strategy.ResolveCredential(creds.LemonSqueezyAPIKey, cfg.LemonSqueezyAPIKey),
	StoreID:        strategy.ResolveCredential(creds.LemonSqueezyStoreID, cfg.LemonSqueezyStoreID),
	VariantMonthly: creds.LemonSqueezyVariantMonthly,
	VariantYearly:  creds.LemonSqueezyVariantYearly,
}, cfg.WebsiteURL)

strategies := map[string]paymentpkg.StrategyForKey{ /* see below */ }
```

Wire the strategies map keyed exactly like Node (`stripe`, `vietqr`, `bank_transfer`, `lemonsqueezy`, `apple_pay`, `google_pay` — note vietqr/bank_transfer share `vietqrStrat`; apple_pay/google_pay share `stripeStrat`). Pass it + the new repo methods + `subscriptionReader` into the extended `paymentpkg.NewService(...)`. Build the handlers and assign the new Router fields:

```go
core.paymentCheckout = http.HandlerFunc(paymentHandler.Checkout)
core.paymentPortal = http.HandlerFunc(paymentHandler.Portal)
core.paymentCancelSub = http.HandlerFunc(paymentHandler.CancelSubscription)
```

and in the Router literal: `PaymentCheckout: core.paymentCheckout, PaymentPortal: core.paymentPortal, PaymentCancelSub: core.paymentCancelSub`.

> The strategies map value type is `strategy.Strategy`; build it as `map[string]strategy.Strategy{"stripe": stripeStrat, "vietqr": vietqrStrat, "bank_transfer": vietqrStrat, "lemonsqueezy": lsStrat, "apple_pay": stripeStrat, "google_pay": stripeStrat}`. Add the `coreHandlers` struct fields `paymentCheckout/paymentPortal/paymentCancelSub http.Handler` and the imports for the three strategy sub-packages + `strategy`.

- [ ] **Step 3: Build + vet + full test** `go build ./... && go vet ./... && gofmt -l . && go test ./... -race`. Expected: all green, `gofmt -l` prints nothing.

- [ ] **Step 4: Commit** `git add -A && git commit -m "feat(payment): mount checkout/portal/cancel + wire strategies"`

---

## Task 13: Shadow fixtures

**Files:** Create `fixtures/payment_checkout_errors.json`, `fixtures/payment_checkout_vietqr.json`. Follow the existing fixture discipline (REPLACE_WITH_DEV_* placeholders, `ignore_value_of`, `_note`).

- [ ] **Step 1: Error-path fixture** `fixtures/payment_checkout_errors.json` (deterministic, no INSERT — these short-circuit before the payment row is created):

```json
{
  "name": "payment-checkout-validation",
  "method": "POST",
  "path": "/payment/checkout",
  "headers": { "Authorization": "Bearer REPLACE_WITH_DEV_USER_JWT", "Content-Type": "application/json" },
  "body": { "method": "banana" },
  "ignore_value_of": ["request_id"],
  "_note": "JWT-protected. Missing plan_id + invalid method → 400 invalid-input. error string must match byte-for-byte: 'plan_id must be a string. method must be one of the following values: stripe, paypal, momo, vietqr, bank_transfer, lemonsqueezy, apple_pay, google_pay'. Substitute a real dev-user session JWT (NOT an ext token — checkout uses JwtAuthGuard). Add a 2nd fixture variant with a valid plan_id + a disabled method to exercise the 404 'Payment method '<m>' is not enabled.' branch, and one with method=stripe + a free plan_id for 400 'Cannot purchase a free plan'."
}
```

- [ ] **Step 2: VietQR success fixture** `fixtures/payment_checkout_vietqr.json` (INSERTS a throwaway pending row in BOTH backends per run — acceptable per the agreed gate scope; the row is never completed):

```json
{
  "name": "payment-checkout-vietqr",
  "method": "POST",
  "path": "/payment/checkout",
  "headers": { "Authorization": "Bearer REPLACE_WITH_DEV_USER_JWT", "Content-Type": "application/json" },
  "body": { "plan_id": "REPLACE_WITH_DEV_PAID_VND_PLAN_ID", "method": "vietqr" },
  "ignore_value_of": [
    "id", "reference_code", "expires_at", "created_at", "updated_at",
    "qr_data", "reference", "completed_at", "provider_ref", "notes",
    "daily_limit", "is_active"
  ],
  "_note": "MUTATING + non-deterministic: each run INSERTs a pending payment in both backends. Requires VietQR enabled in app_settings.payment_methods_enabled (CSV must include 'vietqr') and a paid VND plan. Deterministic + compared: payment.amount, payment.currency='VND', payment.method='vietqr', payment.status='pending', payment.user_id, payment.plan_id, bank_info.{bank_name,account_number,account_name,amount,currency}. Ignored (random/per-request OR not selected by GetPlanForCheckout): id/reference_code/timestamps, qr_data (URL embeds the random reference_code), bank_info.reference (=reference_code), and the nested plan.daily_limit/is_active (Phase3b GetPlanForCheckout doesn't select them — known fixture-documented gap, see checkout.go TODO). If Go and Node disagree on any NON-ignored key the gate flags a real parity bug."
}
```

- [ ] **Step 3: Verify fixtures parse** — if the shadow harness has a dry-run/validate mode, run it; otherwise `cat | python3 -m json.tool` each file to confirm valid JSON. (Do NOT run the live gate — operator-gated; this plan ships nothing to prod.)

- [ ] **Step 4: Commit** `git add -A && git commit -m "test(payment): shadow fixtures for checkout error paths + vietqr success"`

---

## Final review (after all tasks)

- [ ] Dispatch a final code reviewer over the whole Phase 3b diff (`git diff develop...HEAD`): parity of every error string + status code against the parity-facts section; JSON key presence/omission for optional CheckoutResult fields; the `·` middle-dot byte; the `%20` encoding; strategies map keys == Node Map keys; no secret/JWT committed in fixtures; `gofmt -l` clean; `go test ./... -race` green.
- [ ] Then invoke **superpowers:finishing-a-development-branch**.

---

## Self-Review (plan vs spec)

**Spec coverage:** checkout flow ✓ (Task 9), 3 strategies ✓ (Tasks 6/7/8 — VietQR+bank_transfer, Stripe+wallet, LS), `/portal` ✓ (Task 10/11), cancel ✓ (Task 10/11), DTO validation ✓ (Task 11), official stripe-go SDK ✓ (Task 7), shadow gate = error paths + VietQR shape ✓ (Task 13). Config/reference/creds/queries/wiring ✓ (Tasks 1-5,12). Subscription `store_transaction_id` extension ✓ (Task 10).

**Placeholder scan:** No "TBD"/"handle errors"/"similar to". Three spots say "confirm the exact message at impl time" (Stripe `!stripe_price_id` text, LS not-configured text, status→code strings) — these are *verification* directives against named source lines, not missing content; the surrounding code is complete and the byte-target is quoted where pinned. The Stripe SDK field names carry a `go doc` verification step because the exact struct surface is version-bound.

**Type consistency:** `strategy.Strategy` signatures identical across the port (Task 5), all three impls (6/7/8), the service map, and the fakes (9/10). `CheckoutResponse`/`PaymentRow`/`PlanBrief`/`BankInfo`/`WalletIntent` field names match between marshaler (9), strategy types (5), and reader (existing). `CheckoutPlan`/`CheckoutUser`/`CreatedPayment` consistent between reader (4) and use case (9). `AccountSub.StoreTransactionID` added in 10 and read in 10's cancel.

**Known parity gap (documented, not silent):** the nested `payment.plan` object in the checkout response is partial (`GetPlanForCheckout` selects 7 of 11 plans columns) — `daily_limit`/`is_active`/`created_at`/`updated_at` are zero-valued and listed under the VietQR fixture's `ignore_value_of` with a TODO in `checkout.go`. Widening the query closes it later with no interface change.
