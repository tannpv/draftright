import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:url_launcher/url_launcher.dart';

class AboutScreen extends StatelessWidget {
  const AboutScreen({super.key});

  Future<void> _open(BuildContext context, String url) async {
    final messenger = ScaffoldMessenger.of(context);
    final uri = Uri.parse(url);
    // Try external first (browser / mail client), fall back to in-app
    // browser if that fails. Don't gate on canLaunchUrl — on Android 11+
    // it can return false even when a browser is installed if the
    // <queries> block is incomplete, leaving links as silent no-ops.
    try {
      final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (ok) return;
    } catch (_) {/* fall through */}
    try {
      final ok = await launchUrl(uri, mode: LaunchMode.platformDefault);
      if (ok) return;
    } catch (_) {/* fall through */}
    messenger.showSnackBar(
      SnackBar(content: Text('Could not open $url')),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('About')),
      body: FutureBuilder<PackageInfo>(
        future: PackageInfo.fromPlatform(),
        builder: (context, snap) {
          if (!snap.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final info = snap.data!;
          return ListView(
            padding: const EdgeInsets.all(20),
            children: [
              const SizedBox(height: 8),
              Center(
                child: Image.asset(
                  'assets/icon.png',
                  width: 96,
                  height: 96,
                  errorBuilder: (_, __, ___) => const Icon(Icons.edit_note,
                      size: 96, color: Colors.blueAccent),
                ),
              ),
              const SizedBox(height: 16),
              Center(
                child: Text(
                  info.appName.isNotEmpty ? info.appName : 'DraftRight',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                ),
              ),
              const SizedBox(height: 4),
              Center(
                child: Text(
                  'Version ${info.version} (${info.buildNumber})',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: Theme.of(context).hintColor,
                      ),
                ),
              ),
              const SizedBox(height: 28),
              const Padding(
                padding: EdgeInsets.symmetric(horizontal: 4),
                child: Text(
                  'DraftRight rewrites your text in any tone, instantly, '
                  'in any app on your phone.\n\n'
                  'Type a message, email, or post — long-press the text, '
                  'tap DraftRight in the popup, pick a tone. Polished. '
                  'Professional. Concise. Translate. Or run a grammar '
                  'check that catches issues other apps miss.\n\n'
                  'Works alongside the keyboard you already use, in any '
                  'language — Vietnamese, Chinese, Japanese, Korean, and '
                  'more — because typing stays with your familiar keyboard. '
                  'Power users can also install the optional DraftRight '
                  'Keyboard for one-tap rewriting while typing in English.\n\n'
                  'A web playground at draftright.info lets you try without '
                  'installing.',
                  style: TextStyle(height: 1.4),
                ),
              ),
              const SizedBox(height: 28),
              const Divider(),
              ListTile(
                leading: const Icon(Icons.help_outline),
                title: const Text('Help & FAQ'),
                subtitle: const Text('Setup, tones, troubleshooting'),
                onTap: () => _open(context, 'https://draftright.info/help/mobile/'),
              ),
              ListTile(
                leading: const Icon(Icons.public),
                title: const Text('Website'),
                subtitle: const Text('draftright.info'),
                onTap: () => _open(context, 'https://draftright.info'),
              ),
              ListTile(
                leading: const Icon(Icons.privacy_tip_outlined),
                title: const Text('Privacy policy'),
                onTap: () => _open(context, 'https://draftright.info/privacy'),
              ),
              ListTile(
                leading: const Icon(Icons.support_agent),
                title: const Text('Support'),
                subtitle: const Text('support@draftright.info'),
                onTap: () => _open(context, 'mailto:support@draftright.info'),
              ),
              const Divider(),
              const SizedBox(height: 8),
              Center(
                child: Text(
                  'Made by Southern Martin',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).hintColor,
                      ),
                ),
              ),
              const SizedBox(height: 4),
              Center(
                child: GestureDetector(
                  onTap: () => _open(context, 'https://southernmartin.com'),
                  child: Text(
                    'southernmartin.com',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Theme.of(context).colorScheme.primary,
                        ),
                  ),
                ),
              ),
              const SizedBox(height: 24),
            ],
          );
        },
      ),
    );
  }
}
