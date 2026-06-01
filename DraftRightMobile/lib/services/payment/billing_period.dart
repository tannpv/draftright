/// Billing cadence supported by the backend `plans.billing_period`
/// column.  Kept in a typed enum so widgets + services pass the same
/// value everywhere — no raw `'monthly'` / `'yearly'` strings leak
/// into the UI or storage layers.
///
/// When the backend adds a new cadence (e.g. quarterly) add a value
/// here and update `BillingPeriodSelector` to render its label.  No
/// other file branches on the cadence string.
enum BillingPeriod {
  monthly('monthly', 'Monthly'),
  yearly('yearly', 'Yearly');

  /// Lowercase identifier matching `plans.billing_period` in the API
  /// payload.  Use when filtering plans by cadence.
  final String wireName;

  /// User-facing English label (translated by the UI layer if
  /// localisation is wired later).
  final String displayName;

  const BillingPeriod(this.wireName, this.displayName);

  /// Parse a wire value (case-insensitive).  Returns null for unknown
  /// or `'none'` (the Free plan) so callers can decide their fallback.
  static BillingPeriod? fromWire(String? raw) {
    if (raw == null) return null;
    final norm = raw.toLowerCase();
    for (final v in BillingPeriod.values) {
      if (v.wireName == norm) return v;
    }
    return null;
  }
}
