import 'package:flutter/material.dart';
import 'package:draftright_mobile/models/language_module.dart';
import 'package:draftright_mobile/services/ime_pack_service.dart';

/// Languages section for Settings. Bundled languages (English, Vietnamese, …)
/// are always available; candidate languages (Japanese, Chinese, …) carry a
/// downloadable dictionary pack the user installs or removes here.
///
/// Depends on [PackInstaller] (not the concrete service) so it is testable
/// against a fake.
class LanguagePacksSection extends StatelessWidget {
  const LanguagePacksSection({
    super.key,
    required this.modules,
    required this.packInstaller,
    this.onLanguageEnabledChanged,
  });

  final List<LanguageModule> modules;
  final PackInstaller packInstaller;

  /// Fired when a downloadable language is installed (`enabled == true`) or
  /// removed (`false`), with the language id (e.g. `ja`). Lets the host add or
  /// drop the language from the keyboard cycle so downloading is a single step
  /// — install ⇒ in the cycle — instead of also having to toggle it on.
  final void Function(String languageId, bool enabled)? onLanguageEnabledChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final m in modules)
          m.requiresDownload
              ? _DownloadableLanguageTile(
                  module: m,
                  installer: packInstaller,
                  onEnabledChanged: onLanguageEnabledChanged,
                )
              : ListTile(
                  title: Text(m.displayName),
                  trailing: const Text('Included'),
                ),
      ],
    );
  }
}

class _DownloadableLanguageTile extends StatefulWidget {
  const _DownloadableLanguageTile({
    required this.module,
    required this.installer,
    this.onEnabledChanged,
  });

  final LanguageModule module;
  final PackInstaller installer;
  final void Function(String languageId, bool enabled)? onEnabledChanged;

  @override
  State<_DownloadableLanguageTile> createState() => _DownloadableLanguageTileState();
}

class _DownloadableLanguageTileState extends State<_DownloadableLanguageTile> {
  bool _installed = false;
  bool _busy = false;
  double _progress = 0;

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    final installed = await widget.installer.isInstalled(widget.module.pack!.packFileId);
    if (mounted) setState(() => _installed = installed);
  }

  Future<void> _download() async {
    final pack = widget.module.pack!;
    setState(() {
      _busy = true;
      _progress = 0;
    });
    try {
      await widget.installer.install(
        packId: pack.packFileId,
        url: pack.url,
        sha256: pack.sha256,
        sizeBytes: pack.sizeBytes,
        onProgress: (p) {
          if (mounted) setState(() => _progress = p);
        },
      );
      if (mounted) setState(() => _installed = true);
      // Single-step UX: a freshly downloaded language joins the keyboard cycle.
      widget.onEnabledChanged?.call(widget.module.id, true);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Download failed: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _remove() async {
    setState(() => _busy = true);
    try {
      await widget.installer.remove(widget.module.pack!.packFileId);
      if (mounted) setState(() => _installed = false);
      // Removing the pack drops the language from the cycle too.
      widget.onEnabledChanged?.call(widget.module.id, false);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final pack = widget.module.pack!;
    final Widget trailing;
    if (_busy) {
      trailing = SizedBox(
        width: 22,
        height: 22,
        child: CircularProgressIndicator(
          strokeWidth: 2.4,
          value: _progress > 0 && _progress < 1 ? _progress : null,
        ),
      );
    } else if (_installed) {
      trailing = TextButton(onPressed: _remove, child: const Text('Remove data'));
    } else {
      final size = pack.sizeLabel;
      trailing = TextButton(
        onPressed: _download,
        child: Text(size.isEmpty ? 'Download' : 'Download ($size)'),
      );
    }
    return ListTile(
      title: Text(widget.module.displayName),
      subtitle: _installed ? const Text('Installed') : null,
      trailing: trailing,
    );
  }
}
