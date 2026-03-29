import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/settings_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  test('defaults to OpenAI provider', () async {
    final service = SettingsService();
    await service.init();
    expect(service.aiProvider, 'openai');
  });

  test('defaults to gpt-4o-mini model', () async {
    final service = SettingsService();
    await service.init();
    expect(service.model, 'gpt-4o-mini');
  });

  test('defaults to OpenAI endpoint', () async {
    final service = SettingsService();
    await service.init();
    expect(service.endpoint, 'https://api.openai.com/v1/chat/completions');
  });

  test('defaults to 0.3 temperature', () async {
    final service = SettingsService();
    await service.init();
    expect(service.temperature, 0.3);
  });

  test('defaults to Vietnamese translation language', () async {
    final service = SettingsService();
    await service.init();
    expect(service.translateLanguage, 'Vietnamese');
  });

  test('saves and loads provider', () async {
    final service = SettingsService();
    await service.init();
    await service.setAiProvider('custom');
    expect(service.aiProvider, 'custom');
  });

  test('saves and loads endpoint', () async {
    final service = SettingsService();
    await service.init();
    await service.setEndpoint('http://localhost:11434/v1/chat/completions');
    expect(service.endpoint, 'http://localhost:11434/v1/chat/completions');
  });

  test('saves and loads model', () async {
    final service = SettingsService();
    await service.init();
    await service.setModel('llama3');
    expect(service.model, 'llama3');
  });

  test('saves and loads temperature', () async {
    final service = SettingsService();
    await service.init();
    await service.setTemperature(0.7);
    expect(service.temperature, 0.7);
  });

  test('saves and loads translate language', () async {
    final service = SettingsService();
    await service.init();
    await service.setTranslateLanguage('French');
    expect(service.translateLanguage, 'French');
  });
}
