// Direct unit tests for the small shared HTTP helpers extracted during the
// #205 Rule #1 cleanup. They underpin every backend call (base-URL normalize)
// and every error message (server-error parse), so pin their edge cases.
import 'package:flutter_test/flutter_test.dart';

import 'package:draftright_mobile/services/api_client.dart'
    show parseServerErrorMessage;
import 'package:draftright_mobile/services/app_source.dart';
import 'package:draftright_mobile/services/url_util.dart';

void main() {
  group('normalizeBackendUrl', () {
    test('strips one or many trailing slashes', () {
      expect(normalizeBackendUrl('http://h/'), 'http://h');
      expect(normalizeBackendUrl('http://h///'), 'http://h');
      expect(normalizeBackendUrl('http://h/path/'), 'http://h/path');
    });
    test('leaves a clean URL and empty string untouched', () {
      expect(normalizeBackendUrl('http://h'), 'http://h');
      expect(normalizeBackendUrl(''), '');
    });
  });

  test('detectAppSource returns a backend-accepted value on a non-mobile host',
      () {
    // The flutter_test host is neither iOS nor Android → the safe fallback.
    expect(detectAppSource(), appSourceAndroid);
    expect([appSourceIOS, appSourceAndroid], contains(detectAppSource()));
  });

  group('parseServerErrorMessage', () {
    test('extracts a string message', () {
      expect(parseServerErrorMessage('{"message":"bad email"}'), 'bad email');
    });
    test('joins a class-validator array message', () {
      expect(parseServerErrorMessage('{"message":["a","b"]}'), 'a, b');
    });
    test('falls back to the Go {error} shape', () {
      expect(parseServerErrorMessage('{"error":"boom"}'), 'boom');
    });
    test('returns null for no message, empty message, or non-JSON', () {
      expect(parseServerErrorMessage('{}'), isNull);
      expect(parseServerErrorMessage('{"message":""}'), isNull);
      expect(parseServerErrorMessage('not json'), isNull);
    });
  });
}
