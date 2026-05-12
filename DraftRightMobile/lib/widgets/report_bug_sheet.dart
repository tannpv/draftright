import 'dart:io';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/bug_report_service.dart';

/// Opens the "Report a bug" sheet. Bottom sheet on Android; Cupertino
/// modal popup on iOS for native feel.
///
/// [currentRoute] is included in the report's `context` JSON so triagers
/// can see what screen the user was on when they reported.
/// [endpointOverride] redirects the submission target — production by
/// default; integration tests point it at a local stub server.
Future<void> showReportBugSheet(
  BuildContext context, {
  String? currentRoute,
  String? endpointOverride,
}) async {
  if (Platform.isIOS) {
    await showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) => _ReportBugSheet(
        currentRoute: currentRoute,
        endpointOverride: endpointOverride,
      ),
    );
  } else {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => _ReportBugSheet(
        currentRoute: currentRoute,
        endpointOverride: endpointOverride,
      ),
    );
  }
}

class _ReportBugSheet extends StatefulWidget {
  final String? currentRoute;
  final String? endpointOverride;
  const _ReportBugSheet({this.currentRoute, this.endpointOverride});

  @override
  State<_ReportBugSheet> createState() => _ReportBugSheetState();
}

class _ReportBugSheetState extends State<_ReportBugSheet> {
  final _descriptionController = TextEditingController();
  final _emailController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  final _picker = ImagePicker();

  File? _screenshot;
  bool _submitting = false;

  static final _emailRegex = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');

  @override
  void dispose() {
    _descriptionController.dispose();
    _emailController.dispose();
    super.dispose();
  }

  Future<void> _pickImage(ImageSource source) async {
    try {
      final picked = await _picker.pickImage(
        source: source,
        imageQuality: 85,
      );
      if (picked == null) return;
      final file = File(picked.path);
      final length = await file.length();
      if (length > BugReportService.maxScreenshotBytes) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Screenshot too large (max 5 MB).')),
        );
        return;
      }
      setState(() => _screenshot = file);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not load image: $e')),
      );
    }
  }

  void _removeImage() {
    setState(() => _screenshot = null);
  }

  Future<void> _submit() async {
    if (_submitting) return;
    if (!(_formKey.currentState?.validate() ?? false)) return;

    setState(() => _submitting = true);
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    final auth = context.read<AuthService>();
    final isLoggedIn = auth.isLoggedIn;

    final ok = await BugReportService.submitBugReport(
      description: _descriptionController.text.trim(),
      screenshot: _screenshot,
      userEmail: isLoggedIn ? null : _emailController.text.trim(),
      authToken: isLoggedIn ? auth.accessToken : null,
      context: {
        if (widget.currentRoute != null) 'route': widget.currentRoute,
        'platform': Platform.isIOS ? 'ios' : 'android',
      },
      endpointOverride: widget.endpointOverride,
    );

    if (!mounted) return;
    setState(() => _submitting = false);

    if (ok) {
      navigator.pop();
      messenger.showSnackBar(
        const SnackBar(content: Text('Thanks! We\'ll look into it.')),
      );
    } else {
      messenger.showSnackBar(
        const SnackBar(
          content: Text(
            'Could not submit bug report. Check your connection and try again.',
          ),
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
      // Push content above the keyboard when the description field is
      // focused. SafeArea + viewInsets covers iOS + Android both.
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: 16 + viewInsets.bottom,
      ),
      child: Form(
        key: _formKey,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  const Icon(Icons.bug_report_outlined),
                  const SizedBox(width: 8),
                  const Text(
                    'Report a bug',
                    style:
                        TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                  ),
                  const Spacer(),
                  IconButton(
                    icon: const Icon(Icons.close),
                    onPressed: _submitting
                        ? null
                        : () => Navigator.of(context).pop(),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              TextFormField(
                controller: _descriptionController,
                minLines: 4,
                maxLines: 8,
                maxLength: 2000,
                textCapitalization: TextCapitalization.sentences,
                decoration: const InputDecoration(
                  labelText: 'What happened?',
                  hintText:
                      'Describe the issue with as much detail as possible.',
                  border: OutlineInputBorder(),
                ),
                validator: (value) {
                  final trimmed = value?.trim() ?? '';
                  if (trimmed.length < 10) {
                    return 'Please add at least 10 characters.';
                  }
                  return null;
                },
              ),
              if (!isLoggedIn) ...[
                const SizedBox(height: 8),
                TextFormField(
                  controller: _emailController,
                  keyboardType: TextInputType.emailAddress,
                  autocorrect: false,
                  decoration: const InputDecoration(
                    labelText: 'Your email',
                    hintText: 'so we can follow up',
                    border: OutlineInputBorder(),
                  ),
                  validator: (value) {
                    final trimmed = value?.trim() ?? '';
                    if (trimmed.isEmpty) return 'Email is required.';
                    if (!_emailRegex.hasMatch(trimmed)) {
                      return 'Enter a valid email address.';
                    }
                    return null;
                  },
                ),
              ],
              const SizedBox(height: 16),
              const Text('Attach screenshot (optional)',
                  style: TextStyle(fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      icon: const Icon(Icons.photo_camera_outlined),
                      label: const Text('Camera'),
                      onPressed: _submitting
                          ? null
                          : () => _pickImage(ImageSource.camera),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: OutlinedButton.icon(
                      icon: const Icon(Icons.photo_library_outlined),
                      label: const Text('Gallery'),
                      onPressed: _submitting
                          ? null
                          : () => _pickImage(ImageSource.gallery),
                    ),
                  ),
                ],
              ),
              if (_screenshot != null) ...[
                const SizedBox(height: 12),
                Stack(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: Image.file(
                        _screenshot!,
                        height: 160,
                        width: double.infinity,
                        fit: BoxFit.cover,
                      ),
                    ),
                    Positioned(
                      top: 4,
                      right: 4,
                      child: Material(
                        color: Colors.black54,
                        shape: const CircleBorder(),
                        child: IconButton(
                          icon: const Icon(Icons.close, color: Colors.white),
                          tooltip: 'Remove screenshot',
                          onPressed: _submitting ? null : _removeImage,
                        ),
                      ),
                    ),
                  ],
                ),
              ],
              const SizedBox(height: 20),
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
                      onPressed: _submitting ? null : _submit,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );

    if (isIOS) {
      // Cupertino modal popup gives us a translucent backdrop; wrap the
      // form in a Material so TextFormField + Material widgets render
      // correctly inside it.
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
