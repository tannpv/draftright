import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/deep_link_service.dart';

void main() {
  group('DeepLinkService.classify', () {
    test('/payment/success → PaymentReturnEvent success=true, ref empty', () {
      final e = DeepLinkService.classify(Uri.parse('https://draftright.info/payment/success'));
      expect(e, isA<PaymentReturnEvent>());
      final p = e as PaymentReturnEvent;
      expect(p.success, true);
      expect(p.referenceCode, '');
    });

    test('/payment/success?ref=PAY-123 → captures ref', () {
      final e = DeepLinkService.classify(
          Uri.parse('https://draftright.info/payment/success?ref=PAY-123'));
      final p = e as PaymentReturnEvent;
      expect(p.referenceCode, 'PAY-123');
      expect(p.success, true);
    });

    test('/payment/cancel → PaymentReturnEvent success=false', () {
      final e = DeepLinkService.classify(Uri.parse('https://draftright.info/payment/cancel'));
      final p = e as PaymentReturnEvent;
      expect(p.success, false);
    });

    test('unrecognised path → UnknownDeepLink (no crash)', () {
      final e = DeepLinkService.classify(Uri.parse('https://draftright.info/feedback'));
      expect(e, isA<UnknownDeepLink>());
    });

    test('root path → UnknownDeepLink', () {
      final e = DeepLinkService.classify(Uri.parse('https://draftright.info/'));
      expect(e, isA<UnknownDeepLink>());
    });

    test('payment without success/cancel suffix → UnknownDeepLink', () {
      final e = DeepLinkService.classify(Uri.parse('https://draftright.info/payment/refund'));
      expect(e, isA<UnknownDeepLink>());
    });

    test('extra path segments after success are tolerated', () {
      // /payment/success/foo still matches the success arm
      final e = DeepLinkService.classify(
          Uri.parse('https://draftright.info/payment/success/extra?ref=X'));
      expect(e, isA<PaymentReturnEvent>());
      expect((e as PaymentReturnEvent).referenceCode, 'X');
    });
  });
}
