import 'package:flutter/material.dart';

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

    final messenger = _messengerKey.currentState;
    if (messenger == null) return;
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
