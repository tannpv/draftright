import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import 'package:draftright_mobile/services/error_reporter.dart';
import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

/// Wraps the app's MaterialApp child so that every captured [CapturedError]
/// surfaces as a transient SnackBar — short error line + "Report this"
/// button that opens the bug-report sheet pre-filled with the message.
///
/// Auto-submit already runs inside [ErrorReporter] (the captured error has
/// been queued for /errors before this widget reacts), so the SnackBar is
/// purely a visibility layer. Users who want to add context tap "Report
/// this"; users who don't can dismiss and keep working.
///
/// Drop into MaterialApp via `builder:`:
///   MaterialApp(
///     builder: (ctx, child) => ErrorNoticeOverlay(child: child ?? const SizedBox()),
///     home: ...
///   )
class ErrorNoticeOverlay extends StatefulWidget {
  final Widget child;
  const ErrorNoticeOverlay({super.key, required this.child});

  @override
  State<ErrorNoticeOverlay> createState() => _ErrorNoticeOverlayState();
}

class _ErrorNoticeOverlayState extends State<ErrorNoticeOverlay> {
  final GlobalKey<ScaffoldMessengerState> _messengerKey =
      GlobalKey<ScaffoldMessengerState>();
  // De-dupe: don't fire the same error twice in a row when build cycles
  // re-trigger _enqueue rapidly.
  DateTime? _lastShownAt;

  @override
  void initState() {
    super.initState();
    ErrorReporter.lastError.addListener(_onErrorChanged);
  }

  @override
  void dispose() {
    ErrorReporter.lastError.removeListener(_onErrorChanged);
    super.dispose();
  }

  void _onErrorChanged() {
    final err = ErrorReporter.lastError.value;
    if (err == null) return;
    if (_lastShownAt == err.at) return;
    _lastShownAt = err.at;

    // Defer to the next frame. If the notifier fires synchronously from
    // inside a Flutter build (PlatformDispatcher.onError can be invoked
    // while a widget is rebuilding) calling ScaffoldMessenger.showSnackBar
    // directly throws "setState during build", which itself goes through
    // PlatformDispatcher → _enqueue → notifies us again → infinite loop →
    // main-thread starvation → Android ANR. Confirmed on Galaxy A52
    // 2026-05-29: "Input dispatching timed out, waited 10003ms".
    SchedulerBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final messenger = _messengerKey.currentState;
      if (messenger == null) return;
      // showSnackBar asserts that a descendant Scaffold is registered. During
      // bootstrap (splash MaterialApp, before LoginScreen mounts) or on
      // Scaffold-less routes there is none, so the call throws AssertionError.
      // Swallow it — the error was already auto-submitted to /errors above;
      // the snackbar is a "nice-to-have" visibility layer, not the recording
      // path. Without this guard, the assertion itself goes through
      // FlutterError.onError → _enqueue → notifies us → next frame → throws
      // again — a loop that was the root cause of the Galaxy A52 ANR.
      try {
        messenger.clearSnackBars();
        messenger.showSnackBar(
          SnackBar(
            content: Text('Something went wrong: ${err.shortLine}'),
            duration: const Duration(seconds: 8),
            action: SnackBarAction(
              label: 'REPORT',
              onPressed: () {
                final ctx = _messengerKey.currentContext;
                if (ctx == null) return;
                showReportBugSheet(
                  ctx,
                  currentRoute: '/error-notice',
                  initialDescription:
                      'Auto-captured error:\n${err.errorType}: ${err.message}\n\n'
                      'What I was doing when it happened:\n',
                );
              },
            ),
          ),
        );
      } catch (_) {/* no Scaffold yet — skip silently */}
    });
  }

  @override
  Widget build(BuildContext context) {
    // ScaffoldMessenger wraps the child so SnackBars work even on screens
    // that lack their own Scaffold (Login, Onboarding).
    return ScaffoldMessenger(
      key: _messengerKey,
      child: widget.child,
    );
  }
}
