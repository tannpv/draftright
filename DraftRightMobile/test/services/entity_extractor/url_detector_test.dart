import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor/url_detector.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  final det = UrlDetector();

  test('https with path', () {
    final r = det.detect('Visit https://shop.com/item?id=42 now');
    expect(r.single.kind, EntityKind.url);
    expect(r.single.value, 'https://shop.com/item?id=42');
  });

  test('bare domain with TLD allowed', () {
    final r = det.detect('Web shop.com or foo.vn');
    expect(r.length, 2);
    expect(r.map((e) => e.value), containsAll(['shop.com', 'foo.vn']));
  });

  test('strips trailing punctuation', () {
    final r = det.detect('Go to https://x.com/y. Cool, right?');
    expect(r.single.value, 'https://x.com/y');
  });

  test('rejects bare hello.world (not a real TLD)', () {
    expect(det.detect('hello.world'), isEmpty);
  });
}
