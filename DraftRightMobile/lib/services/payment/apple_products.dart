import 'package:draftright_mobile/services/payment/billing_period.dart';

/// The App Store product identifiers for DraftRight Pro. These MUST equal the
/// backend `apple_product_monthly` / `apple_product_yearly` settings and the
/// products configured in App Store Connect — a drift buys the wrong plan.
/// (Backend-served ids would be the better single source of truth — issue #218
/// — but that's out of the iOS-only client spec.)
class AppleProducts {
  static const String monthly = 'com.draftright.pro.monthly';
  static const String yearly = 'com.draftright.pro.yearly';
  static const Set<String> ids = {monthly, yearly};

  /// Maps a product id to its billing cadence as the typed [BillingPeriod]
  /// enum — never a raw `'monthly'`/`'yearly'` string, so the cadence has one
  /// source of truth (the enum) across purchase, display, and plan-resolution.
  static BillingPeriod? billingFor(String productId) {
    switch (productId) {
      case monthly:
        return BillingPeriod.monthly;
      case yearly:
        return BillingPeriod.yearly;
      default:
        return null;
    }
  }
}
