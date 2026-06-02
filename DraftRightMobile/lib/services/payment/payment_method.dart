/// Identity for a payment method advertised by the backend.
///
/// Mirrors `backend/src/payment/entities/payment.entity.ts` `PaymentMethod`
/// enum.  Adding a new method = add a value here + map its wire name +
/// (optionally) override the default handler.  Nothing else in the
/// mobile app should branch on the wire string directly.
enum PaymentMethodKind {
  lemonsqueezy,
  stripe,
  vietqr,
  bankTransfer,
  paypal,
  applePay,
  googlePay;

  /// String the backend uses on the wire.  Single source of truth.
  String get wireName {
    switch (this) {
      case PaymentMethodKind.lemonsqueezy:  return 'lemonsqueezy';
      case PaymentMethodKind.stripe:        return 'stripe';
      case PaymentMethodKind.vietqr:        return 'vietqr';
      case PaymentMethodKind.bankTransfer:  return 'bank_transfer';
      case PaymentMethodKind.paypal:        return 'paypal';
      case PaymentMethodKind.applePay:      return 'apple_pay';
      case PaymentMethodKind.googlePay:     return 'google_pay';
    }
  }

  /// Reverse mapping. Returns null for unknown strings so the catalog
  /// gracefully ignores methods this client doesn't yet implement
  /// (forward-compatible with backend additions).
  static PaymentMethodKind? fromWire(String value) {
    for (final k in PaymentMethodKind.values) {
      if (k.wireName == value) return k;
    }
    return null;
  }
}

/// One payment method advertised by `GET /payment/methods`.
///
/// The endpoint currently returns a list of wire-name strings; this
/// class wraps them with display metadata so the UI doesn't have to
/// branch on strings.  Display name + currency hints are mobile-side
/// for now (one place to localize later).
class PaymentMethodDescriptor {
  final PaymentMethodKind kind;
  final String displayName;
  final String description;
  final List<String> currencies;

  const PaymentMethodDescriptor({
    required this.kind,
    required this.displayName,
    required this.description,
    required this.currencies,
  });

  static PaymentMethodDescriptor? forKind(PaymentMethodKind kind) {
    switch (kind) {
      case PaymentMethodKind.lemonsqueezy:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.lemonsqueezy,
          displayName: 'Credit / Debit Card',
          description: 'Visa, Mastercard, Apple Pay, Google Pay (via Lemon Squeezy)',
          currencies: ['USD'],
        );
      case PaymentMethodKind.stripe:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.stripe,
          displayName: 'Stripe',
          description: 'Credit card via Stripe',
          currencies: ['USD'],
        );
      case PaymentMethodKind.vietqr:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.vietqr,
          displayName: 'VietQR (scan to pay)',
          description: 'Scan with any Vietnamese banking app — auto-confirms',
          currencies: ['VND'],
        );
      case PaymentMethodKind.bankTransfer:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.bankTransfer,
          displayName: 'Bank Transfer',
          description: 'Manual transfer with reference code',
          currencies: ['VND'],
        );
      case PaymentMethodKind.paypal:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.paypal,
          displayName: 'PayPal',
          description: 'Pay with PayPal balance or card',
          currencies: ['USD'],
        );
      case PaymentMethodKind.applePay:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.applePay,
          displayName: 'Apple Pay',
          description: 'Pay with Face ID, Touch ID, or your Apple ID password',
          currencies: ['USD'],
        );
      case PaymentMethodKind.googlePay:
        return const PaymentMethodDescriptor(
          kind: PaymentMethodKind.googlePay,
          displayName: 'Google Pay',
          description: 'Pay instantly with your saved Google Pay cards',
          currencies: ['USD'],
        );
    }
  }
}
