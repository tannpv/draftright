import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:package_info_plus/package_info_plus.dart';

import 'package:draftright_mobile/services/api_client.dart';
import 'package:draftright_mobile/services/url_util.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:draftright_mobile/services/prefs_keys.dart';

/// One captured error, surfaced to the UI for an on-screen notice. The
/// reporter publishes the latest of these via [ErrorReporter.lastError] so
/// any widget can react (banner, snackbar, dev overlay) without depending
/// on the full backend submission pipeline.
class CapturedError {
  final String errorType;
  final String message;
  final String? stack;
  final String severity;
  final DateTime at;
  const CapturedError({
    required this.errorType,
    required this.message,
    this.stack,
    this.severity = 'error',
    required this.at,
  });

  /// Short single-line preview suitable for a snackbar / banner.
  String get shortLine {
    final firstLine = message.split('\n').first.trim();
    return firstLine.length > 140
        ? '${firstLine.substring(0, 137)}…'
        : firstLine;
  }
}

/// Reports unhandled errors and exceptions to the DraftRight backend's
/// /errors endpoint. Wrap your `runApp(...)` call in
/// `ErrorReporter.run(() => runApp(...), backendUrl: ...)` and crashes
/// from anywhere in the Dart code path become DB rows the team can
/// triage.
///
/// Privacy: never sends user-typed text content. Only stack traces +
/// error type + a small sanitized context.
class ErrorReporter {
  static String? _backendUrl;
  static String? _bearerToken;
  static String? _appVersion;
  static final _queue = <Map<String, dynamic>>[];
  static bool _flushScheduled = false;
  static Timer? _flushTimer;

  /// Backend path errors are POSTed to. One source (Rule #1).
  static const _errorsPath = '/errors';

  /// Per-request timeout for the best-effort error flush — shorter than the
  /// main request timeout so telemetry never delays anything user-facing.
  static const _flushTimeout = Duration(seconds: 10);

  /// Debounce before flushing a freshly-enqueued batch.
  static const _flushDelay = Duration(seconds: 3);

  /// Test seam: when set, the flush uses this http client instead of a fresh
  /// one, so unit tests can drive the queue semantics without real network.
  @visibleForTesting
  static http.Client? debugHttpClient;

  /// Latest captured error, or null if none yet. UI widgets can subscribe to
  /// this to show an on-screen notice ("something went wrong: …") without
  /// having to wrap every call site in try/catch. Cleared by calling
  /// `lastError.value = null` after the user dismisses the banner.
  static final ValueNotifier<CapturedError?> lastError =
      ValueNotifier<CapturedError?>(null);

  /// Install crash handlers + record the backend URL / bearer token.
  ///
  /// Synchronous and non-blocking on purpose: the app must already be on
  /// screen before this runs. (A previous version `await`ed app-version
  /// and queue loads before `runApp`, which — if those platform-channel
  /// calls stalled on a clean install — produced a permanent blank screen
  /// and an App Store rejection. Now `runApp` happens first; this just
  /// wires error capture afterward and warms up in the background.)
  static void attach({required String backendUrl, String? bearerToken}) {
    _backendUrl = normalizeBackendUrl(backendUrl);
    _bearerToken = bearerToken;

    // Synchronous Flutter framework errors (build phase, etc.)
    FlutterError.onError = (FlutterErrorDetails details) {
      _enqueue(
        errorType: details.exception.runtimeType.toString(),
        message: details.exceptionAsString(),
        stack: details.stack?.toString(),
        severity: 'error',
        context: {
          'library': details.library,
          'context': details.context?.toString(),
        },
      );
    };

    // Async/platform/engine errors
    PlatformDispatcher.instance.onError = (Object error, StackTrace stack) {
      _enqueue(
        errorType: error.runtimeType.toString(),
        message: error.toString(),
        stack: stack.toString(),
        severity: 'fatal',
      );
      return true; // mark handled — we've recorded it
    };

    // Warm-up (app version + persisted queue) — fire-and-forget so a slow
    // platform channel can never block the UI.
    unawaited(_loadAppVersion());
    unawaited(_loadPersistedQueue());
  }

  /// Update the bearer token after sign-in/out so future reports get
  /// associated with the right user.
  static void setBearerToken(String? token) {
    _bearerToken = token;
  }

  /// Update the backend URL after the user changes it in
  /// Settings → Server.  Without this, auto-captured errors keep
  /// posting to the URL captured at `attach()` time — so switching
  /// from prod → dev silently leaked dev crashes into the prod
  /// /errors stream until the app was restarted.  Normalises
  /// trailing slashes the same way [attach] does so the upload URL
  /// stays consistent (`<base>/errors`).
  static void setBackendUrl(String? url) {
    if (url == null || url.isEmpty) return;
    _backendUrl = normalizeBackendUrl(url);
  }

  /// Manually report a non-fatal issue (e.g. a caught exception in a
  /// service layer that the user shouldn't see but the team should).
  static void reportHandled(
    Object error, {
    StackTrace? stack,
    String severity = 'warning',
    Map<String, dynamic>? context,
  }) {
    _enqueue(
      errorType: error.runtimeType.toString(),
      message: error.toString(),
      stack: (stack ?? StackTrace.current).toString(),
      severity: severity,
      context: context,
    );
  }

  // ── Internals ──────────────────────────────────────────────────────────

  static Future<void> _loadAppVersion() async {
    try {
      final info = await PackageInfo.fromPlatform();
      _appVersion = '${info.version}+${info.buildNumber}';
    } catch (_) {
      _appVersion = 'unknown';
    }
  }

  static Future<void> _loadPersistedQueue() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = prefs.getStringList(PrefsKeys.errorReporterQueue);
      if (raw != null) {
        for (final s in raw) {
          try {
            final m = jsonDecode(s) as Map<String, dynamic>;
            _queue.add(m);
          } catch (_) {/* skip corrupt entries */}
        }
        if (_queue.isNotEmpty) _scheduleFlush();
      }
    } catch (_) {/* persistence is best-effort */}
  }

  static Future<void> _persistQueue() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setStringList(
        PrefsKeys.errorReporterQueue,
        _queue.map(jsonEncode).toList(),
      );
    } catch (_) {/* ignore */}
  }

  /// Patterns that should never reach /errors — expected non-issues that
  /// were being thrown as Exceptions for control flow (e.g. AuthService
  /// throwing "Not logged in" when bootstrap tries to load a token before
  /// the user has signed in). Match by substring on the error message;
  /// case-sensitive. Keep this list short — the real fix is to stop
  /// throwing for expected control flow, but suppressing here keeps the
  /// /errors stream free of noise while we work back to that.
  static const _suppressedSubstrings = <String>[
    'Not logged in',
    // The ErrorNoticeOverlay catches this internally now, but if any other
    // caller hits it on a Scaffold-less route it is also a known non-issue.
    'no descendant Scaffolds to present to',
  ];

  static bool _isSuppressed(String message) {
    for (final s in _suppressedSubstrings) {
      if (message.contains(s)) return true;
    }
    return false;
  }

  static void _enqueue({
    required String errorType,
    required String message,
    String? stack,
    String severity = 'error',
    Map<String, dynamic>? context,
  }) {
    if (_isSuppressed(message)) return;
    final platform = _detectPlatform();
    final entry = <String, dynamic>{
      'platform': platform,
      'app_version': _appVersion ?? 'unknown',
      'severity': severity,
      'error_type': errorType,
      'message': _truncate(message, 5000),
      'stack_trace': _truncate(stack ?? '', 20000),
      'context': context,
    };
    _queue.add(entry);
    if (_queue.length > 100) _queue.removeAt(0); // bound queue
    _persistQueue(); // fire-and-forget
    _scheduleFlush();

    // Surface only severities the user couldn't already see — anything routed
    // through reportHandled with severity='warning' is a known/handled failure
    // whose caller already shows its own UI (banner, snackbar). Raising the
    // overlay too would double-notify. Auto-captured 'error'/'fatal' are the
    // unexpected ones the user needs to be told about.
    if (severity == 'error' || severity == 'fatal') {
      lastError.value = CapturedError(
        errorType: errorType,
        message: message,
        stack: stack,
        severity: severity,
        at: DateTime.now(),
      );
    }
  }

  static String _detectPlatform() {
    if (kIsWeb) return 'web';
    try {
      if (Platform.isIOS) return 'ios';
      if (Platform.isAndroid) return 'android';
      if (Platform.isMacOS) return 'macos';
      if (Platform.isWindows) return 'windows';
      if (Platform.isLinux) return 'linux';
    } catch (_) {/* not on a real platform */}
    return 'unknown';
  }

  static String _truncate(String s, int max) =>
      s.length > max ? s.substring(0, max) : s;

  static void _scheduleFlush() {
    if (_flushScheduled) return;
    if (_backendUrl == null) return;
    _flushScheduled = true;
    _flushTimer = Timer(_flushDelay, _flush);
  }

  static Future<void> _flush() async {
    _flushScheduled = false;
    if (_queue.isEmpty) return;
    final backendUrl = _backendUrl;
    if (backendUrl == null) return;

    // Route through the shared ApiClient chokepoint (JSON + Bearer headers,
    // timeout, error handling) instead of a hand-built request (#205 #9).
    final client = debugHttpClient ?? http.Client();
    final api = ApiClient(
      baseUrl: backendUrl,
      client: client,
      defaultTimeout: _flushTimeout,
    );
    var sentAny = false;
    try {
      // Copy so _queue can be mutated while iterating.
      for (final entry in List<Map<String, dynamic>>.from(_queue)) {
        try {
          await api.postJson(_errorsPath, body: entry, token: _bearerToken);
          _queue.remove(entry); // 2xx — sent
          sentAny = true;
        } on ApiException {
          _queue.remove(entry); // non-2xx — drop to avoid infinite retries
        } catch (_) {
          // Network / timeout — leave in queue, retry next launch or event.
        }
      }
    } finally {
      if (debugHttpClient == null) client.close();
    }

    if (sentAny) await _persistQueue();
    if (_queue.isNotEmpty) _scheduleFlush();
  }

  // ── Test seams (visibleForTesting) ─────────────────────────────────────────
  @visibleForTesting
  static Future<void> flushForTest() => _flush();

  @visibleForTesting
  static void seedQueueForTest(List<Map<String, dynamic>> entries) => _queue
    ..clear()
    ..addAll(entries);

  @visibleForTesting
  static List<Map<String, dynamic>> get queueForTest =>
      List.unmodifiable(_queue);

  @visibleForTesting
  static void resetForTest() {
    _flushTimer?.cancel();
    _flushTimer = null;
    _flushScheduled = false;
    _queue.clear();
    _backendUrl = null;
    _bearerToken = null;
    debugHttpClient = null;
  }
}
