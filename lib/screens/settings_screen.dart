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
              const Text('AI Provider', style: TextStyle(fontWeight: FontWeight.bold)),
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
                    _endpointController.text = 'http://localhost:11434/v1/chat/completions';
                    _modelController.text = 'llama3';
                    settings.setEndpoint(_endpointController.text);
                    settings.setModel(_modelController.text);
                  } else {
                    _endpointController.text = 'https://api.openai.com/v1/chat/completions';
                    _modelController.text = 'gpt-4o-mini';
                    settings.setEndpoint(_endpointController.text);
                    settings.setModel(_modelController.text);
                  }
                },
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _apiKeyController,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: settings.aiProvider == 'openai' ? 'API Key' : 'API Key (optional)',
                  border: const OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setApiKey(value),
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _endpointController,
                decoration: const InputDecoration(
                  labelText: 'Server URL',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setEndpoint(value),
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _modelController,
                decoration: const InputDecoration(
                  labelText: 'Model',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) => settings.setModel(value),
              ),
              const SizedBox(height: 16),
              Row(
                children: [
                  const Text('Temperature'),
                  Expanded(
                    child: Slider(
                      value: settings.temperature,
                      min: 0, max: 1, divisions: 20,
                      onChanged: (value) => settings.setTemperature(value),
                    ),
                  ),
                  Text(settings.temperature.toStringAsFixed(2)),
                ],
              ),
              const SizedBox(height: 24),
              const Text('Translation Language', style: TextStyle(fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                // ignore: deprecated_member_use
                value: settings.translateLanguage,
                decoration: const InputDecoration(border: OutlineInputBorder()),
                items: SettingsService.supportedLanguages
                    .map((lang) => DropdownMenuItem(value: lang, child: Text(lang)))
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
