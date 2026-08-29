import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment_service.dart';

void main() {
  group('PaymentService.appleIapAllowed', () {
    test('appleIapAllowed follows the platform, overridable for tests', () {
      PaymentService.debugForceApplePlatform = true;
      expect(PaymentService.appleIapAllowed, isTrue);
      PaymentService.debugForceApplePlatform = false;
      expect(PaymentService.appleIapAllowed, isFalse);
      PaymentService.debugForceApplePlatform = null;   // reset to real platform
      addTearDown(() => PaymentService.debugForceApplePlatform = null);
    });
  });
}
