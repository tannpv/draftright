// Tests the ErrorReporter flush queue semantics (routed through ApiClient,
// #205 #9): 2xx → sent+removed, non-2xx → dropped (no infinite retry),
// network/timeout → kept for retry. Uses the debugHttpClient test seam.
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'package:draftright_mobile/services/error_reporter.dart';

class _Mock extends http.BaseClient {
  final Future<http.StreamedResponse> Function(http.Request) h;
  _Mock(this.h);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest r) =>
      h(r as http.Request);
}

http.Client status(int code) => _Mock((r) async =>
    http.StreamedResponse(Stream.value(utf8.encode('{}')), code, request: r));

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    ErrorReporter.resetForTest();
    ErrorReporter.setBackendUrl('http://h');
  });
  tearDown(ErrorReporter.resetForTest);

  test('2xx removes the entry (sent)', () async {
    ErrorReporter.debugHttpClient = status(200);
    ErrorReporter.seedQueueForTest([
      {'m': 'a'}
    ]);
    await ErrorReporter.flushForTest();
    expect(ErrorReporter.queueForTest, isEmpty);
  });

  test('non-2xx drops the entry (no infinite retry)', () async {
    ErrorReporter.debugHttpClient = status(400);
    ErrorReporter.seedQueueForTest([
      {'m': 'a'}
    ]);
    await ErrorReporter.flushForTest();
    expect(ErrorReporter.queueForTest, isEmpty);
  });

  test('network error keeps the entry for retry', () async {
    ErrorReporter.debugHttpClient = _Mock((_) async => throw Exception('down'));
    ErrorReporter.seedQueueForTest([
      {'m': 'a'}
    ]);
    await ErrorReporter.flushForTest();
    expect(ErrorReporter.queueForTest, hasLength(1));
  });

  test('mixed batch: sent + dropped removed, network kept', () async {
    var i = 0;
    ErrorReporter.debugHttpClient = _Mock((r) async {
      final n = i++;
      if (n == 2) throw Exception('down');
      return http.StreamedResponse(
          Stream.value(utf8.encode('{}')), n == 0 ? 200 : 400,
          request: r);
    });
    ErrorReporter.seedQueueForTest([
      {'m': '0'},
      {'m': '1'},
      {'m': '2'},
    ]);
    await ErrorReporter.flushForTest();
    final q = ErrorReporter.queueForTest;
    expect(q, hasLength(1));
    expect(q.first['m'], '2');
  });
}
