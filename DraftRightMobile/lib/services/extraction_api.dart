import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/entity.dart';
import 'extraction_cache.dart';

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
    ExtractionCache? cache,
  })  : _http = httpClient ?? http.Client(),
        _timeout = timeout ?? const Duration(seconds: 10),
        _cache = cache ?? _sharedCache;

  final String baseUrl;
  final Future<String?> Function() tokenProvider;
  final http.Client _http;
  final Duration _timeout;
  final ExtractionCache _cache;

  // Shared across the short-lived ExtractionApi instances the UI creates per
  // tap, so a repeat smart-scan of the same message is served from memory.
  static final ExtractionCache _sharedCache = ExtractionCache();

  /// Drop all cached extractions — call on logout so a new user on the same
  /// device starts clean (parity with the rewrite cache's sign-out clear, #147).
  static void clearCache() => _sharedCache.clear();

  Future<List<Entity>> llmExtract(String text) async {
    final token = await tokenProvider();
    if (token == null || token.isEmpty) {
      throw ExtractionUnavailableException('missing auth token');
    }

    // Cache hit (after auth): skip the /extract LLM round-trip for a message
    // already scanned by this authed device this session.
    final cached = _cache.get(text);
    if (cached != null) return cached;
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
    final entities = list
        .map((raw) {
          final m = Map<String, dynamic>.from(raw as Map);
          m['source'] = 'llm';
          return Entity.fromJson(m);
        })
        .toList();
    _cache.set(text, entities); // cache only successful results
    return entities;
  }

  static String _strip(String s) => s.endsWith('/') ? s.substring(0, s.length - 1) : s;
}
