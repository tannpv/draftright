import '../../models/entity.dart';
import 'detector.dart';

class PhoneDetector implements EntityDetector {
  // VN local: 0 followed by 9-10 digits (with optional spaces/dashes between)
  // VN intl:  +84 followed by 9-10 digits
  // Generic intl: +<country 1-3 digits> <body 6-13 digits>
  static final _patterns = <RegExp>[
    // +country then digits; allow spaces, dashes, parens
    RegExp(r'\+\d{1,3}[\s\-]?\(?\d{1,4}\)?[\s\-]?\d{3,4}[\s\-]?\d{3,4}\b'),
    // VN local 0 prefix
    RegExp(r'(?<![\d+])0\d{2,3}[\s\-]?\d{3}[\s\-]?\d{3,4}\b'),
  ];

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    final seenStarts = <int>{};
    for (final p in _patterns) {
      for (final m in p.allMatches(text)) {
        if (seenStarts.contains(m.start)) continue;
        seenStarts.add(m.start);
        final raw = m.group(0)!;
        final digits = raw.replaceAll(RegExp(r'[\s\-\(\)]'), '');
        final normalized = _toE164(digits);
        if (normalized == null) continue;
        out.add(Entity(
          kind: EntityKind.phone,
          value: normalized,
          display: raw.trim(),
          start: m.start,
          end: m.end,
          source: 'regex',
          confidence: 0.95,
        ));
      }
    }
    return out;
  }

  String? _toE164(String digits) {
    if (digits.startsWith('+')) {
      // Already international; ensure length sane.
      if (digits.length < 8 || digits.length > 16) return null;
      return digits;
    }
    if (digits.startsWith('0') && digits.length >= 10 && digits.length <= 11) {
      return '+84${digits.substring(1)}';
    }
    return null;
  }
}
