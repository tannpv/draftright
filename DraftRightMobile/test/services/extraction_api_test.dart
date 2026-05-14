import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/extraction_api.dart';
import 'package:draftright_mobile/models/entity.dart';
import 'package:http/http.dart' as http;

class _FakeClient implements http.Client {
  _FakeClient(this.responder);
  final http.Response Function(http.Request) responder;

  @override
  Future<http.Response> post(Uri url,
      {Map<String, String>? headers,
      Object? body,
      Encoding? encoding}) async {
    final req = http.Request('POST', url)
      ..body = body is String ? body : '';
    return responder(req);
  }

  @override
  void close() {}

  @override
  noSuchMethod(Invocation i) =>
      throw UnsupportedError(i.memberName.toString());
}

void main() {
  test('200 returns parsed entities with source=llm', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response(
            '{"entities":[{"kind":"address","value":"123 Lê Lợi","display":"123 Lê Lợi","start":0,"end":10,"confidence":0.8}],"provider":"openai","tokensUsed":50}',
            200,
            headers: const {'content-type': 'application/json; charset=utf-8'},
          )),
    );
    final out = await api.llmExtract('whatever');
    expect(out.single.kind, EntityKind.address);
    expect(out.single.source, 'llm');
  });

  test('401 throws ExtractionUnavailableException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('unauthorized', 401)),
    );
    expect(api.llmExtract('x'),
        throwsA(isA<ExtractionUnavailableException>()));
  });

  test('402 throws ExtractionQuotaException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('quota', 402)),
    );
    expect(api.llmExtract('x'), throwsA(isA<ExtractionQuotaException>()));
  });

  test('500 throws ExtractionUnavailableException', () async {
    final api = ExtractionApi(
      baseUrl: 'https://api.test',
      tokenProvider: () async => 'jwt',
      httpClient: _FakeClient((_) => http.Response('boom', 500)),
    );
    expect(api.llmExtract('x'),
        throwsA(isA<ExtractionUnavailableException>()));
  });
}
