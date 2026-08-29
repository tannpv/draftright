import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';
import 'package:draftright_mobile/services/payment/billing_period.dart';

void main() {
  test('product ids map to billing periods, one source of truth', () {
    expect(AppleProducts.ids, {AppleProducts.monthly, AppleProducts.yearly});
    expect(AppleProducts.billingFor(AppleProducts.monthly), BillingPeriod.monthly);
    expect(AppleProducts.billingFor(AppleProducts.yearly), BillingPeriod.yearly);
    expect(AppleProducts.billingFor('unknown'), isNull);
  });
}
