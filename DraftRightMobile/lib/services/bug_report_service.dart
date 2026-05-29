import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:package_info_plus/package_info_plus.dart';

import 'package:draftright_mobile/services/logger_service.dart';

/// Outcome of a bug-report submission. Carries a user-presentable error
/// reason on failure so the UI can show the server's message ("Screenshot
/// must be PNG/JPEG", "Rate limit exceeded", etc.) instead of a generic
/// "submit failed" snackbar.
class SubmitBugReportResult {
  final bool ok;
  final String? errorMessage;
  const SubmitBugReportResult({required this.ok, this.errorMessage});
}

/// Submits a bug report to the DraftRight backend.
///
/// Stage D of the cross-platform bug-report rollout. Backend contract:
///   POST https://api.draftright.info/bug-reports  (multipart/form-data)
/// Auth is optional — anonymous submissions are accepted; if a JWT is
/// provided the backend records `user_id` on the row.
class BugReportService {
  /// Production endpoint. Override in tests via [endpointOverride].
  static const String _defaultEndpoint =
      'https://api.draftright.info/bug-reports';

  /// Hard cap from the backend (5 MB). Files larger than this are rejected
  /// with HTTP 413, so we short-circuit client-side too.
  static const int maxScreenshotBytes = 5 * 1024 * 1024;

  /// Result of [submitBugReport]. `ok=true` → 2xx response, no error.
  /// `ok=false` → submission failed; `errorMessage` is a user-presentable
  /// reason (server's JSON `message` field when available, otherwise a
  /// generic network-failure string).
  ///
  /// Returning the message lets the UI tell the user *why* their bug-report
  /// was rejected (e.g. "Screenshot must be PNG/JPEG") instead of the old
  /// blanket "submit failed", which prevented users from self-correcting.
  /// See feedback_clean_code_directive (Rule #1).
  static const String _genericFailure =
      'Could not submit bug report. Check your connection and try again.';

  /// Submit a bug report. Always resolves with a [SubmitBugReportResult].
  ///
  /// [description] is required and must be at least 10 chars (the UI
  /// enforces this — service-layer is permissive in case the caller has
  /// already validated).
  /// [screenshot] is an optional image file (≤ 5 MB; PNG / JPEG / WebP /
  /// HEIC / GIF — the backend's accepted-MIMEs list is the source of truth).
  /// [userEmail] should be supplied for anonymous reports.
  /// [authToken] is the JWT access token if the user is signed in.
  /// [context] is a free-form map (route, locale, plan) — JSON-encoded
  /// onto the wire as the `context` field.
  static Future<SubmitBugReportResult> submitBugReport({
    required String description,
    File? screenshot,
    String? userEmail,
    String? authToken,
    Map<String, dynamic>? context,
    String? endpointOverride,
  }) async {
    final endpoint = endpointOverride ?? _defaultEndpoint;
    final source = _detectSource();
    final appVersion = await _appVersion();
    final osInfo = _osInfo();

    try {
      final request = http.MultipartRequest('POST', Uri.parse(endpoint));
      request.fields['description'] = description;
      request.fields['source'] = source;
      request.fields['app_version'] = appVersion;
      request.fields['os_info'] = osInfo;

      if (userEmail != null && userEmail.isNotEmpty) {
        request.fields['user_email'] = userEmail;
      }
      if (context != null && context.isNotEmpty) {
        request.fields['context'] = jsonEncode(context);
      }

      if (authToken != null && authToken.isNotEmpty) {
        request.headers['Authorization'] = 'Bearer $authToken';
      }

      if (screenshot != null) {
        final length = await screenshot.length();
        if (length > maxScreenshotBytes) {
          DRLogger.warn(
            'Bug report rejected client-side: screenshot $length bytes > 5 MB',
            category: 'BUG_REPORT',
          );
          return const SubmitBugReportResult(
            ok: false,
            errorMessage: 'Screenshot is larger than 5 MB. Pick a smaller image.',
          );
        }
        request.files
            .add(await http.MultipartFile.fromPath('screenshot', screenshot.path));
      }

      final streamed =
          await request.send().timeout(const Duration(seconds: 30));
      final isOk = streamed.statusCode >= 200 && streamed.statusCode < 300;
      if (isOk) {
        DRLogger.log('Bug report submitted ($source)', category: 'BUG_REPORT');
        return const SubmitBugReportResult(ok: true);
      }
      final body = await streamed.stream.bytesToString();
      DRLogger.warn(
        'Bug report failed: ${streamed.statusCode} $body',
        category: 'BUG_REPORT',
      );
      return SubmitBugReportResult(
        ok: false,
        errorMessage: _extractServerMessage(body) ?? _genericFailure,
      );
    } catch (e) {
      DRLogger.warn('Bug report exception: $e', category: 'BUG_REPORT');
      return const SubmitBugReportResult(ok: false, errorMessage: _genericFailure);
    }
  }

  /// Pulls the user-friendly `message` field out of a NestJS error body.
  /// Tolerates both `{"message": "…"}` and `{"message": ["…"]}` (class-validator
  /// returns an array when multiple constraints fail).
  static String? _extractServerMessage(String body) {
    try {
      final parsed = jsonDecode(body);
      if (parsed is Map) {
        final m = parsed['message'];
        if (m is String && m.isNotEmpty) return m;
        if (m is List && m.isNotEmpty) return m.first.toString();
      }
    } catch (_) {/* body wasn't JSON — fall through */}
    return null;
  }

  static String _detectSource() {
    try {
      if (Platform.isIOS) return 'ios-app';
      if (Platform.isAndroid) return 'android-app';
    } catch (_) {/* ignore — non-mobile path */}
    // Mobile-only feature, but provide a safe fallback string the backend
    // will accept rather than throwing.
    return 'android-app';
  }

  static Future<String> _appVersion() async {
    try {
      final info = await PackageInfo.fromPlatform();
      return '${info.version}+${info.buildNumber}';
    } catch (_) {
      return 'unknown';
    }
  }

  /// Max chars sent to the backend — matches the bug-report DTO's
  /// @MaxLength(100). Android's Platform.operatingSystemVersion includes the
  /// full kernel release string (e.g. "5.10.198-android13-…-SMP-PREEMPT…"),
  /// which routinely exceeds 100 chars on Xiaomi/Samsung devices. Sending a
  /// longer value made the backend reject the whole submission with HTTP 400
  /// and the user got no record on the admin portal. Truncating here keeps
  /// the report flowing.
  static const int _maxOsInfoChars = 100;

  static String _osInfo() {
    try {
      final name = Platform.isIOS
          ? 'iOS'
          : Platform.isAndroid
              ? 'Android'
              : Platform.operatingSystem;
      final raw = '$name ${Platform.operatingSystemVersion}';
      return raw.length > _maxOsInfoChars
          ? raw.substring(0, _maxOsInfoChars)
          : raw;
    } catch (_) {
      return 'unknown';
    }
  }
}
