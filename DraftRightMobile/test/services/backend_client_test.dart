// Unit tests for BackendClient — the core rewrite/subscription API client.
// Injects a mock http.Client (flows to the internal ApiClient) and a fake
// AuthService for the bearer token. Previously had no coverage.
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/api_client.dart' show ApiException;
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';

class _FakeAuth extends AuthService {
  final bool refresh;
  _FakeAuth({this.refresh = false});
  @override
  Future<String> getAccessToken() async => 'tok';
  @override
  Future<bool> tryRefresh() async => refresh;
}

class _Mock extends http.BaseClient {
  final Future<http.StreamedResponse> Function(http.Request) handler;
  _Mock(this.handler);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) =>
      handler(request as http.Request);
}

http.Client mock(int status, String body,
        {void Function(http.Request)? capture}) =>
    _Mock((req) async {
      capture?.call(req);
      return http.StreamedResponse(Stream.value(utf8.encode(body)), status,
          request: req);
    });

void main() {
  BackendClient build(http.Client c, {AuthService? auth}) => BackendClient(
        auth: auth ?? _FakeAuth(),
        getBaseUrl: () => 'http://h',
        httpClient: c,
      );

  test('rewrite posts tone+text with bearer and parses rewritten_text',
      () async {
    http.Request? seen;
    final client = build(mock(
      200,
      '{"rewritten_text":"Hello.","usage_today":3,"daily_limit":10}',
      capture: (r) => seen = r,
    ));
    final res = await client.rewrite(text: 'helo', tone: Tone.polished);
    expect(res.rewrittenText, 'Hello.');
    expect(res.usageToday, 3);
    expect(res.dailyLimit, 10);
    expect(res.isGrammarCheck, false);
    expect(seen!.url.toString(), 'http://h/rewrite');
    expect(seen!.headers['authorization'], 'Bearer tok');
    final body = jsonDecode(seen!.body) as Map<String, dynamic>;
    expect(body['tone'], Tone.polished.apiValue);
    expect(body['text'], 'helo');
  });

  test('rewrite parses the grammar_check envelope', () async {
    final client = build(mock(
      200,
      '{"grammar":{"score":80,"issues":[{"type":"spelling","offset":0,'
      '"length":4,"original":"helo","suggestion":"hello","reason":"x"}]},'
      '"usage_today":1,"daily_limit":10}',
    ));
    final res = await client.rewrite(text: 'helo', tone: Tone.grammarCheck);
    expect(res.isGrammarCheck, true);
    expect(res.grammarResult!.score, 80);
    expect(res.grammarResult!.issues.length, 1);
    expect(res.rewrittenText, '');
  });

  test('rewrite rethrows ApiException on non-2xx', () async {
    final client = build(mock(402, '{"message":"quota exceeded"}'));
    await expectLater(
      client.rewrite(text: 'x', tone: Tone.simple),
      throwsA(isA<ApiException>()),
    );
  });

  test('rewrite refreshes the token and retries once on 401', () async {
    var calls = 0;
    final client = BackendClient(
      auth: _FakeAuth(refresh: true),
      getBaseUrl: () => 'http://h',
      httpClient: _Mock((req) async {
        calls++;
        final is401 = calls == 1;
        final body = is401
            ? '{"message":"expired"}'
            : '{"rewritten_text":"Hi.","usage_today":1,"daily_limit":10}';
        return http.StreamedResponse(
            Stream.value(utf8.encode(body)), is401 ? 401 : 200,
            request: req);
      }),
    );
    final res = await client.rewrite(text: 'x', tone: Tone.simple);
    expect(res.rewrittenText, 'Hi.');
    expect(calls, 2); // first 401, then the refreshed retry
  });

  test('getSubscription parses plan + status', () async {
    final client = build(mock(
      200,
      '{"plan":{"name":"Pro","billing_period":"month","daily_limit":100},'
      '"status":"active","usage_today":5}',
    ));
    final sub = await client.getSubscription();
    expect(sub.planName, 'Pro');
    expect(sub.status, 'active');
    expect(sub.dailyLimit, 100);
    expect(sub.isFree, false);
  });
}
