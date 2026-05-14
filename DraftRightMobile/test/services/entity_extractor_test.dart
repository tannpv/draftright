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

  group('EntityExtractor — integration', () {
    test('mixed message: phone + email + url + bank + OTP', () {
      const text =
          'Vietcombank 0123456789 — gọi 0912 345 678, email tan@x.com, '
          'web shop.com, OTP 482917';
      final out = EntityExtractor.extract(text);
      final kinds = out.map((e) => e.kind).toSet();
      expect(kinds, containsAll([
        EntityKind.bankAccount,
        EntityKind.phone,
        EntityKind.email,
        EntityKind.url,
        EntityKind.otp,
      ]));
    });

    test('sorted ascending by start offset', () {
      const text = 'tan@x.com then 0912345678';
      final out = EntityExtractor.extract(text);
      expect(out.first.kind, EntityKind.email);
      expect(out.last.kind, EntityKind.phone);
    });

    test('every entity offset round-trips', () {
      const text =
          'Call 0912345678 or +84912345678. Email tan@x.com. Web shop.com.';
      for (final e in EntityExtractor.extract(text)) {
        expect(text.substring(e.start, e.end).contains(e.display), isTrue,
            reason: 'display "${e.display}" not found at ${e.start}..${e.end}');
      }
    });
  });
}
