import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/screens/subscription_screen.dart';
import 'package:draftright_mobile/screens/change_password_screen.dart';
import 'package:draftright_mobile/screens/about_screen.dart';
import 'package:draftright_mobile/services/share_service.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
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
            title: const Text('Show floating bubble'),
            subtitle: const Text(
              'A draggable button stays on screen. Copy text, tap the bubble, pick a tone — paste back. Works in any app.',
            ),
            value: enabled,
            onChanged: onChanged,
          ),
          if (!enabled) const Padding(
            padding: EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: Text(
              'Asks for "Display over other apps" permission once. We do not read your screen — only the clipboard, and only when you tap the bubble.',
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
    final ok = await ShareService.startBubble();
    await settings.setFloatingBubbleEnabled(ok);
    if (!ok) {
      messenger.showSnackBar(const SnackBar(
        content: Text('Could not start the bubble. Try again.'),
      ));
    }
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
                decoration: const InputDecoration(
                  labelText: 'Backend URL',
                  helperText: 'Leave default unless self-hosting',
                  border: OutlineInputBorder(),
                ),
                onChanged: (value) {
                  if (value.isNotEmpty) {
                    settings.setBackendUrl(value);
                    context.read<AuthService>().setBaseUrl(value);
                  }
                },
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
              const Text('Floating Bubble',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
              const SizedBox(height: 8),
              _FloatingBubbleTile(
                enabled: settings.floatingBubbleEnabled,
                onChanged: (value) => _setBubble(settings, value),
              ),
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
              const Text('About', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
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
