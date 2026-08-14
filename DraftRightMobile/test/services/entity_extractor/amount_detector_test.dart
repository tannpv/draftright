import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:draftright_mobile/services/entity_extractor/amount_detector.dart';

void main() {
  final d = AmountDetector();

  List<Entity> find(String t) => d.detect(t);

  test('VN dong grouped amount', () {
    final r = find('Chuyển 1.500.000đ giúp mình nhé');
    expect(r, hasLength(1));
    expect(r.first.kind, EntityKind.amount);
    expect(r.first.value, '1500000');
    expect(r.first.display, '1.500.000đ');
  });

  test('VND code suffix', () {
    final r = find('Số tiền 500 VND');
    expect(r.single.value, '500');
  });

  test('USD prefix symbol', () {
    final r = find(r'Total is $100 today');
    expect(r.single.value, '100');
  });

  test('USD with cents keeps decimal', () {
    final r = find(r'Pay 1,500.00 USD');
    expect(r.single.value, '1500.00');
  });

  test('EUR prefix with EU grouping', () {
    final r = find('Giá €1.500,50 thôi');
    expect(r.single.value, '1500.50');
  });

  test('bare number with no currency is NOT an amount', () {
    expect(find('call me at 1500000 tomorrow'), isEmpty);
    expect(find('0912345678'), isEmpty);
  });

  group('normalizeAmount', () {
    test('all-thousands grouping → integer', () {
      expect(normalizeAmount('1.500.000'), '1500000');
      expect(normalizeAmount('1,500,000'), '1500000');
    });
    test('1-2 digit tail → decimal', () {
      expect(normalizeAmount('1,500.00'), '1500.00');
      expect(normalizeAmount('1.500,50'), '1500.50');
    });
    test('plain digits unchanged', () {
      expect(normalizeAmount('1500000'), '1500000');
      expect(normalizeAmount('500'), '500');
    });
  });
}
