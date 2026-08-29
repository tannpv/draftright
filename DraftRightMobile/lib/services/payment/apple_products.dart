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

  /// Maps a billing cadence to its App Store product id — the direction
  /// production callers actually need (period the user picked -> id to buy).
  /// Explicit per-value mapping, never a ternary/silent fallthrough at the
  /// call site: an unhandled [BillingPeriod] returns `null` rather than
  /// quietly buying the wrong plan.
  static String? idFor(BillingPeriod period) {
    switch (period) {
      case BillingPeriod.monthly:
        return monthly;
      case BillingPeriod.yearly:
        return yearly;
    }
  }
}
