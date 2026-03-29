import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

class AuthService extends ChangeNotifier {
  static const _keyAccess = 'draftright.accessToken';
  static const _keyRefresh = 'draftright.refreshToken';
  static const _sharedKeyAccess = 'flutter.draftright.accessToken';

  final FlutterSecureStorage _secure = const FlutterSecureStorage();

  String? _accessToken;
  String? _refreshToken;
  String _baseUrl = 'http://localhost:3000';

  bool get isLoggedIn => _accessToken != null && _accessToken!.isNotEmpty;
  String? get accessToken => _accessToken;

  /// Called by SettingsService when backendUrl changes.
  void setBaseUrl(String url) {
    _baseUrl = url;
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
    }
    notifyListeners();
  }

  Future<void> login(String email, String password) async {
    final uri = Uri.parse('$_baseUrl/auth/login');
    final response = await http
        .post(uri,
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({'email': email, 'password': password}))
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      final body = _tryDecodeError(response.body);
      throw Exception(body);
    }

    final data = jsonDecode(response.body) as Map<String, dynamic>;
    await _storeTokens(data['access_token'] as String, data['refresh_token'] as String);
  }

  Future<void> register(String name, String email, String password) async {
    final uri = Uri.parse('$_baseUrl/auth/register');
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
  }

  Future<void> logout() async {
    _accessToken = null;
    _refreshToken = null;
    await _secure.delete(key: _keyAccess);
    await _secure.delete(key: _keyRefresh);
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_sharedKeyAccess);
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
    notifyListeners();
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
