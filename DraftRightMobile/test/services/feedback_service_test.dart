// TC: FEEDBACK-001
// TC: FEEDBACK-002
// TC: FEEDBACK-003
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/services/feedback_service.dart';

void main() {
  test('submitFeatureRequest posts JSON kind=feature with the picked platform',
      () async {
    http.Request? captured;
    final client = _MockClient((req) async {
      captured = req as http.Request;
      return http.Response(
        jsonEncode({'id': 'feat-1', 'message': 'ok'}),
        201,
        headers: {'content-type': 'application/json'},
      );
    });

    final ok = await FeedbackService.submitFeatureRequest(
      title: 'Dark mode',
      targetPlatform: 'linux',
      description: 'please follow system theme',
      authToken: 'tok-123',
      endpointOverride: 'http://localhost:9/feedback',
      httpClient: client,
    );

    expect(ok, isTrue);
    expect(captured!.url.toString(), 'http://localhost:9/feedback');
    expect(captured!.headers['authorization'], 'Bearer tok-123');
    final body = jsonDecode(captured!.body) as Map<String, dynamic>;
    expect(body['kind'], 'feature');
    expect(body['title'], 'Dark mode');
    expect(body['target_platform'], 'linux');
    expect(body['description'], 'please follow system theme');
    expect(body['source'], anyOf('ios-app', 'android-app'));
    expect(body.containsKey('user_email'), isFalse);
  });

  test('submitFeatureRequest returns false on a non-2xx response', () async {
    final client = _MockClient(
      (req) async => http.Response('bad', 400),
    );
    final ok = await FeedbackService.submitFeatureRequest(
      title: 'X',
      targetPlatform: 'mac',
      description: 'd',
      endpointOverride: 'http://localhost:9/feedback',
      httpClient: client,
    );
    expect(ok, isFalse);
  });

  test('submitFeatureRequest includes user_email only when no auth token',
      () async {
    Map<String, dynamic>? body;
    final client = _MockClient((req) async {
      body = jsonDecode((req as http.Request).body) as Map<String, dynamic>;
      return http.Response(jsonEncode({'id': 'x'}), 201);
    });

    // Without auth token — user_email should appear.
    await FeedbackService.submitFeatureRequest(
      title: 'X',
      targetPlatform: 'mac',
      description: 'd',
      userEmail: 'a@b.c',
      endpointOverride: 'http://localhost:9/feedback',
      httpClient: client,
    );
    expect(body!.containsKey('user_email'), isTrue);

    // With auth token — user_email must NOT appear even if supplied.
    body = null;
    await FeedbackService.submitFeatureRequest(
      title: 'X',
      targetPlatform: 'mac',
      description: 'd',
      userEmail: 'a@b.c',
      authToken: 'tok',
      endpointOverride: 'http://localhost:9/feedback',
      httpClient: client,
    );
    expect(body!.containsKey('user_email'), isFalse);
  });
}

/// Minimal http.BaseClient delegating to a handler — no real socket.
class _MockClient extends http.BaseClient {
  final Future<http.Response> Function(http.BaseRequest) handler;
  _MockClient(this.handler);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final r = await handler(request);
    return http.StreamedResponse(
      Stream.value(utf8.encode(r.body)),
      r.statusCode,
      headers: r.headers,
      request: request,
    );
  }
}
