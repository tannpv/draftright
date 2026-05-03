import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:url_launcher/url_launcher.dart';

class AboutScreen extends StatelessWidget {
  const AboutScreen({super.key});

  Future<void> _open(String url) async {
    final uri = Uri.parse(url);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
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
                  'from any app on your phone.\n\n'
                  'Type a message, email, or post — pick a tone, and '
                  'DraftRight rewrites it to match. Polished. Professional. '
                  'Casual. Or run a grammar check that catches issues other '
                  'apps miss.\n\n'
                  'The DraftRight keyboard works in every app you type in. '
                  'The share extension picks up text you select anywhere. '
                  'A web playground at draftright.info lets you try without '
                  'installing.',
                  style: TextStyle(height: 1.4),
                ),
              ),
              const SizedBox(height: 28),
              const Divider(),
              ListTile(
                leading: const Icon(Icons.public),
                title: const Text('Website'),
                subtitle: const Text('draftright.info'),
                onTap: () => _open('https://draftright.info'),
              ),
              ListTile(
                leading: const Icon(Icons.privacy_tip_outlined),
                title: const Text('Privacy policy'),
                onTap: () => _open('https://draftright.info/privacy'),
              ),
              ListTile(
                leading: const Icon(Icons.delete_outline),
                title: const Text('Delete account'),
                onTap: () => _open('https://draftright.info/delete-account'),
              ),
              ListTile(
                leading: const Icon(Icons.support_agent),
                title: const Text('Support'),
                subtitle: const Text('support@draftright.info'),
                onTap: () => _open('mailto:support@draftright.info'),
              ),
              const Divider(),
              const SizedBox(height: 8),
              Center(
                child: Text(
                  'Made by Tan Nguyen',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).hintColor,
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
