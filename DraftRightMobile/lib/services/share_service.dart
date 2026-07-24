import 'package:flutter/services.dart';

/// Bridge to the Android floating-bubble + overlay/accessibility permissions.
/// iOS wires up its own share extension separately; on iOS these are no-ops.
class ShareService {
  static const _channel = MethodChannel('draftright/share');

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
