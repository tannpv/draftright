import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/services/keyboard_status_service.dart';

/// #272 follow-up: the onboarding keyboard page listed four levels of system
/// navigation to follow by hand. Step 1 is now a button.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel(KeyboardStatusService.channelName);
  late List<MethodCall> calls;

  setUp(() {
    calls = <MethodCall>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call);
      return null;
    });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  /// The button, but only once it is the page actually on screen.
  ///
  /// PageView keeps its neighbouring pages built and offstage, so a plain
  /// `find.text` matches a copy that ignores pointers — tapping that silently
  /// does nothing. `hitTestable()` is what distinguishes "built" from "live".
  /// The keyboard page is index 2 of four (welcome, process-text, keyboard,
  /// login), so page forward twice rather than searching by text: PageView
  /// keeps neighbouring pages built but offstage, and a text finder happily
  /// matches one of those — tapping it silently does nothing.
  const keyboardPageIndex = 2;

  Future<void> openKeyboardPage(WidgetTester tester) async {
    await tester.pumpWidget(MaterialApp(
      home: OnboardingScreen(
        onComplete: () {},
        keyboardStatusService:
            KeyboardStatusService(channel: channel, isAndroid: true),
      ),
    ));
    await tester.pumpAndSettle();

    for (var i = 0; i < keyboardPageIndex; i++) {
      await tester.tap(find.text('Next'));
      await tester.pumpAndSettle();
    }
    // The page scrolls; the button sits below the fold on a small test surface.
    await tester.ensureVisible(find.text('Open keyboard settings'));
    await tester.pumpAndSettle();
  }

  testWidgets('the keyboard page offers a button, not just instructions',
      (tester) async {
    await openKeyboardPage(tester);
    expect(find.text('DraftRight Keyboard'), findsOneWidget,
        reason: 'should have reached the keyboard page');
    expect(find.text('Open keyboard settings').hitTestable(), findsOneWidget);
  });

  testWidgets('tapping it opens the system keyboard settings', (tester) async {
    await openKeyboardPage(tester);
    await tester.tap(find.text('Open keyboard settings').hitTestable());
    await tester.pumpAndSettle();
    expect(calls.map((c) => c.method), contains('openKeyboardSettings'));
  });

  testWidgets('the written steps stay for anyone who prefers them',
      (tester) async {
    await openKeyboardPage(tester);
    expect(find.text('Open Settings'), findsOneWidget);
    expect(find.text('Enable "DraftRight"'), findsOneWidget);
  });
}
