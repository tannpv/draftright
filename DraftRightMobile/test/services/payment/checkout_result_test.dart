import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';

void main() {
  group('CheckoutResult.fromJson dispatches on response shape', () {
    test('redirect_url field → RedirectCheckout', () {
      final r = CheckoutResult.fromJson({
        'payment': {'reference_code': 'PAY-ABC'},
        'redirect_url': 'https://example.com/checkout/abc',
      });
      expect(r, isA<RedirectCheckout>());
      expect((r as RedirectCheckout).url, 'https://example.com/checkout/abc');
      expect(r.referenceCode, 'PAY-ABC');
    });

    test('qr_data field → QrCheckout (with optional bank_info)', () {
      final r = CheckoutResult.fromJson({
        'payment': {'reference_code': 'PAY-QR1'},
        'qr_data': 'https://img.vietqr.io/image/MB-12345-compact.jpg',
        'bank_info': {
          'bank_name': 'MB Bank',
          'account_number': '12345',
          'account_name': 'DRAFTRIGHT',
          'amount': 99000,
          'currency': 'VND',
          'reference': 'PAY-QR1',
        },
      });
      expect(r, isA<QrCheckout>());
      final qr = r as QrCheckout;
      expect(qr.qrImageUrl, contains('vietqr.io'));
      expect(qr.bankInfo, isNotNull);
      expect(qr.bankInfo!.accountNumber, '12345');
    });

    test('bank_info only → BankTransferCheckout', () {
      final r = CheckoutResult.fromJson({
        'payment': {'reference_code': 'PAY-BT'},
        'bank_info': {
          'bank_name': 'ACB',
          'account_number': '99999',
          'account_name': 'DRAFTRIGHT',
          'amount': 99000,
          'currency': 'VND',
          'reference': 'PAY-BT',
        },
      });
      expect(r, isA<BankTransferCheckout>());
      expect((r as BankTransferCheckout).info.bankName, 'ACB');
    });

    test('reference_code falls back to top-level field if payment block missing', () {
      final r = CheckoutResult.fromJson({
        'reference_code': 'TOP-REF',
        'redirect_url': 'https://example.com/x',
      });
      expect(r.referenceCode, 'TOP-REF');
    });

    test('redirect_url wins over qr_data when both present', () {
      // Backend can't return both today, but the dispatcher's priority
      // is deterministic — pin it here so future strategies don't
      // accidentally regress to the wrong handler.
      final r = CheckoutResult.fromJson({
        'redirect_url': 'https://example.com/x',
        'qr_data': 'https://example.com/qr.jpg',
      });
      expect(r, isA<RedirectCheckout>());
    });

    test('response with none of the three fields throws FormatException', () {
      expect(
        () => CheckoutResult.fromJson({'payment': {'reference_code': 'X'}}),
        throwsFormatException,
      );
    });
  });
}
