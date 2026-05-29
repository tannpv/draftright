import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:package_info_plus/package_info_plus.dart';
import 'package:shared_preferences/shared_preferences.dart';

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
  static const _persistKey = 'draftright.error_reporter.queue';
  static bool _flushScheduled = false;

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
    _backendUrl = backendUrl.replaceAll(RegExp(r'/+$'), '');
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
      final raw = prefs.getStringList(_persistKey);
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
        _persistKey,
        _queue.map(jsonEncode).toList(),
      );
    } catch (_) {/* ignore */}
  }

  static void _enqueue({
    required String errorType,
    required String message,
    String? stack,
    String severity = 'error',
    Map<String, dynamic>? context,
  }) {
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

    // Surface the error to any subscribed UI overlay. Background auto-submit
    // (above) still runs unconditionally — this is purely for visibility so
    // the user sees that something failed and can decide whether to attach
    // extra context via "Report this".
    lastError.value = CapturedError(
      errorType: errorType,
      message: message,
      stack: stack,
      severity: severity,
      at: DateTime.now(),
    );
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
    Timer(const Duration(seconds: 3), _flush);
  }

  static Future<void> _flush() async {
    _flushScheduled = false;
    if (_queue.isEmpty) return;
    if (_backendUrl == null) return;

    final batch = List<Map<String, dynamic>>.from(_queue);
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (_bearerToken != null && _bearerToken!.isNotEmpty) {
      headers['Authorization'] = 'Bearer $_bearerToken';
    }

    var sentAny = false;
    for (final entry in batch) {
      try {
        final res = await http
            .post(
              Uri.parse('$_backendUrl/errors'),
              headers: headers,
              body: jsonEncode(entry),
            )
            .timeout(const Duration(seconds: 10));
        if (res.statusCode >= 200 && res.statusCode < 300) {
          _queue.remove(entry);
          sentAny = true;
        } else {
          // Server rejected — drop this one to avoid infinite retries
          _queue.remove(entry);
        }
      } catch (_) {
        // Network/timeout — leave in queue, retry next launch or next event
      }
    }

    if (sentAny) await _persistQueue();
    if (_queue.isNotEmpty) _scheduleFlush();
  }
}
