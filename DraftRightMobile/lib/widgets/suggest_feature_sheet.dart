import 'dart:io';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/feedback_service.dart';

/// Opens the "Suggest a feature" sheet.  Bottom sheet on Android;
/// Cupertino modal popup on iOS — mirroring [showReportBugSheet].
///
/// [endpointOverride] redirects the submission target (integration tests).
Future<void> showSuggestFeatureSheet(
  BuildContext context, {
  String? endpointOverride,
}) async {
  if (Platform.isIOS) {
    await showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) =>
          _SuggestFeatureSheet(endpointOverride: endpointOverride),
    );
  } else {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) =>
          _SuggestFeatureSheet(endpointOverride: endpointOverride),
    );
  }
}

// ---------------------------------------------------------------------------
// Platform options presented in the dropdown.
// ---------------------------------------------------------------------------
const _kPlatformOptions = <({String value, String label})>[
  (value: 'mobile', label: 'Mobile'),
  (value: 'playground', label: 'Playground'),
  (value: 'windows', label: 'Windows'),
  (value: 'mac', label: 'macOS'),
  (value: 'linux', label: 'Linux'),
];

class _SuggestFeatureSheet extends StatefulWidget {
  final String? endpointOverride;
  const _SuggestFeatureSheet({this.endpointOverride});

  @override
  State<_SuggestFeatureSheet> createState() => _SuggestFeatureSheetState();
}

class _SuggestFeatureSheetState extends State<_SuggestFeatureSheet> {
  final _titleController = TextEditingController();
  final _detailsController = TextEditingController();
  final _emailController = TextEditingController();

  String _selectedPlatform = 'mobile';
  bool _submitting = false;

  bool get _canSubmit =>
      _titleController.text.trim().isNotEmpty &&
      _detailsController.text.trim().isNotEmpty;

  @override
  void initState() {
    super.initState();
    // Rebuild the submit button whenever text changes.
    _titleController.addListener(_rebuild);
    _detailsController.addListener(_rebuild);
  }

  void _rebuild() => setState(() {});

  @override
  void dispose() {
    _titleController.dispose();
    _detailsController.dispose();
    _emailController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_submitting || !_canSubmit) return;

    setState(() => _submitting = true);
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    final auth = context.read<AuthService>();

    final ok = await FeedbackService.submitFeatureRequest(
      title: _titleController.text.trim(),
      targetPlatform: _selectedPlatform,
      description: _detailsController.text.trim(),
      userEmail: auth.isLoggedIn ? null : _emailController.text.trim(),
      authToken: auth.accessToken,
      endpointOverride: widget.endpointOverride,
    );

    if (!mounted) return;
    setState(() => _submitting = false);

    if (ok) {
      navigator.pop();
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Feature request submitted — thanks!'),
        ),
      );
    } else {
      messenger.showSnackBar(
        const SnackBar(
          content: Text("Couldn't submit — try again"),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthService>();
    final isLoggedIn = auth.isLoggedIn;
    final viewInsets = MediaQuery.of(context).viewInsets;
    final isIOS = Platform.isIOS;

    final content = Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: 16 + viewInsets.bottom,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // Header row.
            Row(
              children: [
                const Icon(Icons.lightbulb_outline),
                const SizedBox(width: 8),
                const Text(
                  'Suggest a feature',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                ),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed:
                      _submitting ? null : () => Navigator.of(context).pop(),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Title field.
            TextField(
              controller: _titleController,
              maxLength: 80,
              textCapitalization: TextCapitalization.sentences,
              decoration: const InputDecoration(
                labelText: 'Feature title',
                hintText: 'Short summary of your idea',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 8),

            // Platform dropdown.
            DropdownButtonFormField<String>(
              initialValue: _selectedPlatform,
              decoration: const InputDecoration(
                labelText: 'Target platform',
                border: OutlineInputBorder(),
              ),
              items: _kPlatformOptions
                  .map((opt) => DropdownMenuItem(
                        value: opt.value,
                        child: Text(opt.label),
                      ))
                  .toList(),
              onChanged: _submitting
                  ? null
                  : (val) {
                      if (val != null) setState(() => _selectedPlatform = val);
                    },
            ),
            const SizedBox(height: 8),

            // Details field.
            TextField(
              controller: _detailsController,
              minLines: 4,
              maxLines: 8,
              maxLength: 2000,
              textCapitalization: TextCapitalization.sentences,
              decoration: const InputDecoration(
                labelText: 'Details',
                hintText:
                    'Describe what the feature should do and why it would help.',
                border: OutlineInputBorder(),
              ),
            ),

            // Email field — shown only for anonymous (not logged-in) users.
            if (!isLoggedIn) ...[
              const SizedBox(height: 8),
              TextField(
                controller: _emailController,
                keyboardType: TextInputType.emailAddress,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Your email (optional)',
                  hintText: 'so we can follow up',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
            const SizedBox(height: 8),

            // "See all requests" link.
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                icon: const Icon(Icons.open_in_new, size: 16),
                label: const Text('See all requests →'),
                onPressed: () => launchUrl(
                  Uri.parse('https://draftright.info/feedback'),
                  mode: LaunchMode.externalApplication,
                ),
              ),
            ),
            const SizedBox(height: 12),

            // Action row.
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: _submitting
                        ? null
                        : () => Navigator.of(context).pop(),
                    child: const Text('Cancel'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton.icon(
                    icon: _submitting
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : const Icon(Icons.send),
                    label: Text(_submitting ? 'Sending...' : 'Submit'),
                    onPressed: (_submitting || !_canSubmit) ? null : _submit,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );

    if (isIOS) {
      // Cupertino modal popup gives us a translucent backdrop; wrap the
      // form in a Material so TextField + Material widgets render correctly.
      return SafeArea(
        top: false,
        child: Container(
          decoration: const BoxDecoration(
            color: CupertinoColors.systemBackground,
            borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
          ),
          child: Material(
            color: Colors.transparent,
            child: content,
          ),
        ),
      );
    }
    return content;
  }
}
