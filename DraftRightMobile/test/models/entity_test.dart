import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';

void main() {
  group('Entity', () {
    test('dedupeKey collapses case-insensitive duplicates regardless of offsets/source', () {
      final a = Entity(
        kind: EntityKind.email,
        value: 'TAN@X.COM',
        display: 'tan@x.com',
        start: 0,
        end: 9,
        source: 'regex',
        confidence: 1.0,
      );
      final b = Entity(
        kind: EntityKind.email,
        value: 'tan@x.com',
        display: 'TAN@X.COM',
        start: 100,
        end: 109,
        source: 'llm',
        confidence: 0.7,
      );
      expect(a.dedupeKey, b.dedupeKey);
    });

    test('toJson / fromJson round trip', () {
      final e = Entity(
        kind: EntityKind.bankAccount,
        value: '0123456789',
        display: 'Vietcombank · 0123456789',
        start: 10,
        end: 20,
        source: 'regex',
        confidence: 0.95,
        meta: const {'bank': 'Vietcombank'},
      );
      final json = e.toJson();
      final round = Entity.fromJson(json);
      expect(round.kind, EntityKind.bankAccount);
      expect(round.value, '0123456789');
      expect(round.meta['bank'], 'Vietcombank');
      expect(round.start, 10);
      expect(round.end, 20);
      expect(round.source, 'regex');
      expect(round.confidence, 0.95);
    });
  });
}
