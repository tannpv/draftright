import 'dart:async';

import 'package:in_app_purchase/in_app_purchase.dart';

import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/error_reporter.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';

/// Drives the StoreKit purchase/restore flow and redeems the resulting
/// StoreKit 2 JWS with the backend, which is the source of truth for
/// granting Pro. `InAppPurchase.instance` is used unconditionally rather
/// than injected — `InAppPurchase` has a private constructor and can't be
/// subclassed, so tests swap the plugin's own test seam instead
/// (`InAppPurchasePlatform.instance`, a `PlatformInterface`).
///
/// A transaction is only finished (`completePurchase`) after the backend
/// confirms the grant. If redemption fails, the transaction is left
/// unfinished on purpose: StoreKit re-delivers unfinished transactions on
/// [purchaseStream] on the next app session (or immediately, per the
/// plugin), so the retry is "for free" and no purchase is silently lost.
class AppleIapService {
  final BackendClient _backend;
  final void Function()? _onEntitlementChanged;
  final InAppPurchase _iap = InAppPurchase.instance; // delegates to InAppPurchasePlatform.instance (the test seam)
  StreamSubscription<List<PurchaseDetails>>? _sub;

  AppleIapService(this._backend, {void Function()? onEntitlementChanged})
      : _onEntitlementChanged = onEntitlementChanged {
    _sub = _iap.purchaseStream.listen(_onPurchases, onError: (e, st) {
      DRLogger.error('iap stream error: $e\n$st', category: 'PaymentService');
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
            await _iap.completePurchase(d); // ONLY after a successful grant
          }
          _onEntitlementChanged?.call();
        } catch (e, st) {
          // Fully handled here: log + report, and DON'T complete → the txn stays
          // pending and retries. Do NOT rethrow — this is a stream onData callback,
          // so a thrown error becomes an unhandled (fatal) async error.
          DRLogger.error('apple redeem failed: $e\n$st', category: 'PaymentService');
          ErrorReporter.reportHandled(e, stack: st, severity: 'warning', context: {'stage': 'apple_redeem'});
        }
      } else if (d.status == PurchaseStatus.error) {
        DRLogger.error('apple purchase error: ${d.error}', category: 'PaymentService');
      }
      // canceled/pending: nothing to do.
    }
  }

  void dispose() => _sub?.cancel();
}
