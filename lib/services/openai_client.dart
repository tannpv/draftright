import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/tone.dart';

class OpenAIClient {
  final http.Client _client;

  OpenAIClient({http.Client? httpClient}) : _client = httpClient ?? http.Client();

  Future<String> rewrite({
    required String text,
    required Tone tone,
    required String apiKey,
    required String endpoint,
    required String model,
    required double temperature,
    String targetLanguage = 'English',
  }) async {
    final uri = Uri.parse(endpoint);
    final inputText = text.length > 3000 ? text.substring(0, 3000) : text;

    final headers = <String, String>{
      'Content-Type': 'application/json',
    };
    if (apiKey.isNotEmpty) {
      headers['Authorization'] = 'Bearer $apiKey';
    }

    final body = jsonEncode({
      'model': model,
      'messages': [
        {'role': 'system', 'content': tone.systemPrompt(targetLanguage: targetLanguage)},
        {'role': 'user', 'content': inputText},
      ],
      'temperature': temperature,
      'max_tokens': 1024,
    });

    final response = await _client
        .post(uri, headers: headers, body: body)
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      throw Exception('HTTP ${response.statusCode}: ${response.body}');
    }

    final decoded = jsonDecode(response.body) as Map<String, dynamic>;
    final choices = decoded['choices'] as List;
    if (choices.isEmpty) {
      throw Exception('No response from AI');
    }

    final content = choices[0]['message']['content'] as String;
    return content.trim();
  }
}
