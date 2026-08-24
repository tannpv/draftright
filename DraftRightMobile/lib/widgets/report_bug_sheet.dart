import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';

import 'package:draftright_mobile/services/auth_service.dart';
import 'package:draftright_mobile/services/bug_report_service.dart';
import 'package:draftright_mobile/services/settings_service.dart';
import 'package:draftright_mobile/services/screenshot_compressor.dart';

/// Opens the "Report a bug" sheet. Bottom sheet on Android; Cupertino
/// modal popup on iOS for native feel.
///
/// [currentRoute] is included in the report's `context` JSON so triagers
/// can see what screen the user was on when they reported.
/// [endpointOverride] redirects the submission target — production by
/// default; integration tests point it at a local stub server.
/// [initialDescription] pre-fills the description field — used when the
/// sheet is opened from an auto-captured error notice so the user doesn't
/// have to retype what just happened.
/// [initialScreenshotBytes] are PNG bytes of the screen the user was on,
/// captured by the launcher (the floating report button) BEFORE this sheet
/// was shown. They're compressed + pre-attached so a report ships with a
/// screenshot by default; the user can reselect, retake, or remove it (#135).
Future<void> showReportBugSheet(
  BuildContext context, {
  String? currentRoute,
  String? endpointOverride,
  String? initialDescription,
  Uint8List? initialScreenshotBytes,
}) async {
  if (Platform.isIOS) {
    await showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) => _ReportBugSheet(
        currentRoute: currentRoute,
        endpointOverride: endpointOverride,
        initialDescription: initialDescription,
        initialScreenshotBytes: initialScreenshotBytes,
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
        initialDescription: initialDescription,
        initialScreenshotBytes: initialScreenshotBytes,
      ),
    );
  }
}

class _ReportBugSheet extends StatefulWidget {
  final String? currentRoute;
  final String? endpointOverride;
  final String? initialDescription;
  final Uint8List? initialScreenshotBytes;
  const _ReportBugSheet({
    this.currentRoute,
    this.endpointOverride,
    this.initialDescription,
    this.initialScreenshotBytes,
  });

  @override
  State<_ReportBugSheet> createState() => _ReportBugSheetState();
}

class _ReportBugSheetState extends State<_ReportBugSheet> {
  late final TextEditingController _descriptionController =
      TextEditingController(text: widget.initialDescription ?? '');
  final _emailController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  final _picker = ImagePicker();

  File? _screenshot;
  bool _submitting = false;

  // Failures are shown in this in-sheet banner, NOT via ScaffoldMessenger: a
  // snackbar renders *behind* the modal sheet and is invisible, which is why
  // a failed submit looked like "nothing happened" (issue #68). Null = hidden.
  String? _errorText;

  static final _emailRegex = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');

  // Guiding template dropped into an EMPTY description once a screenshot is
  // attached, so the user has a structure to fill in rather than a blank box
  // next to the image. Editable; never overwrites text they already typed.
  static const _capturedDescriptionTemplate =
      'What I did:\n\nWhat I expected:\n\nWhat actually happened:\n';

  /// Seed the description with [_capturedDescriptionTemplate] the first time a
  /// screenshot is attached, only if the field is still empty. One place both
  /// the auto-capture and manual-pick paths call (RULE #1).
  void _seedDescriptionForScreenshot() {
    if (_descriptionController.text.trim().isEmpty) {
      _descriptionController.text = _capturedDescriptionTemplate;
    }
  }

  // Pre-fill the email field once, from the signed-in user's cached address,
  // so logged-in users don't retype it (and can still edit before sending).
  bool _prefilledEmail = false;

  @override
  void initState() {
    super.initState();
    final bytes = widget.initialScreenshotBytes;
    if (bytes != null && bytes.isNotEmpty) {
      // Adopt after the first frame so setState is safe.
      WidgetsBinding.instance.addPostFrameCallback((_) => _adoptCapturedBytes(bytes));
    }
  }

  /// Compress + attach the auto-captured screen (#135). Best-effort: any
  /// failure or an oversized shot just leaves the slot empty — the user can
  /// still attach manually.
  Future<void> _adoptCapturedBytes(Uint8List bytes) async {
    try {
      final dir = await getTemporaryDirectory();
      final raw = File(
          '${dir.path}/report_capture_${DateTime.now().millisecondsSinceEpoch}.png');
      await raw.writeAsBytes(bytes);
      final file = await ScreenshotCompressor.compressForUpload(raw);
      if (await file.length() > BugReportService.maxScreenshotBytes) return;
      if (!mounted) return;
      setState(() {
        _screenshot = file;
        _errorText = null;
        _seedDescriptionForScreenshot();
      });
    } catch (_) {/* auto-capture is optional — silently skip */}
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_prefilledEmail) return;
    _prefilledEmail = true;
    // Provider may be out of scope when the sheet is opened from the
    // error-notice overlay (above MultiProvider) — same fallback as _submit.
    try {
      final auth = context.read<AuthService>();
      final email = auth.userEmail;
      if (auth.isLoggedIn &&
          email != null &&
          email.isNotEmpty &&
          _emailController.text.isEmpty) {
        _emailController.text = email;
      }
    } catch (_) {/* no AuthService in scope — leave the field empty */}
  }

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

      // Downscale + recompress before we ever check the size or upload. This
      // is what keeps the multipart body under the backend's 5 MB cap; the
      // raw pick (esp. a camera photo) can be 8-11 MB and would 413 (#68).
      final file =
          await ScreenshotCompressor.compressForUpload(File(picked.path));
      final length = await file.length();
      if (length > BugReportService.maxScreenshotBytes) {
        if (!mounted) return;
        setState(() => _errorText =
            'That screenshot is too large even after compression (max 5 MB). '
            'Try a smaller image.');
        return;
      }
      setState(() {
        _screenshot = file;
        _errorText = null;
        _seedDescriptionForScreenshot();
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _errorText = 'Could not load that image: $e');
    }
  }

  void _removeImage() {
    setState(() => _screenshot = null);
  }

  Future<void> _submit() async {
    if (_submitting) return;
    if (!(_formKey.currentState?.validate() ?? false)) return;

    setState(() {
      _submitting = true;
      _errorText = null;
    });
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    // The sheet may be opened from a context that's above MultiProvider
    // (e.g. the auto-error overlay's snackbar action wires a root-navigator
    // route, which sits above the AuthService Provider). Fall back to an
    // anonymous submit if Provider isn't reachable — better than crashing
    // the only escape hatch users have for reporting that exact crash.
    AuthService? auth;
    try {
      auth = context.read<AuthService>();
    } catch (_) {
      auth = null;
    }
    final isLoggedIn = auth?.isLoggedIn ?? false;

    final email = _emailController.text.trim();
    // Default the target to the configured backend so a dev build never posts
    // to prod (#205 #3); an explicit override (e.g. tests) still wins and skips
    // the SettingsService read. Resolved before the first await.
    final endpoint = widget.endpointOverride ??
        context.read<SettingsService>().endpointFor('/bug-reports');
    final result = await BugReportService.submitBugReport(
      description: _descriptionController.text.trim(),
      screenshot: _screenshot,
      // Always send the (now always-shown) contact email; the backend still
      // stamps user_id from the token when logged in.
      userEmail: email.isEmpty ? null : email,
      authToken: isLoggedIn ? auth!.accessToken : null,
      context: {
        if (widget.currentRoute != null) 'route': widget.currentRoute,
        'platform': Platform.isIOS ? 'ios' : 'android',
      },
      endpointOverride: endpoint,
    );

    if (!mounted) return;
    setState(() => _submitting = false);

    if (result.ok) {
      navigator.pop();
      messenger.showSnackBar(
        const SnackBar(content: Text('Thanks! We\'ll look into it.')),
      );
    } else {
      // Surface the server's explanation (e.g. "only PNG or JPEG screenshots
      // are accepted") in an in-sheet banner. A snackbar here is invisible —
      // it renders behind the still-open modal sheet, which is exactly why a
      // failed submit looked like "nothing happened" (issue #68).
      setState(() => _errorText = result.errorMessage ??
          'Could not submit bug report. Check your connection and try again.');
    }
  }

  @override
  Widget build(BuildContext context) {
    // The email field is now always shown (pre-filled for logged-in users via
    // [didChangeDependencies]), so build no longer branches on auth state —
    // login-awareness lives only in _submit (token) and the prefill hook.
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
              // Shown for everyone: logged-out users must supply a contact
              // address; logged-in users see theirs pre-filled (from
              // [didChangeDependencies]) and can confirm or change it.
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
              if (_errorText != null) ...[
                const SizedBox(height: 16),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.red.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: Colors.red.shade400),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.error_outline,
                          color: Colors.red.shade400, size: 20),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _errorText!,
                          style: TextStyle(color: Colors.red.shade400),
                        ),
                      ),
                    ],
                  ),
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
