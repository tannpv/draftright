import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:draftright_mobile/services/openai_client.dart';
import 'package:draftright_mobile/models/tone.dart';

void main() {
  test('rewrite sends correct request and parses response', () async {
    final mockClient = MockClient((request) async {
      expect(request.method, 'POST');
      expect(request.headers['Content-Type'], 'application/json');
      expect(request.headers['Authorization'], 'Bearer test-key');

      final body = jsonDecode(request.body);
      expect(body['model'], 'gpt-4o-mini');
      expect(body['messages'].length, 2);
      expect(body['messages'][0]['role'], 'system');
      expect(body['messages'][1]['role'], 'user');
      expect(body['messages'][1]['content'], 'Hello world');

      return http.Response(
        jsonEncode({
          'choices': [{'message': {'role': 'assistant', 'content': 'Hi there'}}]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'Hello world',
      tone: Tone.simple,
      apiKey: 'test-key',
      endpoint: 'https://api.openai.com/v1/chat/completions',
      model: 'gpt-4o-mini',
      temperature: 0.3,
    );
    expect(result, 'Hi there');
  });

  test('rewrite skips auth header when apiKey is empty', () async {
    final mockClient = MockClient((request) async {
      expect(request.headers.containsKey('Authorization'), false);
      return http.Response(
        jsonEncode({
          'choices': [{'message': {'role': 'assistant', 'content': 'result'}}]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'test',
      tone: Tone.concise,
      apiKey: '',
      endpoint: 'http://localhost:11434/v1/chat/completions',
      model: 'llama3',
      temperature: 0.3,
    );
    expect(result, 'result');
  });

  test('rewrite throws on HTTP error', () async {
    final mockClient = MockClient((request) async {
      return http.Response('{"error": "bad request"}', 400);
    });

    final client = OpenAIClient(httpClient: mockClient);
    expect(
      () => client.rewrite(
        text: 'test', tone: Tone.simple, apiKey: 'key',
        endpoint: 'https://api.openai.com/v1/chat/completions',
        model: 'gpt-4o-mini', temperature: 0.3,
      ),
      throwsException,
    );
  });

  test('rewrite throws on empty choices', () async {
    final mockClient = MockClient((request) async {
      return http.Response(jsonEncode({'choices': []}), 200);
    });

    final client = OpenAIClient(httpClient: mockClient);
    expect(
      () => client.rewrite(
        text: 'test', tone: Tone.simple, apiKey: 'key',
        endpoint: 'https://api.openai.com/v1/chat/completions',
        model: 'gpt-4o-mini', temperature: 0.3,
      ),
      throwsException,
    );
  });

  test('rewrite passes targetLanguage for translate tone', () async {
    final mockClient = MockClient((request) async {
      final body = jsonDecode(request.body);
      expect(body['messages'][0]['content'].toString().contains('Vietnamese'), true);
      return http.Response(
        jsonEncode({
          'choices': [{'message': {'role': 'assistant', 'content': 'Xin chao'}}]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'Hello', tone: Tone.translate, apiKey: 'key',
      endpoint: 'https://api.openai.com/v1/chat/completions',
      model: 'gpt-4o-mini', temperature: 0.3, targetLanguage: 'Vietnamese',
    );
    expect(result, 'Xin chao');
  });
}
