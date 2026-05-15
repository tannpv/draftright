import '../../models/entity.dart';
import 'detector.dart';

class CreditCardDetector implements EntityDetector {
  static final _pattern = RegExp(r'\b(?:\d[\s\-]?){12,18}\d\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _pattern.allMatches(text)) {
      final raw = m.group(0)!;
      final digits = raw.replaceAll(RegExp(r'[\s\-]'), '');
      if (digits.length < 13 || digits.length > 19) continue;
      if (!_luhn(digits)) continue;
      final last4 = digits.substring(digits.length - 4);
      out.add(Entity(
        kind: EntityKind.creditCard,
        value: digits,
        display: '**** **** **** $last4',
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.99,
        meta: const {'masked': 'true'},
      ));
    }
    return out;
  }

  static bool _luhn(String digits) {
    var sum = 0;
    var doubleIt = false;
    for (var i = digits.length - 1; i >= 0; i--) {
      var d = int.parse(digits[i]);
      if (doubleIt) {
        d *= 2;
        if (d > 9) d -= 9;
      }
      sum += d;
      doubleIt = !doubleIt;
    }
    return sum % 10 == 0;
  }
}
