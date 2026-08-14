import '../models/entity.dart';

/// Max distinct messages cached per session. Named const (RULE #1) — mirrors
/// the desktop client RewriteCache's bounded-size design.
const int kExtractionCacheMaxEntries = 50;

/// In-memory, session-scoped cache of LLM `/extract` results, keyed on the
/// trimmed message text. Bounded with oldest-out (FIFO) eviction.
///
/// Why client-side + text-only key: the offline regex layer is already instant
/// so needs no cache; this only spares a repeat `/extract` LLM round-trip on the
/// same message. Extraction output is a function of the text alone (not the
/// user), and the cache lives per-device, so no per-user key is required —
/// unlike the rewrite cache, which is keyed per user for quota/personalization.
class ExtractionCache {
  ExtractionCache({this.maxEntries = kExtractionCacheMaxEntries});

  final int maxEntries;
  final Map<String, List<Entity>> _map = {};
  final List<String> _order = []; // insertion order for FIFO eviction

  /// Cached entities for [text], or null on a miss. Returns a copy so callers
  /// can't mutate the cached list.
  List<Entity>? get(String text) {
    final v = _map[_key(text)];
    return v == null ? null : List.of(v);
  }

  /// Store [entities] for [text], evicting the oldest entries past [maxEntries].
  void set(String text, List<Entity> entities) {
    final k = _key(text);
    if (!_map.containsKey(k)) _order.add(k);
    _map[k] = List.of(entities);
    while (_order.length > maxEntries) {
      _map.remove(_order.removeAt(0));
    }
  }

  void clear() {
    _map.clear();
    _order.clear();
  }

  static String _key(String text) => text.trim();
}
