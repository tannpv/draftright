import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/logger_service.dart';

/// Coordinates the "Upgrade to Pro" flow.
///
/// Pattern (intentionally minimal):
///   1. Fetch /plans, pick the first Pro-tier plan (paid, monthly).
///   2. POST /payment/checkout with the plan id + method=lemonsqueezy.
///   3. Launch the returned URL in `LaunchMode.inAppBrowserView`, which
///      maps to SFSafariViewController on iOS and Chrome Custom Tab on
///      Android — both share OS browser cookies + support Apple Pay /
///      Google Pay automatically. No webview embedded.
///
/// The caller (SubscriptionScreen) is responsible for refreshing
/// subscription state when the user returns to the app — the
/// payment service has no opinion on that; webhook activation may
/// land before OR after the user comes back.
class PaymentService {
  final BackendClient backend;

  PaymentService(this.backend);

  /// Returns the hosted-checkout URL for upgrading the current user to
  /// Pro via Lemon Squeezy.  Throws on any backend error so the caller
  /// can surface a SnackBar.
  Future<String> createProCheckoutUrl() async {
    final plans = await backend.listPlans();
    final pro = _pickProPlan(plans);
    if (pro == null) {
      throw Exception('Could not find a Pro plan in the catalog');
    }
    final planId = (pro['id'] ?? '').toString();
    if (planId.isEmpty) {
      throw Exception('Pro plan row is missing an id');
    }
    return backend.createCheckout(planId: planId);
  }

  /// Launches the Pro upgrade flow.  Returns true when the OS reported
  /// the launch succeeded (browser opened); false otherwise so the UI
  /// can fall back to a "visit draftright.info" message.
  Future<bool> launchUpgrade() async {
    try {
      final url = await createProCheckoutUrl();
      final uri = Uri.parse(url);
      // inAppBrowserView = SFSafariViewController on iOS, Chrome
      // Custom Tab on Android. Both inherit OS browser cookies, both
      // render Apple Pay (iOS) / Google Pay (Android) automatically.
      // Apple's review classifies this as "external browser" so the
      // 3.1.1 IAP rule doesn't bite.
      final launched = await launchUrl(
        uri,
        mode: LaunchMode.inAppBrowserView,
      );
      DRLogger.log('Upgrade checkout launched: $launched', category: 'PaymentService');
      return launched;
    } catch (e, st) {
      DRLogger.error('Upgrade launch failed: $e', category: 'PaymentService');
      if (kDebugMode) debugPrint(st.toString());
      rethrow;
    }
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
