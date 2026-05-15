import '../../models/entity.dart';
import 'detector.dart';

class UrlDetector implements EntityDetector {
  static const _tlds = <String>{
    'com', 'vn', 'net', 'org', 'io', 'co', 'app', 'me', 'info',
    'biz', 'asia', 'tv', 'gg', 'ai', 'dev', 'xyz',
  };

  static final _httpPattern = RegExp(r'https?://\S+', caseSensitive: false);
  static final _barePattern = RegExp(
    r'\b(?:[a-z0-9\-]+\.)+([a-z]{2,6})(?:/\S*)?\b',
    caseSensitive: false,
  );

  @override
  List<Entity> detect(String text) {
    final out = <Entity>[];
    final consumed = <int>{};
    for (final m in _httpPattern.allMatches(text)) {
      final raw = _stripTrailingPunct(m.group(0)!);
      out.add(Entity(
        kind: EntityKind.url,
        value: raw,
        display: raw,
        start: m.start,
        end: m.start + raw.length,
        source: 'regex',
        confidence: 0.98,
      ));
      for (var i = m.start; i < m.start + raw.length; i++) {
        consumed.add(i);
      }
    }
    for (final m in _barePattern.allMatches(text)) {
      if (consumed.contains(m.start)) continue;
      final tld = m.group(1)!.toLowerCase();
      if (!_tlds.contains(tld)) continue;
      final raw = _stripTrailingPunct(m.group(0)!);
      out.add(Entity(
        kind: EntityKind.url,
        value: raw,
        display: raw,
        start: m.start,
        end: m.start + raw.length,
        source: 'regex',
        confidence: 0.85,
      ));
    }
    return out;
  }

  String _stripTrailingPunct(String s) {
    var end = s.length;
    while (end > 0 && '.,!?;:)'.contains(s[end - 1])) {
      end--;
    }
    return s.substring(0, end);
  }
}
