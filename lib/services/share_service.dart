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

  /// Subscribe to in-flight shares — fired when the user shares to
  /// DraftRight while the app is already foregrounded or backgrounded.
  /// Pass `null` to clear.
  static void setHandler(void Function(String text)? handler) {
    if (handler == null) {
      _channel.setMethodCallHandler(null);
      return;
    }
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'onSharedText' && call.arguments is String) {
        final text = (call.arguments as String).trim();
        if (text.isNotEmpty) handler(text);
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
}
