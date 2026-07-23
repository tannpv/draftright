import 'package:flutter/services.dart';

/// Bridge to the Android side's ACTION_SEND handler + floating-bubble
/// service. iOS wires up its own share extension separately; on iOS the
/// bubble methods are no-ops.
class ShareService {
  static const _channel = MethodChannel('draftright/share');

  // ── Share intent ────────────────────────────────────────────────────────

  /// Drain any text the user shared while the app was not running.
  /// Returns null if there's no pending share.  Idempotent — once read,
  /// the native side clears its buffer.
  static Future<String?> getInitialSharedText() async {
    try {
      return await _channel.invokeMethod<String>('getInitialSharedText');
    } on MissingPluginException {
      return null;
    } catch (_) {
      return null;
    }
  }

  /// Subscribe to in-flight shares + bubble events.
  /// Pass `null` to clear.
  static void setHandler({
    void Function(String text)? onSharedText,
    void Function()? onBubbleEmptyClipboard,
  }) {
    if (onSharedText == null && onBubbleEmptyClipboard == null) {
      _channel.setMethodCallHandler(null);
      return;
    }
    _channel.setMethodCallHandler((call) async {
      switch (call.method) {
        case 'onSharedText':
          final text = (call.arguments as String?)?.trim() ?? '';
          if (text.isNotEmpty) onSharedText?.call(text);
          break;
        case 'onBubbleEmptyClipboard':
          onBubbleEmptyClipboard?.call();
          break;
      }
    });
  }

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

  /// Send DraftRight to the back of the task stack so the previous
  /// foreground app comes back. Called after a successful rewrite so the
  /// user doesn't have to navigate back manually before pasting.
  static Future<void> dismissToBackground() async {
    try {
      await _channel.invokeMethod<void>('dismissToBackground');
    } catch (_) {/* swallow */}
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
