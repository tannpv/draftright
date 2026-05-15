import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/entity.dart';

class ExtractionUnavailableException implements Exception {
  ExtractionUnavailableException(this.reason);
  final String reason;
  @override
  String toString() => 'ExtractionUnavailableException: $reason';
}

class ExtractionQuotaException implements Exception {
  ExtractionQuotaException();
  @override
  String toString() => 'ExtractionQuotaException';
}

class ExtractionApi {
  ExtractionApi({
    required this.baseUrl,
    required this.tokenProvider,
    http.Client? httpClient,
    Duration? timeout,
  })  : _http = httpClient ?? http.Client(),
        _timeout = timeout ?? const Duration(seconds: 10);

  final String baseUrl;
  final Future<String?> Function() tokenProvider;
  final http.Client _http;
  final Duration _timeout;

  Future<List<Entity>> llmExtract(String text) async {
    final token = await tokenProvider();
    if (token == null || token.isEmpty) {
      throw ExtractionUnavailableException('missing auth token');
    }
    final url = Uri.parse('${_strip(baseUrl)}/extract');
    final body = jsonEncode({'text': text});
    final http.Response resp;
    try {
      resp = await _http
          .post(url,
              headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer $token',
              },
              body: body)
          .timeout(_timeout);
    } on TimeoutException {
      throw ExtractionUnavailableException('timeout');
    } catch (e) {
      throw ExtractionUnavailableException('network: $e');
    }

    if (resp.statusCode == 402) throw ExtractionQuotaException();
    if (resp.statusCode == 401 || resp.statusCode == 403) {
      throw ExtractionUnavailableException('auth: ${resp.statusCode}');
    }
    if (resp.statusCode < 200 || resp.statusCode >= 300) {
      throw ExtractionUnavailableException('http: ${resp.statusCode}');
    }
    final Map<String, dynamic> json;
    try {
      json = jsonDecode(resp.body) as Map<String, dynamic>;
    } catch (_) {
      throw ExtractionUnavailableException('malformed response');
    }
    final list = (json['entities'] as List?) ?? const [];
    return list
        .map((raw) {
          final m = Map<String, dynamic>.from(raw as Map);
          m['source'] = 'llm';
          return Entity.fromJson(m);
        })
        .toList();
  }

  static String _strip(String s) => s.endsWith('/') ? s.substring(0, s.length - 1) : s;
}
