import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';

/// Bridge to the Android floating-bubble + overlay/accessibility permissions.
/// iOS wires up its own share extension separately; on iOS these are no-ops.
class ShareService {
  static const _channel = MethodChannel('draftright/share');

  /// The floating bubble needs `SYSTEM_ALERT_WINDOW` (draw over other apps)
  /// plus an `AccessibilityService` to read/replace the focused field — both
  /// Android-only. iOS has no cross-app overlay (sandbox), so the bubble UI
  /// must never surface there. UI gates on this, never on `Platform` directly.
  static bool get supportsFloatingBubble => !kIsWeb && Platform.isAndroid;

  /// iOS has no bubble, but its custom keyboard has hold-to-talk voice
  /// dictation whose AI-polish tone is user-configurable (read from the App
  /// Group). Gates the iOS "Voice dictation" settings section.
  static bool get supportsKeyboardVoice => !kIsWeb && Platform.isIOS;

  // ── Floating bubble (Tier 1) ───────────────────────────────────────────

  /// True iff `Settings.canDrawOverlays(this)` returns true on Android.
  /// On iOS / desktop / web, returns false (no equivalent permission).
  static Future<bool> canDrawOverlays() async {
    try {
      return await _channel.invokeMethod<bool>('canDrawOverlays') ?? false;
    } catch (_) {
      return false;
    }
  }

  /// Launch Settings → "Display over other apps" for the user to grant.
  /// No-op on iOS / desktop / web.
  static Future<void> openOverlaySettings() async {
    try {
      await _channel.invokeMethod<void>('openOverlaySettings');
    } catch (_) {/* swallow */}
  }

  /// Start the floating-bubble foreground service.  Throws if the user
  /// hasn't granted overlay permission yet — caller should check
  /// [canDrawOverlays] first.
  static Future<bool> startBubble() async {
    try {
      return await _channel.invokeMethod<bool>('startBubble') ?? false;
    } on PlatformException catch (e) {
      if (e.code == 'NO_PERMISSION') return false;
      rethrow;
    } catch (_) {
      return false;
    }
  }

  /// Stop the floating-bubble service.
  static Future<bool> stopBubble() async {
    try {
      return await _channel.invokeMethod<bool>('stopBubble') ?? false;
    } catch (_) {
      return false;
    }
  }

  /// Launch system Accessibility settings so the user can enable the
  /// in-place rewrite service. No-op on iOS / desktop / web.
  static Future<void> openAccessibilitySettings() async {
    try {
      await _channel.invokeMethod<void>('openAccessibilitySettings');
    } catch (_) {/* swallow */}
  }

  /// True iff the AccessibilityService backing in-place rewrite is enabled
  /// and bound. False on iOS / desktop / web.
  static Future<bool> isInPlaceRewriteReady() async {
    try {
      return await _channel.invokeMethod<bool>('isInPlaceRewriteReady') ?? false;
    } catch (_) {
      return false;
    }
  }
}
