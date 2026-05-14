import '../models/entity.dart';
import 'entity_extractor/detector.dart';
import 'entity_extractor/email_detector.dart';
import 'entity_extractor/phone_detector.dart';

class EntityExtractor {
  static final List<EntityDetector> _detectors = <EntityDetector>[
    PhoneDetector(),
    EmailDetector(),
  ];

  /// Pure-function entry. Runs every detector, dedupes by (kind, value)
  /// case-insensitive, returns entities sorted by start offset.
  static List<Entity> extract(String text) {
    if (text.trim().isEmpty) return const [];
    final all = <Entity>[];
    for (final d in _detectors) {
      all.addAll(d.detect(text));
    }
    return _dedupe(all)..sort((a, b) => a.start.compareTo(b.start));
  }

  static List<Entity> _dedupe(List<Entity> input) {
    final byKey = <String, Entity>{};
    for (final e in input) {
      final existing = byKey[e.dedupeKey];
      if (existing == null) {
        byKey[e.dedupeKey] = e;
      } else {
        // Higher confidence wins; ties: regex over llm.
        final keepNew = e.confidence > existing.confidence ||
            (e.confidence == existing.confidence &&
                e.source == 'regex' &&
                existing.source != 'regex');
        if (keepNew) byKey[e.dedupeKey] = e;
      }
    }
    return byKey.values.toList();
  }
}
