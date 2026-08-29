import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/payment/apple_products.dart';

void main() {
  test('product ids map to billing periods, one source of truth', () {
    expect(AppleProducts.ids, {AppleProducts.monthly, AppleProducts.yearly});
    expect(AppleProducts.billingFor(AppleProducts.monthly), 'monthly');
    expect(AppleProducts.billingFor(AppleProducts.yearly), 'yearly');
    expect(AppleProducts.billingFor('unknown'), isNull);
  });
}
