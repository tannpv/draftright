import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'auth_service.dart';

// Production default. The Settings UI no longer exposes this — developers
// self-hosting can override at build time with:
//   flutter run --dart-define=DRAFTRIGHT_BACKEND_URL=http://localhost:3000
// The compile-time value takes precedence over any persisted SharedPreferences
// value, so a release build always points at production regardless of what an
// old install previously wrote to disk.
const String _kBuildBackendUrl = String.fromEnvironment(
  'DRAFTRIGHT_BACKEND_URL',
  defaultValue: 'https://api.draftright.info',
);
const String kDefaultBackendUrl = _kBuildBackendUrl;

/// Normalize a URL by trimming trailing slashes.
String _normalizeUrl(String url) {
  var u = url;
  while (u.endsWith('/')) {
    u = u.substring(0, u.length - 1);
  }
  return u;
}

class SettingsService extends ChangeNotifier {
  late SharedPreferences _prefs;

  String _backendUrl = kDefaultBackendUrl;
  String _translateLanguage = 'Vietnamese';
  List<String> _enabledTones = Tone.values.map((t) => t.apiValue).toList();
  String _defaultTone = '';
  bool _floatingBubbleEnabled = false;
  bool _autoCloseAfterRewrite = true;
  List<String> _enabledLanguageIds = const ['en'];
  String _activeLanguageId = 'en';
  String _lastSeenVersion = '';

  String get backendUrl => _backendUrl;
  /// App version that last ran — drives the one-time post-update "What's New".
  String get lastSeenVersion => _lastSeenVersion;
  String get translateLanguage => _translateLanguage;
  List<String> get enabledTones => List.unmodifiable(_enabledTones);
  String get defaultTone => _defaultTone;
  bool get floatingBubbleEnabled => _floatingBubbleEnabled;
  bool get autoCloseAfterRewrite => _autoCloseAfterRewrite;
  List<String> get enabledLanguageIds => List.unmodifiable(_enabledLanguageIds);
  String get activeLanguageId => _activeLanguageId;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
    // Read user-set URL if present (Settings → Server → Backend URL);
    // fall back to the compile-time default.  Re-introduced 2026-06-01
    // so we can point the app at `api.dev.draftright.info` without
    // rebuilding.  An old install that previously wrote a defunct
    // localhost URL will need to clear it manually — acceptable
    // tradeoff for being able to swap envs in the field.
    _backendUrl = _normalizeUrl(
      _prefs.getString('draftright.backendUrl') ?? kDefaultBackendUrl,
    );
    _translateLanguage = _prefs.getString('draftright.translateLanguage') ?? 'Vietnamese';
    _enabledTones = _prefs.getStringList('draftright.enabledTones')
        ?? Tone.values.map((t) => t.apiValue).toList();
    _defaultTone = _prefs.getString('draftright.defaultTone') ?? '';
    _floatingBubbleEnabled = _prefs.getBool('draftright.floatingBubbleEnabled') ?? false;
    _autoCloseAfterRewrite = _prefs.getBool('draftright.autoCloseAfterRewrite') ?? true;
    _enabledLanguageIds = _prefs.getStringList('draftright.enabledLanguageIds')
        ?? const ['en'];
    if (_enabledLanguageIds.isEmpty) _enabledLanguageIds = const ['en'];
    _activeLanguageId = _prefs.getString('draftright.activeLanguageId') ?? 'en';
    if (!_enabledLanguageIds.contains(_activeLanguageId)) {
      _activeLanguageId = _enabledLanguageIds.first;
    }
    _lastSeenVersion = _prefs.getString('draftright.lastSeenVersion') ?? '';

    // Sync backend URL to SharedPreferences for keyboard extensions
    await _prefs.setString('draftright.backendUrl', _backendUrl);
    await AuthService.syncBackendUrlToAppGroup(_backendUrl);
    // Sync keyboard language preferences to App Group so the iOS keyboard
    // extension picks them up (Flutter's shared_preferences writes to the
    // app's standard UserDefaults, which the extension can't read).
    await AuthService.syncEnabledLanguageIdsToAppGroup(_enabledLanguageIds);
    await AuthService.syncActiveLanguageIdToAppGroup(_activeLanguageId);
  }

  Future<void> setLastSeenVersion(String version) async {
    _lastSeenVersion = version;
    await _prefs.setString('draftright.lastSeenVersion', version);
  }

  Future<void> setBackendUrl(String value) async {
    _backendUrl = _normalizeUrl(value);
    await _prefs.setString('draftright.backendUrl', _backendUrl);
    // Sync for keyboard extensions
    await _prefs.setString('draftright.backendUrl', _backendUrl);
    await AuthService.syncBackendUrlToAppGroup(_backendUrl);
    notifyListeners();
  }

  Future<void> setEnabledTones(List<String> tones) async {
    _enabledTones = List.from(tones);
    await _prefs.setStringList('draftright.enabledTones', _enabledTones);
    notifyListeners();
  }

  Future<void> setDefaultTone(String tone) async {
    _defaultTone = tone;
    await _prefs.setString('draftright.defaultTone', tone);
    notifyListeners();
  }

  Future<void> setTranslateLanguage(String value) async {
    _translateLanguage = value;
    await _prefs.setString('draftright.translateLanguage', value);
    notifyListeners();
  }

  Future<void> setFloatingBubbleEnabled(bool value) async {
    _floatingBubbleEnabled = value;
    await _prefs.setBool('draftright.floatingBubbleEnabled', value);
    notifyListeners();
  }

  Future<void> setAutoCloseAfterRewrite(bool value) async {
    _autoCloseAfterRewrite = value;
    await _prefs.setBool('draftright.autoCloseAfterRewrite', value);
    notifyListeners();
  }

  Future<void> setEnabledLanguageIds(List<String> ids) async {
    _enabledLanguageIds = ids.isEmpty ? const ['en'] : List<String>.from(ids);
    await _prefs.setStringList('draftright.enabledLanguageIds', _enabledLanguageIds);
    await AuthService.syncEnabledLanguageIdsToAppGroup(_enabledLanguageIds);
    if (!_enabledLanguageIds.contains(_activeLanguageId)) {
      _activeLanguageId = _enabledLanguageIds.first;
      await _prefs.setString('draftright.activeLanguageId', _activeLanguageId);
      await AuthService.syncActiveLanguageIdToAppGroup(_activeLanguageId);
    }
    notifyListeners();
  }

  Future<void> setActiveLanguageId(String id) async {
    if (!_enabledLanguageIds.contains(id)) return;
    _activeLanguageId = id;
    await _prefs.setString('draftright.activeLanguageId', id);
    await AuthService.syncActiveLanguageIdToAppGroup(id);
    notifyListeners();
  }

  /// Catalog of keyboard languages the Android IME ships. Keep in sync with
  /// LanguageRegistry.PRODUCTION on the Kotlin side.
  static const Map<String, String> keyboardLanguageCatalog = {
    'en': 'English',
    'vi': 'Tiếng Việt',
    'fr': 'Français',
    'es': 'Español',
    'de': 'Deutsch',
    'it': 'Italiano',
    'pt': 'Português',
    'ja': '日本語',
    'ko': '한국어',
    'zh': '中文',
  };

  static const List<String> supportedLanguages = [
    'Arabic', 'Chinese (Simplified)', 'Chinese (Traditional)',
    'Czech', 'Danish', 'Dutch', 'English', 'Finnish', 'French',
    'German', 'Greek', 'Hebrew', 'Hindi', 'Hungarian',
    'Indonesian', 'Italian', 'Japanese', 'Korean', 'Malay',
    'Norwegian', 'Polish', 'Portuguese', 'Romanian', 'Russian',
    'Spanish', 'Swedish', 'Thai', 'Turkish', 'Ukrainian', 'Vietnamese',
  ];
}
