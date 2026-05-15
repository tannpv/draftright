import '../../models/entity.dart';
import 'detector.dart';

class OtpDetector implements EntityDetector {
  static final _triggerPattern = RegExp(
    r'(otp|m[ãa]\s*(x[áa]c\s*minh)?|verification|code|m[ậa]t\s*kh[ẩa]u|password)',
    caseSensitive: false,
  );
  static final _digitPattern = RegExp(r'\b\d{4,8}\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _digitPattern.allMatches(text)) {
      // Within 20 chars before the digits, look for a trigger.
      final windowStart = (m.start - 20).clamp(0, text.length);
      final window = text.substring(windowStart, m.start);
      if (!_triggerPattern.hasMatch(window)) continue;
      out.add(Entity(
        kind: EntityKind.otp,
        value: m.group(0)!,
        display: m.group(0)!,
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.9,
      ));
    }
    return out;
  }
}
