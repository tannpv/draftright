/// Typed view of the JSON returned by `POST /payment/checkout`.
///
/// The backend's `CheckoutResult` (see
/// `backend/src/payment/strategies/payment-strategy.interface.ts`) is a
/// union of three shapes:
///   - `redirect_url` for hosted-checkout providers (Stripe, LS, PayPal)
///   - `qr_data`     for VietQR (URL to an img.vietqr.io image)
///   - `bank_info`   for manual bank transfer
///
/// We model each shape as a separate subclass of [CheckoutResult].  The
/// UI dispatches on `is RedirectCheckout` / `is QrCheckout` / etc, so
/// new shapes (e.g. NFC tap-to-pay) plug in without changing existing
/// code.
sealed class CheckoutResult {
  /// All shapes carry the backend-assigned reference code so the UI
  /// can poll `/payment/status/:ref` for async confirmation.
  final String referenceCode;
  const CheckoutResult({required this.referenceCode});

  /// Build the right subclass from the raw `/payment/checkout` body.
  /// Field priority matches the backend: a redirect URL wins over a
  /// QR (for providers that send both), bank info is the fallback.
  factory CheckoutResult.fromJson(Map<String, dynamic> json) {
    final ref = ((json['payment'] is Map ? json['payment']['reference_code'] : null) ??
            json['reference_code'] ??
            '')
        .toString();
    // Wallet branch wins over redirect/qr/bank — it's only set by the
    // backend for apple_pay / google_pay checkouts.  See backend
    // StripeStrategy.createWalletPaymentIntent.
    final walletRaw = json['wallet_intent'];
    if (walletRaw is Map<String, dynamic>) {
      return WalletCheckout(
        referenceCode: ref,
        clientSecret: (walletRaw['client_secret'] ?? '').toString(),
        publishableKey: (walletRaw['publishable_key'] ?? '').toString(),
        merchantIdentifier: walletRaw['merchant_identifier']?.toString(),
        countryCode: (walletRaw['country_code'] ?? 'US').toString(),
        currencyCode: (walletRaw['currency_code'] ?? 'USD').toString(),
      );
    }
    final redirect = json['redirect_url'];
    if (redirect is String && redirect.isNotEmpty) {
      return RedirectCheckout(referenceCode: ref, url: redirect);
    }
    final qrData = json['qr_data'];
    final bankInfoRaw = json['bank_info'];
    if (qrData is String && qrData.isNotEmpty) {
      return QrCheckout(
        referenceCode: ref,
        qrImageUrl: qrData,
        bankInfo: bankInfoRaw is Map<String, dynamic>
            ? BankInfo.fromJson(bankInfoRaw)
            : null,
      );
    }
    if (bankInfoRaw is Map<String, dynamic>) {
      return BankTransferCheckout(
        referenceCode: ref,
        info: BankInfo.fromJson(bankInfoRaw),
      );
    }
    throw const FormatException(
      'Backend returned a checkout response with none of redirect_url / qr_data / bank_info / wallet_intent',
    );
  }
}

/// Native-wallet checkout — used by Apple Pay (iOS) + Google Pay
/// (Android).  Carries the Stripe PaymentIntent client_secret the
/// SDK confirms client-side, plus merchant + locale context the
/// platform sheet needs to render.
class WalletCheckout extends CheckoutResult {
  /// Stripe PaymentIntent client_secret.  `pi_..._secret_...`.
  final String clientSecret;
  /// Stripe publishable key (`pk_test_...` / `pk_live_...`).
  final String publishableKey;
  /// Apple Pay merchant ID; null on Android.
  final String? merchantIdentifier;
  /// ISO-3166 two-letter merchant country code.
  final String countryCode;
  /// ISO-4217 three-letter currency code.
  final String currencyCode;

  const WalletCheckout({
    required super.referenceCode,
    required this.clientSecret,
    required this.publishableKey,
    required this.countryCode,
    required this.currencyCode,
    this.merchantIdentifier,
  });
}

class RedirectCheckout extends CheckoutResult {
  final String url;
  const RedirectCheckout({required super.referenceCode, required this.url});
}

class QrCheckout extends CheckoutResult {
  /// URL to a QR-code image (e.g. img.vietqr.io); render with
  /// `Image.network(qrImageUrl)`.
  final String qrImageUrl;
  /// Optional account details shown alongside the QR — VietQR returns
  /// both so users who can't scan can still transfer manually.
  final BankInfo? bankInfo;

  const QrCheckout({
    required super.referenceCode,
    required this.qrImageUrl,
    this.bankInfo,
  });
}

class BankTransferCheckout extends CheckoutResult {
  final BankInfo info;
  const BankTransferCheckout({
    required super.referenceCode,
    required this.info,
  });
}

/// Plain DTO for the bank-info block.  Used by both [BankTransferCheckout]
/// and (optionally) [QrCheckout].
class BankInfo {
  final String bankName;
  final String accountNumber;
  final String accountName;
  final num amount;
  final String currency;
  final String reference;

  const BankInfo({
    required this.bankName,
    required this.accountNumber,
    required this.accountName,
    required this.amount,
    required this.currency,
    required this.reference,
  });

  factory BankInfo.fromJson(Map<String, dynamic> j) => BankInfo(
        bankName:      (j['bank_name'] ?? '').toString(),
        accountNumber: (j['account_number'] ?? '').toString(),
        accountName:   (j['account_name'] ?? '').toString(),
        amount:        (j['amount'] as num?) ?? 0,
        currency:      (j['currency'] ?? 'VND').toString(),
        reference:     (j['reference'] ?? '').toString(),
      );
}
