import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';

void main() {
  group('PaymentMethodKind wire mapping', () {
    test('wireName matches backend strings', () {
      expect(PaymentMethodKind.lemonsqueezy.wireName, 'lemonsqueezy');
      expect(PaymentMethodKind.stripe.wireName,       'stripe');
      expect(PaymentMethodKind.vietqr.wireName,       'vietqr');
      expect(PaymentMethodKind.bankTransfer.wireName, 'bank_transfer');
      expect(PaymentMethodKind.paypal.wireName,       'paypal');
    });

    test('fromWire round-trips every known value', () {
      for (final k in PaymentMethodKind.values) {
        expect(PaymentMethodKind.fromWire(k.wireName), k);
      }
    });

    test('fromWire returns null for unknown strings (forward-compat)', () {
      expect(PaymentMethodKind.fromWire('nfc-tap'), isNull);
      expect(PaymentMethodKind.fromWire(''), isNull);
    });
  });

  group('PaymentMethodDescriptor', () {
    test('has a descriptor for every PaymentMethodKind', () {
      for (final k in PaymentMethodKind.values) {
        final d = PaymentMethodDescriptor.forKind(k);
        expect(d, isNotNull, reason: 'Missing descriptor for $k');
        expect(d!.displayName.isNotEmpty, true);
        expect(d.description.isNotEmpty, true);
        expect(d.currencies.isNotEmpty, true);
      }
    });
  });
}
