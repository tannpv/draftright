# Design — Apple IAP: server-side redemption (Spec 1 of the StoreKit epic)

**Date:** 2026-08-28
**Status:** design approved; ready for an implementation plan.
**Companion:** `docs/HANDOFF-2026-08-28-storekit-iap.md` (the briefing this refines).

This is **Spec 1** of the StoreKit / In-App-Purchase epic. It covers only the
**server-side** work: accept a signed Apple StoreKit transaction, verify it,
grant Pro through the existing chokepoint, and stay correct afterwards via App
Store Server Notifications V2. It is pure Go — buildable and testable with no
App Store Connect dependency.

## Decomposition (why this is Spec 1, not the whole thing)

| Sub-project | This spec? | Depends on |
|---|---|---|
| **A. Server-side redemption + notifications** (Go) | ✅ Spec 1 | nothing |
| B. Client purchase flow (Flutter + StoreKit 2) | later spec | A + D |
| C. App Store Connect setup (subscription group, products, pricing) | manual (owner) | — |
| D. Remove Apple Pay from the iOS subscription path | later, deliberate | A live |

A is the spine everything funnels into (`activateSubscription`) and is the only
piece with no Apple dependency, so it is specced and built first.

## Gate before building (from the briefing — do not skip)

Read the **actual App Store rejection** in App Store Connect and confirm it is
Guideline 3.1.1 (digital goods must use IAP). "Asks for Apple Pay" could instead
be an **entitlement mismatch** on `merchant.com.draftright.app.v2`
(`DraftRightMobile/ios/Runner/Runner.entitlements:39`) — a delete-the-capability
fix measured in minutes, not this multi-week build. IAP is Apple-required for
digital subscriptions regardless, so building it is a valid long-term move; but
the *urgency* and *sequencing* depend on the rejection reason. This spec is the
long-term build; the interim unblock (if any) is a separate, smaller task.

## What already exists — reuse, do not rebuild

Verified against the tree 2026-08-28 (re-check; the repo moves):

- `internal/payment/strategy/strategy.go:136` — the `Strategy` port
  (`CreateCheckout`, `CustomerPortalURL`, `CancelSubscription`, `VerifyWebhook`),
  one package per provider (`lemonsqueezy`, `paypal`, `stripe`, `vietqr`).
- `internal/payment/methods.go` — `registeredMethods`, the canonical method list.
- `internal/payment/strategy_method_parity_test.go` — the RULE #1 machine that
  asserts strategy method constants match the canonical ones. Any new method
  must be covered by it.
- `internal/payment/domain.go:60` — `StoreAppleIAP StoreType = "apple_iap"`
  **already a constant**, and `apple_iap` is already a `subscriptions.store_type`
  Postgres enum value (`schema.sql:75`).
- `internal/payment/domain.go:72` — `StoreTypeForMethod`; today
  `MethodApplePay, MethodGooglePay → StoreStripe` (Apple Pay is a Stripe
  PaymentIntent with a wallet intent).
- `internal/payment/webhook.go:243` — `activateSubscription`, the **single** path
  that grants Pro and sends the activation email. StoreKit funnels into it.
- `subscriptions` already has `store_type` + `store_transaction_id` columns.

So the DB and part of the Go plumbing already anticipate `apple_iap`. The work is
producing it and verifying transactions — not inventing a subscription model.

## Architecture decision: one honest `Strategy`, no new capability

**Chosen (owner):** Apple IAP is a `PaymentMethod` implementing the existing
`Strategy` interface — one interface for every provider, so admin, method
listing, reporting, and the parity test treat all providers uniformly, and a
future **Google Play IAP** fits the same shape. Preferred over a separate
`Redeemer` port.

**The recon (2026-08-28) showed the interface already accommodates a
redemption-style provider honestly — no `Kind()` capability is needed** (an
earlier draft of this spec added one; it was dropped because it duplicates what
`registeredMethods` already encodes, which would itself be a RULE #1 violation):

- `CustomerPortalURL` is documented to return `""` when there is no portal
  (`strategy.go:137`). IAP returns the real `itms-apps://apps.apple.com/account/subscriptions`
  manage URL — honest, non-empty.
- `CancelSubscription` is documented to return `false` when the provider can't
  cancel (`strategy.go:139`). Apple forbids server-side cancel → IAP returns
  `false`. Honest, per the existing contract.
- `registeredMethods` (`methods.go:18`) **is** the "can check out" list. **IAP is
  not a checkout method** — the purchase happens on-device — so it is **not added
  to `registeredMethods`**, and `CreateCheckout` is therefore never routed to it.
  IAP's `CreateCheckout` returns an explicit `ErrNotCheckoutMethod`
  (honest, never reached in a correct flow).

So IAP touches the system in **two honest places**:

1. **A new redeem endpoint** — `POST /payment/apple/redeem`. After the on-device
   purchase, the client posts the signed StoreKit transaction; the server
   verifies it and grants. This fits **none** of the four `Strategy` methods
   (it's not a checkout, portal, cancel, or webhook), so it is a **new handler +
   use-case**, not a `Strategy` method.
2. **`VerifyWebhook`** carries **App Store Server Notifications V2** (renewals,
   cancellations, refunds, grace) on `POST /payment/webhook/apple`, exactly like
   the other providers' webhooks.

Both funnel into the existing grant/extend writers. No new interface machinery.

## Components (each: one purpose, one interface, testable alone)

1. **`applestore` strategy package** (`internal/payment/strategy/applestore/`) —
   implements `Strategy`, mirroring the `stripe` package's test-seam shape:
   `CreateCheckout` → `ErrNotCheckoutMethod`; `CustomerPortalURL` → the
   `itms-apps://` manage URL; `CancelSubscription` → `false`; `VerifyWebhook` →
   parse + verify an App Store Server Notification V2 into a `WebhookAction`.
2. **Transaction/notification verifier** (`applestore/verify.go`) — verifies a
   signed StoreKit JWS (the `x5c` cert chain to Apple's root, ES256 signature,
   plus `bundleId` and `environment` checks). A `verify func(...)` seam injected
   in `New(...)` so unit tests never hit Apple (embedded root cert + fixture
   payloads).
3. **product → plan resolver** — App Store product id → plan id, **mirroring the
   LemonSqueezy pattern** (recon §8): `apple_product_monthly` / `apple_product_yearly`
   stored as typed `app_settings` columns (like the LS/PayPal creds), resolved via
   the existing `FindFirstActivePlanID(ctx, billing, currency)`. This **adds two
   settings columns** (a reversible migration + `queries_core.sql` + `sqlc generate`).
   One resolver, one source of truth; a guard test asserts both product ids
   resolve to an active plan (a mismatch would grant the wrong plan silently).
4. **Redeem use-case** — orchestrates the client-transaction path: verify →
   resolve product→plan → grant via the existing activation chokepoint → stamp
   `store_type = apple_iap` + `store_transaction_id` (the original transaction id,
   so later notifications match). No second grant path.
5. **Notification handling** — `VerifyWebhook` + `HandleWebhook`'s action switch:
   map the notification type (`SUBSCRIBED`, `DID_RENEW`,
   `DID_CHANGE_RENEWAL_STATUS`, `EXPIRED`, `REFUND`, `GRACE_PERIOD_EXPIRED`, …) to
   an entitlement update. First cycle stamps the store ref; renewals call
   `ExtendByStoreRef` and **must not re-send the activation email**.

## Data flow

```
Client (StoreKit 2, later spec)
  └─ signed transaction ──▶ POST /payment/apple/redeem
                              └─ verifier (JWS, bundle, env)
                                 └─ product→plan resolver (one map)
                                    └─ activateSubscription  ◀── the ONE grant chokepoint
                                       └─ record store_type=apple_iap, store_transaction_id

Apple ──▶ POST /payment/webhook/apple (ASSN V2, matches existing webhook route convention)
            └─ verify signed notification
               └─ map type → entitlement update
                  └─ activateSubscription (renew: no email) / revoke / extend expires_at
```

Endpoint paths are indicative; match the existing router's convention.

## RULE #1 — the machines that keep it honest

| Concern | Single source of truth | Machine that proves it |
|---|---|---|
| Granting Pro | `activateSubscription` | no second grant path; verified by code review + a test that redemption calls it |
| method const (payment ↔ strategy pkg) | `payment.MethodAppleIAP` | extend `strategy_method_parity_test.go` with `{strategy.MethodAppleIAP, MethodAppleIAP}` |
| store-type production | `StoreTypeForMethod` (`MethodAppleIAP → StoreAppleIAP`) | one mapping; parity/enum tests |
| product id → plan id | one resolver map | bijection guard test (plan↔product) |
| mobile method identity | `payment_method.dart` enum + `wireName` | never branch on the wire string (the file already states this rule) |

## Open items to verify in code before implementing (from the briefing)

1. **`activateSubscription` on an already-active subscription** — does it re-send
   the activation email? Apple renewals fire frequently; the renewal path must
   not email on every renewal. Decide: a `notify` flag, or a separate
   renew-vs-first-grant entry.
2. **`expires_at` on renewal** — Apple's expiry moves forward each renewal.
   Confirm the existing writer's `Grant` extends an active subscription's
   `expires_at` rather than creating a duplicate or ignoring the extension.

These are pre-implementation checks, not open design questions — the answers set
the shape of the renewal path but do not change the architecture above.

## Testing (no Apple dependency to run)

- Verifier: valid transaction; tampered signature; wrong `bundleId`; sandbox vs
  prod environment; expired cert chain. Sample JWS payloads as fixtures.
- product→plan resolver + its bijection guard.
- Redemption use-case: grants once, records the right `store_type` +
  `store_transaction_id`, funnels through `activateSubscription`.
- Notification handler: each notification type → the right entitlement update;
  renewal does **not** re-send email.
- Extended parity test (method ↔ strategy ↔ kind).

## Explicitly out of scope for Spec 1

Client StoreKit 2 / Flutter purchase + restore flow; App Store Connect product
setup; removing the Apple Pay entitlement. Each is its own spec or manual task,
sequenced after this one is green.
