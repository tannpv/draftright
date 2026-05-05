import 'package:flutter/services.dart';

/// Bridge to the Android side's ACTION_SEND handler.  iOS wires up its own
/// share extension separately; on iOS this service is a no-op.
class ShareService {
  static const _channel = MethodChannel('draftright/share');

  /// Drain any text the user shared while the app was not running.
  /// Returns null if there's no pending share.  Idempotent — once read,
  /// the native side clears its buffer.
  static Future<String?> getInitialSharedText() async {
    try {
      return await _channel.invokeMethod<String>('getInitialSharedText');
    } on MissingPluginException {
      return null; // platform without share channel (iOS today, web, desktop)
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
}
