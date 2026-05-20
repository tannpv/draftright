import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/logger_service.dart';

class GrammarIssue {
  final String type;
  final int offset;
  final int length;
  final String original;
  final String suggestion;
  final String reason;

  const GrammarIssue({
    required this.type,
    required this.offset,
    required this.length,
    required this.original,
    required this.suggestion,
    required this.reason,
  });

  factory GrammarIssue.fromJson(Map<String, dynamic> json) {
    return GrammarIssue(
      type: (json['type'] ?? 'grammar').toString(),
      offset: (json['offset'] as num?)?.toInt() ?? 0,
      length: (json['length'] as num?)?.toInt() ?? 0,
      original: (json['original'] ?? '').toString(),
      suggestion: (json['suggestion'] ?? '').toString(),
      reason: (json['reason'] ?? '').toString(),
    );
  }
}

class GrammarResult {
  final int score;
  final List<GrammarIssue> issues;
  final int usageToday;
  final int dailyLimit;

  const GrammarResult({
    required this.score,
    required this.issues,
    required this.usageToday,
    required this.dailyLimit,
  });

  factory GrammarResult.fromJson(Map<String, dynamic> json) {
    final grammar = json['grammar'] as Map<String, dynamic>? ?? {};
    final issuesList = (grammar['issues'] as List<dynamic>?) ?? [];
    return GrammarResult(
      score: (grammar['score'] as num?)?.toInt() ?? 0,
      issues: issuesList
          .map((e) => GrammarIssue.fromJson(e as Map<String, dynamic>))
          .toList(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (json['daily_limit'] as num?)?.toInt() ?? 10,
    );
  }
}

class RewriteResult {
  final String rewrittenText;
  final int usageToday;
  final int dailyLimit;
  final GrammarResult? grammarResult;

  const RewriteResult({
    required this.rewrittenText,
    required this.usageToday,
    required this.dailyLimit,
    this.grammarResult,
  });

  bool get isGrammarCheck => grammarResult != null;
}

class SubscriptionInfo {
  final String planName;
  final String billingPeriod;
  final String status;
  final String? expiresAt;
  final int usageToday;
  final int dailyLimit;

  const SubscriptionInfo({
    required this.planName,
    required this.billingPeriod,
    required this.status,
    this.expiresAt,
    required this.usageToday,
    required this.dailyLimit,
  });

  bool get isFree => billingPeriod == 'none';

  factory SubscriptionInfo.fromJson(Map<String, dynamic> json) {
    final plan = json['plan'];
    String planName;
    String billingPeriod;
    int dailyLimit;

    if (plan is Map<String, dynamic>) {
      planName = (plan['name'] ?? 'Free').toString();
      billingPeriod = (plan['billing_period'] ?? 'none').toString();
      dailyLimit = (plan['daily_limit'] as num?)?.toInt() ?? 10;
    } else {
      planName = (plan ?? 'Free').toString();
      billingPeriod = 'none';
      dailyLimit = (json['daily_limit'] as num?)?.toInt() ?? 10;
    }

    return SubscriptionInfo(
      planName: planName,
      billingPeriod: billingPeriod,
      status: (json['status'] ?? 'active').toString(),
      expiresAt: json['expires_at']?.toString(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: dailyLimit,
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

  /// Returns the base URL with trailing slashes removed.
  String get _baseUrl {
    var url = _getBaseUrl();
    while (url.endsWith('/')) {
      url = url.substring(0, url.length - 1);
    }
    return url;
  }

  Future<RewriteResult> rewrite({
    required String text,
    required Tone tone,
    String? targetLanguage,
  }) async {
    DRLogger.log('Rewrite request: tone=${tone.name}', category: 'API');
    final token = await _auth.getAccessToken();
    final uri = Uri.parse('$_baseUrl/rewrite');

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
      final e = 'HTTP ${response.statusCode}: ${response.body}';
      DRLogger.error('Rewrite error: $e', category: 'API');
      throw Exception(e);
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;

    // Grammar check returns { grammar: { score, issues } } instead of { rewritten_text }
    if (tone == Tone.grammarCheck && data.containsKey('grammar')) {
      final grammarResult = GrammarResult.fromJson(data);
      final result = RewriteResult(
        rewrittenText: '',
        usageToday: grammarResult.usageToday,
        dailyLimit: grammarResult.dailyLimit,
        grammarResult: grammarResult,
      );
      DRLogger.log('Grammar check: score=${grammarResult.score}, issues=${grammarResult.issues.length}', category: 'API');
      return result;
    }

    final result = RewriteResult(
      rewrittenText: (data['rewritten_text'] as String).trim(),
      usageToday: (data['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: (data['daily_limit'] as num?)?.toInt() ?? 10,
    );
    DRLogger.log('Rewrite success: ${result.rewrittenText.length} chars', category: 'API');
    return result;
  }

  Future<SubscriptionInfo> getSubscription() async {
    final token = await _auth.getAccessToken();
    final uri = Uri.parse('$_baseUrl/subscription');

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
