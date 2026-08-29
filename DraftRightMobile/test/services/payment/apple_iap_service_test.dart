// Unit tests for AppleIapService — the StoreKit purchase/restore driver that
// redeems each purchased/restored transaction's StoreKit 2 JWS with the
// backend (BackendClient.redeemAppleTransaction) and only finishes the
// transaction (completePurchase) after a successful grant. A redeem failure
// must NOT complete the transaction, so StoreKit keeps re-delivering it on
// purchaseStream until a retry succeeds.
//
// Test seam: `InAppPurchase.instance`'s methods all delegate to
// `InAppPurchasePlatform.instance` (a PlatformInterface). We swap that
// singleton for a fake — the upstream-idiomatic seam, matching the
// in_app_purchase package's own test suite (in_app_purchase_test.dart) —
// rather than parameterizing AppleIapService on the platform (InAppPurchase
// has a private constructor and can't be subclassed).
//
// `debugDefaultTargetPlatformOverride` is pinned to a platform that isn't
// android/iOS/macOS (fuchsia) before the first `InAppPurchase.instance`
// access: on those platforms `InAppPurchase._getOrCreateInstance()`
// unconditionally calls `InAppPurchaseAndroidPlatform.registerPlatform()` /
// `InAppPurchaseStoreKitPlatform.registerPlatform()`, which overwrites
// `InAppPurchasePlatform.instance` with the real platform implementation and
// clobbers our fake. This test host runs macOS, so without the override the
// very first test in this file would silently swap the fake out from under
// itself. Same guard the plugin's own test uses.

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:in_app_purchase_platform_interface/in_app_purchase_platform_interface.dart';
import 'package:plugin_platform_interface/plugin_platform_interface.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/payment/apple_iap_service.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';

/// Fake `InAppPurchasePlatform` — the upstream-idiomatic test seam.
/// `MockPlatformInterfaceMixin` bypasses the token check that normally stops
/// a `PlatformInterface` from being subclassed outside its own package.
class FakeIapPlatform extends InAppPurchasePlatform
    with MockPlatformInterfaceMixin {
  final _controller = StreamController<List<PurchaseDetails>>.broadcast();
  final List<PurchaseDetails> completed = [];
  List<ProductDetails> productsToReturn = const [];

  @override
  Stream<List<PurchaseDetails>> get purchaseStream => _controller.stream;

  @override
  Future<bool> isAvailable() async => true;

  @override
  Future<ProductDetailsResponse> queryProductDetails(
    Set<String> identifiers,
  ) async =>
      ProductDetailsResponse(
        productDetails: productsToReturn,
        notFoundIDs: const [],
      );

  @override
  Future<bool> buyNonConsumable({required PurchaseParam purchaseParam}) async =>
      true;

  @override
  Future<void> completePurchase(PurchaseDetails purchase) async {
    completed.add(purchase);
  }

  @override
  Future<void> restorePurchases({String? applicationUserName}) async {}

  /// Pushes a purchase update onto the stream `AppleIapService` listens to.
  void emit(PurchaseDetails detail) => _controller.add([detail]);

  void closeStream() => _controller.close();
}

/// `BackendClient` with `redeemAppleTransaction` overridden so tests never
/// touch the network. Mirrors the `_FakeBackend extends BackendClient`
/// pattern used in payment_plan_resolution_test.dart.
class FakeBackend extends BackendClient {
  FakeBackend() : super(auth: AuthService(), getBaseUrl: () => 'http://localhost');

  final List<String> redeemed = [];
  bool failRedeem = false;

  @override
  Future<void> redeemAppleTransaction(String signedTransaction) async {
    redeemed.add(signedTransaction);
    if (failRedeem) {
      throw Exception('redeem failed');
    }
  }
}

PurchaseDetails purchased(String jws, String productId) => PurchaseDetails(
      productID: productId,
      verificationData: PurchaseVerificationData(
        localVerificationData: '',
        serverVerificationData: jws,
        source: 'app_store',
      ),
      transactionDate: null,
      status: PurchaseStatus.purchased,
    )..pendingCompletePurchase = true;

void main() {
  setUp(() {
    debugDefaultTargetPlatformOverride = TargetPlatform.fuchsia;
  });

  tearDown(() {
    debugDefaultTargetPlatformOverride = null;
  });

  test(
      'purchased detail -> redeem then completePurchase, entitlement callback fires',
      () async {
    final fake = FakeIapPlatform();
    InAppPurchasePlatform.instance = fake;
    final backend = FakeBackend();
    var changed = 0;
    final svc = AppleIapService(backend, onEntitlementChanged: () => changed++);

    fake.emit(purchased('signed-jws', AppleProducts.monthly));
    await pumpEventQueue();

    expect(backend.redeemed, ['signed-jws']);
    expect(fake.completed, hasLength(1));
    expect(changed, 1);
    svc.dispose();
  });

  test('redeem failure does NOT complete the transaction (so it retries)',
      () async {
    final fake = FakeIapPlatform();
    InAppPurchasePlatform.instance = fake;
    final backend = FakeBackend()..failRedeem = true;
    final svc = AppleIapService(backend);

    fake.emit(purchased('signed-jws', AppleProducts.monthly));
    await pumpEventQueue();

    expect(backend.redeemed, ['signed-jws']); // attempted
    expect(fake.completed, isEmpty); // NOT finished -> retries
    svc.dispose();
  });
}
