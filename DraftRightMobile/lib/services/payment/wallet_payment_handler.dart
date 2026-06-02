import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_stripe/flutter_stripe.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_handler.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';

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
    if (Stripe.publishableKey != result.publishableKey) {
      Stripe.publishableKey = result.publishableKey;
      if (result.merchantIdentifier != null && _isIos) {
        Stripe.merchantIdentifier = result.merchantIdentifier!;
      }
      await Stripe.instance.applySettings();
    }

    // Native sheet — Apple Pay or Google Pay per `kind`.  Stripe SDK
    // discriminates via the `confirmParams` factory chosen below.
    await Stripe.instance.confirmPlatformPayPaymentIntent(
      clientSecret: result.clientSecret,
      confirmParams: _confirmParams(result),
    );
  }

  PlatformPayConfirmParams _confirmParams(WalletCheckout r) {
    switch (kind) {
      case PaymentMethodKind.applePay:
        return PlatformPayConfirmParams.applePay(
          applePay: ApplePayParams(
            merchantCountryCode: r.countryCode,
            currencyCode: r.currencyCode,
            // SDK uses PaymentIntent amount; this list is for UI
            // line-items only.  Single "DraftRight Pro" line keeps
            // the sheet clean.
            cartItems: const [
              ApplePayCartSummaryItem.immediate(
                label: 'DraftRight Pro',
                amount: '0',
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
