import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:draftright_mobile/services/extraction_cache.dart';

Entity _e(String v) => Entity(
    kind: EntityKind.phone, value: v, display: v, start: 0, end: v.length,
    source: 'llm', confidence: 1.0);

void main() {
  test('miss then hit; key is trimmed', () {
    final c = ExtractionCache();
    expect(c.get('hello'), isNull);
    c.set('hello', [_e('a')]);
    expect(c.get('hello')!.single.value, 'a');
    expect(c.get('  hello  ')!.single.value, 'a'); // trimmed key
  });

  test('get returns a copy (caller cannot mutate the cached list)', () {
    final c = ExtractionCache();
    c.set('t', [_e('a')]);
    c.get('t')!.clear();
    expect(c.get('t'), hasLength(1)); // unaffected
  });

  test('FIFO eviction past maxEntries', () {
    final c = ExtractionCache(maxEntries: 2);
    c.set('a', [_e('1')]);
    c.set('b', [_e('2')]);
    c.set('c', [_e('3')]); // evicts 'a'
    expect(c.get('a'), isNull);
    expect(c.get('b'), isNotNull);
    expect(c.get('c'), isNotNull);
  });

  test('clear empties the cache', () {
    final c = ExtractionCache();
    c.set('a', [_e('1')]);
    c.clear();
    expect(c.get('a'), isNull);
  });
}
