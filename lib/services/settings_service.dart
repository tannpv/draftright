import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'auth_service.dart';

// Default to production backend always. Devs running local backend can override via Settings UI.
const String kDefaultBackendUrl = 'https://api.draftright.info';

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

  String get backendUrl => _backendUrl;
  String get translateLanguage => _translateLanguage;
  List<String> get enabledTones => List.unmodifiable(_enabledTones);
  String get defaultTone => _defaultTone;
  bool get floatingBubbleEnabled => _floatingBubbleEnabled;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
    _backendUrl = _normalizeUrl(
      _prefs.getString('draftright.backendUrl') ?? kDefaultBackendUrl,
    );
    _translateLanguage = _prefs.getString('draftright.translateLanguage') ?? 'Vietnamese';
    _enabledTones = _prefs.getStringList('draftright.enabledTones')
        ?? Tone.values.map((t) => t.apiValue).toList();
    _defaultTone = _prefs.getString('draftright.defaultTone') ?? '';
    _floatingBubbleEnabled = _prefs.getBool('draftright.floatingBubbleEnabled') ?? false;

    // Sync backend URL to SharedPreferences for keyboard extensions
    await _prefs.setString('draftright.backendUrl', _backendUrl);
    await AuthService.syncBackendUrlToAppGroup(_backendUrl);
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

  static const List<String> supportedLanguages = [
    'Arabic', 'Chinese (Simplified)', 'Chinese (Traditional)',
    'Czech', 'Danish', 'Dutch', 'English', 'Finnish', 'French',
    'German', 'Greek', 'Hebrew', 'Hindi', 'Hungarian',
    'Indonesian', 'Italian', 'Japanese', 'Korean', 'Malay',
    'Norwegian', 'Polish', 'Portuguese', 'Romanian', 'Russian',
    'Spanish', 'Swedish', 'Thai', 'Turkish', 'Ukrainian', 'Vietnamese',
  ];
}
