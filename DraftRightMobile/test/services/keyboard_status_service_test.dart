import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/services/keyboard_status_service.dart';

/// #272: after a fresh install the keyboard is installed but disabled, and the
/// app has to be able to see that to offer the fix.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel(KeyboardStatusService.channelName);
  final calls = <MethodCall>[];

  /// Answers `keyboardStatus` with [status]; other methods return null.
  void stub(Object? status, {Object? throwing}) {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      calls.add(call);
      if (throwing != null) throw throwing;
      return call.method == 'keyboardStatus' ? status : null;
    });
  }

  setUp(calls.clear);

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  KeyboardStatusService service() =>
      KeyboardStatusService(channel: channel, isAndroid: true);

  test('maps each native status name', () async {
    for (final entry in {
      'NOT_ENABLED': KeyboardStatus.notEnabled,
      'ENABLED_NOT_SELECTED': KeyboardStatus.enabledNotSelected,
      'ACTIVE': KeyboardStatus.active,
    }.entries) {
      stub(entry.key);
      expect(await service().status(), entry.value, reason: entry.key);
    }
  });

  test('an unrecognised name is unknown rather than a crash', () async {
    stub('SOMETHING_NEW');
    expect(await service().status(), KeyboardStatus.unknown);
  });

  test('platform failure is unknown, never thrown', () async {
    stub(null, throwing: PlatformException(code: 'boom'));
    expect(await service().status(), KeyboardStatus.unknown);
  });

  test('a build without the channel method is unknown', () async {
    stub(null, throwing: MissingPluginException('no impl'));
    expect(await service().status(), KeyboardStatus.unknown);
  });

  test('iOS is unknown and never calls the channel', () async {
    stub('NOT_ENABLED');
    final ios = KeyboardStatusService(channel: channel, isAndroid: false);
    expect(await ios.status(), KeyboardStatus.unknown);
    await ios.openKeyboardSettings();
    expect(calls, isEmpty);
  });

  test('the fix actions reach the platform', () async {
    stub(null);
    await service().openKeyboardSettings();
    await service().openKeyboardPicker();
    expect(
      calls.map((c) => c.method),
      ['openKeyboardSettings', 'openKeyboardPicker'],
    );
  });

  test('a failing fix action does not throw at the caller', () async {
    stub(null, throwing: PlatformException(code: 'no activity'));
    await expectLater(service().openKeyboardSettings(), completes);
  });
}
