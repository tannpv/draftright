import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SettingsService extends ChangeNotifier {
  late SharedPreferences _prefs;
  final FlutterSecureStorage _secure = const FlutterSecureStorage();

  String _aiProvider = 'openai';
  String _apiKey = '';
  String _endpoint = 'https://api.openai.com/v1/chat/completions';
  String _model = 'gpt-4o-mini';
  double _temperature = 0.3;
  String _translateLanguage = 'Vietnamese';

  String get aiProvider => _aiProvider;
  String get apiKey => _apiKey;
  String get endpoint => _endpoint;
  String get model => _model;
  double get temperature => _temperature;
  String get translateLanguage => _translateLanguage;

  Future<void> init() async {
    _prefs = await SharedPreferences.getInstance();
    _aiProvider = _prefs.getString('draftright.aiProvider') ?? 'openai';
    _endpoint = _prefs.getString('draftright.endpoint') ?? 'https://api.openai.com/v1/chat/completions';
    _model = _prefs.getString('draftright.model') ?? 'gpt-4o-mini';
    _temperature = _prefs.getDouble('draftright.temperature') ?? 0.3;
    _translateLanguage = _prefs.getString('draftright.translateLanguage') ?? 'Vietnamese';
    try {
      _apiKey = await _secure.read(key: 'draftright.apiKey') ?? '';
    } catch (_) {
      _apiKey = '';
    }
    // Sync API key to SharedPreferences for keyboard extension (IME) access
    if (_apiKey.isNotEmpty) {
      await _prefs.setString('draftright.apiKey', _apiKey);
    }
  }

  Future<void> setAiProvider(String value) async {
    _aiProvider = value;
    await _prefs.setString('draftright.aiProvider', value);
    notifyListeners();
  }

  Future<void> setApiKey(String value) async {
    _apiKey = value;
    await _secure.write(key: 'draftright.apiKey', value: value);
    // Also write to SharedPreferences so the keyboard extension (IME) can read it
    await _prefs.setString('draftright.apiKey', value);
    notifyListeners();
  }

  Future<void> setEndpoint(String value) async {
    _endpoint = value;
    await _prefs.setString('draftright.endpoint', value);
    notifyListeners();
  }

  Future<void> setModel(String value) async {
    _model = value;
    await _prefs.setString('draftright.model', value);
    notifyListeners();
  }

  Future<void> setTemperature(double value) async {
    _temperature = value;
    await _prefs.setDouble('draftright.temperature', value);
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
