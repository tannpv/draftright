import 'dart:convert';
import 'package:http/http.dart' as http;

/// Raised for any non-2xx response. Carries the status code so callers can
/// branch (e.g. refresh-and-retry on 401) and a human-readable message parsed
/// from the body.
class ApiException implements Exception {
  final int statusCode;
  final String message;
  ApiException(this.statusCode, this.message);
  @override
  String toString() => message;
}

/// One HTTP path for the whole app: builds the URI, sets JSON + optional Bearer
/// headers, applies a timeout, throws [ApiException] on non-2xx, and decodes the
/// JSON body. Token refresh stays in the caller (auth/backend) — this is purely
/// the mechanical request, so it's reusable and easy to test.
class ApiClient {
  ApiClient({
    required this.baseUrl,
    http.Client? client,
    this.defaultTimeout = const Duration(seconds: 15),
  }) : _client = client ?? http.Client();

  String baseUrl;
  final http.Client _client;
  final Duration defaultTimeout;

  Future<Map<String, dynamic>> getJson(String path, {String? token, Duration? timeout}) =>
      _send('GET', path, token: token, timeout: timeout);

  Future<Map<String, dynamic>> postJson(String path, {Object? body, String? token, Duration? timeout}) =>
      _send('POST', path, body: body, token: token, timeout: timeout);

  Future<Map<String, dynamic>> deleteJson(String path, {String? token, Duration? timeout}) =>
      _send('DELETE', path, token: token, timeout: timeout);

  Future<Map<String, dynamic>> _send(String method, String path, {Object? body, String? token, Duration? timeout}) async {
    final uri = Uri.parse('$baseUrl$path');
    final headers = <String, String>{
      'Content-Type': 'application/json',
      if (token != null) 'Authorization': 'Bearer $token',
    };
    final Future<http.Response> future;
    switch (method) {
      case 'GET':
        future = _client.get(uri, headers: headers);
        break;
      case 'DELETE':
        future = _client.delete(uri, headers: headers);
        break;
      default:
        future = _client.post(uri, headers: headers, body: body == null ? null : jsonEncode(body));
    }
    final resp = await future.timeout(timeout ?? defaultTimeout);

    if (resp.statusCode >= 400) {
      throw ApiException(resp.statusCode, _parseError(resp.body, resp.statusCode));
    }
    if (resp.body.isEmpty) return <String, dynamic>{};
    final decoded = jsonDecode(resp.body);
    return decoded is Map<String, dynamic> ? decoded : <String, dynamic>{'data': decoded};
  }

  /// Pulls a useful message out of a NestJS-style error body
  /// (`{message}` string or array, or `{error}`); falls back to the status.
  static String _parseError(String body, int code) {
    try {
      final d = jsonDecode(body);
      if (d is Map) {
        final m = d['message'] ?? d['error'];
        if (m is List && m.isNotEmpty) return m.join(', ');
        if (m is String && m.isNotEmpty) return m;
      }
    } catch (_) {/* non-JSON body */}
    return 'HTTP $code';
  }
}
