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
  /// iOS custom keyboard has hold-to-talk voice dictation whose AI-polish tone
  /// is user-configurable (read from the App Group). Gates the iOS
  /// "Voice dictation" settings section.
  static bool get supportsKeyboardVoice => !kIsWeb && Platform.isIOS;
}
