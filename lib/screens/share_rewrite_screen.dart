import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/settings_service.dart';

/// Lightweight tone-picker that shows when the user reaches DraftRight via
/// the system Share sheet.  Optimised for speed: paste the shared text in
/// place, two taps to a rewrite, auto-copy result, optional dismiss.
class ShareRewriteScreen extends StatefulWidget {
  final String sharedText;
  const ShareRewriteScreen({super.key, required this.sharedText});

  @override
  State<ShareRewriteScreen> createState() => _ShareRewriteScreenState();
}

class _ShareRewriteScreenState extends State<ShareRewriteScreen> {
  Tone? _running;
  String? _result;
  String? _error;

  Future<void> _rewrite(Tone tone) async {
    if (_running != null) return;
    setState(() { _running = tone; _result = null; _error = null; });

    final auth = context.read<AuthService>();
    final settings = context.read<SettingsService>();
    final client = BackendClient(
      auth: auth,
      getBaseUrl: () => settings.backendUrl,
    );

    try {
      final res = await client.rewrite(
        text: widget.sharedText,
        tone: tone,
        targetLanguage: tone == Tone.translate ? settings.translateLanguage : null,
      );
      final out = res.isGrammarCheck
          ? _formatGrammar(res.grammarResult!)
          : res.rewrittenText;
      await Clipboard.setData(ClipboardData(text: out));
      if (!mounted) return;
      setState(() { _result = out; _running = null; });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e.toString(); _running = null; });
    }
  }

  String _formatGrammar(GrammarResult g) {
    if (g.issues.isEmpty) return 'No issues found. Score: ${g.score}/100';
    final lines = ['Score: ${g.score}/100', ''];
    for (final i in g.issues) {
      lines.add('• "${i.original}" → "${i.suggestion}"');
      if (i.reason.isNotEmpty) lines.add('  ${i.reason}');
    }
    return lines.join('\n');
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Rewrite'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => Navigator.of(context).maybePop(),
          tooltip: 'Close',
        ),
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Source text
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Shared text',
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                              color: Theme.of(context).hintColor,
                            )),
                    const SizedBox(height: 6),
                    Text(widget.sharedText,
                        maxLines: 6,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(height: 1.35)),
                  ],
                ),
              ),
              const SizedBox(height: 16),

              // Tone grid
              Text('Pick a tone',
                  style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: 8),
              Expanded(
                child: GridView.count(
                  crossAxisCount: 2,
                  childAspectRatio: 2.6,
                  mainAxisSpacing: 8,
                  crossAxisSpacing: 8,
                  children: Tone.values.map(_toneButton).toList(),
                ),
              ),

              // Result / error / progress
              if (_running != null) ...[
                const SizedBox(height: 8),
                const Row(children: [
                  SizedBox(
                    width: 16, height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                  SizedBox(width: 12),
                  Text('Rewriting…'),
                ]),
              ],
              if (_error != null) ...[
                const SizedBox(height: 8),
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.errorContainer,
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(_error!,
                      style: TextStyle(
                          color: Theme.of(context).colorScheme.onErrorContainer)),
                ),
              ],
              if (_result != null) ...[
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.primaryContainer,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(children: [
                        const Icon(Icons.check_circle, size: 16),
                        const SizedBox(width: 6),
                        Text('Copied to clipboard',
                            style: Theme.of(context).textTheme.labelMedium),
                      ]),
                      const SizedBox(height: 6),
                      Text(_result!,
                          maxLines: 6,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(height: 1.35)),
                      const SizedBox(height: 10),
                      Row(children: [
                        OutlinedButton.icon(
                          onPressed: () async {
                            final messenger = ScaffoldMessenger.of(context);
                            await Clipboard.setData(ClipboardData(text: _result!));
                            if (!mounted) return;
                            messenger.showSnackBar(
                              const SnackBar(content: Text('Copied again')),
                            );
                          },
                          icon: const Icon(Icons.copy, size: 16),
                          label: const Text('Copy'),
                        ),
                        const SizedBox(width: 8),
                        FilledButton.icon(
                          onPressed: () => Navigator.of(context).maybePop(),
                          icon: const Icon(Icons.check, size: 16),
                          label: const Text('Done'),
                        ),
                      ]),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Widget _toneButton(Tone t) {
    final isRunning = _running == t;
    return FilledButton.tonal(
      onPressed: _running != null ? null : () => _rewrite(t),
      child: isRunning
          ? const SizedBox(
              width: 16, height: 16,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : Text(t.displayName),
    );
  }
}
