# HANDOFF — StoreKit / In-App Purchase for iOS

**Date:** 2026-08-28
**Status:** brief only. Nothing implemented, no spec written yet.

Written to be handed to whoever picks this up — paste it whole into a coding
session, or read it as a briefing. Everything below was verified against this
repository on the date above; file and line references were correct then and are
worth re-checking, since this codebase moves.

---

Implement StoreKit / In-App Purchase for the iOS app.

## Why this is needed

The App Store is rejecting the app over payments. DraftRight sells **recurring
subscriptions** (monthly/yearly — `DraftRightMobile/lib/screens/subscription_screen.dart`
has `BillingPeriod.monthly`/`yearly`) through Apple Pay, and there is **no
StoreKit or in-app-purchase code anywhere in the repo** — grep for `storekit`,
`in_app_purchase`, `SKProduct`, `InAppPurchase` across Swift, Dart and pubspec
returns nothing.

Apple's Guideline 3.1.1 requires digital content consumed inside the app to be
sold through In-App Purchase. Apple Pay is for physical goods and services.
Adding a merchant ID or fixing the entitlement will not clear review — the
objection is the payment path itself.

**Before building anything, read the actual rejection text in App Store Connect
and confirm it is 3.1.1.** "Asks for Apple Pay" could also mean an entitlement
mismatch on `merchant.com.draftright.app.v2`
(`DraftRightMobile/ios/Runner/Runner.entitlements:39`), which would be a far
smaller fix — possibly deleting the capability. Do not start a multi-week
StoreKit build on a guess.

## What already exists — reuse it, do not rebuild it

Verified in the codebase:

- **`internal/payment/strategy/strategy.go:136`** — a `Strategy` port with
  `CreateCheckout`, `CustomerPortalURL`, `CancelSubscription`, `VerifyWebhook`.
  One package per provider under `internal/payment/strategy/`: `lemonsqueezy`,
  `paypal`, `stripe`, `vietqr`.
- **`internal/payment/methods.go`** — `registeredMethods`, the canonical method
  list.
- **`internal/payment/strategy_method_parity_test.go`** — already a Rule #1
  machine: asserts the strategy package's method constants match the payment
  package's canonical ones so the two sets cannot drift. Any new method must be
  covered by it.
- **`internal/payment/domain.go:60`** — `StoreAppleIAP StoreType = "apple_iap"`
  **already exists as a constant**, and `apple_iap` is already a value in the
  `subscriptions.store_type` Postgres enum along with `google_play`. Nothing
  produces it yet: `StoreTypeForMethod` (`domain.go:72`) maps
  `MethodApplePay, MethodGooglePay → StoreStripe`, because Apple Pay today is a
  Stripe PaymentIntent carrying a `WalletIntent` with the merchant identifier
  (`strategy.go:57`).
- **`subscriptions`** already has `store_type` and `store_transaction_id`
  columns — the schema was designed for app-store purchases.
- **`internal/payment/webhook.go:243`** — `activateSubscription` is the single
  path that grants Pro and sends the activation email. StoreKit must funnel into
  it, not add a second grant path.

So the database and part of the Go plumbing already anticipate this. The work is
filling in what produces `apple_iap`, not inventing a subscription model.

## The design decision already taken

**StoreKit gets its own small port, not a `Strategy` implementation.**

Every existing provider works server-first: the server creates a checkout, the
user completes it elsewhere, a webhook confirms. StoreKit inverts that — Apple's
sheet completes the purchase on-device and only then does the server receive a
signed transaction to verify.

Three of `Strategy`'s four methods have no honest implementation:

| Strategy method | StoreKit |
|---|---|
| `CreateCheckout` | nothing to create; the purchase already happened on-device |
| `CancelSubscription` | Apple forbids server-side cancellation — only the user, in Settings |
| `CustomerPortalURL` | an `itms-apps://` deep link, not a server-generated URL |
| `VerifyWebhook` | ✅ App Store Server Notifications V2 fit cleanly |

Add a second port beside `Strategy` — something like `Redeemer`, with
**verify a client-supplied transaction** and **verify a provider notification**.
Both are things Apple genuinely does. Do not make StoreKit satisfy `Strategy`
with `CreateCheckout` returning an empty result: an interface whose contract is
false for one implementation is worse than two honest interfaces.

Extend the parity test to cover both port's method sets.

## Rule #1 — this project's first rule

Every value that carries meaning has one source of truth; duplicated logic is
hardcoding because two copies drift; cross-cutting concerns get a chokepoint plus
a machine proving nothing bypassed it. Concretely here:

- **Do not add a second subscription-granting path.** `activateSubscription` is
  the chokepoint; StoreKit routes into it.
- **Do not add a second store-type mapping.** Extend `StoreTypeForMethod`, or
  give the new port its own mapping and cover both with the parity test.
- **Product identifiers are meaning-carrying values.** App Store product ids must
  map to existing plan ids in exactly one place, not be retyped at call sites.
  A mismatch grants the wrong plan silently.
- **The mobile app must not branch on wire strings.**
  `DraftRightMobile/lib/services/payment/payment_method.dart` already says so:
  *"Nothing else in the mobile app should branch on the wire string directly."*
  Add the new method to that enum and its `wireName` map.

## What to build

Work out the phasing yourself, but it spans at least:

1. **App Store Connect** — subscription group, products, pricing. Manual, and a
   prerequisite for everything else.
2. **Client purchase flow** — Flutter plus native StoreKit 2. Buy, restore
   purchases (Apple requires a restore path), and send the signed transaction to
   the backend.
3. **Server-side transaction verification** — validate against Apple, resolve
   product id to plan, grant through `activateSubscription`, record
   `store_transaction_id` and `store_type = apple_iap`.
4. **App Store Server Notifications V2** — renewals, cancellations, refunds,
   billing retries, grace periods. This is what keeps entitlement correct after
   the first purchase, and it is where most IAP integrations are weakest.
5. **Removing Apple Pay from the iOS subscription path**, since it is the thing
   being rejected. Decide deliberately whether the entitlement and merchant id
   come out too, or stay for a non-subscription use.

**Android is a separate question.** Google Play allows alternative billing in
more regions, so `googlePay` may be fine where `applePay` is not. Do not assume
symmetry; scope Android on its own evidence.

## How to approach it

This is large enough that it probably wants more than one spec. Use the
brainstorming skill first, assess whether it decomposes, and write the spec
before writing code — the project checklist requires a Rule #1 pass before
implementation, test cases before code, and a clean-garbage plus full-review pass
before merge.

Two specific things to verify rather than assume:

- **What `activateSubscription` expects.** It takes `billing, userID, planID,
  method` and sends an activation email. Renewals from Apple must not re-send
  that email on every renewal.
- **Whether `expires_at` semantics match.** Apple's subscription expiry moves on
  each renewal; check how the existing writer's `Grant` handles an already-active
  subscription being extended.
