import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';

void main() {
  group('BillingPeriod wire mapping', () {
    test('wireName matches backend strings', () {
      expect(BillingPeriod.monthly.wireName, 'monthly');
      expect(BillingPeriod.yearly.wireName,  'yearly');
    });

    test('displayName is human-readable', () {
      expect(BillingPeriod.monthly.displayName, 'Monthly');
      expect(BillingPeriod.yearly.displayName,  'Yearly');
    });

    test('fromWire round-trips every value (case-insensitive)', () {
      for (final v in BillingPeriod.values) {
        expect(BillingPeriod.fromWire(v.wireName), v);
        expect(BillingPeriod.fromWire(v.wireName.toUpperCase()), v);
      }
    });

    test('fromWire returns null for unknown / free-plan inputs', () {
      expect(BillingPeriod.fromWire(null), isNull);
      expect(BillingPeriod.fromWire(''), isNull);
      expect(BillingPeriod.fromWire('none'), isNull);
      expect(BillingPeriod.fromWire('quarterly'), isNull);
    });
  });
}
