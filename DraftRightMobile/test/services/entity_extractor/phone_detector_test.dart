import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/phone_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = PhoneDetector();
  test('VN local 0xx normalizes to +84xx', () {
    final r = det.detect('Gọi 0912 345 678 nhé');
    expect(r, hasLength(1));
    expect(r.single.kind, EntityKind.phone);
    expect(r.single.value, '+84912345678');
    expect(r.single.display, '0912 345 678');
  });

  test('VN +84 international form', () {
    final r = det.detect('Hotline +84 912 345 678');
    expect(r.single.value, '+84912345678');
  });

  test('US international form +1 (415) 555-2671', () {
    final r = det.detect('US +1 (415) 555-2671');
    expect(r.single.value, '+14155552671');
  });

  test('rejects short numbers and 4-digit years', () {
    expect(det.detect('năm 2024 trời ơi'), isEmpty);
    expect(det.detect('code 1234'), isEmpty);
  });

  test('offsets round trip — text.substring(start, end) matches', () {
    final src = 'A phone 0912345678 here';
    final r = det.detect(src).single;
    expect(src.substring(r.start, r.end), '0912345678');
  });
}
