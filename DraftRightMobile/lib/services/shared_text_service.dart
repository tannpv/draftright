import 'package:flutter/services.dart';

/// Receives plain text shared into the app via Android's ACTION_SEND share
/// sheet (Teams / Zalo / SMS / …), bridged by MainActivity over the native
/// `draftright/share` channel (#143). iOS (share extension) is a separate
/// follow-up. Off Android the channel is absent → calls no-op to null.
class SharedTextService {
  static const _channel = MethodChannel('draftright/share');

  /// Text the app was cold-launched with (consumed once on the native side),
  /// or null if it wasn't launched from a share.
  static Future<String?> initialSharedText() async {
    try {
      return await _channel.invokeMethod<String>('getSharedText');
    } catch (_) {
      return null; // channel not available (iOS/desktop) or no pending text
    }
  }

  /// Register a handler for text shared while the app is already running
  /// (warm start — MainActivity pushes `sharedText`).
  static void onSharedText(void Function(String text) handler) {
    _channel.setMethodCallHandler((call) async {
      if (call.method == 'sharedText') {
        final t = call.arguments as String?;
        if (t != null && t.trim().isNotEmpty) handler(t);
      }
      return null;
    });
  }
}
