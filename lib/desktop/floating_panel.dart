import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/models/tone.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/backend_client.dart';
import 'package:draftright_mobile/services/settings_service.dart';

/// The floating rewrite panel shown when the global hotkey is triggered.
///
/// This widget is the entire content of the (frameless, always-on-top) main
/// window when in "floating panel" mode.  The caller is responsible for making
/// the Flutter window visible/invisible via window_manager; this widget just
/// renders the UI and calls back when the user confirms/cancels.
class FloatingPanel extends StatefulWidget {
  /// Text that was in the clipboard when the hotkey fired.
  final String originalText;

  /// Called when the user chooses Replace or Copy — hands the result back so
  /// desktop_app.dart can write it to the clipboard (and optionally paste).
  final void Function(String rewritten, {required bool replace}) onResult;

  /// Called when the user cancels / presses Escape.
  final VoidCallback onCancel;

  const FloatingPanel({
    super.key,
    required this.originalText,
    required this.onResult,
    required this.onCancel,
  });

  @override
  State<FloatingPanel> createState() => _FloatingPanelState();
}

class _FloatingPanelState extends State<FloatingPanel> {
  Tone? _selectedTone;
  String? _rewrittenText;
  bool _isLoading = false;
  String? _errorMessage;

  // BackendClient is created per-request using providers

  @override
  Widget build(BuildContext context) {
    final settings = context.watch<SettingsService>();

    return KeyboardListener(
      focusNode: FocusNode()..requestFocus(),
      autofocus: true,
      onKeyEvent: (event) {
        if (event is KeyDownEvent &&
            event.logicalKey == LogicalKeyboardKey.escape) {
          widget.onCancel();
        }
      },
      child: Material(
        color: Colors.transparent,
        child: Container(
          width: 400,
          constraints: const BoxConstraints(minHeight: 350, maxHeight: 500),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surface,
            borderRadius: BorderRadius.circular(12),
            boxShadow: [
              BoxShadow(
                color: Colors.black.withOpacity(0.3),
                blurRadius: 20,
                offset: const Offset(0, 8),
              ),
            ],
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _buildTitleBar(),
              const Divider(height: 1),
              _buildOriginalText(),
              const Divider(height: 1),
              _buildToneButtons(settings),
              if (_isLoading) _buildLoadingState(),
              if (_rewrittenText != null && !_isLoading) _buildDiffView(),
              if (_errorMessage != null && !_isLoading) _buildError(),
              const Divider(height: 1),
              _buildActionButtons(),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildTitleBar() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.primary,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 8),
          const Text(
            'DraftRight',
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
          ),
          const Spacer(),
          Text(
            'Esc to dismiss',
            style: TextStyle(
              fontSize: 11,
              color: Theme.of(context).colorScheme.outline,
            ),
          ),
          const SizedBox(width: 8),
          InkWell(
            onTap: widget.onCancel,
            borderRadius: BorderRadius.circular(4),
            child: const Padding(
              padding: EdgeInsets.all(4),
              child: Icon(Icons.close, size: 16),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildOriginalText() {
    final preview = widget.originalText.length > 150
        ? '${widget.originalText.substring(0, 150)}…'
        : widget.originalText;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      width: double.infinity,
      color: Theme.of(context).colorScheme.surfaceVariant.withOpacity(0.5),
      child: Text(
        preview,
        style: TextStyle(
          fontSize: 12,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
        maxLines: 3,
        overflow: TextOverflow.ellipsis,
      ),
    );
  }

  Widget _buildToneButtons(SettingsService settings) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      child: Wrap(
        spacing: 6,
        runSpacing: 6,
        children: Tone.values.map((tone) {
          final isSelected = _selectedTone == tone;
          return ActionChip(
            avatar: Icon(
              _toneIcon(tone),
              size: 14,
              color: isSelected
                  ? Theme.of(context).colorScheme.onPrimary
                  : Theme.of(context).colorScheme.primary,
            ),
            label: Text(
              tone.displayName,
              style: TextStyle(
                fontSize: 12,
                color: isSelected
                    ? Theme.of(context).colorScheme.onPrimary
                    : null,
              ),
            ),
            backgroundColor: isSelected
                ? Theme.of(context).colorScheme.primary
                : null,
            onPressed: _isLoading ? null : () => _rewrite(tone, settings),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildLoadingState() {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 16),
      child: Column(
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 8),
          Text('Rewriting…', style: TextStyle(fontSize: 12)),
        ],
      ),
    );
  }

  Widget _buildDiffView() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Original column
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Original',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.error,
                  ),
                ),
                const SizedBox(height: 4),
                Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: Theme.of(context)
                        .colorScheme
                        .errorContainer
                        .withOpacity(0.3),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    widget.originalText,
                    style: const TextStyle(fontSize: 12),
                    maxLines: 8,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          // Rewritten column
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Rewritten',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
                const SizedBox(height: 4),
                Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: Theme.of(context)
                        .colorScheme
                        .primaryContainer
                        .withOpacity(0.3),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(
                    _rewrittenText!,
                    style: const TextStyle(fontSize: 12),
                    maxLines: 8,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildError() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Text(
        _errorMessage!,
        style: TextStyle(
          color: Theme.of(context).colorScheme.error,
          fontSize: 12,
        ),
      ),
    );
  }

  Widget _buildActionButtons() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          TextButton(
            onPressed: widget.onCancel,
            child: const Text('Cancel'),
          ),
          const SizedBox(width: 8),
          if (_rewrittenText != null) ...[
            OutlinedButton.icon(
              icon: const Icon(Icons.copy, size: 14),
              label: const Text('Copy'),
              onPressed: () => widget.onResult(
                _rewrittenText!,
                replace: false,
              ),
            ),
            const SizedBox(width: 8),
            FilledButton.icon(
              icon: const Icon(Icons.check, size: 14),
              label: const Text('Replace'),
              onPressed: () => widget.onResult(
                _rewrittenText!,
                replace: true,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _rewrite(Tone tone, SettingsService settings) async {
    setState(() {
      _selectedTone = tone;
      _isLoading = true;
      _rewrittenText = null;
      _errorMessage = null;
    });

    try {
      final auth = context.read<AuthService>();
      final client = BackendClient(
        auth: auth,
        getBaseUrl: () => settings.backendUrl,
      );
      final result = await client.rewrite(
        text: widget.originalText,
        tone: tone,
        targetLanguage: tone == Tone.translate ? settings.translateLanguage : null,
      );
      if (mounted) {
        setState(() {
          _rewrittenText = result.rewrittenText;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = 'Error: $e';
          _isLoading = false;
        });
      }
    }
  }

  IconData _toneIcon(Tone tone) {
    switch (tone) {
      case Tone.simple:
        return Icons.text_fields;
      case Tone.natural:
        return Icons.chat_bubble_outline;
      case Tone.polished:
        return Icons.auto_awesome;
      case Tone.concise:
        return Icons.compress;
      case Tone.technical:
        return Icons.build;
      case Tone.translate:
        return Icons.language;
    }
  }
}
