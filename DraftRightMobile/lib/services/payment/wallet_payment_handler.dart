import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_stripe/flutter_stripe.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_handler.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/logger_service.dart';

/// Post-checkout UX for native-wallet payments (Apple Pay on iOS,
/// Google Pay on Android).  Both methods produce a Stripe
/// PaymentIntent server-side; this handler presents the native
/// platform sheet via `flutter_stripe` and lets the SDK confirm the
/// intent client-side.  No browser bounce.
///
/// **One handler for both methods (Rule #1):**
/// the only thing that differs between Apple Pay and Google Pay at
/// runtime is which `PlatformPayPaymentType` value the SDK uses.
/// One class with a `kind` constructor parameter keeps the registry
/// trivial — adding a future wallet (Samsung Pay, …) = one more
/// `PaymentMethodKind` case + one branch in [_paymentType].
class WalletPaymentHandler implements PaymentHandler {
  @override
  final PaymentMethodKind kind;

  const WalletPaymentHandler(this.kind)
      : assert(
          kind == PaymentMethodKind.applePay ||
              kind == PaymentMethodKind.googlePay,
          'WalletPaymentHandler only handles applePay or googlePay',
        );

  @override
  Future<void> handle(BuildContext context, CheckoutResult result) async {
    if (result is! WalletCheckout) {
      throw ArgumentError(
        'WalletPaymentHandler.handle expected WalletCheckout but got ${result.runtimeType}',
      );
    }
    if (!_isAllowedOnThisPlatform()) {
      throw Exception('${kind.name} is not available on this platform');
    }

    // The publishable key is set on Stripe once per app run.  Doing
    // it just-in-time (vs at app bootstrap) keeps Stripe out of the
    // launch path for users who never pay.
    DRLogger.log(
      'WalletPaymentHandler.handle: kind=$kind pk=${_redact(result.publishableKey)} mid=${result.merchantIdentifier ?? "—"} country=${result.countryCode} currency=${result.currencyCode}',
      category: 'PaymentService',
    );
    if (result.publishableKey.isEmpty) {
      throw Exception(
        'Backend returned no publishable_key — STRIPE_PUBLISHABLE_KEY '
        'must be set on the backend.  Wallet (Apple/Google Pay) needs it.',
      );
    }
    // The getter `Stripe.publishableKey` THROWS StripeConfigException
    // before it's ever been set, so we can't compare against it to
    // decide whether to re-apply.  Set unconditionally — the setter
    // dedupes internally (no-op when value matches).
    try {
      Stripe.publishableKey = result.publishableKey;
      if (result.merchantIdentifier != null && _isIos) {
        Stripe.merchantIdentifier = result.merchantIdentifier!;
      }
      await Stripe.instance.applySettings();
    } catch (e, st) {
      DRLogger.error('Stripe.applySettings failed: $e\n$st', category: 'PaymentService');
      throw Exception(_explain('applySettings', e));
    }

    // Pre-flight: confirm the device actually supports the native
    // sheet before launching it.  Without this the SDK error is
    // opaque ("StripeConfigException") — the targeted check tells
    // the user whether to set up the wallet or just pick a card.
    final supported = await Stripe.instance.isPlatformPaySupported();
    DRLogger.log('isPlatformPaySupported=$supported', category: 'PaymentService');
    if (!supported) {
      throw Exception(
        kind == PaymentMethodKind.applePay
            ? 'Apple Pay is not set up on this device. Add a card in Wallet first.'
            : 'Google Pay is not set up on this device. Add a payment method in the Google Pay app first.',
      );
    }

    // Native sheet — Apple Pay or Google Pay per `kind`.  Stripe SDK
    // discriminates via the `confirmParams` factory chosen below.
    try {
      await Stripe.instance.confirmPlatformPayPaymentIntent(
        clientSecret: result.clientSecret,
        confirmParams: _confirmParams(result),
      );
    } catch (e, st) {
      DRLogger.error('confirmPlatformPayPaymentIntent failed: $e\n$st', category: 'PaymentService');
      throw Exception(_explain('confirmPlatformPayPaymentIntent', e));
    }
  }

  /// Pull a useful message out of `StripeException` / `StripeError` /
  /// generic exceptions.  The default toString on these is just the
  /// class name, which gives users (and us) nothing to act on.
  String _explain(String stage, Object e) {
    if (e is StripeConfigException) {
      return '$stage (config): ${e.message}';
    }
    if (e is StripeException) {
      final err = e.error;
      return '$stage: ${err.code} — ${err.message ?? err.toString()}';
    }
    if (e is StripeError) {
      return '$stage: ${e.code} — ${e.message ?? e.toString()}';
    }
    return '$stage: $e';
  }

  /// Mask all but the prefix of a Stripe key for log lines.
  String _redact(String key) {
    if (key.length <= 12) return key.isEmpty ? '(empty)' : '${key.substring(0, key.length)}***';
    return '${key.substring(0, 12)}…(${key.length})';
  }

  PlatformPayConfirmParams _confirmParams(WalletCheckout r) {
    switch (kind) {
      case PaymentMethodKind.applePay:
        return PlatformPayConfirmParams.applePay(
          applePay: ApplePayParams(
            merchantCountryCode: r.countryCode,
            currencyCode: r.currencyCode,
            // Apple Pay sheet renders cartItems verbatim — using the
            // PaymentIntent amount alone leaves the sheet showing $0.00.
            // Backend ships display_amount + display_label tailored
            // for the chosen plan + cadence.
            cartItems: [
              ApplePayCartSummaryItem.immediate(
                label: r.displayLabel,
                amount: r.displayAmount,
              ),
            ],
          ),
        );
      case PaymentMethodKind.googlePay:
        return PlatformPayConfirmParams.googlePay(
          googlePay: GooglePayParams(
            merchantCountryCode: r.countryCode,
            currencyCode: r.currencyCode,
            testEnv: !_looksLikeLiveKey(r.publishableKey),
          ),
        );
      default:
        throw StateError('WalletPaymentHandler kind=$kind has no platform params');
    }
  }

  /// Google Pay differentiates test vs live via a flag on the params
  /// (Stripe key alone isn't enough — Google Pay reads the device's
  /// own merchant lookup).  Use the publishable key prefix as a
  /// proxy: `pk_test_…` ⇒ testEnv=true, `pk_live_…` ⇒ false.
  bool _looksLikeLiveKey(String key) => key.startsWith('pk_live_');

  /// True only on the OS this wallet is native to.  Used by both
  /// `handle` (defence-in-depth) and the descriptor filter that
  /// decides which tiles to show on the Subscription screen.
  bool _isAllowedOnThisPlatform() {
    if (kIsWeb) return false;
    try {
      if (kind == PaymentMethodKind.applePay) return Platform.isIOS;
      if (kind == PaymentMethodKind.googlePay) return Platform.isAndroid;
    } catch (_) {/* desktop / unknown — no wallet here */}
    return false;
  }

  bool get _isIos {
    if (kIsWeb) return false;
    try { return Platform.isIOS; } catch (_) { return false; }
  }
}
