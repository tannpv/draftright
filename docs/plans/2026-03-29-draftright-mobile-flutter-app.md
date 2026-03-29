# DraftRight Mobile — Flutter App Implementation Plan (Plan 1 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Flutter main app with onboarding, settings, and test playground for DraftRight Mobile.

**Architecture:** Flutter app (Dart) that manages user settings (AI provider, API key, model, translation language) and stores them in shared storage accessible by the native keyboard extensions. Includes an onboarding flow and a test playground to verify the API connection.

**Tech Stack:** Flutter 3.x, Dart, `flutter_secure_storage`, `shared_preferences`, `http`

**Spec:** `docs/specs/2026-03-28-draftright-mobile-design.md`

**Note:** This is Plan 1 of 3. Plan 2 covers the iOS keyboard extension (Swift). Plan 3 covers the Android keyboard extension (Kotlin).

---

### Task 1: Create Flutter Project

**Files:**
- Create: `DraftRightMobile/pubspec.yaml`
- Create: `DraftRightMobile/lib/main.dart`

- [ ] **Step 1: Create the Flutter project**

```bash
cd /opt/openAi/DraftRight
flutter create --org com.draftright --project-name draftright_mobile DraftRightMobile
```

- [ ] **Step 2: Update pubspec.yaml dependencies**

Edit `DraftRightMobile/pubspec.yaml` — replace the `dependencies` and `dev_dependencies` sections:

```yaml
name: draftright_mobile
description: AI-powered text rewriting keyboard toolbar
publish_to: 'none'
version: 1.0.0+1

environment:
  sdk: ^3.0.0

dependencies:
  flutter:
    sdk: flutter
  shared_preferences: ^2.2.0
  flutter_secure_storage: ^9.0.0
  http: ^1.1.0

dev_dependencies:
  flutter_test:
    sdk: flutter
  flutter_lints: ^3.0.0

flutter:
  uses-material-design: true
```

- [ ] **Step 3: Run flutter pub get**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter pub get
```

Expected: resolves dependencies with no errors.

- [ ] **Step 4: Verify the project builds**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter build apk --debug 2>&1 | tail -5
```

Expected: `BUILD SUCCESSFUL`

- [ ] **Step 5: Commit**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile
git init
echo -e ".dart_tool/\n.packages\nbuild/\n.flutter-plugins\n.flutter-plugins-dependencies\n*.iml\n.idea/\n.DS_Store\nandroid/.gradle/\nios/Pods/" > .gitignore
git add .
git commit -m "feat: scaffold DraftRight Mobile Flutter project"
```

---

### Task 2: Tone Model

**Files:**
- Create: `DraftRightMobile/lib/models/tone.dart`
- Create: `DraftRightMobile/test/models/tone_test.dart`

- [ ] **Step 1: Write the test**

Create `DraftRightMobile/test/models/tone_test.dart`:

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:draftright_mobile/models/tone.dart';

void main() {
  test('Tone.values has 6 tones', () {
    expect(Tone.values.length, 6);
  });

  test('each tone has a non-empty displayName', () {
    for (final tone in Tone.values) {
      expect(tone.displayName.isNotEmpty, true);
    }
  });

  test('each tone has a non-empty systemPrompt', () {
    for (final tone in Tone.values) {
      expect(tone.systemPrompt().isNotEmpty, true);
    }
  });

  test('translate tone includes target language in prompt', () {
    final prompt = Tone.translate.systemPrompt(targetLanguage: 'Vietnamese');
    expect(prompt.contains('Vietnamese'), true);
  });

  test('translate tone defaults to English', () {
    final prompt = Tone.translate.systemPrompt();
    expect(prompt.contains('English'), true);
  });

  test('each tone has an icon name', () {
    for (final tone in Tone.values) {
      expect(tone.iconName.isNotEmpty, true);
    }
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/models/tone_test.dart
```

Expected: FAIL — file not found.

- [ ] **Step 3: Implement the Tone model**

Create `DraftRightMobile/lib/models/tone.dart`:

```dart
enum Tone {
  simple,
  natural,
  polished,
  concise,
  technical,
  translate;

  String get displayName {
    switch (this) {
      case Tone.simple:
        return 'Simple';
      case Tone.natural:
        return 'More Natural';
      case Tone.polished:
        return 'More Polished';
      case Tone.concise:
        return 'Concise';
      case Tone.technical:
        return 'Technical';
      case Tone.translate:
        return 'Translate';
    }
  }

  String get iconName {
    switch (this) {
      case Tone.simple:
        return 'text_fields';
      case Tone.natural:
        return 'chat_bubble_outline';
      case Tone.polished:
        return 'auto_awesome';
      case Tone.concise:
        return 'compress';
      case Tone.technical:
        return 'build';
      case Tone.translate:
        return 'language';
    }
  }

  String systemPrompt({String targetLanguage = 'English'}) {
    switch (this) {
      case Tone.simple:
        return 'Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.natural:
        return 'Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.polished:
        return 'Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.concise:
        return 'Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations.';
      case Tone.technical:
        return 'Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations.';
      case Tone.translate:
        return 'Translate the following text into $targetLanguage. If the text is already in $targetLanguage, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations.';
    }
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/models/tone_test.dart
```

Expected: All 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/models/tone.dart test/models/tone_test.dart
git commit -m "feat: add Tone enum with display names, icons, and system prompts"
```

---

### Task 3: Settings Service

**Files:**
- Create: `DraftRightMobile/lib/services/settings_service.dart`
- Create: `DraftRightMobile/test/services/settings_service_test.dart`

- [ ] **Step 1: Write the test**

Create `DraftRightMobile/test/services/settings_service_test.dart`:

```dart
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/services/settings_service_test.dart
```

Expected: FAIL — file not found.

- [ ] **Step 3: Implement SettingsService**

Create `DraftRightMobile/lib/services/settings_service.dart`:

```dart
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
    _endpoint = _prefs.getString('draftright.endpoint') ??
        'https://api.openai.com/v1/chat/completions';
    _model = _prefs.getString('draftright.model') ?? 'gpt-4o-mini';
    _temperature = _prefs.getDouble('draftright.temperature') ?? 0.3;
    _translateLanguage =
        _prefs.getString('draftright.translateLanguage') ?? 'Vietnamese';
    try {
      _apiKey = await _secure.read(key: 'draftright.apiKey') ?? '';
    } catch (_) {
      _apiKey = '';
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/services/settings_service_test.dart
```

Expected: All 10 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/services/settings_service.dart test/services/settings_service_test.dart
git commit -m "feat: add SettingsService with SharedPreferences and Keychain storage"
```

---

### Task 4: OpenAI Client

**Files:**
- Create: `DraftRightMobile/lib/services/openai_client.dart`
- Create: `DraftRightMobile/test/services/openai_client_test.dart`

- [ ] **Step 1: Write the test**

Create `DraftRightMobile/test/services/openai_client_test.dart`:

```dart
import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:draftright_mobile/services/openai_client.dart';
import 'package:draftright_mobile/models/tone.dart';

void main() {
  test('rewrite sends correct request and parses response', () async {
    final mockClient = MockClient((request) async {
      expect(request.method, 'POST');
      expect(request.headers['Content-Type'], 'application/json');
      expect(request.headers['Authorization'], 'Bearer test-key');

      final body = jsonDecode(request.body);
      expect(body['model'], 'gpt-4o-mini');
      expect(body['messages'].length, 2);
      expect(body['messages'][0]['role'], 'system');
      expect(body['messages'][1]['role'], 'user');
      expect(body['messages'][1]['content'], 'Hello world');

      return http.Response(
        jsonEncode({
          'choices': [
            {
              'message': {'role': 'assistant', 'content': 'Hi there'}
            }
          ]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'Hello world',
      tone: Tone.simple,
      apiKey: 'test-key',
      endpoint: 'https://api.openai.com/v1/chat/completions',
      model: 'gpt-4o-mini',
      temperature: 0.3,
    );

    expect(result, 'Hi there');
  });

  test('rewrite skips auth header when apiKey is empty', () async {
    final mockClient = MockClient((request) async {
      expect(request.headers.containsKey('Authorization'), false);
      return http.Response(
        jsonEncode({
          'choices': [
            {
              'message': {'role': 'assistant', 'content': 'result'}
            }
          ]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'test',
      tone: Tone.concise,
      apiKey: '',
      endpoint: 'http://localhost:11434/v1/chat/completions',
      model: 'llama3',
      temperature: 0.3,
    );

    expect(result, 'result');
  });

  test('rewrite throws on HTTP error', () async {
    final mockClient = MockClient((request) async {
      return http.Response('{"error": "bad request"}', 400);
    });

    final client = OpenAIClient(httpClient: mockClient);
    expect(
      () => client.rewrite(
        text: 'test',
        tone: Tone.simple,
        apiKey: 'key',
        endpoint: 'https://api.openai.com/v1/chat/completions',
        model: 'gpt-4o-mini',
        temperature: 0.3,
      ),
      throwsException,
    );
  });

  test('rewrite throws on empty choices', () async {
    final mockClient = MockClient((request) async {
      return http.Response(jsonEncode({'choices': []}), 200);
    });

    final client = OpenAIClient(httpClient: mockClient);
    expect(
      () => client.rewrite(
        text: 'test',
        tone: Tone.simple,
        apiKey: 'key',
        endpoint: 'https://api.openai.com/v1/chat/completions',
        model: 'gpt-4o-mini',
        temperature: 0.3,
      ),
      throwsException,
    );
  });

  test('rewrite passes targetLanguage for translate tone', () async {
    final mockClient = MockClient((request) async {
      final body = jsonDecode(request.body);
      expect(body['messages'][0]['content'].toString().contains('Vietnamese'), true);
      return http.Response(
        jsonEncode({
          'choices': [
            {
              'message': {'role': 'assistant', 'content': 'Xin chao'}
            }
          ]
        }),
        200,
      );
    });

    final client = OpenAIClient(httpClient: mockClient);
    final result = await client.rewrite(
      text: 'Hello',
      tone: Tone.translate,
      apiKey: 'key',
      endpoint: 'https://api.openai.com/v1/chat/completions',
      model: 'gpt-4o-mini',
      temperature: 0.3,
      targetLanguage: 'Vietnamese',
    );

    expect(result, 'Xin chao');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/services/openai_client_test.dart
```

Expected: FAIL — file not found.

- [ ] **Step 3: Implement OpenAIClient**

Create `DraftRightMobile/lib/services/openai_client.dart`:

```dart
import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:draftright_mobile/models/tone.dart';

class OpenAIClient {
  final http.Client _client;

  OpenAIClient({http.Client? httpClient}) : _client = httpClient ?? http.Client();

  Future<String> rewrite({
    required String text,
    required Tone tone,
    required String apiKey,
    required String endpoint,
    required String model,
    required double temperature,
    String targetLanguage = 'English',
  }) async {
    final uri = Uri.parse(endpoint);
    final inputText = text.length > 3000 ? text.substring(0, 3000) : text;

    final headers = <String, String>{
      'Content-Type': 'application/json',
    };
    if (apiKey.isNotEmpty) {
      headers['Authorization'] = 'Bearer $apiKey';
    }

    final body = jsonEncode({
      'model': model,
      'messages': [
        {'role': 'system', 'content': tone.systemPrompt(targetLanguage: targetLanguage)},
        {'role': 'user', 'content': inputText},
      ],
      'temperature': temperature,
      'max_tokens': 1024,
    });

    final response = await _client
        .post(uri, headers: headers, body: body)
        .timeout(const Duration(seconds: 15));

    if (response.statusCode >= 400) {
      throw Exception('HTTP ${response.statusCode}: ${response.body}');
    }

    final decoded = jsonDecode(response.body) as Map<String, dynamic>;
    final choices = decoded['choices'] as List;
    if (choices.isEmpty) {
      throw Exception('No response from AI');
    }

    final content = choices[0]['message']['content'] as String;
    return content.trim();
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test test/services/openai_client_test.dart
```

Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/services/openai_client.dart test/services/openai_client_test.dart
git commit -m "feat: add OpenAI client with mock-testable HTTP, auth skip for custom servers"
```

---

### Task 5: Settings Screen

**Files:**
- Create: `DraftRightMobile/lib/screens/settings_screen.dart`

- [ ] **Step 1: Create the Settings screen**

Create `DraftRightMobile/lib/screens/settings_screen.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/settings_service.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late TextEditingController _apiKeyController;
  late TextEditingController _endpointController;
  late TextEditingController _modelController;

  @override
  void initState() {
    super.initState();
    final settings = context.read<SettingsService>();
    _apiKeyController = TextEditingController(text: settings.apiKey);
    _endpointController = TextEditingController(text: settings.endpoint);
    _modelController = TextEditingController(text: settings.model);
  }

  @override
  void dispose() {
    _apiKeyController.dispose();
    _endpointController.dispose();
    _modelController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<SettingsService>(
      builder: (context, settings, _) {
        return Scaffold(
          appBar: AppBar(title: const Text('Settings')),
          body: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // AI Provider
              const Text('AI Provider',
                  style: TextStyle(fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'openai', label: Text('OpenAI')),
                  ButtonSegment(value: 'custom', label: Text('Custom Server')),
                ],
                selected: {settings.aiProvider},
                onSelectionChanged: (value) {
                  final provider = value.first;
                  settings.setAiProvider(provider);
                  if (provider == 'custom') {
                    _endpointController.text =
                        'http://localhost:11434/v1/chat/completions';
                    _modelController.text = 'llama3';
                    settings.setEndpoint(_endpointController.text);
                    settings.setModel(_modelController.text);
                  } else {
                    _endpointController.text =
                        'https://api.openai.com/v1/chat/completions';
                    _modelController.text = 'gpt-4o-mini';
                    settings.setEndpoint(_endpointController.text);
                    settings.setModel(_modelController.text);
                  }
                },
              ),
              const SizedBox(height: 16),

              // API Key
              TextField(
                controller: _apiKeyController,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: settings.aiProvider == 'openai'
                      ? 'API Key'
                      : 'API Key (optional)',
                  border: const OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setApiKey(value),
              ),
              const SizedBox(height: 16),

              // Endpoint
              TextField(
                controller: _endpointController,
                decoration: const InputDecoration(
                  labelText: 'Server URL',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setEndpoint(value),
              ),
              const SizedBox(height: 16),

              // Model
              TextField(
                controller: _modelController,
                decoration: const InputDecoration(
                  labelText: 'Model',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setModel(value),
              ),
              const SizedBox(height: 16),

              // Temperature
              Row(
                children: [
                  const Text('Temperature'),
                  Expanded(
                    child: Slider(
                      value: settings.temperature,
                      min: 0,
                      max: 1,
                      divisions: 20,
                      onChanged: (value) => settings.setTemperature(value),
                    ),
                  ),
                  Text(settings.temperature.toStringAsFixed(2)),
                ],
              ),
              const SizedBox(height: 24),

              // Translation Language
              const Text('Translation Language',
                  style: TextStyle(fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: settings.translateLanguage,
                decoration: const InputDecoration(
                  border: OutlineInputBorder(),
                ),
                items: SettingsService.supportedLanguages
                    .map((lang) =>
                        DropdownMenuItem(value: lang, child: Text(lang)))
                    .toList(),
                onChanged: (value) {
                  if (value != null) settings.setTranslateLanguage(value);
                },
              ),
            ],
          ),
        );
      },
    );
  }
}
```

- [ ] **Step 2: Add `provider` dependency to pubspec.yaml**

Add `provider: ^6.0.0` to the `dependencies` section of `pubspec.yaml`, then run:

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter pub get
```

- [ ] **Step 3: Verify build**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze lib/screens/settings_screen.dart
```

Expected: No issues found.

- [ ] **Step 4: Commit**

```bash
git add lib/screens/settings_screen.dart pubspec.yaml pubspec.lock
git commit -m "feat: add Settings screen with AI provider, API key, language picker"
```

---

### Task 6: Onboarding Screen

**Files:**
- Create: `DraftRightMobile/lib/screens/onboarding_screen.dart`

- [ ] **Step 1: Create the Onboarding screen**

Create `DraftRightMobile/lib/screens/onboarding_screen.dart`:

```dart
import 'dart:io' show Platform;
import 'package:flutter/material.dart';

class OnboardingScreen extends StatefulWidget {
  final VoidCallback onComplete;

  const OnboardingScreen({super.key, required this.onComplete});

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  final PageController _pageController = PageController();
  int _currentPage = 0;

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final pages = [
      _buildWelcomePage(),
      _buildEnableKeyboardPage(),
      _buildSetupApiPage(),
    ];

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: PageView(
                controller: _pageController,
                onPageChanged: (index) => setState(() => _currentPage = index),
                children: pages,
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(24),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  if (_currentPage > 0)
                    TextButton(
                      onPressed: () => _pageController.previousPage(
                        duration: const Duration(milliseconds: 300),
                        curve: Curves.easeInOut,
                      ),
                      child: const Text('Back'),
                    )
                  else
                    const SizedBox.shrink(),
                  Row(
                    children: List.generate(
                      pages.length,
                      (i) => Container(
                        margin: const EdgeInsets.symmetric(horizontal: 4),
                        width: 8,
                        height: 8,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: i == _currentPage
                              ? Theme.of(context).colorScheme.primary
                              : Colors.grey.shade300,
                        ),
                      ),
                    ),
                  ),
                  _currentPage == pages.length - 1
                      ? FilledButton(
                          onPressed: widget.onComplete,
                          child: const Text('Get Started'),
                        )
                      : TextButton(
                          onPressed: () => _pageController.nextPage(
                            duration: const Duration(milliseconds: 300),
                            curve: Curves.easeInOut,
                          ),
                          child: const Text('Next'),
                        ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildWelcomePage() {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.edit_note, size: 80, color: Colors.blue),
          SizedBox(height: 24),
          Text(
            'DraftRight',
            style: TextStyle(fontSize: 32, fontWeight: FontWeight.bold),
          ),
          SizedBox(height: 16),
          Text(
            'AI-powered text rewriting right from your keyboard. '
            'Rewrite text in any app with one tap.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, color: Colors.grey),
          ),
        ],
      ),
    );
  }

  Widget _buildEnableKeyboardPage() {
    final isIOS = Platform.isIOS;
    final steps = isIOS
        ? [
            'Open Settings app',
            'Go to General → Keyboard → Keyboards',
            'Tap "Add New Keyboard..."',
            'Select "DraftRight"',
            'Tap DraftRight → Enable "Allow Full Access"',
          ]
        : [
            'Open Settings app',
            'Go to Language & Input → Manage Keyboards',
            'Enable "DraftRight"',
            'Confirm the permission dialog',
          ];

    return Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.keyboard, size: 60, color: Colors.blue),
          const SizedBox(height: 24),
          const Text(
            'Enable the Keyboard',
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 24),
          ...steps.asMap().entries.map((entry) => Padding(
                padding: const EdgeInsets.symmetric(vertical: 6),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    CircleAvatar(
                      radius: 12,
                      child: Text('${entry.key + 1}',
                          style: const TextStyle(fontSize: 12)),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(entry.value,
                          style: const TextStyle(fontSize: 15)),
                    ),
                  ],
                ),
              )),
          if (isIOS) ...[
            const SizedBox(height: 16),
            const Text(
              'Full Access is required so the keyboard can connect to the AI service.',
              style: TextStyle(fontSize: 13, color: Colors.grey),
              textAlign: TextAlign.center,
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildSetupApiPage() {
    return const Padding(
      padding: EdgeInsets.all(32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.vpn_key, size: 60, color: Colors.blue),
          SizedBox(height: 24),
          Text(
            'Set Up AI',
            style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
          ),
          SizedBox(height: 16),
          Text(
            'After enabling the keyboard, open Settings in this app to enter your OpenAI API key or configure a custom server.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 16, color: Colors.grey),
          ),
        ],
      ),
    );
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze lib/screens/onboarding_screen.dart
```

Expected: No issues found.

- [ ] **Step 3: Commit**

```bash
git add lib/screens/onboarding_screen.dart
git commit -m "feat: add onboarding screen with keyboard enable instructions"
```

---

### Task 7: Playground Screen

**Files:**
- Create: `DraftRightMobile/lib/screens/playground_screen.dart`

- [ ] **Step 1: Create the Playground screen**

Create `DraftRightMobile/lib/screens/playground_screen.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/openai_client.dart';
import 'package:draftright_mobile/services/settings_service.dart';

class PlaygroundScreen extends StatefulWidget {
  const PlaygroundScreen({super.key});

  @override
  State<PlaygroundScreen> createState() => _PlaygroundScreenState();
}

class _PlaygroundScreenState extends State<PlaygroundScreen> {
  final TextEditingController _textController = TextEditingController();
  final OpenAIClient _client = OpenAIClient();
  Tone? _selectedTone;
  String? _result;
  bool _isLoading = false;
  String? _error;

  @override
  void dispose() {
    _textController.dispose();
    super.dispose();
  }

  Future<void> _rewrite(Tone tone) async {
    final text = _textController.text.trim();
    if (text.isEmpty) return;

    final settings = context.read<SettingsService>();

    setState(() {
      _selectedTone = tone;
      _isLoading = true;
      _error = null;
      _result = null;
    });

    try {
      final result = await _client.rewrite(
        text: text,
        tone: tone,
        apiKey: settings.apiKey,
        endpoint: settings.endpoint,
        model: settings.model,
        temperature: settings.temperature,
        targetLanguage: settings.translateLanguage,
      );
      setState(() {
        _result = result;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Playground')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TextField(
              controller: _textController,
              maxLines: 4,
              decoration: const InputDecoration(
                labelText: 'Enter text to rewrite',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 12),

            // Tone buttons
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: Tone.values.map((tone) {
                  final isSelected = _selectedTone == tone;
                  return Padding(
                    padding: const EdgeInsets.only(right: 6),
                    child: ChoiceChip(
                      label: Text(tone.displayName),
                      selected: isSelected,
                      onSelected: _isLoading ? null : (_) => _rewrite(tone),
                    ),
                  );
                }).toList(),
              ),
            ),
            const SizedBox(height: 16),

            // Result
            Expanded(
              child: _isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(
                          child: Text(
                            _error!,
                            style: const TextStyle(color: Colors.red),
                            textAlign: TextAlign.center,
                          ),
                        )
                      : _result != null
                          ? Container(
                              padding: const EdgeInsets.all(12),
                              decoration: BoxDecoration(
                                color: Colors.green.withOpacity(0.05),
                                borderRadius: BorderRadius.circular(8),
                                border: Border.all(
                                    color: Colors.green.withOpacity(0.2)),
                              ),
                              child: SingleChildScrollView(
                                child: SelectableText(
                                  _result!,
                                  style: const TextStyle(fontSize: 15),
                                ),
                              ),
                            )
                          : const Center(
                              child: Text(
                                'Pick a tone to rewrite your text',
                                style: TextStyle(color: Colors.grey),
                              ),
                            ),
            ),

            // Copy button
            if (_result != null)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: FilledButton.icon(
                  onPressed: () {
                    // Copy result to clipboard
                    final data =
                        ClipboardData(text: _result!);
                    Clipboard.setData(data);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Copied to clipboard')),
                    );
                  },
                  icon: const Icon(Icons.copy),
                  label: const Text('Copy Result'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
```

- [ ] **Step 2: Add missing import at the top of the file**

Add `import 'package:flutter/services.dart';` after the material import for `Clipboard` and `ClipboardData`.

- [ ] **Step 3: Verify build**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze lib/screens/playground_screen.dart
```

Expected: No issues found.

- [ ] **Step 4: Commit**

```bash
git add lib/screens/playground_screen.dart
git commit -m "feat: add playground screen for testing rewrites in-app"
```

---

### Task 8: Main App with Navigation

**Files:**
- Modify: `DraftRightMobile/lib/main.dart`

- [ ] **Step 1: Implement main.dart with navigation**

Replace `DraftRightMobile/lib/main.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/onboarding_screen.dart';
import 'package:draftright_mobile/screens/settings_screen.dart';
import 'package:draftright_mobile/screens/playground_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final settings = SettingsService();
  await settings.init();
  runApp(DraftRightApp(settings: settings));
}

class DraftRightApp extends StatelessWidget {
  final SettingsService settings;

  const DraftRightApp({super.key, required this.settings});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider.value(
      value: settings,
      child: MaterialApp(
        title: 'DraftRight',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: Colors.blue,
            brightness: Brightness.dark,
          ),
          useMaterial3: true,
        ),
        home: const HomeScreen(),
      ),
    );
  }
}

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  bool _onboardingComplete = false;
  int _currentIndex = 0;

  @override
  void initState() {
    super.initState();
    _checkOnboarding();
  }

  Future<void> _checkOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    final complete = prefs.getBool('draftright.onboardingComplete') ?? false;
    setState(() => _onboardingComplete = complete);
  }

  Future<void> _completeOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('draftright.onboardingComplete', true);
    setState(() => _onboardingComplete = true);
  }

  @override
  Widget build(BuildContext context) {
    if (!_onboardingComplete) {
      return OnboardingScreen(onComplete: _completeOnboarding);
    }

    final screens = [
      const PlaygroundScreen(),
      const SettingsScreen(),
    ];

    return Scaffold(
      body: screens[_currentIndex],
      bottomNavigationBar: NavigationBar(
        selectedIndex: _currentIndex,
        onDestinationSelected: (index) =>
            setState(() => _currentIndex = index),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.edit_note),
            label: 'Playground',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}
```

- [ ] **Step 2: Verify full build**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze
```

Expected: No issues found.

- [ ] **Step 3: Commit**

```bash
git add lib/main.dart
git commit -m "feat: implement main app with onboarding, navigation, playground, and settings"
```

---

### Task 9: Full Build Verification and README

**Files:**
- Create: `DraftRightMobile/README.md`

- [ ] **Step 1: Run all tests**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter test
```

Expected: All tests pass.

- [ ] **Step 2: Run flutter analyze**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter analyze
```

Expected: No issues found.

- [ ] **Step 3: Build Android APK**

```bash
cd /opt/openAi/DraftRight/DraftRightMobile && flutter build apk --debug 2>&1 | tail -5
```

Expected: BUILD SUCCESSFUL

- [ ] **Step 4: Create README.md**

Create `DraftRightMobile/README.md`:

```markdown
# DraftRight Mobile

AI-powered text rewriting keyboard for iOS and Android. Adds a rewrite toolbar above your system keyboard with tone options: Simple, More Natural, More Polished, Concise, Technical, Translate.

## Project Structure

- `lib/` — Flutter app (settings, onboarding, playground)
- `ios/DraftRightKeyboard/` — iOS keyboard extension (Swift) — Plan 2
- `android/keyboard/` — Android keyboard extension (Kotlin) — Plan 3

## Building

### Flutter App

```bash
flutter pub get
flutter run
```

### Run Tests

```bash
flutter test
```

## Setup

1. Install and open the app
2. Follow onboarding to enable the DraftRight keyboard
3. Enter your OpenAI API key (or configure a custom server) in Settings
4. Use the Playground to test rewrites
5. Switch to any messaging app — the DraftRight toolbar appears above your keyboard
```

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "feat: add README and verify full build"
```
