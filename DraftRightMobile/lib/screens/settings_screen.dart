import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/subscription_screen.dart';
import 'package:draftright_mobile/screens/change_password_screen.dart';
import 'package:draftright_mobile/screens/about_screen.dart';
import 'package:draftright_mobile/services/share_service.dart';
import 'package:draftright_mobile/widgets/report_bug_sheet.dart';
import 'package:draftright_mobile/widgets/suggest_feature_sheet.dart';
import 'package:draftright_mobile/models/language_module.dart';
import 'package:draftright_mobile/services/ime_manifest_client.dart';
import 'package:draftright_mobile/services/ime_pack_service.dart';
import 'package:draftright_mobile/widgets/language_packs_section.dart';
import 'package:draftright_mobile/models/tone.dart';

/// Tones the floating bubble can apply as its one-tap preset. Excludes
/// Grammar Check (returns structured issues, not a rewrite) and Translate
/// (needs a target language) — those aren't "rewrite in place" tones.
const List<Tone> _bubbleRewriteTones = [
  Tone.simple,
  Tone.natural,
  Tone.polished,
  Tone.concise,
  Tone.technical,
  Tone.claude,
];

/// Prominent-disclosure copy shown before the user opts in to in-place rewrite
/// (Play "Prominent Disclosure & Consent"). Kept in one place, not inlined.
/// VN copy + Play Console declaration:
/// docs/superpowers/plans/2026-07-23-android-bubble-a11y-play-declaration.md
const String _kInPlaceRewriteDisclosure =
    'To rewrite text right where you type it, DraftRight uses Android\'s '
    'Accessibility service. When — and only when — you tap the DraftRight '
    'bubble, it reads the text in the field you\'re editing and replaces it '
    'with your chosen rewrite.\n\n'
    '• It runs only on your tap — never in the background.\n'
    '• Your text is sent only to the DraftRight rewrite service you '
    'configured, to produce the rewrite. It is not sold, shared, or used '
    'for ads.\n'
    '• Password fields are always skipped.\n'
    '• You can turn this off anytime in Settings.';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

/// Loads the downloadable-language catalog from the server and lets the user
/// install/remove packs. Hidden silently on platforms without the shared-dir
/// channel (e.g. desktop) or when the catalog has no downloadable languages.
class _DownloadableLanguages extends StatefulWidget {
  final String baseUrl;
  const _DownloadableLanguages({required this.baseUrl});

  @override
  State<_DownloadableLanguages> createState() => _DownloadableLanguagesState();
}

class _DownloadableLanguagesState extends State<_DownloadableLanguages> {
  late final Future<({List<LanguageModule> modules, PackInstaller installer})?> _load =
      _resolve();

  Future<({List<LanguageModule> modules, PackInstaller installer})?> _resolve() async {
    try {
      final installer = await ImePackService.forPlatform();
      final modules =
          await ImeManifestClient(baseUrl: widget.baseUrl).fetchDownloadable();
      if (modules.isEmpty) return null;
      return (modules: modules, installer: installer);
    } catch (_) {
      return null; // no channel (desktop) / offline — just hide the section
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<({List<LanguageModule> modules, PackInstaller installer})?>(
      future: _load,
      builder: (context, snap) {
        if (snap.connectionState != ConnectionState.done) {
          return const Padding(
            padding: EdgeInsets.all(16),
            child: Center(child: SizedBox(
              width: 22, height: 22, child: CircularProgressIndicator(strokeWidth: 2.4))),
          );
        }
        final data = snap.data;
        if (data == null) {
          return const ListTile(
            dense: true,
            title: Text('No additional languages available',
                style: TextStyle(fontSize: 13, color: Colors.grey)),
          );
        }
        return LanguagePacksSection(
          modules: data.modules,
          packInstaller: data.installer,
          onLanguageEnabledChanged: (id, enabled) {
            // Install ⇒ add the language to the keyboard cycle; remove ⇒ drop
            // it. Keeps download a single step instead of also toggling a chip.
            final settings = context.read<SettingsService>();
            final next = List<String>.from(settings.enabledLanguageIds);
            if (enabled) {
              if (!next.contains(id)) next.add(id);
            } else {
              next.remove(id);
            }
            settings.setEnabledLanguageIds(next);
          },
        );
      },
    );
  }
}

class _FloatingBubbleTile extends StatelessWidget {
  final bool enabled;
  final ValueChanged<bool> onChanged;
  const _FloatingBubbleTile({required this.enabled, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Column(
        children: [
          SwitchListTile(
            secondary: const Icon(Icons.bubble_chart_outlined),
            title: const Text('Floating rewrite bubble'),
            subtitle: const Text(
              'A draggable button stays on screen. Type in any app, tap the bubble — your text is rewritten in place with your chosen tone. No copy-paste.',
            ),
            value: enabled,
            onChanged: onChanged,
          ),
          if (!enabled) const Padding(
            padding: EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: Text(
              'Asks once for "Display over other apps" and Accessibility, so the bubble can read the field you\'re typing in and replace it. Text is sent only to your DraftRight rewrite service, only when you tap the bubble.',
              style: TextStyle(fontSize: 12, color: Colors.grey),
            ),
          ),
        ],
      ),
    );
  }
}

class _SettingsScreenState extends State<SettingsScreen> {
  late TextEditingController _backendUrlController;

  @override
  void initState() {
    super.initState();
    final settings = context.read<SettingsService>();
    _backendUrlController = TextEditingController(text: settings.backendUrl);
  }

  @override
  void dispose() {
    _backendUrlController.dispose();
    super.dispose();
  }

  void _saveBackendUrl(SettingsService settings, String value) {
    final url = value.trim();
    if (url.isEmpty || url == settings.backendUrl) return;
    settings.setBackendUrl(url);
    context.read<AuthService>().setBaseUrl(url);
    _backendUrlController.text = settings.backendUrl; // reflect any normalization
    FocusScope.of(context).unfocus();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Backend → ${settings.backendUrl}')),
    );
  }

  Future<void> _setBubble(SettingsService settings, bool enable) async {
    final messenger = ScaffoldMessenger.of(context);
    if (!enable) {
      await ShareService.stopBubble();
      await settings.setFloatingBubbleEnabled(false);
      return;
    }
    final canDraw = await ShareService.canDrawOverlays();
    if (!canDraw) {
      // Send user to the system page to grant permission. We don't auto-toggle
      // back on after they return — the next time they tap the toggle, the
      // permission will be there.
      messenger.showSnackBar(const SnackBar(
        content: Text('Grant "Display over other apps", then enable again.'),
      ));
      await ShareService.openOverlaySettings();
      return;
    }
    // The bubble rewrites in place via the AccessibilityService — show the
    // prominent disclosure and get affirmative consent before enabling (Play policy).
    final consented = await _showInPlaceDisclosure();
    if (consented != true) return;
    final ok = await ShareService.startBubble();
    await settings.setFloatingBubbleEnabled(ok);
    if (!ok) {
      messenger.showSnackBar(const SnackBar(
        content: Text('Could not start the bubble. Try again.'),
      ));
      return;
    }
    // Guide the user to enable the AccessibilityService the bubble needs.
    messenger.showSnackBar(const SnackBar(
      content: Text('Enable "DraftRight" in Accessibility, then tap the bubble over a text field.'),
    ));
    await ShareService.openAccessibilitySettings();
  }

  Future<bool?> _showInPlaceDisclosure() {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Turn on one-tap rewrite in any app'),
        content: const SingleChildScrollView(
          child: Text(_kInPlaceRewriteDisclosure),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Not now'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Turn on'),
          ),
        ],
      ),
    );
  }

  Future<void> _logout() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Sign Out'),
        content: const Text('Are you sure you want to sign out?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Sign Out')),
        ],
      ),
    );
    if (confirm == true && mounted) {
      await context.read<AuthService>().logout();
    }
  }

  Future<void> _deleteAccount() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete Account'),
        content: const Text(
          'This permanently deletes your DraftRight account and all associated '
          'data — subscription, usage history, and saved settings. '
          'This cannot be undone.\n\n'
          'Are you sure you want to delete your account?',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirm != true || !mounted) return;
    try {
      await context.read<AuthService>().deleteAccount();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Your account has been deleted.')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not delete account: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer2<SettingsService, AuthService>(
      builder: (context, settings, auth, _) {
        return Scaffold(
          appBar: AppBar(title: const Text('Settings')),
          body: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // Account section
              const Text('Account', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 12),
              if (auth.isLoggedIn) ...[
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.workspace_premium),
                    title: const Text('Subscription'),
                    subtitle: const Text('View plan, usage, and upgrade'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      Navigator.of(context).push(
                        MaterialPageRoute(builder: (_) => const SubscriptionScreen()),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 8),
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.lock_outline),
                    title: const Text('Change Password'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      Navigator.of(context).push(
                        MaterialPageRoute(builder: (_) => const ChangePasswordScreen()),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 8),
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.logout, color: Colors.red),
                    title: const Text('Sign Out', style: TextStyle(color: Colors.red)),
                    onTap: _logout,
                  ),
                ),
                const SizedBox(height: 8),
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.delete_forever, color: Colors.red),
                    title: const Text('Delete Account', style: TextStyle(color: Colors.red)),
                    subtitle: const Text('Permanently delete your account and data'),
                    onTap: _deleteAccount,
                  ),
                ),
              ] else ...[
                Card(
                  child: ListTile(
                    leading: const Icon(Icons.login),
                    title: const Text('Sign In'),
                    subtitle: const Text('Sign in to use DraftRight'),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      // Navigate back to trigger login screen rebuild
                      context.read<AuthService>().logout();
                    },
                  ),
                ),
              ],

              const SizedBox(height: 24),
              const Text('Server', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 8),
              TextField(
                controller: _backendUrlController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                // Persist on every commit path — submit (Go key), focus loss
                // (tap outside), and the explicit Save button — so the URL can't
                // silently fail to stick (it used to save only on Go).
                onSubmitted: (value) => _saveBackendUrl(settings, value),
                onEditingComplete: () => _saveBackendUrl(settings, _backendUrlController.text),
                onTapOutside: (_) {
                  if (_backendUrlController.text != settings.backendUrl) {
                    _saveBackendUrl(settings, _backendUrlController.text);
                  }
                },
                decoration: InputDecoration(
                  labelText: 'Backend URL',
                  helperText: 'Prod: api.draftright.info  ·  Dev: api.dev.draftright.info',
                  border: const OutlineInputBorder(),
                  suffixIcon: IconButton(
                    icon: const Icon(Icons.save_outlined),
                    tooltip: 'Save backend URL',
                    onPressed: () => _saveBackendUrl(settings, _backendUrlController.text),
                  ),
                ),
              ),

              const SizedBox(height: 24),
              const Text('Translation', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: settings.translateLanguage,
                decoration: const InputDecoration(border: OutlineInputBorder()),
                items: SettingsService.supportedLanguages
                    .map((lang) => DropdownMenuItem(value: lang, child: Text(lang)))
                    .toList(),
                onChanged: (value) {
                  if (value != null) settings.setTranslateLanguage(value);
                },
              ),

              const SizedBox(height: 24),
              const Text('Keyboard languages',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const Text(
                'Pick which languages the DraftRight keyboard cycles through. '
                'Tap the globe key on the keyboard to switch.',
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: SettingsService.keyboardLanguageCatalog.entries.map((entry) {
                      final id = entry.key;
                      final label = entry.value;
                      final isEnabled = settings.enabledLanguageIds.contains(id);
                      return FilterChip(
                        label: Text(label),
                        selected: isEnabled,
                        onSelected: (selected) {
                          final next = List<String>.from(settings.enabledLanguageIds);
                          if (selected) {
                            if (!next.contains(id)) next.add(id);
                          } else {
                            next.remove(id);
                          }
                          settings.setEnabledLanguageIds(next);
                        },
                      );
                    }).toList(),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              const Text('Add a language (download)',
                  style: TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
              const Text(
                'Japanese, Chinese and more install a small dictionary pack on demand.',
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
              const SizedBox(height: 8),
              Card(child: _DownloadableLanguages(baseUrl: settings.backendUrl)),

              const SizedBox(height: 24),
              const Text('Floating Bubble',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 8),
              _FloatingBubbleTile(
                enabled: settings.floatingBubbleEnabled,
                onChanged: (value) => _setBubble(settings, value),
              ),
              if (settings.floatingBubbleEnabled) ...[
                const SizedBox(height: 8),
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('One-tap tone',
                            style: TextStyle(fontWeight: FontWeight.w600)),
                        const SizedBox(height: 4),
                        const Text(
                          'The bubble rewrites your text in place using this tone — like '
                          'One-Click mode on Mac/Windows.',
                          style: TextStyle(fontSize: 12, color: Colors.grey),
                        ),
                        const SizedBox(height: 12),
                        DropdownButtonFormField<String>(
                          value: settings.bubblePresetTone,
                          decoration: const InputDecoration(border: OutlineInputBorder()),
                          items: _bubbleRewriteTones
                              .map((t) => DropdownMenuItem(
                                    value: t.apiValue,
                                    child: Row(children: [
                                      Icon(t.icon, size: 18),
                                      const SizedBox(width: 8),
                                      Text(t.displayName),
                                    ]),
                                  ))
                              .toList(),
                          onChanged: (v) {
                            if (v != null) settings.setBubblePresetTone(v);
                          },
                        ),
                      ],
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 8),
              Card(
                child: SwitchListTile(
                  secondary: const Icon(Icons.exit_to_app),
                  title: const Text('Auto-return to your app'),
                  subtitle: const Text(
                    'After a rewrite, DraftRight closes and returns you to the app you were in. Just paste.',
                  ),
                  value: settings.autoCloseAfterRewrite,
                  onChanged: (v) => settings.setAutoCloseAfterRewrite(v),
                ),
              ),

              const SizedBox(height: 24),
              const Text('Help', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 8),
              Card(
                child: ListTile(
                  leading: const Icon(Icons.bug_report_outlined),
                  title: const Text('Report a bug'),
                  subtitle: const Text(
                    'Tell us what went wrong and attach a screenshot.',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    // Route the bug-report POST through the currently
                    // configured backend (Settings → Server → Backend
                    // URL) — otherwise the sheet defaults to prod and
                    // dev-env testing silently posts to the wrong DB.
                    showReportBugSheet(
                      context,
                      currentRoute: ModalRoute.of(context)?.settings.name ??
                          'SettingsScreen',
                      endpointOverride:
                          '${settings.backendUrl.replaceAll(RegExp(r"/+$"), "")}/bug-reports',
                    );
                  },
                ),
              ),
              const SizedBox(height: 8),
              Card(
                child: ListTile(
                  leading: const Icon(Icons.lightbulb_outline),
                  title: const Text('Suggest a feature'),
                  subtitle: const Text(
                    'Got an idea? Tell us what you\'d like to see.',
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => showSuggestFeatureSheet(context),
                ),
              ),
              const SizedBox(height: 8),
              Card(
                child: ListTile(
                  leading: const Icon(Icons.info_outline),
                  title: const Text('About DraftRight'),
                  subtitle: const Text('Version, links, support'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(builder: (_) => const AboutScreen()),
                    );
                  },
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
