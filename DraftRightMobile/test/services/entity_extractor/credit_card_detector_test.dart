import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/credit_card_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = CreditCardDetector();
  test('valid Visa test card passes Luhn', () {
    final r = det.detect('Card 4242 4242 4242 4242 expires 12/27');
    expect(r.single.kind, EntityKind.creditCard);
    expect(r.single.value, '4242424242424242');
    expect(r.single.display, '**** **** **** 4242');
    expect(r.single.meta['masked'], 'true');
  });

  test('invalid Luhn rejected', () {
    expect(det.detect('Bad card 1234 5678 9012 3456'), isEmpty);
  });

  test('dashes accepted', () {
    final r = det.detect('Try 4242-4242-4242-4242 here');
    expect(r.single.value, '4242424242424242');
  });
}
