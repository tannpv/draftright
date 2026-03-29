// Stub for web platform — DesktopApp is never instantiated on web.
import 'package:flutter/material.dart';
import 'package:draftright_mobile/services/settings_service.dart';

class DesktopApp extends StatelessWidget {
  final SettingsService settings;
  const DesktopApp({super.key, required this.settings});

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
