import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;
import 'dart:math';

import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import 'package:draftright_mobile/services/logger_service.dart';

/// Manages the long-lived `dr_ext_*` extension token used by the iOS
/// keyboard, iOS share extension, and Android keyboard so they can call
/// /rewrite without sharing the user's session JWT or trying to refresh
/// short-lived access tokens themselves.
///
/// The main app calls [ensureMinted] after login to mint or rotate a
/// token tied to a stable per-install device id. The token is persisted:
///   - SharedPreferences key `flutter.draftright.extensionToken`
///     (read by the Android keyboard via FlutterSharedPreferences
///     using key `flutter.draftright.extensionToken`).
///   - On iOS, also synced to App Group keychain via the
///     com.draftright.v2/app_group MethodChannel's `setKeychain` /
///     `deleteKeychain` methods (read by the iOS keyboard and share
///     extensions through SharedKeychain.get).
class ExtensionTokenService {
  ExtensionTokenService({required String baseUrl}) : _baseUrl = baseUrl;

  static const _channel = MethodChannel('com.draftright.v2/app_group');

  /// Stable per-install identifier. Generated once on first call and
  /// kept thereafter; rotation happens by re-minting the token under the
  /// same device id.
  static const _kDeviceId = 'draftright.deviceId';

  /// Key used by SharedKeychain (iOS) and SharedPreferences (Android +
  /// fallback) for the long-lived token. SharedPreferences auto-prefixes
  /// 'flutter.' so the on-disk key is `flutter.draftright.extensionToken`.
  static const _kExtensionToken = 'draftright.extensionToken';

  String _baseUrl;
  set baseUrl(String url) {
    _baseUrl = url;
  }

  /// Returns the per-install device id, generating and persisting one on
  /// first call. UUID v4 format.
  Future<String> deviceId() async {
    final prefs = await SharedPreferences.getInstance();
    final existing = prefs.getString(_kDeviceId);
    if (existing != null && existing.isNotEmpty) return existing;
    final id = _uuidv4();
    await prefs.setString(_kDeviceId, id);
    return id;
  }

  /// Mint or rotate the extension token using the user's session JWT.
  /// Failures are logged and swallowed: the calling code (typically
  /// AuthService._storeTokens) treats this as best-effort and the
  /// extensions fall back to the access JWT path until a future call
  /// succeeds.
  Future<void> ensureMinted({required String accessToken}) async {
    final id = await deviceId();
    final name = _deviceName();

    try {
      final response = await http
          .post(
            Uri.parse('$_baseUrl/auth/extension-tokens'),
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer $accessToken',
            },
            body: jsonEncode({'device_id': id, 'device_name': name}),
          )
          .timeout(const Duration(seconds: 15));

      if (response.statusCode >= 400) {
        DRLogger.error(
          'Mint extension token failed: ${response.statusCode} ${response.body}',
          category: 'AUTH',
        );
        return;
      }

      final data = jsonDecode(response.body) as Map<String, dynamic>;
      final token = data['token'] as String;
      await storeToken(token);
      DRLogger.log('Extension token minted and stored', category: 'AUTH');
    } catch (e) {
      DRLogger.error('Mint extension token errored: $e', category: 'AUTH');
    }
  }

  /// Persist the token to SharedPreferences (Android + fallback) and
  /// iOS App Group keychain.
  Future<void> storeToken(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kExtensionToken, token);
    await _syncToKeychain(_kExtensionToken, token);
  }

  /// Clear the token from all storage. Called on logout. Server-side
  /// revocation is a separate concern handled by AuthService.logout via
  /// the (future) /auth/extension-tokens DELETE call.
  Future<void> clearToken() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_kExtensionToken);
    await _syncToKeychain(_kExtensionToken, null);
  }

  Future<void> _syncToKeychain(String key, String? value) async {
    if (!Platform.isIOS) return;
    try {
      if (value == null) {
        await _channel.invokeMethod('deleteKeychain', {'key': key});
      } else {
        await _channel.invokeMethod('setKeychain', {'key': key, 'value': value});
      }
    } catch (_) {
      // Channel not available (web/desktop test runs).
    }
  }

  String _deviceName() {
    if (Platform.isIOS) return 'iOS';
    if (Platform.isAndroid) return 'Android';
    return 'Mobile';
  }

  String _uuidv4() {
    final rng = Random.secure();
    final bytes = List<int>.generate(16, (_) => rng.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant
    String hex(int b) => b.toRadixString(16).padLeft(2, '0');
    final s = bytes.map(hex).join();
    return '${s.substring(0, 8)}-${s.substring(8, 12)}-${s.substring(12, 16)}-${s.substring(16, 20)}-${s.substring(20)}';
  }
}
