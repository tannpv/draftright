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

/// Default HTTP request timeout across the app — one source (#205 #11).
const Duration kRequestTimeout = Duration(seconds: 15);

/// Pulls a user-facing reason out of an error-response body. Handles the NestJS
/// shape `{"message": "…"}` / `{"message": ["…", …]}` (class-validator returns an
/// array — joined so all validation errors show) and the Go shape
/// `{"error": "…"}`. Returns null if the body has neither / isn't JSON, so each
/// caller can supply its own fallback. Single source of truth (#205 #7).
String? parseServerErrorMessage(String body) {
  try {
    final parsed = jsonDecode(body);
    if (parsed is Map) {
      final m = parsed['message'];
      if (m is String && m.isNotEmpty) return m;
      if (m is List && m.isNotEmpty) return m.join(', ');
      final e = parsed['error'];
      if (e is String && e.isNotEmpty) return e;
    }
  } catch (_) {/* body wasn't JSON — fall through */}
  return null;
}

/// One HTTP path for the whole app: builds the URI, sets JSON + optional Bearer
/// headers, applies a timeout, throws [ApiException] on non-2xx, and decodes the
/// JSON body. Token refresh stays in the caller (auth/backend) — this is purely
/// the mechanical request, so it's reusable and easy to test.
class ApiClient {
  ApiClient({
    required this.baseUrl,
    http.Client? client,
    this.defaultTimeout = kRequestTimeout,
  }) : _client = client ?? http.Client();

  String baseUrl;
  final http.Client _client;
  final Duration defaultTimeout;

  Future<Map<String, dynamic>> getJson(String path,
      {String? token, Duration? timeout}) async {
    final raw = await getAny(path, token: token, timeout: timeout);
    return raw is Map<String, dynamic> ? raw : <String, dynamic>{'data': raw};
  }

  Future<Map<String, dynamic>> postJson(String path,
          {Object? body, String? token, Duration? timeout}) =>
      _send('POST', path, body: body, token: token, timeout: timeout);

  Future<Map<String, dynamic>> deleteJson(String path,
          {String? token, Duration? timeout}) =>
      _send('DELETE', path, token: token, timeout: timeout);

  /// GET that returns whatever shape the server emits — Map, List, scalar.
  /// Use for endpoints whose root response isn't a JSON object
  /// (e.g. `/plans` returns a List). Callers cast as needed.
  Future<dynamic> getAny(String path,
      {String? token, Duration? timeout}) async {
    return _sendAny('GET', path, token: token, timeout: timeout);
  }

  /// POST a multipart/form-data request through the shared client — same
  /// timeout + error contract as the JSON methods (throws [ApiException] with
  /// the parsed server message on non-2xx, returns the decoded 2xx body).
  /// [fields] are text parts, [files] attachments. Centralizes the one
  /// hand-built multipart request in the app (#205 #9). The Authorization
  /// header is set from [token] when non-empty; Content-Type is the multipart
  /// boundary the request sets itself.
  Future<Map<String, dynamic>> postMultipart(
    String path, {
    Map<String, String> fields = const {},
    List<http.MultipartFile> files = const [],
    String? token,
    Duration? timeout,
  }) async {
    final req = http.MultipartRequest('POST', Uri.parse('$baseUrl$path'));
    req.fields.addAll(fields);
    req.files.addAll(files);
    if (token != null && token.isNotEmpty) {
      req.headers['Authorization'] = 'Bearer $token';
    }
    final streamed = await _client.send(req).timeout(timeout ?? defaultTimeout);
    final body = await streamed.stream.bytesToString();
    if (streamed.statusCode >= 400) {
      throw ApiException(
          streamed.statusCode, _parseError(body, streamed.statusCode));
    }
    if (body.isEmpty) return <String, dynamic>{};
    final decoded = jsonDecode(body);
    return decoded is Map<String, dynamic>
        ? decoded
        : <String, dynamic>{'data': decoded};
  }

  Future<Map<String, dynamic>> _send(String method, String path,
      {Object? body, String? token, Duration? timeout}) async {
    final raw = await _sendAny(method, path,
        body: body, token: token, timeout: timeout);
    if (raw is Map<String, dynamic>) return raw;
    return <String, dynamic>{'data': raw};
  }

  Future<dynamic> _sendAny(String method, String path,
      {Object? body, String? token, Duration? timeout}) async {
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
        future = _client.post(uri,
            headers: headers, body: body == null ? null : jsonEncode(body));
    }
    final resp = await future.timeout(timeout ?? defaultTimeout);

    if (resp.statusCode >= 400) {
      throw ApiException(
          resp.statusCode, _parseError(resp.body, resp.statusCode));
    }
    if (resp.body.isEmpty) return <String, dynamic>{};
    return jsonDecode(resp.body);
  }

  /// Pulls a useful message out of a NestJS-style error body
  /// (`{message}` string or array, or `{error}`); falls back to the status.
  static String _parseError(String body, int code) =>
      parseServerErrorMessage(body) ?? 'HTTP $code';
}
