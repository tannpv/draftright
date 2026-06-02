import 'dart:async';
import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';
import 'package:draftright_mobile/services/payment/payment_handler.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';
import 'package:draftright_mobile/services/payment/payment_status.dart';
import 'package:draftright_mobile/services/payment/wallet_payment_handler.dart';

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
  }) : _handlers = {} {
    // Default wiring: async-confirmation handlers receive
    // `watchPayment` so they can show live status inside the sheet.
    // Tests override by passing [handlers].
    _handlers.addAll(handlers ??
        {
          PaymentMethodKind.lemonsqueezy: const RedirectPaymentHandler(PaymentMethodKind.lemonsqueezy),
          PaymentMethodKind.stripe:       const RedirectPaymentHandler(PaymentMethodKind.stripe),
          PaymentMethodKind.paypal:       const RedirectPaymentHandler(PaymentMethodKind.paypal),
          PaymentMethodKind.vietqr:       QrPaymentHandler(watcher: watchPayment),
          PaymentMethodKind.bankTransfer: BankTransferPaymentHandler(watcher: watchPayment),
          // Native-wallet handlers — one shared class, two registry
          // entries.  Platform-gating in listAvailableMethods drops
          // each entry from the picker on platforms that can't run
          // its wallet (Apple Pay on Android, Google Pay on iOS).
          PaymentMethodKind.applePay:     const WalletPaymentHandler(PaymentMethodKind.applePay),
          PaymentMethodKind.googlePay:    const WalletPaymentHandler(PaymentMethodKind.googlePay),
        });
  }

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
    // Hosted Stripe is blocked on iOS by App Store Guideline 3.1.1
    // (no external-browser path through Stripe direct for digital
    // subscriptions in iOS App Store builds).
    if (kind == PaymentMethodKind.stripe && _isIos) return false;
    // Native wallets are only available on the platform they're
    // native to.  Apple Pay = iOS only; Google Pay = Android only.
    // Both are processed by Stripe under the hood and surface
    // through `flutter_stripe`.
    if (kind == PaymentMethodKind.applePay && !_isIos) return false;
    if (kind == PaymentMethodKind.googlePay && !_isAndroid) return false;
    return true;
  }

  bool get _isIos {
    if (kIsWeb) return false;
    try { return Platform.isIOS; } catch (_) { return false; }
  }

  bool get _isAndroid {
    if (kIsWeb) return false;
    try { return Platform.isAndroid; } catch (_) { return false; }
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

  /// Foreground poller for async-confirmation methods (VietQR,
  /// bank-transfer).  Hits `/payment/status/:ref` every [interval]
  /// until the status is terminal (completed / failed / expired /
  /// refunded), [timeout] elapses, or the stream subscription is
  /// cancelled (sheet dismissed).
  ///
  /// Yields a [PaymentStatusUpdate] for every poll — UI can render
  /// "Waiting…" while pending and switch to "Confirmed ✓" once the
  /// backend webhook lands.  Transient errors are logged but don't
  /// terminate the stream; we just keep polling.
  Stream<PaymentStatusUpdate> watchPayment(
    String referenceCode, {
    Duration interval = const Duration(seconds: 3),
    Duration timeout = const Duration(minutes: 15),
  }) async* {
    final deadline = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(deadline)) {
      try {
        final raw = await backend.getPaymentStatus(referenceCode);
        final update = PaymentStatusUpdate.fromJson(raw);
        yield update;
        if (update.status.isTerminal) return;
      } catch (e) {
        DRLogger.warn('Payment status poll failed for $referenceCode: $e',
            category: 'PaymentService');
      }
      await Future<void>.delayed(interval);
    }
    // Deadline reached without a terminal status — emit a synthetic
    // expired update so the UI can show "Took too long, try again".
    yield PaymentStatusUpdate(
      referenceCode: referenceCode,
      status: PaymentStatus.expired,
    );
  }

  /// Open the Lemon Squeezy Customer Portal so the user can cancel,
  /// change plan, or update their card.  The backend mints a signed
  /// URL per request.
  ///
  /// **Why iOS uses the system browser (not in-app) for THIS flow:**
  /// SFSafariViewController has an isolated cookie jar — every launch
  /// is a fresh LS session, so LS prompts for the magic-link login
  /// every single time.  Chrome Custom Tabs on Android share cookies
  /// with the system Chrome browser, so the portal opens directly if
  /// the user has ever logged into LS in Chrome.
  ///
  /// To match that "no-login-prompt" UX on iOS we send portal opens
  /// to system Safari (`LaunchMode.externalApplication`), where the
  /// LS session persists across launches.  Checkout stays in-app
  /// because (a) checkout is a one-shot — no recurring cookies
  /// needed, and (b) keeping the user inside the app for the most
  /// commercially sensitive flow is the right call.
  ///
  /// Throws if the user has no LS subscription, the backend isn't
  /// configured, or the browser refuses to launch.  Callers should
  /// surface the error in a SnackBar.
  Future<void> openCustomerPortal() async {
    final url = await backend.getCustomerPortalUrl();
    final mode = _isIos
        ? LaunchMode.externalApplication
        : LaunchMode.inAppBrowserView;
    final launched = await launchUrl(Uri.parse(url), mode: mode);
    if (!launched) {
      throw Exception('Could not open the customer portal');
    }
  }

  /// Cancel the user's currently-active subscription in-app via the
  /// backend (which calls LS / Stripe APIs directly — no portal trip).
  ///
  /// The user keeps Pro access until the existing renewal date; the
  /// provider's cancellation webhook flips
  /// `subscriptions.status='cancelled'` shortly after this returns.
  /// Callers should refresh `/subscription` once this completes to
  /// pick up the new status.
  Future<CancelSubscriptionResult> cancelSubscription() async {
    return backend.cancelSubscription();
  }

  /// Currency the strategy expects to charge the plan in.  VietQR +
  /// bank-transfer can only settle in VND because the QR code is a
  /// Vietnamese-bank-only spec; everything else defaults to USD.
  String _currencyFor(PaymentMethodKind method) {
    switch (method) {
      case PaymentMethodKind.vietqr:
      case PaymentMethodKind.bankTransfer:
        return 'VND';
      case PaymentMethodKind.lemonsqueezy:
      case PaymentMethodKind.stripe:
      case PaymentMethodKind.paypal:
      case PaymentMethodKind.applePay:
      case PaymentMethodKind.googlePay:
        return 'USD';
    }
  }

  /// Fetch the public plan catalog and return the Pro-tier plan id
  /// the upgrade button should target for [method] at the requested
  /// [billingPeriod] cadence.
  ///
  ///   - Currency-aware so VietQR doesn't pick a USD plan (which
  ///     would bake "499 đồng" into the QR — useless).
  ///   - Cadence-aware so the yearly toggle on the subscription
  ///     screen actually charges the yearly variant.
  ///
  /// Single source of truth so the UI doesn't carry plan-picking
  /// logic.
  Future<String> resolveProPlanId({
    PaymentMethodKind? method,
    BillingPeriod? billingPeriod,
  }) async {
    final plans = await backend.listPlans();
    final currency = method != null ? _currencyFor(method) : null;
    final pro = _pickProPlan(plans, currency: currency, billingPeriod: billingPeriod);
    if (pro == null) {
      final cadence = billingPeriod?.wireName ?? 'any';
      throw Exception(
        currency != null
            ? 'Could not find a Pro plan in $currency ($cadence) for $method'
            : 'Could not find a Pro plan ($cadence) in the catalog',
      );
    }
    final planId = (pro['id'] ?? '').toString();
    if (planId.isEmpty) throw Exception('Pro plan row is missing an id');
    return planId;
  }

  /// Pick the first plan that looks like the paid Pro tier:
  ///   - billing_period != 'none' (excludes the Free plan)
  ///   - is_active = true (excludes archived rows)
  ///   - currency matches when provided (VND for VietQR/bank,
  ///     USD for Stripe/LS/PayPal)
  ///   - cadence matches [billingPeriod] when provided; otherwise
  ///     prefers 'monthly' (legacy default) then falls back to the
  ///     first paid plan.
  Map<String, dynamic>? _pickProPlan(
    List<Map<String, dynamic>> plans, {
    String? currency,
    BillingPeriod? billingPeriod,
  }) {
    final paid = plans.where((p) {
      final bp = (p['billing_period'] ?? '').toString().toLowerCase();
      final active = p['is_active'] ?? true;
      if (bp == 'none' || active != true) return false;
      if (currency != null) {
        final c = (p['currency'] ?? '').toString().toUpperCase();
        if (c != currency.toUpperCase()) return false;
      }
      return true;
    }).toList();
    if (paid.isEmpty) return null;
    if (billingPeriod != null) {
      for (final p in paid) {
        if (BillingPeriod.fromWire(p['billing_period']?.toString()) == billingPeriod) {
          return p;
        }
      }
      // Requested cadence not in the catalog — fall through to the
      // monthly-default fallback so the UI doesn't crash on a
      // partially-configured backend.
    }
    return paid.firstWhere(
      (p) => BillingPeriod.fromWire(p['billing_period']?.toString()) == BillingPeriod.monthly,
      orElse: () => paid.first,
    );
  }
}
