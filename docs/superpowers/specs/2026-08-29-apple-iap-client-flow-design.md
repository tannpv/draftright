# Design — Apple IAP: client StoreKit flow + web-steering removal (Spec 2)

**Date:** 2026-08-29
**Status:** design for review; precedes an implementation plan.
**Companions:** `docs/HANDOFF-2026-08-28-storekit-iap.md`;
`docs/superpowers/specs/2026-08-28-apple-iap-server-redemption-design.md` (Spec 1,
the server side — **merged, PR #217**).

**Spec 2** is the **iOS client** half: let a user buy DraftRight Pro *in the app*
via StoreKit 2, restore purchases, and send the signed transaction to the
already-built `POST /payment/apple/redeem`. It also **removes the "subscribe on
the web" steering** from the iOS subscription screen.

## Why — the actual App Store rejection

The app was rejected on **two** guidelines, and one iOS design causes both:

- **3.1.1 (Payments — In-App Purchase):** digital subscriptions must be sold
  through IAP. The iOS subscription screen today
  (`DraftRightMobile/lib/screens/subscription_screen.dart:251-266`) is
  status-only and tells the user *"Manage your DraftRight plan from your account
  **on the web**."* Removing the in-app external checkout was the right instinct,
  but **steering the user to buy on the web is still a 3.1.1 violation** — Apple
  forbids directing users to any purchase mechanism other than IAP.
- **2.3.10 (Accurate Metadata):** the App Store listing (description/screenshots)
  reflects that external/web purchase path. Apple flags metadata that references
  purchasing outside IAP.

**Both are fixed by the same change:** give iOS a real in-app IAP purchase and
delete the web-steering. (The metadata edit in App Store Connect is manual and
the owner's — see "Out of scope".)

## Scope

**In:** the **iOS** client — StoreKit 2 product fetch, purchase, restore, the
transaction listener, sending the signed transaction to `/payment/apple/redeem`,
refreshing subscription status, and replacing the web-steering panel with the
buy/restore UI.

**Out:** Android (Google Play billing is a separate question — the handoff says
scope it on its own evidence; this spec does not touch the Android flow); App
Store Connect product setup (manual, owner); the App Store **metadata** edit
(manual, owner); the server side (done in Spec 1).

## What already exists

- **Server:** `POST /payment/apple/redeem` (JWT-authed) — body
  `{"signedTransaction": "<JWS string>"}` → 200 on grant. It verifies the JWS
  (chain→Apple root, ES256, bundle/env), resolves product→plan, grants Pro, and
  stamps the original transaction id (Spec 1 / #217).
- **Client:** `SubscriptionScreen` fetches status via `_backend.getSubscription()
  → SubscriptionInfo`; `PaymentMethodKind.appleIap` (`'apple_iap'`) already in the
  SSOT enum (Spec 1 Task 10). No `in_app_purchase` dependency yet.

## Architecture decision — StoreKit 2 via `in_app_purchase`

Use the official Flutter **`in_app_purchase`** + **`in_app_purchase_storekit`**
plugins with **StoreKit 2 enabled**, rather than a hand-rolled native
StoreKit channel.

- **Why the plugin:** it owns the transaction queue, the async transaction
  listener, and restore — the parts that are error-prone to hand-roll. It is
  maintained by the Flutter team and already used across the ecosystem.
- **Why StoreKit 2 specifically:** the backend verifies a StoreKit **2** signed
  transaction (JWS). `in_app_purchase_storekit` with SK2 surfaces that JWS as
  `PurchaseDetails.verificationData.serverVerificationData` — which is exactly
  the `signedTransaction` the redeem endpoint expects. (Confirm at
  implementation time that SK2 mode yields the JWS, not an SK1 base64 receipt;
  if the plugin version in use only exposes SK1 for this field, fall back to a
  thin native SK2 channel returning `Transaction.jwsRepresentation`.)

**User↔transaction linkage:** set `appAccountToken = <userID>` on the purchase
(StoreKit 2 supports it). This travels inside the signed transaction, so a later
server change can grant from an App Store notification even if the redeem POST is
lost (Spec 1's deferred M3 recovery). It is best practice regardless.

## Components (iOS)

1. **`AppleIapService`** (`lib/services/payment/apple_iap_service.dart`) — wraps
   `in_app_purchase`: `available()`, `products()` (query by the configured product
   ids), `buy(planKind)` (with `appAccountToken = userID`), `restore()`, and a
   `purchaseStream` listener that, for each `purchased`/`restored` detail:
   sends `serverVerificationData` → `POST /payment/apple/redeem`, then
   `completePurchase(detail)` (StoreKit requires finishing the transaction), then
   triggers a subscription-status refresh. Errors surface to the UI; a failed
   redeem must NOT finish the transaction (so it retries).
2. **Product ids** — the two App Store product ids (monthly/yearly). These must
   equal the backend's `apple_product_monthly/yearly` settings. **One source of
   truth:** the app fetches them from a single place (a compile-time const map
   or, preferred, from the backend via the existing subscription/config response)
   — not retyped at call sites. A mismatch buys the wrong plan.
3. **Subscription screen (iOS branch)** — replace the status-only "manage on the
   web" panel with: the plan/price (from `products()`), a **Buy** button per
   billing period, and a **Restore Purchases** button (Apple requires a restore
   path). Remove the "on the web" text entirely (fixes 2.3.10). After a
   successful purchase/restore, re-fetch `getSubscription()` so the UI reflects Pro.

## Flow

```
iOS Subscription screen
  ├─ products() ─────────────▶ show price + Buy(monthly|yearly) + Restore
  ├─ Buy ─▶ in_app_purchase.buyNonConsumable(appAccountToken=userID)
  │           └─ StoreKit sheet ─▶ purchaseStream: PurchaseDetails(purchased)
  │                └─ POST /payment/apple/redeem { serverVerificationData }
  │                     ├─ 200 ─▶ completePurchase(detail) ─▶ refresh getSubscription() ─▶ Pro
  │                     └─ !200 ─▶ surface error, DO NOT complete (retries)
  └─ Restore ─▶ in_app_purchase.restorePurchases()
                 └─ purchaseStream: PurchaseDetails(restored) ─▶ same redeem path
```

## RULE #1

- **Product ids: one source of truth.** Do not retype the two ids at call sites;
  resolve them once (backend-provided or one const), matching the server's
  `apple_product_*` settings. A drift buys the wrong plan silently.
- **Do not branch on the wire string.** iOS purchase uses
  `PaymentMethodKind.appleIap` (already in the enum); the redeem call is the
  single place that knows the transport shape.
- **One status source.** Pro status stays `getSubscription()`; the purchase flow
  triggers a refresh — it does not compute entitlement locally.
- **Reuse the backend contract**, don't reshape it: send exactly
  `{"signedTransaction": ...}` to the existing endpoint.

## Open items to decide/verify before building

1. **Does the pinned `in_app_purchase_storekit` version, in SK2 mode, expose the
   JWS in `serverVerificationData`?** If not, a thin native channel returning
   `Transaction.jwsRepresentation` is the fallback. Verify first — it changes the
   service's internals (not its interface).
2. **Product-id source:** compile-time const vs backend-provided. Preferred:
   backend-provided (one source shared with the server's settings) — confirm the
   subscription/config response can carry them, else a documented const with a
   note to keep it in sync with the `apple_product_*` settings.
3. **appAccountToken server read:** Spec 2 sets it; a small Spec 1 follow-up reads
   it for notification recovery (out of scope here, cross-referenced).

## Testing

- `AppleIapService` unit tests with a fake `in_app_purchase` platform: a
  `purchased` detail → posts the JWS to the redeem client + completes on 200 +
  does NOT complete on non-200; a `restored` detail → same redeem path; `error`
  detail → surfaced, not completed.
- Widget test: the iOS subscription branch renders Buy + Restore (not the
  "on the web" text); a stubbed successful purchase flips the panel to Pro.
- The real StoreKit sheet / sandbox purchase is a **device + App Store Connect**
  step (needs products configured) — a manual TestFlight/sandbox pass, not
  automatable here.

## Out of scope (owner / later)

- App Store Connect: create the subscription group + the two products + pricing,
  and set the matching `apple_product_monthly/yearly` backend settings.
- The App Store **metadata** edit (description/screenshots) to remove
  external-purchase / other-platform references — the other half of the 2.3.10
  fix, done in App Store Connect.
- Android IAP (Google Play) — separate evidence, separate spec.
- Spec 1 follow-ups #218 (admin PATCH for product settings) / #219 (REVOKE/
  DID_FAIL_TO_RENEW).
