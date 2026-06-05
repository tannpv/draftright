import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/nudge_state.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/api_client.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/logger_service.dart';
import 'package:draftright_mobile/services/payment/checkout_result.dart';
import 'package:draftright_mobile/services/payment/payment_method.dart';

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
  final NudgeState? nudge;

  const SubscriptionInfo({
    required this.planName,
    required this.billingPeriod,
    required this.status,
    this.expiresAt,
    required this.usageToday,
    required this.dailyLimit,
    this.nudge,
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

    final nudgeJson = json['nudge'];
    final nudge = nudgeJson is Map<String, dynamic> ? NudgeState.fromJson(nudgeJson) : null;

    return SubscriptionInfo(
      planName: planName,
      billingPeriod: billingPeriod,
      status: (json['status'] ?? 'active').toString(),
      expiresAt: json['expires_at']?.toString(),
      usageToday: (json['usage_today'] as num?)?.toInt() ?? 0,
      dailyLimit: dailyLimit,
      nudge: nudge,
    );
  }
}

class BackendClient {
  /// Backend caps rewrite input length; truncate client-side to match.
  static const int _maxInputChars = 3000;
  /// Timeout for the main rewrite/subscription calls.
  static const Duration _requestTimeout = Duration(seconds: 15);
  /// Shorter timeout for the best-effort /health probe.
  static const Duration _healthTimeout = Duration(seconds: 5);

  final AuthService _auth;
  final String Function() _getBaseUrl;
  final http.Client _http;
  late final ApiClient _api = ApiClient(baseUrl: '', client: _http, defaultTimeout: _requestTimeout);

  BackendClient({
    required AuthService auth,
    required String Function() getBaseUrl,
    http.Client? httpClient,
  })  : _auth = auth,
        _getBaseUrl = getBaseUrl,
        _http = httpClient ?? http.Client();

  /// Run an authed call, refreshing the token + retrying once on 401.
  Future<Map<String, dynamic>> _authed(Future<Map<String, dynamic>> Function(String token) call) async {
    _api.baseUrl = _baseUrl;
    final token = await _auth.getAccessToken();
    try {
      return await call(token);
    } on ApiException catch (e) {
      if (e.statusCode == 401 && await _auth.tryRefresh()) {
        return await call(await _auth.getAccessToken());
      }
      rethrow;
    }
  }

  /// Returns the base URL with trailing slashes removed.
  String get _baseUrl {
    var url = _getBaseUrl();
    while (url.endsWith('/')) {
      url = url.substring(0, url.length - 1);
    }
    return url;
  }

  /// Best-effort: fetch `/health` and apply the admin-controlled
  /// `client_log_level` to [DRLogger]. The mobile app doesn't poll /health for
  /// liveness, so this runs once at startup. Any failure leaves the current
  /// level untouched and never blocks startup.
  static Future<void> applyClientLogLevel(String backendUrl) async {
    try {
      var base = backendUrl;
      while (base.endsWith('/')) {
        base = base.substring(0, base.length - 1);
      }
      final resp = await http
          .get(Uri.parse('$base/health'))
          .timeout(_healthTimeout);
      if (resp.statusCode != 200) return;
      final data = jsonDecode(resp.body) as Map<String, dynamic>;
      if (data['app'] != 'draftright') return;
      DRLogger.setMinLevelFromServer(data['client_log_level'] as String?);
    } catch (_) {
      // Best-effort — never block startup or change level on error.
    }
  }

  /// Release notes the backend advertises for [version] on [platform]
  /// ('android' | 'ios' | …), or null if the latest published version no
  /// longer matches (e.g. a newer one is out) or there are no notes. Used for
  /// the post-update "What's New" notice. Best-effort.
  static Future<String?> releaseNotesForVersion(
      String backendUrl, String platform, String version) async {
    try {
      var base = backendUrl;
      while (base.endsWith('/')) {
        base = base.substring(0, base.length - 1);
      }
      final resp = await http
          .get(Uri.parse('$base/updates/latest?platform=$platform'))
          .timeout(_healthTimeout);
      if (resp.statusCode != 200) return null;
      final data = jsonDecode(resp.body) as Map<String, dynamic>;

      // Prefer the per-platform entry; fall back to the legacy top-level envelope.
      String? backendVersion;
      String? notes;
      final platforms = data['platforms'];
      if (platforms is Map && platforms[platform] is Map) {
        final p = platforms[platform] as Map;
        backendVersion = p['version'] as String?;
        notes = p['notes'] as String?;
      }
      backendVersion ??= data['version'] as String?;
      notes ??= data['release_notes'] as String?;

      if (backendVersion != version) return null;
      if (notes == null || notes.trim().isEmpty) return null;
      return notes;
    } catch (_) {
      return null;
    }
  }

  Future<RewriteResult> rewrite({
    required String text,
    required Tone tone,
    String? targetLanguage,
  }) async {
    DRLogger.log('Rewrite request: tone=${tone.name}', category: 'API');

    final body = <String, dynamic>{
      'text': text.length > _maxInputChars ? text.substring(0, _maxInputChars) : text,
      'tone': tone.apiValue,
    };
    if (targetLanguage != null && targetLanguage.isNotEmpty) {
      body['target_language'] = targetLanguage;
    }

    final Map<String, dynamic> data;
    try {
      data = await _authed((t) => _api.postJson('/rewrite', body: body, token: t));
    } catch (e) {
      DRLogger.error('Rewrite error: $e', category: 'API');
      rethrow;
    }

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
    final data = await _authed((t) => _api.getJson('/subscription', token: t));
    return SubscriptionInfo.fromJson(data);
  }

  /// Fetch the public plan catalog. Returns raw rows so the caller can
  /// pick whichever plan (Pro monthly, Pro yearly, etc.) it wants
  /// without leaking the plan id list back to the UI layer.
  ///
  /// Endpoint is unauthenticated → pass no token to the shared
  /// ApiClient. Routes through the same client every other call uses
  /// so timeouts, error envelope decoding, and request-id propagation
  /// stay consistent.
  Future<List<Map<String, dynamic>>> listPlans() async {
    _api.baseUrl = _baseUrl;
    final raw = await _api.getAny('/plans');
    if (raw is List) {
      return raw.cast<Map<String, dynamic>>();
    }
    return const [];
  }

  /// Backend-advertised list of currently enabled payment methods —
  /// the `GET /payment/methods` envelope is `{methods: [string]}`.
  /// Wire strings unknown to this client are filtered out so older
  /// builds keep working when the backend adds a new strategy.
  Future<List<PaymentMethodKind>> listPaymentMethods() async {
    _api.baseUrl = _baseUrl;
    final data = await _api.getJson('/payment/methods');
    final raw = (data['methods'] as List?) ?? const [];
    return raw
        .map((m) => PaymentMethodKind.fromWire(m.toString()))
        .whereType<PaymentMethodKind>()
        .toList();
  }

  /// POST /payment/checkout — returns a typed [CheckoutResult]
  /// (redirect URL, QR image, or bank-info block, depending on the
  /// strategy on the backend).
  Future<CheckoutResult> createCheckout({
    required String planId,
    required PaymentMethodKind method,
  }) async {
    final data = await _authed((t) => _api.postJson(
          '/payment/checkout',
          body: {'plan_id': planId, 'method': method.wireName},
          token: t,
        ));
    return CheckoutResult.fromJson(data);
  }

  /// Poll `/payment/status/:ref` for async confirmation.  Used by the
  /// VietQR / bank-transfer flows where the user pays out-of-band and
  /// the SePay / statement-line webhook activates the subscription.
  /// Returns the raw status envelope; caller checks `status == 'completed'`.
  Future<Map<String, dynamic>> getPaymentStatus(String referenceCode) async {
    _api.baseUrl = _baseUrl;
    return _api.getJson('/payment/status/$referenceCode');
  }

  /// One-shot Customer Portal URL for managing the active
  /// subscription (cancel, change plan, update card).  Backend's
  /// `/payment/portal` looks up the user's active subscription and
  /// dispatches to the correct provider (Lemon Squeezy or Stripe);
  /// VietQR / bank-transfer / admin-granted subscriptions return
  /// 404 (no self-service portal).
  Future<String> getCustomerPortalUrl() async {
    final data = await _authed((t) => _api.getJson('/payment/portal', token: t));
    final url = data['url'] as String?;
    if (url == null || url.isEmpty) {
      throw Exception('Backend did not return a portal URL');
    }
    return url;
  }

  /// Cancel the user's currently-active subscription via the backend
  /// (which calls LS / Stripe APIs directly).  Returns the date Pro
  /// access ends — the user keeps access through that date.
  Future<CancelSubscriptionResult> cancelSubscription() async {
    final data = await _authed((t) => _api.deleteJson('/payment/subscription', token: t));
    return CancelSubscriptionResult(
      cancelled: data['cancelled'] == true,
      accessUntil: data['expires_at'] != null
          ? DateTime.tryParse(data['expires_at'].toString())
          : null,
    );
  }
}

/// Outcome of a successful POST /payment/subscription cancellation —
/// the user has been told the subscription is cancelled but still
/// has Pro access until `accessUntil`.
class CancelSubscriptionResult {
  final bool cancelled;
  final DateTime? accessUntil;
  const CancelSubscriptionResult({required this.cancelled, this.accessUntil});
}
