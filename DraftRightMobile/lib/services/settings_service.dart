import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/prefs_keys.dart';
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
  String _bubblePresetTone = Tone.polished.apiValue;
  List<String> _enabledLanguageIds = const [kDefaultLanguageId];
  String _activeLanguageId = kDefaultLanguageId;
  String _lastSeenVersion = '';
  bool _voicePolishEnabled = true;

  String get backendUrl => _backendUrl;

  /// App version that last ran — drives the one-time post-update "What's New".
  String get lastSeenVersion => _lastSeenVersion;
  String get translateLanguage => _translateLanguage;
  List<String> get enabledTones => List.unmodifiable(_enabledTones);
  String get defaultTone => _defaultTone;
  String get bubblePresetTone => _bubblePresetTone;
  List<String> get enabledLanguageIds => List.unmodifiable(_enabledLanguageIds);
  String get activeLanguageId => _activeLanguageId;

  /// Whether voice dictation is AI-polished. False = insert the raw transcript
  /// (dictate-only, no /rewrite call). The keyboards read this as rawMode's
  /// inverse (#197). Default on = the original behaviour.
  bool get voicePolishEnabled => _voicePolishEnabled;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
    // Read user-set URL if present (Settings → Server → Backend URL);
    // fall back to the compile-time default.  Re-introduced 2026-06-01
    // so we can point the app at `api.dev.draftright.info` without
    // rebuilding.  An old install that previously wrote a defunct
    // localhost URL will need to clear it manually — acceptable
    // tradeoff for being able to swap envs in the field.
    _backendUrl = _normalizeUrl(
      _prefs.getString(PrefsKeys.backendUrl) ?? kDefaultBackendUrl,
    );
    _translateLanguage =
        _prefs.getString(PrefsKeys.translateLanguage) ?? 'Vietnamese';
    _enabledTones = _prefs.getStringList(PrefsKeys.enabledTones) ??
        Tone.values.map((t) => t.apiValue).toList();
    _defaultTone = _prefs.getString(PrefsKeys.defaultTone) ?? '';
    _bubblePresetTone =
        _prefs.getString(PrefsKeys.bubblePresetTone) ?? Tone.polished.apiValue;
    // The floating bubble was removed; purge its orphaned pref so it doesn't
    // linger in storage on upgraded installs.
    await _prefs.remove(PrefsKeys.floatingBubbleEnabled);
    _enabledLanguageIds = _prefs.getStringList(PrefsKeys.enabledLanguageIds) ??
        const [kDefaultLanguageId];
    if (_enabledLanguageIds.isEmpty) {
      _enabledLanguageIds = const [kDefaultLanguageId];
    }
    _activeLanguageId =
        _prefs.getString(PrefsKeys.activeLanguageId) ?? kDefaultLanguageId;
    if (!_enabledLanguageIds.contains(_activeLanguageId)) {
      _activeLanguageId = _enabledLanguageIds.first;
    }
    _lastSeenVersion = _prefs.getString(PrefsKeys.lastSeenVersion) ?? '';
    _voicePolishEnabled = _prefs.getBool(PrefsKeys.voicePolishEnabled) ?? true;

    // Sync backend URL to SharedPreferences for keyboard extensions
    await _prefs.setString(PrefsKeys.backendUrl, _backendUrl);
    await AuthService.syncBackendUrlToAppGroup(_backendUrl);
    // Sync keyboard language preferences to App Group so the iOS keyboard
    // extension picks them up (Flutter's shared_preferences writes to the
    // app's standard UserDefaults, which the extension can't read).
    await AuthService.syncEnabledLanguageIdsToAppGroup(_enabledLanguageIds);
    await AuthService.syncActiveLanguageIdToAppGroup(_activeLanguageId);
    // The iOS keyboard's one-tap rewrite reads this from the App Group.
    await AuthService.syncOneTapToneToAppGroup(_bubblePresetTone);
    // Android reads the SharedPreferences bool directly; iOS needs it in the
    // App Group (#197).
    await AuthService.syncVoicePolishEnabledToAppGroup(_voicePolishEnabled);
  }

  /// Toggle AI-polish of voice dictation. Off = raw transcript inserted (#197).
  Future<void> setVoicePolishEnabled(bool enabled) async {
    _voicePolishEnabled = enabled;
    await _prefs.setBool(PrefsKeys.voicePolishEnabled, enabled);
    await AuthService.syncVoicePolishEnabledToAppGroup(enabled);
    notifyListeners();
  }

  Future<void> setLastSeenVersion(String version) async {
    _lastSeenVersion = version;
    await _prefs.setString(PrefsKeys.lastSeenVersion, version);
  }

  Future<void> setBackendUrl(String value) async {
    _backendUrl = _normalizeUrl(value);
    await _prefs.setString(PrefsKeys.backendUrl, _backendUrl);
    // Sync for keyboard extensions
    await _prefs.setString(PrefsKeys.backendUrl, _backendUrl);
    await AuthService.syncBackendUrlToAppGroup(_backendUrl);
    notifyListeners();
  }

  Future<void> setEnabledTones(List<String> tones) async {
    _enabledTones = List.from(tones);
    await _prefs.setStringList(PrefsKeys.enabledTones, _enabledTones);
    notifyListeners();
  }

  Future<void> setDefaultTone(String tone) async {
    _defaultTone = tone;
    await _prefs.setString(PrefsKeys.defaultTone, tone);
    notifyListeners();
  }

  Future<void> setTranslateLanguage(String value) async {
    _translateLanguage = value;
    await _prefs.setString(PrefsKeys.translateLanguage, value);
    notifyListeners();
  }

  /// Preset tone applied to each one-tap keyboard rewrite — the "simple mode"
  /// default. One setting, two surfaces: the Android keyboard reads it from
  /// SharedPreferences (flutter.draftright.bubblePresetTone); the iOS keyboard's
  /// one-tap button reads it from the App Group (draftright.oneTapTone, synced
  /// via [AuthService.syncOneTapToneToAppGroup]). Stored as the stable
  /// Tone.apiValue. (Key name is legacy — kept to avoid an install migration.)
  Future<void> setBubblePresetTone(String apiValue) async {
    _bubblePresetTone = apiValue;
    await _prefs.setString(PrefsKeys.bubblePresetTone, apiValue);
    // Keep the iOS keyboard's one-tap tone in step with the picker.
    await AuthService.syncOneTapToneToAppGroup(apiValue);
    notifyListeners();
  }

  Future<void> setEnabledLanguageIds(List<String> ids) async {
    _enabledLanguageIds =
        ids.isEmpty ? const [kDefaultLanguageId] : List<String>.from(ids);
    await _prefs.setStringList(
        PrefsKeys.enabledLanguageIds, _enabledLanguageIds);
    await AuthService.syncEnabledLanguageIdsToAppGroup(_enabledLanguageIds);
    if (!_enabledLanguageIds.contains(_activeLanguageId)) {
      _activeLanguageId = _enabledLanguageIds.first;
      await _prefs.setString(PrefsKeys.activeLanguageId, _activeLanguageId);
      await AuthService.syncActiveLanguageIdToAppGroup(_activeLanguageId);
    }
    notifyListeners();
  }

  Future<void> setActiveLanguageId(String id) async {
    if (!_enabledLanguageIds.contains(id)) return;
    _activeLanguageId = id;
    await _prefs.setString(PrefsKeys.activeLanguageId, id);
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
    'Arabic',
    'Chinese (Simplified)',
    'Chinese (Traditional)',
    'Czech',
    'Danish',
    'Dutch',
    'English',
    'Finnish',
    'French',
    'German',
    'Greek',
    'Hebrew',
    'Hindi',
    'Hungarian',
    'Indonesian',
    'Italian',
    'Japanese',
    'Korean',
    'Malay',
    'Norwegian',
    'Polish',
    'Portuguese',
    'Romanian',
    'Russian',
    'Spanish',
    'Swedish',
    'Thai',
    'Turkish',
    'Ukrainian',
    'Vietnamese',
  ];
}
