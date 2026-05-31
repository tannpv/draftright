import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';
import 'package:draftright_mobile/services/payment/widgets/bank_transfer_sheet.dart';
import 'package:draftright_mobile/services/payment/widgets/qr_payment_sheet.dart';

/// Callback supplied to async-confirmation handlers so they can ask
/// the PaymentService to poll `/payment/status/:ref`.  Decoupling
/// handlers from PaymentService keeps them trivially testable.
typedef PaymentStatusWatcher = Stream<PaymentStatusUpdate> Function(String referenceCode);

/// Post-checkout UX for one [PaymentMethodKind].  Implementations own
/// everything that happens after the backend has issued a checkout —
/// opening a browser, showing a QR sheet, copying account info, etc.
///
/// **Why an interface, not a switch in PaymentService:**
/// adding a method (Momo, NFC) = drop a new file in this directory,
/// register it in [PaymentService]'s handler map, done.  No editing
/// existing handlers, no UI branching elsewhere.
abstract class PaymentHandler {
  /// Method this handler knows how to render.  Used as the registry
  /// key in [PaymentService].
  PaymentMethodKind get kind;

  /// Drive the post-checkout flow to completion (UI shown, browser
  /// launched, etc.).  Throws if the result shape isn't compatible
  /// with this handler.
  Future<void> handle(BuildContext context, CheckoutResult result);
}

/// Opens [RedirectCheckout.url] in an in-app browser
/// (SFSafariViewController on iOS, Chrome Custom Tab on Android).
/// Used by every URL-based provider: Lemon Squeezy, Stripe, PayPal.
class RedirectPaymentHandler implements PaymentHandler {
  @override
  final PaymentMethodKind kind;
  const RedirectPaymentHandler(this.kind);

  @override
  Future<void> handle(BuildContext context, CheckoutResult result) async {
    if (result is! RedirectCheckout) {
      throw ArgumentError(
        'RedirectPaymentHandler.handle expected RedirectCheckout but got ${result.runtimeType}',
      );
    }
    final launched = await launchUrl(
      Uri.parse(result.url),
      // inAppBrowserView = SFSafariViewController / Chrome Custom Tab.
      // Both share OS cookies and render Apple Pay / Google Pay
      // automatically.  Apple classifies this as "external browser",
      // so 3.1.1 IAP enforcement does NOT apply.
      mode: LaunchMode.inAppBrowserView,
    );
    if (!launched) {
      throw Exception('Could not open checkout page in browser');
    }
  }
}

/// Shows the VietQR bottom-sheet with a foreground status poller so
/// the user sees "Confirmed ✓" the moment the SePay webhook lands.
class QrPaymentHandler implements PaymentHandler {
  final PaymentStatusWatcher? watcher;
  QrPaymentHandler({this.watcher});

  @override
  PaymentMethodKind get kind => PaymentMethodKind.vietqr;

  @override
  Future<void> handle(BuildContext context, CheckoutResult result) {
    if (result is! QrCheckout) {
      throw ArgumentError(
        'QrPaymentHandler.handle expected QrCheckout but got ${result.runtimeType}',
      );
    }
    final statusStream = watcher?.call(result.referenceCode);
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => QrPaymentSheet(checkout: result, statusStream: statusStream),
    );
  }
}

/// Shows the bank-transfer bottom-sheet with copyable fields and a
/// foreground status poller.
class BankTransferPaymentHandler implements PaymentHandler {
  final PaymentStatusWatcher? watcher;
  BankTransferPaymentHandler({this.watcher});

  @override
  PaymentMethodKind get kind => PaymentMethodKind.bankTransfer;

  @override
  Future<void> handle(BuildContext context, CheckoutResult result) {
    if (result is! BankTransferCheckout) {
      throw ArgumentError(
        'BankTransferPaymentHandler.handle expected BankTransferCheckout but got ${result.runtimeType}',
      );
    }
    final statusStream = watcher?.call(result.referenceCode);
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => BankTransferSheet(checkout: result, statusStream: statusStream),
    );
  }
}
