import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:flutter_facebook_auth/flutter_facebook_auth.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/extension_token_service.dart';
import 'package:draftright_mobile/services/logger_service.dart';

class AuthService extends ChangeNotifier {
  static const _keyAccess = 'draftright.accessToken';
  static const _keyRefresh = 'draftright.refreshToken';
  // Note: shared_preferences plugin auto-prefixes 'flutter.' to all keys.
  // Storing as 'draftright.accessToken' actually persists as 'flutter.draftright.accessToken'
  // which is exactly what the Android keyboard's SharedSettings reads.
  static const _sharedKeyAccess = 'draftright.accessToken';
  static const _appGroupChannel = MethodChannel('com.draftright.v2/app_group');

  final FlutterSecureStorage _secure = const FlutterSecureStorage();

  String? _accessToken;
  String? _refreshToken;
  String _baseUrl = 'http://localhost:3000';
  late final ExtensionTokenService _extension =
      ExtensionTokenService(baseUrl: _baseUrl);

  bool get isLoggedIn => _accessToken != null && _accessToken!.isNotEmpty;
  String? get accessToken => _accessToken;

  /// Called by SettingsService when backendUrl changes.
  void setBaseUrl(String url) {
    _baseUrl = url;
    _extension.baseUrl = url;
  }

  Future<void> init(String baseUrl) async {
    _baseUrl = baseUrl;
    try {
      _accessToken = await _secure.read(key: _keyAccess);
      _refreshToken = await _secure.read(key: _keyRefresh);
    } catch (_) {
      _accessToken = null;
      _refreshToken = null;
    }
    // Sync to SharedPreferences for keyboard extension
    if (_accessToken != null && _accessToken!.isNotEmpty) {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_sharedKeyAccess, _accessToken!);
      await _syncToAppGroup('draftright.accessToken', _accessToken!);
      // Ensure the long-lived extension token exists for the IME / share
      // extension. Users who upgraded into the EXTTOK build without
      // re-logging in won't have one yet — without this, the IME falls
      // back to the 15-min access JWT and shows "Unauthorized" the moment
      // it expires. Best-effort: failures don't block app startup.
      unawaited(_extension.ensureMinted(accessToken: _accessToken!));
    }
    notifyListeners();
  }

  Future<void> login(String email, String password) async {
    final uri = Uri.parse('$_baseUrl/auth/login');
    final normalizedEmail = email.trim().toLowerCase();
    try {
      final response = await http
          .post(uri,
              headers: {'Content-Type': 'application/json'},
              body: jsonEncode({'email': normalizedEmail, 'password': password}))
          .timeout(const Duration(seconds: 15));

      if (response.statusCode >= 400) {
        final body = _tryDecodeError(response.body);
        throw Exception(body);
      }

      final data = jsonDecode(response.body) as Map<String, dynamic>;
      await _storeTokens(data['access_token'] as String, data['refresh_token'] as String);
      DRLogger.log('Login success: $email', category: 'AUTH');
    } catch (e) {
      DRLogger.error('Login failed: $e', category: 'AUTH');
      rethrow;
    }
  }

  Future<void> register(String name, String email, String password) async {
    final uri = Uri.parse('$_baseUrl/auth/register');
    try {
      final response = await http
          .post(uri,
              headers: {'Content-Type': 'application/json'},
              body: jsonEncode({'name': name, 'email': email, 'password': password}))
          .timeout(const Duration(seconds: 15));

      if (response.statusCode >= 400) {
        final body = _tryDecodeError(response.body);
        throw Exception(body);
      }

      final data = jsonDecode(response.body) as Map<String, dynamic>;
      await _storeTokens(data['access_token'] as String, data['refresh_token'] as String);
      DRLogger.log('Register success: $email', category: 'AUTH');
    } catch (e) {
      DRLogger.error('Register failed: $e', category: 'AUTH');
      rethrow;
    }
  }

  // --- Social Login ---

  Future<void> signInWithGoogle() async {
    final googleSignIn = GoogleSignIn(scopes: ['email', 'profile']);
    final account = await googleSignIn.signIn();
    if (account == null) throw Exception('Google sign-in cancelled');

    final auth = await account.authentication;
    final idToken = auth.idToken;
    if (idToken == null) throw Exception('Failed to get Google ID token');

    await _socialLogin('google', idToken, name: account.displayName, email: account.email, avatarUrl: account.photoUrl);
  }

  Future<void> signInWithFacebook() async {
    final result = await FacebookAuth.instance.login(permissions: ['email', 'public_profile']);
    if (result.status != LoginStatus.success) {
      throw Exception(result.message ?? 'Facebook sign-in failed');
    }

    final accessToken = result.accessToken!.tokenString;
    final userData = await FacebookAuth.instance.getUserData(fields: 'name,email,picture.type(large)');

    await _socialLogin('facebook', accessToken, name: userData['name'], email: userData['email'], avatarUrl: userData['picture']?['data']?['url']);
  }

  Future<void> signInWithTikTok() async {
    // TikTok Login Kit requires native SDK integration.
    // For now, show a message that it's coming soon.
    throw Exception('TikTok sign-in coming soon');
  }

  Future<void> _socialLogin(String provider, String idToken, {String? name, String? email, String? avatarUrl}) async {
    final uri = Uri.parse('$_baseUrl/auth/social');
    final response = await http
        .post(uri,
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'provider': provider,
              'id_token': idToken,
              'name': name,
              'email': email,
              'avatar_url': avatarUrl,
            }))
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      final body = _tryDecodeError(response.body);
      throw Exception(body);
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    await _storeTokens(data['access_token'] as String, data['refresh_token'] as String);
  }

  Future<void> logout() async {
    _accessToken = null;
    _refreshToken = null;
    await _secure.delete(key: _keyAccess);
    await _secure.delete(key: _keyRefresh);
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_sharedKeyAccess);
    await _syncToAppGroup('draftright.accessToken', null);
    // Clear the extension token from SharedPreferences and (on iOS) App
    // Group keychain so the extensions can no longer authenticate.
    // Server-side revoke is a follow-up — for now the row stays active
    // until 90-day idle expiry; on next login we re-mint and the old row
    // is revoked via the (user_id, device_id) partial unique index.
    await _extension.clearToken();
    notifyListeners();
  }

  /// Returns valid access token, auto-refreshing if needed.
  Future<String> getAccessToken() async {
    if (_accessToken == null || _accessToken!.isEmpty) {
      throw Exception('Not logged in');
    }
    // Attempt refresh if token looks expired (naive check — backend will 401 anyway)
    return _accessToken!;
  }

  Future<void> changePassword(String currentPassword, String newPassword) async {
    final token = await getAccessToken();
    final uri = Uri.parse('$_baseUrl/auth/change-password');
    final response = await http
        .post(uri,
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer $token',
            },
            body: jsonEncode({
              'current_password': currentPassword,
              'new_password': newPassword,
            }))
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      final body = _tryDecodeError(response.body);
      throw Exception(body);
    }
  }

  /// Called after a 401 response to refresh the token.
  Future<bool> tryRefresh() async {
    if (_refreshToken == null || _refreshToken!.isEmpty) return false;
    try {
      final uri = Uri.parse('$_baseUrl/auth/refresh');
      final response = await http
          .post(uri,
              headers: {'Content-Type': 'application/json'},
              body: jsonEncode({'refresh_token': _refreshToken}))
          .timeout(const Duration(seconds: 15));

      if (response.statusCode >= 400) return false;

      final data = jsonDecode(response.body) as Map<String, dynamic>;
      await _storeTokens(data['access_token'] as String, data['refresh_token'] as String);
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<void> _storeTokens(String access, String refresh) async {
    _accessToken = access;
    _refreshToken = refresh;
    await _secure.write(key: _keyAccess, value: access);
    await _secure.write(key: _keyRefresh, value: refresh);
    // Sync access token to SharedPreferences for keyboard extension
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_sharedKeyAccess, access);
    await _syncToAppGroup('draftright.accessToken', access);
    notifyListeners();
    // Mint or rotate the long-lived extension token in the background.
    // Failures here must not block login — the extensions fall back to
    // the access JWT path via SharedSettings.bearerToken until a future
    // mint succeeds.
    unawaited(_extension.ensureMinted(accessToken: access));
  }

  /// Sync a key/value to iOS App Group UserDefaults for keyboard extension
  /// access. The native handler lives on the implicit-engine init hook,
  /// which can register AFTER this fires on cold start — so retry a few
  /// times before giving up, otherwise the keyboard never sees the token
  /// and rewrite 401s while the in-app Playground works.
  static Future<bool> _syncToAppGroup(String key, String? value) async {
    if (!Platform.isIOS) return true;
    for (var attempt = 0; attempt < 6; attempt++) {
      try {
        await _appGroupChannel.invokeMethod('set', {'key': key, 'value': value});
        return true;
      } catch (_) {
        await Future.delayed(const Duration(milliseconds: 500));
      }
    }
    return false;
  }

  /// Sync backend URL to App Group (called from SettingsService).
  static Future<void> syncBackendUrlToAppGroup(String url) async {
    await _syncToAppGroup('draftright.backendUrl', url);
  }

  /// Sync the enabled keyboard language IDs to App Group as a JSON-encoded
  /// string. The iOS keyboard extension decodes via JSONSerialization.
  static Future<void> syncEnabledLanguageIdsToAppGroup(List<String> ids) async {
    await _syncToAppGroup('draftright.enabledLanguageIds', jsonEncode(ids));
  }

  /// Sync the active keyboard language ID to App Group.
  static Future<void> syncActiveLanguageIdToAppGroup(String id) async {
    await _syncToAppGroup('draftright.activeLanguageId', id);
  }

  String _tryDecodeError(String body) {
    try {
      final data = jsonDecode(body) as Map<String, dynamic>;
      return (data['message'] ?? data['error'] ?? body).toString();
    } catch (_) {
      return body;
    }
  }
}
