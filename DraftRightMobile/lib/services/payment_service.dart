import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/payment/payment_handler.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';

/// Orchestrates upgrade-to-Pro across every payment method the backend
/// advertises.
///
/// **Why a service + handler map instead of a switch:**
///   - Mirrors backend strategy pattern (one strategy per method).
///   - UI calls one method (`upgradeWith`) — never branches on method
///     itself.
///   - Adding Momo, NFC, or PayPal = create `MomoHandler`, drop into
///     the map.  No edits anywhere else.
///   - Handlers are injectable so widget tests stub them.
class PaymentService {
  final BackendClient backend;
  final Map<PaymentMethodKind, PaymentHandler> _handlers;

  PaymentService(
    this.backend, {
    Map<PaymentMethodKind, PaymentHandler>? handlers,
  }) : _handlers = handlers ?? _defaultHandlers();

  /// Default handler wiring — one entry per [PaymentMethodKind] this
  /// client understands.  Tests can pass [handlers] to override.
  static Map<PaymentMethodKind, PaymentHandler> _defaultHandlers() => {
        PaymentMethodKind.lemonsqueezy: const RedirectPaymentHandler(PaymentMethodKind.lemonsqueezy),
        PaymentMethodKind.stripe:       const RedirectPaymentHandler(PaymentMethodKind.stripe),
        PaymentMethodKind.paypal:       const RedirectPaymentHandler(PaymentMethodKind.paypal),
        PaymentMethodKind.vietqr:       QrPaymentHandler(),
        PaymentMethodKind.bankTransfer: BankTransferPaymentHandler(),
      };

  /// Backend-enabled methods filtered for platform-policy reasons.
  ///
  /// Apple App Store Guideline 3.1.1 forbids charging for digital
  /// goods through non-IAP rails INSIDE the iOS app.  Lemon Squeezy
  /// (Merchant-of-Record) and external-browser launches are accepted;
  /// direct Stripe checkout is not, so we drop it on iOS.  Android +
  /// other platforms show everything the backend enables.
  Future<List<PaymentMethodKind>> listAvailableMethods() async {
    final raw = await backend.listPaymentMethods();
    return raw.where(_isAllowedOnThisPlatform).toList();
  }

  bool _isAllowedOnThisPlatform(PaymentMethodKind kind) {
    if (kind == PaymentMethodKind.stripe && _isIos) return false;
    return true;
  }

  bool get _isIos {
    if (kIsWeb) return false;
    try { return Platform.isIOS; } catch (_) { return false; }
  }

  /// Run the full upgrade flow for [method]: create the checkout
  /// server-side, then dispatch to the handler registered for that
  /// kind.  Throws on any backend / handler error; UI surfaces a
  /// SnackBar.
  Future<void> upgradeWith({
    required BuildContext context,
    required String planId,
    required PaymentMethodKind method,
  }) async {
    final handler = _handlers[method];
    if (handler == null) {
      throw Exception('No handler registered for $method on this platform');
    }
    final result = await backend.createCheckout(planId: planId, method: method);
    if (!context.mounted) return;
    DRLogger.log('Checkout created: method=$method ref=${result.referenceCode}',
        category: 'PaymentService');
    await handler.handle(context, result);
  }

  /// Fetch the public plan catalog and return the Pro-tier plan id
  /// the upgrade button should target.  Single source of truth so the
  /// UI doesn't carry plan-picking logic.
  Future<String> resolveProPlanId() async {
    final plans = await backend.listPlans();
    final pro = _pickProPlan(plans);
    if (pro == null) throw Exception('Could not find a Pro plan in the catalog');
    final planId = (pro['id'] ?? '').toString();
    if (planId.isEmpty) throw Exception('Pro plan row is missing an id');
    return planId;
  }

  /// Pick the first plan that looks like the paid Pro tier:
  ///   - billing_period != 'none' (excludes the Free plan)
  ///   - is_active = true (excludes archived rows)
  ///   - prefers 'monthly' over 'yearly' for the upgrade button
  ///
  /// One-place rule so future "Pro Yearly" toggles only edit here.
  Map<String, dynamic>? _pickProPlan(List<Map<String, dynamic>> plans) {
    final paid = plans.where((p) {
      final bp = (p['billing_period'] ?? '').toString().toLowerCase();
      final active = p['is_active'] ?? true;
      return bp != 'none' && active == true;
    }).toList();
    if (paid.isEmpty) return null;
    final monthly = paid.firstWhere(
      (p) => (p['billing_period'] ?? '').toString().toLowerCase() == 'monthly',
      orElse: () => paid.first,
    );
    return monthly;
  }
}
