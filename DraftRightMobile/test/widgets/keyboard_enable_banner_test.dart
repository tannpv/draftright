import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/keyboard_status_service.dart';
import 'package:draftright_mobile/widgets/keyboard_enable_banner.dart';

/// #272: the banner is the only thing telling a fresh-install user that their
/// keyboard is switched off, so it must appear exactly when it should.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel(KeyboardStatusService.channelName);
  late List<MethodCall> calls;

  void stub(String status) {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call);
      return call.method == 'keyboardStatus' ? status : null;
    });
  }

  setUp(() => calls = <MethodCall>[]);

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  Future<void> pump(WidgetTester tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: KeyboardEnableBanner(
          service: KeyboardStatusService(channel: channel, isAndroid: true),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('prompts to enable when the keyboard is off', (tester) async {
    stub('NOT_ENABLED');
    await pump(tester);
    expect(find.text('Turn on the DraftRight keyboard'), findsOneWidget);
    expect(find.text('Open keyboard settings'), findsOneWidget);
  });

  testWidgets('prompts to switch when enabled but not selected',
      (tester) async {
    stub('ENABLED_NOT_SELECTED');
    await pump(tester);
    expect(find.text('Switch to the DraftRight keyboard'), findsOneWidget);
    expect(find.text('Choose keyboard'), findsOneWidget);
  });

  testWidgets('renders nothing when the keyboard is active', (tester) async {
    stub('ACTIVE');
    await pump(tester);
    expect(find.byType(Card), findsNothing);
  });

  testWidgets('renders nothing when the platform cannot say', (tester) async {
    stub('SOMETHING_ELSE');
    await pump(tester);
    expect(find.byType(Card), findsNothing);
  });

  testWidgets('the button opens the right system screen', (tester) async {
    stub('NOT_ENABLED');
    await pump(tester);
    await tester.tap(find.text('Open keyboard settings'));
    await tester.pumpAndSettle();
    expect(calls.map((c) => c.method), contains('openKeyboardSettings'));
  });

  testWidgets('re-checks after returning from settings', (tester) async {
    stub('NOT_ENABLED');
    await pump(tester);
    expect(find.byType(Card), findsOneWidget);

    // User enabled it while away; coming back must clear the banner.
    stub('ACTIVE');
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pumpAndSettle();
    expect(find.byType(Card), findsNothing);
  });
}
