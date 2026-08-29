/// The App Store product identifiers for DraftRight Pro. These MUST equal the
/// backend `apple_product_monthly` / `apple_product_yearly` settings and the
/// products configured in App Store Connect — a drift buys the wrong plan.
/// (Backend-served ids would be the better single source of truth — issue #218
/// — but that's out of the iOS-only client spec.)
class AppleProducts {
  static const String monthly = 'com.draftright.pro.monthly';
  static const String yearly = 'com.draftright.pro.yearly';
  static const Set<String> ids = {monthly, yearly};

  static String? billingFor(String productId) {
    switch (productId) {
      case monthly:
        return 'monthly';
      case yearly:
        return 'yearly';
      default:
        return null;
    }
  }
}
