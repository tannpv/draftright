import '../../models/entity.dart';
import 'detector.dart';

/// Currency amounts — multi-locale. A number is only treated as money when it
/// sits next to a currency marker (symbol or code), which keeps phone/date/
/// account numbers out. **Extend to a new locale by adding a row to
/// [_markers]** — no new detector class needed. (Format-precise money is a
/// locale table; fuzzy multilingual entities like address/name stay with the
/// LLM `/extract` fallback.)
class AmountDetector implements EntityDetector {
  // Currency markers that may sit BEFORE or AFTER the number. Lowercased for
  // matching. VN first (the primary case): đ / ₫ / VND / đồng.
  static const _markers = <String>[
    '₫', 'đ', 'vnd', 'vnđ', 'đồng',
    r'$', 'usd', r'us$',
    '€', 'eur',
    '£', 'gbp',
    '¥', 'jpy', 'yen',
  ];

  // A grouped or plain number: 1.500.000 / 1,500.00 / 1500000 / 500.
  static const _num = r'\d[\d.,]*\d|\d';

  static final RegExp _pattern = () {
    final markers = _markers.map(RegExp.escape).join('|');
    // number then marker  (100đ, 1.500.000 VND)  OR  marker then number ($100).
    // Boundary is `(?![A-Za-z])` not `\b`: Dart's `\b` is ASCII-only, so a `\b`
    // after a Unicode marker (đ, ₫, €, £, ¥) never matches — which silently
    // dropped the primary VN case `1.500.000đ`. The lookahead/lookbehind stop a
    // code marker matching inside a word (e.g. "eur" in "europe").
    return RegExp(
      '(?:($_num)\\s?(?:$markers)(?![A-Za-z]))|(?:(?<![A-Za-z])(?:$markers)\\s?($_num))',
      caseSensitive: false,
    );
  }();

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _pattern.allMatches(text)) {
      final numStr = (m.group(1) ?? m.group(2))!;
      final value = normalizeAmount(numStr);
      if (value.isEmpty) continue;
      out.add(Entity(
        kind: EntityKind.amount,
        value: value,
        display: m.group(0)!.trim(),
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.9,
      ));
    }
    return out;
  }
}

/// Normalize a written number to a plain machine number: strip thousands
/// separators, keep a decimal part. Locale-agnostic heuristic — the LAST
/// separator followed by exactly 3 digits is a thousands group (→ integer);
/// followed by 1–2 digits it is the decimal point.
///   1.500.000 → 1500000 · 1,500.00 → 1500.00 · 1.500,50 → 1500.50 · 500 → 500
String normalizeAmount(String numStr) {
  final sep = RegExp(r'[.,]');
  if (!sep.hasMatch(numStr)) return numStr;
  final lastSep = numStr.lastIndexOf(sep);
  final tail = numStr.substring(lastSep + 1);
  if (tail.length == 3) {
    // all-thousands grouping → integer
    return numStr.replaceAll(sep, '');
  }
  final head = numStr.substring(0, lastSep).replaceAll(sep, '');
  return '$head.$tail';
}
