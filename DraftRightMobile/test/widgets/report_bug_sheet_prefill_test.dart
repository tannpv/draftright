import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/widgets/report_bug_sheet.dart';

/// Overrides just the two getters the report sheet reads for prefill, so we
/// avoid touching real secure-storage / social-sign-in plugins.
class _FakeAuth extends AuthService {
  _FakeAuth({required this.loggedIn, this.email});
  final bool loggedIn;
  final String? email;
  @override
  bool get isLoggedIn => loggedIn;
  @override
  String? get userEmail => email;
}

void main() {
  Future<void> openSheet(WidgetTester tester, AuthService auth) async {
    await tester.pumpWidget(
      ChangeNotifierProvider<AuthService>.value(
        value: auth,
        child: MaterialApp(
          home: Builder(
            builder: (ctx) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => showReportBugSheet(ctx),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets('pre-fills the email for a logged-in user', (tester) async {
    await openSheet(tester, _FakeAuth(loggedIn: true, email: 'me@example.com'));
    expect(find.widgetWithText(TextFormField, 'me@example.com'), findsOneWidget);
  });

  testWidgets('leaves the email empty when logged out', (tester) async {
    await openSheet(tester, _FakeAuth(loggedIn: false));
    // The field is present (now always shown) but carries no pre-filled text.
    expect(find.text('Your email'), findsOneWidget); // the label
    expect(find.widgetWithText(TextFormField, '@'), findsNothing);
  });
}
