// Widget tests for the iOS StoreKit Buy + Restore UI on SubscriptionScreen —
// the change that removes the "manage on the web" status-only panel (App
// Store Guideline 2.3.10: inaccurate metadata pointing users off-platform)
// and replaces it with a real IAP purchase affordance (Guideline 3.1.1:
// digital subscriptions must be purchasable via StoreKit).
//
// Test seam: `SubscriptionScreen(backend:, iapService:)` are optional
// injected overrides (Step 3a) so this test never touches the network or
// real StoreKit. `AppleIapService`'s super-constructor listens on
// `InAppPurchase.instance.purchaseStream`, which on this macOS test host
// would register the real StoreKit platform unless we swap
// `InAppPurchasePlatform.instance` for a fake AND pin
// `debugDefaultTargetPlatformOverride` away from macOS/iOS/android first —
// exactly the guard `apple_iap_service_test.dart` uses (see its file-level
// comment for why the override must land before the first
// `InAppPurchase.instance` access).
import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:in_app_purchase_platform_interface/in_app_purchase_platform_interface.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment/apple_iap_service.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/payment_service.dart';
import 'package:draftright_mobile/screens/subscription_screen.dart';

/// Minimal fake `InAppPurchasePlatform` — only enough so
/// `AppleIapService`'s super-ctor `purchaseStream.listen(...)` is harmless.
/// Mirrors `FakeIapPlatform` in `apple_iap_service_test.dart`.
class _FakeIapPlatform extends InAppPurchasePlatform
    with MockPlatformInterfaceMixin {
  final _controller = StreamController<List<PurchaseDetails>>.broadcast();

  @override
  Stream<List<PurchaseDetails>> get purchaseStream => _controller.stream;

  @override
  Future<bool> isAvailable() async => true;
}

/// `BackendClient` returning a free `SubscriptionInfo` + no payment
/// methods, so the widget never hits the network. Mirrors the
/// `FakeBackend extends BackendClient` pattern in
/// `apple_iap_service_test.dart`.
class FakeBackend extends BackendClient {
  FakeBackend() : super(auth: AuthService(), getBaseUrl: () => 'http://localhost');

  @override
  Future<SubscriptionInfo> getSubscription() async => const SubscriptionInfo(
        planName: 'Free',
        billingPeriod: 'none',
        status: 'active',
        usageToday: 0,
        dailyLimit: 10,
      );

  @override
  Future<List<PaymentMethodKind>> listPaymentMethods() async => const [];
}

FakeBackend fakeFreeBackend() => FakeBackend();

/// `AppleIapService` stub that records `buy`/`restore` calls instead of
/// driving real StoreKit. Calls `super(FakeBackend())` so the real
/// super-ctor listens on the fake platform installed in `setUp`.
class FakeAppleIapService extends AppleIapService {
  FakeAppleIapService() : super(FakeBackend());

  final List<String> bought = [];
  int restoreCalls = 0;

  @override
  Future<void> buy(String productId) async {
    bought.add(productId);
  }

  @override
  Future<void> restore() async {
    restoreCalls++;
  }

  @override
  void dispose() {
    // No-op: nothing real to tear down.
  }
}

/// The default 800x600 test surface is shorter than the full free-plan
/// column (usage card + Buy/Restore section), so `ListView`'s sliver only
/// builds Elements for children within the viewport + cache extent —
/// `find.text` can't see widgets that were never built. Grow the surface so
/// every child in the list is actually built for these assertions.
void _growTestSurface(WidgetTester t) {
  t.view.physicalSize = const Size(800, 2000);
  t.view.devicePixelRatio = 1.0;
  addTearDown(t.view.resetPhysicalSize);
  addTearDown(t.view.resetDevicePixelRatio);
}

void main() {
  setUp(() {
    // Must land before the first `InAppPurchase.instance` access — see
    // file-level comment.
    debugDefaultTargetPlatformOverride = TargetPlatform.fuchsia;
    InAppPurchasePlatform.instance = _FakeIapPlatform();
    PaymentService.debugForceApplePlatform = true;
  });

  tearDown(() {
    PaymentService.debugForceApplePlatform = null;
    debugDefaultTargetPlatformOverride = null;
  });

  testWidgets('iOS free plan shows Buy + Restore, not the web-steering text',
      (t) async {
    _growTestSurface(t);
    await t.pumpWidget(MaterialApp(
      home: SubscriptionScreen(
        backend: fakeFreeBackend(),
        iapService: FakeAppleIapService(),
      ),
    ));
    await t.pumpAndSettle();

    expect(find.textContaining('on the web'), findsNothing); // 2.3.10: steering gone
    expect(find.text('Restore Purchases'), findsOneWidget); // Apple-required
    expect(find.textContaining('Upgrade'), findsWidgets); // buy affordance

    // `debugDefaultTargetPlatformOverride` is a Flutter foundation debug var
    // checked by `debugAssertAllFoundationVarsUnset` right after this test
    // body returns (before package:test's tearDown runs) — reset it here,
    // not just in tearDown, or the framework flags "changed by the test".
    debugDefaultTargetPlatformOverride = null;
  });

  testWidgets('tapping Buy drives AppleIapService.buy', (t) async {
    _growTestSurface(t);
    final iap = FakeAppleIapService();
    await t.pumpWidget(MaterialApp(
      home: SubscriptionScreen(
        backend: fakeFreeBackend(),
        iapService: iap,
      ),
    ));
    await t.pumpAndSettle();

    // The section header ("Upgrade to Pro") and the buy button
    // ("Upgrade to Pro (Monthly)") both contain "Upgrade to Pro", so
    // target the button by type — it's the only plain FilledButton in
    // the free-plan tree.
    await t.tap(find.byType(FilledButton));
    await t.pumpAndSettle();

    expect(iap.bought, [AppleProducts.monthly]); // default billing = monthly
    debugDefaultTargetPlatformOverride = null; // see comment in test 1
  });

  testWidgets('tapping Restore Purchases drives AppleIapService.restore',
      (t) async {
    _growTestSurface(t);
    final iap = FakeAppleIapService();
    await t.pumpWidget(MaterialApp(
      home: SubscriptionScreen(
        backend: fakeFreeBackend(),
        iapService: iap,
      ),
    ));
    await t.pumpAndSettle();

    await t.tap(find.text('Restore Purchases'));
    await t.pumpAndSettle();

    expect(iap.restoreCalls, 1);
    debugDefaultTargetPlatformOverride = null; // see comment in test 1
  });
}
