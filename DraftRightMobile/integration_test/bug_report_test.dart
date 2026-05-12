// Integration test for the "Report a bug" sheet — drives the real widget
// on a connected device/simulator (intended for the iOS simulator) and
// submits to a local stub HTTP server instead of production.
//
// Run:
//   flutter test integration_test/bug_report_test.dart -d <ios-sim-id>
//
// Covers: validation guard (description < 10 chars), successful submission
// with the correct multipart payload (description / source / user_email),
// and the success/failure snackbars.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:provider/provider.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  /// Expected `source` field for whichever platform the test is running on.
  final expectedSource = Platform.isIOS ? 'ios-app' : 'android-app';

  late HttpServer server;
  late String endpoint;
  int statusToReturn = 201;
  Completer<String>? bodyReceived;

  setUp(() async {
    statusToReturn = 201;
    bodyReceived = Completer<String>();
    server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    endpoint = 'http://127.0.0.1:${server.port}/bug-reports';
    server.listen((HttpRequest req) async {
      // The body is multipart/form-data; text fields appear verbatim in the
      // raw bytes, which is all the assertions below need.
      final body = await utf8.decoder.bind(req).join();
      if (!(bodyReceived?.isCompleted ?? true)) bodyReceived!.complete(body);
      req.response.statusCode = statusToReturn;
      req.response.headers.contentType = ContentType.json;
      req.response.write('{"id":"stub-id"}');
      await req.response.close();
    });
  });

  tearDown(() async {
    await server.close(force: true);
  });

  /// Pumps a minimal app whose only screen has a button that opens the bug
  /// sheet pointed at the local stub. AuthService is constructed fresh, so
  /// the user is logged out (email field is required).
  Future<void> pumpHarness(WidgetTester tester) async {
    await tester.pumpWidget(
      ChangeNotifierProvider<AuthService>(
        create: (_) => AuthService(),
        child: MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => showReportBugSheet(
                    context,
                    currentRoute: 'IntegrationTestRoute',
                    endpointOverride: endpoint,
                  ),
                  child: const Text('Open bug sheet'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Open bug sheet'));
    await tester.pumpAndSettle();
    expect(find.text('Report a bug'), findsOneWidget);
  }

  /// Pumps in short bursts until [finder] reaches [present]-ness, or the
  /// timeout elapses — pumpAndSettle would stall on the SnackBar's 4s
  /// auto-dismiss timer (and the modal's dismiss animation).
  Future<void> pumpUntil(WidgetTester tester, Finder finder,
      {bool present = true,
      Duration timeout = const Duration(seconds: 10)}) async {
    final deadline = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(deadline)) {
      await tester.pump(const Duration(milliseconds: 100));
      if (finder.evaluate().isNotEmpty == present) return;
    }
    fail('Timed out waiting for $finder to be ${present ? "present" : "gone"}');
  }

  testWidgets('rejects a description shorter than 10 chars', (tester) async {
    await pumpHarness(tester);

    await tester.enterText(find.byType(TextFormField).first, 'too short');
    await tester.enterText(find.byType(TextFormField).last, 'tester@example.com');
    await tester.tap(find.widgetWithText(FilledButton, 'Submit'));
    await tester.pumpAndSettle();

    expect(find.text('Please add at least 10 characters.'), findsOneWidget);
    expect(find.text('Report a bug'), findsOneWidget); // sheet still open
    expect(bodyReceived!.isCompleted, isFalse); // nothing sent
  });

  testWidgets('submits a valid report with the correct payload',
      (tester) async {
    await pumpHarness(tester);

    const description = 'The rewrite button does nothing on the iOS app today.';
    await tester.enterText(find.byType(TextFormField).first, description);
    await tester.enterText(find.byType(TextFormField).last, 'tester@example.com');
    await tester.tap(find.widgetWithText(FilledButton, 'Submit'));
    await tester.pump(); // kick off async submit

    final body = await bodyReceived!.future.timeout(const Duration(seconds: 10));
    expect(body, contains('name="description"'));
    expect(body, contains(description));
    expect(body, contains('name="source"'));
    expect(body, contains(expectedSource));
    expect(body, contains('name="user_email"'));
    expect(body, contains('tester@example.com'));
    // currentRoute is threaded into the context JSON.
    expect(body, contains('IntegrationTestRoute'));

    await pumpUntil(tester, find.text("Thanks! We'll look into it."));
    // The modal dismiss animation runs in parallel with the snackbar; wait
    // for the sheet to finish leaving the tree.
    await pumpUntil(tester, find.text('Report a bug'), present: false);
  });

  testWidgets('shows an error snackbar when the backend rejects', (tester) async {
    statusToReturn = 500;
    await pumpHarness(tester);

    await tester.enterText(
        find.byType(TextFormField).first, 'Detailed enough description here.');
    await tester.enterText(find.byType(TextFormField).last, 'tester@example.com');
    await tester.tap(find.widgetWithText(FilledButton, 'Submit'));
    await tester.pump();

    await pumpUntil(
        tester, find.textContaining('Could not submit bug report'));
    expect(find.text('Report a bug'), findsOneWidget); // sheet stays open
  });
}
