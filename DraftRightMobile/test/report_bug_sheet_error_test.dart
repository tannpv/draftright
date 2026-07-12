// Widget test: a failed bug-report submit must show a VISIBLE in-sheet error
// banner, not a snackbar. Regression guard for issue #68 — the snackbar
// rendered behind the modal sheet, so a failed submit looked like nothing
// happened.
//
// Note: TestWidgetsFlutterBinding blocks real network — every HttpClient
// request resolves to 400 with no body. That's exactly a server rejection, so
// it drives the failure path without a stub. We assert the banner appears (not
// its exact text) and that the sheet stays open, which is the behaviour #68
// was missing.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

void main() {
  testWidgets('failed submit shows a visible error banner, sheet stays open',
      (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Builder(
          builder: (context) => Center(
            child: ElevatedButton(
              onPressed: () => showReportBugSheet(context,
                  endpointOverride: 'http://127.0.0.1:1/bug-reports'),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    ));

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    // Anonymous (no AuthService in scope): description + email fields.
    final fields = find.byType(TextFormField);
    await tester.enterText(fields.at(0), 'The rewrite button does nothing.');
    await tester.enterText(fields.at(1), 'reporter@example.com');

    await tester.tap(find.widgetWithText(FilledButton, 'Submit'));
    await tester.pump(); // enter the _submitting state

    // The faked 400 resolves on the real event loop — only runAsync advances
    // it under the automated test binding.
    await tester.runAsync(() async {
      await Future<void>.delayed(const Duration(milliseconds: 500));
    });
    await tester.pumpAndSettle();

    // A visible error banner is present...
    final banner = find.byIcon(Icons.error_outline);
    expect(banner, findsOneWidget);
    // ...NOT a snackbar (which would be hidden behind the modal sheet).
    expect(find.byType(SnackBar), findsNothing);
    // ...and the sheet did NOT close — the user keeps their typed text.
    expect(find.widgetWithText(FilledButton, 'Submit'), findsOneWidget);
    expect(find.text('The rewrite button does nothing.'), findsOneWidget);
  });
}
