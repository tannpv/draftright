import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

const String kDefaultBackendUrl = 'https://api.draftright.app';

class SettingsService extends ChangeNotifier {
  late SharedPreferences _prefs;

  String _backendUrl = kDefaultBackendUrl;
  String _translateLanguage = 'Vietnamese';

  String get backendUrl => _backendUrl;
  String get translateLanguage => _translateLanguage;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
    _backendUrl = _prefs.getString('draftright.backendUrl') ?? kDefaultBackendUrl;
    _translateLanguage = _prefs.getString('draftright.translateLanguage') ?? 'Vietnamese';

    // Sync backend URL to SharedPreferences for keyboard extensions
    await _prefs.setString('flutter.draftright.backendUrl', _backendUrl);
  }

  Future<void> setBackendUrl(String value) async {
    _backendUrl = value;
    await _prefs.setString('draftright.backendUrl', value);
    // Sync for keyboard extensions
    await _prefs.setString('flutter.draftright.backendUrl', value);
    notifyListeners();
  }

  Future<void> setTranslateLanguage(String value) async {
    _translateLanguage = value;
    await _prefs.setString('draftright.translateLanguage', value);
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
