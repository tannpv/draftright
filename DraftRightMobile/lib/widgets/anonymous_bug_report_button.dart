import 'package:flutter/material.dart';
import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

/// Inline "Having trouble? Report a bug" link for anonymous (pre-auth)
/// screens — Login, Register, Forgot Password, etc.
///
/// The backend bug-report endpoint accepts anonymous submissions, so users
/// who hit a blocker before they can sign in still have an escape hatch.
/// Without this, "I can't sign in" is silently unreportable.
///
/// [routeName] is stamped on the report's `context.route` so triagers can
/// see what screen the user was on (e.g. `/login`, `/register`).
class AnonymousBugReportButton extends StatelessWidget {
  final String routeName;
  const AnonymousBugReportButton({super.key, required this.routeName});

  @override
  Widget build(BuildContext context) {
    return TextButton.icon(
      icon: const Icon(Icons.bug_report_outlined, size: 18),
      label: const Text('Having trouble? Report a bug'),
      onPressed: () => showReportBugSheet(context, currentRoute: routeName),
    );
  }
}
