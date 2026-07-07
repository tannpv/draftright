import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';
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
        // Always set an explicit image MIME.  Without this,
        // http.MultipartFile.fromPath falls back to
        // application/octet-stream when the file extension can't be
        // resolved (image_picker on Android sometimes hands back
        // cache paths with no extension), and the backend MIME
        // filter rejects the submission with HTTP 400 "only
        // PNG/JPEG screenshots are accepted" — exact 400 captured
        // 2026-06-01 (BUG-7).  Sniff the first bytes so PNG +
        // JPEG + WebP land on the right Content-Type; everything
        // else falls through to JPEG since image_picker
        // re-encodes most pickers to JPEG when imageQuality<100.
        final contentType = await _detectImageMime(screenshot);
        request.files.add(await http.MultipartFile.fromPath(
          'screenshot',
          screenshot.path,
          contentType: contentType,
        ));
      }

      final streamed =
          await request.send().timeout(const Duration(seconds: 30));
      final isOk = streamed.statusCode >= 200 && streamed.statusCode < 300;
      if (isOk) {
        DRLogger.log('Bug report submitted ($source)', category: 'BUG_REPORT');
        return const SubmitBugReportResult(ok: true);
      }
      final body = await streamed.stream.bytesToString();
      final status = streamed.statusCode;
      DRLogger.warn(
        'Bug report failed: $status $body',
        category: 'BUG_REPORT',
      );
      // 413 = request body over the server's cap. The screenshot is the only
      // large part, so point the user at it (their connection is fine — a
      // generic "check your connection" here is misleading; issue #68).
      if (status == 413) {
        return const SubmitBugReportResult(
          ok: false,
          errorMessage:
              'That screenshot is too large to upload. Remove it or attach a smaller image.',
        );
      }
      return SubmitBugReportResult(
        ok: false,
        // Fall back to the status code so a failure is never a dead end.
        errorMessage:
            _extractServerMessage(body) ?? 'Server error ($status). Please try again.',
      );
    } catch (e) {
      DRLogger.warn('Bug report exception: $e', category: 'BUG_REPORT');
      return const SubmitBugReportResult(ok: false, errorMessage: _genericFailure);
    }
  }

  /// Pulls a user-friendly reason out of an error body. Handles both the
  /// NestJS shape `{"message": "…"}` / `{"message": ["…"]}` (class-validator
  /// returns an array) and the Go backend shape `{"error": "…"}`.
  static String? _extractServerMessage(String body) {
    try {
      final parsed = jsonDecode(body);
      if (parsed is Map) {
        final m = parsed['message'];
        if (m is String && m.isNotEmpty) return m;
        if (m is List && m.isNotEmpty) return m.first.toString();
        final e = parsed['error'];
        if (e is String && e.isNotEmpty) return e;
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

  /// Best-effort image MIME by magic-byte sniff.  Falls back to
  /// image/jpeg (the most common image_picker output) so the upload
  /// always carries an image Content-Type — never
  /// application/octet-stream, which the backend rejects.
  static Future<MediaType> _detectImageMime(File file) async {
    try {
      final raf = await file.open();
      final header = await raf.read(12);
      await raf.close();
      if (header.length >= 8 &&
          header[0] == 0x89 && header[1] == 0x50 &&
          header[2] == 0x4E && header[3] == 0x47) {
        return MediaType('image', 'png');
      }
      if (header.length >= 3 &&
          header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF) {
        return MediaType('image', 'jpeg');
      }
      if (header.length >= 12 &&
          header[0] == 0x52 && header[1] == 0x49 &&
          header[2] == 0x46 && header[3] == 0x46 &&
          header[8] == 0x57 && header[9] == 0x45 &&
          header[10] == 0x42 && header[11] == 0x50) {
        return MediaType('image', 'webp');
      }
      if (header.length >= 12 &&
          header[4] == 0x66 && header[5] == 0x74 &&
          header[6] == 0x79 && header[7] == 0x70 &&
          ((header[8] == 0x68 && header[9] == 0x65 && header[10] == 0x69 && header[11] == 0x63) ||
           (header[8] == 0x68 && header[9] == 0x65 && header[10] == 0x69 && header[11] == 0x66))) {
        return MediaType('image', 'heic');
      }
      if (header.length >= 6 &&
          header[0] == 0x47 && header[1] == 0x49 &&
          header[2] == 0x46 && header[3] == 0x38) {
        return MediaType('image', 'gif');
      }
    } catch (_) {/* fall through */}
    return MediaType('image', 'jpeg');
  }

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
