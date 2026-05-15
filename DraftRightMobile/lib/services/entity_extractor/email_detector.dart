import '../../models/entity.dart';
import 'detector.dart';

class EmailDetector implements EntityDetector {
  // Standard pragmatic email regex. Local part allows letters/digits/dot/_/%/+/-.
  static final _pattern =
      RegExp(r'\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b');

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    for (final m in _pattern.allMatches(text)) {
      final raw = m.group(0)!;
      // Reject double-@.
      if ('@'.allMatches(raw).length != 1) continue;
      out.add(Entity(
        kind: EntityKind.email,
        value: raw.toLowerCase(),
        display: raw,
        start: m.start,
        end: m.end,
        source: 'regex',
        confidence: 0.98,
      ));
    }
    return out;
  }
}
