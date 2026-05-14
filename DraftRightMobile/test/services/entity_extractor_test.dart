import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/entity_extractor.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  group('EntityExtractor — shell', () {
    test('empty input → empty list', () {
      expect(EntityExtractor.extract(''), isEmpty);
    });

    test('whitespace only → empty list', () {
      expect(EntityExtractor.extract('   \n  \t'), isEmpty);
    });

    test('dedupe collapses (kind, value) duplicates', () {
      // Two phones in different formats but same E.164 normalized value
      // should collapse to one. Will be filled in by phone detector task.
      final out = EntityExtractor.extract('Call 0912345678 or +84912345678');
      final phones = out.where((e) => e.kind == EntityKind.phone).toList();
      expect(phones.length, 1, reason: 'normalized phones must dedupe');
    });
  });
}
