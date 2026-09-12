import 'dart:io';

import 'package:flutter/services.dart';

/// Whether the DraftRight keyboard is usable (#272).
///
/// Mirrors the Kotlin `KeyboardStatus` enum; the decision is made natively so
/// there is one source of truth for it.
enum KeyboardStatus {
  /// Installed but not in the system's enabled list — needs system Settings.
  notEnabled,

  /// Enabled but another keyboard is selected — needs the input-method picker.
  enabledNotSelected,

  /// Enabled and selected; nothing to prompt.
  active,

  /// Platform can't answer (iOS, or an older build without the channel).
  /// Treated as "say nothing" so we never nag on a platform we can't read.
  unknown,
}

/// Reads whether our keyboard is enabled, and opens the screens that fix it.
///
/// Android never auto-enables a newly installed input method — an IME sees
/// everything typed, so enabling it is an explicit user decision. Every fresh
/// install therefore leaves the keyboard present but dead (#272), and without
/// this the app gives no sign of it.
class KeyboardStatusService {
  KeyboardStatusService({MethodChannel? channel, bool? isAndroid})
      : _channel = channel ?? const MethodChannel(channelName),
        _isAndroid = isAndroid ?? Platform.isAndroid;

  /// Same channel the shared pack dir uses — one bridge, not two.
  static const channelName = 'draftright/share';

  final MethodChannel _channel;
  final bool _isAndroid;

  static const _statusByNativeName = <String, KeyboardStatus>{
    'NOT_ENABLED': KeyboardStatus.notEnabled,
    'ENABLED_NOT_SELECTED': KeyboardStatus.enabledNotSelected,
    'ACTIVE': KeyboardStatus.active,
  };

  /// Current status, or [KeyboardStatus.unknown] when the platform can't say.
  ///
  /// Never throws: this drives a passive banner, and a failure to read the
  /// status must not break the screen hosting it.
  Future<KeyboardStatus> status() async {
    if (!_isAndroid) return KeyboardStatus.unknown;
    try {
      final name = await _channel.invokeMethod<String>('keyboardStatus');
      return _statusByNativeName[name] ?? KeyboardStatus.unknown;
    } on PlatformException {
      return KeyboardStatus.unknown;
    } on MissingPluginException {
      return KeyboardStatus.unknown;
    }
  }

  /// Opens the system input-method settings, where the keyboard is switched on.
  Future<void> openKeyboardSettings() =>
      _invokeIgnoringFailure('openKeyboardSettings');

  /// Opens the input-method picker, where an enabled keyboard is selected.
  Future<void> openKeyboardPicker() =>
      _invokeIgnoringFailure('openKeyboardPicker');

  Future<void> _invokeIgnoringFailure(String method) async {
    if (!_isAndroid) return;
    try {
      await _channel.invokeMethod<void>(method);
    } on PlatformException {
      // Nothing useful to tell the user: the button simply does nothing rather
      // than throwing up an error over a settings screen that failed to open.
    } on MissingPluginException {
      // Older build without the channel method.
    }
  }
}
