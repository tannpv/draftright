import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/auth_service.dart';

class RewriteResult {
  final String rewrittenText;
  final int usageToday;
  final int dailyLimit;

  const RewriteResult({
    required this.rewrittenText,
    required this.usageToday,
    required this.dailyLimit,
  });
}

class SubscriptionInfo {
  final String plan;
  final String status;
  final String? expiresAt;
  final int usageToday;
  final int dailyLimit;

  const SubscriptionInfo({
    required this.plan,
    required this.status,
    this.expiresAt,
    required this.usageToday,
    required this.dailyLimit,
  });

  factory SubscriptionInfo.fromJson(Map<String, dynamic> json) {
    return SubscriptionInfo(
      plan: (json['plan'] ?? 'free').toString(),
      status: (json['status'] ?? 'active').toString(),
      expiresAt: json['expires_at']?.toString(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 10,
    );
  }
}

class BackendClient {
  final AuthService _auth;
  final String Function() _getBaseUrl;
  final http.Client _http;

  BackendClient({
    required AuthService auth,
    required String Function() getBaseUrl,
    http.Client? httpClient,
  })  : _auth = auth,
        _getBaseUrl = getBaseUrl,
        _http = httpClient ?? http.Client();

  Future<RewriteResult> rewrite({
    required String text,
    required Tone tone,
    String? targetLanguage,
  }) async {
    final token = await _auth.getAccessToken();
    final uri = Uri.parse('${_getBaseUrl()}/rewrite');

    final body = <String, dynamic>{
      'text': text.length > 3000 ? text.substring(0, 3000) : text,
      'tone': tone.apiValue,
    };
    if (targetLanguage != null && targetLanguage.isNotEmpty) {
      body['target_language'] = targetLanguage;
    }

    http.Response response = await _post(uri, body, token);

    // Auto-refresh on 401
    if (response.statusCode == 401) {
      final refreshed = await _auth.tryRefresh();
      if (refreshed) {
        final newToken = await _auth.getAccessToken();
        response = await _post(uri, body, newToken);
      }
    }

    if (response.statusCode >= 400) {
      throw Exception('HTTP ${response.statusCode}: ${response.body}');
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return RewriteResult(
      rewrittenText: (data['rewritten_text'] as String).trim(),
      usageToday: (data['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (data['daily_limit'] as num?)?.toInt() ?? 10,
    );
  }

  Future<SubscriptionInfo> getSubscription() async {
    final token = await _auth.getAccessToken();
    final uri = Uri.parse('${_getBaseUrl()}/subscription');

    var response = await _http.get(
      uri,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    ).timeout(const Duration(seconds: 15));

    if (response.statusCode == 401) {
      final refreshed = await _auth.tryRefresh();
      if (refreshed) {
        final newToken = await _auth.getAccessToken();
        response = await _http.get(
          uri,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer $newToken',
          },
        ).timeout(const Duration(seconds: 15));
      }
    }

    if (response.statusCode >= 400) {
      throw Exception('HTTP ${response.statusCode}: ${response.body}');
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return SubscriptionInfo.fromJson(data);
  }

  Future<http.Response> _post(Uri uri, Map<String, dynamic> body, String token) {
    return _http.post(
      uri,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
      body: jsonEncode(body),
    ).timeout(const Duration(seconds: 15));
  }
}
