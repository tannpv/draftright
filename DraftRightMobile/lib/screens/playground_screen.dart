import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/subscription_screen.dart';
import 'package:draftright_mobile/widgets/nudge_host.dart';

class PlaygroundScreen extends StatefulWidget {
  const PlaygroundScreen({super.key});

  @override
  State<PlaygroundScreen> createState() => _PlaygroundScreenState();
}

class _PlaygroundScreenState extends State<PlaygroundScreen> {
  final TextEditingController _textController = TextEditingController();
  Tone? _selectedTone;
  String? _result;
  bool _isLoading = false;
  String? _error;
  late final BackendClient _backend;

  @override
  void initState() {
    super.initState();
    final auth = context.read<AuthService>();
    final settings = context.read<SettingsService>();
    _backend = BackendClient(
      auth: auth,
      getBaseUrl: () => settings.backendUrl,
    );
  }

  @override
  void dispose() {
    _textController.dispose();
    super.dispose();
  }

  Future<void> _rewrite(Tone tone) async {
    final text = _textController.text.trim();
    if (text.isEmpty) return;

    setState(() {
      _selectedTone = tone;
      _isLoading = true;
      _error = null;
      _result = null;
    });

    try {
      final result = await _backend.rewrite(
        text: text,
        tone: tone,
        targetLanguage: tone == Tone.translate ? context.read<SettingsService>().translateLanguage : null,
      );
      setState(() {
        _result = result.rewrittenText;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Playground')),
      body: NudgeHost(
        backend: _backend,
        onUpgrade: () => Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
        ),
        child: Padding(
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
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: Tone.values.map((tone) {
                  final isSelected = _selectedTone == tone;
                  return Padding(
                    padding: const EdgeInsets.only(right: 6),
                    child: ChoiceChip(
                      avatar: Icon(tone.icon, size: 18),
                      label: Text(tone.displayName),
                      selected: isSelected,
                      onSelected: _isLoading ? null : (_) => _rewrite(tone),
                    ),
                  );
                }).toList(),
              ),
            ),
            const SizedBox(height: 16),
            Expanded(
              child: _isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              SelectableText(
                                _error!,
                                style: const TextStyle(color: Colors.red),
                                textAlign: TextAlign.center,
                              ),
                              const SizedBox(height: 8),
                              TextButton.icon(
                                onPressed: () {
                                  Clipboard.setData(ClipboardData(text: _error!));
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(content: Text('Error copied')),
                                  );
                                },
                                icon: const Icon(Icons.copy, size: 14),
                                label: const Text('Copy Error'),
                              ),
                            ],
                          ),
                        )
                      : _result != null
                          ? Container(
                              padding: const EdgeInsets.all(12),
                              decoration: BoxDecoration(
                                color: Colors.green.withValues(alpha: 0.05),
                                borderRadius: BorderRadius.circular(8),
                                border: Border.all(color: Colors.green.withValues(alpha: 0.2)),
                              ),
                              child: SingleChildScrollView(
                                child: SelectableText(_result!, style: const TextStyle(fontSize: 15)),
                              ),
                            )
                          : const Center(
                              child: Text(
                                'Pick a tone to rewrite your text',
                                style: TextStyle(color: Colors.grey),
                              ),
                            ),
            ),
            if (_result != null)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: FilledButton.icon(
                  onPressed: () {
                    Clipboard.setData(ClipboardData(text: _result!));
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
      ),
    );
  }
}
