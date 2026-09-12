import 'package:flutter/material.dart';

import '../services/keyboard_status_service.dart';

/// Tells the user their DraftRight keyboard is installed but unusable, and
/// offers the one tap that fixes it (#272).
///
/// Renders nothing when the keyboard is active or the platform can't say, so it
/// can be dropped onto any screen without a visibility check at the call site.
/// Re-checks when the app returns to the foreground, since the fix happens in
/// system settings and the user comes back expecting the banner to be gone.
class KeyboardEnableBanner extends StatefulWidget {
  const KeyboardEnableBanner({super.key, this.service});

  /// Injectable for tests; defaults to the real platform channel.
  final KeyboardStatusService? service;

  @override
  State<KeyboardEnableBanner> createState() => _KeyboardEnableBannerState();
}

class _KeyboardEnableBannerState extends State<KeyboardEnableBanner>
    with WidgetsBindingObserver {
  late final KeyboardStatusService _service =
      widget.service ?? KeyboardStatusService();
  KeyboardStatus _status = KeyboardStatus.unknown;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _refresh();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _refresh();
  }

  Future<void> _refresh() async {
    final status = await _service.status();
    if (mounted) setState(() => _status = status);
  }

  @override
  Widget build(BuildContext context) {
    final copy = _copyFor(_status);
    if (copy == null) return const SizedBox.shrink();

    return Card(
      color: Theme.of(context).colorScheme.errorContainer,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.keyboard_alt_outlined),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    copy.title,
                    style: const TextStyle(
                        fontWeight: FontWeight.w600, fontSize: 15),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(copy.body, style: const TextStyle(fontSize: 13)),
            const SizedBox(height: 12),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton(
                onPressed: () async {
                  await copy.action(_service);
                  await _refresh();
                },
                child: Text(copy.buttonLabel),
              ),
            ),
          ],
        ),
      ),
    );
  }

  static _BannerCopy? _copyFor(KeyboardStatus status) {
    switch (status) {
      case KeyboardStatus.notEnabled:
        return _BannerCopy(
          title: 'Turn on the DraftRight keyboard',
          body: 'Android does not switch a new keyboard on by itself. Enable '
              'DraftRight in your keyboard settings, then pick it with the '
              'globe key.',
          buttonLabel: 'Open keyboard settings',
          action: (s) => s.openKeyboardSettings(),
        );
      case KeyboardStatus.enabledNotSelected:
        return _BannerCopy(
          title: 'Switch to the DraftRight keyboard',
          body: 'DraftRight is enabled but another keyboard is in use.',
          buttonLabel: 'Choose keyboard',
          action: (s) => s.openKeyboardPicker(),
        );
      case KeyboardStatus.active:
      case KeyboardStatus.unknown:
        return null;
    }
  }
}

class _BannerCopy {
  const _BannerCopy({
    required this.title,
    required this.body,
    required this.buttonLabel,
    required this.action,
  });

  final String title;
  final String body;
  final String buttonLabel;
  final Future<void> Function(KeyboardStatusService) action;
}
