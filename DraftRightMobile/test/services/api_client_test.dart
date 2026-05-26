// TC: APICLIENT-001 postJson decodes 2xx body + sends bearer
// TC: APICLIENT-002 non-2xx throws ApiException with parsed message + status
// TC: APICLIENT-003 getJson, empty body, non-JSON error fallback
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/services/api_client.dart';

void main() {
  http.Client mock(int Function(http.Request) code, String Function(http.Request) body, {void Function(http.Request)? capture}) =>
      _Mock((req) async {
        capture?.call(req);
        return http.StreamedResponse(Stream.value(utf8.encode(body(req))), code(req), request: req);
      });

  test('postJson sends bearer + JSON body and decodes the response', () async {
    http.Request? seen;
    final api = ApiClient(baseUrl: 'http://h', client: mock((_) => 200, (_) => '{"ok":true}', capture: (r) => seen = r));
    final res = await api.postJson('/x', body: {'a': 1}, token: 'TKN');
    expect(res['ok'], true);
    expect(seen!.headers['authorization'], 'Bearer TKN');
    expect(seen!.headers['content-type'], contains('application/json'));
    expect(jsonDecode(seen!.body)['a'], 1);
  });

  test('throws ApiException with parsed message + status on non-2xx', () async {
    final api = ApiClient(baseUrl: 'http://h', client: mock((_) => 400, (_) => '{"message":"bad email"}'));
    await expectLater(
      api.postJson('/x', body: {}),
      throwsA(isA<ApiException>()
          .having((e) => e.statusCode, 'statusCode', 400)
          .having((e) => e.message, 'message', 'bad email')),
    );
  });

  test('joins array messages; falls back to HTTP <code> for non-JSON', () async {
    final api1 = ApiClient(baseUrl: 'http://h', client: mock((_) => 422, (_) => '{"message":["a","b"]}'));
    await expectLater(api1.postJson('/x'), throwsA(isA<ApiException>().having((e) => e.message, 'm', 'a, b')));
    final api2 = ApiClient(baseUrl: 'http://h', client: mock((_) => 500, (_) => 'boom'));
    await expectLater(api2.getJson('/x'), throwsA(isA<ApiException>().having((e) => e.message, 'm', 'HTTP 500')));
  });

  test('empty 2xx body returns empty map', () async {
    final api = ApiClient(baseUrl: 'http://h', client: mock((_) => 200, (_) => ''));
    expect(await api.getJson('/x'), <String, dynamic>{});
  });
}

class _Mock extends http.BaseClient {
  final Future<http.StreamedResponse> Function(http.Request) handler;
  _Mock(this.handler);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) => handler(request as http.Request);
}
