# Apple IAP — Client StoreKit Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an iOS user buy DraftRight Pro in-app via StoreKit 2 and restore purchases, sending the signed transaction to the merged `POST /payment/apple/redeem`; and remove the "buy on the web" steering that causes the App Store 3.1.1 + 2.3.10 rejections.

**Architecture:** A standalone `AppleIapService` wraps the `in_app_purchase` plugin (StoreKit 2): query products, purchase, restore, and a transaction listener that posts the JWS `serverVerificationData` to a new `BackendClient.redeemAppleTransaction`, finishes the transaction on 200, and refreshes subscription status. The iOS branch of `SubscriptionScreen` shows Buy + Restore instead of the web-steering text. Android is untouched.

**Tech Stack:** Flutter/Dart (`in_app_purchase` + `in_app_purchase_storekit`, StoreKit 2), the existing `BackendClient`/`_authed` pattern, hand-written test fakes (no mockito).

## Global Constraints

- **iOS only.** Do not touch the Android/other-platform payment flow. All new behaviour is behind the iOS gate.
- **No raw `Platform.isIOS` at a call site.** Platform gating goes through the `PaymentService` chokepoint getter (today `inAppCheckoutAllowed`, `payment_service.dart:84-89`). Refine that getter's meaning; don't scatter platform checks (RULE #1).
- **Every authed backend call routes through `BackendClient._authed`** (`backend_client.dart:163-180`) — never a bare `_api.postJson` (it handles 401 refresh).
- **Redeem contract is fixed:** `POST /payment/apple/redeem`, body `{"signedTransaction": "<JWS>"}`, 200 on grant. Send exactly that; do not reshape it.
- **Product ids are meaning-carrying:** the two App Store product ids must equal the backend's `apple_product_monthly/yearly` settings. Keep them in ONE place in the app (a documented const), never retyped at call sites; a drift buys the wrong plan. (A backend-served source is the better long-term SSOT — issue #218 — but is out of this iOS-only spec.)
- **Do not compute entitlement locally.** Pro status stays `BackendClient.getSubscription()`; the purchase flow triggers a refresh (`_load()`), mirroring `_onCancelTap`'s explicit-refresh-after-mutation (`subscription_screen.dart:407`), NOT the lifecycle-resume path.
- **`appAccountToken` is deferred** (no client user-UUID exists; the redeem is JWT-authed so the server knows the user). Notification-recovery (Spec 1 M3) that needs it is a future follow-up.
- **A failed redeem must NOT finish the StoreKit transaction** — so it stays pending and retries; only `completePurchase` on a 200.
- **Errors are reported, not swallowed:** mirror `WalletPaymentHandler` — `DRLogger.error` + `ErrorReporter.reportHandled(...)` then surface a friendly message to the screen's SnackBar.
- Run all Flutter commands from `DraftRightMobile/`.

---

### Task 1: Add the StoreKit dependencies

**Files:**
- Modify: `DraftRightMobile/pubspec.yaml` (`dependencies:`, near `flutter_stripe`)

**Interfaces:**
- Produces: `package:in_app_purchase/in_app_purchase.dart` + `package:in_app_purchase_storekit/...` available to import.

- [ ] **Step 1: Add the deps**

In `pubspec.yaml` `dependencies:`, alongside `flutter_stripe`, add (use the latest caret-compatible versions the resolver picks; these plugins are federated — `in_app_purchase` pulls the storekit impl, but pin it explicitly so SK2 config is reachable):
```yaml
  in_app_purchase: ^3.2.0
  in_app_purchase_storekit: ^0.3.0
```

- [ ] **Step 2: Resolve + verify build**

Run: `cd DraftRightMobile && flutter pub get && flutter analyze --no-fatal-infos`
Expected: deps resolve; analyze has no NEW errors (pre-existing infos ok).

- [ ] **Step 3: Commit**

```bash
git add DraftRightMobile/pubspec.yaml DraftRightMobile/pubspec.lock
git commit -m "chore(mobile): add in_app_purchase + storekit deps for Apple IAP"
```

---

### Task 2: `BackendClient.redeemAppleTransaction`

**Files:**
- Modify: `DraftRightMobile/lib/services/backend_client.dart`
- Test: `DraftRightMobile/test/services/redeem_apple_transaction_test.dart`

**Interfaces:**
- Produces: `Future<void> BackendClient.redeemAppleTransaction(String signedTransaction)` — posts to `/payment/apple/redeem` via `_authed`; throws `ApiException` on non-2xx.

- [ ] **Step 1: Write the failing test**

Create `redeem_apple_transaction_test.dart`. Mirror the existing fake pattern (`_FakeBackend extends BackendClient`), but here test the real method against a fake `http.Client` (the class accepts `httpClient`). Simplest: inject a `MockClient` from `package:http/testing.dart` that asserts the POST path + body and returns 200:
```dart
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/services/backend_client.dart';
// + the app's AuthService + a stub token provider

void main() {
  test('redeemAppleTransaction posts the signed transaction to /payment/apple/redeem', () async {
    late http.Request seen;
    final client = MockClient((req) async {
      seen = req;
      return http.Response('{}', 200);
    });
    final backend = BackendClient(
      auth: fakeAuth(token: 'jwt'),                 // helper: AuthService stub returning a token
      getBaseUrl: () => 'https://api.test',
      httpClient: client,
    );
    await backend.redeemAppleTransaction('signed-jws-1');
    expect(seen.method, 'POST');
    expect(seen.url.path, '/payment/apple/redeem');
    expect(jsonDecode(seen.body), {'signedTransaction': 'signed-jws-1'});
    expect(seen.headers['authorization'], 'Bearer jwt');
  });
}
```
(If constructing a real `AuthService` stub is heavy, follow the `_FakeBackend extends BackendClient` override pattern used in `payment_plan_resolution_test.dart` instead and assert the call is made — but the MockClient form above also proves the exact wire body, which is the point.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/redeem_apple_transaction_test.dart`
Expected: FAIL — `redeemAppleTransaction` not defined.

- [ ] **Step 3: Implement (mirror `createCheckout`, `backend_client.dart:327-337`)**

```dart
  /// POST /payment/apple/redeem — verify + grant Pro from a StoreKit
  /// transaction. 200 on grant; throws ApiException otherwise.
  Future<void> redeemAppleTransaction(String signedTransaction) async {
    await _authed((t) => _api.postJson(
          '/payment/apple/redeem',
          body: {'signedTransaction': signedTransaction},
          token: t,
        ));
  }
```

- [ ] **Step 4: Run — verify pass**

Run: `cd DraftRightMobile && flutter test test/services/redeem_apple_transaction_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/backend_client.dart DraftRightMobile/test/services/redeem_apple_transaction_test.dart
git commit -m "feat(mobile): BackendClient.redeemAppleTransaction -> POST /payment/apple/redeem"
```

---

### Task 3: Apple product-id source (one place)

**Files:**
- Create: `DraftRightMobile/lib/services/payment/apple_products.dart`
- Test: `DraftRightMobile/test/services/payment/apple_products_test.dart`

**Interfaces:**
- Produces: `class AppleProducts { static const monthly = '...'; static const yearly = '...'; static const ids = {monthly, yearly}; static String? billingFor(String productId); }`.

- [ ] **Step 1: Write the failing test**

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';

void main() {
  test('product ids map to billing periods, one source of truth', () {
    expect(AppleProducts.ids, {AppleProducts.monthly, AppleProducts.yearly});
    expect(AppleProducts.billingFor(AppleProducts.monthly), 'monthly');
    expect(AppleProducts.billingFor(AppleProducts.yearly), 'yearly');
    expect(AppleProducts.billingFor('unknown'), isNull);
  });
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/payment/apple_products_test.dart`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

```dart
/// The App Store product identifiers for DraftRight Pro. These MUST equal the
/// backend `apple_product_monthly` / `apple_product_yearly` settings and the
/// products configured in App Store Connect — a drift buys the wrong plan.
/// (Backend-served ids would be the better single source of truth — issue #218
/// — but that's out of the iOS-only client spec.)
class AppleProducts {
  static const String monthly = 'com.draftright.pro.monthly';
  static const String yearly = 'com.draftright.pro.yearly';
  static const Set<String> ids = {monthly, yearly};

  static String? billingFor(String productId) {
    switch (productId) {
      case monthly:
        return 'monthly';
      case yearly:
        return 'yearly';
      default:
        return null;
    }
  }
}
```
(Confirm the exact product-id strings with the owner's App Store Connect setup before shipping; the values here are placeholders to be replaced with the real product ids — they must match the backend settings.)

- [ ] **Step 4: Run — verify pass**

Run: `cd DraftRightMobile && flutter test test/services/payment/apple_products_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/payment/apple_products.dart DraftRightMobile/test/services/payment/apple_products_test.dart
git commit -m "feat(mobile): Apple product-id -> billing map (one source)"
```

---

### Task 4: `AppleIapService`

The core: wraps `in_app_purchase`, drives buy/restore, redeems via the backend, finishes transactions. Testable against a fake `InAppPurchasePlatform` + a fake backend.

**Files:**
- Create: `DraftRightMobile/lib/services/payment/apple_iap_service.dart`
- Test: `DraftRightMobile/test/services/payment/apple_iap_service_test.dart`

**Interfaces:**
- Consumes: `InAppPurchase.instance` (overridable seam for tests), `BackendClient.redeemAppleTransaction` (Task 2), `AppleProducts` (Task 3).
- Produces:
  ```dart
  class AppleIapService {
    AppleIapService(this._backend, {InAppPurchase? iap, void Function()? onEntitlementChanged});
    Future<bool> available();
    Future<List<ProductDetails>> products();
    Future<void> buy(String productId);      // productId ∈ AppleProducts.ids
    Future<void> restore();
    void dispose();
  }
  ```
  On each `purchased`/`restored` detail from the plugin's `purchaseStream`: call `_backend.redeemAppleTransaction(detail.verificationData.serverVerificationData)`, then `iap.completePurchase(detail)` on success + invoke `onEntitlementChanged`; on redeem failure do NOT complete (log + report + surface). On `error`/`canceled` details: surface/ignore, do not complete a non-purchased detail.

- [ ] **Step 1: Write the failing test**

Use a fake `InAppPurchase` (subclass or a thin wrapper interface if `InAppPurchase` isn't directly subclassable — introduce a minimal `IapPlatform` seam the service depends on, with a real impl delegating to `InAppPurchase.instance` and a fake for tests). Drive a `purchased` detail through a `StreamController` and assert the redeem+complete sequence:
```dart
// Fake IAP exposes a purchaseStream controller, records completePurchase calls,
// and returns canned products. Fake backend records redeemAppleTransaction args
// and can be made to throw.
test('purchased detail -> redeem then completePurchase, entitlement callback fires', () async {
  final iap = FakeIap();
  final backend = FakeBackend();
  var changed = 0;
  final svc = AppleIapService(backend, iap: iap, onEntitlementChanged: () => changed++);
  iap.emit(purchased('signed-jws', AppleProducts.monthly));
  await pumpEventQueue();
  expect(backend.redeemed, ['signed-jws']);
  expect(iap.completed, hasLength(1));
  expect(changed, 1);
  svc.dispose();
});

test('redeem failure does NOT complete the transaction (so it retries)', () async {
  final iap = FakeIap();
  final backend = FakeBackend()..failRedeem = true;
  final svc = AppleIapService(backend, iap: iap);
  iap.emit(purchased('signed-jws', AppleProducts.monthly));
  await pumpEventQueue();
  expect(backend.redeemed, ['signed-jws']);   // attempted
  expect(iap.completed, isEmpty);             // NOT finished
  svc.dispose();
});
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/payment/apple_iap_service_test.dart`
Expected: FAIL — `AppleIapService` undefined.

- [ ] **Step 3: Implement**

```dart
import 'dart:async';
import 'package:in_app_purchase/in_app_purchase.dart';
import '../backend_client.dart';
import '../../helpers/logger.dart';            // DRLogger (confirm path)
import '../error_reporter.dart';               // ErrorReporter (confirm path)
import 'apple_products.dart';

class AppleIapService {
  final BackendClient _backend;
  final InAppPurchase _iap;
  final void Function()? _onEntitlementChanged;
  StreamSubscription<List<PurchaseDetails>>? _sub;

  AppleIapService(this._backend, {InAppPurchase? iap, void Function()? onEntitlementChanged})
      : _iap = iap ?? InAppPurchase.instance,
        _onEntitlementChanged = onEntitlementChanged {
    _sub = _iap.purchaseStream.listen(_onPurchases, onError: (e, st) {
      DRLogger.error('iap stream error', e, st);
    });
  }

  Future<bool> available() => _iap.isAvailable();

  Future<List<ProductDetails>> products() async {
    final resp = await _iap.queryProductDetails(AppleProducts.ids);
    return resp.productDetails;
  }

  Future<void> buy(String productId) async {
    final resp = await _iap.queryProductDetails({productId});
    final product = resp.productDetails.firstWhere((p) => p.id == productId);
    // Non-consumable purchase; StoreKit surfaces the transaction on purchaseStream.
    await _iap.buyNonConsumable(purchaseParam: PurchaseParam(productDetails: product));
  }

  Future<void> restore() => _iap.restorePurchases();

  Future<void> _onPurchases(List<PurchaseDetails> details) async {
    for (final d in details) {
      if (d.status == PurchaseStatus.purchased || d.status == PurchaseStatus.restored) {
        try {
          await _backend.redeemAppleTransaction(d.verificationData.serverVerificationData);
          if (d.pendingCompletePurchase) {
            await _iap.completePurchase(d);   // ONLY after a successful grant
          }
          _onEntitlementChanged?.call();
        } catch (e, st) {
          DRLogger.error('apple redeem failed', e, st);
          ErrorReporter.reportHandled(e, stack: st, severity: 'warning', context: {'stage': 'apple_redeem'});
          // Do NOT complete — the transaction stays pending and retries.
          rethrow;
        }
      } else if (d.status == PurchaseStatus.error) {
        DRLogger.error('apple purchase error', d.error, null);
      }
      // canceled/pending: nothing to do.
    }
  }

  void dispose() => _sub?.cancel();
}
```
(Confirm the real `DRLogger`/`ErrorReporter` import paths from `wallet_payment_handler.dart`. If `InAppPurchase` can't be faked directly in the test, introduce a tiny `IapPlatform` interface the service takes instead of `InAppPurchase`, with a prod impl delegating to `InAppPurchase.instance`; keep the public interface above unchanged.)

- [ ] **Step 4: Run — verify pass**

Run: `cd DraftRightMobile && flutter test test/services/payment/apple_iap_service_test.dart`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/payment/apple_iap_service.dart DraftRightMobile/test/services/payment/apple_iap_service_test.dart
git commit -m "feat(mobile): AppleIapService — StoreKit buy/restore -> redeem, finish on grant"
```

---

### Task 5: iOS gate — allow the IAP path

Today `PaymentService.inAppCheckoutAllowed` returns false on iOS ("no in-app checkout at all"). That was for non-IAP rails. StoreKit IAP is compliant, so iOS must allow the IAP path while still hiding the external-checkout tiles. Add a distinct getter rather than overloading the old one.

**Files:**
- Modify: `DraftRightMobile/lib/services/payment_service.dart` (~84-96)
- Test: `DraftRightMobile/test/services/payment/payment_status_test.dart` (or a new `payment_platform_gate_test.dart`)

**Interfaces:**
- Produces: `static bool get appleIapAllowed` — true only on iOS (not web, not Android). `inAppCheckoutAllowed` keeps its meaning (external-rail tiles) and stays false on iOS.

- [ ] **Step 1: Write the failing test**

```dart
test('appleIapAllowed is iOS-only; external checkout stays off on iOS', () {
  // These read Platform via the same guarded pattern; on the test host
  // (non-iOS) appleIapAllowed is false and inAppCheckoutAllowed is true —
  // assert the getters exist and are mutually consistent for the host.
  expect(PaymentService.appleIapAllowed, isA<bool>());
  // On a non-iOS test host both reduce to the host's platform; the key
  // invariant: they are never BOTH true on iOS. (Device-verified separately.)
  expect(PaymentService.appleIapAllowed && !PaymentService.inAppCheckoutAllowed || !PaymentService.appleIapAllowed, isTrue);
});
```
(Pragmatic: platform getters are hard to unit-test on a non-iOS host. Keep this test to the getter's existence + the mutual-exclusion invariant; the real iOS behaviour is device-verified. If the repo has a platform-override seam, prefer that and assert both iOS and non-iOS branches.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/services/payment/payment_status_test.dart`
Expected: FAIL — `appleIapAllowed` undefined.

- [ ] **Step 3: Implement**

In `payment_service.dart`, beside `inAppCheckoutAllowed`:
```dart
  /// iOS shows the StoreKit IAP path (compliant with 3.1.1), not the external
  /// checkout tiles. True only on iOS.
  static bool get appleIapAllowed => _platformIsIos;
```
(`_platformIsIos` already exists, `payment_service.dart:86-96`.)

- [ ] **Step 4: Run — verify pass**

Run: `cd DraftRightMobile && flutter test test/services/payment/payment_status_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/services/payment_service.dart DraftRightMobile/test/services/payment/payment_status_test.dart
git commit -m "feat(mobile): PaymentService.appleIapAllowed — iOS IAP path gate"
```

---

### Task 6: Subscription screen — Buy + Restore on iOS, remove web-steering

Replace the status-only "manage on the web" block (`subscription_screen.dart:251-266`) with the StoreKit purchase UI. This is the change that fixes the 3.1.1 + 2.3.10 rejections.

**Files:**
- Modify: `DraftRightMobile/lib/screens/subscription_screen.dart`
- Test: `DraftRightMobile/test/screens/subscription_screen_iap_test.dart`

**Interfaces:**
- Consumes: `AppleIapService` (Task 4), `PaymentService.appleIapAllowed` (Task 5), `AppleProducts` (Task 3), existing `_load()` refresh.

- [ ] **Step 1: Write the failing widget test**

```dart
// Pump SubscriptionScreen with a fake backend (isFree) + a fake AppleIapService
// injected; force the iOS/appleIapAllowed branch via the platform-override seam
// or by injecting the service. Assert:
testWidgets('iOS free plan shows Buy + Restore, not the web-steering text', (t) async {
  await t.pumpWidget(wrapWithIapAllowed(SubscriptionScreen(/* injected fakes */)));
  await t.pumpAndSettle();
  expect(find.textContaining('on the web'), findsNothing);   // 2.3.10: steering gone
  expect(find.text('Restore Purchases'), findsOneWidget);    // Apple-required
  expect(find.textContaining('Upgrade'), findsOneWidget);    // buy affordance
});
testWidgets('tapping Buy drives AppleIapService.buy then refreshes on success', (t) async {
  final iap = FakeAppleIapService();
  await t.pumpWidget(wrapWithIapAllowed(SubscriptionScreen(iapService: iap, backend: fakeBackend)));
  await t.pumpAndSettle();
  await t.tap(find.text('Upgrade to Pro (monthly)'));   // label per impl
  await t.pumpAndSettle();
  expect(iap.bought, [AppleProducts.monthly]);
});
```
(To make the screen testable, add optional injected `AppleIapService`/`BackendClient` constructor params defaulting to the real ones — mirror how the screen already constructs `_backend`/`_payments` in `initState`.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd DraftRightMobile && flutter test test/screens/subscription_screen_iap_test.dart`
Expected: FAIL — web-steering text still present / no Restore button.

- [ ] **Step 3: Implement**

Replace the `else` branch (lines 251-266) so the free-plan section is:
```dart
if (PaymentService.inAppCheckoutAllowed) ...[
  // ... unchanged external-rail tiles (Android/web) ...
] else if (PaymentService.appleIapAllowed) ...[
  Text('Upgrade to Pro', style: /* titleMedium bold */),
  const SizedBox(height: 8),
  BillingPeriodSelector(value: _billingPeriod, onChanged: (p) => setState(() => _billingPeriod = p)),
  const SizedBox(height: 16),
  FilledButton(
    onPressed: _iapBusy ? null : () => _onIapBuy(),          // buys AppleProducts.<period>
    child: Text(_iapBusy ? 'Processing…' : 'Upgrade to Pro (${_billingPeriod.name})'),
  ),
  const SizedBox(height: 8),
  TextButton(
    onPressed: _iapBusy ? null : () => _onIapRestore(),
    child: const Text('Restore Purchases'),
  ),
]
```
Add `_iap = AppleIapService(_backend, onEntitlementChanged: _load)` in `initState` (dispose it in `dispose`), and:
```dart
Future<void> _onIapBuy() async {
  setState(() => _iapBusy = true);
  try {
    final id = _billingPeriod == BillingPeriod.yearly ? AppleProducts.yearly : AppleProducts.monthly;
    await _iap.buy(id);                 // completion + _load() happen via onEntitlementChanged
  } catch (e) {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Purchase failed. Please try again.')));
  } finally {
    if (mounted) setState(() => _iapBusy = false);
  }
}
Future<void> _onIapRestore() async { /* same shape, _iap.restore(); */ }
```
Delete the "Manage your DraftRight plan from your account on the web…" text entirely.

- [ ] **Step 4: Run — verify pass + analyze**

Run: `cd DraftRightMobile && flutter test test/screens/subscription_screen_iap_test.dart && flutter analyze --no-fatal-infos`
Expected: PASS + no new analyze errors.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/lib/screens/subscription_screen.dart DraftRightMobile/test/screens/subscription_screen_iap_test.dart
git commit -m "feat(mobile): iOS IAP buy+restore, remove web-steering (fixes 3.1.1 + 2.3.10)"
```

---

## Final verification (after all tasks)

- [ ] `cd DraftRightMobile && flutter test && flutter analyze --no-fatal-infos` — green.
- [ ] Grep proves the steering is gone: `grep -rn "on the web" DraftRightMobile/lib/screens/subscription_screen.dart` → nothing.
- [ ] `AppleProducts` ids referenced from one place (grep the literal product-id strings — only in `apple_products.dart`).

## Out of scope (owner / later / needs devices)

- **App Store Connect:** create the subscription group + the two products with the real ids + pricing; set the backend `apple_product_monthly/yearly` to match; set `APPLE_BUNDLE_ID` + `APPLE_ENVIRONMENT`.
- **App Store metadata edit** (description/screenshots) to remove external-purchase / other-platform references — the other half of the 2.3.10 fix.
- **Live sandbox/TestFlight purchase verification** — needs the ASC products + a device; unit/widget tests here use fakes.
- **`appAccountToken` + notification-recovery** (Spec 1 M3) — deferred; needs a client user-UUID source (a `/me` endpoint or JWT decode) — a small follow-up.
- **Verify the pinned `in_app_purchase_storekit` SK2 mode yields the JWS** in `serverVerificationData`; if not, a thin native SK2 channel returning `Transaction.jwsRepresentation` (service internals only).
- Android IAP (Google Play) — separate spec.
