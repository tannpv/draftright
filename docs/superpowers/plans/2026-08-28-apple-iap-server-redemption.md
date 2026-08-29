# Apple IAP — Server-Side Redemption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Accept and verify a signed Apple StoreKit transaction on the Go backend, grant Pro through the existing chokepoint, and keep entitlement correct via App Store Server Notifications V2.

**Architecture:** Apple IAP is a `PaymentMethod` implementing the existing `Strategy` interface (one interface for every provider). It is **not** a checkout method — the purchase happens on-device — so it is absent from `registeredMethods` and its `CreateCheckout` is never routed. First purchase enters via a new `POST /payment/apple/redeem` handler + use-case; the subscription lifecycle enters via `VerifyWebhook` on `POST /payment/webhook/apple`. Both funnel into the existing `subsWriter` grant/extend writers. No new interface capability.

**Tech Stack:** Go 1.25 (chi router, pgx + sqlc), standard-library crypto (`crypto/ecdsa`, `crypto/x509`, `encoding/base64`, `encoding/json`) for StoreKit JWS verification. Flutter/Dart for the one client SSOT enum change.

## Global Constraints

- **One grant chokepoint.** Pro is granted only through `s.subsWriter.Grant(...)` / `ExtendByStoreRef(...)`; never add a second grant path (`webhook.go:243` `activateSubscription` is the reference).
- **One store-type mapping.** `apple_iap` is produced only by `StoreTypeForMethod(MethodAppleIAP)`; do not scatter the string.
- **One product→plan map.** App Store product ids resolve to plan ids in exactly one place (the resolver); a mismatch grants the wrong plan silently.
- **Never branch on the wire string in the app.** Add `apple_iap` to the Dart `PaymentMethodKind` enum + `wireName`; consumers use the enum.
- **`apple_pay` ≠ `apple_iap`.** `apple_pay` is the existing Stripe-wallet method (→ `StoreStripe`). `apple_iap` is the App Store method (→ `StoreAppleIAP`). Never conflate.
- **Cross-package method const duplication** (`payment.MethodAppleIAP` vs `strategy.MethodAppleIAP`) exists to avoid an import cycle and is guarded by `strategy_method_parity_test.go`. Any new method const goes in both, with a parity case.
- **Renewals must not re-send the activation email.** Only the first grant emails.
- **Module path:** `github.com/tannpv/draftright-rewrite`. Run all Go commands from `backend-rewrite-go/`.
- **Gate (do before Task 1 ships to prod, not a code step):** confirm the App Store rejection is Guideline 3.1.1, per the spec.
- **Bind-to-code lookups (confirm against the cited files, don't invent):** the auth-context user-id accessor used by existing handlers (Task 9 — read a neighbouring authed handler); the exact `StampStoreRef`/`ExtendByStoreRef`/`ExpireByStoreRef` signatures (`subscription/webhook_writer.go:70/80`); the credentials struct + how LemonSqueezy variants are read, to add `apple_product_monthly/yearly` the same way (`cmd/server/main.go:660-747`, `usecase.go:73-75`). Test fakes (`fakeSubsWriter`, `fakeEmailer`, `newTestService*`) mirror the existing payment-package test helpers — reuse them, don't rebuild.

---

### Task 1: Method identity + parity guard

**Files:**
- Modify: `backend-rewrite-go/internal/payment/domain.go` (const block ~line 17-26)
- Modify: `backend-rewrite-go/internal/payment/strategy/strategy.go` (method consts ~line 104-108)
- Modify: `backend-rewrite-go/internal/payment/strategy_method_parity_test.go`

**Interfaces:**
- Produces: `payment.MethodAppleIAP PaymentMethod = "apple_iap"`, `strategy.MethodAppleIAP = "apple_iap"`.

- [ ] **Step 1: Add the failing parity case**

In `strategy_method_parity_test.go`, add to the `cases` slice:
```go
		{strategy.MethodAppleIAP, MethodAppleIAP},
```

- [ ] **Step 2: Run — verify it fails to compile**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestStrategyMethodConstsMatchPayment`
Expected: FAIL — `undefined: strategy.MethodAppleIAP` and `undefined: MethodAppleIAP`.

- [ ] **Step 3: Add both consts**

In `domain.go`, in the `PaymentMethod` const block, after `MethodGooglePay`:
```go
	MethodAppleIAP PaymentMethod = "apple_iap"
```
In `strategy/strategy.go`, in the method-const block (~104-108), after `MethodGooglePay`:
```go
	MethodAppleIAP = "apple_iap"
```

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestStrategyMethodConstsMatchPayment`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/domain.go backend-rewrite-go/internal/payment/strategy/strategy.go backend-rewrite-go/internal/payment/strategy_method_parity_test.go
git commit -m "feat(payment): add apple_iap method const + parity case"
```

---

### Task 2: Store-type mapping

**Files:**
- Modify: `backend-rewrite-go/internal/payment/domain.go` (`StoreTypeForMethod` ~line 72-91)
- Test: `backend-rewrite-go/internal/payment/domain_test.go`

**Interfaces:**
- Produces: `StoreTypeForMethod(MethodAppleIAP) == StoreAppleIAP`.

- [ ] **Step 1: Write the failing test**

Append to `domain_test.go`:
```go
func TestStoreTypeForMethod_AppleIAP(t *testing.T) {
	if got := StoreTypeForMethod(MethodAppleIAP); got != StoreAppleIAP {
		t.Fatalf("StoreTypeForMethod(apple_iap) = %q, want %q", got, StoreAppleIAP)
	}
	// apple_pay stays Stripe-backed — must not be conflated with apple_iap.
	if got := StoreTypeForMethod(MethodApplePay); got != StoreStripe {
		t.Fatalf("StoreTypeForMethod(apple_pay) = %q, want %q", got, StoreStripe)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestStoreTypeForMethod_AppleIAP`
Expected: FAIL — got `admin_granted`, want `apple_iap` (falls to `default`).

- [ ] **Step 3: Add the case**

In `StoreTypeForMethod`, before `case MethodApplePay, MethodGooglePay:`:
```go
	case MethodAppleIAP:
		return StoreAppleIAP
```

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestStoreTypeForMethod_AppleIAP`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/domain.go backend-rewrite-go/internal/payment/domain_test.go
git commit -m "feat(payment): map apple_iap method to apple_iap store type"
```

---

### Task 3: Shared webhook/action types for Apple

**Files:**
- Modify: `backend-rewrite-go/internal/payment/strategy/strategy.go` (`WebhookAction` ~116-128, action consts ~82-97)

**Interfaces:**
- Produces: `WebhookAction.AppleTransactionID`, `.AppleOriginalTransactionID`, `.AppleProductID`, `.ExpiresAtUnix int64`; consts `ActionAppleSubscribed`, `ActionAppleRenewed`, `ActionAppleExpired`, `ActionAppleRefunded`; `var ErrNotCheckoutMethod = errors.New(...)`.

- [ ] **Step 1: Add a compile-anchoring test**

Create `backend-rewrite-go/internal/payment/strategy/apple_types_test.go`:
```go
package strategy

import "testing"

func TestAppleWebhookActionFields(t *testing.T) {
	a := WebhookAction{
		Type:                       ActionAppleRenewed,
		AppleTransactionID:         "t1",
		AppleOriginalTransactionID: "o1",
		AppleProductID:             "p1",
		CurrentPeriodEnd:           123, // reused existing field, not a new one
	}
	if a.Type != ActionAppleRenewed || a.AppleOriginalTransactionID != "o1" || a.CurrentPeriodEnd != 123 {
		t.Fatal("apple webhook action fields not wired")
	}
	if ErrNotCheckoutMethod == nil {
		t.Fatal("ErrNotCheckoutMethod must be defined")
	}
}
```

- [ ] **Step 2: Run — verify it fails to compile**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/ -run TestAppleWebhookActionFields`
Expected: FAIL — undefined `ActionAppleRenewed`, `AppleOriginalTransactionID`, `ExpiresAtUnix`, `ErrNotCheckoutMethod`.

- [ ] **Step 3: Add the types**

In `strategy.go`, add to the `WebhookAction` struct (the expiry reuses the
existing `CurrentPeriodEnd int64` already on the struct — do not add a new one):
```go
	// Apple App Store (IAP). Expiry uses the existing CurrentPeriodEnd field.
	AppleTransactionID         string
	AppleOriginalTransactionID string
	AppleProductID             string
```
Add to the action-const block:
```go
	ActionAppleSubscribed = "apple_subscribed"
	ActionAppleRenewed    = "apple_renewed"
	ActionAppleExpired    = "apple_expired"
	ActionAppleRefunded   = "apple_refunded"
```
**Reuse `CurrentPeriodEnd`, do not add a new expiry field** (review m3): `WebhookAction.CurrentPeriodEnd int64` (strategy.go:123) already means "period end, unix seconds" and is used by Stripe/LS/PayPal. The Apple path uses it too — do **not** add `ExpiresAtUnix`.
At package scope (add `"errors"` to imports if absent):
```go
// ErrNotCheckoutMethod is returned by CreateCheckout for redemption-style
// providers (Apple IAP) that never create a server checkout. It is never hit in
// a correct flow — such methods are absent from registeredMethods.
var ErrNotCheckoutMethod = errors.New("payment: method does not support server checkout")
```

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/ -run TestAppleWebhookActionFields`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/strategy/strategy.go backend-rewrite-go/internal/payment/strategy/apple_types_test.go
git commit -m "feat(payment): add Apple fields/actions + ErrNotCheckoutMethod to strategy"
```

---

### Task 4: StoreKit JWS verifier

Verify a signed StoreKit JWS (transaction or notification): decode the `x5c` chain, verify it to the injected root, verify the ES256 signature over `header.payload`, then check `bundleId` and `environment`. The Apple root is **injected** (not hard-coded) so tests use a self-signed fixture chain and prod injects Apple's Root CA G3.

**Files:**
- Create: `backend-rewrite-go/internal/payment/strategy/applestore/verify.go`
- Test: `backend-rewrite-go/internal/payment/strategy/applestore/verify_test.go`

**Interfaces:**
- Produces:
  ```go
  type JWSPayload struct {
      BundleID           string `json:"bundleId"`
      Environment        string `json:"environment"` // "Production" | "Sandbox"
      ProductID          string `json:"productId"`
      TransactionID      string `json:"transactionId"`
      OriginalTransactionID string `json:"originalTransactionId"`
      ExpiresDate        int64  `json:"expiresDate"` // unix millis
  }
  type Verifier struct{ roots *x509.CertPool; bundleID string; wantEnv string }
  func NewVerifier(roots *x509.CertPool, bundleID, environment string) *Verifier
  func (v *Verifier) Verify(jws string) (JWSPayload, error)
  ```

- [ ] **Step 1: Write the failing test**

Create `verify_test.go`. It builds a self-signed ECDSA leaf, signs a payload as a JWS with the leaf cert in `x5c`, and asserts verify accepts it, then rejects a tampered signature and a wrong bundle id. (Helper `makeJWS` is in the test file.)
```go
package applestore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

func makeJWS(t *testing.T, key *ecdsa.PrivateKey, der []byte, payload JWSPayload) string {
	t.Helper()
	hdr := map[string]any{"alg": "ES256", "x5c": []string{base64.StdEncoding.EncodeToString(der)}}
	hb, _ := json.Marshal(hdr)
	pb, _ := json.Marshal(payload)
	seg := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
	sum := sha256Sum([]byte(seg)) // helper defined in verify.go
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return seg + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func selfSigned(t *testing.T) (*ecdsa.PrivateKey, []byte, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	crt, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(crt)
	return key, der, pool
}

func TestVerify_Valid(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox", ProductID: "p", TransactionID: "t", OriginalTransactionID: "o", ExpiresDate: 1000})
	got, err := v.Verify(jws)
	if err != nil {
		t.Fatalf("valid jws rejected: %v", err)
	}
	if got.ProductID != "p" || got.OriginalTransactionID != "o" {
		t.Fatalf("payload not decoded: %+v", got)
	}
}

func TestVerify_TamperedSig(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws[:len(jws)-2] + "xy"); err == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestVerify_WrongBundle(t *testing.T) {
	key, der, pool := selfSigned(t)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.evil.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws); err == nil {
		t.Fatal("wrong bundleId accepted")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/applestore/`
Expected: FAIL — package/`Verify`/`sha256Sum` undefined.

- [ ] **Step 3: Implement the verifier**

Create `verify.go`:
```go
package applestore

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type JWSPayload struct {
	BundleID              string `json:"bundleId"`
	Environment           string `json:"environment"`
	ProductID             string `json:"productId"`
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ExpiresDate           int64  `json:"expiresDate"`
}

type Verifier struct {
	roots    *x509.CertPool
	bundleID string
	wantEnv  string
}

func NewVerifier(roots *x509.CertPool, bundleID, environment string) *Verifier {
	return &Verifier{roots: roots, bundleID: bundleID, wantEnv: environment}
}

func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// verifySignature checks the x5c chain to the configured root + the ES256
// signature, and returns the RAW decoded payload bytes. It does NOT check any
// claims — the outer ASSN V2 envelope has no top-level bundleId/environment, so
// claim checks belong only to the inner transaction JWS (see Verify).
func (v *Verifier) verifySignature(jws string) ([]byte, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return nil, errors.New("jws: want 3 segments")
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jws header: %w", err)
	}
	var hdr struct {
		Alg string   `json:"alg"`
		X5c []string `json:"x5c"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, fmt.Errorf("jws header json: %w", err)
	}
	if hdr.Alg != "ES256" || len(hdr.X5c) == 0 {
		return nil, errors.New("jws: expect ES256 with x5c")
	}
	var chain []*x509.Certificate
	for _, b64 := range hdr.X5c {
		der, err := base64.StdEncoding.DecodeString(b64) // x5c is standard base64 (RFC 7515)
		if err != nil {
			return nil, fmt.Errorf("x5c decode: %w", err)
		}
		crt, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("x5c parse: %w", err)
		}
		chain = append(chain, crt)
	}
	leaf := chain[0]
	inter := x509.NewCertPool()
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	// KeyUsages: ExtKeyUsageAny — Apple's StoreKit leaf is NOT a TLS server-auth
	// cert (it carries Apple's OID 1.2.840.113635.100.6.11.1). Leaving KeyUsages
	// empty defaults to ServerAuth and REJECTS the real Apple chain (review M1).
	// Validity dates are still checked by x509.Verify. OCSP/revocation and the
	// Apple leaf OID are intentionally NOT checked — chain-to-Apple-root is the gate.
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: inter,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("x5c chain: %w", err)
	}
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("leaf key not ecdsa")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return nil, errors.New("jws signature format")
	}
	sum := sha256Sum([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, sum[:], r, s) {
		return nil, errors.New("jws signature invalid")
	}
	payBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jws payload: %w", err)
	}
	return payBytes, nil
}

// Verify checks signature AND the bundleId/environment claims — use for the
// inner transaction JWS. Returns the decoded transaction payload.
func (v *Verifier) Verify(jws string) (JWSPayload, error) {
	payBytes, err := v.verifySignature(jws)
	if err != nil {
		return JWSPayload{}, err
	}
	var p JWSPayload
	if err := json.Unmarshal(payBytes, &p); err != nil {
		return JWSPayload{}, fmt.Errorf("jws payload json: %w", err)
	}
	if p.BundleID != v.bundleID {
		return JWSPayload{}, fmt.Errorf("bundleId %q != %q", p.BundleID, v.bundleID)
	}
	if v.wantEnv != "" && p.Environment != v.wantEnv {
		return JWSPayload{}, fmt.Errorf("environment %q != %q", p.Environment, v.wantEnv)
	}
	return p, nil
}

// VerifyEnvelope checks the signature of an ASSN V2 outer JWS (no claim checks)
// and returns its raw payload for the notification envelope decode.
func (v *Verifier) VerifyEnvelope(jws string) ([]byte, error) {
	return v.verifySignature(jws)
}
```
Add a fourth test to `verify_test.go` for `ExtKeyUsageAny` being tolerated (a leaf with a non-serverAuth EKU still verifies) so M1 can't regress:
```go
func TestVerify_NonServerAuthLeaf(t *testing.T) {
	// A leaf with an unrelated EKU must still verify (Apple's leaf isn't serverAuth).
	key, der, pool := selfSignedWithEKU(t, x509.ExtKeyUsageCodeSigning)
	v := NewVerifier(pool, "com.draftright.app", "Sandbox")
	jws := makeJWS(t, key, der, JWSPayload{BundleID: "com.draftright.app", Environment: "Sandbox"})
	if _, err := v.Verify(jws); err != nil {
		t.Fatalf("non-serverAuth leaf rejected: %v", err)
	}
}
```
(`selfSignedWithEKU` mirrors `selfSigned` but sets `tmpl.ExtKeyUsage = []x509.ExtKeyUsage{eku}` and `IsCA:false` with a separate CA — or reuse the self-signed CA and set the EKU on it.)

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/applestore/`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/strategy/applestore/verify.go backend-rewrite-go/internal/payment/strategy/applestore/verify_test.go
git commit -m "feat(applestore): StoreKit JWS verifier (x5c chain + ES256 + claims)"
```

---

### Task 5: `applestore` Strategy implementation

**Files:**
- Create: `backend-rewrite-go/internal/payment/strategy/applestore/applestore.go`
- Test: `backend-rewrite-go/internal/payment/strategy/applestore/applestore_test.go`

**Interfaces:**
- Consumes: `Verifier` (Task 4), `strategy.Strategy`, `strategy.WebhookAction`, `strategy.ErrNotCheckoutMethod`, `strategy.ActionApple*` (Task 3).
- Produces: `func New(v *Verifier) *Strategy` implementing `strategy.Strategy`; exported `func (s *Strategy) VerifyNotification(signedPayload string) (strategy.WebhookAction, error)`.

- [ ] **Step 1: Write the failing test**

Create `applestore_test.go`:
```go
package applestore

import (
	"context"
	"net/http"
	"testing"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

func TestStrategy_ContractShape(t *testing.T) {
	var _ strategy.Strategy = New(nil) // compile-time interface assertion

	s := New(nil)
	if _, err := s.CreateCheckout(context.Background(), strategy.Payment{}, strategy.Plan{}, strategy.Options{}); err != strategy.ErrNotCheckoutMethod {
		t.Fatalf("CreateCheckout = %v, want ErrNotCheckoutMethod", err)
	}
	url, _ := s.CustomerPortalURL(context.Background(), strategy.PortalUser{})
	if url == "" {
		t.Fatal("CustomerPortalURL should return the manage-subscriptions deep link")
	}
	if ok, _ := s.CancelSubscription(context.Background(), "x"); ok {
		t.Fatal("CancelSubscription must return false (Apple forbids server cancel)")
	}
	if _, err := s.VerifyWebhook(context.Background(), []byte("{}"), http.Header{}); err == nil {
		t.Fatal("VerifyWebhook should reject a body with no signedPayload")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/applestore/ -run TestStrategy_ContractShape`
Expected: FAIL — `New`/`Strategy` undefined.

- [ ] **Step 3: Implement the strategy**

Create `applestore.go`:
```go
package applestore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/payment/strategy"
)

const manageSubscriptionsURL = "itms-apps://apps.apple.com/account/subscriptions"

// notificationType → strategy action.
var notifAction = map[string]string{
	"SUBSCRIBED":               strategy.ActionAppleSubscribed,
	"DID_RENEW":                strategy.ActionAppleRenewed,
	"EXPIRED":                  strategy.ActionAppleExpired,
	"GRACE_PERIOD_EXPIRED":     strategy.ActionAppleExpired,
	"REFUND":                   strategy.ActionAppleRefunded,
	"DID_CHANGE_RENEWAL_STATUS": strategy.ActionAppleRenewed, // status flips don't grant; mapped, handler decides
}

type Strategy struct{ v *Verifier }

var _ strategy.Strategy = (*Strategy)(nil)

func New(v *Verifier) *Strategy { return &Strategy{v: v} }

// CreateCheckout: IAP is redeemed, not checked out. Never routed (absent from
// registeredMethods); returns the honest sentinel if it ever is.
func (s *Strategy) CreateCheckout(ctx context.Context, p strategy.Payment, plan strategy.Plan, opts strategy.Options) (strategy.Result, error) {
	return strategy.Result{}, strategy.ErrNotCheckoutMethod
}

// CustomerPortalURL: Apple subscriptions are managed in Settings.
func (s *Strategy) CustomerPortalURL(ctx context.Context, u strategy.PortalUser) (string, error) {
	return manageSubscriptionsURL, nil
}

// CancelSubscription: Apple forbids server-side cancel.
func (s *Strategy) CancelSubscription(ctx context.Context, subscriptionID string) (bool, error) {
	return false, nil
}

// VerifyWebhook: App Store Server Notifications V2 — the body is
// {"signedPayload": "<JWS>"}; the JWS decodes to {notificationType, data:{signedTransactionInfo}}.
func (s *Strategy) VerifyWebhook(ctx context.Context, payload []byte, headers http.Header) (strategy.WebhookAction, error) {
	var body struct {
		SignedPayload string `json:"signedPayload"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.SignedPayload == "" {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "missing signedPayload"}
	}
	return s.VerifyNotification(body.SignedPayload)
}

// VerifyNotification verifies an ASSN V2 signed payload and its inner
// transaction, returning the mapped action.
//
// The OUTER envelope is signature-verified only (VerifyEnvelope): it carries
// notificationType + data.signedTransactionInfo and has NO top-level
// bundleId/environment, so running the claim-checking Verify on it would always
// fail (review C3). bundleId/environment are checked on the INNER transaction.
func (s *Strategy) VerifyNotification(signedPayload string) (strategy.WebhookAction, error) {
	if s.v == nil {
		return strategy.WebhookAction{}, errors.New("applestore: verifier not configured")
	}
	envBytes, err := s.v.VerifyEnvelope(signedPayload)
	if err != nil {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "notification signature: " + err.Error()}
	}
	var env struct {
		NotificationType string `json:"notificationType"`
		Data             struct {
			SignedTransactionInfo string `json:"signedTransactionInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(envBytes, &env); err != nil || env.Data.SignedTransactionInfo == "" {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 400, Message: "notification: no signedTransactionInfo"}
	}
	tx, err := s.v.Verify(env.Data.SignedTransactionInfo) // inner: full claim check
	if err != nil {
		return strategy.WebhookAction{}, &strategy.WebhookError{Status: 401, Message: "transaction signature: " + err.Error()}
	}
	act, ok := notifAction[env.NotificationType]
	if !ok {
		return strategy.Ignored(), nil
	}
	return strategy.WebhookAction{
		Type:                       act,
		AppleTransactionID:         tx.TransactionID,
		AppleOriginalTransactionID: tx.OriginalTransactionID,
		AppleProductID:             tx.ProductID,
		CurrentPeriodEnd:           tx.ExpiresDate / 1000, // ms → unix seconds
	}, nil
}
```

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/strategy/applestore/`
Expected: PASS (contract shape + verifier tests).

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/strategy/applestore/
git commit -m "feat(applestore): Strategy impl — honest checkout/portal/cancel + ASSN V2 VerifyWebhook"
```

---

### Task 6: Product → plan resolver (LemonSqueezy pattern)

Mirror `resolvePlanIDFromLSVariant` (`webhook.go:272-289`): Apple product ids come from credentials, resolve to a plan via billing period + `FindFirstActivePlanID`.

**Files:**
- Migration: add `apple_product_monthly` + `apple_product_yearly` columns to the settings table (a reversible up/down migration; the LS/PayPal credentials are typed `app_settings` columns, so this **is** a schema change — review m4)
- Modify: `backend-rewrite-go/internal/platform/db/queries_core.sql` (settings upsert/select, ~lines 40-48) + run `sqlc generate`
- Modify: the `Credentials` struct (`settings_pg.go:44-54`) + the resolver seam (follow `VariantResolver`, usecase.go:73-75) to expose `appleProducts(ctx) (monthly, yearly string, err error)`
- Modify: `backend-rewrite-go/internal/payment/webhook.go` (add `resolvePlanIDFromAppleProduct`)
- Test: `backend-rewrite-go/internal/payment/webhook_test.go`

**Interfaces:**
- Consumes: `WebhookRepo.FindFirstActivePlanID(ctx, billing, currency)` (usecase.go:60), the credentials resolver.
- Produces: `func (s *Service) resolvePlanIDFromAppleProduct(ctx context.Context, productID string) (planID string, billing string, err error)`.

- [ ] **Step 1: Write the failing test**

Add to `webhook_test.go` (using the package's existing fakes for `WebhookRepo` + credentials; follow the LS resolver test as the template):
```go
func TestResolvePlanIDFromAppleProduct(t *testing.T) {
	s := newTestServiceWithAppleProducts(t, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	plan, billing, err := s.resolvePlanIDFromAppleProduct(context.Background(), "com.draftright.pro.yearly")
	if err != nil {
		t.Fatal(err)
	}
	if billing != "yearly" || plan == "" {
		t.Fatalf("got plan=%q billing=%q", plan, billing)
	}
	if _, _, err := s.resolvePlanIDFromAppleProduct(context.Background(), "unknown.product"); err == nil {
		t.Fatal("unknown product id must error, not grant a plan")
	}
}
```
(`newTestServiceWithAppleProducts` is a small helper mirroring the existing LS test setup — wires a fake `WebhookRepo` whose `FindFirstActivePlanID` returns a stub id, and Apple product ids in the credentials.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestResolvePlanIDFromAppleProduct`
Expected: FAIL — `resolvePlanIDFromAppleProduct` undefined.

- [ ] **Step 3: Implement the resolver**

Add the Apple product ids to the credentials source (mirror how LS variants / PayPal plans are read, recon §7-8), then in `webhook.go`:
```go
// resolvePlanIDFromAppleProduct maps an App Store product id to a plan id via
// its billing period — the single source of truth for product→plan (RULE #1).
func (s *Service) resolvePlanIDFromAppleProduct(ctx context.Context, productID string) (string, string, error) {
	monthly, yearly, err := s.appleProducts(ctx) // credentials-backed, like LemonSqueezyVariants
	if err != nil {
		return "", "", err
	}
	var billing string
	switch productID {
	case monthly:
		billing = "monthly"
	case yearly:
		billing = "yearly"
	default:
		return "", "", fmt.Errorf("apple product %q maps to no plan", productID)
	}
	planID, err := s.webhookRepo.FindFirstActivePlanID(ctx, billing, "USD")
	if err != nil {
		return "", "", err
	}
	return planID, billing, nil
}
```
(Wire `appleProducts(ctx) (monthly, yearly string, err error)` on the resolver seam alongside `LemonSqueezyVariants`.)

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestResolvePlanIDFromAppleProduct`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/
git commit -m "feat(payment): resolve App Store product id -> plan id (LS pattern, one source)"
```

---

### Task 7: Redeem use-case (client transaction → grant)

**Files:**
- Create: `backend-rewrite-go/internal/payment/apple_redeem.go`
- Migration + `queries_auth.sql` + `sqlc generate`: new query `StampStoreRefByUser` — sets `store_transaction_id` on the user's newest active sub matched by `user_id` + `store_type` (NO `payments` join; the existing `StampStoreRefByReference` joins on `payments.reference_code`, which the redeem path never creates — review C1).
- Modify: `backend-rewrite-go/internal/subscription/webhook_writer.go` (+ the `SubsWriter` port in `usecase.go:43-50`): add `StampStoreRefByUser(ctx, userID, storeType, txnID string) error`.
- Test: `backend-rewrite-go/internal/payment/apple_redeem_test.go`

**Why not `StampStoreRef`:** it returns only `error` and matches by `reference_code` via a `payments` join (queries_auth.sql:215-223). The IAP redeem creates no payment row, so it would stamp nothing — leaving `store_transaction_id` NULL and making every later `ExtendByStoreRef`/`ExpireByStoreRef` match 0 rows (review C1/C2). Stamp by `user_id` instead.

**Interfaces:**
- Consumes: the `applestore` verifier seam (`s.appleVerify`); `resolvePlanIDFromAppleProduct` (Task 6); `s.subsWriter.Grant` (webhook_writer.go:43) + the new `StampStoreRefByUser`; `s.emailer.SubscriptionActivated`; `StoreAppleIAP`.
- Produces: `func (s *Service) RedeemAppleTransaction(ctx context.Context, userID, signedTransaction string) error`; `SubsWriter.StampStoreRefByUser(ctx, userID, storeType, txnID string) error`.

- [ ] **Step 1: Write the failing test**

Create `apple_redeem_test.go`. Uses fakes: a verifier stub returning a known `JWSPayload`, a fake `subsWriter` recording `Grant` + `StampStoreRef`, Apple product ids in creds.
Extend the existing `fakeSubsWriter` (webhook_test.go — real fields are `granted bool`, `grantStore`, `stamped`, `extended`, `expired`; add `stampByUserRef string` + implement the new port method to record it). Then:
```go
func TestRedeemAppleTransaction_GrantsOnceAndStamps(t *testing.T) {
	f := &fakeSubsWriter{}
	em := &fakeEmailer{}
	s := newTestServiceForRedeem(t, f, em, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	// verifier stub yields productId=monthly product, originalTransactionId=o1, future expiry
	if err := s.RedeemAppleTransaction(context.Background(), "user-1", "stub-jws-monthly"); err != nil {
		t.Fatal(err)
	}
	if !f.granted || f.grantStore != string(StoreAppleIAP) {
		t.Fatalf("grant store type = %q, want apple_iap (granted=%v)", f.grantStore, f.granted)
	}
	if f.stampByUserRef != "o1" {
		t.Fatalf("store ref = %q, want original transaction id o1", f.stampByUserRef)
	}
	if em.activatedCalls != 1 {
		t.Fatalf("first IAP purchase should send one activation email, got %d", em.activatedCalls)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestRedeemAppleTransaction`
Expected: FAIL — `RedeemAppleTransaction` undefined.

- [ ] **Step 3: Implement the use-case**

Create `apple_redeem.go`:
```go
package payment

import (
	"context"
	"fmt"
	"time"
)

// RedeemAppleTransaction verifies a client-supplied StoreKit transaction, grants
// Pro (cancelling any prior active sub, via Grant), stamps the ORIGINAL
// transaction id so later notifications match this row, and sends the activation
// email (parity with the other providers' first grant). No second grant path.
func (s *Service) RedeemAppleTransaction(ctx context.Context, userID, signedTransaction string) error {
	tx, err := s.appleVerify(signedTransaction) // seam over applestore.Verifier.Verify
	if err != nil {
		return fmt.Errorf("apple verify: %w", err)
	}
	planID, billing, err := s.resolvePlanIDFromAppleProduct(ctx, tx.ProductID)
	if err != nil {
		return err
	}
	exp := time.UnixMilli(tx.ExpiresDate).UTC()
	if tx.ExpiresDate == 0 { // fall back to billing period if the transaction omits it
		if billing == "yearly" {
			exp = s.now().AddDate(1, 0, 0)
		} else {
			exp = s.now().AddDate(0, 1, 0)
		}
	}
	if err := s.subsWriter.Grant(ctx, userID, planID, string(StoreAppleIAP), &exp); err != nil {
		return err
	}
	// Stamp the ORIGINAL transaction id ONTO the just-granted row (matched by
	// user_id + store_type — NOT by a payments reference, which doesn't exist for
	// IAP) so renewals/expiries/refunds match via Extend/ExpireByStoreRef.
	if err := s.subsWriter.StampStoreRefByUser(ctx, userID, string(StoreAppleIAP), tx.OriginalTransactionID); err != nil {
		return err
	}
	// First IAP purchase emails like every other provider (webhook.go:255-257).
	if email, name, err := s.webhookRepo.UserEmailName(ctx, userID); err == nil && email != "" {
		s.emailer.SubscriptionActivated(ctx, email, name, "Pro")
	}
	return nil
}
```
(Add the `appleVerify func(signedTransaction string) (applestore.JWSPayload, error)` seam + `resolvePlanIDFromAppleProduct` (Task 6) to `Service`; inject the real verifier in `main.go`, Task 9. `StampStoreRefByUser` is the new writer above — a plain `UPDATE ... WHERE user_id=$1 AND store_type=$2 AND is_active` on the newest active sub, returning `error`.)

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestRedeemAppleTransaction`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/apple_redeem.go backend-rewrite-go/internal/payment/apple_redeem_test.go
git commit -m "feat(payment): RedeemAppleTransaction — verify, grant via chokepoint, stamp store ref"
```

---

### Task 8: Notification lifecycle in `HandleWebhook`

Add the Apple action cases to `HandleWebhook`'s switch (`webhook.go:39`), mirroring the PayPal block (webhook.go:141-179): renewals `ExtendByStoreRef` (no email), expiry/refund revoke.

**Files:**
- Modify: `backend-rewrite-go/internal/payment/webhook.go`
- Test: `backend-rewrite-go/internal/payment/webhook_test.go`

**Interfaces:**
- Consumes: `strategy.ActionApple*`, `WebhookAction.Apple*`/`ExpiresAtUnix`; `subsWriter.ExtendByStoreRef` (webhook_writer.go:80), `ExpireByStoreRef`.

- [ ] **Step 1: Write the failing tests**

Add to `webhook_test.go`. `appleAction(actionType, originalTxnID string, periodEnd int64)` wires a fake `strategy.Strategy` whose `VerifyWebhook` returns a `WebhookAction{Type, AppleOriginalTransactionID, CurrentPeriodEnd}`; register it in the test service's `strategies` map under `string(MethodAppleIAP)`. Assert against the real `fakeSubsWriter` fields (`extended`, `expired`) + `fakeEmailer` (`activatedCalls`) — extend the fake to make `ExtendByStoreRef` return a settable rows-affected so the `n==0` branch is testable:
```go
func TestHandleWebhook_AppleRenew_ExtendsNoEmail(t *testing.T) {
	f := &fakeSubsWriter{extendRows: 1} // one row matched
	em := &fakeEmailer{}
	s := newTestServiceForWebhook(t, f, em, appleAction(strategy.ActionAppleRenewed, "o1", 4102444800)) // 2100
	if err := s.HandleWebhook(context.Background(), string(MethodAppleIAP), []byte(`{"signedPayload":"x"}`), nil); err != nil {
		t.Fatal(err)
	}
	if !f.extended {
		t.Fatal("renewal must ExtendByStoreRef")
	}
	if em.activatedCalls != 0 {
		t.Fatal("renewal must not re-send activation email")
	}
}

func TestHandleWebhook_AppleExpired_Revokes(t *testing.T) {
	f := &fakeSubsWriter{}
	s := newTestServiceForWebhook(t, f, &fakeEmailer{}, appleAction(strategy.ActionAppleExpired, "o1", 0))
	if err := s.HandleWebhook(context.Background(), string(MethodAppleIAP), []byte(`{"signedPayload":"x"}`), nil); err != nil {
		t.Fatal(err)
	}
	if !f.expired {
		t.Fatal("expiry must ExpireByStoreRef")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestHandleWebhook_Apple`
Expected: FAIL — no Apple cases; `extendCalls`/`expireCalls` stay 0.

- [ ] **Step 3: Add the cases**

In `HandleWebhook`'s `switch action.Type` (mirror the PayPal renewal/cancel arms).
Expiry uses the reused `action.CurrentPeriodEnd`. Match the real
`ExtendByStoreRef`/`ExpireByStoreRef` signatures (webhook_writer.go:80 — returns
`(int64, error)`; and `ExpireByStoreRef`):
```go
	case strategy.ActionAppleSubscribed, strategy.ActionAppleRenewed:
		// Extend the existing sub matched by the ORIGINAL transaction id (stamped
		// on first redeem). Renewals must NOT re-send the activation email.
		exp := time.Unix(action.CurrentPeriodEnd, 0).UTC()
		n, err := s.subsWriter.ExtendByStoreRef(ctx, string(StoreAppleIAP), action.AppleOriginalTransactionID, exp)
		if err != nil {
			return err
		}
		if n == 0 {
			// No matching row → the first redeem never landed (lost POST). We
			// cannot grant from here in Spec 1: the notification carries no user
			// identity (user↔transaction linkage via StoreKit appAccountToken
			// arrives with the CLIENT spec). Recovery is client retry of /redeem.
			// Log for observability; do not error the webhook (Apple would retry).
			s.logUnmatchedAppleNotification(action.AppleOriginalTransactionID)
		}
	case strategy.ActionAppleExpired, strategy.ActionAppleRefunded:
		if err := s.subsWriter.ExpireByStoreRef(ctx, string(StoreAppleIAP), action.AppleOriginalTransactionID); err != nil {
			return err
		}
```
(`logUnmatchedAppleNotification` is a one-line structured log on `s`; if the
service has no logger seam yet, use the existing logging pattern in this file.)

- [ ] **Step 4: Run — verify pass**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestHandleWebhook_Apple`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/webhook.go backend-rewrite-go/internal/payment/webhook_test.go
git commit -m "feat(payment): Apple notification lifecycle — extend on renew (no email), revoke on expire/refund"
```

---

### Task 9: HTTP handlers, router, and main wiring

**Files:**
- Create: `backend-rewrite-go/internal/payment/handler_apple.go`
- Modify: `backend-rewrite-go/internal/payment/handler_webhook.go` (add `AppleWebhook` thin method)
- Modify: `backend-rewrite-go/internal/shared/router.go` (fields ~95-115, mounts ~298-315 + auth group ~431)
- Modify: `backend-rewrite-go/cmd/server/main.go` (strategy build ~660-747, handler wiring ~735-747)
- Test: `backend-rewrite-go/internal/payment/handler_apple_test.go`

**Interfaces:**
- Consumes: `Service.RedeemAppleTransaction` (Task 7), `Handler.webhook` (handler_webhook.go:14).
- Produces: `POST /payment/apple/redeem` (auth-gated), `POST /payment/webhook/apple` (public).

- [ ] **Step 1: Write the failing handler test**

`NewHandler` takes a concrete `*Service` (handler.go:17-20), so build a real
`Service` with fake `subsWriter` + the verifier seam (reuse `newTestServiceForRedeem`
from Task 7), and inject the auth claims via the real accessor. Create
`handler_apple_test.go`:
```go
func TestAppleRedeemHandler_GrantsForAuthedUser(t *testing.T) {
	f := &fakeSubsWriter{}
	svc := newTestServiceForRedeem(t, f, &fakeEmailer{}, "com.draftright.pro.monthly", "com.draftright.pro.yearly")
	h := NewHandler(svc /* + the other deps NewHandler needs; see an existing handler test */)

	req := httptest.NewRequest(http.MethodPost, "/payment/apple/redeem", strings.NewReader(`{"signedTransaction":"stub-jws-monthly"}`))
	// inject claims the way the auth middleware does (shared.ClaimsFromContext reads this key):
	req = req.WithContext(shared.WithClaims(req.Context(), &shared.Claims{Sub: "user-9"}))
	rec := httptest.NewRecorder()
	h.AppleRedeem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if !f.granted || f.grantStore != string(StoreAppleIAP) {
		t.Fatalf("handler did not grant apple_iap for the authed user: %+v", f)
	}
}
```
(Confirm the exact claims-injection helper name against `shared` — `ClaimsFromContext` is at handler.go:70-75; use its matching setter. If none is exported, set the context key the middleware uses.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd backend-rewrite-go && go test ./internal/payment/ -run TestAppleRedeemHandler`
Expected: FAIL — `AppleRedeem` undefined.

- [ ] **Step 3: Implement handler + wiring**

**M2 — the webhook must bypass the storefront `EnabledMethods` gate.** `HandleWebhook` 404s any method not in `EnabledMethods(ctx)` (webhook.go:40-46), and `apple_iap` is deliberately absent from `registeredMethods`/the enabled CSV. So the Apple webhook needs a path that gates on the **strategy registry only**. Add a `Service` method:
```go
// HandleProviderNotification runs a provider webhook WITHOUT the storefront
// enabled-methods gate — for webhook-only methods (Apple IAP) that are never
// offered for checkout but must still process notifications. It resolves the
// strategy directly; everything after (VerifyWebhook + action switch) is shared
// with HandleWebhook.
func (s *Service) HandleProviderNotification(ctx context.Context, method string, payload []byte, headers http.Header) error {
	strat, ok := s.strategies[method]
	if !ok {
		return &strategy.WebhookError{Status: 404, Message: method + " not configured"}
	}
	action, err := strat.VerifyWebhook(ctx, payload, headers)
	if err != nil {
		return err
	}
	return s.applyWebhookAction(ctx, method, action) // extract the existing switch into applyWebhookAction
}
```
Refactor `HandleWebhook`'s action switch into `applyWebhookAction(ctx, method, action)` so both entry points share it (no duplicated grant logic — RULE #1). `HandleWebhook` keeps the enabled-gate + calls `applyWebhookAction`.

`handler_apple.go`:
```go
package payment

import (
	"encoding/json"
	"net/http"

	"github.com/tannpv/draftright-rewrite/internal/shared"
)

// AppleRedeem accepts a client-verified StoreKit transaction and grants Pro.
func (h *Handler) AppleRedeem(w http.ResponseWriter, r *http.Request) {
	claims := shared.ClaimsFromContext(r.Context()) // the app's auth-context accessor (handler.go:70-75)
	if claims == nil || claims.Sub == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		SignedTransaction string `json:"signedTransaction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SignedTransaction == "" {
		http.Error(w, "signedTransaction required", http.StatusBadRequest)
		return
	}
	if err := h.svc.RedeemAppleTransaction(r.Context(), claims.Sub, body.SignedTransaction); err != nil {
		http.Error(w, "redeem failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// AppleWebhook processes App Store Server Notifications V2 — ungated (webhook-only method).
func (h *Handler) AppleWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if err := h.svc.HandleProviderNotification(r.Context(), string(MethodAppleIAP), body, r.Header); err != nil {
		var we *strategy.WebhookError
		if errors.As(err, &we) {
			http.Error(w, we.Message, we.Status)
			return
		}
		http.Error(w, "webhook error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
```
In `router.go`: add fields `PaymentWebhookApple http.Handler` + `PaymentAppleRedeem http.Handler`; mount webhook in the PUBLIC group, redeem in the AUTH group:
```go
	if r.PaymentWebhookApple != nil {
		mux.Method(http.MethodPost, "/payment/webhook/apple", r.PaymentWebhookApple)
	}
	// inside the authenticated group, beside /payment/checkout:
	if r.PaymentAppleRedeem != nil {
		authed.Method(http.MethodPost, "/payment/apple/redeem", r.PaymentAppleRedeem)
	}
```
In `main.go`: add config fields `cfg.AppleBundleID`, `cfg.AppleEnvironment` (config struct + env, review m5); `loadAppleRootCAs()` embeds Apple Root CA G3 PEM (`//go:embed` a `.pem`, `x509.NewCertPool().AppendCertsFromPEM`) and returns `*x509.CertPool`:
```go
	appleRoots := loadAppleRootCAs()
	appleStrat := applestore.New(applestore.NewVerifier(appleRoots, cfg.AppleBundleID, cfg.AppleEnvironment))
	strategies[string(paymentpkg.MethodAppleIAP)] = appleStrat
	// inject the redeem verifier seam into the payment Service (a NewService option)
	core.paymentWebhookApple = http.HandlerFunc(paymentHandler.AppleWebhook)
	core.paymentAppleRedeem = http.HandlerFunc(paymentHandler.AppleRedeem)
```
Do **not** add `apple_iap` to `registeredMethods` (it is not a checkout method).

- [ ] **Step 4: Run — verify pass + full build**

Run: `cd backend-rewrite-go && go test ./internal/payment/... && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 5: Commit**

```bash
git add backend-rewrite-go/internal/payment/handler_apple.go backend-rewrite-go/internal/payment/handler_webhook.go backend-rewrite-go/internal/shared/router.go backend-rewrite-go/cmd/server/main.go backend-rewrite-go/internal/payment/handler_apple_test.go
git commit -m "feat(payment): wire Apple redeem + webhook routes and strategy registration"
```

---

### Task 10: Flutter wire-identity SSOT

**Files:**
- Modify: `DraftRightMobile/lib/services/payment/payment_method.dart`
- Test: `DraftRightMobile/test/services/payment_method_test.dart` (create if absent)

**Interfaces:**
- Produces: `PaymentMethodKind.appleIap` with `wireName == 'apple_iap'`.

- [ ] **Step 1: Write the failing test**

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';

void main() {
  test('appleIap round-trips its wire name', () {
    expect(PaymentMethodKind.appleIap.wireName, 'apple_iap');
    expect(PaymentMethodKind.fromWire('apple_iap'), PaymentMethodKind.appleIap);
    // apple_pay stays distinct.
    expect(PaymentMethodKind.applePay.wireName, 'apple_pay');
  });
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/payment_method_test.dart`
Expected: FAIL — `appleIap` not defined.

- [ ] **Step 3: Add the enum value + wire mapping**

Add `appleIap` to the `PaymentMethodKind` enum; in `wireName`:
```dart
      case PaymentMethodKind.appleIap:
        return 'apple_iap';
```
Add a `PaymentMethodDescriptor.forKind` case for it (mirror `applePay`'s descriptor, distinct label "App Store").

- [ ] **Step 4: Run — verify pass**

Run: `cd DraftRightMobile && flutter test test/services/payment_method_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/payment/payment_method.dart DraftRightMobile/test/services/payment_method_test.dart
git commit -m "feat(mobile): add apple_iap to PaymentMethodKind + wireName (SSOT)"
```

---

## Final verification (after all tasks)

- [ ] `cd backend-rewrite-go && go vet ./... && go test ./...` — all green.
- [ ] `cd DraftRightMobile && flutter analyze --no-fatal-infos && flutter test` — green.
- [ ] Re-confirm the RULE #1 machines pass: `strategy_method_parity_test.go` (method parity), product→plan resolver guard, `StoreTypeForMethod` test.
- [ ] Grep proves no second grant path: `grep -rn "subsWriter.Grant" internal/payment` shows only `activateSubscription` + `RedeemAppleTransaction`.

## Not in this plan (later specs / manual)

- App Store Connect subscription group + products + pricing (manual, owner).
- Client StoreKit 2 purchase + restore UI (Flutter + native) — its own spec.
- Removing the Apple Pay entitlement from the iOS subscription path — deliberate, after this is live.
- The two pre-implementation checks from the spec (`activateSubscription` email-on-renewal semantics; `expires_at` extension) — verify against the code before Task 7/8; the plan routes renewals through `ExtendByStoreRef` (not `activateSubscription`) so no renewal email fires.
- **Notification-driven first-grant recovery (review M3):** if the client purchase succeeds but the `/redeem` POST is lost, Spec 1 relies on **client retry** of `/redeem`. A `SUBSCRIBED`/`DID_RENEW` notification for an unknown subscription cannot grant, because the notification carries no user identity — the user↔transaction link is a StoreKit `appAccountToken` the **client** sets at purchase, which lands with the client spec. Until then, the unmatched notification is logged (`logUnmatchedAppleNotification`), not errored. The client spec will add `appAccountToken` to `JWSPayload` + a notification-recovery grant.

### Implementation notes carried from review

- `handler_apple.go`'s `AppleWebhook` needs `io`, `errors`, and the `strategy` + `shared` imports.
- Task 9 extracts `HandleWebhook`'s action switch (built in Task 8) into `applyWebhookAction(ctx, method, action)` so the ungated Apple path (`HandleProviderNotification`) and the gated path share ONE grant/extend switch — no duplicated lifecycle logic (RULE #1).
- All new writer methods (`StampStoreRefByUser`) go on the `SubsWriter` port (`usecase.go:43-50`) AND its `*subscription.WebhookWriter` impl AND every fake used in tests, or the package won't compile.
