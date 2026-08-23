import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;

/// Platform-capability gates for keyboard/share features.
///
/// The Android floating bubble (overlay + AccessibilityService) was removed —
/// banking and other security-sensitive apps block overlays / accessibility,
/// and it required a Play accessibility-service declaration. In-place rewrite
/// now lives only in the keyboard (⚡ one-tap) and the Process-Text / share
/// path, both of which are sandbox-safe and work everywhere.
class ShareService {
  /// Both the iOS and Android custom keyboards have hold-to-talk voice dictation
  /// (#64/#75): the dictation tone + the polish-on/off toggle (#197). Gates the
  /// "Voice dictation" settings section — it must show wherever the voice
  /// keyboard runs, not iOS only. iOS reads these from the App Group, Android
  /// from SharedPreferences.
  static bool get supportsKeyboardVoice =>
      !kIsWeb && (Platform.isIOS || Platform.isAndroid);
}
